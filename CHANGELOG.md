# v0.2.0

## FEATURES

- `config.DynamicSource`: an extension to `Source` for sources whose bindings change as the walk progresses. `Enter` is called on the way down, before a field's children are walked, and returns the `Source` to stand in for the receiver over that field, its children, and their own `Enter`. The field's siblings see what their shared parent returned
- `inject` bindings are lexically scoped. `Assign` opens a scope for every field it walks, and what a module installs lands in the scope of the struct that contains it: visible to that module's siblings and everything nested under them, invisible outside, and shadowing a binding of the same name from further out until the walk leaves
- `inject` error codes: `NotBound`, `TypeMismatch`, `ModuleCycle` and `InvalidModule`, so a caller can tell why a lookup or an install failed without matching on the message

## IMPROVEMENTS

- Nothing a module installs survives the `Assign` that installed it, so the same injector can be reused and a second `Assign` starts clean. There is nothing to tear down and nothing to leak: a scope stops being reachable when the walk leaves the subtree it was made for, on every path out, including the ones an error cuts short
- A module's `Install` sees what modules nested inside it bound, since those published into the scope it is handed
- `config.Assign(source, v)` is now `config.New(source).Assign(v)` rather than a second hand-rolled walk, so there is one place that knows how a walk is driven
- `inject.New` no longer allocates its maps up front; a scope that binds nothing costs a single small struct
- A failed assignment reports the type it could not assign and the path it was assigning to, as a `TypeMismatch` rather than a bare string

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

- A module's bindings no longer reach outside the struct that contains it. A module nested in a plain struct used to publish to the whole `Assign`, and now publishes to that struct alone
- A module's bindings no longer outlive the `Assign` that installed them. Code that called `Assign` once to wire an injector up and then relied on those bindings later will no longer find them
- `inject.Module.Install` is handed the scope the module was found in rather than the injector itself. The signature is unchanged, so implementations keep compiling

## UPGRADE NOTES

- A module whose bindings have to reach the level above must be hoisted out of the struct that contains it, or the enclosing module must re-`Bind` what it wants to expose
- A tag that collided across two unrelated modules before now shadows instead. Two modules binding the same tag at the *same* level still panic, which is where most collisions were