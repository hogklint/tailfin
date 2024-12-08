package stern

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	"golang.org/x/time/rate"
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
				err = tail.Start()
			} else {
				err = tail.Resume(resumeRequest)
			}
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
