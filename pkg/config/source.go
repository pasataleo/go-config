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
	err := reflectx.Walk(v, func(path reflectx.Path, value reflect.Value, field reflect.StructField) error {
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
func (config *Config) Assign(v interface{}) error {
	return reflectx.Walk(v, func(path reflectx.Path, value reflect.Value, field reflect.StructField) error {
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
	})
}

// Find queries a single source for every exported field in v and returns a map
// from field path to the value that source would provide.
//
// Note: Find will initialize nil pointer fields in v as a side effect of
// traversal.
func Find(source Source, v interface{}) map[string]interface{} {
	cfg := New(source)
	found := map[string]interface{}{}
	err := reflectx.Walk(v, func(path reflectx.Path, value reflect.Value, field reflect.StructField) error {
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
	cfg := New(source)
	return reflectx.Walk(v, func(path reflectx.Path, value reflect.Value, field reflect.StructField) error {
		_, err := source.Assign(cfg, path, value, field)
		return err
	})
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
