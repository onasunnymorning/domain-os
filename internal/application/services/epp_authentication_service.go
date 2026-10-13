package services

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// eppServerEventSource is the outbox source of credential changes made over
// EPP itself: <newPW>.
const eppServerEventSource = "domain-os/epp"

// EPPAuthenticationService authenticates EPP logins against the credentials
// EPPCredentialService manages (ADR 0014).
//
// A login is checked in this order: the login exists and is enabled, the
// source is in one of the registrar's allowed networks, the certificate is
// pinned and in date, the (login, source) pair is not locked out, and the
// password matches one of the login's usable passwords. Only a wrong password
// counts towards a lockout: by then the client has shown an allowed address
// and a pinned certificate, so a stranger cannot lock a registrar out.
//
// The credentials are read from the repository on every login, so a change
// made through the admin API applies to the next login on every node.
type EPPAuthenticationService struct {
	repo    repositories.EPPCredentialRepository
	hasher  entities.PasswordHasher
	lockout interfaces.EPPLoginLockout
	uses    *EPPCredentialUseRecorder
	now     func() time.Time
	// verify bounds concurrent password verifications. Argon2id costs memory
	// per verification, so a burst of logins must queue, not multiply it.
	verify chan struct{}
}

// NewEPPAuthenticationService returns an authenticator. maxConcurrentVerify
// bounds the password verifications in progress; lockout may be nil to
// disable lockouts.
func NewEPPAuthenticationService(repo repositories.EPPCredentialRepository, hasher entities.PasswordHasher, lockout interfaces.EPPLoginLockout, uses *EPPCredentialUseRecorder, maxConcurrentVerify int) *EPPAuthenticationService {
	if maxConcurrentVerify < 1 {
		maxConcurrentVerify = 1
	}
	return &EPPAuthenticationService{
		repo: repo, hasher: hasher, lockout: lockout, uses: uses, now: time.Now,
		verify: make(chan struct{}, maxConcurrentVerify),
	}
}

// Login authenticates cmd.
func (s *EPPAuthenticationService) Login(ctx context.Context, cmd commands.EPPLoginCommand) (*entities.EPPAuthenticatedLogin, error) {
	access, err := s.repo.GetAccessByLogin(ctx, cmd.LoginID)
	if err != nil {
		if errors.Is(err, entities.ErrEPPLoginNotFound) {
			return nil, entities.ErrEPPUnknownLogin
		}
		return nil, fmt.Errorf("%w: %w", entities.ErrEPPCredentialsUnavailable, err)
	}
	now := s.now()
	adm, err := access.Admit(entities.EPPLoginAttempt{
		LoginID: cmd.LoginID, Source: cmd.Source, Certificate: cmd.Certificate, CheckSource: cmd.CheckSource,
	}, now)
	if err != nil {
		return nil, err
	}

	key := lockoutKey(cmd.LoginID, cmd.Source)
	if s.lockout != nil {
		// A lockout store that cannot answer does not lock: its
		// implementation fails open to a node-local count (ADR 0013).
		if d, _ := s.lockout.LockedFor(ctx, key); d > 0 {
			return nil, fmt.Errorf("%w for %s", entities.ErrEPPLoginLocked, d.Round(time.Second))
		}
	}

	pw, err := s.verifyPassword(ctx, adm, cmd.Password, now)
	if err != nil {
		if errors.Is(err, entities.ErrEPPPasswordMismatch) && s.lockout != nil {
			_, _ = s.lockout.RecordFailure(ctx, key)
		}
		return nil, err
	}
	if s.lockout != nil {
		_ = s.lockout.Clear(ctx, key)
	}

	result := &entities.EPPAuthenticatedLogin{
		ClID: adm.ClID, LoginID: adm.Login.LoginID, Role: adm.Login.Role, SessionCap: adm.Login.SessionCap,
		PasswordID: pw.ID, PinID: adm.Pin.ID, CertificateNotAfter: cmd.Certificate.NotAfter.UTC(),
	}
	if adm.Network != nil {
		result.NetworkID = adm.Network.ID
	}
	if cmd.NewPassword != "" {
		newID, err := s.replacePassword(ctx, adm, pw.ID, cmd.NewPassword)
		if err != nil {
			return nil, err
		}
		result.ReplacementPasswordID = newID
	}
	if s.uses != nil {
		// The password presented is the one used, even if <newPW> has just
		// retired it; the new one has not been used yet.
		s.uses.Record(string(adm.ClID), adm.Uses(pw.ID, cmd.Source.String(), now))
	}
	return result, nil
}

