package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// eppEventSource is the outbox source of credential changes made through the
// admin API.
const eppEventSource = "domain-os/api"

// generatedPasswordAlphabet leaves out characters that are easy to misread
// (0/O, 1/l/I) and ones that need quoting in XML or a shell. Sixteen
// characters from it carry about 92 bits.
const generatedPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789-_.+=@#%"

// EPPCredentialService manages registrars' EPP credentials. Every change is
// one repository transaction that also writes the change's outbox event.
type EPPCredentialService struct {
	repo       repositories.EPPCredentialRepository
	registrars repositories.RegistrarRepository
	hasher     entities.PasswordHasher
	now        func() time.Time
}

// NewEPPCredentialService returns a service over repo. registrars is used to
// keep login IDs from impersonating other registrars.
func NewEPPCredentialService(repo repositories.EPPCredentialRepository, registrars repositories.RegistrarRepository, hasher entities.PasswordHasher) *EPPCredentialService {
	return &EPPCredentialService{repo: repo, registrars: registrars, hasher: hasher, now: time.Now}
}

// GetAccess returns a registrar's logins, passwords, pins and allowed
// networks. Password hashes are never serialised.
func (s *EPPCredentialService) GetAccess(ctx context.Context, clid string) (*entities.EPPRegistrarAccess, error) {
	return s.repo.GetAccess(ctx, clid)
}

// CreateLogin adds a login. A login ID may be the registrar's own ClID, but
// never another registrar's: responses name the sponsoring registrar by ClID,
// and a login called after someone else would be read as them.
func (s *EPPCredentialService) CreateLogin(ctx context.Context, clid string, cmd commands.CreateEPPLoginCommand) (*entities.EPPLogin, error) {
	if cmd.LoginID != clid {
		other, err := s.registrars.GetByClID(ctx, cmd.LoginID, false)
		switch {
		case err == nil && other != nil:
			return nil, fmt.Errorf("%w: %q is another registrar's client ID", entities.ErrEPPLoginExists, cmd.LoginID)
		case err != nil && !errors.Is(err, entities.ErrRegistrarNotFound):
			return nil, fmt.Errorf("check login ID %q: %w", cmd.LoginID, err)
		}
	}
	role := entities.EPPLoginRole(cmd.Role)
	if role == "" {
		role = entities.EPPLoginRoleTransform
	}
	actor := actorOf(ctx)
	var login *entities.EPPLogin
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		l, err := a.AddLogin(cmd.LoginID, role, actor, s.now())
		if err != nil {
			return nil, err
		}
		login = l
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPLoginCreated, a.ClID,
			fmt.Sprintf("EPP login %s created for %s", l.LoginID, a.ClID),
			entities.EPPCredentialEventData{ClID: a.ClID, LoginID: l.LoginID, Role: l.Role})}, nil
	})
	if err != nil {
		return nil, err
	}
	return login, nil
}

// UpdateLogin changes a login's role, enabled flag or session cap.
func (s *EPPCredentialService) UpdateLogin(ctx context.Context, clid, loginID string, cmd commands.UpdateEPPLoginCommand) (*entities.EPPLogin, error) {
	changes := entities.EPPLoginChanges{Enabled: cmd.Enabled, SessionCap: cmd.SessionCap}
	var role entities.EPPLoginRole
	if cmd.Role != nil {
		role = entities.EPPLoginRole(*cmd.Role)
		changes.Role = &role
	}
	var login *entities.EPPLogin
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		l, err := a.UpdateLogin(loginID, changes, s.now())
		if err != nil {
			return nil, err
		}
		login = l
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPLoginUpdated, a.ClID,
			fmt.Sprintf("EPP login %s updated", l.LoginID),
			entities.EPPCredentialEventData{ClID: a.ClID, LoginID: l.LoginID, Role: role, Enabled: cmd.Enabled, SessionCap: cmd.SessionCap})}, nil
	})
	if err != nil {
		return nil, err
	}
	return login, nil
}

// AddPassword adds a password to a login, generating one when cmd has none.
func (s *EPPCredentialService) AddPassword(ctx context.Context, clid, loginID string, cmd commands.AddEPPPasswordCommand) (*entities.EPPPassword, string, error) {
	plain, generated := cmd.Password, ""
	if plain == "" {
		var err error
		if plain, err = generateEPPPassword(); err != nil {
			return nil, "", err
		}
		generated = plain
	}
	actor := actorOf(ctx)
	var pw *entities.EPPPassword
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		p, err := a.AddPassword(loginID, plain, s.hasher, actor, s.now())
		if err != nil {
			return nil, err
		}
		pw = p
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPPasswordAdded, a.ClID,
			fmt.Sprintf("password added to EPP login %s", loginID),
			entities.EPPCredentialEventData{ClID: a.ClID, LoginID: entities.ClIDType(loginID), CredentialID: p.ID.String(), Kind: entities.EPPCredentialPassword})}, nil
	})
	if err != nil {
		return nil, "", err
	}
	return pw, generated, nil
}

