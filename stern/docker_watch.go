package stern

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"k8s.io/klog/v2"
)

func WatchDockers(ctx context.Context, config *Config, filter *dockerTargetFilter, client *dockerclient.Client) (chan *DockerTarget, error) {
	added := make(chan *DockerTarget)
	go func() {
		visitor := func(t *DockerTarget) {
			klog.V(7).InfoS("New container", "id", t.Id, "name", t.Name)
			added <- t
		}
		args := filters.NewArgs()
		args.Add("event", string(events.ActionStop))
		args.Add("event", string(events.ActionStart))
		ctx, cancel := context.WithCancel(ctx)
		opts := types.EventsOptions{Filters: args}
		watcher, errc := client.Events(ctx, opts)
		defer cancel()

		containers, err := ListDockers(ctx, config, client, filter, visitor)
		if err != nil {
			return
		}
		for _, target := range containers {
			filter.visit(target, visitor)
		}

		for {
			select {
			case e := <-watcher:
				switch e.Action {
				case events.ActionStart:
					container, err := client.ContainerInspect(ctx, e.ID)
					if err != nil {
						fmt.Fprintf(config.ErrOut, "failed to inspect container id=%s: %v\n", e.ID, err)
						continue
					}
					filter.visit(container, visitor)
				case events.ActionStop:
					filter.forget(e.ID)
				}
			case <-ctx.Done():
				close(added)
			case <-errc:
				close(added)
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return added, nil
}
