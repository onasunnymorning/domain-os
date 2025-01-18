package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"log"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	// ErrDomainExists is returned when a domain already exists
	ErrDomainExists = errors.New("domain exists")
	// ErrDomainBlocked is returned when a domain is blocked
	ErrDomainBlocked = errors.New("domain is blocked")
	// ErrPhaseRequired is returned when a phase is required to check domain availability
	ErrPhaseRequired = errors.New("phase is required to check domain availability")
	// ErrAutoRenewNotEnabledRar is returned when auto renew is not enabled for the registrar
	ErrAutoRenewNotEnabledRar = errors.New("auto renew is not enabled for this registrar")
	// ErrAutoRenewNotEnabledRar is returned when auto renew is not enabled for the TLD
	ErrAutoRenewNotEnabledTLD = errors.New("auto renew is not enabled for this TLD")
	// ErrRegistrarNotAccredited is returned when the registrar is not accredited for the TLD
	ErrRegistrarNotAccredited = errors.New("registrar is not accredited for this TLD")
	// ErrCouldNotDetermineAccreditation is returned when the accreditation could not be determined
	ErrCouldNotDetermineAccreditation = errors.New("could not determine accreditation")
	// ErrMissingFXRate is returned when the FX rate is required but can't be determined
	ErrMissingFXRate = errors.New("missing FX rate")
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
	rarRepo          repositories.RegistrarRepository
	logger           *zap.Logger
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
	rRepo repositories.RegistrarRepository,
) *DomainService {
	logger, _ := zap.NewProduction()
	return &DomainService{
		domainRepository: dRepo,
		hostRepository:   hRepo,
		roidService:      roidService,
		nndnRepo:         nndrepo,
		tldRepo:          tldRepo,
		phaseRepo:        phr,
		premiumLabelRepo: plr,
		fxRepo:           fxr,
		rarRepo:          rRepo,
		logger:           logger,
	}
}

// CreateDomain creates a new domain using the provided CreateDomainCommand.
// It generates a RoID if none is provided, sets optional fields from the command,
// validates the resulting domain entity, and persists it in the domain repository.
// Returns the created Domain or an error if validation fails or persistence is unsuccessful.
// It optionally validates the domain against the current GA phase polify if so stated in the command.
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
	d.GrandFathering = cmd.GrandFathering
	d.RenewedYears = cmd.RenewedYears

	// Check if the domain is valid
	if err := d.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// If the phase is provided, check if the domain is valid in the phase
	if cmd.EnforcePhasePolicy {
		// Get the phase
		tld, err := s.tldRepo.GetByName(ctx, d.Name.ParentDomain(), true)
		if err != nil {
			return nil, err
		}
		phase, err := tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}

		// Check if the name is valid in this phase
		if !phase.Policy.LabelIsAllowed(d.Name.Label()) {
			return nil, errors.Join(entities.ErrInvalidDomain, entities.ErrLabelNotValidInPhase)
		}

		// Apply the contact data policy
		err = d.ApplyContactDataPolicy(phase.Policy.ContactDataPolicy)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidDomain, err)
		}
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

// UpdateDomain updates the details of an existing domain identified by its name.
// It retrieves the domain from the repository, applies the changes specified in upDom,
// optionally validates the domain against the current GA phase policy if so stated in the command,
//
// Parameters:
//   - ctx: Context for the operation, enabling cancellation and deadlines.
//   - name: The identifier (name) of the domain to be updated.
//   - upDom: The command containing new domain data such as names and contact info.
//   - phase: Optional phase data containing policy rules for contact data validation.
//
// Returns:
//   - A pointer to the updated domain if successful.
//   - An error if retrieval, validation, or updating fails.
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
	dom.GrandFathering = upDom.GrandFathering

	// Validate the domain
	if err := dom.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}

	// If the phase is provided, check if the domain contacts are valid in the phase
	if upDom.EnforcePhasePolicy {
		// Get the phase
		tld, err := s.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
		if err != nil {
			return nil, err
		}
		phase, err := tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
		// Since this is an update we don't check the validity of the label as it already exists and doesn't change

		// Apply the contact data policy
		err = dom.ApplyContactDataPolicy(phase.Policy.ContactDataPolicy)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidDomain, err)
		}
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

