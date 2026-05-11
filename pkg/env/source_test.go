package env

import (
	"testing"

	"github.com/pasataleo/go-testingx/pkg/diff"
	"github.com/pasataleo/go-testingx/pkg/render"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestFind(t *testing.T) {
	tcs := map[string]struct {
		in   interface{}
		want map[string]string
	}{
		"empty": {
			in:   &struct{}{},
			want: map[string]string{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
			want: map[string]string{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
			want: map[string]string{},
		},
		"claimed": {
			in: &struct {
				Claimed string `env:"TEST_CLAIMED"`
			}{},
			want: map[string]string{
				"Claimed": "TEST_CLAIMED",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Find, tc.in).Equal(tc.want)
		})
	}
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		env  map[string]string
		in   interface{}
		want interface{}
	}{
		"empty": {
			in:   &struct{}{},
			want: &struct{}{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
			want: &struct {
				hidden string
			}{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
			want: &struct {
				Unclaimed string
			}{},
		},
		"claimed": {
			env: map[string]string{
				"TEST_ENV_CLAIMED": "value",
			},
			in: &struct {
				Claimed string `env:"TEST_ENV_CLAIMED"`
			}{},
			want: &struct {
				Claimed string `env:"TEST_ENV_CLAIMED"`
			}{
				Claimed: "value",
			},
		},
		"unset": {
			in: &struct {
				Claimed string `env:"TEST_ENV_UNSET"`
			}{},
			want: &struct {
				Claimed string `env:"TEST_ENV_UNSET"`
			}{},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			testingx.Call(t, Assign, tc.in).NoError()
			testingx.Capture(t, tc.in).Equal(
				tc.want,
				diff.WithRenderOpts(render.NewOpts(t, render.WithSkipUnexported(true))))
		})
	}
}
