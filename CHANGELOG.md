# v0.2.0

## FEATURES

- `config.Initializer`: an optional interface for sources needing the full `Config` before it is used. `config.New` calls `Init` on each implementing source, in source order
- `config.Must`: `config.New` for callers that treat a failed source as unrecoverable
- `inject` package: `Register` for registering modules to be installed by `config.New`
- `inject` package: `Install` for installing modules immediately from within another module's `Install`, so a module can install a dependency and use what it bound; transitive self-installation is reported as a cycle rather than overflowing the stack
- `inject` package: `Get` and `GetTag` for reading a bound value out of an injector by type or by tag

<!--
## IMPROVEMENTS
Enhancements to existing functionality.
-->

<!--
## BUG FIXES
Issues that have been resolved.
-->

<!--
## SECURITY
Vulnerabilities or security-related changes addressed in this release.
-->

<!--
## DEPRECATIONS
Functionality that will be removed in a future release.
-->

## BREAKING CHANGES

- `config.New` now returns `(*Config, error)` so that it can initialize sources implementing `config.Initializer`
- `config.Source`'s `Find` and `Assign` no longer take a `*config.Config`; no source used it, and a source that needs the `Config` should implement `config.Initializer` instead
- `inject` modules are no longer installed by appearing as a struct field; they are registered with `Register` and installed during `config.New`, before any struct is assigned. Registration order replaces struct field order as dependency order, and a registered module is no longer bound for injection

## UPGRADE NOTES

- Replace `c := config.New(...)` with `c, err := config.New(...)`, or with `c := config.Must(...)` to keep a single-value call
- Custom `config.Source` implementations must drop the leading `cfg *config.Config` parameter from `Find` and `Assign`
- Modules that were fields on a config struct move to `injector.Register(&MyModule{})`, ordered by dependency. Drop the field unless something reads it; to read a module's state afterwards, keep the pointer you registered
- Test and startup code that set environment variables or flag values between `config.New` and `Assign` must now do so before `config.New`, since module fields are populated during construction