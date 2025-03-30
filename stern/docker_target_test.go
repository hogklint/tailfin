package stern

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

func TestTargetFilter(t *testing.T) {
	validTimestamp := "2000-01-01T00:00:00+00:00"
	validTime, _ := time.Parse(time.RFC3339, validTimestamp)
	createContainer := func(compose, id, name, image string) types.ContainerJSON {
		var labels map[string]string
		jsonName := name
		if compose != "" {
			labels = map[string]string{
				"com.docker.compose.project": compose,
				"com.docker.compose.service": name,
			}
			jsonName = name + "-0"
		}
		return types.ContainerJSON{
			Mounts: []types.MountPoint{},
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: id,
				State: &types.ContainerState{
					Status:     "",
					Running:    true,
					Paused:     true,
					Restarting: true,
					OOMKilled:  true,
					Dead:       true,
					StartedAt:  validTimestamp,
					FinishedAt: "",
				},
				Name: jsonName,
			},
			Config: &container.Config{
				Image:  image,
				Labels: labels,
			},
		}
	}

	containers := []types.ContainerJSON{
		createContainer("", "id1", "container1", "image1"),
		createContainer("", "id2", "container2", "image1"),
		createContainer("compose1", "id3", "container1", "image1"),
		createContainer("compose1", "id4", "container2", "image2"),
		createContainer("compose2", "id5", "container1", "image2"),
		createContainer("compose2", "id6", "container2", "image2"),
	}

	genTarget := func(composeProject, id, name string) DockerTarget {
		return DockerTarget{
			ComposeProject: composeProject,
			Id:             id,
			Name:           name,
			StartedAt:      validTime,
		}
	}

	tests := []struct {
		name     string
		config   dockerTargetFilterConfig
		expected []DockerTarget
	}{
		{
			name: "match all",
			config: dockerTargetFilterConfig{
				containerFilter:        []*regexp.Regexp{regexp.MustCompile(`.*`)},
				containerExcludeFilter: nil,
				composeProjectFilter:   []*regexp.Regexp{},
				imageFilter:            []*regexp.Regexp{},
			},
			expected: []DockerTarget{
				genTarget("", "id1", "container1"),
				genTarget("", "id2", "container2"),
				genTarget("compose1", "id3", "container1"),
				genTarget("compose1", "id4", "container2"),
				genTarget("compose2", "id5", "container1"),
				genTarget("compose2", "id6", "container2"),
			},
		},
		{
			name: "filter by multiple containerFilter",
			config: dockerTargetFilterConfig{
				containerFilter: []*regexp.Regexp{
					regexp.MustCompile(`container1`),
					regexp.MustCompile(`container2`),
				},
				containerExcludeFilter: nil,
				composeProjectFilter:   []*regexp.Regexp{},
				imageFilter:            []*regexp.Regexp{},
			},
			expected: []DockerTarget{
				genTarget("", "id1", "container1"),
				genTarget("", "id2", "container2"),
				genTarget("compose1", "id3", "container1"),
				genTarget("compose1", "id4", "container2"),
				genTarget("compose2", "id5", "container1"),
				genTarget("compose2", "id6", "container2"),
			},
		},
		{
			name: "filter by containerFilter",
			config: dockerTargetFilterConfig{
				containerFilter: []*regexp.Regexp{
					// Don't match non-compose service name
					regexp.MustCompile(`container1-0`),
					regexp.MustCompile(`not-matched`),
				},
				containerExcludeFilter: nil,
				composeProjectFilter:   []*regexp.Regexp{},
				imageFilter:            []*regexp.Regexp{},
			},
			expected: []DockerTarget{},
		},
		{
			name: "filter by excludeFilter",
			config: dockerTargetFilterConfig{
				containerFilter:        []*regexp.Regexp{},
				containerExcludeFilter: []*regexp.Regexp{regexp.MustCompile(`container1`)},
				composeProjectFilter:   []*regexp.Regexp{},
				imageFilter:            []*regexp.Regexp{},
			},
			expected: []DockerTarget{
				genTarget("", "id2", "container2"),
				genTarget("compose1", "id4", "container2"),
				genTarget("compose2", "id6", "container2"),
			},
		},
		{
			name: "filter by multiple excludeFilter",
			config: dockerTargetFilterConfig{
				containerFilter: []*regexp.Regexp{},
				containerExcludeFilter: []*regexp.Regexp{
					regexp.MustCompile(`not-matched`),
					regexp.MustCompile(`container2`),
				},
				composeProjectFilter: []*regexp.Regexp{},
				imageFilter:          []*regexp.Regexp{},
			},
			expected: []DockerTarget{
				genTarget("", "id1", "container1"),
				genTarget("compose1", "id3", "container1"),
				genTarget("compose2", "id5", "container1"),
			},
		},
		{
			name: "filter by imageFilter",
			config: dockerTargetFilterConfig{
				containerFilter:        []*regexp.Regexp{},
				containerExcludeFilter: []*regexp.Regexp{},
				composeProjectFilter:   []*regexp.Regexp{},
				imageFilter:            []*regexp.Regexp{regexp.MustCompile(`image1`)},
			},
			expected: []DockerTarget{
				genTarget("", "id1", "container1"),
				genTarget("", "id2", "container2"),
				genTarget("compose1", "id3", "container1"),
			},
		},
		{
			name: "filter by composeFilter",
			config: dockerTargetFilterConfig{
				containerFilter:        []*regexp.Regexp{},
				containerExcludeFilter: []*regexp.Regexp{},
				composeProjectFilter:   []*regexp.Regexp{regexp.MustCompile(`compose1`)},
				imageFilter:            []*regexp.Regexp{},
			},
			expected: []DockerTarget{
				genTarget("compose1", "id3", "container1"),
				genTarget("compose1", "id4", "container2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := []DockerTarget{}
			for _, container := range containers {
				filter := newDockerTargetFilter(tt.config, 10)
				filter.visit(container, func(target *DockerTarget) {
					actual = append(actual, *target)
				})
			}

			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v, but actual %v", tt.expected, actual)
			}
		})
	}
}

