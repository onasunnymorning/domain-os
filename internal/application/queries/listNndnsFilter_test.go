package queries

import "testing"

func TestToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListNndnsFilter
		expected string
	}{
		{
			name:     "all fields empty",
			filter:   ListNndnsFilter{},
			expected: "",
		},
		{
			name: "only NameLike set",
			filter: ListNndnsFilter{
				NameLike: "example",
			},
			expected: "&name_like=example",
		},
		{
			name: "only TldEquals set",
			filter: ListNndnsFilter{
				TldEquals: "com",
			},
			expected: "&tld_equals=com",
		},
		{
			name: "only ReasonEquals set",
			filter: ListNndnsFilter{
				ReasonEquals: "test",
			},
			expected: "&reason_equals=test",
		},
		{
			name: "only ReasonLike set",
			filter: ListNndnsFilter{
				ReasonLike: "match",
			},
			expected: "&reason_like=match",
		},
		{
			name: "multiple fields set",
			filter: ListNndnsFilter{
				NameLike:     "ex",
				TldEquals:    "org",
				ReasonEquals: "fail",
				ReasonLike:   "partial",
			},
			expected: "&name_like=ex&tld_equals=org&reason_equals=fail&reason_like=partial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.ToQueryParams()
			if result != tt.expected {
				t.Errorf("ToQueryParams() = %q, want %q", result, tt.expected)
			}
		})
	}
}
