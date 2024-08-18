package stern

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

func ListDockers(ctx context.Context, config *DockerConfig, client *dockerclient.Client, filter *dockerTargetFilter, visitor func(t *DockerTarget)) ([]types.ContainerJSON, error) {
	opts := container.ListOptions{All: true}
	containers, err := client.ContainerList(ctx, opts)
	if err != nil {
		return []types.ContainerJSON{}, err
	}
	containersJson := make([]types.ContainerJSON, len(containers))
	for i, c := range containers {
		container, err := client.ContainerInspect(ctx, c.ID)
		if err != nil {
			fmt.Fprintf(config.ErrOut, "failed to inspect container id=%s: %v\n", container.ID, err)
			continue
		}
		containersJson[i] = container
	}
	return containersJson, err
}
