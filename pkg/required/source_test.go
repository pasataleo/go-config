package required

import (
	"reflect"
	"testing"

	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-testingx/pkg/testingx"

	config "github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/oneof"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
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
				Claimed string `required:"--claimed"`
			}{},
			want: map[string]string{
				"Claimed": "--claimed",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Capture(t, Find(tc.in)).Equal(tc.want)
		})
	}
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		in      interface{}
		wantErr bool
	}{
		"empty": {
			in: &struct{}{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
		},
		"required_set": {
			in: &struct {
				Claimed string `required:"--claimed"`
			}{
				Claimed: "value",
			},
		},
		"required_unset": {
			in: &struct {
				Claimed string `required:"--claimed"`
			}{},
			wantErr: true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			if tc.wantErr {
				testingx.Call(t, Assign, tc.in).Error()
			} else {
				testingx.Call(t, Assign, tc.in).NoError()
			}
		})
	}
}

// Without the oneof source, nothing records a group, so a field with a oneof
// tag is just required on its own.
func TestAssign_oneofWithoutTheSource(t *testing.T) {
	in := &struct {
		Port   int    `required:"port" oneof:"listen"`
		Socket string `required:"socket" oneof:"listen"`
	}{Port: 8080}

	err := Assign(in)
	testingx.Call(t, errorsx.ErrorCode, err).Equal(config.MissingValue)
	testingx.Call(t, func() string { return err.Error() }).Equal("required value not set: socket")
}

// With a group recorded, an unset member is left for the oneof source to
// judge, and recorded under oneof.UnsetKey.
func TestAssign_groupedFieldIsDeferred(t *testing.T) {
	cfg := config.New(&groupRecorder{}, new(Source))
	in := &struct {
		Socket string `required:"socket"`
	}{}

	if err := cfg.Assign(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// groupRecorder stands in for the oneof source: it puts socket in a group
// before required sees the field, and checks what required recorded.
type groupRecorder struct{ config.Source }

func (g *groupRecorder) Name() string { return "group" }

func (g *groupRecorder) Assign(cfg *config.Config, _ reflectx.Path, _ reflect.Value, _ reflect.StructField) (bool, error) {
	cfg.Data()[oneof.Key] = map[string][]string{"listen": {"socket"}}
	return false, nil
}

func (g *groupRecorder) Finish(cfg *config.Config) error {
	unset, _ := cfg.Data()[oneof.UnsetKey].(map[string]bool)
	if !unset["socket"] {
		return errorsx.Newf(errorsx.Internal, nil, "socket not recorded as unset: %v", cfg.Data())
	}
	return nil
}
