package queries

import "testing"

func TestContactsToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListContactsFilter
		expected string
	}{
		{
			name:     "empty filter returns empty string",
			filter:   ListContactsFilter{},
			expected: "",
		},
		{
			name: "only RoidGreaterThan",
			filter: ListContactsFilter{
				RoidGreaterThan: "123",
			},
			expected: "&roid_greater_than=123",
		},
		{
			name: "only RoidLessThan",
			filter: ListContactsFilter{
				RoidLessThan: "456",
			},
			expected: "&roid_less_than=456",
		},
		{
			name: "only IdLike",
			filter: ListContactsFilter{
				IdLike: "idValue",
			},
			expected: "&id_like=idValue",
		},
		{
			name: "only EmailLike",
			filter: ListContactsFilter{
				EmailLike: "email@example.com",
			},
			expected: "&email_like=email@example.com",
		},
		{
			name: "only ClidEquals",
			filter: ListContactsFilter{
				ClidEquals: "clidVal",
			},
			expected: "&clid_equals=clidVal",
		},
		{
			name: "multiple fields",
			filter: ListContactsFilter{
				RoidGreaterThan: "123",
				RoidLessThan:    "456",
				IdLike:          "idValue",
				EmailLike:       "email@example.com",
				ClidEquals:      "clidVal",
			},
			expected: "&roid_greater_than=123&roid_less_than=456&id_like=idValue&email_like=email@example.com&clid_equals=clidVal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.ToQueryParams()
			if got != tt.expected {
				t.Errorf("ToQueryParams() = %q, want %q", got, tt.expected)
			}
		})
	}
}
