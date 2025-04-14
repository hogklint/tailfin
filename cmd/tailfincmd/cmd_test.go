package tailfincmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/hogklint/tailfin/stern"
	"github.com/spf13/pflag"
)

func TestTailfinCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		out  string
	}{
		{
			"Output version info with --version",
			[]string{"--version"},
			"version: dev",
		},
		{
			"Output completion code for bash with --completion=bash",
			[]string{"--completion=bash"},
			"complete -o default -F __start_tailfin tailfin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var errout bytes.Buffer
			streams := IOStreams{Out: &out, ErrOut: &errout}
			stern, err := NewTailfinCmd(streams)
			if err != nil {
				t.Fatal(err)
			}
			stern.SetArgs(tt.args)

			if err := stern.Execute(); err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(out.String(), tt.out) {
				t.Errorf("expected to contain %s, but actual %s", tt.out, out.String())
			}
		})
	}
}

func TestOptionsComplete(t *testing.T) {
	var out bytes.Buffer
	var errout bytes.Buffer
	streams := IOStreams{Out: &out, ErrOut: &errout}

	tests := []struct {
		name                   string
		env                    map[string]string
		args                   []string
		expectedConfigFilePath string
	}{
		{
			name:                   "No environment variables",
			env:                    map[string]string{},
			args:                   []string{},
			expectedConfigFilePath: defaultConfigFilePath,
		},
		{
			name: "Set STERNCONFIG env to ./config.yaml",
			env: map[string]string{
				"TAILFINCONFIG": "./config.yaml",
			},
			args:                   []string{},
			expectedConfigFilePath: "./config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			o := NewOptions(streams)
			_ = o.Complete(tt.args)

			if tt.expectedConfigFilePath != o.configFilePath {
				t.Errorf("expected %s for configFilePath, but got %s", tt.expectedConfigFilePath, o.configFilePath)
			}
		})
	}
}

func TestOptionsValidate(t *testing.T) {
	var out bytes.Buffer
	var errout bytes.Buffer
	streams := IOStreams{Out: &out, ErrOut: &errout}

	tests := []struct {
		name string
		o    *options
		err  string
	}{
		{
			"No required options",
			NewOptions(streams),
			"One of container-query, --label, --image, or --stdin is required",
		},
		{
			"Specify container-query",
			func() *options {
				o := NewOptions(streams)
				o.containerQuery = []string{"."}

				return o
			}(),
			"",
		},
		{
			"Specify stdin",
			func() *options {
				o := NewOptions(streams)
				o.stdin = true

				return o
			}(),
			"",
		},
		{
			"Specify image",
			func() *options {
				o := NewOptions(streams)
				o.image = []string{"nginx"}

				return o
			}(),
			"",
		},
		{
			"Specify label",
			func() *options {
				o := NewOptions(streams)
				o.label = []string{"app"}

				return o
			}(),
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.o.Validate()
			if err == nil {
				if tt.err != "" {
					t.Errorf("expected %q err, but actual no err", tt.err)
				}
			} else {
				if tt.err != err.Error() {
					t.Errorf("expected %q err, but actual %q", tt.err, err)
				}
			}
		})
	}
}

