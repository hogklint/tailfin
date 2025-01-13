package stern

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/hashicorp/golang-lru/v2"
	"k8s.io/klog/v2"
)

type DockerTarget struct {
	Id             string
	Name           string
	ComposeProject string
	Tty            bool
	StartedAt      time.Time
	FinishedAt     string
	SeenPreviously bool
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
	seenContainers   *lru.Cache[string, bool]
	mu               sync.RWMutex
}

func newDockerTargetFilter(filterConfig dockerTargetFilterConfig, lruCacheSize int) *dockerTargetFilter {
	lru, err := lru.New[string, bool](lruCacheSize)
	if err != nil {
		panic(err)
	}

	return &dockerTargetFilter{
		config:           filterConfig,
		activeContainers: make(map[string]time.Time),
		seenContainers:   lru,
	}
}

func (f *dockerTargetFilter) visit(container types.ContainerJSON, visitor func(t *DockerTarget)) {
	composeProject := ""
	containerName := strings.TrimPrefix(container.Name, "/")
	if p, ok := container.Config.Labels["com.docker.compose.project"]; ok {
		composeProject = p
	}
	if s, ok := container.Config.Labels["com.docker.compose.service"]; ok {
		containerName = s
	}

	if !f.matchingNameFilter(containerName) ||
		!f.matchingComposeFilter(composeProject) ||
		!f.matchingImageFilter(container.Config.Image) ||
		f.matchingNameExcludeFilter(containerName) {
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

	target := &DockerTarget{
		Id:             container.ID,
		Name:           containerName,
		ComposeProject: composeProject,
		Tty:            container.Config.Tty,
		StartedAt:      startedAt,
		FinishedAt:     container.State.FinishedAt,
		SeenPreviously: f.seenContainers.Contains(container.ID),
	}

	if f.shouldAdd(target) {
		visitor(target)
	}
}

func (f *dockerTargetFilter) shouldAdd(t *DockerTarget) bool {
	f.mu.Lock()
	startedAt, found := f.activeContainers[t.Id]
	f.activeContainers[t.Id] = t.StartedAt
	f.mu.Unlock()

	// Listed already terminated containers will not emit a Die event so they will stay in the activeContainers map. When
	// restarted it should still be added if the start time is different. If the start time is the same it means the
	// container was listed as well as received in a start event i.e. should not be added twice.
	if found && startedAt == t.StartedAt {
		klog.V(7).InfoS("Container ID existed before observation",
			"id", t.Id, "name", t.Name)
		return false
	}

	f.seenContainers.Add(t.Id, true)
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
	klog.V(7).InfoS("Container name does not match filters", "image", containerName)
	return false
}

func (f *dockerTargetFilter) matchingNameExcludeFilter(containerName string) bool {
	for _, re := range f.config.containerExcludeFilter {
		if re.MatchString(containerName) {
			klog.V(7).InfoS("Container name matches exclude filter",
				"name", containerName, "excludeFilter", re)
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
	klog.V(7).InfoS("Compose project name does not match filters", "compose", composeProject)
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
	klog.V(7).InfoS("Image does not match image filters", "image", containerImage)
	return false
}

func (f *dockerTargetFilter) inactive(containerId string) {
	f.seenContainers.Add(containerId, true)
	f.mu.Lock()
	defer f.mu.Unlock()

	klog.V(7).InfoS("Inactive container", "target", containerId)
	delete(f.activeContainers, containerId)
}

func (f *dockerTargetFilter) forget(containerId string) {
	klog.V(7).InfoS("Forget container", "target", containerId)
	// Actively remove container from LRU cache to minimize the risk of old (but not removed) containers getting evicted
	f.seenContainers.Remove(containerId)
}

func (f *dockerTargetFilter) isActive(t *DockerTarget) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.activeContainers[t.Id]
	return ok
}
