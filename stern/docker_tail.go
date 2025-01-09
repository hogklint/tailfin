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
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/fatih/color"
	"k8s.io/klog/v2"
)

type DockerTail struct {
	client         *dockerclient.Client
	ContainerId    string
	ContainerName  string
	ComposeProject string
	Tty            bool
	StartedAt      time.Time
	FinishedAt     string
	composeColor   *color.Color
	containerColor *color.Color
	Options        *TailOptions
	tmpl           *template.Template
	closed         chan struct{}
	out            io.Writer
	errOut         io.Writer
}

func NewDockerTail(
	client *dockerclient.Client,
	containerId string,
	containerName string,
	composeProject string,
	tty bool,
	startedAt time.Time,
	finishedAt string,
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
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
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
		if errors.Is(err, context.Canceled) {
			return nil
		}
	}

	return err
}

func (t *DockerTail) Close() {
	t.printStopping()
	close(t.closed)
}

// The maximum "since time" is the time the container was last started. Because logs are sometimes missing when using
// the start time, the finished time is used instead (because there are no logs between the finish and start time). If
// the Options.SinceTime is after the finished time it'll be used with the same reasoning in addition to it might be
// after the start time, which in that case is the desired "since time".
func (t *DockerTail) getSinceTime() time.Time {
	finished, err := time.Parse(time.RFC3339, t.FinishedAt)
	// If there's no finish time it should mean the container only started once so it's safe to use the options time. The
	// same applies if the finish time is earlier than the option time.
	if err != nil ||
		finished.Before(t.Options.DockerSinceTime) ||
		t.StartedAt.Before(t.Options.DockerSinceTime) {
		return t.Options.DockerSinceTime
	}

	// Sometimes early logs are missing if StartedAt is used
	if finished.Before(t.StartedAt) {
		return finished
	}
	return t.StartedAt
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
			Since:      t.getSinceTime().Format(time.RFC3339),
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
		NodeName:       "",
		Namespace:      "",
		PodName:        "",
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
