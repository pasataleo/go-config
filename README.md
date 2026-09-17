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

A source that also implements `config.DynamicSource` is given an `Enter` on the way down, before a field's children are walked, and returns the `Source` to stand in for itself over that field and everything below it. That is how `inject` scopes bindings to the struct they were installed in. `Find` never calls `Enter`, so it stays read-only.

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

Struct field order is dependency order within a scope — earlier modules' bindings are available to later modules and fields at the same level.

#### Binding scopes

Bindings are lexically scoped. `Assign` opens a scope for every field it walks, and what a module installs lands in the scope of the struct that *contains* it:

- It is visible to the module's siblings in that struct, and to everything nested under them.
- It is **not** visible outside that struct.
- It **shadows** a binding of the same name from further out, and the outer binding is back in force once the walk leaves.
- Two bindings of the same name in the *same* scope are still a collision, and still panic.

```go
type App struct {
    Plain struct {
        Mod  DBModule // binds "conn"
        Peer string   `inject:"conn"` // resolved: a sibling of Mod
    }
    Conn string `inject:"conn"` // NOT resolved: "conn" died with Plain's scope
}
```

Plain structs are not transparent — every struct is a scope, so a module that needs to publish to the level above has to live at that level. The same rule is what keeps a walk from leaving anything behind: nothing a module installs survives the `Assign` that installed it, so the same injector can be reused and a second `Assign` starts clean.

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
