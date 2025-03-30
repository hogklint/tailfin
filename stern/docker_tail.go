package stern

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/fatih/color"
	"k8s.io/klog/v2"
)

type DockerTail struct {
	client         *dockerclient.Client
	ContainerId    string
	ContainerName  string
	ComposeProject string
	Tty            bool
	composeColor   *color.Color
	containerColor *color.Color
	Options        *TailOptions
	tmpl           *template.Template
	closed         chan struct{}
	last           struct {
		timestamp string // RFC3339 timestamp (not RFC3339Nano)
		lines     int    // the number of lines seen during this timestamp
	}
	resumeRequest *ResumeRequest
	out           io.Writer
	errOut        io.Writer
}

func NewDockerTail(
	client *dockerclient.Client,
	containerId string,
	containerName string,
	composeProject string,
	tty bool,
	tmpl *template.Template,
	out, errOut io.Writer,
	options *TailOptions,
) *DockerTail {
	composeColor, containerColor := determineDockerColor(containerName, composeProject)

	return &DockerTail{
		client:         client,
		ContainerId:    containerId,
		ContainerName:  containerName,
		ComposeProject: composeProject,
		Tty:            tty,
		Options:        options,
		composeColor:   composeColor,
		containerColor: containerColor,
		tmpl:           tmpl,
		closed:         make(chan struct{}),
		out:            out,
		errOut:         errOut,
	}
}

func determineDockerColor(containerName, composeProject string) (*color.Color, *color.Color) {
	containerColor := colorList[colorIndex(containerName)][1]
	if composeProject == "" {
		return colorList[0][0], containerColor
	}
	return colorList[colorIndex(composeProject)][0], containerColor
}

func (t *DockerTail) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-t.closed
		cancel()
	}()

	t.printStarting()

	err := t.consumeRequest(ctx)
	if err != nil {
		klog.V(7).ErrorS(err, "Error fetching logs for container", "name", t.ContainerName, "id", t.ContainerId)
		if errors.Is(err, context.Canceled) || errdefs.IsConflict(err) {
			return nil
		}
	}

	return err
}

func (t *DockerTail) Close() {
	t.printStopping()
	close(t.closed)
}

func (t *DockerTail) Resume(ctx context.Context, resumeRequest *ResumeRequest) error {
	t.resumeRequest = resumeRequest
	t.Options.DockerSinceTime = resumeRequest.Timestamp
	t.Options.DockerTailLines = "-1"
	return t.Start(ctx)
}

func (t *DockerTail) consumeRequest(ctx context.Context) error {
	logs, err := t.client.ContainerLogs(
		ctx,
		t.ContainerId,
		container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     t.Options.Follow,
			Timestamps: true,
			Since:      t.Options.DockerSinceTime,
			Tail:       t.Options.DockerTailLines,
		},
	)
	if err != nil {
		return err
	}
	defer logs.Close()

	r := bufio.NewReader(logs)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) != 0 {
			t.consumeLine(strings.TrimSuffix(string(line), "\n"))
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

func (t *DockerTail) consumeLine(line string) {
	rfc3339Nano, content, err := splitLogLine(trimLeadingChars(line, t.Tty))
	if err != nil {
		t.Print(fmt.Sprintf("[%v] %s", err, line))
		return
	}

	rfc3339 := removeSubsecond(rfc3339Nano)
	t.rememberLastTimestamp(rfc3339)
	if t.resumeRequest.shouldSkip(rfc3339) {
		return
	}
	t.resumeRequest = nil

	if t.Options.IsExclude(content) || !t.Options.IsInclude(content) {
		return
	}

	msg := t.Options.HighlightMatchedString(content)

	if t.Options.Timestamps {
		updatedTs, err := t.Options.UpdateTimezoneAndFormat(rfc3339Nano)
		if err != nil {
			t.Print(fmt.Sprintf("[%v] %s", err, line))
			return
		}
		msg = updatedTs + " " + msg
	}

	t.Print(msg)
}

func (t *DockerTail) Print(msg string) {
	vm := Log{
		Message:        msg,
		ContainerName:  t.ContainerName,
		ComposeProject: t.ComposeProject,
		ComposeColor:   t.composeColor,
		ContainerColor: t.containerColor,
	}

	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, vm); err != nil {
		fmt.Fprintf(t.errOut, "expanding template failed: %s\n", err)
		klog.V(7).ErrorS(err, "Template failure", "message", msg)
		return
	}
	fmt.Fprint(t.out, buf.String())
}

func (t *DockerTail) printStarting() {
	if !t.Options.OnlyLogLines {
		g := color.New(color.FgHiGreen, color.Bold).SprintFunc()
		p := t.composeColor.SprintFunc()
		c := t.containerColor.SprintFunc()
		if t.ComposeProject == "" {
			fmt.Fprintf(t.errOut, "%s %s\n", g("+"), c(t.ContainerName))
		} else {
			fmt.Fprintf(t.errOut, "%s %s › %s\n", g("+"), p(t.ComposeProject), c(t.ContainerName))
		}
	}
}

func (t *DockerTail) printStopping() {
	if !t.Options.OnlyLogLines {
		r := color.New(color.FgHiRed, color.Bold).SprintFunc()
		p := t.composeColor.SprintFunc()
		c := t.containerColor.SprintFunc()
		if t.ComposeProject == "" {
			fmt.Fprintf(t.errOut, "%s %s\n", r("-"), c(t.ContainerName))
		} else {
			fmt.Fprintf(t.errOut, "%s %s › %s\n", r("-"), p(t.ComposeProject), c(t.ContainerName))
		}
	}
}

// Container stream format: https://docs.docker.com/reference/api/engine/version/v1.47/#tag/Container/operation/ContainerAttach
// When TTY is not enabled the lines are prefixed with stream type (stdin/stdout/stderr). We don't need it, so it's just
// stripped away.
func trimLeadingChars(line string, tty bool) string {
	if tty {
		return line
	}
	if len(line) < 8 {
		// And sometimes the line is something else...?
		klog.V(7).InfoS("Invalid log line format received", "line", line)
		return ""
	}
	return line[8:]
}

func (t *DockerTail) rememberLastTimestamp(timestamp string) {
	if t.last.timestamp == timestamp {
		t.last.lines++
		return
	}
	t.last.timestamp = timestamp
	t.last.lines = 1
}

func (t *DockerTail) GetResumeRequest() *ResumeRequest {
	if t.last.timestamp == "" {
		return nil
	}
	return &ResumeRequest{Timestamp: t.last.timestamp, LinesToSkip: t.last.lines}
}