// DeleteDomainByName deletes a domain identified by its name.
// This is an admin delete and will remove the domain from the system unless the repository does not allow it.
// If you want to purge a domain as part of its normal domain lifecycle, use the PurgeDomain method.
// It takes a context for managing request-scoped values and cancellation,
// and the name of the domain to be deleted.
// It returns an error if the deletion fails.
func (s *DomainService) DeleteDomainByName(ctx context.Context, name string) error {
	// We don't want to fail of a deleting a domain because it doesn't exist - idemp.
	prevState, _ := s.GetDomainByName(ctx, name, false)

	err := s.domainRepository.DeleteDomainByName(ctx, name)
	if err != nil {
		return err
	}

	// log a lifecycle event
	clid := "n/a" // in case the domain doesn't exist
	if prevState != nil {
		// if we have the information we use it
		clid = prevState.ClID.String()
	}
	dom := entities.DomainName(name)
	event, err := entities.NewDomainLifeCycleEvent(
		clid,
		"",
		dom.ParentDomain(),
		name,
		0,
		entities.TransactionTypeAdminDelete,
	)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Domain %s ADMIN deleted", name)
	s.logDomainLifecycleEvent(ctx, msg, event, nil, nil, prevState)

	return nil
}

// PurgeDomain deletes a domain by its name if it meets certain conditions.
// It performs the following steps:
//
// 1. Retrieves the domain by its name.
//
// 2. Checks if the domain can be purged. (if the purge date has passed)
//
// 3. If the domain has associated hosts, it dissociates all hosts.
//
// 4. If the domain is flagged for DropCatching, it creates an NNDN record.
//
// 5. Deletes the domain from the repository.
//
// 6. Logs a lifecycle event for the domain.
func (s *DomainService) PurgeDomain(ctx context.Context, name string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, false)
	if err != nil {
		return err
	}

	// Check if the domain can be purged
	if !dom.CanBePurged() {
		return errors.Join(entities.ErrDomainDeleteNotAllowed, errors.New("the purge date is in the future"))
	}

	// Check if the domain is linked to any hosts
	if len(dom.Hosts) > 0 {
		// Dissasociate all hosts if there are any
		err := s.RemoveAllDomainHosts(ctx, name)
		if err != nil {
			return err
		}
	}

	// If the domain is flaged for DropCatching, create an NNDN record first
	var createdNNDN *entities.NNDN
	if dom.DropCatch {
		// Create an NNDN record
		nndn, err := entities.NewNNDN(dom.Name.String())
		if err != nil {
			return err
		}
		// Set the reason
		nndn.Reason = "Domain.DropCatch is true"
		createdNNDN, err = s.nndnRepo.CreateNNDN(ctx, nndn)
		if err != nil {
			return err
		}
	}

	// Delete the domain
	err = s.domainRepository.DeleteDomainByName(ctx, name)
	if err != nil {
		return err
	}

	// Log a lifecycle event
	event, err := entities.NewDomainLifeCycleEvent(
		dom.ClID.String(),
		"",
		dom.Name.ParentDomain(),
		dom.Name.String(),
		0,
		entities.TransactionTypePurge,
	)
	if err != nil {
		return err
	}

	event.DomainRoID = dom.RoID.String()

	msg := fmt.Sprintf("Domain %s purged", name)
	s.logDomainLifecycleEvent(ctx, msg, event, nil, createdNNDN, dom)

	return nil

}

// ListDomains returns a list of domains
func (s *DomainService) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListDomains(ctx, pageSize, cursor)
}

// AddHostToDomain adds a host to a domain
func (s *DomainService) AddHostToDomain(ctx context.Context, name string, roid string, ignoreUpdateProhibitions bool) error {
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
	i, err := dom.AddHost(host, ignoreUpdateProhibitions)
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

// AddHostToDomainByHostName adds a host to a domain by host name
func (s *DomainService) AddHostToDomainByHostName(ctx context.Context, domainName, hostName string, ignoreUpdateProhibitions bool) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, domainName, true)
	if err != nil {
		return err
	}

	// Get the host by host name and clid
	host, err := s.hostRepository.GetHostByNameAndClID(ctx, strings.ToLower(hostName), dom.ClID.String())
	if err != nil {
		return err
	}

	// Add the host to the domain
	i, err := dom.AddHost(host, ignoreUpdateProhibitions)
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

