package memstore

import (
	"context"
	"slices"
	"strings"

	"github.com/Rhymond/go-money"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Launch applications (ADR 0024 §3): the registry as a
// repositories.LaunchApplicationRepository, and the quote an application
// keeps, as services.EPPDomainService needs them.

// Applications is the registry as a repositories.LaunchApplicationRepository.
func (r *Registry) Applications() repositories.LaunchApplicationRepository {
	return registryApplications{r: r}
}

// Application returns a copy of the stored application with ID id, or nil.
func (r *Registry) Application(id string) *entities.LaunchApplication {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.applications[id]; ok {
		return a.DeepCopy()
	}
	return nil
}

// SetApplication changes a stored application, as the operator would:
// allocating or rejecting it, say.
func (r *Registry) SetApplication(id string, fn func(*entities.LaunchApplication)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(r.applications[id])
}

// QuoteRegistration stands in for DomainService.QuoteRegistration: the
// phase's registration price in its base currency, times the years, and
// no premium class. Pricing proper belongs to DomainService.
func (r *Registry) QuoteRegistration(_ context.Context, name, clid string, years int, phase *entities.Phase) (*entities.Quote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	q := entities.NewQuote(phase.Policy.BaseCurrency)
	q.DomainName, q.Clid, q.Years = entities.DomainName(name), entities.ClIDType(clid), years
	q.TransactionType = entities.TransactionTypeRegistration
	if p, err := phase.GetPrice(phase.Policy.BaseCurrency); err == nil {
		q.Price = money.New(int64(p.RegistrationAmount)*int64(years), phase.Policy.BaseCurrency) // #nosec G115 -- a fixture's price
	}
	return q, nil
}

type registryApplications struct {
	repositories.LaunchApplicationRepository
	r *Registry
}

func (v registryApplications) CreateApplication(_ context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	for _, o := range v.r.applications {
		if o.Name == a.Name && o.PhaseName == a.PhaseName && o.ClID == a.ClID && !o.Status.IsFinal() {
			return nil, entities.ErrApplicationExists
		}
	}
	stored := a.DeepCopy()
	v.r.applications[a.ID.String()] = stored
	return stored.DeepCopy(), nil
}

func (v registryApplications) GetApplicationByID(_ context.Context, id entities.RoidType) (*entities.LaunchApplication, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	if a, ok := v.r.applications[id.String()]; ok {
		return a.DeepCopy(), nil
	}
	return nil, entities.ErrApplicationNotFound
}

func (v registryApplications) ListApplicationsByName(_ context.Context, name string) ([]*entities.LaunchApplication, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	var out []*entities.LaunchApplication
	for _, a := range v.r.applications {
		if a.Name.String() == strings.ToLower(name) {
			out = append(out, a.DeepCopy())
		}
	}
	slices.SortFunc(out, func(a, b *entities.LaunchApplication) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

func (v registryApplications) CountApplicationsByPhase(_ context.Context, tld, phase string) (int64, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return 0, v.r.err
	}
	var n int64
	for _, a := range v.r.applications {
		if a.TLDName.String() == tld && a.PhaseName.String() == phase {
			n++
		}
	}
	return n, nil
}

// UpdateApplicationIfUnchanged refuses an application changed since a was
// read (ErrApplicationChanged), as the Postgres repository does.
func (v registryApplications) UpdateApplicationIfUnchanged(_ context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	stored, ok := v.r.applications[a.ID.String()]
	switch {
	case !ok:
		return nil, entities.ErrApplicationNotFound
	case !stored.UpdatedAt.Equal(a.UpdatedAt):
		return nil, entities.ErrApplicationChanged
	}
	next := a.DeepCopy()
	next.UpdatedAt = v.r.now().UTC()
	v.r.applications[a.ID.String()] = next
	return next.DeepCopy(), nil
}

func (v registryApplications) DeleteApplicationIfUnchanged(_ context.Context, a *entities.LaunchApplication) error {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return v.r.err
	}
	stored, ok := v.r.applications[a.ID.String()]
	switch {
	case !ok:
		return entities.ErrApplicationNotFound
	case !stored.UpdatedAt.Equal(a.UpdatedAt):
		return entities.ErrApplicationChanged
	}
	delete(v.r.applications, a.ID.String())
	return nil
}

// GetPhaseByTLDAndName returns a copy of the phase, as the Postgres
// repository does, or entities.ErrPhaseNotFound.
func (v registryPhases) GetPhaseByTLDAndName(_ context.Context, tld, name string) (*entities.Phase, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	for _, p := range v.r.phases[tld] {
		if p.Name.String() == name {
			c := *p
			return &c, nil
		}
	}
	return nil, entities.ErrPhaseNotFound
}