func TestOptionsGenerateTemplate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	var errout bytes.Buffer
	streams := IOStreams{Out: &out, ErrOut: &errout}

	tests := []struct {
		name        string
		o           *options
		message     string
		want        string
		wantError   bool
		withCompose bool
	}{
		{
			"output=default",
			func() *options {
				o := NewOptions(streams)
				o.output = "default"

				return o
			}(),
			"default message",
			"container1 default message\n",
			false,
			false,
		},
		{
			"output=default with compose",
			func() *options {
				o := NewOptions(streams)
				o.output = "default"

				return o
			}(),
			"default message",
			"compose1 service1 default message\n",
			false,
			true,
		},
		{
			"output=raw",
			func() *options {
				o := NewOptions(streams)
				o.output = "raw"

				return o
			}(),
			"raw message",
			"raw message\n",
			false,
			false,
		},
		{
			"output=json",
			func() *options {
				o := NewOptions(streams)
				o.output = "json"

				return o
			}(),
			"json message",
			`{"message":"json message","container":"container1","service":"service1","namespace":"compose1","number":"0"}
`,
			false,
			true,
		},
		{
			"output=extjson",
			func() *options {
				o := NewOptions(streams)
				o.output = "extjson"

				return o
			}(),
			`{"msg":"extjson message"}`,
			`{"namespace": "compose1", "service": "service1", "message": {"msg":"extjson message"}}
`,
			false,
			true,
		},
		{
			"output=ppextjson",
			func() *options {
				o := NewOptions(streams)
				o.output = "ppextjson"

				return o
			}(),
			`{"msg":"ppextjson message"}`,
			`{
  "namespace": "compose1",
  "service": "service1",
  "message": {"msg":"ppextjson message"}
}
`,
			false,
			true,
		},
		{
			"invalid output",
			func() *options {
				o := NewOptions(streams)
				o.output = "invalid"

				return o
			}(),
			"message",
			"",
			true,
			false,
		},
		{
			"template",
			func() *options {
				o := NewOptions(streams)
				o.template = "Message={{.Message}} Namespace={{.Namespace}} ContainerName={{.ContainerName}}"

				return o
			}(),
			"template message", // no new line
			"Message=template message Namespace=compose1 ContainerName=container1",
			false,
			true,
		},
		{
			"invalid template",
			func() *options {
				o := NewOptions(streams)
				o.template = "{{invalid"

				return o
			}(),
			"template message",
			"",
			true,
			false,
		},
		{
			"template-file",
			func() *options {
				o := NewOptions(streams)
				o.templateFile = "test.tpl"

				return o
			}(),
			"template message",
			"compose1 container1 template message\n",
			false,
			true,
		},
		{
			"template-file-json-log-ts-float",
			func() *options {
				o := NewOptions(streams)
				o.templateFile = "test.tpl"

				return o
			}(),
			`{"ts": 123, "level": "INFO", "msg": "template message"}`,
			"compose1 container1 [1970-01-01T00:02:03Z] INFO template message\n",
			false,
			true,
		},
		{
			"template-file-json-log-ts-str",
			func() *options {
				o := NewOptions(streams)
				o.templateFile = "test.tpl"

				return o
			}(),
			`{"ts": "1970-01-01T01:02:03+01:00", "level": "INFO", "msg": "template message"}`,
			"compose1 container1 [1970-01-01T00:02:03Z] INFO template message\n",
			false,
			true,
		},
		{
			"template-to-timestamp-with-timezone",
			func() *options {
				o := NewOptions(streams)
				o.template = `{{ toTimestamp .Message "Jan 02 2006 15:04 MST" "US/Eastern" }}`
				return o
			}(),
			`2024-01-01T05:00:00`,
			`Jan 01 2024 00:00 EST`,
			false,
			false,
		},
		{
			"template-to-timestamp-without-timezone",
			func() *options {
				o := NewOptions(streams)
				o.template = `{{ toTimestamp .Message "Jan 02 2006 15:04 MST" }}`
				return o
			}(),
			`2024-01-01T05:00:00`,
			`Jan 01 2024 05:00 UTC`,
			false,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := stern.Log{
				Message:        tt.message,
				ContainerName:  "container1",
				ServiceName:    "container1",
				ContainerColor: color.New(color.FgBlue),
			}
			if tt.withCompose {
				log.Namespace = "compose1"
				log.ServiceName = "service1"
				log.ContainerNumber = "0"
				log.NamespaceColor = color.New(color.FgBlue)
			}
			tmpl, err := tt.o.generateTemplate()

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, but got no error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, log); err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if want, got := tt.want, buf.String(); want != got {
				t.Errorf("want %v, but got %v", want, got)
			}
		})
	}
}

