package entities

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Credential caps. They are the defaults docs/epp/POLICY_REFERENCE.md lists;
// per-tier values arrive with the policy engine (Phase 5).
const (
	// MaxEPPLoginsPerRegistrar caps a registrar's logins, enabled or not.
	MaxEPPLoginsPerRegistrar = 10
	// MaxEPPPasswordsPerLogin caps a login's usable passwords: the current one
	// and the one replacing it.
	MaxEPPPasswordsPerLogin = 2
	// MaxEPPCertificatePinsPerRegistrar caps usable pins: current, next, and a
	// spare for a staggered roll across sites.
	MaxEPPCertificatePinsPerRegistrar = 3
	// MaxEPPAllowedNetworksPerRegistrar caps usable allowlist entries.
	MaxEPPAllowedNetworksPerRegistrar = 20
	// MaxEPPSessionCap is the largest per-login session cap that can be set.
	MaxEPPSessionCap = 100

	// The broadest networks a registrar may allow. Anything wider stops being
	// an allowlist.
	MinEPPAllowedPrefixV4 = 24
	MinEPPAllowedPrefixV6 = 48

	// EPP passwords are pwType in RFC 5730's schema: a token of 6 to 16
	// characters.
	MinEPPPasswordLength = 6
	MaxEPPPasswordLength = 16

	minEPPRSAKeyBits = 2048
)

// EPPLoginRole is what a login may do. A readonly login may query but not
// transform; the command allowlist that enforces it arrives with the policy
// engine (Phase 5).
type EPPLoginRole string

const (
	EPPLoginRoleTransform EPPLoginRole = "transform"
	EPPLoginRoleReadonly  EPPLoginRole = "readonly"
)

// IsValid reports whether r is a known role.
func (r EPPLoginRole) IsValid() bool {
	return r == EPPLoginRoleTransform || r == EPPLoginRoleReadonly
}

// PasswordHasher hashes and verifies EPP passwords. The encoding is the
// implementation's; callers store and compare it opaquely.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(encoded, plain string) (bool, error)
}

// EPPRegistrarAccess is everything that lets a registrar's systems connect:
// its logins with their passwords, its certificate pins and its source-IP
// allowlist. It is one aggregate so that the caps hold across concurrent
// changes; repositories load and save it whole, under one lock.
type EPPRegistrarAccess struct {
	ClID            ClIDType             `json:"clid"`
	Logins          []*EPPLogin          `json:"logins"`
	CertificatePins []*EPPCertificatePin `json:"certificate_pins"`
	AllowedNetworks []*EPPAllowedNetwork `json:"allowed_networks"`
}

// EPPLogin is one system's identity at a registrar: the clID it sends in
// <login>. Objects are sponsored by the registrar, never by the login
// (ADR 0006).
type EPPLogin struct {
	LoginID    ClIDType       `json:"login_id"`
	Role       EPPLoginRole   `json:"role"`
	Enabled    bool           `json:"enabled"`
	SessionCap int            `json:"session_cap"` // 0 means the policy default
	CreatedAt  time.Time      `json:"created_at"`
	CreatedBy  string         `json:"created_by,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Passwords  []*EPPPassword `json:"passwords"`
}

// EPPPassword is one of a login's passwords. A login holds several so that a
// password can be rolled without an outage (ADR 0014).
type EPPPassword struct {
	EPPCredentialLifecycle
	Hash string `json:"-"`
}

func (p *EPPPassword) lifecycle() *EPPCredentialLifecycle { return &p.EPPCredentialLifecycle }

// EPPCertificatePin accepts any client certificate carrying one public key,
// identified by the SHA-256 of its SubjectPublicKeyInfo. Renewing a
// certificate on the same key needs no new pin. The certificate fields record
// the certificate the pin was made from.
type EPPCertificatePin struct {
	EPPCredentialLifecycle
	SPKISHA256 string    `json:"spki_sha256"`
	Subject    string    `json:"subject"`
	Issuer     string    `json:"issuer"`
	Serial     string    `json:"serial"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	// Logins restricts the pin to these logins. Empty means every login.
	Logins []ClIDType `json:"logins,omitempty"`
	// ExpiryNotice is the expiry notice the pin has had, in days before
	// its NotAfter: 30, then 7; 0 for none yet (NoteExpiringPins).
	ExpiryNotice int `json:"expiry_notice_days,omitempty"`
}

func (p *EPPCertificatePin) lifecycle() *EPPCredentialLifecycle { return &p.EPPCredentialLifecycle }

