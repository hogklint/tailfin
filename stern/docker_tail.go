package stern

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func StartDocker() {
	// Maybe hardcode the version to the lowest version needed?
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		panic(err)
	}

	for _, ctr := range containers {
		fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
		go Start(apiClient, ctr)
	}
	time.Sleep(30 * time.Second)
}

func Start(apiClient *client.Client, ctr types.Container) {
	fmt.Println("Getting logs", ctr.ID, ctr.Names)
	logs, err := apiClient.ContainerLogs(context.Background(), ctr.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		panic(err)
	}
	fmt.Println("Got logs")

	r := bufio.NewReader(logs)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			panic(err)
		}
		if len(line) != 0 {
			fmt.Println(strings.TrimSuffix(string(line), "\n"))
		}
	}
}
