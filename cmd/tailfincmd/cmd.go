//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package tailfincmd

import (
	"context"
	"encoding/json"
	goflag "flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/hogklint/tailfin/stern"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"io"
	"k8s.io/klog/v2"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	// load all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	dockerclient "github.com/docker/docker/client"
)

// Use "~" to avoid exposing the user name in the help message
var defaultConfigFilePath = "~/.config/tailfin/config.yaml"

type IOStreams struct {
	Out    io.Writer
	ErrOut io.Writer
}

type options struct {
	IOStreams

	excludeContainer []string
	timestamps       string
	timezone         string
	since            time.Duration
	exclude          []string
	include          []string
	highlight        []string
	tail             int64
	color            string
	version          bool
	completion       string
	template         string
	templateFile     string
	output           string
	containerQuery   string
	noFollow         bool
	verbosity        int
	onlyLogLines     bool
	maxLogRequests   int
	configFilePath   string
	stdin            bool
	containerColors  []string
	//containerStates     []string
	//selector            string
	//compose            []string

	dockerClient *dockerclient.Client
}

func NewOptions(streams IOStreams) *options {
	return &options{
		IOStreams: streams,

		color: "auto",
		//containerStates:     []string{stern.ALL_STATES},
		output:         "default",
		since:          48 * time.Hour,
		tail:           -1,
		template:       "",
		templateFile:   "",
		timestamps:     "",
		timezone:       "Local",
		noFollow:       false,
		maxLogRequests: -1,
		configFilePath: defaultConfigFilePath,
	}
}

func (o *options) Complete(args []string) error {
	if len(args) > 0 {
		o.containerQuery = args[0]
	}

	envVar, ok := os.LookupEnv("TAILFINCONFIG")
	if ok {
		o.configFilePath = envVar
	}

	var err error
	o.dockerClient, err = dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	return nil
}

func (o *options) Validate() error {
	if o.containerQuery == "" && !o.stdin {
		// TODO: Add image matching
		return errors.New("One of container-query or --stdin is required")
	}

	return nil
}

func (o *options) Run(cmd *cobra.Command) error {
	if err := o.setVerbosity(); err != nil {
		return err
	}
	if err := o.setColorList(); err != nil {
		return err
	}

	config, err := o.tailfinConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return stern.RunDocker(ctx, o.dockerClient, config)
}

func (o *options) tailfinConfig() (*stern.DockerConfig, error) {
	container, err := regexp.Compile(o.containerQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression from query")
	}

	excludeContainer, err := compileREs(o.excludeContainer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for excluded container query")
	}

	exclude, err := compileREs(o.exclude)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for exclusion filter")
	}

	include, err := compileREs(o.include)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for inclusion filter")
	}

	highlight, err := compileREs(o.highlight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for highlight filter")
	}

	var tailLines *int64
	if o.tail != -1 {
		tailLines = &o.tail
	}

	switch o.color {
	case "always":
		color.NoColor = false
	case "never":
		color.NoColor = true
	case "auto":
	default:
		return nil, errors.New("color should be one of 'always', 'never', or 'auto'")
	}

	template, err := o.generateTemplate()
	if err != nil {
		return nil, err
	}

	var timestampFormat string
	switch o.timestamps {
	case "default":
		timestampFormat = stern.TimestampFormatDefault
	case "short":
		timestampFormat = stern.TimestampFormatShort
	case "":
	default:
		return nil, errors.New("timestamps should be one of 'default', or 'short'")
	}

	// --timezone
	location, err := time.LoadLocation(o.timezone)
	if err != nil {
		return nil, err
	}

	maxLogRequests := o.maxLogRequests
	if maxLogRequests == -1 {
		if o.noFollow {
			maxLogRequests = 5
		} else {
			maxLogRequests = 50
		}
	}

	return &stern.DockerConfig{
		ContainerQuery:        container,
		Timestamps:            timestampFormat != "",
		TimestampFormat:       timestampFormat,
		Location:              location,
		ExcludeContainerQuery: excludeContainer,
		Exclude:               exclude,
		Include:               include,
		Highlight:             highlight,
		Since:                 o.since,
		TailLines:             tailLines,
		Template:              template,
		Follow:                !o.noFollow,
		OnlyLogLines:          o.onlyLogLines,
		MaxLogRequests:        maxLogRequests,
		Stdin:                 o.stdin,

		Out:    o.Out,
		ErrOut: o.ErrOut,
	}, nil
}

