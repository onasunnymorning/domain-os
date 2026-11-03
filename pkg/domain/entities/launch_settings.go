package entities

import (
	"errors"
	"fmt"
)

// Launch phase refusals. The resolution errors (a phase named but not a
// launch phase, not active, ambiguous) are in registration_context.go.
var (
	ErrInvalidLaunchMode        = errors.New("invalid launch mode")
	ErrInvalidRegistrationModel = errors.New("invalid registration model")
	ErrInvalidLaunchSettings    = errors.New("invalid launch settings")
	ErrLaunchSettingsOnGA       = errors.New("launch settings apply to launch phases only")
	ErrLaunchModeFrozen         = errors.New("the launch mode cannot change once the phase has started")
	ErrLaunchModelFrozen        = errors.New("the registration model cannot change once the phase has started or has applications")
)

// LaunchMode is what kind of launch phase a phase is, in the terms of RFC
// 8334 §2.3: the value a registrar sends in <launch:phase>. Sunrise and
// claims phases need a verifier (signed marks, claims notices) that does
// not exist yet; until it does, registrations in them are refused.
type LaunchMode string

const (
	// LaunchModeOpen is an open registration phase under launch terms
	// (RFC 8334: "open").
	LaunchModeOpen LaunchMode = "open"
	// LaunchModeLandrush is a landrush phase, usually by application.
	LaunchModeLandrush LaunchMode = "landrush"
	// LaunchModeCustom is a registry-defined phase, named by the registrar
	// with the name attribute of <launch:phase>.
	LaunchModeCustom LaunchMode = "custom"
	// LaunchModeSunrise is a trademark sunrise: a registration needs a
	// signed mark. Not yet supported over EPP.
	LaunchModeSunrise LaunchMode = "sunrise"
	// LaunchModeClaims is a trademark claims period: a registration of a
	// name on the claims list needs an accepted notice. Not yet supported
	// over EPP.
	LaunchModeClaims LaunchMode = "claims"
)

// String returns the mode as RFC 8334 spells it.
func (m LaunchMode) String() string { return string(m) }

// IsValid reports whether m is one of the five modes.
func (m LaunchMode) IsValid() bool {
	switch m {
	case LaunchModeOpen, LaunchModeLandrush, LaunchModeCustom, LaunchModeSunrise, LaunchModeClaims:
		return true
	}
	return false
}

// RegistrationModel is how a create in a launch phase is answered: a
// registration at once, or an application the registry allocates later.
type RegistrationModel string

const (
	// RegistrationModelFirstCome registers the domain at once, as in GA,
	// under the launch phase's prices, fees and policy.
	RegistrationModelFirstCome RegistrationModel = "first-come"
	// RegistrationModelApplication creates a pending application (RFC 8334
	// §2.1); the registry allocates at most one per name, usually once the
	// phase has ended.
	RegistrationModelApplication RegistrationModel = "application"
)

// String returns the model's name.
func (m RegistrationModel) String() string { return string(m) }

// IsValid reports whether m is one of the two models.
func (m RegistrationModel) IsValid() bool {
	return m == RegistrationModelFirstCome || m == RegistrationModelApplication
}

// LaunchSettings is what a launch phase is and how it registers: the
// value object a Phase of type Launch carries beside its PhasePolicy
// (which it shares with GA phases). A GA phase carries the zero value.
//
// The mark settings say which forms of trademark evidence a sunrise
// accepts (RFC 8334 §2.6: a signed mark, encoded or not, or a mark with a
// validator code) and whether the marks are kept with the applications
// (§3.1.2, includeMark). They are stored and validated now, and read when
// sunrise support lands.
type LaunchSettings struct {
	Mode  LaunchMode        `json:"mode,omitempty" example:"landrush"`
	Model RegistrationModel `json:"model,omitempty" example:"application"`
	// AcceptSignedMarks accepts <smd:signedMark> and <smd:encodedSignedMark>.
	AcceptSignedMarks bool `json:"acceptSignedMarks"`
	// AcceptCodeMarks accepts <launch:codeMark> (a mark with a validator code).
	AcceptCodeMarks bool `json:"acceptCodeMarks"`
	// RetainMarks keeps the marks given with applications.
	RetainMarks bool `json:"retainMarks"`
}

// NewLaunchSettings returns the defaults of a launch phase: open, first
// come, signed marks accepted should the mode become sunrise.
func NewLaunchSettings() LaunchSettings {
	return LaunchSettings{Mode: LaunchModeOpen, Model: RegistrationModelFirstCome, AcceptSignedMarks: true}
}

// IsZero reports whether s is the zero value, as a GA phase carries.
func (s LaunchSettings) IsZero() bool { return s == LaunchSettings{} }

// Validate checks the mode and model are known and that a sunrise accepts
// at least one form of mark.
func (s LaunchSettings) Validate() error {
	if !s.Mode.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidLaunchMode, s.Mode)
	}
	if !s.Model.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidRegistrationModel, s.Model)
	}
	if s.Mode == LaunchModeSunrise && !s.AcceptSignedMarks && !s.AcceptCodeMarks {
		return fmt.Errorf("%w: a sunrise must accept signed marks or code marks", ErrInvalidLaunchSettings)
	}
	return nil
}

// RequiresVerification reports whether registrations in this phase need
// trademark evidence verified: a sunrise (signed marks) or a claims period
// (notices). Neither verifier exists yet, so such phases refuse
// registrations over EPP.
func (s LaunchSettings) RequiresVerification() bool {
	return s.Mode == LaunchModeSunrise || s.Mode == LaunchModeClaims
}
