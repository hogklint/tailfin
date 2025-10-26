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
	"fmt"
	"github.com/containerd/log"
	"github.com/fatih/color"
	"github.com/hogklint/tailfin/stern"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

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

	color            string
	completion       string
	compose          []string
	context          string
	configFilePath   string
	containerColors  []string
	containerQuery   []string
	exclude          []string
	excludeContainer []string
	highlight        []string
	image            []string
	include          []string
	label            []string
	maxLogRequests   int
	namespaceColor   []string
	noFollow         bool
	onlyLogLines     bool
	output           string
	since            time.Duration
	stdin            bool
	tail             int64
	template         string
	templateFile     string
	timestamps       string
	timezone         string
	verbosity        string
	version          bool
	//containerStates     []string
	//selector            string

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
		verbosity:      "fatal",
		maxLogRequests: -1,
		configFilePath: defaultConfigFilePath,
	}
}

func (o *options) Complete(args []string) {
	if len(args) > 0 {
		o.containerQuery = args
	}

	envVar, ok := os.LookupEnv("TAILFINCONFIG")
	if ok {
		o.configFilePath = envVar
	}
}

func (o *options) Validate() error {
	if len(o.containerQuery) == 0 && len(o.label) == 0 && len(o.image) == 0 && !o.stdin {
		return errors.New("One of container-query, --label, --image, or --stdin is required")
	}

	return nil
}

func (o *options) Run(cmd *cobra.Command) error {
	if err := o.setColorList(); err != nil {
		return err
	}

	config, err := o.tailfinConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = o.dockerClient.Ping(ctx)
	if err != nil {
		return err
	}

	return stern.RunDocker(ctx, o.dockerClient, config)
}

func (o *options) tailfinConfig() (*stern.DockerConfig, error) {
	container, err := compileREs(o.containerQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression from query")
	}

	excludeContainer, err := compileREs(o.excludeContainer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for excluded container query")
	}

	compose, err := compileREs(o.compose)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for compose filter")
	}

	exclude, err := compileREs(o.exclude)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for exclusion filter")
	}

	image, err := compileREs(o.image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for image filter")
	}

	include, err := compileREs(o.include)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for inclusion filter")
	}

	highlight, err := compileREs(o.highlight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for highlight filter")
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
		ComposeProjectQuery:   compose,
		ContainerQuery:        container,
		Exclude:               exclude,
		ExcludeContainerQuery: excludeContainer,
		Follow:                !o.noFollow,
		Highlight:             highlight,
		ImageQuery:            image,
		Include:               include,
		Label:                 o.label,
		Location:              location,
		MaxLogRequests:        maxLogRequests,
		OnlyLogLines:          o.onlyLogLines,
		Since:                 o.since,
		Stdin:                 o.stdin,
		TailLines:             o.tail,
		Template:              template,
		TimestampFormat:       timestampFormat,
		Timestamps:            timestampFormat != "",

		Out:    o.Out,
		ErrOut: o.ErrOut,
	}, nil
}

// setVerbosity sets the log level verbosity
func (o *options) setVerbosity() error {
	if o.verbosity != "" {
		return log.SetLevel(o.verbosity)
	}
	return nil
}

