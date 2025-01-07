package stern

import (
	"io"
	"testing"
	"time"
)

func TestGetSinceTime(t *testing.T) {
	t1, _ := time.Parse(time.RFC3339, "2025-01-07T21:02:08Z")
	t2 := t1.Add(5 * time.Second)
	t3 := t1.Add(10 * time.Second)
	tests := []struct {
		desc        string
		startTime   time.Time
		finishTime  string
		optionsTime time.Time
		expected    time.Time
	}{
		{
			desc:        "invalid finish, earlier start",
			startTime:   t1,
			finishTime:  "",
			optionsTime: t2,
			expected:    t2,
		},
		{
			desc:        "invalid finish, later start",
			startTime:   t2,
			finishTime:  "",
			optionsTime: t1,
			expected:    t1,
		},
		{
			desc:        "finish earlier than option",
			startTime:   t3,
			finishTime:  t1.Format(time.RFC3339),
			optionsTime: t2,
			expected:    t2,
		},
		{
			desc:        "start earlier than option",
			startTime:   t1,
			finishTime:  t3.Format(time.RFC3339),
			optionsTime: t2,
			expected:    t2,
		},
		{
			desc:        "option later than both 1",
			startTime:   t2,
			finishTime:  t1.Format(time.RFC3339),
			optionsTime: t3,
			expected:    t3,
		},
		{
			desc:        "option later than both 2",
			startTime:   t1,
			finishTime:  t2.Format(time.RFC3339),
			optionsTime: t3,
			expected:    t3,
		},
		{
			desc:        "truncated to start",
			startTime:   t2,
			finishTime:  t3.Format(time.RFC3339),
			optionsTime: t1,
			expected:    t2,
		},
		{
			desc:        "truncated to finish",
			startTime:   t3,
			finishTime:  t2.Format(time.RFC3339),
			optionsTime: t1,
			expected:    t2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			options := &TailOptions{
				DockerSinceTime: tt.optionsTime,
			}
			tail := NewDockerTail(nil, "", "", "", false, tt.startTime, tt.finishTime, nil, io.Discard, io.Discard, options)

			actual := tail.getSinceTime()
			if tt.expected != actual {
				t.Errorf("expected %v, but actual %v", tt.expected, actual)
			}
		})
	}
}
