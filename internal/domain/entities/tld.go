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

// checkPhaseCanBeAdded is a helper function to determine if a phase can be added to a TLD without overlapping with existing phases. Will return an error if the phase already exists or if it overlaps with an existing phase.
func (t *TLD) checkPhaseCanBeAdded(new_phase *Phase) error {
	for i := 0; i < len(t.Phases); i++ {
		if t.Phases[i].Name == new_phase.Name {
			return ErrPhaseAlreadyExists
		}
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

// AddPhase Adds a phase to the TLD. Will return an error if the phase name already exists or if it overlaps with an existing phase.
func (t *TLD) AddPhase(p *Phase) error {
	err := t.checkPhaseCanBeAdded(p)
	if err != nil {
		return err
	}
	t.Phases = append(t.Phases, *p)
	return nil
}

// GetCurrentPhase Returns the current phase, based on the current time. Will return an error if no active phase is found.
func (t *TLD) GetCurrentPhase() (*Phase, error) {
	for i := 0; i < len(t.Phases); i++ {
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

// DeletePhase deletes a phase from the TLD. Will return an error if the phase is the current phase or if the phase is in the past. We can only delete future phases, in order to keep the history. Only an exact match will delete the phase (ClIDType is case sensitive).
func (t *TLD) DeletePhase(pn ClIDType) error {
	phase, err := t.FindPhaseByName(pn)
	if err != nil {
		return err
	}
	curPhase, err := t.GetCurrentPhase()
	if err == nil {
		if pn == curPhase.Name {
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
	// Check all OTHER phases
	for i := 0; i < len(t.Phases); i++ {
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
