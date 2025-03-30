package stern

import (
	"bytes"
	"context"
	"io"
	"testing"
	"text/template"
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

func TestConsumeStreamTail(t *testing.T) {
	logLines := []byte(`2023-02-13T21:20:30.000000001Z line 1
2023-02-13T21:20:30.000000002Z line 2
2023-02-13T21:20:31.000000001Z line 3
2023-02-13T21:20:31.000000002Z line 4`)
	tmpl := template.Must(template.New("").Parse(`{{printf "%s (%s)\n" .Message .ContainerName}}`))

	tests := []struct {
		name      string
		resumeReq *ResumeRequest
		expected  []byte
	}{
		{
			name: "normal",
			expected: []byte(`line 1 (container1)
line 2 (container1)
line 3 (container1)
line 4 (container1)
`),
		},
		{
			name:      "ResumeRequest LinesToSkip=1",
			resumeReq: &ResumeRequest{Timestamp: "2023-02-13T21:20:30Z", LinesToSkip: 1},
			expected: []byte(`line 2 (container1)
line 3 (container1)
line 4 (container1)
`),
		},
		{
			name:      "ResumeRequest LinesToSkip=2",
			resumeReq: &ResumeRequest{Timestamp: "2023-02-13T21:20:30Z", LinesToSkip: 2},
			expected: []byte(`line 3 (container1)
line 4 (container1)
`),
		},
		{
			name:      "ResumeRequest LinesToSkip=3 (exceed)",
			resumeReq: &ResumeRequest{Timestamp: "2023-02-13T21:20:30Z", LinesToSkip: 3},
			expected: []byte(`line 3 (container1)
line 4 (container1)
`),
		},
		{
			name:      "ResumeRequest does not match",
			resumeReq: &ResumeRequest{Timestamp: "2222-22-22T21:20:30Z", LinesToSkip: 3},
			expected: []byte(`line 1 (container1)
line 2 (container1)
line 3 (container1)
line 4 (container1)
`),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			tail := NewDockerTail(
				nil,
				"id",
				"container1",
				"",
				true,
				tmpl,
				out,
				out,
				&TailOptions{},
			)
			tail.resumeRequest = tt.resumeReq
			if err := tail.consumeStream(context.TODO(), bytes.NewReader(logLines)); err != nil {
				t.Fatalf("%d: unexpected err %v", i, err)
			}

			if !bytes.Equal(tt.expected, out.Bytes()) {
				t.Errorf("%d: expected %s, but actual %s", i, tt.expected, out)
			}
		})
	}
}
