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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			target := <-added
			StartTail(config, client, target)
		}
	}()
	wg.Wait()
	return nil
}
