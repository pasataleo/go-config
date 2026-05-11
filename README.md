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

c := config.New(
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

- `config.New(sources ...Source)` — create a config with ordered sources
- `config.Find(source, v)` — query a single source for all fields
- `config.Assign(source, v)` — populate fields from a single source
- `config.FilterBySource[T](found, name)` — filter `Find` results by source name

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

### `required`

Validates that fields marked with the `required` struct tag have been set to a non-zero value. Returns an error for any that are missing.

### `hidden`

Marks fields as hidden via the `hidden` struct tag. Metadata-only — does not assign values. Useful for filtering fields out of help text.

### `usage`

Extracts usage descriptions via the `usage` struct tag. Metadata-only — does not assign values. Useful for generating help text.

### `inject` — Modules

Modules extend the inject system to support swappable, self-configuring components. A module is a struct that implements `inject.Module` — after the framework populates the module's own fields (env vars, flags, injected values from earlier modules), it calls `Install`, which registers new bindings on the inject source.

```go
type Module interface {
    Install(s *Source) error
}
```

When `inject.Source.Assign` assigns a value to a field and the concrete value implements `Module`, it:

1. Populates the module's own fields using the full config (all sources).
2. Calls `module.Install(s)` so the module can register new bindings.
3. Returns — subsequent fields in the struct tree can use the new bindings.

Struct field order is dependency order — earlier modules' bindings are available to later modules and fields.

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
    DB      *DatabaseModule `inject:"database"` // populated first, Install binds *sql.DB
    Handler *MyHandler                          // populated second, receives *sql.DB via inject
}

injector := inject.New()
injector.Bind(&DatabaseModule{}, "database")

c := config.New(injector, &env.Source{}, &defaults.Source{})
c.Assign(&app)
```

#### Swappability

Modules are bound by tag (or type), so swapping implementations is straightforward:

```go
// Production
injector := inject.New()
injector.Bind(&DatabaseModule{}, "database")

// Test
injector := inject.New()
injector.Bind(&TestDatabaseModule{}, "database")
```

Both implement `Module`. The test module can provide mock bindings without needing real env vars or flags.
