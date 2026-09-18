# v0.2.0

## FEATURES

- `config.DynamicSource`: an extension to `Source` for sources whose bindings change as the walk progresses. `Enter` is called on the way down, before a field's children are walked, and returns the `Source` to stand in for the receiver over that field, its children, and their own `Enter`. The field's siblings see what their shared parent returned
- `inject` bindings are lexically scoped. `Assign` opens a scope for every field it walks, and what a module installs lands in the scope of the struct that contains it: visible to that module's siblings and everything nested under them, invisible outside, and shadowing a binding of the same name from further out until the walk leaves
- `inject` error codes: `NotBound`, `TypeMismatch`, `ModuleCycle` and `InvalidModule`, so a caller can tell why a lookup or an install failed without matching on the message
- A module reached from its own subtree panics with a `ModuleCycle` code naming the field it was found at. It used to recurse until it ran out of memory, since populating a module's fields walks it afresh and nothing was watching for a module it had already started
- A module instance reached twice in one `Assign` - by two fields bound to it, say - installs once. The record is per walk, so a later `Assign` installs it again

## IMPROVEMENTS

- Nothing a module installs survives the `Assign` that installed it, so the same injector can be reused and a second `Assign` starts clean. There is nothing to tear down and nothing to leak: a scope stops being reachable when the walk leaves the subtree it was made for, on every path out, including the ones an error cuts short
- A module reads what modules nested inside it bound through its own fields, which are populated in the scope those published into. `Install` itself is handed the enclosing scope, which is where the module publishes to rather than where its children did
- A module field the walk reached itself is installed, where before only a field a binding claimed was. The field is not reported as claimed, so the sources after `inject` still see it
- `inject.Get[T]` and `inject.GetTag[T]` read a binding back without declaring a field for it, matching exactly as an untagged or tagged field of that type would. They resolve against the scope of the `Source` they are given, so inside an `Install` they see what is in scope where the module was found, shadowing included
- `inject.TryGet[T]` and `inject.TryGetTag[T]` are the same for a lookup that is genuinely optional, reporting whether the binding was there instead of panicking. A binding that exists but cannot be assigned to `T` panics either way
- `inject.Source.Register` queues root modules, so a program can pick its wiring at run time - a real implementation or a fake one - rather than from the shape of the struct being assigned. Registration order is dependency order
- `config.Starter`: an optional source interface called at the start of every `Assign`, before any field is visited, on the `Source` that will assign the walk's fields. For a `DynamicSource` that is the scope its own `Enter` opened, which is how the work belongs to the one walk. `inject` installs its registered modules there, so their bindings last for that `Assign` and the next one installs them again from the same wiring. `Find` never calls it
- `Register` works only on the `Source` that `inject.New` built. Every other is a scope opened during an `Assign`, and registering into one panics, so the module graph is fixed before a walk starts
- `config.AdditionalTargets`: an optional source interface for structs a source brings along itself, and `Config.AdditionalTargets` to collect them. A registered inject module is one: it holds flags and environment variables like any other struct, but nothing the caller owns points at it, so without this there was no way to report its fields. `Find` does not walk them - a caller building help text calls `Find` on each and merges
- `config.Assign(source, v)` is now `config.New(source).Assign(v)` rather than a second hand-rolled walk, so there is one place that knows how a walk is driven
- `inject.New` no longer allocates its maps up front; a scope that binds nothing costs a single small struct
- A failed assignment reports the type it could not assign and the path it was assigning to, as a `TypeMismatch` rather than a bare string
- A panic raised while a module is installing - a duplicate binding in its `Install`, say - names the module and where it was found, rather than only what collided
- A zero-sized module is exempt from install dedupe. Go may put every zero-size allocation at the same address, so two distinct empty modules are indistinguishable; installing both means a genuine double registration still collides loudly instead of being silently dropped
- A module is taken through its field's address wherever there is one, so a module whose `Install` has a value receiver is built and populated rather than rejected for not being a pointer
- An interface-typed field installs whatever module a binding put in it. `reflectx.Walk` does not recurse into interfaces, so this is the only way such a field can work
- A nil module field installs nothing instead of panicking
- A module with no address behind it - a value a binding put in an interface field, which cannot be populated - panics with an `InvalidModule` code rather than failing somewhere deeper in the walk. Wiring the injector up wrongly panics, as a duplicate binding already did; errors are for values a user supplied

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