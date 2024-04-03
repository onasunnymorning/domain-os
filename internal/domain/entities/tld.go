package entities

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/idna"
)

var (
	ErrTLDNotFound         = errors.New("TLD not found")
	ErrPhaseAlreadyExists  = errors.New("phase with this name already exists")
	ErrPhaseOverlaps       = errors.New("phase date range overlaps with existing phase")
	ErrNoActivePhase       = errors.New("no active phase found")
	ErrPhaseNotFound       = errors.New("phase not found")
	ErrDeleteHistoricPhase = errors.New("cannot delete a historic phase")
	ErrUpdateHistoricPhase = errors.New("cannot update a historic phase")
	ErrDeleteCurrentPhase  = errors.New("cannot delete the current phase, set an end date instead")
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
	Name      DomainName `json:"Name"`  // Name is the ASCII name of the TLD (aka A-label)
	Type      TLDType    `json:"Type"`  // Type is the type of TLD (generic, country-code, second-level)
	UName     DomainName `json:"UName"` // UName is the unicode name of the TLD (aka U-label). Should be empty if the TLD is not an IDN.
	Phases    []Phase    `json:"Phases"`
	CreatedAt time.Time  `json:"CreatedAt"`
	UpdatedAt time.Time  `json:"UpdatedAt"`
}

// NewTLD returns a pointer to a TLD struct or an error (ErrInvalidDomainName) if the domain name is invalid. It will set the Uname and TLDType fields.
func NewTLD(name string) (*TLD, error) {
	d, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	tld := &TLD{Name: *d}
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

// checkGAPhaseCanBeAdded is a helper function to determine if a phase can be added to a TLD without overlapping with existing phases. Will return an error if the phase already exists or if it overlaps with an existing phase.
func (t *TLD) checkGAPhaseCanBeAdded(new_phase *Phase) error {
	for i := 0; i < len(t.Phases); i++ {
		// if either condition A or condition B are true, we have an overlap
		var conda, condb bool
		// condition A: new phase starts before or at the same time an existing phase ends.
		conda = !(new_phase.Ends == nil) && (t.Phases[i].Ends.Before(t.Phases[i].Starts) || t.Phases[i].Ends.Equal(t.Phases[i].Starts))
		// condition B: new phase ends after or at the same time the existing phase starts.
		condb = !(t.Phases[i].Ends == nil) && (t.Phases[i].Ends.Before(new_phase.Starts) || t.Phases[i].Ends.Equal(new_phase.Starts))
		if !(conda || condb) {
			return ErrPhaseOverlaps
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
	for i := 0; i < len(t.GetGAPhases()); i++ {
		// If the end date is nil, just look at the start date
		if t.Phases[i].Ends == nil {
			// If the start date is in the past, it is the current phase
			if t.Phases[i].Starts.Before(time.Now().UTC()) {
				return &t.Phases[i], nil
			}
			// if not, it's a future phase without enddate, we continue looking
			continue
		}
		// If the end date is not nil => it needs to be in the future and the start date in the past
		if t.Phases[i].Ends.After(time.Now().UTC()) && t.Phases[i].Starts.Before(time.Now().UTC()) {
			// this must be the current phase
			return &t.Phases[i], nil
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
	curPhases := t.GetCurrentPhases()
	for i := 0; i < len(curPhases); i++ {
		if curPhases[i].Name == pn {
			return ErrDeleteCurrentPhase
		}
	}
	if phase.Starts.Before(time.Now()) {
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
	// Trying to update a historic phase, it's not allowed to change the past
	if phase.Ends.Before(time.Now().UTC()) {
		return ErrUpdateHistoricPhase
	}
	// Check all OTHER GA phases (no need to check the Launch phases for overlap)
	for i := 0; i < len(t.GetGAPhases()); i++ {
		if t.Phases[i].Name == pn {
			// this is the phase we are modifying no need to compare
			continue
		}
		if t.Phases[i].Ends != nil && t.Phases[i].Ends.Before(time.Now().UTC()) {
			// If the phase has already ended, we dont need to check
			continue
		}
		// If the phase hasn't ended yet, we need to check if the new end date overlaps with the start date of the phase
		if t.Phases[i].Starts.Before(new_end) {
			return ErrPhaseOverlaps
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
	for i := 0; i < len(t.GetGAPhases()); i++ {
		if t.Phases[i].Name == pn {
			// This is the phase we are editing, no need to compare
			continue
		}
		// If the other phase starts after this one, and we remove the end date, they will overlap
		if t.Phases[i].Starts.After(phase.Starts) {
			return ErrPhaseOverlaps
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
