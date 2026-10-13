package entities

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// Why a login was refused. The EPP server answers every one of them with the
// same result, so a client learns nothing about which check failed; the
// reason is for logs and metrics.
var (
	ErrEPPAuthentication         = errors.New("EPP authentication failed")
	ErrEPPLoginDisabled          = fmt.Errorf("%w: login disabled", ErrEPPAuthentication)
	ErrEPPSourceNotAllowed       = fmt.Errorf("%w: source address not allowed", ErrEPPAuthentication)
	ErrEPPCertificateMissing     = fmt.Errorf("%w: no client certificate", ErrEPPAuthentication)
	ErrEPPCertificateNotPinned   = fmt.Errorf("%w: client certificate not pinned", ErrEPPAuthentication)
	ErrEPPCertificateOutOfDate   = fmt.Errorf("%w: client certificate outside its validity period", ErrEPPAuthentication)
	ErrEPPPasswordMismatch       = fmt.Errorf("%w: wrong password", ErrEPPAuthentication)
	ErrEPPUnknownLogin           = fmt.Errorf("%w: unknown login", ErrEPPAuthentication)
	ErrEPPLoginLocked            = fmt.Errorf("%w: too many failed attempts", ErrEPPAuthentication)
	ErrEPPCredentialsUnavailable = errors.New("EPP credentials unavailable")
)

// EPPLoginAttempt is what a client presents before its password is checked:
// the login ID it names, the address it connects from, and its certificate.
type EPPLoginAttempt struct {
	LoginID     string
	Source      netip.Addr
	Certificate *x509.Certificate
	// CheckSource enforces the allowed networks. Only local development turns
	// it off.
	CheckSource bool
}

// EPPAdmission is an attempt that passed every check except the password:
// the login is enabled, the source is allowed and the certificate is pinned
// and in date. Only an admitted attempt can count towards a lockout, so a
// stranger cannot lock a registrar out (ADR 0014).
type EPPAdmission struct {
	ClID    ClIDType
	Login   *EPPLogin
	Pin     *EPPCertificatePin
	Network *EPPAllowedNetwork // nil when the source is not checked
}

// Admit runs the checks that come before the password, in order: the login
// is enabled, the source is in an allowed network, and the certificate's key
// is pinned for this login and the certificate is within its validity
// period, with no grace either side.
func (a *EPPRegistrarAccess) Admit(at EPPLoginAttempt, now time.Time) (*EPPAdmission, error) {
	login, err := a.Login(at.LoginID)
	if err != nil {
		return nil, ErrEPPUnknownLogin
	}
	if !login.Enabled {
		return nil, ErrEPPLoginDisabled
	}
	adm := &EPPAdmission{ClID: a.ClID, Login: login}

	if at.CheckSource {
		src := at.Source.Unmap()
		for _, n := range a.AllowedNetworks {
			if n.Usable(now) && n.Prefix.Contains(src) {
				adm.Network = n
				break
			}
		}
		if adm.Network == nil {
			return nil, ErrEPPSourceNotAllowed
		}
	}

	if at.Certificate == nil {
		return nil, ErrEPPCertificateMissing
	}
	spki := SPKISHA256(at.Certificate)
	for _, p := range a.CertificatePins {
		if p.SPKISHA256 == spki && p.Usable(now) && p.AllowsLogin(login.LoginID) {
			adm.Pin = p
			break
		}
	}
	if adm.Pin == nil {
		return nil, ErrEPPCertificateNotPinned
	}
	if now.Before(at.Certificate.NotBefore) || now.After(at.Certificate.NotAfter) {
		return nil, ErrEPPCertificateOutOfDate
	}
	return adm, nil
}

// VerifyPassword returns the usable password of the admitted login that plain
// matches.
func (adm *EPPAdmission) VerifyPassword(plain string, h PasswordHasher, now time.Time) (*EPPPassword, error) {
	for _, p := range adm.Login.Passwords {
		if !p.Usable(now) {
			continue
		}
		ok, err := h.Verify(p.Hash, plain)
		if err != nil {
			return nil, fmt.Errorf("verify password %s: %w", p.ID, err)
		}
		if ok {
			return p, nil
		}
	}
	return nil, ErrEPPPasswordMismatch
}

// Uses are the credentials a successful login used, for last-used tracking.
func (adm *EPPAdmission) Uses(passwordID uuid.UUID, from string, at time.Time) []EPPCredentialUse {
	uses := []EPPCredentialUse{{ID: passwordID, At: at, From: from}, {ID: adm.Pin.ID, At: at, From: from}}
	if adm.Network != nil {
		uses = append(uses, EPPCredentialUse{ID: adm.Network.ID, At: at, From: from})
	}
	return uses
}

// EPPAuthenticatedLogin is who a session is once <login> succeeds: the
// registrar that sponsors its objects, the login, and the credentials it
// authenticated with, so that retiring one as compromised can end it.
type EPPAuthenticatedLogin struct {
	ClID       ClIDType
	LoginID    ClIDType
	Role       EPPLoginRole
	SessionCap int
	// PasswordID is the password presented, even when <newPW> replaced it.
	PasswordID uuid.UUID
	PinID      uuid.UUID
	NetworkID  uuid.UUID // uuid.Nil when the source is not checked
	// ReplacementPasswordID is the password <newPW> created, or uuid.Nil.
	ReplacementPasswordID uuid.UUID
	// CertificateNotAfter is the expiry of the certificate presented.
	CertificateNotAfter time.Time
}

// PasswordReplaced reports whether <newPW> replaced the password presented.
func (l *EPPAuthenticatedLogin) PasswordReplaced() bool {
	return l.ReplacementPasswordID != uuid.Nil
}

// CredentialIDs are the credentials the session is tied to: those it
// authenticated with and, after <newPW>, the password it set. Retiring any of
// them as compromised ends the session.
func (l *EPPAuthenticatedLogin) CredentialIDs() []uuid.UUID {
	ids := []uuid.UUID{l.PasswordID, l.PinID}
	if l.NetworkID != uuid.Nil {
		ids = append(ids, l.NetworkID)
	}
	if l.ReplacementPasswordID != uuid.Nil {
		ids = append(ids, l.ReplacementPasswordID)
	}
	return ids
}
