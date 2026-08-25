package queries

import "testing"

func TestHostToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListHostsFilter
		expected string
	}{
		{
			name:     "empty filter returns empty string",
			filter:   ListHostsFilter{},
			expected: "",
		},
		{
			name: "only RoidGreaterThan set",
			filter: ListHostsFilter{
				RoidGreaterThan: "1234_DOM-APEX", // 65 converts to "A" when using string(65)
			},
			expected: "&roid_greater_than=1234_DOM-APEX",
		},
		{
			name: "only RoidLessThan set",
			filter: ListHostsFilter{
				RoidLessThan: "1235_DOM-APEX", // 66 converts to "B"
			},
			expected: "&roid_less_than=1235_DOM-APEX",
		},
		{
			name: "only ClidEquals set",
			filter: ListHostsFilter{
				ClidEquals: "client123",
			},
			expected: "&clid_equals=client123",
		},
		{
			name: "only NameLike set",
			filter: ListHostsFilter{
				NameLike: "host",
			},
			expected: "&name_like=host",
		},
		{
			name: "multiple fields set",
			filter: ListHostsFilter{
				RoidGreaterThan: "1234_DOM-APEX", // "A"
				RoidLessThan:    "1235_DOM-APEX", // "B"
				ClidEquals:      "client123",
				NameLike:        "host",
			},
			expected: "&roid_greater_than=1234_DOM-APEX&roid_less_than=1235_DOM-APEX&clid_equals=client123&name_like=host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.ToQueryParams()
			if result != tt.expected {
				t.Errorf("expected %q but got %q", tt.expected, result)
			}
		})
	}
}
