package services

import (
	"errors"
	"strings"

	"log"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

var (
	// ErrDomainExists is returned when a domain already exists
	ErrDomainExists = errors.New("domain exists")
	// ErrDomainBlocked is returned when a domain is blocked
	ErrDomainBlocked = errors.New("domain is blocked")
	// ErrPhaseRequired is returned when a phase is required to check domain availability
	ErrPhaseRequired = errors.New("phase is required to check domain availability")
	// ErrLabelNotValidInPhase is returned when a label is not valid in a phase
	ErrLabelNotValidInPhase = errors.New("label is not valid in this phase")
)

// DomainService immplements the DomainService interface
type DomainService struct {
	domainRepository repositories.DomainRepository
	hostRepository   repositories.HostRepository
	roidService      RoidService
	nndnRepo         repositories.NNDNRepository
	tldRepo          repositories.TLDRepository
	phaseRepo        repositories.PhaseRepository
	premiumLabelRepo repositories.PremiumLabelRepository
	fxRepo           repositories.FXRepository
}

// NewDomainService returns a new instance of a DomainService
func NewDomainService(
	dRepo repositories.DomainRepository,
	hRepo repositories.HostRepository,
	roidService RoidService,
	nndrepo repositories.NNDNRepository,
	tldRepo repositories.TLDRepository,
	phr repositories.PhaseRepository,
	plr repositories.PremiumLabelRepository,
	fxr repositories.FXRepository,
) *DomainService {
	return &DomainService{
		domainRepository: dRepo,
		hostRepository:   hRepo,
		roidService:      roidService,
		nndnRepo:         nndrepo,
		tldRepo:          tldRepo,
		phaseRepo:        phr,
		premiumLabelRepo: plr,
		fxRepo:           fxr,
	}
}

// CreateDomain creates a new domain from a create domain command
func (s *DomainService) CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) (*entities.Domain, error) {
	var roid entities.RoidType
	var err error
	if cmd.RoID == "" {
		// Generate a RoID if none is supplied
		roid, err = s.roidService.GenerateRoid("domain")
		if err != nil {
			return nil, err
		}
	} else {
		roid = entities.RoidType(cmd.RoID) // Validity will be checked in NewDomain
	}
	d, err := entities.NewDomain(roid.String(), cmd.Name, cmd.ClID, cmd.AuthInfo)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	// Set the optional fields
	if cmd.OriginalName != "" {
		d.OriginalName = entities.DomainName(strings.ToLower(cmd.OriginalName))
	}
	if cmd.UName != "" {
		d.UName = entities.DomainName(strings.ToLower(cmd.UName))
	}
	if cmd.RegistrantID != "" {
		d.RegistrantID = entities.ClIDType(cmd.RegistrantID)
	}
	if cmd.AdminID != "" {
		d.AdminID = entities.ClIDType(cmd.AdminID)
	}
	if cmd.TechID != "" {
		d.TechID = entities.ClIDType(cmd.TechID)
	}
	if cmd.BillingID != "" {
		d.BillingID = entities.ClIDType(cmd.BillingID)
	}
	if cmd.CrRr != "" {
		d.CrRr = entities.ClIDType(cmd.CrRr)
	}
	if cmd.UpRr != "" {
		d.UpRr = entities.ClIDType(cmd.UpRr)
	}
	if !cmd.ExpiryDate.IsZero() {
		d.ExpiryDate = cmd.ExpiryDate
	}
	if !cmd.CreatedAt.IsZero() {
		d.CreatedAt = cmd.CreatedAt
	}
	if !cmd.UpdatedAt.IsZero() {
		d.UpdatedAt = cmd.UpdatedAt
	}
	if !cmd.Status.IsNil() {
		d.Status = cmd.Status
	}
	if !cmd.RGPStatus.IsNil() {
		d.RGPStatus = cmd.RGPStatus
	}
	// Check if the domain is valid
	if err := d.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// Save the domain
	createdDomain, err := s.domainRepository.CreateDomain(ctx, d)
	if err != nil {
		if errors.Is(err, entities.ErrDomainAlreadyExists) {
			return nil, errors.Join(entities.ErrInvalidDomain, err)
		}
		return nil, err
	}

	return createdDomain, nil
}