// RemoveAllDomainHosts purges/dissasociates all hosts from a domain. It does not delete the hosts
func (s *DomainService) RemoveAllDomainHosts(ctx context.Context, name string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, name, true)
	if err != nil {
		return err
	}

	// Dissasociate all hosts
	for _, h := range dom.Hosts {
		err := dom.RemoveHost(h)
		if err != nil {
			return err
		}
		domRoid, err := dom.RoID.Int64()
		if err != nil {
			return err
		}
		hostRoid, err := h.RoID.Int64()
		if err != nil {
			return err
		}
		err = s.domainRepository.RemoveHostFromDomain(ctx, domRoid, hostRoid)
		if err != nil {
			return err
		}
	}

	// Save the domain
	_, err = s.domainRepository.UpdateDomain(ctx, dom)
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

// RemoveHostFromDomainByHostName removes a host from a domain
func (s *DomainService) RemoveHostFromDomainByHostName(ctx context.Context, domainName, hostName string) error {
	// Get the domain
	dom, err := s.GetDomainByName(ctx, domainName, true)
	if err != nil {
		return err
	}

	domRoidInt, err := dom.RoID.Int64()
	if err != nil {
		return err
	}

	// Get the host
	host, err := s.hostRepository.GetHostByNameAndClID(ctx, strings.ToLower(hostName), dom.ClID.String())
	if err != nil {
		return err
	}
	hostRoidInt, err := host.RoID.Int64()
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

// CheckDomainAvailability checks if a domain is available for registration.
// It performs the following checks:
// 1. Validates the domain name against the RFCs.
// 2. Checks if the domain already exists.
// 3. Checks if the domain is blocked.
// 4. Retrieves the phase by name if provided, otherwise gets the current GA phase.
// 5. Checks if the domain label is valid in the current phase.
//
// Parameters:
// - ctx: The context for the request.
// - domainName: The name of the domain to check.
// - phaseName: The name of the phase to check against.
//
// Returns:
// - A DomainCheckResult containing the availability status and reason if not available.
// - An error if any of the checks fail.
func (svc *DomainService) CheckDomainAvailability(ctx context.Context, domainName, phaseName string) (*queries.DomainCheckResult, error) {
	response := &queries.DomainCheckResult{
		TimeStamp:  time.Now().UTC(),
		Available:  false,
		Reason:     "",
		DomainName: domainName,
		PhaseName:  "n/a",
	}
	// NewDomainName will validate the domain name against the RFCs
	dom, err := entities.NewDomainName(domainName)
	if err != nil {
		response.Reason = err.Error()
		return response, err
	}

	// Check if the domain exists
	exists, err := svc.CheckDomainExists(ctx, domainName)
	if err != nil {
		response.Reason = err.Error()
		return response, err
	}
	if exists {
		response.Reason = ErrDomainExists.Error()
		return response, err
	}

	// Check if the domain is blocked
	blocked, err := svc.CheckDomainIsBlocked(ctx, domainName)
	if err != nil {
		response.Reason = err.Error()
		return response, err
	}
	if blocked {
		response.Reason = ErrDomainBlocked.Error()
		return response, err
	}

	// Retrieve the phase by name if provided, otherwise get the current GA phase
	// This will also error if the TLD does not exist
	phase := &entities.Phase{}
	if phaseName != "" {
		// Check the provided phase
		phase, err = svc.phaseRepo.GetPhaseByTLDAndName(ctx, dom.ParentDomain(), phaseName)
		if err != nil {
			response.PhaseName = phaseName
			response.Reason = err.Error()
			return response, err
		}
	} else {
		// Check the current GA phase by retrieving the TLD with preloaded phases
		tld, err := svc.tldRepo.GetByName(ctx, dom.ParentDomain(), true)
		if err != nil {
			response.Reason = err.Error()
			return response, err
		}
		// Get the current GA phase from the tld
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			response.Reason = err.Error()
			return response, err
		}
		response.PhaseName = phase.Name.String()
	}

	// Check if the domain label is valid in the current phase
	if !phase.Policy.LabelIsAllowed(dom.Label()) {
		response.Reason = entities.ErrLabelNotValidInPhase.Error()
		return response, errors.Join(entities.ErrInvalidDomain, entities.ErrLabelNotValidInPhase)
	}

	// If all checks pass, the domain is available
	response.Available = true
	return response, nil
}

