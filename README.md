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
- `inject.Get[T](source)` / `inject.GetTag[T](source, tag)` — read a binding back without declaring a field, panicking if it is not there
- `inject.TryGet[T](source)` / `inject.TryGetTag[T](source, tag)` — the same for a lookup that is genuinely optional, reporting whether it was found

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

A module is installed whether or not a binding claims its field:

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

## Planned: the rest of the features from `liamcervante/pause`

> Design notes for work not yet done. Everything below is **not** in the library today. Delete
> this section once it is, the same way the earlier `Planned:` notes went.

This branch rewound to before `main`'s `Register`/`Initializer` shape and rebuilt module
bindings as lexical scopes instead. `main` and `liamcervante/pause` still hold features worth
having, and this is what is left to bring across. Read them with
`git show liamcervante/pause:pkg/inject/source.go` — but take the behaviour, not the shape:
they were written against two flat binding frames, which the scope chain replaces.

Done so far: the scoping itself, typed error codes, module validation through the field's
address, the cycle guard with per-walk install dedupe, installing module fields the walk
reached itself, and `Get`/`TryGet`.

### Two ways to install a module, and both are wanted

The two are complementary and neither replaces the other:

- **Declared as a struct field.** The walk finds it, populates it and installs it. This is for
  a module a struct always wants — wiring that belongs to the shape of the config itself. Done.
- **Registered up front on the injector.** Chosen at run time, before anything is walked, which
  is how a program swaps a real implementation for a fake one. These are the program's roots
  and their bindings are global. Feature 1 below.

```go
injector := inject.New()
if testing {
    injector.Register(&TestDatabaseModule{})
} else {
    injector.Register(&DatabaseModule{})
}
```

`Register` belongs to wiring time only, and must be unavailable once a walk starts — see
feature 1.

### The invariant that decides panic from error

Written down in `pkg/inject/errors.go`, and it governs everything below. A value a user
supplied — an unparseable flag, a missing environment variable — comes back as an **error**. A
mistake in the wiring — a duplicate binding, a tag nothing is bound to, a module that is not a
pointer, a module cycle — **panics**, because it fails on the first run and every run after it.
`Module.Install` returning an error stays an error: `sql.Open` failing is a runtime failure,
not a wiring fault.

### 1. `Register` and `config.Initializer`

Register modules up front, outside the walk:

```go
injector := inject.New()
injector.Register(&DatabaseModule{})
cfg, err := config.New(injector, new(env.Source))
```

`config.Initializer` is an optional source interface (`Init(cfg *Config) error`) that
`config.New` calls in source order once the full source list is known. `inject.Source.Init`
installs everything queued by `Register`. Modules registered after `Init` returns queue for a
subsequent one.

`config.New` becomes `(*Config, error)`, with `Must` for callers that treat a failed source as
fatal at startup. The error is needed for `Install` failures, not for wiring faults.

#### `Register` only works on the `Source` that `New` built

The module graph is fixed before anything is walked. That is what lets `Find` report every
flag, env var and usage string a program has without running an `Install`, and it is why a
module that needs another module declares it as a field rather than registering one mid-flight.

Rather than track whether a walk is in progress, make it structural: **`Register` panics on any
`Source` with a parent.** Only the one `New` built has none; every other is a frame `Enter`
opened during an `Assign`, and those can never register. No counter, no flag, and nothing on
the `Source` becomes mutable — so concurrent `Assign`s stay safe by construction.

This closes the path that matters, because user code is never handed the root `Source` during a
walk. `install` calls `mod.Install(s.parent)` where `s` is the field's own frame, so the
shallowest thing an `Install` can receive is the walk's root frame — whose parent is the root
`Source`, not the root itself. A `BindFunc` factory receives only a `reflect.Value` and has no
`Source` at all.

The gap to write down: this keys on the object, not the moment. Closing over the `injector`
variable and calling `Register` from inside an `Install` still gets through. Pause's `busy`
counter caught that; this does not. Add the counter if it ever bites — the frame rule is the
cheap 95%.

It fits the scope chain better than it fitted pause. A registered root's `Install` is handed
the injector itself, so its bindings are permanent and global; anything nested inside it lands
in the scope `Init`'s own `Assign` opened and dies when `Init` returns. Pause needed an
`initializing` flag and a `local()` predicate to force that, and both disappear.

That does leave one place `Register` is still reachable, and it is the deliberate one: a
registered root's `Install` holds the root `Source`, because that is what makes its bindings
global. **Decide what that means.** Either `Init` drains `pending` in a loop, so a root module
registering further modules is a supported way to build a graph at wiring time — the install
dedupe already stops one registering itself forever — or registering there is rejected. What it
must not do is silently queue for an `Init` that never comes, which is what falls out if
`Init` drains once.

**State the consequence in the docs:** a registered root's grandchildren are not global. A root
that wants to expose what a child module bound has to re-`Bind` it:

```go
type DatabaseModule struct {
    Pool *PoolModule // binds *sql.DB into DatabaseModule's scope
}

func (m *DatabaseModule) Install(s *inject.Source) error {
    s.Bind(m.conn) // -> global, the app sees this
    return nil
}
```

### 2. `config.AdditionalTargets`

An optional source interface returning structs the source brings along itself, so a caller can
`Find` their fields. Mostly exists *because* of `Register`: once a module is registered rather
than declared as a field, its flags and env vars are not reachable from the caller's struct,
and help text silently loses them. `inject.Source` returns every registered module, installed
or not — the answer is the same before and after `Init`, which is the point.

`Find` deliberately does not walk them. It reports the one struct it is given; a caller that
wants the program's whole field surface calls `Find` on each additional target and merges.

### Do not bring these across

- `Scoper`, `Scope()`, `localByTag`/`localByType`, `local()`, `tagDest()`, `typeDest()` — the
  two-frame model the scope chain replaces.
- `AssignNested` as pause has it. It existed to skip the lock and the scope boundary;
  `cfg.Assign(mod)` already does the right thing, because the `cfg` a source is handed is
  already scoped.
- The `sync.RWMutex` concurrency guard. It was needed because `Assign` mutated the `Source`;
  it no longer does, so concurrent `Assign`s are safe by construction. If `Register`/`Init`
  wants a guard, scope it to those and the install stack rather than the whole `Config`.
- **Dropping `cfg *Config` from `Source.Find` and `Source.Assign`.** `main` and pause removed
  it in favour of `Initializer` holding the `Config`. That parameter is how the scope reaches
  `inject` now, so it stays. `Initializer` can come across without it.

### Finally

Once this branch has parity, **overwrite `main` with it** — `main`'s `Register`/`Initializer`
shape is superseded, so this is a replacement rather than a merge — and delete
`liamcervante/pause`. Then delete this section.

Check `go-plugin` and `go-command` before that lands: both pin the old `go-config` and use
`config.Must`, which does not exist on this branch.