// AllowsLogin reports whether the pin may be used by loginID.
func (p *EPPCertificatePin) AllowsLogin(loginID ClIDType) bool {
	return len(p.Logins) == 0 || slices.Contains(p.Logins, loginID)
}

// EPPAllowedNetwork is a source network the registrar's systems connect from.
type EPPAllowedNetwork struct {
	EPPCredentialLifecycle
	Prefix netip.Prefix `json:"prefix"`
	Note   string       `json:"note,omitempty"`
}

func (n *EPPAllowedNetwork) lifecycle() *EPPCredentialLifecycle { return &n.EPPCredentialLifecycle }

// NewEPPRegistrarAccess returns the empty access of a registrar.
func NewEPPRegistrarAccess(clid string) (*EPPRegistrarAccess, error) {
	c, err := NewClIDType(clid)
	if err != nil {
		return nil, err
	}
	return &EPPRegistrarAccess{ClID: c}, nil
}

// Login returns the login with loginID.
func (a *EPPRegistrarAccess) Login(loginID string) (*EPPLogin, error) {
	for _, l := range a.Logins {
		if string(l.LoginID) == loginID {
			return l, nil
		}
	}
	return nil, ErrEPPLoginNotFound
}

// AddLogin adds an enabled login. Login IDs are unique across registrars; the
// repository enforces that, since this aggregate sees one registrar only.
func (a *EPPRegistrarAccess) AddLogin(loginID string, role EPPLoginRole, by string, now time.Time) (*EPPLogin, error) {
	id, err := NewClIDType(loginID)
	if err != nil {
		return nil, fmt.Errorf("%w: login ID must be 3 to 16 ASCII characters", ErrInvalidEPPCredential)
	}
	if !role.IsValid() {
		return nil, fmt.Errorf("%w: role must be %q or %q", ErrInvalidEPPCredential, EPPLoginRoleTransform, EPPLoginRoleReadonly)
	}
	if _, err := a.Login(string(id)); err == nil {
		return nil, ErrEPPLoginExists
	}
	if len(a.Logins) >= MaxEPPLoginsPerRegistrar {
		return nil, fmt.Errorf("%w: a registrar has at most %d logins", ErrEPPCredentialLimit, MaxEPPLoginsPerRegistrar)
	}
	l := &EPPLogin{LoginID: id, Role: role, Enabled: true, CreatedAt: now.UTC(), CreatedBy: by, UpdatedAt: now.UTC()}
	a.Logins = append(a.Logins, l)
	return l, nil
}

// EPPLoginChanges are the settable fields of a login; nil leaves one as it is.
type EPPLoginChanges struct {
	Role       *EPPLoginRole
	Enabled    *bool
	SessionCap *int
}

// UpdateLogin applies changes to a login. Disabling a login refuses new
// logins; sessions already open run until they end.
func (a *EPPRegistrarAccess) UpdateLogin(loginID string, c EPPLoginChanges, now time.Time) (*EPPLogin, error) {
	l, err := a.Login(loginID)
	if err != nil {
		return nil, err
	}
	if c.Role != nil && !c.Role.IsValid() {
		return nil, fmt.Errorf("%w: role must be %q or %q", ErrInvalidEPPCredential, EPPLoginRoleTransform, EPPLoginRoleReadonly)
	}
	if c.SessionCap != nil && (*c.SessionCap < 0 || *c.SessionCap > MaxEPPSessionCap) {
		return nil, fmt.Errorf("%w: session cap must be 0 (the policy default) to %d", ErrInvalidEPPCredential, MaxEPPSessionCap)
	}
	if c.Role != nil {
		l.Role = *c.Role
	}
	if c.Enabled != nil {
		l.Enabled = *c.Enabled
	}
	if c.SessionCap != nil {
		l.SessionCap = *c.SessionCap
	}
	l.UpdatedAt = now.UTC()
	return l, nil
}

// ValidateEPPPassword checks a password against RFC 5730's pwType: 6 to 16
// characters of a token, which this server narrows to printable ASCII with
// no leading, trailing or repeated spaces.
func ValidateEPPPassword(pw string) error {
	if len(pw) < MinEPPPasswordLength || len(pw) > MaxEPPPasswordLength {
		return fmt.Errorf("%w: must be %d to %d characters", ErrInvalidEPPPassword, MinEPPPasswordLength, MaxEPPPasswordLength)
	}
	for i := 0; i < len(pw); i++ {
		if pw[i] < 0x20 || pw[i] > 0x7e {
			return fmt.Errorf("%w: only printable ASCII characters are allowed", ErrInvalidEPPPassword)
		}
	}
	if strings.TrimSpace(pw) != pw || strings.Contains(pw, "  ") {
		return fmt.Errorf("%w: no leading, trailing or repeated spaces", ErrInvalidEPPPassword)
	}
	return nil
}

