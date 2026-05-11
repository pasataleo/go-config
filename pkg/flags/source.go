package flags

import (
	"maps"
	"reflect"
	"strings"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/didyoumean"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

type Source struct {
	values map[string][]string
}

type Flag struct {
	Name string
	Type reflect.Type
}

func NewSource(values map[string][]string) *Source {
	return &Source{values: values}
}

func (s *Source) Name() string {
	return "flags"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	tag, ok := field.Tag.Lookup("flag")
	if !ok {
		return nil, nil
	}
	if !reflectx.CanSetString(value.Type()) {
		return Flag{
			Name: tag,
			Type: value.Type(),
		}, errorsx.Annotate(errorsx.Newf(errorsx.Unknown, nil, "%q can't set from string", value.Type().String()), "path", path)
	}
	return Flag{
		Name: tag,
		Type: value.Type(),
	}, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	tag, ok := field.Tag.Lookup("flag")
	if !ok {
		return false, nil
	}

	vals := s.values[tag]
	negVals := s.values["no-"+tag]

	unpacked := reflectx.Unpack(value)

	if unpacked.Kind() == reflect.Bool && len(negVals) > 0 {
		if len(vals) > 0 {
			return true, errorsx.Annotate(errorsx.Newf(config.InvalidValue, nil, "cannot set both --%s and --no-%s", tag, tag), "path", path)
		}
		unpacked.SetBool(false)
		return true, nil
	}

	if len(vals) == 0 {
		return false, nil
	}

	var errs error
	for _, v := range vals {
		if err := reflectx.SetString(unpacked, v, reflectx.EmptyStringIsTrue()); err != nil {
			errs = errorsx.Append(errs, errorsx.Annotate(errorsx.Newf(config.InvalidValue, nil, "unable to read --%s: %s", tag, err), "path", path))
		}
	}
	return true, errs
}

func (s *Source) Validate(flags map[string]Flag) map[string]string {
	availableFlags := make(map[string]reflect.Type, len(flags))
	for _, v := range flags {
		availableFlags[v.Name] = v.Type
	}

	missingFlags := make(map[string]string)
	for k := range s.values {
		if _, ok := availableFlags[k]; !ok {
			if k, ok := strings.CutPrefix(k, "no-"); ok {
				if t, ok := availableFlags[k]; ok && t.Kind() == reflect.Bool {
					// then this matched a `no-` for a boolean.
					continue
				}
			}

			missingFlags[k] = didyoumean.Suggest(k, maps.Keys(availableFlags))
		}
	}
	return missingFlags
}

// Find returns a map from field path to the flag name that would be used to
// populate it, based on the "flag" struct tag.
func Find(v interface{}) map[string]Flag {
	found := config.Find(new(Source), v)
	f := make(map[string]Flag, len(found))
	for k, v := range found {
		f[k] = v.(Flag)
	}
	return f
}

// Assign populates the exported fields of v from the parsed flag values, using
// the "flag" struct tag to match fields to flag names.
func Assign(values map[string][]string, v interface{}) error {
	return config.Assign(NewSource(values), v)
}
