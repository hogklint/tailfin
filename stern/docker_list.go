package stern

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"k8s.io/klog/v2"
)

func WatchDockers(config *Config, filter *dockerTargetFilter, client *dockerclient.Client) (chan *DockerTarget, error) {
	added := make(chan *DockerTarget)
	go func() {
		knownContainers := make(map[string]bool)
		opts := container.ListOptions{All: true}
		for {
			containers, err := client.ContainerList(context.Background(), opts)
			if err != nil {
				return
			}

			newContainers := make(map[string]bool)
			for _, ctr := range containers {
				//fmt.Printf("%s %s (status: %s)\n", ctr.ID, ctr.Image, ctr.Status)
				if _, ok := knownContainers[ctr.ID]; !ok {
					filter.visit(ctr, func(t *DockerTarget) {
						klog.V(7).InfoS("New container", "id", t.Id, "names", t.Names)
						added <- t
					})
				}
				newContainers[ctr.ID] = true
			}
			knownContainers = newContainers
			time.Sleep(1 * time.Second)
		}
	}()
	return added, nil
}
