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
		wantStatus        string
		wantIANAStatus    entities.IANARegistrarStatus
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
		{
			name: "Accredited registrar sets status to ok",
			registrar: entities.IANARegistrar{
				GurID:   200,
				Name:    "Accredited Corp",
				Status:  entities.IANARegistrarStatusAccredited,
				RdapURL: "https://rdap.accredited.com/",
			},
			wantErr:        false,
			wantClID:       "200-accredited-c",
			wantName:       "Accredited Corp",
			wantGurID:      200,
			wantRdap:       "https://rdap.accredited.com/",
			wantStatus:     string(entities.RegistrarStatusOK),
			wantIANAStatus: entities.IANARegistrarStatusAccredited,
		},
		{
			name: "Terminated registrar sets status to terminated",
			registrar: entities.IANARegistrar{
				GurID:   300,
				Name:    "Terminated LLC",
				Status:  entities.IANARegistrarStatusTerminated,
				RdapURL: "https://rdap.terminated.com/",
			},
			wantErr:        false,
			wantClID:       "300-terminated-l",
			wantName:       "Terminated LLC",
			wantGurID:      300,
			wantRdap:       "https://rdap.terminated.com/",
			wantStatus:     string(entities.RegistrarStatusTerminated),
			wantIANAStatus: entities.IANARegistrarStatusTerminated,
		},
		{
			name: "Reserved special GurID 9995 sets status to ok",
			registrar: entities.IANARegistrar{
				GurID:  9995,
				Name:   "Reserved for Pre-Delegation Testing transactions #1 reporting",
				Status: entities.IANARegistrarStatusReserved,
			},
			wantErr:        false,
			wantClID:       "9995-pdt-1",
			wantName:       "Reserved for Pre-Delegation Testing transactions #1 reporting",
			wantGurID:      9995,
			wantStatus:     string(entities.RegistrarStatusOK),
			wantIANAStatus: entities.IANARegistrarStatusReserved,
		},
		{
			name: "Reserved special GurID 9996 sets status to ok",
			registrar: entities.IANARegistrar{
				GurID:  9996,
				Name:   "Reserved for Pre-Delegation Testing transactions #2 reporting",
				Status: entities.IANARegistrarStatusReserved,
			},
			wantErr:        false,
			wantClID:       "9996-pdt-2",
			wantName:       "Reserved for Pre-Delegation Testing transactions #2 reporting",
			wantGurID:      9996,
			wantStatus:     string(entities.RegistrarStatusOK),
			wantIANAStatus: entities.IANARegistrarStatusReserved,
		},
		{
			name: "Reserved special GurID 9997 sets status to ok",
			registrar: entities.IANARegistrar{
				GurID:  9997,
				Name:   "Reserved for ICANN's Registry SLA Monitoring System transactions reporting",
				Status: entities.IANARegistrarStatusReserved,
			},
			wantErr:        false,
			wantClID:       "9997-sla-monitor",
			wantName:       "Reserved for ICANN's Registry SLA Monitoring System transactions reporting",
			wantGurID:      9997,
			wantStatus:     string(entities.RegistrarStatusOK),
			wantIANAStatus: entities.IANARegistrarStatusReserved,
		},
		{
			name: "Reserved non-special GurID leaves status empty",
			registrar: entities.IANARegistrar{
				GurID:  9998,
				Name:   "Reserved Registrar",
				Status: entities.IANARegistrarStatusReserved,
			},
			wantErr:        false,
			wantClID:       "9998-reserved-re",
			wantName:       "Reserved Registrar",
			wantGurID:      9998,
			wantStatus:     "",
			wantIANAStatus: entities.IANARegistrarStatusReserved,
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
				// Verify status mapping
				if cmd.Status != tt.wantStatus {
					t.Errorf("unexpected Status: got %q, want %q", cmd.Status, tt.wantStatus)
				}
				if cmd.IANAStatus != tt.wantIANAStatus {
					t.Errorf("unexpected IANAStatus: got %q, want %q", cmd.IANAStatus, tt.wantIANAStatus)
				}
				// Verify PostalInfo is populated
				if cmd.PostalInfo[0] == nil {
					t.Error("PostalInfo[0] should not be nil")
				}
				// Verify placeholder email
				if cmd.Email != "i.need@2be.replaced" {
					t.Errorf("unexpected Email: got %q, want %q", cmd.Email, "i.need@2be.replaced")
				}
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
		{
			// Special reserved registrar in steady state: IANA "Reserved" but
			// platform deliberately "ok". Expected platform status is "ok", so
			// there must be no diff — this is what stops the recurring
			// "status set to ok" event on every sync run.
			name: "special reserved in sync - reserved IANA but ok platform",
			iana: entities.IANARegistrar{GurID: 9995, Status: entities.IANARegistrarStatusReserved},
			rar: entities.RegistrarListItem{
				ClID:       "9995-pdt-1",
				Status:     entities.RegistrarStatusOK,
				IANAStatus: entities.IANARegistrarStatusReserved,
			},
			wantNil: true,
		},
		{
			// Special reserved registrar whose platform status drifted away
			// from ok: expected platform status is "ok", so it is corrected
			// back to ok (never to "reserved").
			name: "special reserved drift - corrected back to ok",
			iana: entities.IANARegistrar{GurID: 9997, Status: entities.IANARegistrarStatusReserved},
			rar: entities.RegistrarListItem{
				ClID:       "9997-sla-monitor",
				Status:     entities.RegistrarStatusReadonly,
				IANAStatus: entities.IANARegistrarStatusReserved,
			},
			wantNil:       false,
			wantNewStatus: "ok",
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
