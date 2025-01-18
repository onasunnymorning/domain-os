package commands

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestCreateDomainCommand_FromRdeDomain(t *testing.T) {
	contacts := []entities.RDEDomainContact{
		{
			Type: "admin",
			ID:   "test-admin",
		},
		{
			Type: "tech",
			ID:   "test-tech",
		},
		{
			Type: "billing",
			ID:   "test-billing",
		},
	}

	tetcases := []struct {
		name      string
		rdeDomain *entities.RDEDomain
		cmd       *CreateDomainCommand
		wantErr   error
	}{
		{
			name: "valid RDEDomain with valid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345_DOM-APEX",
				Name:       "example.com",
				ClID:       "test",
				CrDate:     "2020-01-01T00:00:00Z",
				ExDate:     "2021-01-01T00:00:00Z",
				CrRr:       "test",
				UpRr:       "test",
				Registrant: "test-registrant",
				Contact:    contacts,
			},
			cmd: &CreateDomainCommand{
				RoID:       "12345_DOM-APEX",
				Name:       "example.com",
				ClID:       "test",
				CrRr:       "test",
				UpRr:       "test",
				CreatedAt:  time.Time(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiryDate: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				AuthInfo:   "escr0W1mP*rt",
				Status: entities.DomainStatus{
					Inactive: true,
				},
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			wantErr: nil,
		},
		{
			name: "valid RDEDomain with INvalid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345",
				Name:       "example.com",
				ClID:       "test",
				CrDate:     "2020-01-01T00:00:00Z",
				ExDate:     "2021-01-01T00:00:00Z",
				CrRr:       "test",
				UpRr:       "test",
				Registrant: "test-registrant",
				Contact:    contacts,
			},
			cmd: &CreateDomainCommand{
				Name:       "example.com",
				ClID:       "test",
				CrRr:       "test",
				UpRr:       "test",
				CreatedAt:  time.Time(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiryDate: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				AuthInfo:   "escr0W1mP*rt",
				Status: entities.DomainStatus{
					Inactive: true,
				},
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			wantErr: nil,
		},
		{
			name: "invalid ClID",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345",
				Name:       "example.com",
				ClID:       "r",
				CrRr:       "test",
				UpRr:       "test",
				Contact:    contacts,
				Registrant: "test-registrant",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidClIDType,
		},
		{
			name: "missing registrant",
			rdeDomain: &entities.RDEDomain{
				RoID:    "12345",
				Name:    "example.com",
				ClID:    "r",
				CrRr:    "test",
				UpRr:    "test",
				Contact: contacts,
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidClIDType,
		},
	}

	for _, tc := range tetcases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &CreateDomainCommand{}
			err := cmd.FromRdeDomain(tc.rdeDomain)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.Equal(t, tc.cmd, cmd)
			}
		})
	}

}
func TestUpdateDomainCommand_FromEntity(t *testing.T) {
	dom := &entities.Domain{
		OriginalName: entities.DomainName("example.com"),
		UName:        entities.DomainName("example.com"),
		RegistrantID: entities.ClIDType("registrant"),
		AdminID:      entities.ClIDType("admin"),
		TechID:       entities.ClIDType("tech"),
		BillingID:    entities.ClIDType("billing"),
		ClID:         entities.ClIDType("client"),
		CrRr:         entities.ClIDType("test"),
		UpRr:         entities.ClIDType("test"),
		ExpiryDate:   time.Now(),
		AuthInfo:     entities.AuthInfoType("authinfo"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       entities.DomainStatus{},
		RGPStatus:    entities.DomainRGPStatus{},
	}

	cmd := &UpdateDomainCommand{}
	cmd.FromEntity(dom)

	require.Equal(t, dom.OriginalName.String(), cmd.OriginalName)
	require.Equal(t, dom.UName.String(), cmd.UName)
	require.Equal(t, dom.RegistrantID.String(), cmd.RegistrantID)
	require.Equal(t, dom.AdminID.String(), cmd.AdminID)
	require.Equal(t, dom.TechID.String(), cmd.TechID)
	require.Equal(t, dom.BillingID.String(), cmd.BillingID)
	require.Equal(t, dom.ClID.String(), cmd.ClID)
	require.Equal(t, dom.CrRr.String(), cmd.CrRr)
	require.Equal(t, dom.UpRr.String(), cmd.UpRr)
	require.Equal(t, dom.ExpiryDate, cmd.ExpiryDate)
	require.Equal(t, dom.AuthInfo.String(), cmd.AuthInfo)
	require.Equal(t, dom.CreatedAt, cmd.CreatedAt)
	require.Equal(t, dom.UpdatedAt, cmd.UpdatedAt)
	require.Equal(t, dom.Status, cmd.Status)
	require.Equal(t, dom.RGPStatus, cmd.RGPStatus)
}

func TestRegisterDomainCommand_ApplyContactDataPolicy(t *testing.T) {
	tcases := []struct {
		name    string
		cmd     RegisterDomainCommand
		policy  entities.ContactDataPolicy
		wantErr error
		wantIDs struct {
			registrant string
			admin      string
			tech       string
			billing    string
		}
	}{
		{
			name: "All mandatory fields set, no error expected",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeMandatory,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "test-registrant",
				admin:      "test-admin",
				tech:       "test-tech",
				billing:    "test-billing",
			},
		},
		{
			name: "Registrant mandatory not set",
			cmd: RegisterDomainCommand{
				AdminID:   "test-admin",
				TechID:    "test-tech",
				BillingID: "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeOptional,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeOptional,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeOptional,
			},
			wantErr: entities.ErrRegistrantIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "",
				admin:      "test-admin",
				tech:       "test-tech",
				billing:    "test-billing",
			},
		},
		{
			name: "Tech mandatory not set",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-reg",
				AdminID:      "test-admin",
				BillingID:    "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeOptional,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeOptional,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeOptional,
			},
			wantErr: entities.ErrTechIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "test-reg",
				admin:      "test-admin",
				billing:    "test-billing",
			},
		},
		{
			name: "Billing mandatory not set",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-reg",
				AdminID:      "test-admin",
				TechID:       "test-tech",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeOptional,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeOptional,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeOptional,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: entities.ErrBillingIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "test-reg",
				admin:      "test-admin",
				tech:       "test-tech",
			},
		},
		{
			name: "Admin mandatory not set",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-reg",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeOptional,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeMandatory,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeOptional,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeOptional,
			},
			wantErr: entities.ErrAdminIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "test-reg",
				tech:       "test-tech",
				billing:    "test-billing",
			},
		},
		{
			name: "All prohibited fields removed",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "",
				admin:      "",
				tech:       "",
				billing:    "",
			},
		},
		{
			name: "Mixed policy (registrant mandatory, others prohibited)",
			cmd: RegisterDomainCommand{
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "test-registrant",
				admin:      "",
				tech:       "",
				billing:    "",
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.ApplyContactDataPolicy(tc.policy)
			require.Equal(t, tc.wantErr, err)

			require.Equal(t, tc.wantIDs.registrant, tc.cmd.RegistrantID, "RegistrantID mismatch")
			require.Equal(t, tc.wantIDs.admin, tc.cmd.AdminID, "AdminID mismatch")
			require.Equal(t, tc.wantIDs.tech, tc.cmd.TechID, "TechID mismatch")
			require.Equal(t, tc.wantIDs.billing, tc.cmd.BillingID, "BillingID mismatch")
		})
	}
}