// CheckDomain checks the availability of a domain name
// This was intended to mimic the EPP check command, but needs to be re-evaluated if that is the best approach
func (svc *DomainService) CheckDomain(ctx context.Context, q *queries.DomainCheckQuery) (*queries.DomainCheckResult, error) {
	// Make sure the currency is uppercased
	q.Currency = strings.ToUpper(q.Currency)

	// Check the availability of the domain in the phase or the current GA phase
	availability, err := svc.CheckDomainAvailability(ctx, q.DomainName.String(), q.PhaseName)
	if err != nil && !errors.Is(err, ErrDomainExists) && !errors.Is(err, ErrDomainBlocked) && !errors.Is(err, entities.ErrLabelNotValidInPhase) {
		return nil, err
	}
	// Create the result object
	result := queries.NewDomainCheckQueryResult(q.DomainName.String())
	// set the phase name
	result.PhaseName = q.PhaseName
	// set the availability and reason
	result.Available = availability.Available
	if !availability.Available {
		result.Reason = err.Error()
	}

	// So far so good, the domain doesn't exist and is not blocked
	// Return the result now if a quote is not requested
	if !q.GetQuote {
		return result, nil
	}

	// Get the quote
	quote, err := svc.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      q.DomainName.String(),
		ClID:            q.ClID.String(),
		TransactionType: entities.TransactionTypeRegistration,
		Currency:        q.Currency,
		Years:           1,
		PhaseName:       q.PhaseName,
	})
	if err != nil {
		return nil, err
	}
	result.Quote = quote

	// retrun the result
	return result, nil
}

// RegisterDomain registers a new domain based on the provided command parameters.
// It checks if the registrar is accredited for the TLD and the domain's availability, optionally validates fees (pending implementation), determines the
// relevant TLD and phase information, generates a unique ROID, creates the domain
// entity, attaches any specified hosts, and persists the resulting domain in the
// repository. It returns the created domain or an error if any step fails.
func (svc *DomainService) RegisterDomain(ctx context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error) {
	// Check if the registrar is accredited for the TLD
	domName := entities.DomainName(cmd.Name)
	isAccredited, err := svc.rarRepo.IsRegistrarAccreditedForTLD(ctx, domName.ParentDomain(), cmd.ClID)
	if err != nil {
		return nil, errors.Join(ErrCouldNotDetermineAccreditation, err)
	}
	if !isAccredited {
		dom := entities.DomainName(cmd.Name)
		return nil, errors.Join(ErrRegistrarNotAccredited, fmt.Errorf("Registrar.ClID: %s, TLD: %s", cmd.ClID, dom.ParentDomain()))
	}

	// Check if the domain is available
	includeFees := false // We will be removing includefees from DomainCheckQuery as this is replaced with QuoteRequest a bit down the line
	q, err := queries.NewDomainCheckQuery(cmd.Name, includeFees)
	if err != nil {
		return nil, err
	}
	q.ClID = entities.ClIDType(cmd.ClID)
	if cmd.PhaseName != "" {
		q.PhaseName = cmd.PhaseName
	}
	if includeFees {
		q.Currency = cmd.Fee.Currency
	}
	checkResult, err := svc.CheckDomainAvailability(ctx, cmd.Name, cmd.PhaseName)
	if err != nil {
		return nil, err
	}

	// If the domain is not available, return now
	if !checkResult.Available {
		return nil, errors.Join(entities.ErrInvalidDomain, errors.New(checkResult.Reason))
	}

	// Create a lifecycle event for logging
	event, err := entities.NewDomainLifeCycleEvent(
		cmd.ClID,
		"",
		domName.ParentDomain(),
		domName.String(),
		cmd.Years,
		entities.TransactionTypeRegistration,
	)
	if err != nil {
		return nil, err
	}

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

	// Get a quote
	var cur string
	// If the currency is not specified, use the base currency of the Registrar
	if cmd.Fee.Currency == "" {
		cur = phase.Policy.BaseCurrency
	} else {
		cur = cmd.Fee.Currency
	}
	quote, err := svc.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      cmd.Name,
		ClID:            cmd.ClID,
		TransactionType: entities.TransactionTypeRegistration,
		Currency:        cur,
		Years:           cmd.Years,
		PhaseName:       cmd.PhaseName,
	})
	if err != nil {
		return nil, err
	}
	event.Quote = *quote
	checkResult.Quote = quote

	// TODO: do something with the quote
	// We should somehow compare the Quote with the FeeExtension
	// Because of the currency conversion, we need to check if the fee is within a certain range instead of an exact match
	// This requiress some thought and is not implemented yet
	// Ref: https://github.com/onasunnymorning/domain-os/issues/225

	// Generate a RoID for our new domain
	roid, err := svc.roidService.GenerateRoid(entities.RoidTypeDomain)
	if err != nil {
		return nil, err
	}
	event.DomainRoID = roid.String()

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
		_, err = dom.AddHost(host, false)
		if err != nil {
			return nil, err
		}
	}

	// Save the domain including optional host associations
	createdDomain, err := svc.domainRepository.CreateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}

	// Log the domain registration
	msg := fmt.Sprintf("Domain %s registered by %s for %d years", cmd.Name, cmd.ClID, cmd.Years)
	svc.logDomainLifecycleEvent(ctx, msg, event, cmd, createdDomain, nil)

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

	// Create a lifecycle event for logging
	domName := entities.DomainName(cmd.Name)
	event, err := entities.NewDomainLifeCycleEvent(
		cmd.ClID,
		"",
		domName.ParentDomain(),
		domName.String(),
		cmd.Years,
		entities.TransactionTypeRenewal,
	)
	if err != nil {
		return nil, err
	}

	// Get a quote
	var cur string
	// If the currency is not specified, use the base currency of the Registrar
	if cmd.Fee.Currency == "" {
		cur = phase.Policy.BaseCurrency
	} else {
		cur = cmd.Fee.Currency
	}
	quote, err := svc.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      cmd.Name,
		ClID:            cmd.ClID,
		TransactionType: entities.TransactionTypeRenewal,
		Currency:        cur,
		Years:           cmd.Years,
		PhaseName:       phase.Name.String(),
	})
	if err != nil {
		return nil, err
	}
	event.Quote = *quote

	// save the previous state
	prevState := dom.Clone()

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

	event.DomainRoID = updatedDomain.RoID.String()
	// Log the domain renewal
	msg := fmt.Sprintf("Domain %s renewed by %s for %d years", cmd.Name, cmd.ClID, cmd.Years)
	svc.logDomainLifecycleEvent(ctx, msg, event, cmd, updatedDomain, prevState)

	return updatedDomain, nil
}

