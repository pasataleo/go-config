# go-config

A struct-based configuration library for Go. Populate struct fields from multiple sources (flags, environment variables, defaults, dependency injection) using struct tags, with priority-based resolution.

## Installation

```sh
go get github.com/pasataleo/go-config
```

## Usage

Define a config struct with tags, then use `config.New` to compose sources in priority order:

```go
type Config struct {
    Host    string `flag:"host" env:"HOST" default:"localhost" required:"host" usage:"server hostname"`
    Port    int    `flag:"port" env:"PORT" default:"8080" usage:"server port"`
    Verbose bool   `flag:"verbose" hidden:"" usage:"enable verbose logging"`
}

var cfg Config
args, values := flags.Parse(os.Args[1:])

c := config.Must(
    flags.NewSource(values),  // highest priority
    &env.Source{},
    &defaults.Source{},
    &required.Source{},       // validation (runs last)
)

if err := c.Assign(&cfg); err != nil {
    log.Fatal(err)
}
```

Sources are tried in order — the first source that provides a value for a field wins.

## Packages

### `config`

Core package. Defines the `Source` interface and the `Config` type that composes sources with priority ordering.

- `config.New(sources ...Source) (*Config, error)` — create a config with ordered sources
- `config.Must(sources ...Source) *Config` — the same, panicking instead of returning an error
- `config.Find(source, v)` — query a single source for all fields
- `config.Assign(source, v)` — populate fields from a single source
- `config.FilterBySource[T](found, name)` — filter `Find` results by source name

`New` returns an error because sources may need to initialize. A source that implements `config.Initializer` has its `Init(cfg *Config)` called by `New`, in source order, once the full source list is known — this is how `inject` installs its modules.

### `flags`

Parses CLI flags and populates fields via the `flag` struct tag.

- Supports `--flag value`, `--flag=value`, `-f value`
- Boolean negation with `--no-flag`
- `--` terminates flag parsing
- `flags.Parse(args)` — parse arguments into remaining args and flag values
- `flags.NewSource(values)` — create a source from parsed values
- `source.Validate(flags)` — check for unknown flags

### `env`

Populates fields from environment variables via the `env` struct tag.

### `defaults`

Sets default values via the `default` struct tag. Skips fields that already have a non-zero value.

### `inject`

Dependency injection for struct fields, matched by `inject` struct tag or by type.

- `source.Bind(value, tags...)` — bind a static value
- `source.BindFunc(fn, tags...)` — bind a factory function that receives the current field value
- `inject.Get[T](source)` — read the value bound to `T`
- `inject.GetTag[T](source, tag)` — read the value bound to a tag

### `required`

Validates that fields marked with the `required` struct tag have been set to a non-zero value. Returns an error for any that are missing.

### `hidden`

Marks fields as hidden via the `hidden` struct tag. Metadata-only — does not assign values. Useful for filtering fields out of help text.

### `usage`

Extracts usage descriptions via the `usage` struct tag. Metadata-only — does not assign values. Useful for generating help text.

### `inject` — Modules

Modules extend the inject system to support swappable, self-configuring components. A module is a struct that implements `inject.Module`:

```go
type Module interface {
    Install(s *Source) error
}
```

Register modules on the injector and `config.New` installs them all before anything else touches the config. Installing a module:

1. Populates the module's own fields using the full config — flags, env vars, defaults, and bindings from modules installed before it.
2. Calls `module.Install(s)` so the module can register new bindings.

Registration order is dependency order: a module can use bindings registered by modules registered before it.

#### Example

```go
type DatabaseModule struct {
    Host string `env:"DB_HOST"`
    Port int    `env:"DB_PORT"`
}

func (m *DatabaseModule) Install(s *inject.Source) error {
    db, err := sql.Open("postgres", fmt.Sprintf("%s:%d", m.Host, m.Port))
    if err != nil {
        return err
    }
    s.Bind(db)
    return nil
}

type MyHandler struct {
    DB *sql.DB // injected by DatabaseModule.Install
}

type App struct {
    Handler *MyHandler // receives *sql.DB via inject
}

injector := inject.New()
injector.Register(&DatabaseModule{})

c := config.Must(injector, &env.Source{}, &defaults.Source{})
c.Assign(&app)
```

Registering a module does not bind it, so the module itself is not injectable. Keep your own pointer to it if you need to read its state afterwards, or have it call `s.Bind(m)` from within `Install`.

#### Modules installing modules

A module can install further modules from inside its own `Install`. Those complete before `Install` continues, so the parent can read what they bound:

```go
func (m *ServerModule) Install(s *inject.Source) error {
    if err := s.Install(&DatabaseModule{}); err != nil {
        return err
    }

    db, err := inject.Get[*sql.DB](s)
    if err != nil {
        return err
    }

    s.Bind(NewServer(db))
    return nil
}
```

`Register` and `Install` cover the two phases and each panics if used in the other's: `Register` queues a module for the next `config.New`, `Install` installs one immediately and is only valid while modules are installing. A module that transitively installs itself is reported as a cycle rather than overflowing the stack.

Note that installing a module populates its fields from every source, so pre-setting a field in code (`&Child{Host: "x"}`) is not reliable — env, flags and inject will all overwrite it. Configure children through bindings instead.

#### Swappability

Swapping implementations is a matter of registering a different module:

```go
// Production
injector := inject.New()
injector.Register(&DatabaseModule{})

// Test
injector := inject.New()
injector.Register(&TestDatabaseModule{})
```

Both implement `Module`. The test module can provide mock bindings without needing real env vars or flags.
