package stern

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
	"k8s.io/klog/v2"
	//"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type DockerTail struct {
	client         *dockerclient.Client
	ContainerId    string
	ContainerName  string
	StartedAt      string
	FinishedAt     string
	containerColor *color.Color
	Options        *TailOptions
	tmpl           *template.Template
	out            io.Writer
	errOut         io.Writer
}

func NewDockerTail(client *dockerclient.Client, containerId, containerName, startedAt, finishedAt string, tmpl *template.Template, out, errOut io.Writer, options *TailOptions) *DockerTail {
	colors := colorList[colorIndex(containerId)]

	return &DockerTail{
		client:         client,
		ContainerId:    containerId,
		ContainerName:  containerName,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		Options:        options,
		containerColor: colors[1],
		tmpl:           tmpl,
		out:            out,
		errOut:         errOut,
	}
}

func (t *DockerTail) Start( /*ctx?*/ ) {
	t.printStarting()
	defer t.printStopping()

	err := t.consumeRequest()
	if err != nil {
		fmt.Fprintf(t.errOut, "tailing container '%v' failed: %v\n", t.ContainerName, err)
		klog.V(7).ErrorS(err, "Could not fetch logs for container", "name", t.ContainerName, "id", t.ContainerId)
		return
	}
}

func (t *DockerTail) getSinceTime() string {
	started, err1 := time.Parse(time.RFC3339, t.StartedAt)
	finished, err2 := time.Parse(time.RFC3339, t.FinishedAt)
	if err1 != nil || err2 != nil {
		return t.StartedAt
	}

	if finished.Before(started) {
		return t.FinishedAt
	}
	return t.StartedAt
}

func (t *DockerTail) consumeRequest() error {
	logs, err := t.client.ContainerLogs(
		context.Background(),
		t.ContainerId,
		container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
			Since:      t.getSinceTime(),
		},
	)
	if err != nil {
		return err
	}

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
	rfc3339Nano, content, err := splitLogLine(line)
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
		PodColor:       nil,
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
		c := t.containerColor.SprintFunc()
		fmt.Fprintf(t.errOut, "%s › %s\n", g("+"), c(t.ContainerName))
	}
}

func (t *DockerTail) printStopping() {
	if !t.Options.OnlyLogLines {
		r := color.New(color.FgHiRed, color.Bold).SprintFunc()
		c := t.containerColor.SprintFunc()
		fmt.Fprintf(t.errOut, "%s › %s\n", r("-"), c(t.ContainerName))
	}
}
