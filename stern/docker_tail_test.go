package stern

import (
	"bytes"
	"io"
	"testing"
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
