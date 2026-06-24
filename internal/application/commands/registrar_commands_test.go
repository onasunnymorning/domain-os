package commands

import (
	"reflect"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestChunkCreateRegistrarCommands(t *testing.T) {
	tests := []struct {
		name      string
		cmds      []CreateRegistrarCommand
		chunkSize int
		expected  [][]CreateRegistrarCommand
	}{
		{
			name: "chunk size 1",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 1,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}},
				{{ClID: "2"}},
				{{ClID: "3"}},
			},
		},
		{
			name: "chunk size 2",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 2,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}, {ClID: "2"}},
				{{ClID: "3"}},
			},
		},
		{
			name: "chunk size greater than length",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 5,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}, {ClID: "2"}, {ClID: "3"}},
			},
		},
		{
			name: "chunk size zero",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 0,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}},
				{{ClID: "2"}},
				{{ClID: "3"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := ChunkCreateRegistrarCommands(tt.cmds, tt.chunkSize)
			var result [][]CreateRegistrarCommand
			for chunk := range ch {
				result = append(result, chunk)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCreateCreateRegistrarCommandFromIANARegistrar(t *testing.T) {
	tests := []struct {
		name              string
		registrar         entities.IANARegistrar
		wantErr           bool
		wantClID          string
		wantName          string
		wantGurID         int
		wantRdap          string
		errStringcontains string
	}{
		{
			name: "Valid input",
			registrar: entities.IANARegistrar{
				GurID:   100,
				Name:    "Example Registrar",
				RdapURL: "https://rdap.example.com/",
			},
			wantErr:   false,
			wantClID:  "100-example-regi",
			wantName:  "Example Registrar",
			wantGurID: 100,
			wantRdap:  "https://rdap.example.com/",
		},
		{
			name: "Negative GurID triggers ClID error",
			registrar: entities.IANARegistrar{
				GurID:   -1,
				Name:    "Bad Registrar",
				RdapURL: "https://rdap.bad.com/",
			},
			wantErr:           true,
			errStringcontains: "invalid GurID for registrar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := CreateCreateRegistrarCommandFromIANARegistrar(tt.registrar)
			if (err != nil) != tt.wantErr {
				if !strings.Contains(err.Error(), tt.errStringcontains) {
					t.Fatalf("expected error containing %q but got %q", tt.errStringcontains, err.Error())
				}
			}
			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}

			if !tt.wantErr {
				// Check a few fields to ensure correctness.
				if cmd == nil {
					t.Fatalf("expected non-nil result, got nil")
				}
				if cmd.ClID != tt.wantClID {
					t.Errorf("unexpected ClID: got %q, want %q", cmd.ClID, tt.wantClID)
				}
				if cmd.Name != tt.wantName {
					t.Errorf("unexpected Name: got %q, want %q", cmd.Name, tt.wantName)
				}
				if cmd.GurID != tt.wantGurID {
					t.Errorf("unexpected GurID: got %d, want %d", cmd.GurID, tt.wantGurID)
				}
				if cmd.RdapBaseURL != tt.wantRdap {
					t.Errorf("unexpected RdapBaseURL: got %q, want %q", cmd.RdapBaseURL, tt.wantRdap)
				}
				// etc... check other fields if needed
			}
		})
	}
}

func TestCompareIANARegistrarStatusWithRarStatus(t *testing.T) {
	tests := []struct {
		name              string
		iana              entities.IANARegistrar
		rar               entities.RegistrarListItem
		wantNil           bool
		wantNewStatus     string
		wantNewIANAStatus string
	}{
		{
			name: "both in sync - accredited/ok",
			iana: entities.IANARegistrar{GurID: 1, Status: entities.IANARegistrarStatusAccredited},
			rar: entities.RegistrarListItem{
				ClID:       "1-test",
				Status:     entities.RegistrarStatusOK,
				IANAStatus: entities.IANARegistrarStatusAccredited,
			},
			wantNil: true,
		},
		{
			name: "both in sync - terminated/terminated",
			iana: entities.IANARegistrar{GurID: 2, Status: entities.IANARegistrarStatusTerminated},
			rar: entities.RegistrarListItem{
				ClID:       "2-test",
				Status:     entities.RegistrarStatusTerminated,
				IANAStatus: entities.IANARegistrarStatusTerminated,
			},
			wantNil: true,
		},
		{
			name: "platform status drift - terminated IANA but ok platform",
			iana: entities.IANARegistrar{GurID: 3, Status: entities.IANARegistrarStatusTerminated},
			rar: entities.RegistrarListItem{
				ClID:       "3-test",
				Status:     entities.RegistrarStatusOK,
				IANAStatus: entities.IANARegistrarStatusTerminated,
			},
			wantNil:       false,
			wantNewStatus: "terminated",
		},
		{
			name: "IANA status drift only - platform correct but IANA stale",
			iana: entities.IANARegistrar{GurID: 4, Status: entities.IANARegistrarStatusAccredited},
			rar: entities.RegistrarListItem{
				ClID:       "4-test",
				Status:     entities.RegistrarStatusOK,
				IANAStatus: entities.IANARegistrarStatusUnknown,
			},
			wantNil:           false,
			wantNewStatus:     "",
			wantNewIANAStatus: "Accredited",
		},
		{
			name: "both statuses drift",
			iana: entities.IANARegistrar{GurID: 5, Status: entities.IANARegistrarStatusTerminated},
			rar: entities.RegistrarListItem{
				ClID:       "5-test",
				Status:     entities.RegistrarStatusOK,
				IANAStatus: entities.IANARegistrarStatusAccredited,
			},
			wantNil:           false,
			wantNewStatus:     "terminated",
			wantNewIANAStatus: "Terminated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := CompareIANARegistrarStatusWithRarStatus(tt.iana, tt.rar)

			if tt.wantNil {
				if cmd != nil {
					t.Fatalf("expected nil result, got %+v", cmd)
				}
				return
			}

			if cmd == nil {
				t.Fatalf("expected non-nil result, got nil")
			}

			if cmd.NewStatus != tt.wantNewStatus {
				t.Errorf("NewStatus: got %q, want %q", cmd.NewStatus, tt.wantNewStatus)
			}
			if cmd.NewIANAStatus != tt.wantNewIANAStatus {
				t.Errorf("NewIANAStatus: got %q, want %q", cmd.NewIANAStatus, tt.wantNewIANAStatus)
			}
		})
	}
}