// UpdateDomain Updates a new domain from a create domain command
func (s *DomainService) UpdateDomain(ctx context.Context, name string, upDom *commands.UpdateDomainCommand) (*entities.Domain, error) {
	// Look up the domain
	dom, err := s.domainRepository.GetDomainByName(ctx, name, false)
	if err != nil {
		return nil, err
	}
	// Make the changes
	dom.OriginalName = entities.DomainName(upDom.OriginalName)
	dom.UName = entities.DomainName(upDom.UName)
	dom.RegistrantID = entities.ClIDType(upDom.RegistrantID)
	dom.AdminID = entities.ClIDType(upDom.AdminID)
	dom.TechID = entities.ClIDType(upDom.TechID)
	dom.BillingID = entities.ClIDType(upDom.BillingID)
	dom.CrRr = entities.ClIDType(upDom.CrRr)
	dom.UpRr = entities.ClIDType(upDom.UpRr)
	dom.ExpiryDate = upDom.ExpiryDate
	dom.AuthInfo = entities.AuthInfoType(upDom.AuthInfo)
	dom.CreatedAt = upDom.CreatedAt
	dom.UpdatedAt = upDom.UpdatedAt
	dom.Status = upDom.Status
	dom.RGPStatus = upDom.RGPStatus

	// Validate the domain
	if err := dom.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// Save and return
	updatedDomain, err := s.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	return updatedDomain, nil
}

// GetDomainByName retrieves a domain by its name from the repository
func (s *DomainService) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	return s.domainRepository.GetDomainByName(ctx, name, preloadHosts)
}

// DeleteDomainByName deletes a domain by its name from the repository
func (s *DomainService) DeleteDomainByName(ctx context.Context, name string) error {
	return s.domainRepository.DeleteDomainByName(ctx, name)
}

// ListDomains returns a list of domains
func (s *DomainService) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListDomains(ctx, pageSize, cursor)
}

// AddHostToDomain adds a host to a domain
func (s *DomainService) AddHostToDomain(ctx context.Context, name string, roid string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, true)
	if err != nil {
		return err
	}

	// Get the host
	hRoid := entities.RoidType(roid)
	if err := hRoid.Validate(); err != nil {
		return err
	}
	if hRoid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return entities.ErrInvalidHostRoID
	}
	hostRoidInt, err := hRoid.Int64()
	if err != nil {
		return err
	}

	host, err := s.hostRepository.GetHostByRoid(ctx, hostRoidInt)
	if err != nil {
		return err
	}

	// Add the host to the domain
	i, err := dom.AddHost(host)
	if err != nil {
		if errors.Is(err, entities.ErrDuplicateHost) {
			return nil // No error if the host is already associated, idempotent
		}
		return err
	}

	// Update the Domain which will save the association as well
	_, err = s.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return err
	}

	// Update the host to set the linked flag
	_, err = s.hostRepository.UpdateHost(ctx, dom.Hosts[i])
	if err != nil {
		return err
	}

	return nil
}

