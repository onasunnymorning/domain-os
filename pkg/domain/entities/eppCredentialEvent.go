package entities

import "time"

// Outbox event types for EPP credential changes. A registrar's credentials
// changing unexpectedly is how a compromise shows up, so every change is
// announced (ADR 0014 decision 7). The subject of each event is the
// registrar's ClID.
const (
	EventEPPLoginCreated            = "epp.login.created"
	EventEPPLoginUpdated            = "epp.login.updated"
	EventEPPPasswordAdded           = "epp.password.added"
	EventEPPPasswordReplaced        = "epp.password.replaced"
	EventEPPCertificatePinAdded     = "epp.certificate_pin.added"
	EventEPPAllowedNetworkAdded     = "epp.allowed_network.added"
	EventEPPCredentialRetiredSuffix = ".retired"
)

// EPPCredentialEventData is the payload of an EPP credential event. It never
// carries a password or a hash.
type EPPCredentialEventData struct {
	ClID         ClIDType          `json:"clid"`
	LoginID      ClIDType          `json:"login_id,omitempty"`
	CredentialID string            `json:"credential_id,omitempty"`
	Kind         EPPCredentialKind `json:"kind,omitempty"`
	RetireKind   EPPRetireKind     `json:"retire_kind,omitempty"`
	RetireAt     *time.Time        `json:"retire_at,omitempty"`
	SPKISHA256   string            `json:"spki_sha256,omitempty"`
	NotAfter     *time.Time        `json:"not_after,omitempty"`
	Network      string            `json:"network,omitempty"`
	Role         EPPLoginRole      `json:"role,omitempty"`
	Enabled      *bool             `json:"enabled,omitempty"`
	SessionCap   *int              `json:"session_cap,omitempty"`
}

// EPPCredentialRetiredEvent is the event type for retiring a credential of
// kind, e.g. "epp.password.retired".
func EPPCredentialRetiredEvent(kind EPPCredentialKind) string {
	return "epp." + string(kind) + EventEPPCredentialRetiredSuffix
}
