package queries

import "testing"

func TestROFilterToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListRegistryOperatorsFilter
		expected string
	}{
		{
			name:     "All fields empty",
			filter:   ListRegistryOperatorsFilter{},
			expected: "",
		},
		{
			name:     "Only RyidLike set",
			filter:   ListRegistryOperatorsFilter{RyidLike: "foo"},
			expected: "&ryid_like=foo",
		},
		{
			name:     "Only NameLike set",
			filter:   ListRegistryOperatorsFilter{NameLike: "bar"},
			expected: "&name_like=bar",
		},
		{
			name:     "Only EmailLike set",
			filter:   ListRegistryOperatorsFilter{EmailLike: "baz"},
			expected: "&email_like=baz",
		},
		{
			name:     "All fields set",
			filter:   ListRegistryOperatorsFilter{RyidLike: "foo", NameLike: "bar", EmailLike: "baz"},
			expected: "&ryid_like=foo&name_like=bar&email_like=baz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.filter.ToQueryParams()
			if got != tc.expected {
				t.Errorf("Expected query params %q, got %q", tc.expected, got)
			}
		})
	}
}