// CanAutoRenew checks if a domain can be auto-renewed based on several conditions:
// 1. Retrieves the domain by its name without including the hosts.
// 2. Retrieves the TLD (Top-Level Domain) including its phases.
// 3. Uses the current General Availability (GA) phase policy to determine if auto-renewal is allowed.
// 4. Checks if the registrar has opted in for auto-renewal.
//
// Parameters:
// - ctx: The context for managing request-scoped values, cancellation, and deadlines.
// - domainName: The name of the domain to check for auto-renewal eligibility.
//
// Returns:
// - bool: True if the domain can be auto-renewed, false otherwise.
// - error: An error if any step in the process fails.
func (svc *DomainService) CanAutoRenew(ctx context.Context, domainName string) (bool, error) {
	// Get the domain wihtout the hosts
	dom, err := svc.GetDomainByName(ctx, domainName, false)
	if err != nil {
		return false, err
	}

	// Check if the domain status allows it to be renewed
	if !dom.CanBeRenewed() {
		return false, nil
	}

	// Get the TLD including the phases
	tld, err := svc.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return false, err
	}

	// Always use the current Ga phase policy for renewals (phase extention does not apply to renews)
	phase, err := tld.GetCurrentGAPhase()
	if err != nil {
		return false, err
	}

	// Check if the current GA (General Availability) phase policy allows auto-renewal.
	if phase.Policy.AllowAutoRenew != nil && !*phase.Policy.AllowAutoRenew {
		return false, nil
	}

	// Get the Registrar and check if the registrar has opted in for auto renew
	rar, err := svc.rarRepo.GetByClID(ctx, dom.ClID.String(), false)
	if err != nil {
		return false, err
	}
	if !rar.Autorenew {
		return false, nil
	}

	return true, nil
}

