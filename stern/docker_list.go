package stern

import (
	"context"
	"fmt"
	"iter"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

func ContainerGenerator(ctx context.Context, config *DockerConfig, client *dockerclient.Client) (iter.Seq[types.ContainerJSON], error) {
	opts := container.ListOptions{All: true}
	containers, err := client.ContainerList(ctx, opts)
	if err != nil {
		return nil, err
	}
	return func(yield func(types.ContainerJSON) bool) {
		for _, c := range containers {
			container, err := client.ContainerInspect(ctx, c.ID)
			if err != nil {
				fmt.Fprintf(config.ErrOut, "failed to inspect container id=%s: %v\n", container.ID, err)
				continue
			}
			yield(container)
		}
	}, nil
}
