package entities

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRDEDomain_ToEntity(t *testing.T) {
	tests := []struct {
		name      string
		rdeDomain *RDEDomain
		domain    *Domain
		wantErr   error
		wantWarn  []error
	}{
		{
			name: "invalid roid",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_HOST-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidDomainRoID,
		},
		{
			name: "invalid crrr",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrRr:         "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid uprr",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid registrant",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid admin ID",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid tech ID",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid billing ID",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "GoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMammaGoMamma",
						Type: "billing",
					},
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidClIDType,
		},
		{
			name: "invalid contact",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Contact: []RDEDomainContact{
					{
						ID:   "Gomamma",
						Type: "odd-contact-type",
					},
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidContact,
		},
		{
			name: "invalid domain status",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Status: []RDEDomainStatus{
					{
						S: "pendingDelete",
					},
					{
						S: "pendingCreate",
					},
				},
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrInvalidDomainStatusCombination,
		},
		{
			name: "invalid originalname for non-idn domain",
			rdeDomain: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				OriginalName: "apex.domains",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				ExDate:       "2022-01-01T00:00:00Z",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
				Registrant:   "GoMamma",
				Status: []RDEDomainStatus{
					{
						S: "pendingDelete",
					},
					{
						S: "inactive",
					},
				},
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrOriginalNameFieldReservedForIDN,
		},
		{
			name: "invalid uname for non-idn domain",
			rdeDomain: &RDEDomain{
				Name:       "apex.domains",
				RoID:       "12345_DOM-APEX",
				UName:      "apex.domains",
				ClID:       "GoMamma",
				CrRr:       "GoMamma",
				ExDate:     "2022-01-01T00:00:00Z",
				CrDate:     "2021-01-01T00:00:00Z",
				UpRr:       "GoMamma",
				UpDate:     "2021-01-01T00:00:00Z",
				Registrant: "GoMamma",
				Status: []RDEDomainStatus{
					{
						S: "pendingDelete",
					},
					{
						S: "inactive",
					},
				},
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain:  nil,
			wantErr: ErrUNameFieldReservedForIDNDomains,
		},
		{
			name: "valid domain",
			rdeDomain: &RDEDomain{
				Name:       "apex.domains",
				RoID:       "12345_DOM-APEX",
				ClID:       "GoMamma",
				CrRr:       "GoMamma",
				ExDate:     "2022-01-01T00:00:00Z",
				CrDate:     "2021-01-01T00:00:00Z",
				UpRr:       "GoMamma",
				UpDate:     "2021-01-01T00:00:00Z",
				Registrant: "GoMamma",
				Status: []RDEDomainStatus{
					{
						S: "pendingDelete",
					},
					{
						S: "inactive",
					},
				},
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain: &Domain{
				Name:         DomainName("apex.domains"),
				RoID:         "12345_DOM-APEX",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				UpRr:         "GoMamma",
				CreatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				ExpiryDate:   time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
				RegistrantID: "GoMamma",
				AdminID:      "GoMamma",
				TechID:       "GoMamma",
				BillingID:    "GoMamma",
				Status: DomainStatus{
					PendingDelete: true,
					Inactive:      true,
					OK:            false,
				},
			},
			wantErr: nil,
		},
		{
			name: "Fix-CNIC-1: valid domain with multiple statuses",
			rdeDomain: &RDEDomain{
				Name:       "apex.domains",
				RoID:       "12345_DOM-APEX",
				ClID:       "GoMamma",
				CrRr:       "GoMamma",
				ExDate:     "2222-01-01T00:00:00Z",
				CrDate:     "2021-01-01T00:00:00Z",
				UpRr:       "GoMamma",
				UpDate:     "2021-01-01T00:00:00Z",
				Registrant: "GoMamma",
				Status: []RDEDomainStatus{
					{
						S: "pendingDelete",
					},
					{
						S: "serverDeleteProhibited",
					},
				},
				Contact: []RDEDomainContact{
					{
						ID:   "GoMamma",
						Type: "admin",
					},
					{
						ID:   "GoMamma",
						Type: "tech",
					},
					{
						ID:   "GoMamma",
						Type: "billing",
					},
				},
			},
			domain: &Domain{
				Name:         DomainName("apex.domains"),
				RoID:         "12345_DOM-APEX",
				ClID:         "GoMamma",
				CrRr:         "GoMamma",
				UpRr:         "GoMamma",
				CreatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				ExpiryDate:   time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
				RegistrantID: "GoMamma",
				AdminID:      "GoMamma",
				TechID:       "GoMamma",
				BillingID:    "GoMamma",
				Status: DomainStatus{
					PendingDelete:          false,
					ServerDeleteProhibited: true,
					Inactive:               true,
					OK:                     false,
				},
			},
			wantErr: nil,
			wantWarn: []error{
				errors.New("removed pendingDelete status from domain apex.domains - Ref.https://www.notion.so/apex-domains/Importing-Escrows-Experiment-1956c0599d5380b488d8f6f4ead200e8?pvs=4"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.rdeDomain.ToEntity()
			require.ErrorIs(t, err, tt.wantErr)

			if tt.wantErr == nil {

				if tt.wantErr != nil {
					require.Equal(t, tt.wantWarn[0].Error(), result.Warnings[0].Error())
				}

				domain := result.Domain

				require.Equal(t, tt.domain.Name, domain.Name)
				require.Equal(t, tt.domain.Name.ParentDomain(), domain.TLDName.String())
				require.Equal(t, tt.domain.ClID, domain.ClID)
				require.Equal(t, tt.domain.RoID, domain.RoID)
				require.Equal(t, tt.domain.UName, domain.UName)
				require.Equal(t, tt.domain.OriginalName, domain.OriginalName)
				require.Equal(t, tt.domain.CrRr, domain.CrRr)
				require.Equal(t, tt.domain.UpRr, domain.UpRr)
				require.Equal(t, tt.domain.RegistrantID, domain.RegistrantID)
				require.Equal(t, tt.domain.AdminID, domain.AdminID)
				require.Equal(t, tt.domain.BillingID, domain.BillingID)
				require.Equal(t, tt.domain.TechID, domain.TechID)
				require.Equal(t, tt.domain.Status, domain.Status)
			}
		})
	}

}
func TestRDEDomain_ToCSV(t *testing.T) {
	tc := []struct {
		name string
		d    *RDEDomain
		want []string
	}{{
		name: "valid domain",
		d: &RDEDomain{
			Name:         "apex.domains",
			RoID:         "12345_DOM-APEX",
			UName:        "apex.domains",
			IdnTableId:   "idnTableId",
			OriginalName: "apex.domains",
			Registrant:   "GoMamma",
			ClID:         "GoMamma",
			CrRr:         "GoMamma",
			CrDate:       "2021-01-01T00:00:00Z",
			ExDate:       "2022-01-01T00:00:00Z",
			UpRr:         "GoMamma",
			UpDate:       "2021-01-01T00:00:00Z",
		},
		want: []string{"apex.domains", "12345_DOM-APEX", "apex.domains", "idnTableId", "apex.domains", "GoMamma", "GoMamma", "GoMamma", "2021-01-01T00:00:00Z", "2022-01-01T00:00:00Z", "GoMamma", "2021-01-01T00:00:00Z"},
	},
		{
			name: "empty fields",
			d: &RDEDomain{
				Name:         "apex.domains",
				RoID:         "12345_DOM-APEX",
				UName:        "apex.domains",
				IdnTableId:   "idnTableId",
				OriginalName: "apex.domains",
				Registrant:   "GoMamma",
				ClID:         "GoMamma",
				CrDate:       "2021-01-01T00:00:00Z",
				UpRr:         "GoMamma",
				UpDate:       "2021-01-01T00:00:00Z",
			},
			want: []string{"apex.domains", "12345_DOM-APEX", "apex.domains", "idnTableId", "apex.domains", "GoMamma", "GoMamma", "", "2021-01-01T00:00:00Z", "", "GoMamma", "2021-01-01T00:00:00Z"},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.ToCSV()

			require.Equal(t, tt.want, got)
			require.Equal(t, len(RDE_DOMAIN_CSV_HEADER), len(got))
		})
	}
}
func TestGetDomainStatusFromRDEDomainStatus(t *testing.T) {
	tc := []struct {
		name     string
		statuses []RDEDomainStatus
		want     DomainStatus
	}{
		{
			name: "all statuses",
			statuses: []RDEDomainStatus{
				{S: "clientDeleteProhibited"},
				{S: "clientHold"},
				{S: "clientRenewProhibited"},
				{S: "clientTransferProhibited"},
				{S: "clientUpdateProhibited"},
				{S: "inactive"},
				{S: "ok"},
				{S: "pendingCreate"},
				{S: "pendingDelete"},
				{S: "pendingRenew"},
				{S: "pendingTransfer"},
				{S: "pendingUpdate"},
				{S: "serverDeleteProhibited"},
				{S: "serverHold"},
				{S: "serverRenewProhibited"},
				{S: "serverTransferProhibited"},
				{S: "serverUpdateProhibited"},
			},
			want: DomainStatus{
				ClientDeleteProhibited:   true,
				ClientHold:               true,
				ClientRenewProhibited:    true,
				ClientTransferProhibited: true,
				ClientUpdateProhibited:   true,
				Inactive:                 true,
				OK:                       true,
				PendingCreate:            true,
				PendingDelete:            true,
				PendingRenew:             true,
				PendingTransfer:          true,
				PendingUpdate:            true,
				ServerDeleteProhibited:   true,
				ServerHold:               true,
				ServerRenewProhibited:    true,
				ServerTransferProhibited: true,
				ServerUpdateProhibited:   true,
			},
		},
		{
			name: "just inactive",
			statuses: []RDEDomainStatus{
				{S: "inactive"},
			},
			want: DomainStatus{
				Inactive: true,
				OK:       false,
			},
		},
		{
			name: "just ok",
			statuses: []RDEDomainStatus{
				{S: "ok"},
			},
			want: DomainStatus{
				OK: true,
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDomainStatusFromRDEDomainStatus(tt.statuses)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

}