// AutoRenewDomain renews a domain for a specified number of years automatically.
//
// It performs the following steps:
// 1. Retrieves the domain by its name without including the hosts.
// 2. Retrieves the TLD (Top-Level Domain) including the phases.
// 3. Checks if the current GA (General Availability) phase policy allows auto-renewal.
// 4. Retrieves the registrar and checks if the registrar has opted in for auto-renewal.
// 5. Renews the domain using the specified number of years.
// 6. Saves the updated domain.
//
// Parameters:
// - ctx: The context for controlling cancellation and deadlines.
// - name: The name of the domain to be renewed.
// - years: The number of years to renew the domain for.
//
// Returns:
// - A pointer to the updated domain entity.
// - An error if any step in the process fails.
func (svc *DomainService) AutoRenewDomain(ctx context.Context, name string, years int) (*entities.Domain, error) {
	// Get the domain wihtout the hosts
	dom, err := svc.GetDomainByName(ctx, name, false)
	if err != nil {
		return nil, err
	}

	// Get the TLD including the phases
	tld, err := svc.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// Always use the current Ga phase policy for renewals (phase extention does not apply to renews)
	phase, err := tld.GetCurrentGAPhase()
	if err != nil {
		return nil, err
	}
	if phase.Policy.AllowAutoRenew != nil && !*phase.Policy.AllowAutoRenew {
		return nil, ErrAutoRenewNotEnabledTLD
	}

	// Get the Registrar and check if the registrar has opted in for auto renew
	rar, err := svc.rarRepo.GetByClID(ctx, dom.ClID.String(), false)
	if err != nil {
		return nil, err
	}
	if !rar.Autorenew {
		return nil, ErrAutoRenewNotEnabledRar
	}

	// Create a lifecycle event for logging
	event, err := entities.NewDomainLifeCycleEvent(
		rar.ClID.String(),
		"",
		dom.Name.ParentDomain(),
		dom.Name.String(),
		1,
		entities.TransactionTypeAutoRenewal,
	)
	if err != nil {
		return nil, err
	}

	// Get a quote
	quote, err := svc.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      dom.Name.String(),
		ClID:            rar.ClID.String(),
		TransactionType: entities.TransactionTypeAutoRenewal,
		Currency:        phase.Policy.BaseCurrency,
		Years:           1,
		PhaseName:       phase.Name.String(),
	})
	if err != nil {
		return nil, err
	}
	event.Quote = *quote

	// Save the previous state
	prevState := dom.Clone()

	// Renew the domain using our entity
	err = dom.Renew(years, true, phase)
	if err != nil {
		return nil, err
	}

	// Save the domain
	updatedDomain, err := svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}
	event.DomainRoID = updatedDomain.RoID.String()

	// Log the domain auto renewal
	msg := fmt.Sprintf("Domain %s auto-renewed for %d years", name, years)
	svc.logDomainLifecycleEvent(ctx, msg, event, nil, updatedDomain, prevState)

	return updatedDomain, nil
}

// MarkDomainForDeletion marks a domain for deletion by its name.
// It retrieves the domain, its TLD, and the current GA phase, then marks the domain for deletion (this sets all of the appropriate RGP statuses)
// and updates it in the repository.
// This is what you would use to process an EPP delete command. (should we rename this to EPPDeleteDomain?)
//
// Parameters:
//   - ctx: The context for the request.
//   - domainName: The name of the domain to be marked for deletion.
//
// Returns:
//   - *entities.Domain: The updated domain entity.
//   - error: An error if any occurred during the process.
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

	// Create a lifecycle event for logging
	event, err := entities.NewDomainLifeCycleEvent(
		dom.ClID.String(),
		"",
		dom.Name.ParentDomain(),
		dom.Name.String(),
		0,
		entities.TransactionTypeDelete,
	)
	if err != nil {
		return nil, err
	}

	// Save the previous state
	prevState := dom.Clone()

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

	event.DomainRoID = updatedDomain.RoID.String()

	// Log the domain deletion
	msg := fmt.Sprintf("Domain %s marked for deletion (starting EOL cycle)", domainName)
	svc.logDomainLifecycleEvent(ctx, msg, event, nil, updatedDomain, prevState)

	return updatedDomain, nil
}

