package entities

import (
	"fmt"
	"time"

	"errors"
)

const (
	MaxHostsPerDomain = 10
)

var (
	ErrDomainNotFound                  = errors.New("domain not found")
	ErrDomainAlreadyExists             = errors.New("domain already exists")
	ErrInvalidDomain                   = errors.New("invalid domain")
	ErrTLDAsDomain                     = errors.New("can't create a TLD as a domain")
	ErrInvalidDomainRoID               = fmt.Errorf("invalid Domain.RoID.ObjectIdentifier(), expecing '%s'", DOMAIN_ROID_ID)
	ErrUNameFieldReservedForIDNDomains = errors.New("UName field is reserved for IDN domains")
	ErrOriginalNameFieldReservedForIDN = errors.New("OriginalName field is reserved for IDN domains")
	ErrOriginalNameShouldBeAlabel      = errors.New("OriginalName field should be an A-label")
	ErrOriginalNameEqualToDomain       = errors.New("OriginalName field should not be equal to the domain name, it should point to the a-label of which this domain is a variant")
	ErrNoUNameProvidedForIDNDomain     = errors.New("UName field must be provided for IDN domains")
	ErrUNameDoesNotMatchDomain         = errors.New("UName must be the unicode version of the the domain name (a-label)")
	ErrMaxHostsPerDomainExceeded       = fmt.Errorf("domain can contain %d hosts at most", MaxHostsPerDomain)
	ErrDuplicateHost                   = errors.New("a host with this name is already associated with domain")
	ErrHostSponsorMismatch             = errors.New("host is not owned by the same registrar as the domain")
	ErrInBailiwickHostsMustHaveAddress = errors.New("hosts must have at least one address to be used In-Bailiwick")
	ErrPhaseNotProvided                = errors.New("phase is mandatory for registration")
	ErrDomainRenewNotAllowed           = errors.New("domain renew not allowed")
	ErrDomainRenewExceedsMaxHorizon    = errors.New("domain renew exceeds the maximum horizon")
	ErrInvalidRenewal                  = errors.New("invalid renewal")
	ErrZeroRenewalPeriod               = errors.New("years must be greater than 0")
	ErrDomainDeleteNotAllowed          = errors.New("domain status does not allow delete")
	ErrDomainRestoreNotAllowed         = errors.New("domain cannot be restored")
	ErrDomainExpiryNotAllowed          = errors.New("domain expiry not allowed")
	ErrDomainExpiryFailed              = errors.New("domain expiry failed")
)

// Domain is the domain object in a domain Name registry inspired by the EPP Domain object.
// Ref: https://datatracker.ietf.org/doc/html/rfc5731
type Domain struct {
	RoID           RoidType             `json:"RoID"`
	Name           DomainName           `json:"Name"`         // in case of IDN, this contains the A-label
	OriginalName   DomainName           `json:"OriginalName"` // is used to indicate that the domain name is an IDN variant. This element contains the domain name (A-label) used to generate the IDN variant.
	UName          DomainName           `json:"UName"`        // is used in case the domain is an IDN domain. This element contains the Unicode representation of the domain name (aka U-label).
	RegistrantID   ClIDType             `json:"RegistrantID"`
	AdminID        ClIDType             `json:"AdminID"`
	TechID         ClIDType             `json:"TechID"`
	BillingID      ClIDType             `json:"BillingID"`
	ClID           ClIDType             `json:"ClID"`
	CrRr           ClIDType             `json:"CrRr"`
	UpRr           ClIDType             `json:"UpRr"`
	TLDName        DomainName           `json:"TLDName"`
	ExpiryDate     time.Time            `json:"ExpiryDate"`
	DropCatch      bool                 `json:"DropCatch"`
	RenewedYears   int                  `json:"RenewedYears"`
	AuthInfo       AuthInfoType         `json:"AuthInfo"`
	CreatedAt      time.Time            `json:"CreatedAt"`
	UpdatedAt      time.Time            `json:"UpdatedAt"`
	Status         DomainStatus         `json:"Status"`
	RGPStatus      DomainRGPStatus      `json:"RGPStatus"`
	GrandFathering DomainGrandFathering `json:"GrandFathering"`
	Hosts          []*Host              `json:"Hosts"`
}

