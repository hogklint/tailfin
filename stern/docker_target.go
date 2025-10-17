package stern

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/container"
	"github.com/hashicorp/golang-lru/v2"
)

type DockerTarget struct {
	Id              string
	Name            string
	ServiceName     string
	ComposeProject  string
	ContainerNumber string
	Tty             bool
	ResumeRequest   *ResumeRequest
}

type dockerTargetFilterConfig struct {
	containerFilter        []*regexp.Regexp
	containerExcludeFilter []*regexp.Regexp
	composeProjectFilter   []*regexp.Regexp
	imageFilter            []*regexp.Regexp
}

type dockerTargetFilter struct {
	config           dockerTargetFilterConfig
	activeContainers map[string]time.Time
	seenContainers   *lru.Cache[string, *ResumeRequest]
	mu               sync.RWMutex
}

func newDockerTargetFilter(filterConfig dockerTargetFilterConfig, lruCacheSize int) *dockerTargetFilter {
	lru, err := lru.New[string, *ResumeRequest](lruCacheSize)
	if err != nil {
		panic(err)
	}

	return &dockerTargetFilter{
		config:           filterConfig,
		activeContainers: make(map[string]time.Time),
		seenContainers:   lru,
	}
}

func (f *dockerTargetFilter) visit(container container.InspectResponse, visitor func(t *DockerTarget)) {
	var composeProject, containerNumber string
	containerName := strings.TrimPrefix(container.Name, "/")
	serviceName := containerName
	if p, ok := container.Config.Labels["com.docker.compose.project"]; ok {
		composeProject = p
	}
	if s, ok := container.Config.Labels["com.docker.compose.service"]; ok {
		serviceName = s
	}
	if n, ok := container.Config.Labels["com.docker.compose.container-number"]; ok {
		containerNumber = n
	}

	if !f.matchingNameFilter(serviceName) ||
		!f.matchingComposeFilter(composeProject) ||
		!f.matchingImageFilter(container.Config.Image) ||
		f.matchingNameExcludeFilter(serviceName) {
		return
	}

	// Not yet started containers have no logs
	if container.State.Status == "created" {
		return
	}

	startedAt, err := time.Parse(time.RFC3339, container.State.StartedAt)
	if err != nil {
		return
	}

	var resumeRequest *ResumeRequest
	if rr, ok := f.seenContainers.Peek(container.ID); ok {
		resumeRequest = rr
	}
	target := &DockerTarget{
		Id:              container.ID,
		Name:            containerName,
		ServiceName:     serviceName,
		ComposeProject:  composeProject,
		ContainerNumber: containerNumber,
		Tty:             container.Config.Tty,
		ResumeRequest:   resumeRequest,
	}

	if f.shouldAdd(target, startedAt) {
		visitor(target)
	}
}

func (f *dockerTargetFilter) shouldAdd(t *DockerTarget, startedAt time.Time) bool {
	f.mu.Lock()
	activeStartedAt, found := f.activeContainers[t.Id]
	f.activeContainers[t.Id] = startedAt
	f.mu.Unlock()

	// Listed already terminated containers will not emit a Die event so they will stay in the activeContainers map. When
	// restarted it should still be added if the start time is different. If the start time is the same it means the
	// container was listed as well as received in a start event i.e. should not be added twice.
	if found && activeStartedAt.Equal(startedAt) {
		log.L.WithField("id", t.Id).WithField("name", t.Name).Info("Container ID existed before observation")
		return false
	}

	return true
}

func (f *dockerTargetFilter) matchingNameFilter(containerName string) bool {
	if len(f.config.containerFilter) == 0 {
		return true
	}

	for _, re := range f.config.containerFilter {
		if re.MatchString(containerName) {
			return true
		}
	}
	log.L.WithField("name", containerName).Info("Container name does not match filters")
	return false
}

func (f *dockerTargetFilter) matchingNameExcludeFilter(containerName string) bool {
	for _, re := range f.config.containerExcludeFilter {
		if re.MatchString(containerName) {
			log.L.WithField("name", containerName).WithField("excludeFilter", re).Info("Container name matches exclude filter")
			return true
		}
	}
	return false
}

func (f *dockerTargetFilter) matchingComposeFilter(composeProject string) bool {
	if len(f.config.composeProjectFilter) == 0 {
		return true
	} else if len(composeProject) > 0 {
		for _, re := range f.config.composeProjectFilter {
			if re.MatchString(composeProject) {
				return true
			}
		}
	}
	log.L.WithField("compose", composeProject).Info("Compose project name does not match filters")
	return false
}

func (f *dockerTargetFilter) matchingImageFilter(containerImage string) bool {
	if len(f.config.imageFilter) == 0 {
		return true
	}

	for _, re := range f.config.imageFilter {
		if re.MatchString(containerImage) {
			return true
		}
	}
	log.L.WithField("image", containerImage).Info("Image does not match image filters")
	return false
}

func (f *dockerTargetFilter) inactive(containerId string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	log.L.WithField("id", containerId).Info("Inactive container")
	delete(f.activeContainers, containerId)
}

func (f *dockerTargetFilter) setResumeRequest(containerId string, resume *ResumeRequest) {
	if resume == nil {
		return
	}
	log.L.WithField("id", containerId).WithField("resume", resume).Info("Storing resume request")
	f.seenContainers.Add(containerId, resume)
}

func (f *dockerTargetFilter) forget(containerId string) {
	log.L.WithField("id", containerId).Info("Forget container")
	// Actively remove container from LRU cache to minimize the risk of old (but not removed) containers getting evicted
	f.seenContainers.Remove(containerId)
}

func (f *dockerTargetFilter) isActive(t *DockerTarget) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.activeContainers[t.Id]
	return ok
}
