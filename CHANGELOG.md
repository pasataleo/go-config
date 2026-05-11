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

<!--
## BREAKING CHANGES
Changes that are not backwards compatible and require updates from consumers.
-->

<!--
## UPGRADE NOTES
Steps required when upgrading from a previous version.
-->