func TestOptionsTailfinConfig(t *testing.T) {
	var out bytes.Buffer
	var errout bytes.Buffer
	streams := IOStreams{Out: &out, ErrOut: &errout}

	local, _ := time.LoadLocation("Local")
	utc, _ := time.LoadLocation("UTC")

	re := regexp.MustCompile

	defaultConfig := func() *stern.DockerConfig {
		return &stern.DockerConfig{
			Timestamps:            false,
			TimestampFormat:       "",
			Location:              local,
			Label:                 []string(nil),
			ContainerQuery:        []*regexp.Regexp(nil),
			ExcludeContainerQuery: nil,
			ComposeProjectQuery:   nil,
			Exclude:               nil,
			ImageQuery:            []*regexp.Regexp(nil),
			Include:               nil,
			Highlight:             nil,
			Since:                 48 * time.Hour,
			TailLines:             -1,
			Template:              nil, // ignore when comparing
			Follow:                true,
			OnlyLogLines:          false,
			MaxLogRequests:        50,
			Stdin:                 false,

			Out:    streams.Out,
			ErrOut: streams.ErrOut,
		}
	}

	tests := []struct {
		name      string
		o         *options
		want      *stern.DockerConfig
		wantError bool
	}{
		{
			"default",
			NewOptions(streams),
			defaultConfig(),
			false,
		},
		{
			"change all options",
			func() *options {
				o := NewOptions(streams)
				o.timestamps = "default"
				o.timezone = "UTC" // Location
				o.containerQuery = []string{"container1"}
				o.excludeContainer = []string{"exc1", "exc2"}
				o.compose = []string{"compose1"}
				o.exclude = []string{"ex1", "ex2"}
				o.image = []string{"nginx"}
				o.include = []string{"in1", "in2"}
				o.highlight = []string{"hi1", "hi2"}
				o.since = 1 * time.Hour
				o.label = []string{"app"}
				o.tail = 10
				o.noFollow = true // Follow = false
				o.maxLogRequests = 30
				o.onlyLogLines = true

				return o
			}(),
			func() *stern.DockerConfig {
				c := defaultConfig()
				c.Timestamps = true
				c.TimestampFormat = stern.TimestampFormatDefault
				c.Location = utc
				c.ContainerQuery = []*regexp.Regexp{re("container1")}
				c.ExcludeContainerQuery = []*regexp.Regexp{re("exc1"), re("exc2")}
				c.ComposeProjectQuery = []*regexp.Regexp{re("compose1")}
				c.Exclude = []*regexp.Regexp{re("ex1"), re("ex2")}
				c.ImageQuery = []*regexp.Regexp{re("nginx")}
				c.Include = []*regexp.Regexp{re("in1"), re("in2")}
				c.Highlight = []*regexp.Regexp{re("hi1"), re("hi2")}
				c.Since = 1 * time.Hour
				c.Label = []string{"app"}
				c.TailLines = 10
				c.Follow = false
				c.OnlyLogLines = true
				c.MaxLogRequests = 30

				return c
			}(),
			false,
		},
		{
			"timestamp=short",
			func() *options {
				o := NewOptions(streams)
				o.timestamps = "short"

				return o
			}(),
			func() *stern.DockerConfig {
				c := defaultConfig()
				c.Timestamps = true
				c.TimestampFormat = stern.TimestampFormatShort

				return c
			}(),
			false,
		},
		{
			"noFollow has the different default",
			func() *options {
				o := NewOptions(streams)
				o.noFollow = true // Follow = false

				return o
			}(),
			func() *stern.DockerConfig {
				c := defaultConfig()
				c.Follow = false
				c.MaxLogRequests = 5 // default of noFollow

				return c
			}(),
			false,
		},
		{
			"nil should be allowed",
			func() *options {
				o := NewOptions(streams)
				o.excludeContainer = nil
				o.exclude = nil
				o.include = nil
				o.highlight = nil

				return o
			}(),
			func() *stern.DockerConfig {
				c := defaultConfig()

				return c
			}(),
			false,
		},
		{
			"error container-query",
			func() *options {
				o := NewOptions(streams)
				o.containerQuery = []string{"[invalid"}

				return o
			}(),
			nil,
			true,
		},
		{
			"error excludeContainer",
			func() *options {
				o := NewOptions(streams)
				o.excludeContainer = []string{"exc1", "[invalid"}

				return o
			}(),
			nil,
			true,
		},
		{
			"error exclude",
			func() *options {
				o := NewOptions(streams)
				o.exclude = []string{"ex1", "[invalid"}

				return o
			}(),
			nil,
			true,
		},
		{
			"error include",
			func() *options {
				o := NewOptions(streams)
				o.include = []string{"in1", "[invalid"}

				return o
			}(),
			nil,
			true,
		},
		{
			"error highlight",
			func() *options {
				o := NewOptions(streams)
				o.highlight = []string{"hi1", "[invalid"}

				return o
			}(),
			nil,
			true,
		},
		{
			"error color",
			func() *options {
				o := NewOptions(streams)
				o.color = "invalid"

				return o
			}(),
			nil,
			true,
		},
		{
			"error output",
			func() *options {
				o := NewOptions(streams)
				o.output = "invalid"

				return o
			}(),
			nil,
			true,
		},
		{
			"error timezone",
			func() *options {
				o := NewOptions(streams)
				o.timezone = "invalid"

				return o
			}(),
			nil,
			true,
		},
		{
			"error timestamps",
			func() *options {
				o := NewOptions(streams)
				o.timestamps = "invalid"

				return o
			}(),
			nil,
			true,
		},
	}
	// TODO test label

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.o.tailfinConfig()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, but got no error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// We skip the template as it is difficult to check
			// and is tested in TestOptionsGenerateTemplate().
			got.Template = nil

			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("want %+v, but got %+v", tt.want, got)
			}
		})
	}
}

