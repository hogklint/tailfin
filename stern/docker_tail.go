package stern

import (
	"bufio"
	"context"
	"fmt"
	"io"
	//"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func StartDocker() {
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	knownContainers := make(map[string]bool)
	opts := container.ListOptions{All: true}
	for {
		containers, err := apiClient.ContainerList(context.Background(), opts)
		if err != nil {
			panic(err)
		}

		newContainers := make(map[string]bool)
		for _, ctr := range containers {
			//fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
			if _, ok := knownContainers[ctr.ID]; !ok {
				go Start(apiClient, ctr)
			}
			newContainers[ctr.ID] = true
		}
		knownContainers = newContainers
		time.Sleep(1 * time.Second)
	}
}

func Start(apiClient *client.Client, ctr types.Container) error {
	fmt.Println("Getting logs", ctr.ID, ctr.Names)
	logs, err := apiClient.ContainerLogs(context.Background(), ctr.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
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
