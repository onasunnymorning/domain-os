package queries

import (
	"testing"
)

func TestPremiumListsToQueryParams(t *testing.T) {
	tests := []struct {
		name   string
		filter ListPremiumListsFilter
		want   string
	}{
		{
			name:   "All fields empty",
			filter: ListPremiumListsFilter{},
			want:   "",
		},
		{
			name: "Only NameLike set",
			filter: ListPremiumListsFilter{
				NameLike: "example",
			},
			want: "&name_like=example",
		},
		{
			name: "Only RyIDEquals set",
			filter: ListPremiumListsFilter{
				RyIDEquals: "12345",
			},
			want: "&ry_id_equals=12345",
		},
		{
			name: "Only CreatedBefore set",
			filter: ListPremiumListsFilter{
				CreatedBefore: "2023-01-01",
			},
			want: "&created_before=2023-01-01",
		},
		{
			name: "Only CreatedAfter set",
			filter: ListPremiumListsFilter{
				CreatedAfter: "2022-01-01",
			},
			want: "&created_after=2022-01-01",
		},
		{
			name: "Multiple fields set",
			filter: ListPremiumListsFilter{
				NameLike:      "test",
				RyIDEquals:    "789",
				CreatedBefore: "2024-01-01",
				CreatedAfter:  "2020-01-01",
			},
			want: "&name_like=test&ry_id_equals=789&created_before=2024-01-01&created_after=2020-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.ToQueryParams()
			if got != tt.want {
				t.Errorf("ToQueryParams() = %v, want %v", got, tt.want)
			}
		})
	}
}
