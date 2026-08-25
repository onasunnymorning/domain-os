package queries

import (
	"strings"
	"testing"
)

func TestContactToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListRegistrarsFilter
		expected string
	}{
		{
			name:     "empty filter",
			filter:   ListRegistrarsFilter{},
			expected: "",
		},
		{
			name: "only ClidLike",
			filter: ListRegistrarsFilter{
				ClidLike: "exampleClid",
			},
			expected: "&clid_like=exampleClid",
		},
		{
			name: "all fields set",
			filter: ListRegistrarsFilter{
				ClidLike:         "c1",
				NameLike:         "n1",
				NickNameLike:     "nn1",
				GuridEquals:      123,
				EmailLike:        "e1@example.com",
				StatusEquals:     "active",
				IANAStatusEquals: "registered",
				AutorenewEquals:  "yes",
			},
			// Note: The GuridEquals value is formatted using fmt.Sprintf and itoa.Itoa.
			// We're expecting it to output "123" for GuridEquals.
			expected: "&clid_like=c1&name_like=n1&nick_name_like=nn1&gur_id_equals=123&email_like=e1@example.com&status_equals=active&iana_status_equals=registered&autorenew_equals=yes",
		},
		{
			name: "partial fields set",
			filter: ListRegistrarsFilter{
				NameLike:     "name",
				GuridEquals:  456,
				StatusEquals: "inactive",
			},
			expected: "&name_like=name&gur_id_equals=456&status_equals=inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.ToQueryParams()
			// The order of query parameters is fixed based on the function logic.
			if got != tt.expected {
				t.Errorf("unexpected query string.\nGot:      %s\nExpected: %s", got, tt.expected)
			}
			// Optionally check that the returned string starts with '&' if not empty.
			if tt.expected != "" && !strings.HasPrefix(got, "&") {
				t.Errorf("expected query to start with '&', got: %s", got)
			}
		})
	}
}