// SetOKStatusIfNeeded sets Domain.Status.OK = true if no other prohibition or pendings are present on the DomainStatus
func (d *Domain) SetOKStatusIfNeeded() {
	// if nil, we set OK
	if d.Status.IsNil() {
		d.Status.OK = true
		return
	}
	// if no prohibitions and no pending exists, we set OK
	if !d.Status.HasProhibitions() && !d.Status.HasPendings() {
		d.Status.OK = true
		return
	}
}

// SetUnsetInactiveStatus is to be triggered when making changes to the hosts. Sets Domain.Status.Inactive = true if the domain has no hosts associated with it. It will set Domain.Status.Inactive = false otherwise.
func (d *Domain) SetUnsetInactiveStatus() {
	d.Status.Inactive = !d.HasHosts()
}

// HasHosts checks if the domain has hosts associated with it
func (d *Domain) HasHosts() bool {
	return len(d.Hosts) > 0
}

// GetHostsAsStringSlice returns a slice of strings containing the host names associated with the domain. This is useful for building WHOIS responses
func (d *Domain) GetHostsAsStringSlice() []string {
	hosts := make([]string, 0)
	for _, h := range d.Hosts {
		hosts = append(hosts, h.Name.String())
	}
	return hosts
}

// UnSetOKStatusIfNeeded unsets the Domain.Status.OK flag if a prohibition or pending action is present on the DomainStatus
func (d *Domain) UnSetOKStatusIfNeeded() {
	if d.Status.HasPendings() || d.Status.HasProhibitions() {
		d.Status.OK = false
	}
}

// NewDomain creates a new Domain object. It returns an error if the Domain object is invalid.
// This function is intended to admin functionality such as importing domains from an escrow file.
// It does not set RGP statuses. If creating a domain in the context of a registration, see RegisterDomain instead
func NewDomain(roid, name, clid, authInfo string) (*Domain, error) {
	var err error

	n, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}

	if n.ParentDomain() == "" {
		return nil, ErrTLDAsDomain
	}

	c, err := NewClIDType(clid)
	if err != nil {
		return nil, err
	}

	a, err := NewAuthInfoType(authInfo)
	if err != nil {
		return nil, err
	}

	d := &Domain{
		RoID:     RoidType(roid),
		Name:     *n,
		ClID:     c,
		AuthInfo: a,
	}

	d.TLDName = DomainName(d.Name.ParentDomain())

	if isIDN, _ := d.Name.IsIDN(); isIDN {
		uName, _ := d.Name.ToUnicode() // Error is already checked in NewDomainName
		d.UName = DomainName(uName)
	}

	d.Status = NewDomainStatus() // set the default statuses
	d.CreatedAt = time.Now().UTC()

	if err := d.Validate(); err != nil {
		return nil, err
	}

	return d, nil
}

// Validate checks if the Domain object is valid
func (d *Domain) Validate() error {
	if err := d.RoID.Validate(); err != nil {
		return err
	}
	if d.RoID.ObjectIdentifier() != DOMAIN_ROID_ID {
		return ErrInvalidDomainRoID
	}
	if err := d.Name.Validate(); err != nil {
		return err
	}
	if err := d.ClID.Validate(); err != nil {
		return err
	}
	if err := d.AuthInfo.Validate(); err != nil {
		return err
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}
	if isIDN, _ := d.Name.IsIDN(); !isIDN {
		// if the domain is not an IDN domain, the OriginalName and UName fields must be empty
		if d.OriginalName != "" {
			return ErrOriginalNameFieldReservedForIDN
		}
		if d.UName != "" {
			return ErrUNameFieldReservedForIDNDomains
		}
	} else {
		// the domain is an IDN domain
		// the UName field must not be empty
		if d.UName == "" {
			return ErrNoUNameProvidedForIDNDomain
		}
		// the UName field must be the unicode version of the domain name (a-label)
		if uLabel, _ := d.Name.ToUnicode(); uLabel != string(d.UName) {
			return ErrUNameDoesNotMatchDomain
		}
		// if the OriginalName field is not empty, it should be an A-label (contain only ASCII characters)
		if d.OriginalName != "" && !IsASCII(d.OriginalName.String()) {
			return ErrOriginalNameShouldBeAlabel
		}
		// if the OriginalName field is not empty, it should not be equal to the domain name (it should point to the orignal A-label of which this domain is a variant)
		if d.OriginalName != "" && d.Name == d.OriginalName {
			return ErrOriginalNameEqualToDomain
		}
	}
	return nil
}

