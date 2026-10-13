package commands

import "time"

// EPP credential write inputs (ADR 0014). Business rules live in
// entities.EPPRegistrarAccess; these carry what the caller supplied.

// CreateEPPLoginCommand adds a login to a registrar. Role defaults to
// transform.
type CreateEPPLoginCommand struct {
	LoginID string `json:"login_id" binding:"required"`
	Role    string `json:"role"`
}

// UpdateEPPLoginCommand changes a login; an omitted field is left as it is.
type UpdateEPPLoginCommand struct {
	Role       *string `json:"role"`
	Enabled    *bool   `json:"enabled"`
	SessionCap *int    `json:"session_cap"`
}

// AddEPPPasswordCommand adds a password to a login. Without one, the server
// generates a 16-character password and returns it once.
type AddEPPPasswordCommand struct {
	Password string `json:"password"` //nolint:gosec // G117: the request field that carries the password to be hashed
}

// AddEPPCertificatePinCommand pins the key of a client certificate. Logins,
// when given, restricts the pin to those logins.
type AddEPPCertificatePinCommand struct {
	CertificatePEM string   `json:"certificate_pem" binding:"required"`
	Logins         []string `json:"logins"`
}

// AddEPPAllowedNetworkCommand allows a source network: a CIDR or an address.
type AddEPPAllowedNetworkCommand struct {
	Network string `json:"network" binding:"required"`
	Note    string `json:"note"`
}

// RetireEPPCredentialCommand retires a password, pin or allowed network. Kind
// is "normal" (the default) or "compromise". At schedules a normal
// retirement: without it the credential stays usable for
// entities.DefaultEPPRetireWindow, and a time that is not in the future
// retires it now. A compromise always takes effect now.
type RetireEPPCredentialCommand struct {
	Kind string     `json:"kind"`
	At   *time.Time `json:"at"`
}