// setVerbosity sets the log level verbosity
func (o *options) setVerbosity() error {
	if o.verbosity != 0 {
		// klog does not have an external method to set verbosity,
		// so we need to set it by a flag.
		// See https://github.com/kubernetes/klog/issues/336 for details
		var fs goflag.FlagSet
		klog.InitFlags(&fs)
		return fs.Set("v", strconv.Itoa(o.verbosity))
	}
	return nil
}

func (o *options) setColorList() error {
	if len(o.containerColors) > 0 {
		return stern.SetColorList([]string{}, o.containerColors)
	}
	return nil
}

// overrideFlagSetDefaultFromConfig overrides the default value of the flagSets
// from the config file
func (o *options) overrideFlagSetDefaultFromConfig(fs *pflag.FlagSet) error {
	expanded, err := homedir.Expand(o.configFilePath)
	if err != nil {
		return err
	}

	if o.configFilePath == defaultConfigFilePath {
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			return nil
		}
	}

	configFile, err := os.Open(expanded)
	if err != nil {
		return err
	}

	data := make(map[string]interface{})

	if err := yaml.NewDecoder(configFile).Decode(data); err != nil && err != io.EOF {
		return err
	}

	for name, value := range data {
		flag := fs.Lookup(name)
		if flag == nil {
			// To avoid command execution failure, we only output a warning
			// message instead of exiting with an error if an unknown option is
			// specified.
			klog.Warningf("Unknown option specified in the config file: %s", name)
			continue
		}

		// flag has higher priority than the config file
		if flag.Changed {
			continue
		}

		if valueSlice, ok := value.([]any); ok {
			// the value is an array
			if flagSlice, ok := flag.Value.(pflag.SliceValue); ok {
				values := make([]string, len(valueSlice))
				for i, v := range valueSlice {
					values[i] = fmt.Sprint(v)
				}
				if err := flagSlice.Replace(values); err != nil {
					return fmt.Errorf("invalid value %q for %q in the config file: %v", value, name, err)
				}
				continue
			}
		}

		if err := flag.Value.Set(fmt.Sprint(value)); err != nil {
			return fmt.Errorf("invalid value %q for %q in the config file: %v", value, name, err)
		}
	}

	return nil
}