// CanBeDeleted checks if the Domain can be deleted (e.g. no delete prohibition is present in its status object: ClientDeleteProhibited or ServerDeleteProhibited). If the domain is alread in pending Delete status, it can't be deleted
func (d *Domain) CanBeDeleted() bool {
	return !d.Status.ClientDeleteProhibited && !d.Status.ServerDeleteProhibited && !d.Status.PendingDelete
}

// CanBeRenewed checks if the Domain can be renewed (e.g. no renew prohibition is present in its status object: ClientRenewProhibited or ServerRenewProhibited). If the domain has any pending status, it can't be renewed
func (d *Domain) CanBeRenewed() bool {
	return !d.Status.ClientRenewProhibited && !d.Status.ServerRenewProhibited && !d.Status.HasPendings()
}

// CanBeTransferred checks if the Domain can be transferred (e.g. no transfer prohibition is present in its status object: ClientTransferProhibited or ServerTransferProhibited). If the domain is alread in pending Transfer status, it can't be transferred
func (d *Domain) CanBeTransferred() bool {
	return !d.Status.ClientTransferProhibited && !d.Status.ServerTransferProhibited && !d.Status.PendingTransfer
}

// CanBeUpdated checks if the Domain can be updated (e.g. no update prohibition is present in its status object: ClientUpdateProhibited or ServerUpdateProhibited). If the domain is alread in pending Update status, it can't be updated
func (d *Domain) CanBeUpdated() bool {
	return !d.Status.ClientUpdateProhibited && !d.Status.ServerUpdateProhibited && !d.Status.PendingUpdate
}

// CanBeRestored checks if the Domain can be restored (if we are in the redemption grace period)
func (d *Domain) CanBeRestored() bool {
	return time.Now().UTC().Before(d.RGPStatus.RedemptionPeriodEnd) && d.Status.PendingDelete
}

// AddHost Adds a host to the domain and updates the Domain.Status.Inactive and Host.Satus.Linked flags if needed.
// Unless force=true it will return an error if the Domain has an update prohibition (ClientUpdateProhibited or ServerUpdateProhibited).
func (d *Domain) AddHost(host *Host, ignoreUpdateProhibitions bool) (int, error) {
	if !d.CanBeUpdated() && !ignoreUpdateProhibitions {
		return 0, ErrDomainUpdateNotAllowed
	}
	// Check this first before looking for the maximum number that way we avoid a useless error saying we hit the maximum while the host is already associated
	_, hasHostAssociation := d.containsHost(host)
	if hasHostAssociation {
		return 0, ErrDuplicateHost
	}
	// Limit the number of hosts per domain
	if len(d.Hosts) >= MaxHostsPerDomain {
		return 0, ErrMaxHostsPerDomainExceeded
	}
	if d.ClID != host.ClID {
		return 0, ErrHostSponsorMismatch
	}
	if host.Name.ParentDomain() == string(d.Name) {
		// Require at least one address if the host is being used in-bailiwick.
		if len(host.Addresses) == 0 {
			return 0, ErrInBailiwickHostsMustHaveAddress
		}
		// Set the host as being used in-bailiwick
		host.InBailiwick = true
	}
	// Set the hosts linked status to true
	err := host.SetStatus(HostStatusLinked)
	if err != nil {
		return 0, err
	}
	d.Hosts = append(d.Hosts, host)
	// Update the inactive status and set OK if needed
	d.SetUnsetInactiveStatus()
	d.SetOKStatusIfNeeded()
	return len(d.Hosts) - 1, nil
}

// containsHost Checks if the domain contains the host and returns the index and true if it does
func (d *Domain) containsHost(host *Host) (int, bool) {
	for i, h := range d.Hosts {
		if h.Name == host.Name {
			return i, true
		}
	}
	return 0, false
}

// RemoveHost Removes a host from the domain sets the Domain.Status.Inactive flag if needed.
func (d *Domain) RemoveHost(host *Host) error {
	if len(d.Hosts) == 0 {
		return ErrHostNotFound // Catch and ignore this error downstream if you want to be idempotent
	}
	index, hasHostAssociation := d.containsHost(host)
	if !hasHostAssociation {
		return ErrHostNotFound // Catch and ignore this error downstream if you want to be idempotent
	}
	d.Hosts = append(d.Hosts[:index], d.Hosts[index+1:]...)
	// Update the inactive status
	d.SetUnsetInactiveStatus()
	return nil
}

