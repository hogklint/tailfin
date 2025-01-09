[![Build](https://github.com/hogklint/tailfin/workflows/CI/badge.svg)](https://github.com/hogklint/tailfin/actions?query=workflow%3ACI+branch%3Amaster)
# tailfin

Tailfin allows you to tail multiple Docker containers. Each result is color coded for quicker debugging.

The query is a regular expression of container names for easy filtering. If a container terminates it gets removed from
tail and if a new container starts it automatically gets tailed.

Containers can also be filtered on the image name which is handy when they are assigned random names.

Built on the excellent work of [stern](https://github.com/stern/stern).

## Installation

### Download binary

Download a [binary release](https://github.com/hogklint/tailfin/releases)

### Build from source

```
go install github.com/hogklint/tailfin/cmd/tailfin@latest
```

## Usage

```
tailfin [flags] [container query]...
```

The `container query` is a regular expression of the container name; you could provide `"web-\w"` to tail
`web-backend` and `web-frontend` containers but not `web-123`.

### CLI Flags

<!-- auto generated cli flags begin --->
 flag                        | default                         | purpose
-----------------------------|---------------------------------|---------
 `--color`                   | `auto`                          | Force set color output. 'auto':  colorize if tty attached, 'always': always colorize, 'never': never colorize.
 `--completion`              |                                 | Output stern command-line completion code for the specified shell. Can be 'bash', 'zsh' or 'fish'.
 `--compose-colors`          |                                 | Specifies the colors used to highlight container names. Provide colors as a comma-separated list using SGR (Select Graphic Rendition) sequences, e.g., "91,92,93,94,95,96".
 `--config`                  | `~/.config/tailfin/config.yaml` | Path to the tailfin config file
 `--container-colors`        |                                 | Specifies the colors used to highlight compose project names. Use the same format as --container-colors. Defaults to the values of --container-colors if omitted, and must match its length.
 `--exclude`, `-e`           | `[]`                            | Log lines to exclude. (regular expression)
 `--exclude-container`, `-E` | `[]`                            | Container name to exclude. (regular expression)
 `--highlight`, `-H`         | `[]`                            | Log lines to highlight. (regular expression)
 `--image`, `-m`             | `[]`                            | Images to match (regular expression)
 `--include`, `-i`           | `[]`                            | Log lines to include. (regular expression)
 `--max-log-requests`        | `-1`                            | Maximum number of concurrent logs to request. Defaults to 50, but 5 when specifying --no-follow
 `--no-follow`               | `false`                         | Exit when all logs have been shown.
 `--only-log-lines`          | `false`                         | Print only log lines
 `--output`, `-o`            | `default`                       | Specify predefined template. Currently support: [default, raw, json, extjson, ppextjson]
 `--since`, `-s`             | `48h0m0s`                       | Return logs newer than a relative duration like 5s, 2m, or 3h. The duration is truncated at container start time.
 `--stdin`                   | `false`                         | Parse logs from stdin. All Docker related flags are ignored when it is set.
 `--tail`                    | `-1`                            | The number of lines from the end of the logs to show. Defaults to -1, showing all logs.
 `--template`                |                                 | Template to use for log lines, leave empty to use --output flag.
 `--template-file`, `-T`     |                                 | Path to template to use for log lines, leave empty to use --output flag. It overrides --template option.
 `--timestamps`, `-t`        |                                 | Print timestamps with the specified format. One of 'default' or 'short' in the form '--timestamps=format' ('=' cannot be omitted). If specified but without value, 'default' is used.
 `--timezone`                | `Local`                         | Set timestamps to specific timezone.
 `--verbosity`               | `0`                             | Number of the log level verbosity
 `--version`, `-v`           | `false`                         | Print the version and exit.
<!-- auto generated cli flags end --->

See `tailfin --help` for details

Tailfin will use the [Docker environment variables](https://docs.docker.com/reference/cli/docker/#environment-variables)
if set. <!--*TODO* If both the environment variable and `--context` flag are passed the CLI flag will be used.-->

### config file
You can use the config file to change the default values of tailfin options. The default config file path is
`~/.config/tailfin/config.yaml`.

```yaml
# <flag name>: <value>
max-log-requests: 999
timestamps: short
```
<!-- TODO (tail)
```yaml
# <flag name>: <value>
tail: 10
max-log-requests: 999
timestamps: short
```
-->
You can change the config file path with `--config` flag or `TAILFINCONFIG` environment variable.
### templates

Tailfin supports outputting custom log messages.  There are a few predefined templates which you can use by specifying
the `--output` flag:

| output    | description                                                                                           |
|-----------|-------------------------------------------------------------------------------------------------------|
| `default` | Displays the compose project and container, and decorates it with color depending on --color          |
| `raw`     | Only outputs the log message itself, useful when your logs are json and you want to pipe them to `jq` |
| `json`    | Marshals the log struct to json. Useful for programmatic purposes                                     |

It accepts a custom template through the `--template` flag, which will be
compiled to a Go template and then used for every log message. This Go template
will receive the following struct:

| property        | type   | description                                    |
|-----------------|--------|------------------------------------------------|
| `Message`       | string | The log message itself                         |
| `ComposeProject`| string | The name of the docker compose project, if any |
| `ContainerName` | string | The name of the container                      |

The following functions are available within the template (besides the [builtin
functions](https://golang.org/pkg/text/template/#hdr-Functions)):

| func            | arguments             | description                                                                       |
|-----------------|-----------------------|-----------------------------------------------------------------------------------|
| `json`          | `object`              | Marshal the object and output it as a json text                                   |
| `color`         | `color.Color, string` | Wrap the text in color (.ContainerColor and .ComposeColor provided)               |
| `parseJSON`     | `string`              | Parse string as JSON                                                              |
| `tryParseJSON`  | `string`              | Attempt to parse string as JSON, return nil on failure                            |
| `extractJSONParts`    | `string, ...string` | Parse string as JSON and concatenate the given keys.                          |
| `tryExtractJSONParts` | `string, ...string` | Attempt to parse string as JSON and concatenate the given keys. , return text on failure |
| `extjson`         | `string`              | Parse the object as json and output colorized json                                |
| `ppextjson`       | `string`              | Parse the object as json and output pretty-print colorized json                   |
| `toRFC3339Nano`   | `object`              | Parse timestamp (string, int, json.Number) and output it using RFC3339Nano format |
| `msToRFC3339Nano` | `object`            | Parse milliseconds timestamp (string, int) and output it using RFC3339Nano format   |
| `toTimestamp`     | `object, string [, string]` | Parse timestamp (string, int, json.Number) and output it using the given layout in the timezone that is optionally given (defaults to UTC). |
| `levelColor`      | `string`              | Print log level using appropriate color                                           |
| `bunyanLevelColor` | `string`             | Print [bunyan](https://github.com/trentm/node-bunyan) numeric log level using appropriate color |
| `colorBlack`      | `string`              | Print text using black color                                                      |
| `colorRed`        | `string`              | Print text using red color                                                        |
| `colorGreen`      | `string`              | Print text using green color                                                      |
| `colorYellow`     | `string`              | Print text using yellow color                                                     |
| `colorBlue`       | `string`              | Print text using blue color                                                       |
| `colorMagenta`    | `string`              | Print text using magenta color                                                    |
| `colorCyan`       | `string`              | Print text using cyan color                                                       |
| `colorWhite`      | `string`              | Print text using white color                                                      |

### Max log requests

Tailfin has the maximum number of concurrent logs to request to prevent unintentional load to a docker daemon.
The number can be configured by the `--max-log-requests` flag.

The behavior and the default are different depending on the presence of the `--no-follow` flag.

| `--no-follow` | default | behavior         |
|---------------|---------|------------------|
| specified     | 5       | limits the number of concurrent logs to request |
| not specified | 50      | exits with an error when if it reaches the concurrent limit |

### Customize highlight colors
You can configure highlight colors for compose projects and containers in [the config file](#config-file) using a comma-separated list of [SGR (Select Graphic Rendition) sequences](https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters), as shown below. If you omit `container-colors`, the compose project colors will be used as container colors as well.

```yaml
# Green, Yellow, Blue, Magenta, Cyan, White
compose-colors: "32,33,34,35,36,37"

# Colors with underline (4)
# If empty, the compose colors will be used as container colors
container-colors: "32;4,33;4,34;4,35;4,36;4,37;4"
```

This format enables the use of various attributes, such as underline, background colors, 8-bit colors, and 24-bit colors, if your terminal supports them.

The equivalent flags `--compose-colors` and `--container-colors` are also available. The following command applies [24-bit colors](https://en.wikipedia.org/wiki/ANSI_escape_code#24-bit) using the `--compose-colors` flag.

```bash
# Monokai theme
composeColors="38;2;255;97;136,38;2;169;220;118,38;2;255;216;102,38;2;120;220;232,38;2;171;157;242"
tailfin --compose-colors "$composeColors" app
```

## Examples:
Tail all logs
```
tailfin .
```
<!--
*TODO* Tail the `test` compose project without printing any prior logs
```
tailfin . -c test --tail 0
```
-->
Tail everything excluding logs from `backend` container
```
tailfin --exclude-container backend .
```

Show auth activity from 15min ago with timestamps
```
tailfin auth -t --since 15m
```

Show all logs of the last 5min by time, sorted by time (`sort -k3` when using docker compose)
```
tailfin --since=5m --no-follow --only-log-lines -t . | sort -k2
```

Show `backend` container with timestamps in specific timezone (default is your local timezone)
```
tailfin backend -t --timezone Asia/Tokyo
```
<!--
*TODO* Follow the development of `some-new-feature` in esc
```
tailfin some-new-feature --context esc
```
-->
<!--
*TODO* Tail the containers filtered by `run=nginx` label selector
```
tailfin -l run=nginx
```
-->
Pipe the log message to jq:
```
tailfin backend -o json | jq .
```

Only output the log message itself:
```
tailfin backend -o raw
```

Output using a custom template:

```
tailfin --template '{{printf "%s (%s/%s)\n" .Message .ComposeProject .ContainerName}}' backend
```

Output using a custom template with tailfin-provided colors:

```
tailfin --template '{{.Message}} ({{color .ComposeColor .ComposeProject}}/{{color .ContainerColor .ContainerName}}){{"\n"}}' backend
```

Output using a custom template with `parseJSON`:

```
tailfin --template='{{.ComposeProject}}/{{.ContainerName}} {{with $d := .Message | parseJSON}}[{{$d.level}}] {{$d.message}}{{end}}{{"\n"}}' backend
```

Output using a custom template that tries to parse JSON or fallbacks to raw format:

```
tailfin --template='{{.ComposeProject}}/{{.ContainerName}} {{ with $msg := .Message | tryParseJSON }}[{{ colorGreen (toRFC3339Nano $msg.ts) }}] {{ levelColor $msg.level }} ({{ colorCyan $msg.caller }}) {{ $msg.msg }}{{ else }} {{ .Message }} {{ end }}{{"\n"}}' backend
```

Load custom template from file:

```
tailfin --template-file=~/.tailfin.tpl backend
```

Output log lines only:

```
tailfin . --only-log-lines
```

Read from stdin:

```
tailfin --stdin < service.log
```

## Completion

Tailfin supports command-line auto completion for bash, zsh or fish. `tailfin
--completion=(bash|zsh|fish)` outputs the shell completion code which work by being
evaluated in `.bashrc`, etc for the specified shell. <!--*TODO* In addition, Tailfin
supports dynamic completion for `--context`and flags with pre-defined choices.-->

If you use bash, tailfin bash completion code depends on the
[bash-completion](https://github.com/scop/bash-completion).

Note that bash-completion must be sourced before sourcing the tailfin bash
completion code in `.bashrc`.

```sh
source /path/to/bash_completion.sh"
source <(tailfin --completion=bash)
```

If you use zsh, just source the tailfin zsh completion code in `.zshrc`.

```sh
source <(tailfin --completion=zsh)
```

if you use fish shell, just source the tailfin fish completion code.

```sh
tailfin --completion=fish | source

# To load completions for each session, execute once:
tailfin --completion=fish >~/.config/fish/completions/tailfin.fish
```

## Contributing to this repository

Please see [CONTRIBUTING](CONTRIBUTING.md) for details.
