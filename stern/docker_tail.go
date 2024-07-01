package stern

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"text/template"
	//"strings"

	"github.com/fatih/color"
	//"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type DockerTail struct {
	client         *dockerclient.Client
	ContainerId    string
	ContainerNames []string
	containerColor *color.Color
	tmpl           *template.Template
	out            io.Writer
	errOut         io.Writer
}

func NewDockerTail(client *dockerclient.Client, containerId string, containerNames []string, tmpl *template.Template, out, errOut io.Writer) *DockerTail {
	return &DockerTail{
		client:         client,
		ContainerId:    containerId,
		ContainerNames: containerNames,
		containerColor: nil,
		tmpl:           tmpl,
		out:            out,
		errOut:         errOut,
	}
}

func (t *DockerTail) Start( /*ctx?*/ ) {
	err := t.consumeRequest()
	if err != nil {
		fmt.Fprintf(t.errOut, "Some error occured: %v\n", err)
		return
	}
}

func (t *DockerTail) consumeRequest() error {
	fmt.Println("Getting logs", t.ContainerId, t.ContainerNames[0])
	logs, err := t.client.ContainerLogs(context.Background(), t.ContainerId, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		panic(err)
	}
	//fmt.Println("Got logs")
	gotLogs := false

	r := bufio.NewReader(logs)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) != 0 {
			if !gotLogs {
				fmt.Println("Got logs")
				break
				//fmt.Println(strings.TrimSuffix(string(line), "\n"))
			}
			gotLogs = true
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
	return nil
}