func TestCreateDomainCommand_ApplyContactDataPolicy(t *testing.T) {
	tcases := []struct {
		name    string
		cmd     CreateDomainCommand
		policy  entities.ContactDataPolicy
		wantErr error
		wantIDs struct {
			registrant string
			admin      string
			tech       string
			billing    string
		}
	}{
		{
			name: "All mandatory fields set",
			cmd: CreateDomainCommand{
				RegistrantID: "reg",
				AdminID:      "adm",
				TechID:       "tec",
				BillingID:    "bil",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeMandatory,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "reg",
				admin:      "adm",
				tech:       "tec",
				billing:    "bil",
			},
		},
		{
			name: "Registrant mandatory but not set",
			cmd:  CreateDomainCommand{},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: entities.ErrRegistrantIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{},
		},
		{
			name: "All prohibited",
			cmd: CreateDomainCommand{
				RegistrantID: "reg",
				AdminID:      "adm",
				TechID:       "tec",
				BillingID:    "bil",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "",
				admin:      "",
				tech:       "",
				billing:    "",
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.ApplyContactDataPolicy(tc.policy)
			require.Equal(t, tc.wantErr, err)
			require.Equal(t, tc.wantIDs.registrant, tc.cmd.RegistrantID)
			require.Equal(t, tc.wantIDs.admin, tc.cmd.AdminID)
			require.Equal(t, tc.wantIDs.tech, tc.cmd.TechID)
			require.Equal(t, tc.wantIDs.billing, tc.cmd.BillingID)
		})
	}
}

