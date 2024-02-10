package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestSpec5_ToSpec5Label(t *testing.T) {
	tests := []struct {
		name  string
		label *Spec5Label
		want  *entities.Spec5Label
	}{
		{
			name: "success",
			label: &Spec5Label{
				Label: "label1",
				Type:  "type1",
			},
			want: &entities.Spec5Label{
				Label: "label1",
				Type:  "type1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToSpec5Label(tt.label); *got != *tt.want {
				t.Errorf("ToSpec5Label() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestSpec5_ToDBSpec5Label(t *testing.T) {
	tests := []struct {
		name  string
		label *entities.Spec5Label
		want  *Spec5Label
	}{
		{
			name: "success",
			label: &entities.Spec5Label{
				Label: "label1",
				Type:  "type1",
			},
			want: &Spec5Label{
				Label: "label1",
				Type:  "type1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToDBSpec5Label(tt.label); *got != *tt.want {
				t.Errorf("ToDBSpec5Label() = %v, want %v", *got, *tt.want)
			}
		})
	}
}
