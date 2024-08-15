package stern

import (
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"k8s.io/klog/v2"
)

type DockerTarget struct {
	Id         string
	Name       string
	StartedAt  string
	FinishedAt string
}

type dockerTargetFilterConfig struct {
	containerFilter        *regexp.Regexp
	containerExcludeFilter []*regexp.Regexp
}

type dockerTargetFilter struct {
	config dockerTargetFilterConfig
	mu     sync.RWMutex
}

func newDockerTargetFilter(filterConfig dockerTargetFilterConfig) *dockerTargetFilter {
	return &dockerTargetFilter{
		config: filterConfig,
	}
}

func (f *dockerTargetFilter) visit(container types.ContainerJSON, visitor func(t *DockerTarget)) {
	if !f.config.containerFilter.MatchString(container.Name) {
		return
	}
	for _, re := range f.config.containerExcludeFilter {
		if re.MatchString(container.Name) {
			klog.V(7).InfoS("Container matches exclude filter", "id", container.ID, "names", container.Name, "excludeFilter", re)
			return
		}
	}

	target := &DockerTarget{
		Id:         container.ID,
		Name:       strings.TrimPrefix(container.Name, "/"),
		StartedAt:  container.State.StartedAt,
		FinishedAt: container.State.FinishedAt,
	}
	visitor(target)
}

func (f *dockerTargetFilter) forget(podUID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	//for targetID, state := range f.targetStates {
	//	if state.podUID == podUID {
	//		klog.V(7).InfoS("Forget targetState", "target", targetID)
	//		delete(f.targetStates, targetID)
	//	}
	//}
}