// RegisterDomain creates a new Domain object and sets all required (RGP) statuses
func RegisterDomain(roid, name, clid, authInfo, registrantID, adminID, techID, billingID string, phase *Phase, years int) (*Domain, error) {
	if phase == nil {
		return nil, ErrPhaseNotProvided
	}
	dom, err := NewDomain(roid, name, clid, authInfo)
	if err != nil {
		return nil, err
	}

	// Set the create registrar
	dom.CrRr = dom.ClID

	// Set the RGP statuses
	dom.RGPStatus.AddPeriodEnd = time.Now().UTC().AddDate(0, 0, phase.Policy.RegistrationGP)
	dom.RGPStatus.TransferLockPeriodEnd = time.Now().UTC().AddDate(0, 0, phase.Policy.TransferLockPeriod)

	// Set the expiry date
	dom.ExpiryDate = dom.CreatedAt.AddDate(years, 0, 0)

	// If the registration period is more than 1 year, set the renewed years
	if years > 1 {
		dom.RenewedYears = years - 1
	}

	// Set the contacts
	dom.RegistrantID = ClIDType(registrantID)
	dom.AdminID = ClIDType(adminID)
	dom.TechID = ClIDType(techID)
	dom.BillingID = ClIDType(billingID)

	return dom, nil
}

// RenewDomain renews a domain and sets the new expiry date and appropriate RGP statuses. Since renew does not support the launch phase extension, the phase should always be the current GA phase.
func (d *Domain) Renew(years int, isAutoRenew bool, phase *Phase) error {
	if phase == nil {
		return errors.Join(ErrInvalidRenewal, ErrPhaseNotProvided)
	}
	if years == 0 {
		return errors.Join(ErrInvalidRenewal, ErrZeroRenewalPeriod)

	}
	if !d.CanBeRenewed() {
		return errors.Join(ErrInvalidRenewal, ErrDomainRenewNotAllowed)
	}

	// Check if we exceed the maximum renewal period
	if d.ExpiryDate.AddDate(years, 0, 0).After(time.Now().UTC().AddDate(phase.Policy.MaxHorizon, 0, 0)) {
		return errors.Join(ErrInvalidRenewal, ErrDomainRenewExceedsMaxHorizon)
	}

	d.ExpiryDate = d.ExpiryDate.AddDate(years, 0, 0)
	d.RenewedYears += years
	d.UpRr = d.ClID

	// Set the RGP statuses
	if isAutoRenew {
		d.RGPStatus.AutoRenewPeriodEnd = time.Now().UTC().AddDate(0, 0, phase.Policy.AutoRenewalGP)
	} else {
		d.RGPStatus.RenewPeriodEnd = time.Now().UTC().AddDate(0, 0, phase.Policy.RenewalGP)
	}

	return nil
}

// MarkForDeletion ititiates the end-of-life lifecycle for a domain when a delete command is received form the user. Use this to process user delete commands. It sets the domain status to PendingDelete and sets the appropriate RGP statuses depending on the phase policy.
// If the domain is still in AddGracePeriod, the domain does not go through an EOL process and RGP Statuses are set to it can be deleted immediately.
// This funciton depends on downstream logic to purge the domain from the repository, we just set the RGP time parameters here.
// If there is a DeleteProhibition, this function will return an error.
// If the domain status cannot be set to PendingDelete, this function will return an error.
func (d *Domain) MarkForDeletion(phase *Phase) error {
	if !d.CanBeDeleted() {
		return ErrDomainDeleteNotAllowed
	}

	err := d.SetStatus(DomainStatusPendingDelete)
	if err != nil {
		return err
	}
	d.UpRr = d.ClID

	// Set the RGP statuses

	// If the domain is still in AddGracePeriod (Domain.RGPStatus.AddPeriodEnd is in the future), the domain does not go through an EOL process and may be deleted immediately
	// We rely on downstream logic to purge the domain, we just set the time parameters here.
	// Both RedemptionPeriodEnd and PurgeDate are set to the current time so there is no redemption period and the domain can be purged immediately
	if time.Now().UTC().Before(d.RGPStatus.AddPeriodEnd) {
		d.RGPStatus.RedemptionPeriodEnd = time.Now().UTC()
		d.RGPStatus.PurgeDate = time.Now().UTC()
		return nil
	}
	// If the domain is no longer in AddGracePeriod, we set the RGP statuses as per the phase policy EOL settings
	// Since this is a delete command, we set the RedemptionPeriodEnd and PurgeDate based on current time and the phase policy
	d.RGPStatus.RedemptionPeriodEnd = time.Now().UTC().AddDate(0, 0, phase.Policy.RedemptionGP)
	// Set the purge date based on the redemption period end date and the phase policy
	d.RGPStatus.PurgeDate = d.RGPStatus.RedemptionPeriodEnd.AddDate(0, 0, phase.Policy.PendingDeleteGP)
	return nil
}