// RemoveHostFromDomain removes a host from a domain
func (s *DomainService) RemoveHostFromDomain(ctx context.Context, name string, roid string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, true)
	if err != nil {
		return err
	}

	domRoidInt, err := dom.RoID.Int64()
	if err != nil {
		return err
	}

	// Get the host
	hRoid := entities.RoidType(roid)
	if err := hRoid.Validate(); err != nil {
		return err
	}
	if hRoid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return entities.ErrInvalidHostRoID
	}
	hostRoidInt, err := hRoid.Int64()
	if err != nil {
		return err
	}

	host, err := s.hostRepository.GetHostByRoid(ctx, hostRoidInt)
	if err != nil {
		return err
	}

	// Remove the host from the domain
	err = dom.RemoveHost(host)
	if err != nil {
		return err
	}

	// Remove tha association
	err = s.domainRepository.RemoveHostFromDomain(ctx, domRoidInt, hostRoidInt)
	if err != nil {
		return err
	}

	// Save the domain, this will update the association
	_, err = s.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return err
	}
	// Check if the host is associated with any other domains
	count, err := s.hostRepository.GetHostAssociationCount(ctx, hostRoidInt)
	if err != nil {
		// Our operation is successful, but we can't determine if the host is associated with any other domains
		log.Printf("Failed to get host association count for host %s: %v", host.RoID.String(), err)
	}
	if count == 0 {
		// If not, unset the linked flag
		err := host.UnsetStatus(entities.HostStatusLinked)
		if err != nil {
			// Our operation is successful, but we can't unset the linked flag
			log.Printf("Failed to unset linked flag on host %s: %v", host.RoID.String(), err)
		}
		// Update the host
		_, err = s.hostRepository.UpdateHost(ctx, host)
		if err != nil {
			// Our operation is successful, but we can't update the host
			log.Printf("Failed to update host %s: %v", host.RoID.String(), err)
		}
	}
	return nil
}