func (o *options) setColorList() error {
	if len(o.containerColors) > 0 || len(o.namespaceColor) > 0 {
		return stern.SetColorList(o.namespaceColor, o.containerColors)
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
			log.L.Warningf("Unknown option specified in the config file: %s", name)
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
	fs.StringVar(&o.completion, "completion", o.completion, "Output tailfin command-line completion code for the specified shell. Can be 'bash', 'zsh' or 'fish'.")
	fs.StringArrayVar(&o.compose, "compose", o.compose, "Compose project name to match (regular expression)")
	fs.StringArrayVarP(&o.exclude, "exclude", "e", o.exclude, "Log lines to exclude. (regular expression)")
	fs.StringArrayVarP(&o.excludeContainer, "exclude-container", "E", o.excludeContainer, "Container name to exclude. (regular expression)")
	fs.BoolVar(&o.noFollow, "no-follow", o.noFollow, "Exit when all logs have been shown.")
	fs.StringArrayVarP(&o.image, "image", "m", o.image, "Images to match (regular expression)")
	fs.StringArrayVarP(&o.include, "include", "i", o.include, "Log lines to include. (regular expression)")
	fs.StringVar(&o.context, "context", o.context, "Docker context to use")
	fs.StringArrayVarP(&o.highlight, "highlight", "H", o.highlight, "Log lines to highlight. (regular expression)")
	fs.StringArrayVarP(&o.label, "label", "l", o.label, "Label query to filter on. One `key` or `key=value` per flag instance.")
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
	fs.StringVar(&o.verbosity, "verbosity", o.verbosity, "Log level. One of panic, fatal, error, warning, info, debug, or trace")
	fs.BoolVarP(&o.version, "version", "v", o.version, "Print the version and exit.")
	fs.BoolVar(&o.stdin, "stdin", o.stdin, "Parse logs from stdin. All Docker related flags are ignored when it is set.")
	fs.StringSliceVar(&o.containerColors, "container-colors", o.containerColors, "Specifies the colors used to highlight container names. Use the same format as --namespace-colors. Defaults to the values of --namespace-colors if omitted, and must match its length.")
	fs.StringSliceVar(&o.namespaceColor, "namespace-colors", o.namespaceColor, "Specifies the colors used to highlight namespace (compose project). Provide colors as a comma-separated list using SGR (Select Graphic Rendition) sequences, e.g., \"91,92,93,94,95,96\".")
	// TODO: --context for docker context? Seems to be a `docker` thing, not a dockerd thing.
	// TODO  --prompt??

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
			t = "{{if .Namespace}}{{color .NamespaceColor .Namespace}} {{end}}{{color .ContainerColor .ServiceName}} {{.Message}}"
		case "raw":
			t = "{{.Message}}"
		case "json":
			t = "{{json .}}"
		case "extjson":
			t = "{\"namespace\": \"{{if .Namespace}}{{color .NamespaceColor .Namespace}}{{end}}\", \"service\": \"{{color .ContainerColor .ServiceName}}\", \"message\": {{extjson .Message}}}"
		case "ppextjson":
			t = "{\n  \"namespace\": \"{{if .Namespace}}{{color .NamespaceColor .Namespace}}{{end}}\",\n  \"service\": \"{{color .ContainerColor .ServiceName}}\",\n  \"message\": {{extjson .Message}}\n}"
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
		"msToRFC3339Nano": func(ts any) string {
			return toTimeMilli(ts).Format(time.RFC3339Nano)
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
		"levelColor": func(value any) string {
			switch level := value.(type) {
			case string:
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
			default:
				return ""
			}
		},
		"bunyanLevelColor": func(value any) string {
			var lv int64
			var err error

			switch level := value.(type) {
			// tryParseJSON yields json.Number
			case json.Number:
				lv, err = level.Int64()
				if err != nil {
					return ""
				}
			// parseJSON yields float64
			case float64:
				lv = int64(level)
			default:
				return ""
			}

			var levelColor *color.Color
			switch {
			case lv < 30:
				levelColor = color.New(color.FgMagenta)
			case lv < 40:
				levelColor = color.New(color.FgBlue)
			case lv < 50:
				levelColor = color.New(color.FgYellow)
			case lv < 60:
				levelColor = color.New(color.FgRed)
			case lv < 100:
				levelColor = color.New(color.FgCyan)
			default:
				return strconv.FormatInt(lv, 10)
			}
			return levelColor.SprintFunc()(lv)
		},
	}
	template, err := template.New("log").Funcs(funs).Parse(t)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse template")
	}
	return template, err
}

func NewTailfinCmd(streams IOStreams) (*cobra.Command, error) {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:   "tailfin container-query",
		Short: "Tail multiple docker containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.setVerbosity(); err != nil {
				return err
			}

			// Output version information and exit
			if o.version {
				outputVersionInfo(o.Out)
				return nil
			}

			// Output shell completion code for the specified shell and exit
			if o.completion != "" {
				return runCompletion(o.completion, cmd, o.Out)
			}

			o.Complete(args)

			if err := o.overrideFlagSetDefaultFromConfig(cmd.Flags()); err != nil {
				return err
			}

			if err := o.Validate(); err != nil {
				return err
			}

			cmd.SilenceUsage = true

			var err error
			if o.dockerClient, err = getDockerClient(o.context); err != nil {
				return err
			}

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
