package stern

import (
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"k8s.io/klog/v2"
)

type DockerTarget struct {
	Id    string
	Name  string
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
	var matchedName string = ""
	for _, name := range container.Names {
		if len(matchedName) == 0 && f.config.containerFilter.MatchString(name) {
			matchedName = name
		}
		for _, re := range f.config.containerExcludeFilter {
			if re.MatchString(name) {
				klog.V(7).InfoS("Container matches exclude filter", "id", container.ID, "names", container.Names, "excludeFilter", re)
				return
			}
		}
	}
	if len(matchedName) == 0 {
		return
	}

	fixedNames := make([]string, len(container.Names))
	for i, name := range container.Names {
		fixedNames[i] = strings.TrimPrefix(name, "/")
	}
	t := &DockerTarget{
		Id:    container.ID,
		Name:  strings.TrimPrefix(matchedName, "/"),
		Names: fixedNames,
	}

	visitor(t)
}