// AddFlags adds all the flags used by tailfin.
func (o *options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.color, "color", o.color, "Force set color output. 'auto':  colorize if tty attached, 'always': always colorize, 'never': never colorize.")
	fs.StringVar(&o.completion, "completion", o.completion, "Output stern command-line completion code for the specified shell. Can be 'bash', 'zsh' or 'fish'.")
	fs.StringArrayVarP(&o.exclude, "exclude", "e", o.exclude, "Log lines to exclude. (regular expression)")
	fs.StringArrayVarP(&o.excludeContainer, "exclude-container", "E", o.excludeContainer, "Container name to exclude. (regular expression)")
	fs.BoolVar(&o.noFollow, "no-follow", o.noFollow, "Exit when all logs have been shown.")
	fs.StringArrayVarP(&o.include, "include", "i", o.include, "Log lines to include. (regular expression)")
	fs.StringArrayVarP(&o.highlight, "highlight", "H", o.highlight, "Log lines to highlight. (regular expression)")
	fs.IntVar(&o.maxLogRequests, "max-log-requests", o.maxLogRequests, "Maximum number of concurrent logs to request. Defaults to 50, but 5 when specifying --no-follow")
	fs.StringVarP(&o.output, "output", "o", o.output, "Specify predefined template. Currently support: [default, raw, json, extjson, ppextjson]")
	fs.DurationVarP(&o.since, "since", "s", o.since, "Return logs newer than a relative duration like 5s, 2m, or 3h.")
	fs.Int64Var(&o.tail, "tail", o.tail, "The number of lines from the end of the logs to show. Defaults to -1, showing all logs.")
	fs.StringVar(&o.template, "template", o.template, "Template to use for log lines, leave empty to use --output flag.")
	fs.StringVarP(&o.templateFile, "template-file", "T", o.templateFile, "Path to template to use for log lines, leave empty to use --output flag. It overrides --template option.")
	fs.StringVarP(&o.timestamps, "timestamps", "t", o.timestamps, "Print timestamps with the specified format. One of 'default' or 'short' in the form '--timestamps=format' ('=' cannot be omitted). If specified but without value, 'default' is used.")
	fs.StringVar(&o.timezone, "timezone", o.timezone, "Set timestamps to specific timezone.")
	fs.BoolVar(&o.onlyLogLines, "only-log-lines", o.onlyLogLines, "Print only log lines")
	fs.StringVar(&o.configFilePath, "config", o.configFilePath, "Path to the tailfin config file")
	fs.IntVar(&o.verbosity, "verbosity", o.verbosity, "Number of the log level verbosity")
	fs.BoolVarP(&o.version, "version", "v", o.version, "Print the version and exit.")
	fs.BoolVar(&o.stdin, "stdin", o.stdin, "Parse logs from stdin. All Kubernetes related flags are ignored when it is set.")
	fs.StringSliceVar(&o.containerColors, "container-colors", o.containerColors, "Specifies the colors used to highlight container names. Use the same format as --pod-colors. Defaults to the values of --pod-colors if omitted, and must match its length.")

	fs.Lookup("timestamps").NoOptDefVal = "default"
}

func (o *options) generateTemplate() (*template.Template, error) {
	t := o.template
	if o.templateFile != "" {
		data, err := os.ReadFile(o.templateFile)
		if err != nil {
			return nil, err
		}
		t = string(data)
	}
	if t == "" {
		switch o.output {
		case "default":
			t = "{{if .ComposeProject}}{{color .ComposeColor .ComposeProject}} {{end}}{{color .ContainerColor .ContainerName}} {{.Message}}"
		case "raw":
			t = "{{.Message}}"
		case "json":
			t = "{{json .}}"
		case "extjson":
			t = "{\"compose\": \"{{if .ComposeProject}}{{color .ComposeColor .ComposeProject}}{{end}}\", \"container\": \"{{color .ContainerColor .ContainerName}}\", \"message\": {{extjson .Message}}}"
		case "ppextjson":
			t = "{\n  \"compose\": \"{{if .ComposeProject}}{{color .ComposeColor .ComposeProject}}{{end}}\",\n  \"container\": \"{{color .ContainerColor .ContainerName}}\",\n  \"message\": {{extjson .Message}}\n}"
		default:
			return nil, errors.New("output should be one of 'default', 'raw', 'json', 'extjson', and 'ppextjson'")
		}
		t += "\n"
	}

	funs := map[string]interface{}{
		"json": func(in interface{}) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"tryParseJSON": func(text string) map[string]interface{} {
			decoder := json.NewDecoder(strings.NewReader(text))
			decoder.UseNumber()
			obj := make(map[string]interface{})
			if err := decoder.Decode(&obj); err != nil {
				return nil
			}
			return obj
		},
		"parseJSON": func(text string) (map[string]interface{}, error) {
			obj := make(map[string]interface{})
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				return obj, err
			}
			return obj, nil
		},
		"extractJSONParts": func(text string, part ...string) (string, error) {
			obj := make(map[string]interface{})
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				return "", err
			}
			parts := make([]string, 0)
			for _, key := range part {
				parts = append(parts, fmt.Sprintf("%v", obj[key]))
			}
			return strings.Join(parts, ", "), nil
		},
		"tryExtractJSONParts": func(text string, part ...string) string {
			obj := make(map[string]interface{})
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				return text
			}
			parts := make([]string, 0)
			for _, key := range part {
				parts = append(parts, fmt.Sprintf("%v", obj[key]))
			}
			return strings.Join(parts, ", ")
		},
		"extjson": func(in string) (string, error) {
			if json.Valid([]byte(in)) {
				return strings.TrimSuffix(in, "\n"), nil
			}
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return strings.TrimSuffix(string(b), "\n"), nil
		},
		"toRFC3339Nano": func(ts any) string {
			return toTime(ts).Format(time.RFC3339Nano)
		},
		"toUTC": func(ts any) time.Time {
			return toTime(ts).UTC()
		},
		"toTimestamp": func(ts any, layout string, optionalTZ ...string) (string, error) {
			t, parseErr := toTimeE(ts)
			if parseErr != nil {
				return "", parseErr
			}

			var tz string
			if len(optionalTZ) > 0 {
				tz = optionalTZ[0]
			}

			loc, loadErr := time.LoadLocation(tz)
			if loadErr != nil {
				return "", loadErr
			}

			return t.In(loc).Format(layout), nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
		"colorBlack":   color.BlackString,
		"colorRed":     color.RedString,
		"colorGreen":   color.GreenString,
		"colorYellow":  color.YellowString,
		"colorBlue":    color.BlueString,
		"colorMagenta": color.MagentaString,
		"colorCyan":    color.CyanString,
		"colorWhite":   color.WhiteString,
		"levelColor": func(level string) string {
			var levelColor *color.Color
			switch strings.ToLower(level) {
			case "debug":
				levelColor = color.New(color.FgMagenta)
			case "info":
				levelColor = color.New(color.FgBlue)
			case "warn":
				levelColor = color.New(color.FgYellow)
			case "warning":
				levelColor = color.New(color.FgYellow)
			case "error":
				levelColor = color.New(color.FgRed)
			case "dpanic":
				levelColor = color.New(color.FgRed)
			case "panic":
				levelColor = color.New(color.FgRed)
			case "fatal":
				levelColor = color.New(color.FgCyan)
			case "critical":
				levelColor = color.New(color.FgCyan)
			default:
				return level
			}
			return levelColor.SprintFunc()(level)
		},
	}
	template, err := template.New("log").Funcs(funs).Parse(t)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse template")
	}
	return template, err
}

