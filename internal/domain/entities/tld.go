package entities

import (
	"strings"
	"time"

	"errors"

	"golang.org/x/net/idna"
)

var (
	ErrTLDNotFound                           = errors.New("TLD not found")
	ErrPhaseAlreadyExists                    = errors.New("phase with this name already exists")
	ErrGAPhaseOverlaps                       = errors.New("GA phase date range overlaps with existing GA phase")
	ErrTLDEscrowImportNotAllowed             = errors.New("escrow import not allowed for this TLD")
	ErrNoActivePhase                         = errors.New("no active phase found")
	ErrPhaseNotFound                         = errors.New("phase not found")
	ErrDeleteHistoricPhase                   = errors.New("cannot delete a historic phase")
	ErrUpdateHistoricPhase                   = errors.New("cannot update a historic phase")
	ErrDeleteCurrentPhase                    = errors.New("cannot delete the current phase, set an end date instead")
	ErrCannotSetEscrowImportWithActivePhases = errors.New("cannot set AllowEscrowImport to true with active phases")
)

// TLDType is a custom type describing the type of TLD
type TLDType string

// String returns the string representation of the TLDType
func (t TLDType) String() string {
	return string(t)
}

// TLDType constants
const (
	TLDTypeGTLD  = "generic"
	TLDTypeCCTLD = "country-code"
	TLDTypeSLD   = "second-level"
)

