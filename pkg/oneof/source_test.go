package oneof_test

import (
	"testing"

	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-testingx/pkg/testingx"

	config "github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-config/pkg/inject"
	"github.com/pasataleo/go-config/pkg/oneof"
	"github.com/pasataleo/go-config/pkg/required"
)

// assign runs a walk the way a program sets one up: oneof first, then the
// sources that claim fields, then required last.
func assign(t *testing.T, args []string, v interface{}, claiming ...config.Source) error {
	t.Helper()

	_, values := flags.Parse(args)
	sources := []config.Source{new(oneof.Source), flags.NewSource(values)}
	sources = append(sources, claiming...)
	sources = append(sources, new(required.Source))
	return config.New(sources...).Assign(v)
}

// listen is a group of two: exactly one of port and socket.
type listen struct {
	Port   int    `flag:"port" required:"port" oneof:"listen"`
	Socket string `flag:"socket" required:"socket" oneof:"listen"`
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		args     []string
		in       interface{}
		wantCode errorsx.Code
		wantMsg  string
	}{
		"one set": {
			args: []string{"--port", "8080"},
			in:   new(listen),
		},
		"the other set": {
			args: []string{"--socket", "/tmp/plugin.sock"},
			in:   new(listen),
		},
		"none set": {
			in:       new(listen),
			wantCode: config.MissingValue,
			wantMsg:  "required value not set: one of port, socket",
		},
		"both set": {
			args:     []string{"--port", "8080", "--socket", "/tmp/plugin.sock"},
			in:       new(listen),
			wantCode: config.ConflictingValues,
			wantMsg:  "only one of port, socket can be set, got port, socket",
		},
		// Set means a source provided it, as for a plain required field, so
		// an explicit zero counts.
		"set to zero": {
			args: []string{"--port", "0"},
			in:   new(listen),
		},
		// A value already in the struct is set too, as for a plain required
		// field.
		"set in code": {
			in: &listen{Socket: "/tmp/plugin.sock"},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			err := assign(t, tc.args, tc.in)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			testingx.Call(t, errorsx.ErrorCode, err).Equal(tc.wantCode)
			testingx.Call(t, func() string { return err.Error() }).Equal(tc.wantMsg)
		})
	}
}

// Groups are global, as flag names are.
func TestAssign_global(t *testing.T) {
	type split struct {
		Server struct {
			Port int `flag:"port" required:"port" oneof:"listen"`
		}
		Client struct {
			Socket string `flag:"socket" required:"socket" oneof:"listen"`
		}
	}

	t.Run("members in different structs", func(t *testing.T) {
		testingx.Call(t, errorsx.ErrorCode, assign(t, nil, new(split))).Equal(config.MissingValue)
		if err := assign(t, []string{"--socket", "/tmp/plugin.sock"}, new(split)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		testingx.Call(t, errorsx.ErrorCode, assign(t, []string{"--port", "8080", "--socket", "/tmp/plugin.sock"}, new(split))).Equal(config.ConflictingValues)
	})

	t.Run("a name read in several places is one member", func(t *testing.T) {
		type twice struct {
			Primary   listen
			Secondary listen
		}

		if err := assign(t, []string{"--port", "8080"}, new(twice)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err := assign(t, nil, new(twice))
		testingx.Call(t, func() string { return err.Error() }).Equal("required value not set: one of port, socket")
	})

	t.Run("separate groups stay separate", func(t *testing.T) {
		in := &struct {
			Listen listen
			Output string `flag:"output" required:"output" oneof:"destination"`
			Stdout bool   `flag:"stdout" required:"stdout" oneof:"destination"`
		}{}

		err := assign(t, []string{"--port", "8080"}, in)
		testingx.Call(t, func() string { return err.Error() }).Equal("required value not set: one of output, stdout")
	})
}

// portModule holds one member of a group. inject populates it with an Assign
// nested inside the outer one, so the group only sees both members if what
// that walk records reaches the outer Finish.
type portModule struct {
	Port int `flag:"port" required:"port" oneof:"listen"`
}

func (m *portModule) Install(*inject.Source) error {
	return nil
}

func TestAssign_acrossModules(t *testing.T) {
	run := func(args ...string) error {
		injector := inject.New()
		injector.Register(new(portModule))

		in := &struct {
			Socket string `flag:"socket" required:"socket" oneof:"listen"`
		}{}
		return assign(t, args, in, injector)
	}

	if err := run("--port", "8080"); err != nil {
		t.Errorf("port from the module: unexpected error: %v", err)
	}
	if err := run("--socket", "/tmp/plugin.sock"); err != nil {
		t.Errorf("socket from the outer struct: unexpected error: %v", err)
	}
	testingx.Call(t, errorsx.ErrorCode, run()).Equal(config.MissingValue)
	testingx.Call(t, errorsx.ErrorCode, run("--port", "8080", "--socket", "/tmp/plugin.sock")).Equal(config.ConflictingValues)
}

// What one Assign records is gone by the next.
func TestAssign_configIsReusable(t *testing.T) {
	_, values := flags.Parse(nil)
	cfg := config.New(new(oneof.Source), flags.NewSource(values), new(required.Source))

	testingx.Call(t, errorsx.ErrorCode, cfg.Assign(new(listen))).Equal(config.MissingValue)
	if err := cfg.Assign(&listen{Port: 8080}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testingx.Call(t, errorsx.ErrorCode, cfg.Assign(new(listen))).Equal(config.MissingValue)
}

func TestAssign_withoutRequiredTagPanics(t *testing.T) {
	in := &struct {
		Port int `oneof:"listen"`
	}{}
	testingx.Panics(t, nil, func() { _ = assign(t, nil, in) })
}

func TestFind(t *testing.T) {
	found := config.Find(new(oneof.Source), &struct {
		Name   string `required:"name"`
		Listen listen
	}{})

	testingx.Capture(t, found).Equal(map[string]interface{}{
		"Listen.Port":   "listen",
		"Listen.Socket": "listen",
	})
}
