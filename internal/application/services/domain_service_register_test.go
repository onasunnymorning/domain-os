package services

import (
	"context"
	"testing"
	"time"

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

// registerTestService wires a DomainService whose TLD has a GA phase and an
// ended launch phase, both priced, for the phase-resolution tests below.
func registerTestService(t *testing.T) (*DomainService, *entities.TLD, *repositories.MockDomainRepository) {
	t.Helper()
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	nndnRepo := new(mockNNDNRepository)
	rarRepo := new(repositories.MockRegistrarRepository)
	eventPub := new(batchMockEventPublisher)
	svc := newTestBatchDomainService(domRepo, repositories.NewMockHostRepository(), rarRepo, tldRepo, nndnRepo,
		eventPub, new(mockPhaseRepository), new(mockPremiumLabelRepository), new(mockFXRepository))

	tld := newTestTLDWithGAPhase("com")
	gaPrice, err := entities.NewPrice("USD", 1000, 1000, 1000, 3000)
	require.NoError(t, err)
	_, err = tld.Phases[0].AddPrice(*gaPrice)
	require.NoError(t, err)

	ended, err := entities.NewPhase("sunrise", string(entities.PhaseTypeLaunch), time.Now().UTC().AddDate(0, -3, 0))
	require.NoError(t, err)
	endedAt := time.Now().UTC().AddDate(0, -1, 0)
	ended.Ends = &endedAt
	ended.TLDName = "com"
	require.NoError(t, tld.AddPhase(ended))

	landrush, err := entities.NewPhase("landrush", string(entities.PhaseTypeLaunch), time.Now().UTC().AddDate(0, 0, -1))
	require.NoError(t, err)
	landrush.Launch = entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true}
	landrushPrice, err := entities.NewPrice("USD", 5000, 1000, 1000, 3000)
	require.NoError(t, err)
	_, err = landrush.AddPrice(*landrushPrice)
	require.NoError(t, err)
	landrush.TLDName = "com"
	require.NoError(t, tld.AddPhase(landrush))

	applications, err := entities.NewPhase("auction", string(entities.PhaseTypeLaunch), time.Now().UTC().AddDate(0, 0, -1))
	require.NoError(t, err)
	applications.Launch = entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true}
	applications.TLDName = "com"
	require.NoError(t, tld.AddPhase(applications))

	sunrise, err := entities.NewPhase("sunrise-fc", string(entities.PhaseTypeLaunch), time.Now().UTC().AddDate(0, 0, -1))
	require.NoError(t, err)
	sunrise.Launch = entities.LaunchSettings{Mode: entities.LaunchModeSunrise, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true}
	sunrise.TLDName = "com"
	require.NoError(t, tld.AddPhase(sunrise))

	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)
	tldRepo.On("GetWithPhases", mock.Anything, "com").Return(tld, nil)
	rarRepo.On("IsRegistrarAccreditedForTLD", mock.Anything, "com", "rar-on").Return(true, nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)
	return svc, tld, domRepo
}

