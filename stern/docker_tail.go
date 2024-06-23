package stern

import (
	"bufio"
	"context"
	"fmt"
	"io"
	//"strings"

	//"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func StartTail(config *Config, apiClient *client.Client, target *DockerTarget) {
	err := Start(apiClient, target)
	if err != nil {
		fmt.Fprintf(config.ErrOut, "Some error occured 4567: %v\n", err)
		return
	}
}

func Start(apiClient *client.Client, target *DockerTarget) error {
	fmt.Println("Getting logs", target.Id, target.Names[0])
	logs, err := apiClient.ContainerLogs(context.Background(), target.Id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
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