// TLD is a struct representing a top-level domain
type TLD struct {
	// ASCII name of the TLD (aka A-label)
	Name DomainName `json:"Name"`
	// The type of TLD (generic, country-code, second-level)
	Type TLDType `json:"Type"`
	// UName is the unicode name of the TLD (aka U-label). Should be empty if the TLD is not an IDN.
	UName DomainName `json:"UName"`
	// RyID is the Registry Operator ID
	RyID ClIDType `json:"RyID"`
	// AllowEscrowImports is a boolean indicating if the TLD allows escrow imports
	AllowEscrowImport bool `json:"AllowEscrowImport"`
	// EnableDNS is a boolean indicating if the TLD has DNS enabled
	EnableDNS bool `json:"EnableDNS"`
	// Phases is a slice of phases for the TLD
	Phases    []Phase   `json:"Phases"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
}

// NewTLD returns a pointer to a TLD struct or an error (ErrInvalidDomainName) if the domain name is invalid. It will set the Uname and TLDType fields.
func NewTLD(name, RyID string) (*TLD, error) {
	d, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	validatedRyID, err := NewClIDType(RyID)
	if err != nil {
		return nil, err
	}
	tld := &TLD{Name: *d}
	tld.RyID = validatedRyID
	tld.SetUname()
	tld.setTLDType()
	tld.CreatedAt = RoundTime(time.Now().UTC())
	return tld, nil
}

// SetUname sets the unicode name of the TLD based on the name. Uname is only set if the tld's domain name is an IDN. If the name is not an IDN the Uname will be empty.
func (t *TLD) SetUname() {
	if isIDN, _ := t.Name.IsIDN(); isIDN {
		unicode_string, _ := idna.ToUnicode(string(t.Name))
		t.UName = DomainName(unicode_string)
	}
}

// setTLDType Determines TLD type from the name. If the name is 2 characters long, it's a country-code TLD. If it contains a dot, it's a second-level TLD. Otherwise, it's a generic TLD.
func (t *TLD) setTLDType() {
	if len(string(t.Name)) == 2 {
		t.Type = TLDTypeCCTLD
	} else if strings.Contains(string(t.Name), ".") {
		t.Type = TLDTypeSLD
	} else {
		t.Type = TLDTypeGTLD
	}
}

// checkPhaseNameExists is a helper function to determine if a phase name already exists in the TLD. Will return an error if the phase name already exists.
func (t *TLD) checkPhaseNameExists(pn ClIDType) error {
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Name == pn {
			return ErrPhaseAlreadyExists
		}
	}
	return nil
}

// checkGAPhaseCanBeAdded is a helper function to determine if a phase can be added to a TLD without overlapping with existing GA phases. Will return an error if the phase already exists or if it overlaps with an existing GA phase.
func (t *TLD) checkGAPhaseCanBeAdded(new_phase *Phase) error {
	for _, gaPhase := range t.GetGAPhases() {
		if new_phase.OverlapsWith(&gaPhase) {
			return ErrGAPhaseOverlaps
		}
	}
	return nil
}

// AddPhase Adds a phase to the TLD. Will return an error if the phase name already exists or if a GA Phase overlaps with an existing GA Phase.
func (t *TLD) AddPhase(p *Phase) error {
	// Error our quickly if the phase name already exists
	if err := t.checkPhaseNameExists(p.Name); err != nil {
		return err
	}
	// If the phase is a launch phase, we only need to check if the name already exists (Launch phases may overlap with GA and with other Launch phases)
	if p.Type == PhaseTypeLaunch {
		t.Phases = append(t.Phases, *p)
		return nil
	}
	// If the phase is a GA phase, we need to check if it can be added without overlapping with existing GA phases
	err := t.checkGAPhaseCanBeAdded(p)
	if err != nil {
		return err
	}
	t.Phases = append(t.Phases, *p)
	return nil
}

// GetCurrentGAPhase Returns the current phase, based on the current time. Will return an error if no active phase is found.
func (t *TLD) GetCurrentGAPhase() (*Phase, error) {
	for _, gaPhase := range t.GetGAPhases() {
		// If the end date is nil, just look at the start date
		if gaPhase.Ends == nil {
			// If the start date is in the past, it is the current phase
			if gaPhase.Starts.Before(time.Now().UTC()) {
				return &gaPhase, nil
			}
			// if not, it's a future phase without enddate, we continue looking
			continue
		}
		// If the end date is not nil => it needs to be in the future and the start date in the past
		if gaPhase.Ends.After(time.Now().UTC()) && gaPhase.Starts.Before(time.Now().UTC()) {
			// this must be the current phase
			return &gaPhase, nil
		}
	}
	// if we haven't found anything by now, there is no current phase
	return nil, ErrNoActivePhase
}

// GetCurrentLaunchPhases returns a slice of all current launch phases. If no active launch phase is found, an empty slice is returned.
func (t *TLD) GetCurrentLaunchPhases() []Phase {
	var phases []Phase
	for i := 0; i < len(t.GetLaunchPhases()); i++ {
		if t.Phases[i].Starts.Before(time.Now().UTC()) && (t.Phases[i].Ends == nil || t.Phases[i].Ends.After(time.Now().UTC())) {
			phases = append(phases, t.Phases[i])
		}
	}
	return phases
}

// DeletePhase deletes a phase from the TLD. Only future phases can be deleted. We keep current and histric phases for tracability. Will return an error if the phase is the current phase or if the phase is in the past. Only an exact match will delete the phase (ClIDType is case sensitive).
func (t *TLD) DeletePhase(pn ClIDType) error {
	phase, err := t.FindPhaseByName(pn)
	// TODO: Make idempotent?
	if err != nil {
		return err
	}
	for _, curPhase := range t.GetCurrentPhases() {
		if curPhase.Name == pn {
			return ErrDeleteCurrentPhase
		}
	}
	if phase.Starts.Before(time.Now().UTC()) {
		return ErrDeleteHistoricPhase
	}
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Name == pn {
			t.Phases = append(t.Phases[:i], t.Phases[i+1:]...)
		}
	}
	return nil
}

// FindPhaseByName finds a phase by name. Will return an error if the phase is not found. This is case sensitive and only and exact match will return a phase.
func (t *TLD) FindPhaseByName(pn ClIDType) (*Phase, error) {
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Name == pn {
			return &t.Phases[i], nil
		}
	}
	return nil, ErrPhaseNotFound
}

// EndPhase sets an end date to a phase. The end date must be in the future and after the start date. Returns an error if the end date is in the past or before the start date. Timestamps are converted to UTC.
func (t *TLD) EndPhase(pn ClIDType, endTime time.Time) (*Phase, error) {
	phase, err := t.FindPhaseByName(pn)
	if err != nil {
		return nil, err
	}
	if !IsUTC(endTime) {
		return nil, ErrTimeStampNotUTC
	}
	// If there is no end date, we can set it
	if phase.Ends == nil {
		err = phase.SetEnd(endTime)
		if err != nil {
			return nil, err
		}
		return phase, nil
	}
	// If there is already an end date, we need to check if the new end date is valid
	err = t.checkPhaseEndUpdate(pn, endTime)
	if err != nil {
		return nil, err
	}
	err = phase.SetEnd(endTime)
	if err != nil {
		return nil, err
	}
	return phase, nil
}

// checkPhaseEndUpdate is a helper function to determine if a new phase enddate is valid. The new enddate can't be in the past. We can't update historic phases.
// The new enddate should not cause any overlap with other phases. If any of these conditions are met, an error is returned.
func (t *TLD) checkPhaseEndUpdate(pn ClIDType, new_end time.Time) error {
	phase, err := t.FindPhaseByName(pn)
	if err != nil {
		return err
	}
	if new_end.Before(time.Now().UTC()) {
		return ErrEndDateInPast
	}
	if new_end.Before(phase.Starts) {
		return ErrEndDateBeforeStart
	}
	// Trying to update a historic phase, changing the past can have consequences in the present, you don't want to create a grandfather paradox
	if phase.Ends.Before(time.Now().UTC()) {
		return ErrUpdateHistoricPhase
	}
	// If its a launch phase we are ending, we can safely do so
	if phase.Type == PhaseTypeLaunch {
		return nil
	}
	// If its a GA phase, Check all OTHER GA phases (no need to check the Launch phases for overlap)
	for _, gaPhase := range t.GetGAPhases() {
		if gaPhase.Name == pn {
			// this is the phase we are modifying no need to compare
			continue
		}
		if gaPhase.Ends != nil && gaPhase.Ends.Before(time.Now().UTC()) {
			// If the phase has already ended, we dont need to check
			continue
		}
		// If the phase hasn't ended yet, we need to check if the new end date overlaps with the start date of the phase
		if gaPhase.Starts.Before(new_end) {
			return ErrGAPhaseOverlaps
		}
	}
	return nil
}

// Remove end date of a phase
func (t *TLD) UnSetPhaseEnd(pn ClIDType) (*Phase, error) {
	phase, err := t.FindPhaseByName(pn)
	if err != nil {
		return nil, err
	}
	err = t.checkPhaseEndUnset(pn)
	if err != nil {
		return nil, err
	}
	phase.Ends = nil
	return phase, nil
}

// checkPhaseEndUnset is a helper function to determine if a phase end date can be unset without creating an overlap
func (t *TLD) checkPhaseEndUnset(pn ClIDType) error {
	phase, err := t.FindPhaseByName(pn)
	if err != nil {
		return err
	}
	if phase.Ends == nil {
		// nothing to do, but don't error to be idempotent
		return nil
	}
	if phase.Ends.Before(time.Now().UTC()) {
		return ErrUpdateHistoricPhase
	}
	// Check all OTHER GA phases for overlap
	for _, gaPhase := range t.GetGAPhases() {
		if gaPhase.Name == pn {
			// This is the phase we are editing, no need to compare
			continue
		}
		// If the other phase starts after this one, and we remove the end date, they will overlap
		if gaPhase.Starts.After(phase.Starts) {
			return ErrGAPhaseOverlaps
		}
	}
	return nil
}

// GetGAPhases returns all phases of type GA
func (t *TLD) GetGAPhases() []Phase {
	var phases []Phase
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Type == PhaseTypeGA {
			phases = append(phases, t.Phases[i])
		}
	}
	return phases
}

// GetLaunchPhases returns all phases of type Launch
func (t *TLD) GetLaunchPhases() []Phase {
	var phases []Phase
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Type == PhaseTypeLaunch {
			phases = append(phases, t.Phases[i])
		}
	}
	return phases
}

// GetCurrentPhases returns all current phases GA and Launch
func (t *TLD) GetCurrentPhases() []Phase {
	var phases []Phase
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].IsCurrentlyActive() {
			phases = append(phases, t.Phases[i])
		}
	}
	return phases
}

// SetAllowEscrowImport sets the AllowEscrowImport field of the TLD
// This will only allow the flag to be set if there are no active phases
// This requies the calling code to ensure the Phases property is populated
func (t *TLD) SetAllowEscrowImport(allow bool) error {
	// there are no restrictions to setting this to false
	if !allow {
		t.AllowEscrowImport = allow
		return nil
	}
	// if we are trying to set it to true, we need to check if there are any active phases
	currentPhases := t.GetCurrentPhases()
	if len(currentPhases) > 0 {
		return ErrCannotSetEscrowImportWithActivePhases
	}

	t.AllowEscrowImport = allow
	return nil
}

// SetEnableDNS sets the EnableDNS field of the TLD
func (t *TLD) SetEnableDNS(enable bool) {
	t.EnableDNS = enable
}
