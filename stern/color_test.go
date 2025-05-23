package stern

import (
	"testing"

	"github.com/fatih/color"
)

func TestParseColors(t *testing.T) {
	tests := []struct {
		desc            string
		namespaceColors []string
		containerColors []string
		want            [][2]*color.Color
		wantError       bool
	}{
		{
			desc:            "both compose and container colors are specified",
			namespaceColors: []string{"91", "92", "93"},
			containerColors: []string{"31", "32", "33"},
			want: [][2]*color.Color{
				{color.New(color.FgHiRed), color.New(color.FgRed)},
				{color.New(color.FgHiGreen), color.New(color.FgGreen)},
				{color.New(color.FgHiYellow), color.New(color.FgYellow)},
			},
		},
		{
			desc:            "only compose colors are specified",
			namespaceColors: []string{"91", "92", "93"},
			containerColors: []string{},
			want: [][2]*color.Color{
				{color.New(color.FgHiRed), color.New(color.FgHiRed)},
				{color.New(color.FgHiGreen), color.New(color.FgHiGreen)},
				{color.New(color.FgHiYellow), color.New(color.FgHiYellow)},
			},
		},
		{
			desc:            "multiple attributes",
			namespaceColors: []string{"4;91"},
			containerColors: []string{"38;2;255;97;136"},
			want: [][2]*color.Color{
				{
					color.New(color.Underline, color.FgHiRed),
					color.New(38, 2, 255, 97, 136), // 24-bit color
				},
			},
		},
		{
			desc:            "spaces are ignored",
			namespaceColors: []string{"  91 ", "\t92\t"},
			containerColors: []string{},
			want: [][2]*color.Color{
				{color.New(color.FgHiRed), color.New(color.FgHiRed)},
				{color.New(color.FgHiGreen), color.New(color.FgHiGreen)},
			},
		},
		// error patterns
		{
			desc:            "only container colors are specified",
			namespaceColors: []string{},
			containerColors: []string{"31", "32", "33"},
			wantError:       true,
		},
		{
			desc:            "both compose and container colors are empty",
			namespaceColors: []string{},
			containerColors: []string{},
			wantError:       true,
		},
		{
			desc:            "invalid color",
			namespaceColors: []string{"a"},
			containerColors: []string{""},
			wantError:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			colorList, err := parseColors(tt.namespaceColors, tt.containerColors)

			if tt.wantError {
				if err == nil {
					t.Error("expected err, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if len(tt.want) != len(colorList) {
				t.Fatalf("expected colorList of size %d, but got %d", len(tt.want), len(colorList))
			}

			for i, wantPair := range tt.want {
				gotPair := colorList[i]
				if !wantPair[0].Equals(gotPair[0]) {
					t.Errorf("colorList[%d][0]: expected %v, but got %v", i, wantPair[0], gotPair[0])
				}
				if !wantPair[1].Equals(gotPair[1]) {
					t.Errorf("colorList[%d][1]: expected %v, but got %v", i, wantPair[1], gotPair[1])
				}
			}
		})
	}
}
