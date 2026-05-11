package flags

import (
	"testing"

	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestParse(t *testing.T) {
	tcs := map[string]struct {
		args     []string
		wantVals map[string][]string
		wantArgs []string
	}{
		"empty": {
			args:     nil,
			wantVals: map[string][]string{},
			wantArgs: nil,
		},
		"positional_only": {
			args:     []string{"arg1", "arg2"},
			wantVals: map[string][]string{},
			wantArgs: []string{"arg1", "arg2"},
		},
		"long_flag_with_value": {
			args: []string{"--flag", "value"},
			wantVals: map[string][]string{
				"flag": {"value"},
			},
			wantArgs: nil,
		},
		"long_flag_with_equals": {
			args: []string{"--flag=value"},
			wantVals: map[string][]string{
				"flag": {"value"},
			},
			wantArgs: nil,
		},
		"short_flag_with_value": {
			args: []string{"-f", "value"},
			wantVals: map[string][]string{
				"f": {"value"},
			},
			wantArgs: nil,
		},
		"double_dash_separator": {
			args: []string{"command", "--flag", "value", "--", "--not-a-flag"},
			wantVals: map[string][]string{
				"flag": {"value"},
			},
			wantArgs: []string{"command", "--", "--not-a-flag"},
		},
		"boolean_flag": {
			args: []string{"--verbose"},
			wantVals: map[string][]string{
				"verbose": {""},
			},
			wantArgs: nil,
		},
		"repeated_flag": {
			args: []string{"--item", "a", "--item", "b"},
			wantVals: map[string][]string{
				"item": {"a", "b"},
			},
			wantArgs: nil,
		},
		"mixed": {
			args: []string{"pos1", "--flag", "value", "pos2"},
			wantVals: map[string][]string{
				"flag": {"value"},
			},
			wantArgs: []string{"pos1", "pos2"},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Parse, tc.args).
				Equal(tc.wantVals).
				Equal(tc.wantArgs)
		})
	}
}