func TestOptionsOverrideFlagSetDefaultFromConfig(t *testing.T) {
	orig := defaultConfigFilePath
	defer func() {
		defaultConfigFilePath = orig
	}()

	defaultConfigFilePath = "./config.yaml"
	wd, _ := os.Getwd()

	tests := []struct {
		name                    string
		flagConfigFilePathValue string
		flagTailValue           string
		expectedTailValue       int64
		wantErr                 bool
	}{
		{
			name:                    "--config=testdata/config-tail1.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-tail1.yaml"),
			expectedTailValue:       1,
			wantErr:                 false,
		},
		{
			name:                    "--config=testdata/config-empty.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-empty.yaml"),
			expectedTailValue:       -1,
			wantErr:                 false,
		},
		{
			name:                    "--config=config-not-exist.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "config-not-exist.yaml"),
			wantErr:                 true,
		},
		{
			name:                    "--config=config-invalid.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-invalid.yaml"),
			wantErr:                 true,
		},
		{
			name:                    "--config=config-unknown-option.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-unknown-option.yaml"),
			expectedTailValue:       1,
			wantErr:                 false,
		},
		{
			name:                    "--config=config-tail-invalid-value.yaml",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-tail-invalid-value.yaml"),
			wantErr:                 true,
		},
		{
			name:              "config file path is not specified and config file does not exist",
			expectedTailValue: -1,
			wantErr:           false,
		},
		{
			name:                    "--config=testdata/config-tail1.yaml and --tail=2",
			flagConfigFilePathValue: filepath.Join(wd, "testdata/config-tail1.yaml"),
			flagTailValue:           "2",
			expectedTailValue:       2,
			wantErr:                 false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOptions(IOStreams{Out: io.Discard, ErrOut: io.Discard})
			fs := pflag.NewFlagSet("", pflag.ExitOnError)
			o.AddFlags(fs)

			args := []string{}
			if tt.flagConfigFilePathValue != "" {
				args = append(args, "--config="+tt.flagConfigFilePathValue)
			}
			if tt.flagTailValue != "" {
				args = append(args, "--tail="+tt.flagTailValue)
			}

			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}

			err := o.overrideFlagSetDefaultFromConfig(fs)
			if tt.wantErr {
				if err == nil {
					t.Error("expected err, but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			if tt.expectedTailValue != o.tail {
				t.Errorf("expected %d for tail, but got %d", tt.expectedTailValue, o.tail)
			}
		})
	}
}

func TestOptionsOverrideFlagSetDefaultFromConfigArray(t *testing.T) {
	tests := []struct {
		config string
		want   []string
	}{
		{
			config: "testdata/config-string.yaml",
			want:   []string{"hello-world"},
		},
		{
			config: "testdata/config-array0.yaml",
			want:   []string{},
		},
		{
			config: "testdata/config-array1.yaml",
			want:   []string{"abcd"},
		},
		{
			config: "testdata/config-array2.yaml",
			want:   []string{"abcd", "efgh"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.config, func(t *testing.T) {
			o := NewOptions(IOStreams{Out: io.Discard, ErrOut: io.Discard})
			fs := pflag.NewFlagSet("", pflag.ExitOnError)
			o.AddFlags(fs)
			if err := fs.Parse([]string{"--config=" + tt.config}); err != nil {
				t.Fatal(err)
			}
			if err := o.overrideFlagSetDefaultFromConfig(fs); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tt.want, o.exclude) {
				t.Errorf("expected %v, but got %v", tt.want, o.exclude)
			}
		})
	}

}
