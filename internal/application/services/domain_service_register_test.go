package services

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A registration reads the TLD once: the availability check and the quote
// work from the phase it gives, and the domain is read once, to see that
// it is not registered.
func TestRegisterDomainReadsTheTLDOnce(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	nndnRepo := new(mockNNDNRepository)
	rarRepo := new(repositories.MockRegistrarRepository)
	eventPub := new(batchMockEventPublisher)
	svc := newTestBatchDomainService(domRepo, repositories.NewMockHostRepository(), rarRepo, tldRepo, nndnRepo,
		eventPub, new(mockPhaseRepository), new(mockPremiumLabelRepository), new(mockFXRepository))

	tld := newTestTLDWithGAPhase("com")
	price, err := entities.NewPrice("USD", 1000, 1000, 1000, 3000)
	require.NoError(t, err)
	_, err = tld.Phases[0].AddPrice(*price)
	require.NoError(t, err)
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)
	rarRepo.On("IsRegistrarAccreditedForTLD", mock.Anything, "com", "rar-on").Return(true, nil)
	domRepo.On("GetDomainByName", mock.Anything, "once.com", false).Return((*entities.Domain)(nil), entities.ErrDomainNotFound)
	nndnRepo.On("GetNNDN", mock.Anything, "once.com").Return(nil, entities.ErrNNDNNotFound)
	created := newTestDomain("once.com", "rar-on", "6001_DOM-TEST", tld.Phases[0].Starts.AddDate(1, 0, 0))
	domRepo.On("Create", mock.Anything, mock.Anything).Return(created, nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	dom, err := svc.RegisterDomain(context.Background(), &commands.RegisterDomainCommand{Name: "once.com", ClID: "rar-on", Years: 1, RegistrantID: "reg-1", AdminID: "adm-1", TechID: "tec-1"})
	require.NoError(t, err)
	require.Equal(t, entities.DomainName("once.com"), dom.Name)

	tldRepo.AssertNumberOfCalls(t, "GetByName", 1)
	domRepo.AssertNumberOfCalls(t, "GetDomainByName", 1)
}