// CheckDomainExists checks if a domain exists. If the domain exists, the function returns true, otherwise it returns false. If an error occurs, it is returned.
func (svc *DomainService) CheckDomainExists(ctx context.Context, domainName string) (bool, error) {
	_, err := svc.domainRepository.GetDomainByName(ctx, domainName, false)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckDomainIsBlocked checks if a domain is blocked. If the domain is blocked, the function returns true, otherwise it returns false. If an error occurs, it is returned.
func (svc *DomainService) CheckDomainIsBlocked(ctx context.Context, domainName string) (bool, error) {
	_, err := svc.nndnRepo.GetNNDN(ctx, domainName)
	if err != nil {
		if errors.Is(err, entities.ErrNNDNNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckDomainAvailability checks if a domain is available. A domain is availabel if
// * it is a valid domain name
// * it is allowed in the current phase
// * it does not exist
// * it is not blocked
func (svc *DomainService) CheckDomainAvailability(ctx context.Context, domainName string, phase *entities.Phase) (bool, error) {
	if phase == nil {
		return false, ErrPhaseRequired
	}
	dom, err := entities.NewDomainName(domainName)
	if err != nil {
		return false, err
	}

	// Check if the domain label is valid in the current phase
	if !phase.Policy.LabelIsAllowed(dom.Label()) {
		return false, ErrLabelNotValidInPhase
	}

	// Check if the domain exists
	exists, err := svc.CheckDomainExists(ctx, domainName)
	if err != nil {
		return false, err
	}
	if exists {
		return false, ErrDomainExists
	}

	// Check if the domain is blocked
	blocked, err := svc.CheckDomainIsBlocked(ctx, domainName)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, ErrDomainBlocked
	}

	// If all checks pass, the domain is available
	return true, nil
}

// CheckDomain checks the availability of a domain name
func (svc *DomainService) CheckDomain(ctx context.Context, q *queries.DomainCheckQuery) (*queries.DomainCheckResult, error) {
	// Make sure the currency is uppercased
	q.Currency = strings.ToUpper(q.Currency)
	// check if the TLD exists
	tld, err := svc.tldRepo.GetByName(ctx, q.DomainName.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// if the phase is not provided, get the current GA phase
	var phase *entities.Phase
	if q.PhaseName == "" {
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
	} else { // if the phase is provided, get the phase by name
		phase, err = tld.FindPhaseByName(entities.ClIDType(q.PhaseName))
		if err != nil {
			return nil, err
		}
	}

	avail, err := svc.CheckDomainAvailability(ctx, q.DomainName.String(), phase)
	if err != nil && !errors.Is(err, ErrDomainExists) && !errors.Is(err, ErrDomainBlocked) && !errors.Is(err, ErrLabelNotValidInPhase) {
		return nil, err
	}
	// Create the result object
	result := queries.NewDomainCheckQueryResult(q.DomainName)
	// set the phase name
	result.PhaseName = phase.Name.String()
	// set the availability and reason
	result.Available = avail
	if !avail {
		result.Reason = err.Error()
	}

	// So far so good, the domain doesn't exist and is not blocked
	// Return the result now if fees are not required
	if !q.IncludeFees {
		return result, nil
	}
	// If fees are requested, prepare the result
	result.PricePoints = &queries.DomainPricePoints{}

	// Get the full phase from the repo to ensure preloading the price and fee objects
	phase, err = svc.phaseRepo.GetPhaseByTLDAndName(ctx, tld.Name.String(), phase.Name.String())
	if err != nil {
		return nil, err
	}

	var needsFX bool // Flag to check if we need to convert the price to the requested currency

	// set the price for the currency.
	result.PricePoints.Price, err = phase.GetPrice(q.Currency)
	if errors.Is(err, entities.ErrPriceNotFound) && q.Currency != phase.Policy.BaseCurrency {
		// If the price is not found for the requested currency, try to get the price in the base currency
		result.PricePoints.Price, _ = phase.GetPrice(phase.Policy.BaseCurrency)
		if result.PricePoints.Price != nil {
			needsFX = true
		}
	}

	// set the fees for the currency
	result.PricePoints.Fees = phase.GetFees(q.Currency)
	if len(result.PricePoints.Fees) == 0 && q.Currency != phase.Policy.BaseCurrency {
		// If the fees are not found for the requested currency, try to get the fees in the base currency
		result.PricePoints.Fees = phase.GetFees(phase.Policy.BaseCurrency)
		if len(result.PricePoints.Fees) > 0 {
			needsFX = true
		}
	}

	// Get the PremiumLabels for the premiumList associated with the phase
	if phase.PremiumListName != nil {
		result.PricePoints.PremiumPrice, err = svc.premiumLabelRepo.GetByLabelListAndCurrency(ctx, q.DomainName.Label(), *phase.PremiumListName, q.Currency)
		if err != nil && !errors.Is(err, entities.ErrPremiumLabelNotFound) {
			return nil, err
		}
		// If the premium price is not found for the requested currency, try to get the premium price in the base currency
		if errors.Is(err, entities.ErrPremiumLabelNotFound) && q.Currency != phase.Policy.BaseCurrency {
			result.PricePoints.PremiumPrice, _ = svc.premiumLabelRepo.GetByLabelListAndCurrency(ctx, q.DomainName.Label(), *phase.PremiumListName, phase.Policy.BaseCurrency)
			if result.PricePoints.PremiumPrice != nil {
				needsFX = true
			}
		}
	}

	// If we need to convert currencies, include the FX rate
	if needsFX {
		result.PricePoints.FX, err = svc.fxRepo.GetByBaseAndTargetCurrency(phase.Policy.BaseCurrency, q.Currency)
		if err != nil {
			return nil, err
		}
	}

	// retrun the result
	return result, nil
}

// RegisterDomain registers a domain
func (svc *DomainService) RegisterDomain(ctx context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error) {
	// Check if the domain is available
	includeFees := cmd.Fee == commands.FeeExtension{} // If the fee extension is proivded, include the fees in the check
	q, err := queries.NewDomainCheckQuery(cmd.Name, includeFees)
	if err != nil {
		return nil, err
	}
	if cmd.PhaseName != "" {
		q.PhaseName = cmd.PhaseName
	}
	if includeFees {
		q.Currency = cmd.Fee.Currency
	}
	checkResult, err := svc.CheckDomain(ctx, q)
	if err != nil {
		return nil, err
	}
	if !checkResult.Available {
		return nil, errors.Join(entities.ErrInvalidDomain, errors.New(checkResult.Reason))
	}

	//TODO: FIXME do a fee check here - dependent on currency conversion

	// Get the Phase through the TLD
	domainName := entities.DomainName(cmd.Name)
	tld, err := svc.tldRepo.GetByName(ctx, domainName.ParentDomain(), true)
	if err != nil {
		return nil, err
	}
	var phase *entities.Phase
	if cmd.PhaseName == "" {
		// If no phase is provided we will use the current GA phase
		phase, err = tld.GetCurrentGAPhase()
	} else {
		// If a phase is provided we will use that
		phase, err = tld.FindPhaseByName(entities.ClIDType(cmd.PhaseName))
	}
	if err != nil {
		return nil, err
	}

	// Generate a RoID
	roid, err := svc.roidService.GenerateRoid(entities.RoidTypeDomain)
	if err != nil {
		return nil, err
	}

	// Create the domain entity
	dom, err := entities.RegisterDomain(roid.String(), cmd.Name, cmd.ClID, cmd.AuthInfo, cmd.RegistrantID, cmd.AdminID, cmd.TechID, cmd.BillingID, phase, cmd.Years)
	if err != nil {
		return nil, err
	}

	// Add the hosts if there are any
	for _, h := range cmd.HostNames {
		// Lookup the host
		host, err := svc.hostRepository.GetHostByNameAndClID(ctx, strings.ToLower(h), cmd.ClID)
		if err != nil {
			return nil, err
		}
		// Add the host to the domain
		_, err = dom.AddHost(host)
		if err != nil {
			return nil, err
		}
	}

	// Save the domain
	// This should save the host associtations as well => to be tested
	createdDomain, err := svc.domainRepository.CreateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	// return
	return createdDomain, nil

}

// RenewDomain renews a domain
func (svc *DomainService) RenewDomain(ctx context.Context, cmd *commands.RenewDomainCommand) (*entities.Domain, error) {
	// Get the domain wihtout the hosts
	dom, err := svc.GetDomainByName(ctx, cmd.Name, false)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidRenewal, err)
	}

	// Check if the Current Registrar ClID matches the command
	if dom.ClID != entities.ClIDType(cmd.ClID) {
		return nil, errors.Join(entities.ErrInvalidRenewal, entities.ErrInvalidRegistrar)
	}

	// Get the TLD including the phases
	tld, err := svc.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// Always use the current Ga phase policy for renewals (phase extention does not apply to renews)
	phase, err := tld.GetCurrentGAPhase()
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidRenewal, err)
	}

	// Renew the domain using our entity
	err = dom.Renew(cmd.Years, false, phase)
	if err != nil {
		return nil, err
	}

	// Save the domain
	updatedDomain, err := svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	return updatedDomain, nil
}

