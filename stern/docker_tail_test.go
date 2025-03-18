package stern

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestDetermineColor(t *testing.T) {
	composeColor1, containerColor1 := determineDockerColor("cont1", "comp1")
	composeColor2, containerColor2 := determineDockerColor("cont2", "comp2")

	if composeColor1 == composeColor2 {
		t.Errorf("expected color for compose to be the different between invocations but was %v and %v",
			composeColor1, composeColor2)
	}
	if containerColor1 == containerColor2 {
		t.Errorf("expected color for container to be the different between invocations but was %v and %v",
			containerColor1, containerColor2)
	}
}

func TestDetermineColorNoCompose(t *testing.T) {
	containerName := "cont1"
	composeColor1, containerColor1 := determineDockerColor(containerName, "")
	composeColor2, containerColor2 := determineDockerColor(containerName, "")

	if composeColor1 != composeColor2 {
		t.Errorf("expected color for compose to be the same between invocations but was %v and %v",
			composeColor1, composeColor2)
	}
	if containerColor1 != containerColor2 {
		t.Errorf("expected color for container to be same between invocations but was the same: %v",
			containerColor1)
	}
}

func TestPrintStarting(t *testing.T) {
	tests := []struct {
		useCompose bool
		options    *TailOptions
		expected   []byte
	}{
		{
			true,
			&TailOptions{},
			[]byte("+ compose › name\n"),
		},
		{
			false,
			&TailOptions{},
			[]byte("+ name\n"),
		},
		{
			true,
			&TailOptions{
				OnlyLogLines: true,
			},
			[]byte{},
		},
	}

	for i, tt := range tests {
		errOut := new(bytes.Buffer)
		compose := ""
		if tt.useCompose {
			compose = "compose"
		}
		tail := NewDockerTail(
			nil,
			"id",
			"name",
			compose,
			false,
			time.Now(),
			"",
			false,
			nil,
			io.Discard,
			errOut,
			tt.options,
		)
		tail.printStarting()

		if !bytes.Equal(tt.expected, errOut.Bytes()) {
			t.Errorf("%d: expected %q, but actual %q", i, tt.expected, errOut)
		}
	}
}

func TestPrintStopping(t *testing.T) {
	tests := []struct {
		useCompose bool
		options    *TailOptions
		expected   []byte
	}{
		{
			true,
			&TailOptions{},
			[]byte("- compose › name\n"),
		},
		{
			false,
			&TailOptions{},
			[]byte("- name\n"),
		},
		{
			true,
			&TailOptions{
				OnlyLogLines: true,
			},
			[]byte{},
		},
	}

	for i, tt := range tests {
		errOut := new(bytes.Buffer)
		compose := ""
		if tt.useCompose {
			compose = "compose"
		}
		tail := NewDockerTail(
			nil,
			"id",
			"name",
			compose,
			false,
			time.Now(),
			"",
			false,
			nil,
			io.Discard,
			errOut,
			tt.options,
		)
		tail.printStopping()

		if !bytes.Equal(tt.expected, errOut.Bytes()) {
			t.Errorf("%d: expected %q, but actual %q", i, tt.expected, errOut)
		}
	}
}

func TestGetSinceTime(t *testing.T) {
	t1, _ := time.Parse(time.RFC3339, "2025-01-07T21:02:08Z")
	t2 := t1.Add(5 * time.Second)
	t3 := t1.Add(10 * time.Second)
	tests := []struct {
		desc        string
		startTime   time.Time
		finishTime  string
		optionsTime time.Time
		seen        bool
		expected    time.Time
	}{
		{
			desc:        "invalid finish, seen",
			startTime:   t1,
			finishTime:  "",
			optionsTime: t2,
			seen:        true,
			expected:    t2,
		},
		{
			desc:        "finish earlier than option, seen",
			startTime:   t3,
			finishTime:  t1.Format(time.RFC3339),
			optionsTime: t2,
			seen:        true,
			expected:    t1,
		},
		{
			desc:        "start earlier than option, seen",
			startTime:   t1,
			finishTime:  t3.Format(time.RFC3339),
			optionsTime: t2,
			seen:        true,
			expected:    t2,
		},
		{
			desc:        "option later than both, seen",
			startTime:   t2,
			finishTime:  t1.Format(time.RFC3339),
			optionsTime: t3,
			seen:        true,
			expected:    t3,
		},
		{
			desc:        "truncated to start, seen",
			startTime:   t2,
			finishTime:  t3.Format(time.RFC3339),
			optionsTime: t1,
			seen:        true,
			expected:    t2,
		},
		{
			desc:        "truncated to finish, seen",
			startTime:   t3,
			finishTime:  t2.Format(time.RFC3339),
			optionsTime: t1,
			seen:        true,
			expected:    t2,
		},
		{
			desc:        "start earlier than option, not seen",
			startTime:   t1,
			finishTime:  t2.Format(time.RFC3339),
			optionsTime: t3,
			seen:        false,
			expected:    t3,
		},
		{
			desc:        "start later than option, not seen",
			startTime:   t2,
			finishTime:  t3.Format(time.RFC3339),
			optionsTime: t1,
			seen:        false,
			expected:    t1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			options := &TailOptions{
				DockerSinceTime: tt.optionsTime,
			}
			tail := NewDockerTail(nil, "", "", "", false, tt.startTime, tt.finishTime, tt.seen, nil, io.Discard, io.Discard, options)

			actual := tail.getSinceTime()
			if tt.expected != actual {
				t.Errorf("expected %v, but actual %v", tt.expected, actual)
			}
		})
	}
}