func (s *EPPAuthenticationService) verifyPassword(ctx context.Context, adm *entities.EPPAdmission, plain string, now time.Time) (*entities.EPPPassword, error) {
	select {
	case s.verify <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %w", entities.ErrEPPCredentialsUnavailable, ctx.Err())
	}
	defer func() { <-s.verify }()
	pw, err := adm.VerifyPassword(plain, s.hasher, now)
	if err != nil && !errors.Is(err, entities.ErrEPPPasswordMismatch) {
		// A stored hash that cannot be read is an operational fault, not a
		// wrong password; it must not count towards a lockout.
		return nil, fmt.Errorf("%w: %w", entities.ErrEPPCredentialsUnavailable, err)
	}
	return pw, err
}

// replacePassword is <newPW>: the new password replaces the one used, which
// stops working at once (RFC 5730; ADR 0014 decision 2).
func (s *EPPAuthenticationService) replacePassword(ctx context.Context, adm *entities.EPPAdmission, old uuid.UUID, newPW string) (uuid.UUID, error) {
	if err := entities.ValidateEPPPassword(newPW); err != nil {
		return uuid.Nil, err
	}
	loginID := string(adm.Login.LoginID)
	var replaced uuid.UUID
	_, err := s.repo.UpdateAccess(ctx, string(adm.ClID), func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		p, err := a.ReplacePassword(loginID, old, newPW, s.hasher, loginID, s.now())
		if err != nil {
			return nil, err
		}
		replaced = p.ID
		e := entities.NewDomainEvent(eppServerEventSource, entities.EventEPPPasswordReplaced, string(a.ClID),
			fmt.Sprintf("EPP login %s replaced its password with <newPW>", loginID),
			entities.EPPCredentialEventData{ClID: a.ClID, LoginID: adm.Login.LoginID, CredentialID: p.ID.String(), Kind: entities.EPPCredentialPassword})
		e.Actor = loginID
		return []entities.DomainEvent{e}, nil
	})
	switch {
	case err == nil:
		return replaced, nil
	case errors.Is(err, entities.ErrEPPCredentialRetired), errors.Is(err, entities.ErrEPPCredentialNotFound):
		// Retired between the check and the change.
		return uuid.Nil, entities.ErrEPPPasswordMismatch
	case errors.Is(err, entities.ErrInvalidEPPPassword):
		return uuid.Nil, err
	default:
		return uuid.Nil, fmt.Errorf("%w: %w", entities.ErrEPPCredentialsUnavailable, err)
	}
}

// AllowedNetworks returns every registrar's usable allowed networks.
func (s *EPPAuthenticationService) AllowedNetworks(ctx context.Context) ([]netip.Prefix, error) {
	return s.repo.ListUsableAllowedNetworks(ctx, s.now())
}

// CompromisedSince returns credentials retired as compromised since.
func (s *EPPAuthenticationService) CompromisedSince(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return s.repo.ListCompromisedSince(ctx, since)
}

func lockoutKey(loginID string, source netip.Addr) string {
	return loginID + "|" + source.Unmap().String()
}

// EPPCredentialUseRecorder coalesces last-used updates. A successful login
// records its credentials here; Flush writes the latest use of each, so a
// burst of logins is one write per registrar per flush, not one per login
// (ADR 0014).
type EPPCredentialUseRecorder struct {
	repo    repositories.EPPCredentialRepository
	mu      sync.Mutex
	pending map[string]map[uuid.UUID]entities.EPPCredentialUse
}

// NewEPPCredentialUseRecorder returns a recorder writing to repo.
func NewEPPCredentialUseRecorder(repo repositories.EPPCredentialRepository) *EPPCredentialUseRecorder {
	return &EPPCredentialUseRecorder{repo: repo, pending: map[string]map[uuid.UUID]entities.EPPCredentialUse{}}
}

// Record notes uses of a registrar's credentials, keeping the latest of each.
func (r *EPPCredentialUseRecorder) Record(clid string, uses []entities.EPPCredentialUse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.pending[clid]
	if m == nil {
		m = map[uuid.UUID]entities.EPPCredentialUse{}
		r.pending[clid] = m
	}
	for _, u := range uses {
		if prev, ok := m[u.ID]; !ok || u.At.After(prev.At) {
			m[u.ID] = u
		}
	}
}

// Flush writes what was recorded since the last flush. A registrar whose
// write fails keeps its uses for the next flush, unless newer ones replace
// them.
func (r *EPPCredentialUseRecorder) Flush(ctx context.Context) error {
	r.mu.Lock()
	pending := r.pending
	r.pending = map[string]map[uuid.UUID]entities.EPPCredentialUse{}
	r.mu.Unlock()

	var errs []error
	for clid, m := range pending {
		uses := make([]entities.EPPCredentialUse, 0, len(m))
		for _, u := range m {
			uses = append(uses, u)
		}
		_, err := r.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
			a.RecordUse(uses)
			return nil, nil
		})
		if err != nil {
			r.Record(clid, uses)
			errs = append(errs, fmt.Errorf("record EPP credential use for %s: %w", clid, err))
		}
	}
	return errors.Join(errs...)
}
