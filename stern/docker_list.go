package stern

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

func ListDockers(ctx context.Context, client *dockerclient.Client, filter *dockerTargetFilter, visitor func(t *DockerTarget)) ([]types.ContainerJSON, error) {
	opts := container.ListOptions{All: true}
	containers, err := client.ContainerList(ctx, opts)
	if err != nil {
		return []types.ContainerJSON{}, err
	}
	containersJson := make([]types.ContainerJSON, len(containers))
	for i, c := range containers {
		container, err := client.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}
		containersJson[i] = container
	}
	return containersJson, err
}
