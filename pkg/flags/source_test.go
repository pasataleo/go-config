package flags

import (
	"reflect"
	"testing"

	"github.com/pasataleo/go-reflectx/pkg/reflectx"
	"github.com/pasataleo/go-testingx/pkg/diff"
	"github.com/pasataleo/go-testingx/pkg/render"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestFind(t *testing.T) {
	tcs := map[string]struct {
		in   interface{}
		want map[string]Flag
	}{
		"empty": {
			in:   &struct{}{},
			want: map[string]Flag{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
			want: map[string]Flag{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
			want: map[string]Flag{},
		},
		"claimed": {
			in: &struct {
				Claimed string `flag:"claimed"`
			}{},
			want: map[string]Flag{
				"Claimed": {
					Name: "claimed",
					Type: reflect.TypeFor[string](),
				},
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Find, tc.in).Equal(tc.want,
				diff.WithDiffer(new(reflectx.TypeDiffer)),
				diff.WithRenderOpts(render.NewOpts(t, render.WithRenderer(new(reflectx.TypeRenderer)))))
		})
	}
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		values map[string][]string
		in     interface{}
		want   interface{}
	}{
		"empty": {
			values: map[string][]string{},
			in:     &struct{}{},
			want:   &struct{}{},
		},
		"private": {
			values: map[string][]string{},
			in: &struct {
				hidden string
			}{},
			want: &struct {
				hidden string
			}{},
		},
		"unclaimed": {
			values: map[string][]string{},
			in: &struct {
				Unclaimed string
			}{},
			want: &struct {
				Unclaimed string
			}{},
		},
		"claimed": {
			values: map[string][]string{
				"claimed": {"value"},
			},
			in: &struct {
				Claimed string `flag:"claimed"`
			}{},
			want: &struct {
				Claimed string `flag:"claimed"`
			}{
				Claimed: "value",
			},
		},
		"no_value": {
			values: map[string][]string{},
			in: &struct {
				Claimed string `flag:"claimed"`
			}{},
			want: &struct {
				Claimed string `flag:"claimed"`
			}{},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Assign, tc.values, tc.in).NoError()
			testingx.Capture(t, tc.in).Equal(
				tc.want,
				diff.WithRenderOpts(render.NewOpts(t, render.WithSkipUnexported(true))))
		})
	}
}
