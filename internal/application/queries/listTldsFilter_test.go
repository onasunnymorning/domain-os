package queries

import (
	"testing"
)

func TestListTldsFilter_ToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListTldsFilter
		expected string
	}{
		{
			name: "all fields empty",
			filter: ListTldsFilter{
				NameLike:   "",
				TypeEquals: "",
				RyIDEquals: "",
			},
			expected: "",
		},
		{
			name: "only NameLike set",
			filter: ListTldsFilter{
				NameLike:   "example",
				TypeEquals: "",
				RyIDEquals: "",
			},
			expected: "name_like=example&",
		},
		{
			name: "only TypeEquals set",
			filter: ListTldsFilter{
				NameLike:   "",
				TypeEquals: "tld",
				RyIDEquals: "",
			},
			expected: "type_equals=tld&",
		},
		{
			name: "only RyIDEquals set",
			filter: ListTldsFilter{
				NameLike:   "",
				TypeEquals: "",
				RyIDEquals: "123",
			},
			expected: "ry_id_equals=123&",
		},
		{
			name: "two fields set: NameLike and TypeEquals",
			filter: ListTldsFilter{
				NameLike:   "example",
				TypeEquals: "tld",
				RyIDEquals: "",
			},
			expected: "name_like=example&type_equals=tld&",
		},
		{
			name: "all fields set",
			filter: ListTldsFilter{
				NameLike:   "example",
				TypeEquals: "tld",
				RyIDEquals: "123",
			},
			expected: "name_like=example&type_equals=tld&ry_id_equals=123&",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.ToQueryParams()
			if result != tt.expected {
				t.Errorf("ToQueryParams() = %q; expected %q", result, tt.expected)
			}
		})
	}
}