// A registration names a launch phase or nothing: an ended launch phase, a
// future one and the GA phase by name are refused before anything is read
// or written (ADR 0024).
func TestRegisterDomainRefusesPhasesItCannotRegisterIn(t *testing.T) {
	svc, _, domRepo := registerTestService(t)
	for _, tc := range []struct {
		phase string
		err   error
	}{
		{"sunrise", entities.ErrPhaseNotActive},
		{"GA-phase", entities.ErrPhaseNotLaunch},
		{"nope", entities.ErrPhaseNotFound},
		// Active, but the phase collects applications, or needs verification
		// that does not exist yet: a registration would claim the name.
		{"auction", entities.ErrLaunchApplicationsUnavailable},
		{"sunrise-fc", entities.ErrLaunchModeUnsupported},
	} {
		_, err := svc.RegisterDomain(context.Background(), &commands.RegisterDomainCommand{
			Name: "late.com", ClID: "rar-on", Years: 1, PhaseName: tc.phase, RegistrantID: "reg-1", AdminID: "adm-1", TechID: "tec-1"})
		require.ErrorIs(t, err, tc.err, tc.phase)
	}
	domRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// A registration in an active launch phase is priced by that phase and the
// domain remembers it; one without a phase is the GA phase's and does not.
func TestRegisterDomainInLaunchPhase(t *testing.T) {
	svc, tld, domRepo := registerTestService(t)
	nndn := svc.nndnRepo.(*mockNNDNRepository)
	nndn.On("GetNNDN", mock.Anything, mock.Anything).Return(nil, entities.ErrNNDNNotFound)
	domRepo.On("GetDomainByName", mock.Anything, mock.Anything, false).Return((*entities.Domain)(nil), entities.ErrDomainNotFound)
	var written []*entities.Domain
	domRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		written = append(written, args.Get(1).(*entities.Domain))
	}).Return(newTestDomain("x.com", "rar-on", "6002_DOM-TEST", tld.Phases[0].Starts.AddDate(1, 0, 0)), nil)

	_, err := svc.RegisterDomain(context.Background(), &commands.RegisterDomainCommand{
		Name: "rush.com", ClID: "rar-on", Years: 1, PhaseName: "landrush", RegistrantID: "reg-1", AdminID: "adm-1", TechID: "tec-1",
		Fee: commands.FeeExtension{Currency: "USD", Amount: 5000}})
	require.NoError(t, err, "the landrush price is what the fee extension must match")
	require.Len(t, written, 1)
	require.Equal(t, entities.ClIDType("landrush"), written[0].RegistrationPhaseName)

	_, err = svc.RegisterDomain(context.Background(), &commands.RegisterDomainCommand{
		Name: "plain.com", ClID: "rar-on", Years: 1, RegistrantID: "reg-1", AdminID: "adm-1", TechID: "tec-1",
		Fee: commands.FeeExtension{Currency: "USD", Amount: 5000}})
	require.Error(t, err, "without a phase the GA price applies, so the landrush amount is a mismatch")

	_, err = svc.RegisterDomain(context.Background(), &commands.RegisterDomainCommand{
		Name: "plain.com", ClID: "rar-on", Years: 1, RegistrantID: "reg-1", AdminID: "adm-1", TechID: "tec-1"})
	require.NoError(t, err)
	require.Len(t, written, 2)
	require.Empty(t, written[1].RegistrationPhaseName)
}

// Availability follows the same rule as registration: the phase named must
// be an active launch phase, else the GA phase in force applies. It reads
// the TLD with its phases only, never the priced form with its counts.
func TestCheckDomainAvailabilityResolvesThePhaseAsRegistrationDoes(t *testing.T) {
	svc, _, domRepo := registerTestService(t)
	tldRepo := svc.tldRepo.(*mockTLDRepository)
	defer tldRepo.AssertNotCalled(t, "GetByName", mock.Anything, mock.Anything, mock.Anything)
	nndn := svc.nndnRepo.(*mockNNDNRepository)
	nndn.On("GetNNDN", mock.Anything, mock.Anything).Return(nil, entities.ErrNNDNNotFound)
	domRepo.On("GetDomainByName", mock.Anything, mock.Anything, false).Return((*entities.Domain)(nil), entities.ErrDomainNotFound)

	res, err := svc.CheckDomainAvailability(context.Background(), "free.com", "")
	require.NoError(t, err)
	require.True(t, res.Available)
	require.Equal(t, "GA-phase", res.PhaseName)

	res, err = svc.CheckDomainAvailability(context.Background(), "free.com", "landrush")
	require.NoError(t, err)
	require.True(t, res.Available)
	require.Equal(t, "landrush", res.PhaseName)

	res, err = svc.CheckDomainAvailability(context.Background(), "free.com", "sunrise")
	require.ErrorIs(t, err, entities.ErrPhaseNotActive)
	require.False(t, res.Available)
	require.Equal(t, "sunrise", res.PhaseName)

	_, err = svc.CheckDomainAvailability(context.Background(), "free.com", "GA-phase")
	require.ErrorIs(t, err, entities.ErrPhaseNotLaunch)
}
