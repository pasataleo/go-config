package hidden

import (
	"reflect"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

type Source struct{}

func (s *Source) Name() string {
	return "hidden"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	_, ok := field.Tag.Lookup("hidden")
	if !ok {
		return nil, nil
	}
	return true, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	return false, nil
}

// Find returns a map from field path to whether that field is hidden, based on
// the "hidden" struct tag.
func Find(v interface{}) map[string]bool {
	found := config.Find(new(Source), v)
	f := make(map[string]bool, len(found))
	for k, v := range found {
		f[k] = v.(bool)
	}
	return f
}
