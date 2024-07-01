package stern

import (
	"context"
	"fmt"
	"sync"

	dockerclient "github.com/docker/docker/client"
)

func RunDocker(ctx context.Context, client *dockerclient.Client, config *Config) error {
	// TOOD: Use container queries
	filter := newDockerTargetFilter(dockerTargetFilterConfig{
		containerFilter:        config.PodQuery,
		containerExcludeFilter: config.ExcludePodQuery,
	})

	added, err := WatchDockers(config, filter, client)
	if err != nil {
		fmt.Fprintf(config.ErrOut, "failed to list containers: %v\n", err)
		return err
	}

	tailTarget := func(target *DockerTarget) {
		tail := NewDockerTail(client, target.Id, target.Names, config.Template, config.Out, config.ErrOut)
		tail.Start()
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			target := <-added
			go func() {
				tailTarget(target)
			}()
		}
	}()
	wg.Wait()
	return nil
}
