package flags

import (
	"reflect"
	"testing"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/defaults"
	"github.com/pasataleo/go-config/pkg/required"
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
		// A pointer field is only allocated when a flag sets it.
		"pointer_no_value": {
			values: map[string][]string{},
			in: &struct {
				Count *int  `flag:"count"`
				On    *bool `flag:"on"`
			}{},
			want: &struct {
				Count *int  `flag:"count"`
				On    *bool `flag:"on"`
			}{},
		},
		"pointer_zero_value": {
			values: map[string][]string{
				"count": {"0"},
			},
			in: &struct {
				Count *int `flag:"count"`
			}{},
			want: &struct {
				Count *int `flag:"count"`
			}{
				Count: ptr(0),
			},
		},
		"pointer_negated": {
			values: map[string][]string{
				"no-on": {""},
			},
			in: &struct {
				On *bool `flag:"on"`
			}{},
			want: &struct {
				On *bool `flag:"on"`
			}{
				On: ptr(false),
			},
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

func ptr[T any](v T) *T {
	return &v
}

// TestAssign_leavesUnsetFieldsToLaterSources runs flags ahead of defaults and
// required, as a program does. A flag that was not given must leave its field
// as it was, or those sources see a value that nobody set.
func TestAssign_leavesUnsetFieldsToLaterSources(t *testing.T) {
	t.Run("default applies", func(t *testing.T) {
		in := &struct {
			Count *int `flag:"count" default:"5"`
		}{}

		testingx.Call(t, config.New(NewSource(nil), new(defaults.Source)).Assign, in).NoError()
		testingx.Capture(t, in.Count).Equal(ptr(5))
	})

	t.Run("flag beats default", func(t *testing.T) {
		in := &struct {
			Count *int `flag:"count" default:"5"`
		}{}

		values := map[string][]string{"count": {"0"}}
		testingx.Call(t, config.New(NewSource(values), new(defaults.Source)).Assign, in).NoError()
		testingx.Capture(t, in.Count).Equal(ptr(0))
	})

	t.Run("required reports it missing", func(t *testing.T) {
		in := &struct {
			Name *string `flag:"name" required:"name"`
		}{}

		testingx.Call(t, config.New(NewSource(nil), new(required.Source)).Assign, in).ErrorCode(config.MissingValue)
	})
}
