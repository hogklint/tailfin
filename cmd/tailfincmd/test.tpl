{{color .ComposeColor .ComposeProject}} {{color .ContainerColor .ContainerName}} {{ with $msg := .Message | tryParseJSON }}[{{ colorGreen (toRFC3339Nano (toUTC $msg.ts)) }}] {{ levelColor $msg.level }} {{ $msg.msg }}{{ else }}{{ .Message }}{{ end }}
