package oneof

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	config "github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source   = (*Source)(nil)
	_ config.Finisher = (*Source)(nil)
)

const (
	// Key is the config.Data key under which Source records its groups, as a
	// map[string][]string from each group to the names in it, in the order
	// they were first seen.
	Key = "oneof"

	// UnsetKey is the config.Data key under which the required source records
	// the names in a group that were not set, as a map[string]bool. Source
	// reads it to work out which of a group's names were.
	UnsetKey = "oneof.unset"
)

// Source makes required fields alternatives: fields tagged oneof:"<group>"
// form a group, and exactly one name in each group must be set. A field's name
// is its required tag, which every field in a group must have.
//
// Groups are global, as flag names are: a field joins its group from whichever
// struct it is in, including the modules inject populates with Assigns of its
// own, and a name read in several places is one member.
//
// Source depends on the required source, which works without it: required is
// what records which names were not set, under UnsetKey, so without it every
// name in a group looks set. Source also has to come before every source that
// claims fields, so that it sees each field before one is claimed.
type Source struct{}

func (s *Source) Name() string {
	return "oneof"
}

// Find reports the group a field belongs to.
func (s *Source) Find(_ *config.Config, path reflectx.Path, _ reflect.Value, field reflect.StructField) (interface{}, error) {
	group, _, ok := member(path, field)
	if !ok {
		return nil, nil
	}
	return group, nil
}

// Assign records the field's name in its group. It never claims the field.
func (s *Source) Assign(cfg *config.Config, path reflectx.Path, _ reflect.Value, field reflect.StructField) (bool, error) {
	group, name, ok := member(path, field)
	if !ok {
		return false, nil
	}

	groups, _ := cfg.Data()[Key].(map[string][]string)
	if groups == nil {
		groups = map[string][]string{}
		cfg.Data()[Key] = groups
	}
	if !slices.Contains(groups[group], name) {
		groups[group] = append(groups[group], name)
	}
	return false, nil
}

// Finish checks that exactly one name in each group was set: every name the
// required source did not record as unset.
func (s *Source) Finish(cfg *config.Config) error {
	groups, _ := cfg.Data()[Key].(map[string][]string)
	unset, _ := cfg.Data()[UnsetKey].(map[string]bool)

	names := make([]string, 0, len(groups))
	for group := range groups {
		names = append(names, group)
	}
	sort.Strings(names)

	var errs error
	for _, group := range names {
		members := groups[group]

		var set []string
		for _, name := range members {
			if !unset[name] {
				set = append(set, name)
			}
		}

		switch {
		case len(set) == 0:
			errs = errorsx.Append(errs, errorsx.Annotate(errorsx.Newf(config.MissingValue, nil, "required value not set: one of %s", strings.Join(members, ", ")), "group", group))
		case len(set) > 1:
			errs = errorsx.Append(errs, errorsx.Annotate(errorsx.Newf(config.ConflictingValues, nil, "only one of %s can be set, got %s", strings.Join(members, ", "), strings.Join(set, ", ")), "group", group))
		}
	}
	return errs
}

// member reports the group and name of a field in a oneof group. A group's
// members are named by their required tag, so a oneof tag without one is a
// programmer error.
func member(path reflectx.Path, field reflect.StructField) (string, string, bool) {
	group, ok := field.Tag.Lookup("oneof")
	if !ok {
		return "", "", false
	}
	name, ok := field.Tag.Lookup("required")
	if !ok {
		panic(fmt.Sprintf("field %s has a oneof tag but no required tag", path))
	}
	return group, name, true
}
