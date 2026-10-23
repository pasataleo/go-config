package required

import (
	"reflect"
	"slices"

	config "github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/oneof"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

// Source validates required fields. A field that no earlier source claimed and
// that is still zero is missing, unless the oneof source has put its name in a
// group: then it is only recorded under oneof.UnsetKey, and oneof decides.
type Source struct{}

func (s *Source) Name() string {
	return "required"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	name, ok := field.Tag.Lookup("required")
	if !ok {
		return nil, nil
	}
	return name, nil
}

func (s *Source) Assign(cfg *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	name, ok := field.Tag.Lookup("required")
	if !ok {
		return false, nil
	}

	if !value.IsZero() {
		return false, nil
	}

	if grouped(cfg.Data(), name) {
		// One of a group, so whether it is missing depends on the rest of the
		// group, which only the oneof source sees whole.
		unset, _ := cfg.Data()[oneof.UnsetKey].(map[string]bool)
		if unset == nil {
			unset = map[string]bool{}
			cfg.Data()[oneof.UnsetKey] = unset
		}
		unset[name] = true
		return false, nil
	}

	return false, errorsx.Annotate(errorsx.Newf(config.MissingValue, nil, "required value not set: %s", name), "path", path)
}

// grouped reports whether the oneof source has recorded name in a group. The
// oneof source runs before every source that claims fields, so it has always
// seen the field being assigned by the time it reaches here.
func grouped(data map[string]any, name string) bool {
	groups, _ := data[oneof.Key].(map[string][]string)
	for _, names := range groups {
		if slices.Contains(names, name) {
			return true
		}
	}
	return false
}

// Find returns a map from field path to the required tag value (the display
// name for error messages), based on the "required" struct tag.
func Find(v interface{}) map[string]string {
	found := config.Find(new(Source), v)
	f := make(map[string]string, len(found))
	for k, v := range found {
		f[k] = v.(string)
	}
	return f
}

// Assign validates that all fields marked with the "required" struct tag have
// been set to a non-zero value. Returns an error for any required field that
// is still zero.
func Assign(v interface{}) error {
	return config.Assign(new(Source), v)
}
