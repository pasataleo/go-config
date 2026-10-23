# v0.2.0

## FEATURES

- `config.DynamicSource`, for sources whose bindings change as the walk goes down: `Enter` returns the source to use for a field and everything below it.
- `config.Starter`, an optional source interface called at the start of every `Assign`.
- `config.Finisher`, an optional source interface called at the end of the outermost `Assign`.
- `config.AdditionalTargets`, for structs a source brings along itself, such as registered inject modules.
- `Config.Data`, a map sources share during one `Assign`, including the `Assign`s nested inside it.
- `config.ConflictingValues` error code.
- `oneof` source: required fields tagged `oneof:"<group>"` are alternatives, and exactly one in each group must be set.
- `inject.Source.Register` queues modules to install at the start of every `Assign`.
- `inject.Get`, `inject.GetTag`, `inject.TryGet` and `inject.TryGetTag` read a binding back without a field.
- `inject` error codes: `NotBound`, `TypeMismatch`, `ModuleCycle` and `InvalidModule`.

## IMPROVEMENTS

- `inject` bindings are scoped to the struct that installed them, and shadow bindings from further out.
- A module field the walk reaches is installed, not only one a binding claimed.
- A module can be installed through an interface-typed field, or through its field's address when `Install` has a value receiver.
- A module instance reached twice in one `Assign` is installed once.
- A nil module field is skipped instead of panicking.
- `config.Assign` uses `Config.Assign` rather than its own walk.
- `inject.New` no longer allocates its maps up front.
- Failed assignments and install panics name the type, module and path involved.

## BUG FIXES

- The `flags` source no longer initialises pointer fields when no flag value has been provided.
- A module that installs itself panics with `ModuleCycle` instead of recursing until it runs out of memory.

<!--
## SECURITY
Vulnerabilities or security-related changes addressed in this release.
-->

<!--
## DEPRECATIONS
Functionality that will be removed in a future release.
-->

## BREAKING CHANGES

- A module's bindings are visible only within the struct that contains it.
- A module's bindings no longer outlive the `Assign` that installed them.
- `inject.Module.Install` is given the scope the module was found in rather than the injector.
- `inject.Source.Register` panics on any source other than the one `inject.New` returned.

## UPGRADE NOTES

- Hoist a module whose bindings must be seen outside its struct, or re-`Bind` them from the enclosing module.
- A tag bound by two modules at different levels now shadows instead of panicking.

# v0.1.0

## FEATURES

- `config.New` for composing multiple sources with priority ordering; first source to provide a value wins
- `Source` interface for implementing custom configuration sources
- `flags` package: CLI flag parsing with `--flag value`, `--flag=value`, `-f value`, boolean negation via `--no-flag`, and `--` termination; `Validate` for detecting unknown flags
- `env` package: environment variable population via `env` struct tag
- `defaults` package: default value population via `default` struct tag; skips fields with non-zero values
- `inject` package: dependency injection by tag or type with `Bind` and `BindFunc`; `Module` interface for self-configuring components that register new bindings after their own fields are populated
- `required` package: validation that fields marked with the `required` tag are non-zero after all sources have run
- `hidden` package: metadata source for marking fields as hidden via the `hidden` tag
- `usage` package: metadata source for extracting field descriptions via the `usage` tag
