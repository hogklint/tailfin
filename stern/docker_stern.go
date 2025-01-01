package stern

import (
	"context"
	"fmt"
	"os"
	"sync"

	dockerclient "github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

func RunDocker(ctx context.Context, client *dockerclient.Client, config *DockerConfig) error {
	newTailOptions := func() *TailOptions {
		return &TailOptions{
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
	}
	newTail := func(target *DockerTarget) *DockerTail {
		return NewDockerTail(
			client,
			target.Id,
			target.Name,
			target.ComposeProject,
			target.StartedAt,
			target.FinishedAt,
			config.Template,
			config.Out,
			config.ErrOut,
			newTailOptions(),
		)
	}

	if config.Stdin {
		tail := NewFileTail(config.Template, os.Stdin, config.Out, config.ErrOut, newTailOptions())
		return tail.Start()
	}

	filter := newDockerTargetFilter(dockerTargetFilterConfig{
		containerFilter:        config.ContainerQuery,
		containerExcludeFilter: config.ExcludeContainerQuery,
	})

	if !config.Follow {
		containers, err := FilteredContainerGenerator(ctx, config, client, filter)
		if err != nil {
			return err
		}

		var eg errgroup.Group
		eg.SetLimit(config.MaxLogRequests)
		for target := range containers {
			target := target
			eg.Go(func() error {
				tail := newTail(target)
				defer tail.Close()
				return tail.Start(ctx)
			})
		}
		return eg.Wait()
	}

	added, err := WatchDockers(ctx, config, filter, client)
	if err != nil {
		fmt.Fprintf(config.ErrOut, "failed to list containers: %v\n", err)
		return err
	}

	tailTarget := func(target *DockerTarget) {
		tail := newTail(target)
		defer tail.Close()
		err := tail.Start(ctx)
		if err != nil && filter.isActive(target) {
			fmt.Fprintf(config.ErrOut, "failed to tail %s: %v\n", target.Name, err)
		}
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
