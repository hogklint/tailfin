package stern

import (
	"github.com/docker/docker/api/types"
	"regexp"
)

type DockerTarget struct {
	Id    string
	Names []string
}

type dockerTargetFilterConfig struct {
	containerFilter        *regexp.Regexp
	containerExcludeFilter []*regexp.Regexp
}

type dockerTargetFilter struct {
	config dockerTargetFilterConfig
}

func newDockerTargetFilter(filterConfig dockerTargetFilterConfig) *dockerTargetFilter {
	return &dockerTargetFilter{
		config: filterConfig,
	}
}

func (f *dockerTargetFilter) visit(container types.Container, visitor func(t *DockerTarget)) {
	matched := false
	for _, name := range container.Names {
		if f.config.containerFilter.MatchString(name) {
			matched = true
		}
		for _, re := range f.config.containerExcludeFilter {
			if re.MatchString(name) {
				return
			}
		}
	}
	if !matched {
		return
	}

	t := &DockerTarget{
		Id:    container.ID,
		Names: container.Names,
	}

	visitor(t)
}