// AddCertificatePin pins the key of a client certificate.
func (s *EPPCredentialService) AddCertificatePin(ctx context.Context, clid string, cmd commands.AddEPPCertificatePinCommand) (*entities.EPPCertificatePin, error) {
	actor := actorOf(ctx)
	var pin *entities.EPPCertificatePin
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		p, err := a.AddCertificatePin([]byte(cmd.CertificatePEM), cmd.Logins, actor, s.now())
		if err != nil {
			return nil, err
		}
		pin = p
		notAfter := p.NotAfter
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPCertificatePinAdded, a.ClID,
			fmt.Sprintf("client certificate key %s pinned for %s", p.SPKISHA256, a.ClID),
			entities.EPPCredentialEventData{ClID: a.ClID, CredentialID: p.ID.String(), Kind: entities.EPPCredentialCertificatePin, SPKISHA256: p.SPKISHA256, NotAfter: &notAfter})}, nil
	})
	if err != nil {
		return nil, err
	}
	return pin, nil
}

// AddAllowedNetwork allows a source network.
func (s *EPPCredentialService) AddAllowedNetwork(ctx context.Context, clid string, cmd commands.AddEPPAllowedNetworkCommand) (*entities.EPPAllowedNetwork, error) {
	actor := actorOf(ctx)
	var network *entities.EPPAllowedNetwork
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		n, err := a.AddAllowedNetwork(cmd.Network, cmd.Note, actor, s.now())
		if err != nil {
			return nil, err
		}
		network = n
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPAllowedNetworkAdded, a.ClID,
			fmt.Sprintf("network %s allowed for %s", n.Prefix, a.ClID),
			entities.EPPCredentialEventData{ClID: a.ClID, CredentialID: n.ID.String(), Kind: entities.EPPCredentialAllowedNetwork, Network: n.Prefix.String()})}, nil
	})
	if err != nil {
		return nil, err
	}
	return network, nil
}

// RetireCredential retires a password, pin or allowed network.
func (s *EPPCredentialService) RetireCredential(ctx context.Context, clid string, ref entities.EPPCredentialRef, cmd commands.RetireEPPCredentialCommand) error {
	kind := entities.EPPRetireKind(cmd.Kind)
	if kind == "" {
		kind = entities.EPPRetireNormal
	}
	actor := actorOf(ctx)
	_, err := s.repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		lc, err := a.RetireCredential(ref, kind, cmd.At, actor, s.now())
		if err != nil {
			return nil, err
		}
		data := entities.EPPCredentialEventData{ClID: a.ClID, CredentialID: ref.ID.String(), Kind: ref.Kind, RetireKind: kind, RetireAt: lc.RetireAt}
		if ref.Kind == entities.EPPCredentialPassword {
			data.LoginID = entities.ClIDType(ref.LoginID)
		}
		return []entities.DomainEvent{s.event(ctx, entities.EPPCredentialRetiredEvent(ref.Kind), a.ClID,
			fmt.Sprintf("EPP %s %s of %s retired (%s)", ref.Kind, ref.ID, a.ClID, kind), data)}, nil
	})
	return err
}

func (s *EPPCredentialService) event(ctx context.Context, eventType string, clid entities.ClIDType, msg string, data entities.EPPCredentialEventData) entities.DomainEvent {
	e := entities.NewDomainEvent(eppEventSource, eventType, string(clid), msg, data)
	if v, ok := appcontext.TraceID(ctx); ok {
		e.TraceID = v
	}
	if v, ok := appcontext.CorrelationID(ctx); ok {
		e.CorrelationID = v
	}
	e.Actor = actorOf(ctx)
	return e
}

func actorOf(ctx context.Context) string {
	if v, ok := appcontext.UserID(ctx); ok {
		return v
	}
	return ""
}

// generateEPPPassword returns a random password of the longest length EPP
// allows.
func generateEPPPassword() (string, error) {
	b := make([]byte, entities.MaxEPPPasswordLength)
	limit := big.NewInt(int64(len(generatedPasswordAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate EPP password: %w", err)
		}
		b[i] = generatedPasswordAlphabet[n.Int64()]
	}
	return string(b), nil
}
