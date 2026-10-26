# v0.2.0

## FEATURES

- Optional source interfaces: `DynamicSource` scopes a source to a field and everything below it, `Starter` and `Finisher` run at the start and end of an `Assign`, and `AdditionalTargets` reports structs a source brings along, with `Config.Data` for sources to share state during one `Assign`.
- `oneof` source: required fields tagged `oneof:"<group>"` are alternatives, and exactly one in each group must be set, failing with the new `config.ConflictingValues` when several are.
- `inject.Source.Register` queues modules to install at the start of every `Assign`.
- `inject.Get`, `inject.GetTag`, `inject.TryGet` and `inject.TryGetTag` read a binding back without a field.
- `inject` error codes: `NotBound`, `TypeMismatch`, `ModuleCycle` and `InvalidModule`.

## IMPROVEMENTS

- `inject` bindings are scoped to the struct that installed them, and shadow bindings from further out.
- A module is installed wherever the walk reaches it, once per `Assign`: through a binding, an interface-typed field, or a value-receiver `Install`, with a nil module field skipped.
- `config.Assign` honours `DynamicSource`, `Starter` and `Finisher`, as `Config.Assign` does.
- Failed assignments and install panics name the type, module and path involved.

## BUG FIXES

- The `flags` source no longer initialises pointer fields when no flag value has been provided.
- A module that installs itself panics with `ModuleCycle` instead of recursing until it runs out of memory.
- `inject:""` matches by type and requires a binding, rather than looking for a binding tagged with the empty string.
- `Find` strips `;optional` from an `inject` tag before looking up the binding.

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
- Call `Register` on the injector `inject.New` returned, before the first `Assign`; a module that needs another module declares it as a field rather than registering it from `Install`.
- A module that keeps the `Source` its `Install` was given must not bind into it after that `Assign` returns: the scope ends with it.

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
