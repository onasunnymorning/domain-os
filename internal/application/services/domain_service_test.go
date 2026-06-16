package services

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/assert"
)

func TestDomainFromCreateDomainCommand(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		t.Fatal(err)
	}
	roidService := NewRoidService(idgen)

	domainService := &DomainService{
		roidService: *roidService,
	}

	tests := []struct {
		name    string
		cmd     *commands.CreateDomainCommand
		wantErr bool
	}{
		{
			name: "Valid command with RoID",
			cmd: &commands.CreateDomainCommand{
				RoID:           "123_DOM-APEX",
				Name:           "example.com",
				ClID:           "client123",
				AuthInfo:       "sTr0N5p@zzWqRD",
				RegistrantID:   "registrant123",
				AdminID:        "admin123",
				TechID:         "tech123",
				BillingID:      "billing123",
				CrRr:           "crrr123",
				UpRr:           "uprr123",
				ExpiryDate:     time.Now().AddDate(1, 0, 0),
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
				Status:         entities.DomainStatus{},
				RGPStatus:      entities.DomainRGPStatus{},
				GrandFathering: entities.DomainGrandFathering{},
				RenewedYears:   1,
			},
			wantErr: false,
		},
		{
			name: "Valid command without RoID",
			cmd: &commands.CreateDomainCommand{
				Name:           "example.com",
				ClID:           "client123",
				AuthInfo:       "sTr0N5p@zzWqRD",
				RegistrantID:   "registrant123",
				AdminID:        "admin123",
				TechID:         "tech123",
				BillingID:      "billing123",
				CrRr:           "crrr123",
				UpRr:           "uprr123",
				ExpiryDate:     time.Now().AddDate(1, 0, 0),
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
				Status:         entities.DomainStatus{},
				RGPStatus:      entities.DomainRGPStatus{},
				GrandFathering: entities.DomainGrandFathering{},
				RenewedYears:   1,
			},
			wantErr: false,
		},
		{
			name: "Invalid command with empty name",
			cmd: &commands.CreateDomainCommand{
				RoID:     "123_DOM-APEX",
				Name:     "",
				ClID:     "client123",
				AuthInfo: "sTr0N5p@zzWqRD",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := domainService.domainFromCreateDomainCommand(tt.cmd)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, domain)
				assert.Equal(t, tt.cmd.Name, domain.Name.String())
				assert.Equal(t, tt.cmd.ClID, domain.ClID.String())
				assert.Equal(t, tt.cmd.AuthInfo, domain.AuthInfo.String())
				if tt.cmd.RoID == "" {
					assert.Contains(t, domain.RoID.String(), "DOM-APEX")
				} else {
					assert.Equal(t, tt.cmd.RoID, domain.RoID.String())
				}
			}
		})
	}
}
func TestBulkDomainFromCreateDomainCommands(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		t.Fatal(err)
	}
	roidService := NewRoidService(idgen)

	domainService := &DomainService{
		roidService: *roidService,
	}

	tests := []struct {
		name    string
		cmds    []*commands.CreateDomainCommand
		wantErr bool
	}{
		{
			name: "Valid commands",
			cmds: []*commands.CreateDomainCommand{
				{
					RoID:           "123_DOM-APEX",
					Name:           "example1.com",
					ClID:           "client123",
					AuthInfo:       "sTr0N5p@zzWqRD",
					RegistrantID:   "registrant123",
					AdminID:        "admin123",
					TechID:         "tech123",
					BillingID:      "billing123",
					CrRr:           "crrr123",
					UpRr:           "uprr123",
					ExpiryDate:     time.Now().AddDate(1, 0, 0),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					Status:         entities.DomainStatus{},
					RGPStatus:      entities.DomainRGPStatus{},
					GrandFathering: entities.DomainGrandFathering{},
					RenewedYears:   1,
				},
				{
					Name:           "example2.com",
					ClID:           "client456",
					AuthInfo:       "sTr0N5p@zzWqRD",
					RegistrantID:   "registrant456",
					AdminID:        "admin456",
					TechID:         "tech456",
					BillingID:      "billing456",
					CrRr:           "crrr456",
					UpRr:           "uprr456",
					ExpiryDate:     time.Now().AddDate(1, 0, 0),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					Status:         entities.DomainStatus{},
					RGPStatus:      entities.DomainRGPStatus{},
					GrandFathering: entities.DomainGrandFathering{},
					RenewedYears:   1,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid command with empty name",
			cmds: []*commands.CreateDomainCommand{
				{
					RoID:     "123_DOM-APEX",
					Name:     "",
					ClID:     "client123",
					AuthInfo: "sTr0N5p@zzWqRD",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domains, err := domainService.bulkDomainFromCreateDomainCommands(tt.cmds)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, domains)
				assert.Equal(t, len(tt.cmds), len(domains))
				for i, cmd := range tt.cmds {
					assert.Equal(t, cmd.Name, domains[i].Name.String())
					assert.Equal(t, cmd.ClID, domains[i].ClID.String())
					assert.Equal(t, cmd.AuthInfo, domains[i].AuthInfo.String())
					if cmd.RoID == "" {
						assert.Contains(t, domains[i].RoID.String(), "DOM-APEX")
					} else {
						assert.Equal(t, cmd.RoID, domains[i].RoID.String())
					}
				}
			}
		})
	}
}

func TestDomainService_UpdateDomain_PreservesCreatedAt(t *testing.T) {
	ctx := context.Background()

	originalCreatedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	existingDomain := &entities.Domain{
		RoID:       "123_DOM-APEX",
		Name:       "example.com",
		ClID:       "client123",
		AuthInfo:   "sTr0N5p@zzWqRD",
		ExpiryDate: time.Now().AddDate(1, 0, 0),
		CreatedAt:  originalCreatedAt,
		UpdatedAt:  time.Now(),
		Status:     entities.DomainStatus{OK: true},
	}

	repo := new(repositories.MockDomainRepository)
	repo.On("GetDomainByName", ctx, "example.com", false).Return(existingDomain, nil)
	// We just want to check if the entity passed to UpdateDomain has the preserved CreatedAt
	repo.On("UpdateDomain", ctx, existingDomain).Return(existingDomain, nil)

	service := &DomainService{
		domainRepository: repo,
		eventPublisher:   &noopPublisher{},
	}

	upDomCmd := &commands.UpdateDomainCommand{
		RegistrantID: "registrant123",
		AdminID:      "admin123",
		TechID:       "tech123",
		BillingID:    "billing123",
		CrRr:         "crrr123",
		UpRr:         "uprr123",
		ExpiryDate:   existingDomain.ExpiryDate,
		AuthInfo:     "sTr0N5p@zzWqRD",
		CreatedAt:    time.Time{}, // Zero value mapped from JSON
		UpdatedAt:    time.Now(),
		Status:       entities.DomainStatus{OK: true},
	}

	updatedDom, err := service.UpdateDomain(ctx, "example.com", upDomCmd)
	if err != nil {
		t.Fatalf("unexpected error during update: %v", err)
	}

	if updatedDom.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be preserved, but it was set to zero value")
	}

	if !updatedDom.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to equal %v, got %v", originalCreatedAt, updatedDom.CreatedAt)
	}
}

type noopPublisher struct{}

func (n *noopPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	return nil
}