func TestUpdateDomainCommand_ApplyContactDataPolicy(t *testing.T) {
	tcases := []struct {
		name    string
		cmd     UpdateDomainCommand
		policy  entities.ContactDataPolicy
		wantErr error
		wantIDs struct {
			registrant string
			admin      string
			tech       string
			billing    string
		}
	}{
		{
			name: "All mandatory fields set",
			cmd: UpdateDomainCommand{
				RegistrantID: "reg",
				AdminID:      "adm",
				TechID:       "tec",
				BillingID:    "bil",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeMandatory,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeMandatory,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "reg",
				admin:      "adm",
				tech:       "tec",
				billing:    "bil",
			},
		},
		{
			name: "Admin mandatory not set",
			cmd: UpdateDomainCommand{
				RegistrantID: "reg",
				TechID:       "tec",
				BillingID:    "bil",
			},
			policy: entities.ContactDataPolicy{
				AdminContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
			},
			wantErr: entities.ErrAdminIDRequiredButNotSet,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "reg",
				tech:       "tec",
				billing:    "bil",
			},
		},
		{
			name: "All prohibited",
			cmd: UpdateDomainCommand{
				RegistrantID: "reg",
				AdminID:      "adm",
				TechID:       "tec",
				BillingID:    "bil",
			},
			policy: entities.ContactDataPolicy{
				RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
				AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
				TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
				BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
			},
			wantErr: nil,
			wantIDs: struct {
				registrant string
				admin      string
				tech       string
				billing    string
			}{
				registrant: "",
				admin:      "",
				tech:       "",
				billing:    "",
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.ApplyContactDataPolicy(tc.policy)
			require.Equal(t, tc.wantErr, err)
			require.Equal(t, tc.wantIDs.registrant, tc.cmd.RegistrantID)
			require.Equal(t, tc.wantIDs.admin, tc.cmd.AdminID)
			require.Equal(t, tc.wantIDs.tech, tc.cmd.TechID)
			require.Equal(t, tc.wantIDs.billing, tc.cmd.BillingID)
		})
	}
}
func TestFeeExtension_IsZero(t *testing.T) {
	tcases := []struct {
		name     string
		fee      FeeExtension
		expected bool
	}{
		{
			name:     "Zero value FeeExtension",
			fee:      FeeExtension{},
			expected: true,
		},
		{
			name: "Non-zero value FeeExtension",
			fee: FeeExtension{
				Currency: "USD",
				Amount:   10.0,
			},
			expected: false,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fee.IsZero()
			require.Equal(t, tc.expected, result)
		})
	}
}
