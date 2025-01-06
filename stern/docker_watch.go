package stern

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"k8s.io/klog/v2"
)

func WatchDockers(ctx context.Context, config *DockerConfig, filter *dockerTargetFilter, client *dockerclient.Client) (chan *DockerTarget, error) {
	added := make(chan *DockerTarget)
	go func() {
		visitor := func(t *DockerTarget) {
			klog.V(7).InfoS("New container", "id", t.Id, "name", t.Name)
			added <- t
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Start watching for container events
		args := filters.NewArgs()
		args.Add("event", string(events.ActionDie))
		args.Add("event", string(events.ActionStart))
		opts := types.EventsOptions{Filters: args}
		watcher, errc := client.Events(ctx, opts)

		// Then list all current containers
		containers, err := ContainerGenerator(ctx, config, client)
		if err != nil {
			return
		}
		for target := range containers {
			filter.visit(target, visitor)
		}

		for {
			select {
			case e := <-watcher:
				switch e.Action {
				case events.ActionStart:
					container, err := client.ContainerInspect(ctx, e.ID)
					if err != nil {
						klog.V(7).ErrorS(err, "failed to inspect container", "id", e.ID)
						continue
					}
					klog.V(8).InfoS("Container inspect", "id", e.ID, "container JSON", container)
					filter.visit(container, visitor)
				case events.ActionDie:
					filter.forget(e.ID)
				}
			case <-ctx.Done():
				close(added)
				return
			case <-errc:
				close(added)
				return
			}
		}
	}()
	return added, nil
}
