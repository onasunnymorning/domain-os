package queries

import "testing"

func TestToSpec5QueryParams(t *testing.T) {
	tests := []struct {
		name   string
		filter ListSpec5LabelsFilter
		want   string
	}{
		{
			name:   "Empty filter",
			filter: ListSpec5LabelsFilter{},
			want:   "",
		},
		{
			name:   "Only LabelLike set",
			filter: ListSpec5LabelsFilter{LabelLike: "foo"},
			want:   "&label_like=foo",
		},
		{
			name:   "Only TypeEquals set",
			filter: ListSpec5LabelsFilter{TypeEquals: "bar"},
			want:   "&type_equals=bar",
		},
		{
			name:   "Both LabelLike and TypeEquals set",
			filter: ListSpec5LabelsFilter{LabelLike: "foo", TypeEquals: "bar"},
			want:   "&label_like=foo&type_equals=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.ToQueryParams()
			if got != tt.want {
				t.Errorf("ToQueryParams() = %q, want %q", got, tt.want)
			}
		})
	}
}
