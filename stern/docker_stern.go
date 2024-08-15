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
		containerExcludeFilter: config.ExcludeContainerQuery,
	})

	added, err := WatchDockers(ctx, config, filter, client)
	if err != nil {
		fmt.Fprintf(config.ErrOut, "failed to list containers: %v\n", err)
		return err
	}

	tailTarget := func(target *DockerTarget) {
		options := &TailOptions{
			Timestamps:      config.Timestamps,
			TimestampFormat: config.TimestampFormat,
			Location:        config.Location,
			SinceSeconds:    nil,
			Exclude:         config.Exclude,
			Include:         config.Include,
			Highlight:       config.Highlight,
			Namespace:       false,
			TailLines:       config.TailLines,
			Follow:          config.Follow,
			OnlyLogLines:    config.OnlyLogLines,
		}
		tail := NewDockerTail(client, target.Id, target.Name, config.Template, config.Out, config.ErrOut, options)
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
