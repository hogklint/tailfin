package stern

import (
	"context"
	"fmt"
	//"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerTarget struct {
	Id   string
	Name string
}

func Go(config *Config) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	added, err := WatchDockers(config, apiClient)
	if err != nil {
		fmt.Fprintf(config.ErrOut, "failed to list containers: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			target := <-added
			StartTail(config, apiClient, target)

		}
	}()
	wg.Wait()
}

func WatchDockers(config *Config, apiClient *client.Client) (chan *DockerTarget, error) {
	added := make(chan *DockerTarget)
	go func() {
		knownContainers := make(map[string]bool)
		opts := container.ListOptions{All: true}
		for {
			containers, err := apiClient.ContainerList(context.Background(), opts)
			if err != nil {
				return
			}

			newContainers := make(map[string]bool)
			for _, ctr := range containers {
				//fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
				if _, ok := knownContainers[ctr.ID]; !ok {
					t := &DockerTarget{Id: ctr.ID, Name: ctr.Names[0]}
					fmt.Printf("New container: %s\n", *t)
					added <- t
				}
				newContainers[ctr.ID] = true
			}
			knownContainers = newContainers
			time.Sleep(1 * time.Second)
		}
	}()
	return added, nil
}
