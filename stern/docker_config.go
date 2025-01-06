package stern

import (
	"io"
	"regexp"
	"text/template"
	"time"
)

// Config contains the config for tailfin
type DockerConfig struct {
	Timestamps            bool
	TimestampFormat       string
	Location              *time.Location
	ContainerQuery        []*regexp.Regexp
	ExcludeContainerQuery []*regexp.Regexp
	Exclude               []*regexp.Regexp
	ImageQuery            []*regexp.Regexp
	Include               []*regexp.Regexp
	Highlight             []*regexp.Regexp
	Since                 time.Duration
	TailLines             *int64
	Template              *template.Template
	Follow                bool
	OnlyLogLines          bool
	MaxLogRequests        int
	Stdin                 bool

	Out    io.Writer
	ErrOut io.Writer
}