// AddPassword adds a password to a login. At most MaxEPPPasswordsPerLogin can
// be usable at once; retire one before adding another.
func (a *EPPRegistrarAccess) AddPassword(loginID, plain string, h PasswordHasher, by string, now time.Time) (*EPPPassword, error) {
	l, err := a.Login(loginID)
	if err != nil {
		return nil, err
	}
	if err := ValidateEPPPassword(plain); err != nil {
		return nil, err
	}
	if countUsable(l.Passwords, now) >= MaxEPPPasswordsPerLogin {
		return nil, fmt.Errorf("%w: a login has at most %d usable passwords; retire one first", ErrEPPCredentialLimit, MaxEPPPasswordsPerLogin)
	}
	hash, err := h.Hash(plain)
	if err != nil {
		return nil, fmt.Errorf("hash EPP password: %w", err)
	}
	p := &EPPPassword{EPPCredentialLifecycle: newEPPCredentialLifecycle(by, now), Hash: hash}
	l.Passwords = append(l.Passwords, p)
	l.UpdatedAt = now.UTC()
	return p, nil
}

// ReplacePassword is <login><newPW>: it adds newPW and retires the password
// used to log in, at once (RFC 5730; ADR 0014 decision 2). The cap does not
// apply, because the count of usable passwords does not change.
func (a *EPPRegistrarAccess) ReplacePassword(loginID string, old uuid.UUID, newPW string, h PasswordHasher, by string, now time.Time) (*EPPPassword, error) {
	l, err := a.Login(loginID)
	if err != nil {
		return nil, err
	}
	if err := ValidateEPPPassword(newPW); err != nil {
		return nil, err
	}
	var current *EPPPassword
	for _, p := range l.Passwords {
		if p.ID == old {
			current = p
		}
	}
	if current == nil {
		return nil, ErrEPPCredentialNotFound
	}
	hash, err := h.Hash(newPW)
	if err != nil {
		return nil, fmt.Errorf("hash EPP password: %w", err)
	}
	if err := current.retire(EPPRetireReplaced, nil, by, now); err != nil {
		return nil, err
	}
	p := &EPPPassword{EPPCredentialLifecycle: newEPPCredentialLifecycle(by, now), Hash: hash}
	l.Passwords = append(l.Passwords, p)
	l.UpdatedAt = now.UTC()
	return p, nil
}

// AddCertificatePin pins the public key of the certificate in certPEM. A
// certificate that has expired is refused; one not yet valid is accepted, so
// a pin can be in place before the registrar switches to it.
func (a *EPPRegistrarAccess) AddCertificatePin(certPEM []byte, logins []string, by string, now time.Time) (*EPPCertificatePin, error) {
	cert, err := parseEPPClientCertificate(certPEM)
	if err != nil {
		return nil, err
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("%w: expired %s", ErrInvalidEPPCertificate, cert.NotAfter.UTC().Format(time.RFC3339))
	}
	restrict := make([]ClIDType, 0, len(logins))
	for _, id := range logins {
		l, err := a.Login(strings.TrimSpace(id))
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, id)
		}
		if !slices.Contains(restrict, l.LoginID) {
			restrict = append(restrict, l.LoginID)
		}
	}
	spki := SPKISHA256(cert)
	for _, p := range a.CertificatePins {
		if p.SPKISHA256 == spki && p.Usable(now) {
			return nil, ErrEPPCertificatePinExists
		}
	}
	if countUsable(a.CertificatePins, now) >= MaxEPPCertificatePinsPerRegistrar {
		return nil, fmt.Errorf("%w: a registrar has at most %d usable certificate pins; retire one first", ErrEPPCredentialLimit, MaxEPPCertificatePinsPerRegistrar)
	}
	p := &EPPCertificatePin{
		EPPCredentialLifecycle: newEPPCredentialLifecycle(by, now),
		SPKISHA256:             spki,
		Subject:                cert.Subject.String(),
		Issuer:                 cert.Issuer.String(),
		Serial:                 cert.SerialNumber.Text(16),
		NotBefore:              cert.NotBefore.UTC(),
		NotAfter:               cert.NotAfter.UTC(),
		Logins:                 restrict,
	}
	a.CertificatePins = append(a.CertificatePins, p)
	return p, nil
}

