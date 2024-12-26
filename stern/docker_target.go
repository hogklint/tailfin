package stern

import (
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"k8s.io/klog/v2"
)

type DockerTarget struct {
	Id             string
	Name           string
	StartedAt      string
	FinishedAt     string
	ComposeProject string
}

type dockerTargetFilterConfig struct {
	containerFilter        *regexp.Regexp
	containerExcludeFilter []*regexp.Regexp
}

type dockerTargetFilter struct {
	config           dockerTargetFilterConfig
	activeContainers map[string]bool
	mu               sync.RWMutex
}

func newDockerTargetFilter(filterConfig dockerTargetFilterConfig) *dockerTargetFilter {
	return &dockerTargetFilter{
		config:           filterConfig,
		activeContainers: make(map[string]bool),
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

	if !f.config.containerFilter.MatchString(containerName) {
		return
	}
	for _, re := range f.config.containerExcludeFilter {
		if re.MatchString(containerName) {
			klog.V(7).InfoS("Container matches exclude filter", "id", container.ID, "names", containerName, "excludeFilter", re)
			return
		}
	}

	target := &DockerTarget{
		Id:             container.ID,
		Name:           containerName,
		ComposeProject: composeProject,
		StartedAt:      container.State.StartedAt,
		FinishedAt:     container.State.FinishedAt,
	}

	if f.shouldAdd(target) {
		visitor(target)
	}
}

func (f *dockerTargetFilter) shouldAdd(t *DockerTarget) bool {
	f.mu.Lock()
	_, alreadyActive := f.activeContainers[t.Id]
	f.activeContainers[t.Id] = true
	f.mu.Unlock()

	if alreadyActive {
		klog.V(7).InfoS("Container ID existed before observation",
			"id", t.Id, "name", t.Name)
		return false
	}

	return true
}

func (f *dockerTargetFilter) forget(containerId string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	klog.V(7).InfoS("Forget container", "target", containerId)
	delete(f.activeContainers, containerId)
}

func (f *dockerTargetFilter) isActive(t *DockerTarget) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.activeContainers[t.Id]
	return ok
}
