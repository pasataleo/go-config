package didyoumean

import (
	"slices"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"apply", "appyl", 2},
		{"destroy", "destory", 2}, // nolint misspell: deliberate
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	commands := []string{"init", "plan", "apply", "destroy", "version", "help"}

	tests := []struct {
		input string
		want  string
	}{
		{"destory", "destroy"}, // nolint misspell: deliberate
		{"appyl", "apply"},
		{"plna", "plan"},
		{"inti", "init"},
		{"versiom", "version"},
		{"hlep", "help"},
		{"xyz", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Suggest(tt.input, slices.Values(commands)); got != tt.want {
			t.Errorf("Suggest(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSuggestNoCandidates(t *testing.T) {
	if got := Suggest("test", nil); got != "" {
		t.Errorf("Suggest with nil candidates = %q, want empty", got)
	}
	if got := Suggest("test", slices.Values([]string{})); got != "" {
		t.Errorf("Suggest with empty candidates = %q, want empty", got)
	}
}
