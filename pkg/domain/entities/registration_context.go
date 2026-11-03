package entities

import (
	"errors"
	"fmt"
	"time"
)

// Phase resolution refusals, answered to a registrar naming a phase it
// cannot register in.
var (
	ErrPhaseNotLaunch          = errors.New("the phase named is not a launch phase")
	ErrPhaseNotActive          = errors.New("the phase named is not active")
	ErrLaunchPhaseAmbiguous    = errors.New("more than one launch phase of this mode is active; name the phase")
	ErrLaunchPhaseModeMismatch = errors.New("the phase named is not of the mode given")
)

// PhaseRef is how a registration names the phase it wants: by name (the
// admin API's phase parameter, or the name attribute of <launch:phase>),
// by launch mode (the value of <launch:phase>, RFC 8334 §2.3), or by
// both. The zero value asks for the current GA phase.
type PhaseRef struct {
	Name ClIDType
	Mode LaunchMode
}

// IsGA reports whether the ref names no phase, so the GA phase applies.
func (r PhaseRef) IsGA() bool { return r.Name == "" && r.Mode == "" }

// RegistrationContext is what governs a registration at an instant: the
// phase whose prices, fees and policy apply, and the claims phase in force
// at the same time, if any. Claims is set when a claims-mode launch phase
// is active, whatever Phase is: a claims period usually overlays GA, and a
// name on the claims list then needs an accepted notice however it is
// registered (RFC 8334 §3.1.3). Nothing reads Claims until claims support
// lands; the shape is here so that the one rule for choosing a phase does
// not have to change when it does.
type RegistrationContext struct {
	Phase  *Phase
	Claims *Phase
}

// RegistrationContextAt is the one rule for which phase governs a
// registration at now, for the admin API and EPP alike.
//
// With no ref, it is the GA phase in force (ErrNoActivePhase if none): GA
// is never named. A name must be a launch phase of this TLD
// (ErrPhaseNotFound, ErrPhaseNotLaunch), of the mode given if one is
// (ErrLaunchPhaseModeMismatch), and active at now (ErrPhaseNotActive). A
// mode alone picks the active launch phase of that mode: none is
// ErrPhaseNotActive, more than one ErrLaunchPhaseAmbiguous, so a registrar
// need name the phase only when two of one mode are open at once.
//
// Allocation of an application after its phase has ended does not go
// through here: it registers in the application's phase as recorded.
func (t *TLD) RegistrationContextAt(ref PhaseRef, now time.Time) (RegistrationContext, error) {
	rc := RegistrationContext{}
	switch {
	case ref.IsGA():
		phase, err := t.GAPhaseAt(now)
		if err != nil {
			return rc, err
		}
		rc.Phase = phase
	case ref.Name != "":
		phase, err := t.FindPhaseByName(ref.Name)
		if err != nil {
			return rc, err
		}
		if phase.Type != PhaseTypeLaunch {
			return rc, fmt.Errorf("%w: %s", ErrPhaseNotLaunch, ref.Name)
		}
		if ref.Mode != "" && phase.Launch.Mode != ref.Mode {
			return rc, fmt.Errorf("%w: %s is %s, not %s", ErrLaunchPhaseModeMismatch, ref.Name, phase.Launch.Mode, ref.Mode)
		}
		if !phase.IsActiveAt(now) {
			return rc, fmt.Errorf("%w: %s", ErrPhaseNotActive, ref.Name)
		}
		p := *phase
		rc.Phase = &p
	default:
		phases := t.ActiveLaunchPhases(ref.Mode, now)
		switch len(phases) {
		case 0:
			return rc, fmt.Errorf("%w: no %s phase is open", ErrPhaseNotActive, ref.Mode)
		case 1:
			rc.Phase = &phases[0]
		default:
			return rc, fmt.Errorf("%w: %s", ErrLaunchPhaseAmbiguous, ref.Mode)
		}
	}
	if claims := t.ActiveLaunchPhases(LaunchModeClaims, now); len(claims) > 0 {
		rc.Claims = &claims[0]
	}
	return rc, nil
}
