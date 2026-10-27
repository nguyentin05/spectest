package generate

import (
	"reflect"
	"testing"
)

func TestSplitCallPath_TrailingSlash(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
		hasEmpty bool
	}{
		{
			name:     "root slash",
			path:     "/",
			expected: nil,
			hasEmpty: false,
		},
		{
			name:     "no trailing slash",
			path:     "/appointments",
			expected: []string{"appointments"},
			hasEmpty: false,
		},
		{
			name:     "with trailing slash",
			path:     "/appointments/",
			expected: []string{"appointments"},
			hasEmpty: false,
		},
		{
			name:     "nested with trailing slash",
			path:     "/appointments/{id}/records/",
			expected: []string{"appointments", "{id}", "records"},
			hasEmpty: false,
		},
		{
			name:     "double slash middle",
			path:     "/appointments//records",
			expected: []string{"appointments", "", "records"},
			hasEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := splitCallPath(tt.path)
			if !reflect.DeepEqual(segs, tt.expected) {
				t.Errorf("splitCallPath(%q) = %#v, want %#v", tt.path, segs, tt.expected)
			}
			if hasEmptySegment(segs) != tt.hasEmpty {
				t.Errorf("hasEmptySegment(%#v) = %v, want %v", segs, hasEmptySegment(segs), tt.hasEmpty)
			}
		})
	}
}
