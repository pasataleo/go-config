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
- `cfg.AdditionalTargets()` — the structs the sources bring along themselves, ready to pass to `Find` or `Assign`

A source that also implements `config.DynamicSource` is given an `Enter` on the way down, before a field's children are walked, and returns the `Source` to stand in for itself over that field and everything below it. That is how `inject` scopes bindings to the struct they were installed in.

A source that implements `config.Starter` is given a `Start` at the beginning of every `Assign`, before any field is visited. It is called on whichever `Source` will assign the walk's fields — for a `DynamicSource` that is the scope its own `Enter` opened, which is what makes the work belong to that one walk. That is where `inject` installs its registered modules, so what they bind goes with the walk. A source that is not a `DynamicSource` has no scope to be called in, so it is called on itself and whatever it does there outlives the walk.

Sources share what they learn during an `Assign` through `cfg.Data()`, a `map[string]any` created by the outermost `Assign` and shared with every `Assign` nested inside it — the ones a source makes through the `Config` it was handed, as `inject` does to populate a module. Each source documents the keys it writes, and a reader has to cope with a key being absent, since the source that writes it may not be configured. `Data` is nil during `Find`.

A source that implements `config.Finisher` is given a `Finish` once per `Assign` the caller made, at the end of the outermost one. It is for checks that span fields, built up in `Data` as the walk hands the source one field at a time. Its errors join the walk's own, and it runs even when the walk failed, so a problem spanning fields is reported alongside the ones found in them. That is where `oneof` checks its groups.

`Find` calls none of them, so it stays read-only.

A source implementing `config.AdditionalTargets` brings structs of its own to be populated, beyond the one passed to `Assign`. A module registered on an inject `Source` is such a struct: it holds flags and environment variables like any other, but nothing the caller owns points at it. `Find` does not walk them — it reports the one struct it is given, so a caller building help text calls `Find` on each additional target as well and merges:

```go
found := cfg.Find(&app)
for _, target := range cfg.AdditionalTargets() {
    maps.Copy(found, cfg.Find(target))
}
```

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
- `source.Register(modules...)` — queue root modules for every `Assign` to install
- `inject.Get[T](source)` / `inject.GetTag[T](source, tag)` — read a binding back without declaring a field, panicking if it is not there
- `inject.TryGet[T](source)` / `inject.TryGetTag[T](source, tag)` — the same for a lookup that is genuinely optional, reporting whether it was found

### `required`

Validates that fields marked with the `required` struct tag have been set to a non-zero value. Returns an error for any that are missing.

A field whose name the `oneof` source has put in a group is not failed on its own: it is recorded as unset in `cfg.Data()` under `oneof.UnsetKey`, and `oneof` decides. `required` works without `oneof`; a field with a `oneof` tag is then just required on its own.

### `oneof`

Makes required fields alternatives: fields tagged `oneof:"<group>"` form a group, and exactly one name in it must be set. It only works alongside `required`, which it depends on: every field in a group needs a `required` tag, and `required.Source` has to be configured too.

```go
type Server struct {
    Port   int    `flag:"port" required:"port" oneof:"listen"`
    Socket string `flag:"socket" required:"socket" oneof:"listen"`
}

c := config.New(
    &oneof.Source{},          // first, before anything claims a field
    flags.NewSource(values),
    &env.Source{},
    &defaults.Source{},
    &required.Source{},       // last, records which names were not set
)
```

Neither set fails with `config.MissingValue` ("required value not set: one of port, socket"), and both set fails with `config.ConflictingValues`. Set means the same as for a plain required field: a source provided it, so `--port 0` counts, or the struct already held a value.

A group's members are named by their `required` tag, and groups are global, as flag names are: a field joins its group from whichever struct it is in, including the modules `inject` populates with `Assign`s of their own, and a name read in several places is one member.

- `oneof` has to come before every source that claims fields, so it sees each field before one is claimed. It records each group's names in `cfg.Data()` under `oneof.Key`, and never claims a field itself.
- `required` is what records which names were not set, under `oneof.UnsetKey`. Without it, every name in a group looks set.
- `oneof` without `required` on a field is a programming mistake, and panics.
- `Find` reports a field's group, so help text can show the alternatives together.

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

There are two ways a module gets installed, and they are complementary:

- **Declared as a struct field.** Wiring that belongs to the shape of the config itself — a struct that always wants that module.
- **Registered on the injector with `source.Register(...)`.** Chosen at run time, before anything is walked, which is how a program swaps a real implementation for a fake one. Registration order is dependency order, and their bindings are in force for the whole `Assign`.

```go
injector := inject.New()
if testing {
    injector.Register(&TestDatabaseModule{})
} else {
    injector.Register(&DatabaseModule{})
}
```

`Register` belongs to wiring time and works only on the `Source` that `inject.New` built. Every other `Source` is a scope opened during an `Assign`, and registering into one panics — the module graph is fixed before a walk starts, so `Find` can report every flag, env var and usage string a program has without installing anything. A module that needs another module declares it as a field.

Registered modules install at the start of every `Assign`, into the scope that `Assign` opens, so they follow the same rule as everything else: their bindings last for that walk and no longer, and the next `Assign` installs them again from the same wiring. Nothing accumulates on the injector.

A module declared as a field is installed whether or not a binding claims it:

- **A field the walk reached itself** — `Mod DBModule`, with nothing bound to it. The walk allocated it and every source has already filled its fields in, so it goes straight to `Install`. The field is not reported as claimed, so the sources after `inject` still see it.
- **A field a binding claimed** — `Mod *DBModule \`inject:"database"\``. The instance came from outside the walk with nothing filled in, so its own fields are populated from the full config first, then `Install` runs. This is also the only way an interface-typed field can work, since the walk does not recurse into interfaces.

Either way, subsequent fields in the struct tree can use the new bindings.

A module reads what modules nested inside it bound through its own fields, which are populated in the scope those published into. `Install` is handed the enclosing scope — where the module publishes to, not where its children did.

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

#### Errors and panics

Who made the mistake decides how it is reported.

A value a user supplied comes back as an **error** — an unparseable flag, a missing environment variable, or a `Module.Install` that failed at runtime because the database was unreachable. The program is working and the input is not.

Wiring the injector up wrongly **panics** — a duplicate binding in the same scope, a tag nothing is bound to, a module that is not a pointer, a module that installs itself. None of those depend on the input, so they fail on the first run and every run after it, and an error return would only carry them further from the line that caused them.

The `inject` error codes (`NotBound`, `TypeMismatch`, `ModuleCycle`, `InvalidModule`) are used both ways: a panic carries the coded error as its value, so a `recover` can still ask what went wrong.

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
