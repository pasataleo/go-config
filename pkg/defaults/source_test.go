package defaults

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
				Claimed string `default:"claimed"`
			}{},
			want: map[string]string{
				"Claimed": "claimed",
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
			in: &struct {
				Claimed string `default:"claimed"`
			}{},
			want: &struct {
				Claimed string `default:"claimed"`
			}{
				Claimed: "claimed",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Assign, tc.in).NoError()
			testingx.Capture(t, tc.in).Equal(
				tc.want,
				diff.WithRenderOpts(render.NewOpts(t, render.WithSkipUnexported(true))))
		})
	}
}
