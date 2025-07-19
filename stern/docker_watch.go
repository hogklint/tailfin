package stern

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

func WatchDockers(ctx context.Context, config *DockerConfig, filter *dockerTargetFilter, client *dockerclient.Client) (chan *DockerTarget, error) {
	added := make(chan *DockerTarget)
	go func() {
		visitor := func(t *DockerTarget) {
			log.L.WithFields(log.Fields{"id": t.Id, "name": t.Name}).Info("Active container")
			added <- t
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Start watching for container events
		args := filters.NewArgs()
		args.Add("event", string(events.ActionDie))
		args.Add("event", string(events.ActionStart))
		args.Add("event", string(events.ActionDestroy))
		for _, label := range config.Label {
			args.Add("label", label)
		}
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
					log.L.WithField("id", e.ID).Info("Inspect container")
					container, err := client.ContainerInspect(ctx, e.ID)
					if err != nil {
						log.L.WithField("id", e.ID).Error(err, ": failed to inspect container")
						continue
					}
					filter.visit(container, visitor)
				case events.ActionDie:
					filter.inactive(e.ID)
				case events.ActionDestroy:
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
