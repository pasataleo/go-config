package internal_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/defaults"
	"github.com/pasataleo/go-config/pkg/env"
	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-config/pkg/inject"
	"github.com/pasataleo/go-config/pkg/required"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
	"github.com/pasataleo/go-testingx/pkg/diff"
	"github.com/pasataleo/go-testingx/pkg/render"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestConfigAssign(t *testing.T) {
	t.Run("defaults_only", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost"`
			Port int    `default:"8080"`
		}
		in := &cfg{}
		want := &cfg{Host: "localhost", Port: 8080}
		c := config.New(new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("env_overrides_default", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" env:"TEST_CFG_ENV_OVERRIDE"`
		}
		t.Setenv("TEST_CFG_ENV_OVERRIDE", "remotehost")
		in := &cfg{}
		want := &cfg{Host: "remotehost"}
		c := config.New(new(env.Source), new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("env_not_set_falls_through_to_default", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" env:"TEST_CFG_ENV_UNSET"`
		}
		in := &cfg{}
		want := &cfg{Host: "localhost"}
		c := config.New(new(env.Source), new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("flags_override_env_and_default", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" env:"TEST_CFG_FLAG_OVERRIDE" flag:"host"`
		}
		t.Setenv("TEST_CFG_FLAG_OVERRIDE", "envhost")
		in := &cfg{}
		want := &cfg{Host: "flaghost"}
		c := config.New(
			flags.NewSource(map[string][]string{"host": {"flaghost"}}),
			new(env.Source),
			new(defaults.Source),
		)
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("flag_not_set_falls_through_to_env", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" env:"TEST_CFG_FLAG_FALLTHROUGH" flag:"host"`
		}
		t.Setenv("TEST_CFG_FLAG_FALLTHROUGH", "envhost")
		in := &cfg{}
		want := &cfg{Host: "envhost"}
		c := config.New(
			flags.NewSource(nil),
			new(env.Source),
			new(defaults.Source),
		)
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("required_satisfied_by_default", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" required:"--host"`
		}
		in := &cfg{}
		want := &cfg{Host: "localhost"}
		c := config.New(new(defaults.Source), new(required.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("required_satisfied_by_env", func(t *testing.T) {
		type cfg struct {
			Host string `env:"TEST_CFG_REQUIRED_ENV" required:"--host"`
		}
		t.Setenv("TEST_CFG_REQUIRED_ENV", "envhost")
		in := &cfg{}
		want := &cfg{Host: "envhost"}
		c := config.New(new(env.Source), new(required.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("required_unsatisfied", func(t *testing.T) {
		type cfg struct {
			Host string `required:"--host"`
		}
		in := &cfg{}
		c := config.New(new(defaults.Source), new(required.Source))
		testingx.Call(t, c.Assign, in).Error()
	})

	t.Run("multiple_required_errors_aggregated", func(t *testing.T) {
		type cfg struct {
			Host string `required:"--host"`
			Port int    `required:"--port"`
		}
		in := &cfg{}
		c := config.New(new(required.Source))
		err := c.Assign(in)
		testingx.Capture(t, err).
			HasError("required value not set: --host")
		testingx.Capture(t, err).
			HasError("required value not set: --port")
	})

	t.Run("inject_with_defaults", func(t *testing.T) {
		type Logger struct{ Name string }
		type cfg struct {
			Host   string  `default:"localhost"`
			Logger *Logger `inject:"logger"`
		}
		injector := inject.New()
		injector.Bind(&Logger{Name: "test"}, "logger")
		in := &cfg{}
		want := &cfg{Host: "localhost", Logger: &Logger{Name: "test"}}
		c := config.New(injector, new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("inject_by_type", func(t *testing.T) {
		type cfg struct {
			Name  string `default:"app"`
			Count int
		}
		injector := inject.New()
		injector.Bind(42)
		in := &cfg{}
		want := &cfg{Name: "app", Count: 42}
		c := config.New(injector, new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("nested_struct", func(t *testing.T) {
		type database struct {
			Host string `default:"localhost" env:"TEST_CFG_NEST_HOST"`
			Port int    `default:"5432" flag:"db-port"`
		}
		type cfg struct {
			Database database
			Name     string `default:"myapp" flag:"name"`
		}
		t.Setenv("TEST_CFG_NEST_HOST", "dbhost")
		in := &cfg{}
		want := &cfg{
			Database: database{Host: "dbhost", Port: 9999},
			Name:     "flagapp",
		}
		c := config.New(
			flags.NewSource(map[string][]string{
				"db-port": {"9999"},
				"name":    {"flagapp"},
			}),
			new(env.Source),
			new(defaults.Source),
		)
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("mixed_types", func(t *testing.T) {
		type cfg struct {
			Name    string  `default:"app"`
			Port    int     `default:"8080"`
			Rate    float64 `default:"1.5"`
			Verbose bool    `default:"true"`
		}
		in := &cfg{}
		want := &cfg{Name: "app", Port: 8080, Rate: 1.5, Verbose: true}
		c := config.New(new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("partial_sources", func(t *testing.T) {
		type cfg struct {
			Host string `flag:"host"`
			Port int    `default:"8080"`
			Name string `env:"TEST_CFG_PARTIAL_NAME"`
		}
		t.Setenv("TEST_CFG_PARTIAL_NAME", "myapp")
		in := &cfg{}
		want := &cfg{Host: "flaghost", Port: 8080, Name: "myapp"}
		c := config.New(
			flags.NewSource(map[string][]string{"host": {"flaghost"}}),
			new(env.Source),
			new(defaults.Source),
		)
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})
}

func TestConfigAssignModules(t *testing.T) {
	t.Run("basic_module", func(t *testing.T) {
		type cfg struct {
			DB       *dbModuleWrapper `inject:"database"`
			ConnInfo string           `inject:"conn"`
		}

		injector := inject.New()
		injector.Bind(&dbModuleWrapper{}, "database")
		in := &cfg{}
		want := &cfg{
			DB:       &dbModuleWrapper{Host: "localhost", Port: 5432},
			ConnInfo: "localhost:5432",
		}
		c := config.New(injector, new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("module_install_error", func(t *testing.T) {
		type cfg struct {
			Mod errorModuleWrapper `inject:"bad"`
		}
		injector := inject.New()
		injector.Bind(&errorModuleWrapper{}, "bad")
		in := &cfg{}
		c := config.New(injector)
		testingx.Call(t, c.Assign, in).Error()
	})

	t.Run("module_chaining", func(t *testing.T) {
		// First module provides a prefix, second module uses it to provide a greeting
		type cfg struct {
			First   prefixModuleWrapper   `inject:"prefix-mod"`
			Second  greetingModuleWrapper `inject:"greeting-mod"`
			Message string                `inject:"message"`
		}
		injector := inject.New()
		injector.Bind(&prefixModuleWrapper{}, "prefix-mod")
		injector.Bind(&greetingModuleWrapper{}, "greeting-mod")
		in := &cfg{}
		want := &cfg{
			First:   prefixModuleWrapper{Prefix: "hello"},
			Second:  greetingModuleWrapper{Greeting: "hello world"},
			Message: "hello world",
		}
		c := config.New(injector, new(defaults.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("module_with_env", func(t *testing.T) {
		type cfg struct {
			Mod      envModuleWrapper `inject:"env-mod"`
			ConnInfo string           `inject:"env-conn"`
		}
		t.Setenv("TEST_MODULE_HOST", "remotehost")
		injector := inject.New()
		injector.Bind(&envModuleWrapper{}, "env-mod")
		in := &cfg{}
		want := &cfg{
			Mod:      envModuleWrapper{Host: "remotehost"},
			ConnInfo: "remotehost:configured",
		}
		c := config.New(injector, new(env.Source))
		testingx.Call(t, c.Assign, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})
}

// Test module types — these must be defined at package level for reflect to work.

type dbModuleWrapper struct {
	Host string `default:"localhost"`
	Port int    `default:"5432"`
}

func (m *dbModuleWrapper) Install(s *inject.Source) error {
	s.Bind(fmt.Sprintf("%s:%d", m.Host, m.Port), "conn")
	return nil
}

type errorModuleWrapper struct{}

func (m *errorModuleWrapper) Install(s *inject.Source) error {
	return fmt.Errorf("install failed")
}

type prefixModuleWrapper struct {
	Prefix string `default:"hello"`
}

func (m *prefixModuleWrapper) Install(s *inject.Source) error {
	s.Bind(m.Prefix, "prefix")
	return nil
}

type greetingModuleWrapper struct {
	Greeting string `inject:"prefix"`
}

func (m *greetingModuleWrapper) Install(s *inject.Source) error {
	msg := m.Greeting + " world"
	m.Greeting = msg
	s.Bind(msg, "message")
	return nil
}

type envModuleWrapper struct {
	Host string `env:"TEST_MODULE_HOST"`
}

func (m *envModuleWrapper) Install(s *inject.Source) error {
	s.Bind(m.Host+":configured", "env-conn")
	return nil
}

func TestConfigFind(t *testing.T) {
	t.Run("all_sources_reported", func(t *testing.T) {
		type cfg struct {
			Host string `default:"localhost" env:"APP_HOST" flag:"host" required:"--host"`
		}
		in := &cfg{}
		c := config.New(
			flags.NewSource(nil),
			new(env.Source),
			new(defaults.Source),
			new(required.Source),
		)
		testingx.Call(t, c.Find, in).
			Equal(map[string]map[string]interface{}{
				"Host": {
					"defaults": "localhost",
					"env":      "APP_HOST",
					"flags": flags.Flag{
						Name: "host",
						Type: reflect.TypeFor[string](),
					},
					"required": "--host",
				},
			}, diff.WithDiffer(new(reflectx.TypeDiffer)),
				diff.WithRenderOpts(render.NewOpts(t, render.WithRenderer(new(reflectx.TypeRenderer)))))
	})

	t.Run("nested_struct_find", func(t *testing.T) {
		type database struct {
			Host string `default:"localhost" env:"DB_HOST"`
		}
		type cfg struct {
			Database database
		}
		in := &cfg{}
		c := config.New(new(env.Source), new(defaults.Source))
		testingx.Call(t, c.Find, in).
			Equal(map[string]map[string]interface{}{
				"Database.Host": {
					"defaults": "localhost",
					"env":      "DB_HOST",
				},
				"Database": {},
			})
	})
}