// SPKISHA256 is the pin of cert: the lowercase hex SHA-256 of its
// SubjectPublicKeyInfo.
func SPKISHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// parseEPPClientCertificate reads the first certificate in certPEM and checks
// that its key is one worth pinning.
func parseEPPClientCertificate(certPEM []byte) (*x509.Certificate, error) {
	rest := certPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("%w: no PEM CERTIFICATE block", ErrInvalidEPPCertificate)
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidEPPCertificate, err)
		}
		switch k := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			if k.N.BitLen() < minEPPRSAKeyBits {
				return nil, fmt.Errorf("%w: RSA keys must be at least %d bits", ErrInvalidEPPCertificate, minEPPRSAKeyBits)
			}
		case *ecdsa.PublicKey, ed25519.PublicKey:
		default:
			return nil, fmt.Errorf("%w: unsupported key type %T", ErrInvalidEPPCertificate, k)
		}
		return cert, nil
	}
}

// AddAllowedNetwork allows a source network, given as a CIDR or a single
// address.
func (a *EPPRegistrarAccess) AddAllowedNetwork(cidr, note, by string, now time.Time) (*EPPAllowedNetwork, error) {
	prefix, err := ParseEPPAllowedPrefix(cidr)
	if err != nil {
		return nil, err
	}
	for _, n := range a.AllowedNetworks {
		if n.Prefix == prefix && n.Usable(now) {
			return nil, ErrEPPAllowedNetworkExists
		}
	}
	if countUsable(a.AllowedNetworks, now) >= MaxEPPAllowedNetworksPerRegistrar {
		return nil, fmt.Errorf("%w: a registrar has at most %d usable allowed networks; retire one first", ErrEPPCredentialLimit, MaxEPPAllowedNetworksPerRegistrar)
	}
	n := &EPPAllowedNetwork{EPPCredentialLifecycle: newEPPCredentialLifecycle(by, now), Prefix: prefix, Note: strings.TrimSpace(note)}
	a.AllowedNetworks = append(a.AllowedNetworks, n)
	return n, nil
}

// ParseEPPAllowedPrefix parses an allowlist entry: a CIDR no broader than
// MinEPPAllowedPrefixV4 or MinEPPAllowedPrefixV6, or a single address. The
// result is masked, and IPv4-mapped IPv6 is unmapped.
func ParseEPPAllowedPrefix(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	var prefix netip.Prefix
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%w: %q is not a CIDR", ErrInvalidEPPAllowedNetwork, s)
		}
		if p.Addr().Is4In6() && p.Bits() >= 96 {
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		prefix = p
	} else {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%w: %q is not an IP address", ErrInvalidEPPAllowedNetwork, s)
		}
		if addr.Zone() != "" {
			return netip.Prefix{}, fmt.Errorf("%w: zoned addresses are not allowed", ErrInvalidEPPAllowedNetwork)
		}
		addr = addr.Unmap()
		prefix = netip.PrefixFrom(addr, addr.BitLen())
	}
	minBits := MinEPPAllowedPrefixV6
	if prefix.Addr().Is4() {
		minBits = MinEPPAllowedPrefixV4
	}
	if prefix.Bits() < minBits {
		return netip.Prefix{}, fmt.Errorf("%w: %s is broader than /%d", ErrInvalidEPPAllowedNetwork, prefix, minBits)
	}
	return prefix.Masked(), nil
}

// EPPCredentialRef names one credential: its kind, its ID and, for a
// password, its login. A reference whose parts disagree finds nothing, so a
// request naming one kind can never retire another.
type EPPCredentialRef struct {
	Kind    EPPCredentialKind
	LoginID string
	ID      uuid.UUID
}

// RetireCredential retires the credential ref names. See EPPRetireKind for
// what each kind of retirement does.
func (a *EPPRegistrarAccess) RetireCredential(ref EPPCredentialRef, kind EPPRetireKind, at *time.Time, by string, now time.Time) (*EPPCredentialLifecycle, error) {
	if kind == EPPRetireReplaced {
		return nil, fmt.Errorf("%w: only <newPW> replaces a password", ErrInvalidEPPCredential)
	}
	lc, ck, loginID := a.credential(ref.ID)
	if lc == nil || ck != ref.Kind || (ck == EPPCredentialPassword && string(loginID) != ref.LoginID) {
		return nil, ErrEPPCredentialNotFound
	}
	if err := lc.retire(kind, at, by, now); err != nil {
		return nil, err
	}
	return lc, nil
}

// EPPCredentialKind names the kinds of credential.
type EPPCredentialKind string

