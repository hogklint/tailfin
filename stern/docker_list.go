package stern

import (
	"context"
	"fmt"
	"iter"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

func ContainerGenerator(ctx context.Context, config *DockerConfig, client *dockerclient.Client) (iter.Seq[container.InspectResponse], error) {
	args := filters.NewArgs()
	for _, label := range config.Label {
		args.Add("label", label)
	}
	opts := container.ListOptions{All: true, Filters: args}
	containers, err := client.ContainerList(ctx, opts)
	if err != nil {
		return nil, err
	}
	return func(yield func(container.InspectResponse) bool) {
		for _, c := range containers {
			container, err := client.ContainerInspect(ctx, c.ID)
			if err != nil {
				fmt.Fprintf(config.ErrOut, "failed to inspect container id=%s: %v\n", c.ID, err)
				continue
			}
			if !yield(container) {
				return
			}
		}
	}, nil
}

func FilteredContainerGenerator(ctx context.Context, config *DockerConfig, client *dockerclient.Client, filter *dockerTargetFilter) (iter.Seq[*DockerTarget], error) {
	containers, err := ContainerGenerator(ctx, config, client)
	if err != nil {
		return nil, err
	}

	return func(yield func(*DockerTarget) bool) {
		visitor := func(t *DockerTarget) {
			yield(t)
		}
		for target := range containers {
			filter.visit(target, visitor)
		}
	}, nil
}