// Expire initiates the end-of-life lifecycle for a domain when a domain expires. It sets the domain status to PendingDelete and sets the appropriate RGP statuses depending on the phase policy.
// Exiring a domain is only allowed after the domain has expired. It will return an error if the domain has not expired yet.
// It does not error if the domain is already in PendingDelete status or if there is a DeleteProhibition. It does return an error if the domain status cannot be set to PendingDelete.
// Use this function to process domain expiry events.
// This function depends on downstream logic to purge the domain from the repository, we just set the RGP time parameters here.
func (d *Domain) Expire(phase *Phase) error {
	// Don't allow expiring a domain that has not expired yet.
	if time.Now().UTC().Before(d.ExpiryDate) {
		return errors.Join(ErrDomainExpiryNotAllowed, fmt.Errorf("domain expiry date is %s", d.ExpiryDate))
	}

	// If ServerDeleteProhibited is set, we respect it as it provides a deterministic way to prevent domains from being deleted.
	// The calling code should raise the error if appropriate.
	if d.Status.ServerDeleteProhibited {
		return errors.Join(ErrDomainExpiryNotAllowed, fmt.Errorf("%s is set", DomainStatusServerDeleteProhibited))
	}

	// Unset conflicting statuses:
	unsetStatuses := []string{
		DomainStatusClientUpdateProhibited, // We remove update prohibitiions as they will also trigger an error when updating the domain status
		DomainStatusServerUpdateProhibited, // see above
		DomainStatusClientDeleteProhibited, // We remove the clientDeleteProhibited as it will trigger an error setting pendingDelete (this prohibition applies to client DELETE commands not lifecycle events)
	}
	for _, status := range unsetStatuses {
		err := d.UnSetStatus(status)
		if err != nil {
			return errors.Join(ErrDomainExpiryFailed, fmt.Errorf("failed to unset status %s: %s", status, err))
		}
	}
	// Set the domain status to PendingDelete
	err := d.SetStatus(DomainStatusPendingDelete)
	if err != nil {
		return errors.Join(ErrDomainExpiryFailed, fmt.Errorf("failed to set pendingDelete: %s", err))
	}

	// Set the redemption period end date based on the phase policy and expiration date
	d.RGPStatus.RedemptionPeriodEnd = d.ExpiryDate.AddDate(0, 0, phase.Policy.RedemptionGP)
	// Set the purge date based on the redemption period end date and the phase policy
	d.RGPStatus.PurgeDate = d.RGPStatus.RedemptionPeriodEnd.AddDate(0, 0, phase.Policy.PendingDeleteGP)
	return nil
}

// Restore restores a domain if is is pendingDelete and within the redemption grace period
func (d *Domain) Restore() error {
	if !d.CanBeRestored() {
		return ErrDomainRestoreNotAllowed
	}

	// Unset the pending delete status and set the pending restore status
	err := d.UnSetStatus(DomainStatusPendingDelete)
	if err != nil {
		return errors.Join(ErrDomainRestoreNotAllowed, err)
	}
	err = d.SetStatus(DomainStatusPendingRestore)
	if err != nil {
		return errors.Join(ErrDomainRestoreNotAllowed, err)
	}
	// Set the registrar who made the update
	d.UpRr = d.ClID

	return nil
}

// IsGrandFathered checks if the domain is grand fathered
func (d *Domain) IsGrandFathered() bool {
	if d.GrandFathering.GFAmount == 0 && d.GrandFathering.GFCurrency == "" {
		return false
	}
	return true
}
