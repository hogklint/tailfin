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
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func StartDocker() {
	// Maybe hardcode the version to the lowest version needed?
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	var lastCtr string = ""
	opts := container.ListOptions{All: false}
	for {
		if len(lastCtr) > 0 {
			args := filters.NewArgs(filters.KeyValuePair{Key: "since", Value: lastCtr})
			if err != nil {
				panic(err)
			}
			opts.Filters = args
			//opts.Filters = filters.Args{"since": {lastCtr: true}}
		} else {
			fmt.Println("Listing all containers")
		}
		//fmt.Println("Opts", opts.Since)
		containers, err := apiClient.ContainerList(context.Background(), opts)
		if err != nil {
			panic(err)
		}

		for _, ctr := range containers {
			//fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
			fmt.Printf("Started: %s\n", time.Unix(ctr.Created, 0))
			go Start(apiClient, ctr)
			lastCtr = ctr.ID
		}
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