// ExpireDomain expires a domain by its name. It retrieves the domain,
// fetches the TLD and its current GA phase, and then uses the domain layer to expire the domain.
// Finally, it updates the domain in the repository.
//
// Parameters:
//
//	ctx - The context for managing request-scoped values, deadlines, and cancelation signals.
//	domainName - The name of the domain to be expired.
//
// Returns:
//
//	*entities.Domain - The updated domain entity after expiration.
//	error - An error if any operation fails, otherwise nil.
func (svc *DomainService) ExpireDomain(ctx context.Context, domainName string) (*entities.Domain, error) {
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

	// Create a lifecycle event for logging
	event, err := entities.NewDomainLifeCycleEvent(
		dom.ClID.String(),
		"",
		dom.Name.ParentDomain(),
		dom.Name.String(),
		0,
		entities.TransactionTypeExpiry,
	)
	if err != nil {
		return nil, err
	}

	// Save the previous state
	prevState := dom.Clone()

	// Expire the domain
	err = dom.Expire(phase)
	if err != nil {
		return nil, err
	}

	// Save the domain
	updatedDomain, err := svc.domainRepository.UpdateDomain(ctx, dom)
	if err != nil {
		return nil, err
	}
	event.DomainRoID = updatedDomain.RoID.String()

	// Log the domain expiration
	msg := fmt.Sprintf("Domain %s expired", domainName)
	svc.logDomainLifecycleEvent(ctx, msg, event, nil, updatedDomain, prevState)

	return updatedDomain, nil
}

// RestoreDomain restores a domain. It does a soft restore by setting the status tu pendingRestore. Another process will pick this up and complete the restore.
func (svc *DomainService) RestoreDomain(ctx context.Context, domainName string) (*entities.Domain, error) {
	// Get the domain
	dom, err := svc.GetDomainByName(ctx, domainName, false)
	if err != nil {
		return nil, err
	}

	tld, err := svc.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// For a restore we always use the current GA phase
	currentPhase, err := tld.GetCurrentGAPhase()
	if err != nil {
		return nil, err
	}

	// Create a lifecycle event for logging
	event, err := entities.NewDomainLifeCycleEvent(
		dom.ClID.String(),
		"",
		dom.Name.ParentDomain(),
		dom.Name.String(),
		0,
		entities.TransactionTypeRestore,
	)
	if err != nil {
		return nil, err
	}

	// Get a quote
	quote, err := svc.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      dom.Name.String(),
		ClID:            dom.ClID.String(),
		TransactionType: entities.TransactionTypeAutoRenewal,
		Currency:        currentPhase.Policy.BaseCurrency,
		Years:           1,
		PhaseName:       currentPhase.Name.String(),
	})
	if err != nil {
		return nil, err
	}
	event.Quote = *quote

	// Save the previous state
	prevState := dom.Clone()

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
	event.DomainRoID = updatedDomain.RoID.String()

	// Log the domain restoration
	msg := fmt.Sprintf("Domain %s restored by %s", domainName, dom.ClID)
	svc.logDomainLifecycleEvent(ctx, msg, event, nil, updatedDomain, prevState)

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

// GetNSRecordsPerTLD gets NS records for a TLD
func (s *DomainService) GetNSRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error) {
	response, err := s.domainRepository.GetActiveDomainsWithHosts(ctx, strings.ToLower(tld))
	if err != nil {
		return nil, err
	}
	return response, nil
}

