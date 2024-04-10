package services

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"golang.org/x/net/context"
)

// MockTLDRepository is a mock implementation of the TLDRepository interface
type MockTLDRepository struct {
	Tlds []*entities.TLD
}

// CreateTLD creates a TLD
func (repo *MockTLDRepository) Create(ctx context.Context, tld *entities.TLD) error {
	repo.Tlds = append(repo.Tlds, tld)
	return nil
}

// GetByName returns a TLD by name
func (repo *MockTLDRepository) GetByName(ctx context.Context, name string) (*entities.TLD, error) {
	for _, tld := range repo.Tlds {
		if tld.Name.String() == name {
			return tld, nil
		}
	}
	return nil, nil
}

// List returns a list of all TLDs
func (repo *MockTLDRepository) List(ctx context.Context, pageSize int, pageCursor string) ([]*entities.TLD, error) {
	return repo.Tlds, nil
}

// DeleteByName deletes a TLD by name
func (repo *MockTLDRepository) DeleteByName(ctx context.Context, name string) error {
	for i, tld := range repo.Tlds {
		if tld.Name.String() == name {
			repo.Tlds = append(repo.Tlds[:i], repo.Tlds[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestTLDService_CreateTLD(t *testing.T) {
	tldRepo := MockTLDRepository{}
	service := NewTLDService(&tldRepo)

	tld, err := entities.NewTLD("com")
	if err != nil {
		t.Error(err)
	}

	cmd := getCreateTLDCommand(tld)

	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	if len(tldRepo.Tlds) != 1 {
		t.Errorf("Expected 1 tld, got %d", len(tldRepo.Tlds))
	}

}

func getCreateTLDCommand(tld *entities.TLD) *commands.CreateTLDCommand {
	return &commands.CreateTLDCommand{
		Name: tld.Name.String(),
	}
}

func TestTLDService_GetTLDByName(t *testing.T) {
	tldRepo := MockTLDRepository{}
	service := NewTLDService(&tldRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Get the first TLD
	tld, err = service.GetTLDByName(context.Background(), "apex")
	if err != nil {
		t.Error(err)
	}
	if tld.Name.String() != "apex" {
		t.Errorf("Expected apex, got %s", tld.Name.String())
	}

	// Get the second TLD
	tld, err = service.GetTLDByName(context.Background(), "com.apex")
	if err != nil {
		t.Error(err)
	}
	if tld.Name.String() != "com.apex" {
		t.Errorf("Expected com.apex, got %s", tld.Name.String())
	}
}

func TestTLDService_ListTLDs(t *testing.T) {
	tldRepo := MockTLDRepository{}
	service := NewTLDService(&tldRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, err := service.ListTLDs(context.Background(), 100, "")
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 2 {
		t.Errorf("Expected 2 tlds, got %d", len(tlds))
	}
}

func TestTLDService_DeleteTLDByName(t *testing.T) {
	tldRepo := MockTLDRepository{}
	service := NewTLDService(&tldRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Delete the first TLD
	err = service.DeleteTLDByName(context.Background(), "apex")
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, err := service.ListTLDs(context.Background(), 100, "")
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 1 {
		t.Errorf("Expected 1 tld, got %d", len(tlds))
	}

	// Delete the second TLD
	err = service.DeleteTLDByName(context.Background(), "com.apex")
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, err = service.ListTLDs(context.Background(), 100, "")
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 0 {
		t.Errorf("Expected 0 tlds, got %d", len(tlds))
	}
}
