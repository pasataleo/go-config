package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

// Source provides values for struct fields from a particular source (e.g.
// environment variables, flags, config files). Implementations are called for
// every exported field in the struct, including struct-typed fields, and should
// handle struct-typed fields gracefully (e.g. return nil/false).
type Source interface {
	Name() string
	Find(cfg *Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error)
	Assign(cfg *Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error)
}

// DynamicSource is an extension to the source interface in which bindings and
// tags are changed as the walk progresses. Enter is called on the way down,
// before a field's children are walked, and returns the Source that stands in
// for the receiver over that field: the field's own Assign sees it, so do the
// field's children, and so does the Enter of each of them. The field's siblings
// see what their shared parent returned. Returning the receiver leaves the
// subtree unchanged.
//
// As each walk should be idempotent, and not carry side effects, there is
// nothing to tear down. The walk holds no state of its own, so a scoped Source
// stops being reachable the moment the walk leaves the subtree it was made for,
// on every path out - including the ones an error cuts short.
//
// Enter reaches every DynamicSource, in source order. It is not subject to
// claiming the way Assign is, and only Assign consults it: Find never opens a
// scope, so it stays read-only.
type DynamicSource interface {
	Source
	Enter(cfg *Config, path reflectx.Path) (Source, error)
}

// Config holds an ordered list of sources for populating struct fields.
// Sources are consulted in order, with earlier sources taking priority.
type Config struct {
	sources []Source
}

// New creates a Config with the given sources. Sources are tried in order
// during Assign, so earlier sources take priority.
func New(sources ...Source) *Config {
	return &Config{
		sources: sources,
	}
}

// enter derives the Config for one field from the Config of the field above it,
// giving every DynamicSource the chance to stand something else in for itself
// over that field and everything below it.
func (config *Config) enter(path reflectx.Path) (*Config, error) {
	scoped := &Config{
		sources: make([]Source, len(config.sources)),
	}

	var errs error
	for i, source := range config.sources {
		scoped.sources[i] = source

		dynamic, ok := source.(DynamicSource)
		if !ok {
			continue
		}

		next, err := dynamic.Enter(config, path)
		if err != nil {
			errs = errorsx.Append(errs, err)
			continue
		}
		if next != nil {
			scoped.sources[i] = next
		}
	}
	return scoped, errs
}

// Find queries all sources for every exported field in v and returns a map
// from field path to source name to the value that source would provide.
// Unlike Assign, all sources are queried regardless of priority.
//
// Returned value is path -> tag -> value.
//
// Note: Find will initialize nil pointer fields in v as a side effect of
// traversal.
func (config *Config) Find(v interface{}) map[string]map[string]interface{} {
	found := make(map[string]map[string]interface{})
	err := reflectx.Walk(v, nil, func(path reflectx.Path, value reflect.Value, field reflect.StructField, _ interface{}) error {
		var errs error
		fs := map[string]interface{}{}
		for _, source := range config.sources {
			if found, err := source.Find(config, path, value, field); err != nil {
				errs = errorsx.Append(errs, err)
				continue
			} else if found != nil {
				fs[source.Name()] = found
			}
		}
		found[path.String()] = fs
		return errs
	})
	if err != nil {
		// all errors returned by Find functions should be considered programmer errors
		errs := errorsx.Errors(err)
		msgs := make([]string, 0, len(errs))
		for _, err := range errs {
			msgs = append(msgs, fmt.Sprintf("%-v", err))
		}
		panic(strings.Join(msgs, "\n"))
	}
	return found
}

// Assign populates the exported fields of v by trying sources in order.
// For each field, the first source that claims it (returns true) wins, and
// remaining sources are skipped.
//
// Each field is assigned by the Config a DynamicSource opened for it, so a
// source that binds something while assigning one field publishes it only as
// far as the scope it is in. Assign opens a scope of its own around the whole
// walk: a source that binds something at the top level of v is still writing
// into the walk, not into the sources New was given, so nothing it does here
// outlives the call.
func (config *Config) Assign(v interface{}) error {
	root, err := config.enter(nil)
	if err != nil {
		return err
	}

	walker := reflectx.Walker{
		Enter: func(path reflectx.Path, _ reflect.Value, _ reflect.StructField, data interface{}) (interface{}, error) {
			return data.(*Config).enter(path)
		},
		Visit: func(path reflectx.Path, value reflect.Value, field reflect.StructField, data interface{}) error {
			return data.(*Config).assign(path, value, field)
		},
	}
	return walker.Walk(v, root)
}

// assign offers one field to each of the config's sources in turn, and is where
// priority is applied: the first source to claim the field wins and the rest do
// not see it.
func (config *Config) assign(path reflectx.Path, value reflect.Value, field reflect.StructField) error {
	var errs error
	for _, source := range config.sources {
		f, err := source.Assign(config, path, value, field)
		if err != nil {
			errs = errorsx.Append(errs, err)
		}
		if f {
			break // stop once a source has claimed the value
		}
	}
	return errs
}

// Find queries a single source for every exported field in v and returns a map
// from field path to the value that source would provide.
//
// Note: Find will initialize nil pointer fields in v as a side effect of
// traversal.
func Find(source Source, v interface{}) map[string]interface{} {
	cfg := New(source)
	found := map[string]interface{}{}
	err := reflectx.Walk(v, nil, func(path reflectx.Path, value reflect.Value, field reflect.StructField, _ interface{}) error {
		if f, err := source.Find(cfg, path, value, field); err != nil {
			return err
		} else if f != nil {
			found[path.String()] = f
		}
		return nil
	})
	if err != nil {
		// all errors returned by Find functions should be considered programmer errors
		errs := errorsx.Errors(err)
		msgs := make([]string, 0, len(errs))
		for _, err := range errs {
			msgs = append(msgs, fmt.Sprintf("%-v", err))
		}
		panic(strings.Join(msgs, "\n"))
	}
	return found
}

// Assign populates the exported fields of v using a single source.
func Assign(source Source, v interface{}) error {
	return New(source).Assign(v)
}

func FilterBySource[T any](found map[string]map[string]interface{}, want string) map[string]T {
	fs := make(map[string]T)
	for k, vs := range found {
		v, ok := vs[want]
		if !ok {
			continue
		}
		fs[k] = v.(T)
	}
	return fs
}