// GetGlueRecordsPerTLD gets Glue records for a TLD
func (s *DomainService) GetGlueRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error) {
	response, err := s.domainRepository.GetActiveDomainGlue(ctx, tld)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Count returns the number of domains
func (s *DomainService) Count(ctx context.Context) (int64, error) {
	return s.domainRepository.Count(ctx)
}

// ListExpiringDomains returns a list of expiring domains
func (s *DomainService) ListExpiringDomains(ctx context.Context, q *queries.ExpiringDomainsQuery, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListExpiringDomains(ctx, q.Before, pageSize, q.ClID.String(), q.TLD.String(), cursor)
}

// CountExpiringDomains returns the number of expiring domains
func (s *DomainService) CountExpiringDomains(ctx context.Context, q *queries.ExpiringDomainsQuery) (int64, error) {
	return s.domainRepository.CountExpiringDomains(ctx, q.Before, q.ClID.String(), q.TLD.String())
}

// ListPurgeableDomains returns a list of purgeable domains. This means the domain has PendingDelete and the grace period has expired (RGPStatus.Purgedate is in the past)
func (s *DomainService) ListPurgeableDomains(ctx context.Context, q *queries.PurgeableDomainsQuery, pageSize int, cursor string) ([]*entities.Domain, error) {
	return s.domainRepository.ListPurgeableDomains(ctx, q.After, pageSize, q.ClID.String(), q.TLD.String(), cursor)
}

// CountPurgeableDomains returns the number of purgeable domains
func (s *DomainService) CountPurgeableDomains(ctx context.Context, q *queries.PurgeableDomainsQuery) (int64, error) {
	return s.domainRepository.CountPurgeableDomains(ctx, q.After, q.ClID.String(), q.TLD.String())
}

// GetQuote retrieves a quote for a domain based on the provided QuoteRequest.
// It validates the request, retrieves the appropriate TLD and phase, and calculates
// the quote using the PriceEngine.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	q - The QuoteRequest containing the details for the quote. All parameters are required, except for phaseName which defaults to the "Currently Active GA Phase".
//
// Returns:
//
//	*entities.Quote - The calculated quote for the domain.
//	error - An error if the request is invalid or if there is an issue retrieving
//	        the necessary data or calculating the quote.
func (s *DomainService) GetQuote(ctx context.Context, q *queries.QuoteRequest) (*entities.Quote, error) {
	// Validate the request and
	if err := q.Validate(); err != nil {
		return nil, err
	}
	domainName, err := entities.NewDomainName(q.DomainName)
	if err != nil {
		return nil, err
	}
	// Get a fully preloaded TLD
	tld, err := s.tldRepo.GetByName(ctx, domainName.ParentDomain(), true)
	if err != nil {
		return nil, err
	}
	// If no phase name is provided, default to the "Currently Active GA Phase"
	var phase *entities.Phase
	if q.PhaseName == "" {
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, use the specified phase
		phase, err = tld.FindPhaseByName(entities.ClIDType(q.PhaseName))
		if err != nil {
			return nil, err
		}
	}
	if phase == nil {
		return nil, entities.ErrPhaseNotFound
	}

	domain, err := s.domainRepository.GetDomainByName(ctx, domainName.String(), false)
	// Get the domain
	if err != nil {
		if !errors.Is(err, entities.ErrDomainNotFound) {
			// If there was an error other than domain not found, return it
			return nil, err
		}
		// If we don't have the domain, create a placeholder
		domain, err = entities.NewDomain("123_DOM-APEX", domainName.String(), q.ClID, "str0ngP@zz")
		if err != nil {
			return nil, err
		}
	}

	// Get the PremiumLabels in all currencies
	pe := []*entities.PremiumLabel{}
	if phase.PremiumListName != nil {
		pe, err = s.premiumLabelRepo.List(ctx, 25, "", *phase.PremiumListName, "", domainName.Label())
		if err != nil {
			return nil, err
		}
	}

	// Create a default FX
	fx := &entities.FX{
		BaseCurrency:   phase.Policy.BaseCurrency,
		TargetCurrency: phase.Policy.BaseCurrency,
		Rate:           1,
	}
	if q.Currency != phase.Policy.BaseCurrency {
		fx, err = s.fxRepo.GetByBaseAndTargetCurrency(ctx, phase.Policy.BaseCurrency, strings.ToUpper(q.Currency))
		if err != nil {
			// If we don't have an FX rate, and we need it, return an error
			return nil, errors.Join(ErrMissingFXRate, err)
		}
	}

	// Instantiate a PriceEngine
	calc := entities.NewPriceEngine(*phase, *domain, *fx, pe)

	// Get/Return the quote
	return calc.GetQuote(*q.ToEntity())
}

// logDomainLifecycleEvent logs a domain lifecycle event with the provided context, event, command, and result.
// It extracts trace_id and correlation_id from the context if they exist and includes them in the event.
//
// Parameters:
//
//	ctx - The context containing trace_id and correlation_id.
//	event - The domain lifecycle event to be logged.
//	command - The command associated with the event.
//	result - The result of the domain lifecycle operation.
func (s *DomainService) logDomainLifecycleEvent(
	ctx context.Context,
	msg string,
	event *entities.DomainLifeCycleEvent,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	// Populate trace_id and correlation_id if they exist
	if trace_id, ok := ctx.Value("trace_id").(string); ok {
		event.TraceID = trace_id
	}
	if correlation_id, ok := ctx.Value("correlation_id").(string); ok {
		event.CorrelationID = correlation_id
	}
	// Log the domain lifecycle event
	s.logger.Info(
		msg,
		zap.String("event_type", "domain_lifecycle_event"),
		zap.Any("domain_lifecycle_event", event),
		zap.Any("command", command),
		zap.Any("new_state", newState),
		zap.Any("previous_state", previousState),
	)
}
