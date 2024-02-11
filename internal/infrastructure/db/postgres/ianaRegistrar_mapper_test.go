package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestIANARegistrar_ToIANARegistrar(t *testing.T) {
	tests := []struct {
		name      string
		registrar *IANARegistrar
		want      *entities.IANARegistrar
	}{
		{
			name: "success",
			registrar: &IANARegistrar{
				GurID:   1234,
				Name:    "name",
				Status:  "status",
				RdapURL: "rdapURL",
			},
			want: &entities.IANARegistrar{
				GurID:   1234,
				Name:    "name",
				Status:  "status",
				RdapURL: "rdapURL",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToIanaRegistrar(tt.registrar); *got != *tt.want {
				t.Errorf("ToIanaRegistrar() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestIANARegistrar_ToDBIANARegistrar(t *testing.T) {
	tests := []struct {
		name      string
		registrar *entities.IANARegistrar
		want      *IANARegistrar
	}{
		{
			name: "success",
			registrar: &entities.IANARegistrar{
				GurID:   1234,
				Name:    "name",
				Status:  "status",
				RdapURL: "rdapURL",
			},
			want: &IANARegistrar{
				GurID:   1234,
				Name:    "name",
				Status:  "status",
				RdapURL: "rdapURL",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToDBIANARegistrar(tt.registrar); *got != *tt.want {
				t.Errorf("ToDBIANARegistrar() = %v, want %v", *got, *tt.want)
			}
		})
	}
}