// MarkForDelete marks a domain for deletion
func (svc *DomainService) MarkDomainForDeletion(ctx context.Context, domainName string) (*entities.Domain, error) {
	// Get the domain
	dom, err := svc.GetDomainByName(ctx, domainName, false)
	if err != nil {
		return nil, err
	}

	// Get the TLD and phases
	tld, err := svc.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// Get the current GA phase
	phase, err := tld.GetCurrentGAPhase()
	if err != nil {
		return nil, err
	}

	// Mark the domain for deletion
	err = dom.MarkForDeletion(phase)
	if err != nil {
		return nil, err
	}

	// Save the domain
	updatedDomain, err := svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	return updatedDomain, nil
}

// RestoreDomain restores a domain. It does a soft restore by setting the status tu pendingRestore. Another process will pick this up and complete the restore.
func (svc *DomainService) RestoreDomain(ctx context.Context, domainName string) (*entities.Domain, error) {
	// Get the domain
	dom, err := svc.GetDomainByName(ctx, domainName, false)
	if err != nil {
		return nil, err
	}

	// Restore the domain
	err = dom.Restore()
	if err != nil {
		return nil, err
	}

	// Save the domain
	updatedDomain, err := svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	return updatedDomain, nil
}

// DropCatch sets or unsets the DropCatch flag on a domain
func (svc *DomainService) DropCatchDomain(ctx context.Context, domainName string, dropcatch bool) error {
	// Get the domain
	dom, err := svc.GetDomainByName(ctx, domainName, false)
	if err != nil {
		return err
	}

	// Set or unset the DropCatch flag
	if dropcatch {
		dom.DropCatch = true
	} else {
		dom.DropCatch = false
	}

	// Save the domain
	_, err = svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return err
	}

	return nil
}