func TestTargetFilterShouldAdd(t *testing.T) {
	filter := newDockerTargetFilter(dockerTargetFilterConfig{
		// matches all
		containerFilter:        []*regexp.Regexp{regexp.MustCompile(`.*`)},
		containerExcludeFilter: nil,
		composeProjectFilter:   []*regexp.Regexp{},
		imageFilter:            []*regexp.Regexp{},
	}, 10)
	createContainer := func(id, name, startTime string) types.ContainerJSON {
		return types.ContainerJSON{
			Mounts: []types.MountPoint{},
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: id,
				State: &types.ContainerState{
					StartedAt: startTime,
				},
				Name: name,
			},
			Config: &container.Config{
				Labels: map[string]string{},
			},
		}
	}
	genTarget := func(id, name, timestring string, seen bool) DockerTarget {
		t, _ := time.Parse(time.RFC3339, timestring)
		return DockerTarget{
			Id:        id,
			Name:      name,
			StartedAt: t,
		}
	}
	tests := []struct {
		name      string
		forget    bool
		container types.ContainerJSON
		expected  []DockerTarget
	}{
		{
			name:      "running container observed the first time",
			container: createContainer("id1", "c1", "2000-01-01T00:00:00+00:00"),
			expected:  []DockerTarget{genTarget("id1", "c1", "2000-01-01T00:00:00+00:00", false)},
		},
		{
			name:      "same container ID with same start time should be ignored",
			container: createContainer("id1", "c1", "2000-01-01T00:00:00+00:00"),
			expected:  []DockerTarget{},
		},
		{
			name:      "same container ID with new start time should be added",
			container: createContainer("id1", "c1", "2000-01-01T00:00:01+00:00"),
			expected:  []DockerTarget{genTarget("id1", "c1", "2000-01-01T00:00:01+00:00", true)},
		},
		{
			name:      "different container ID can be added",
			container: createContainer("id2", "c2", "2000-01-01T00:00:00+00:00"),
			expected:  []DockerTarget{genTarget("id2", "c2", "2000-01-01T00:00:00+00:00", false)},
		},
		{
			name:      "inactive() allows the same ID ",
			forget:    true,
			container: createContainer("id2", "c2", "2000-01-01T00:00:00+00:00"),
			expected:  []DockerTarget{genTarget("id2", "c2", "2000-01-01T00:00:00+00:00", true)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.forget {
				filter.inactive("id2")
			}
			actual := []DockerTarget{}
			filter.visit(tt.container, func(target *DockerTarget) {
				actual = append(actual, *target)
			})
			if !reflect.DeepEqual(tt.expected, actual) {
				t.Errorf("expected %v, but actual %v", tt.expected, actual)
			}
		})
	}
}