const (
	EPPCredentialPassword       EPPCredentialKind = "password"
	EPPCredentialCertificatePin EPPCredentialKind = "certificate_pin"
	EPPCredentialAllowedNetwork EPPCredentialKind = "allowed_network"
)

// credential finds a credential by ID, with its kind and, for a password, its
// login.
func (a *EPPRegistrarAccess) credential(id uuid.UUID) (*EPPCredentialLifecycle, EPPCredentialKind, ClIDType) {
	for _, l := range a.Logins {
		for _, p := range l.Passwords {
			if p.ID == id {
				return &p.EPPCredentialLifecycle, EPPCredentialPassword, l.LoginID
			}
		}
	}
	for _, p := range a.CertificatePins {
		if p.ID == id {
			return &p.EPPCredentialLifecycle, EPPCredentialCertificatePin, ""
		}
	}
	for _, n := range a.AllowedNetworks {
		if n.ID == id {
			return &n.EPPCredentialLifecycle, EPPCredentialAllowedNetwork, ""
		}
	}
	return nil, "", ""
}

// RecordUse moves the last-used time of each credential in uses forward.
// Unknown IDs are ignored: the credential may have been removed since.
func (a *EPPRegistrarAccess) RecordUse(uses []EPPCredentialUse) {
	for _, u := range uses {
		if lc, _, _ := a.credential(u.ID); lc != nil {
			lc.recordUse(u.At, u.From)
		}
	}
}

// Clone returns a deep copy, so a change can be tried on the copy and
// discarded if it fails.
func (a *EPPRegistrarAccess) Clone() *EPPRegistrarAccess {
	c := &EPPRegistrarAccess{ClID: a.ClID}
	for _, l := range a.Logins {
		lc := *l
		lc.Passwords = nil
		for _, p := range l.Passwords {
			pc := *p
			pc.EPPCredentialLifecycle = p.EPPCredentialLifecycle.clone()
			lc.Passwords = append(lc.Passwords, &pc)
		}
		c.Logins = append(c.Logins, &lc)
	}
	for _, p := range a.CertificatePins {
		pc := *p
		pc.EPPCredentialLifecycle = p.EPPCredentialLifecycle.clone()
		pc.Logins = slices.Clone(p.Logins)
		c.CertificatePins = append(c.CertificatePins, &pc)
	}
	for _, n := range a.AllowedNetworks {
		nc := *n
		nc.EPPCredentialLifecycle = n.EPPCredentialLifecycle.clone()
		c.AllowedNetworks = append(c.AllowedNetworks, &nc)
	}
	return c
}

// EventEPPCertificateExpiring records a pinned certificate nearing its
// expiry, 30 and again 7 days before.
const EventEPPCertificateExpiring = "epp.certificate.expiring"

// EPPCertificateExpiryNotices are the notices a pin gets before its
// NotAfter, in days, largest first.
var EPPCertificateExpiryNotices = []int{30, 7}

// EPPCertificateExpiringData is an epp.certificate.expiring event's data.
type EPPCertificateExpiringData struct {
	ClID     ClIDType  `json:"clid"`
	PinID    string    `json:"pin_id"`
	Subject  string    `json:"subject"`
	NotAfter time.Time `json:"not_after"`
	Days     int       `json:"days"`
}

// NoteExpiringPins marks each usable pin whose certificate expires within
// one of EPPCertificateExpiryNotices of now, and has not had that notice,
// and returns an epp.certificate.expiring event for each. A pin past its
// NotAfter gets none: its certificate no longer logs in. Saved with its
// events in one transaction, a notice is given once however many callers
// run it.
func (a *EPPRegistrarAccess) NoteExpiringPins(now time.Time, source string) []DomainEvent {
	var out []DomainEvent
	for _, p := range a.CertificatePins {
		if !p.Usable(now) || !now.Before(p.NotAfter) {
			continue
		}
		due := 0
		for _, days := range EPPCertificateExpiryNotices {
			if p.NotAfter.Sub(now) <= time.Duration(days)*24*time.Hour {
				due = days
			}
		}
		if due == 0 || (p.ExpiryNotice != 0 && p.ExpiryNotice <= due) {
			continue
		}
		p.ExpiryNotice = due
		e := NewDomainEvent(source, EventEPPCertificateExpiring, a.ClID.String(),
			fmt.Sprintf("EPP certificate %s of %s expires in %d days or less", p.Subject, a.ClID, due),
			EPPCertificateExpiringData{ClID: a.ClID, PinID: p.ID.String(), Subject: p.Subject, NotAfter: p.NotAfter, Days: due})
		e.Actor = "system"
		out = append(out, e)
	}
	return out
}
