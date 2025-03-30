package stern

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	dockerclient "github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

func RunDocker(ctx context.Context, client *dockerclient.Client, config *DockerConfig) error {
	newTailOptions := func() *TailOptions {
		return &TailOptions{
			Timestamps:      config.Timestamps,
			TimestampFormat: config.TimestampFormat,
			Location:        config.Location,
			DockerSinceTime: time.Now().Add(-config.Since),
			Exclude:         config.Exclude,
			Include:         config.Include,
			Highlight:       config.Highlight,
			DockerTailLines: strconv.FormatInt(config.TailLines, 10),
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
			target.Tty,
			target.StartedAt,
			target.FinishedAt,
			target.SeenPreviously,
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
		composeProjectFilter:   config.ComposeProjectQuery,
		imageFilter:            config.ImageQuery,
	},
		max(config.MaxLogRequests*2, 100),
	)

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
				err := tail.Start(ctx)
				if err != nil && filter.isActive(target) {
					fmt.Fprintf(config.ErrOut, "failed to tail %s: %v\n", target.Name, err)
					return err
				}
				return nil
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
		limiter := rate.NewLimiter(rate.Every(time.Second*20), 2)
		var resumeRequest *ResumeRequest
		for {
			if err := limiter.Wait(ctx); err != nil {
				fmt.Fprintf(config.ErrOut, "failed to retry: %v\n", err)
				return
			}
			tail := newTail(target)
			var err error
			if resumeRequest == nil {
				err = tail.Start(ctx)
			} else {
				err = tail.Resume(ctx, resumeRequest)
			}
			tail.Close()

			if err == nil {
				return
			}
			if !filter.isActive(target) {
				fmt.Fprintf(config.ErrOut, "failed to tail: %v\n", err)
				return
			}
			fmt.Fprintf(config.ErrOut, "failed to tail: %v, will retry\n", err)
			if resumeReq := tail.GetResumeRequest(); resumeReq != nil {
				resumeRequest = resumeReq
			}
		}
	}

	var numRequests atomic.Int64
	for {
		target := <-added
		numRequests.Add(1)
		if numRequests.Load() > int64(config.MaxLogRequests) {
			return fmt.Errorf("tailfin reached the maximum number of log requests (%d),"+
				" use --max-log-requests to increase the limit\n",
				config.MaxLogRequests)
		}
		go func() {
			tailTarget(target)
			numRequests.Add(-1)
		}()
	}
}