// TODO: Adjust to container filter? Maybe this is where image filtering is done
//func (o *options) generateFieldSelector() (fields.Selector, error) {
//	var queries []string
//	if o.fieldSelector != "" {
//		queries = append(queries, o.fieldSelector)
//	}
//	if o.node != "" {
//		queries = append(queries, fmt.Sprintf("spec.nodeName=%s", o.node))
//	}
//	if len(queries) == 0 {
//		return fields.Everything(), nil
//	}
//
//	fieldSelector, err := fields.ParseSelector(strings.Join(queries, ","))
//	if err != nil {
//		return nil, errors.Wrap(err, "failed to parse selector as field selector")
//	}
//	return fieldSelector, nil
//}

func NewTailfinCmd(streams IOStreams) (*cobra.Command, error) {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:   "tailfin container-query",
		Short: "Tail multiple docker containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Output version information and exit
			if o.version {
				outputVersionInfo(o.Out)
				return nil
			}

			// Output shell completion code for the specified shell and exit
			if o.completion != "" {
				return runCompletion(o.completion, cmd, o.Out)
			}

			if err := o.Complete(args); err != nil {
				return err
			}

			if err := o.overrideFlagSetDefaultFromConfig(cmd.Flags()); err != nil {
				return err
			}

			if err := o.Validate(); err != nil {
				return err
			}

			cmd.SilenceUsage = true

			return o.Run(cmd)
		},
		ValidArgsFunction: queryCompletionFunc(o),
	}

	o.AddFlags(cmd.Flags())

	if err := registerCompletionFuncForFlags(cmd, o); err != nil {
		return cmd, err
	}

	return cmd, nil
}

// makeUnique makes items in string slice unique
func makeUnique(items []string) []string {
	result := []string{}
	m := make(map[string]struct{})

	for _, item := range items {
		if item == "" {
			continue
		}

		if _, ok := m[item]; !ok {
			m[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

func compileREs(exprs []string) ([]*regexp.Regexp, error) {
	var regexps []*regexp.Regexp
	for _, s := range exprs {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}
	return regexps, nil
}
