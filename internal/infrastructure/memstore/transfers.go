package memstore

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RequestTransferAsRead stands in for DomainService.RequestTransferAsRead:
// it requests the transfer in the TLD's current GA phase, and stores the
// domain, the transfer and its poll messages together, as the Postgres
// repository does. Pricing and events belong to DomainService.
func (r *Registry) RequestTransferAsRead(_ context.Context, dom *entities.Domain, gaining entities.ClIDType, authInfo string, years int) (*entities.Domain, *entities.DomainTransfer, error) {
	phase, err := r.currentPhase(dom.Name.ParentDomain())
	if err != nil {
		return nil, nil, err
	}
	now := r.now().UTC()
	tr, err := dom.RequestTransfer(gaining, authInfo, years, phase, now)
	if err != nil {
		return nil, nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, nil, r.err
	}
	stored, err := r.writeDomainLocked(dom, dom.ClID)
	if err != nil {
		return nil, nil, err
	}
	r.recordTransfer(tr, entities.NewTransferPollMessages(tr, gaining, now))
	return stored, tr, nil
}

// ResolveTransferAsRead stands in for DomainService.ResolveTransferAsRead,
// moving and re-linking hosts as entities.TransferResolution describes, with
// the nameservers planned as DomainService plans them: those under the
// domain and the external ones.
func (r *Registry) ResolveTransferAsRead(_ context.Context, dom *entities.Domain, tr *entities.DomainTransfer, status entities.TransferStatus, actor entities.ClIDType) (*entities.Domain, *entities.DomainTransfer, error) {
	phase, err := r.currentPhase(dom.Name.ParentDomain())
	if err != nil {
		return nil, nil, err
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, nil, r.err
	}
	all := r.transfers[dom.RoID]
	if len(all) == 0 || all[len(all)-1].ID != tr.ID || all[len(all)-1].Status != entities.TransferStatusPending {
		return nil, nil, entities.ErrDomainChanged
	}
	// Everything is checked before anything changes, as the Postgres
	// transaction refuses a stale approval whole: the domain's version and
	// sponsor, and its nameservers as the plan was made from them.
	current, ok := r.domains[dom.Name.String()]
	switch {
	case !ok:
		return nil, nil, entities.ErrDomainNotFound
	case !current.UpdatedAt.Equal(dom.UpdatedAt) || current.ClID != tr.LosingClID:
		return nil, nil, entities.ErrDomainChanged
	case !sameNameservers(r.copyDomain(current).Hosts, dom.Hosts):
		return nil, nil, entities.ErrDomainChanged
	}
	if err := tr.Resolve(status, actor, now); err != nil {
		return nil, nil, err
	}
	if status.IsApproved() {
		if err := dom.CompleteTransfer(tr, phase, now); err != nil {
			return nil, nil, err
		}
		r.moveHosts(dom, tr, r.relinkPlan(dom))
	} else {
		dom.EndTransfer()
	}
	stored, err := r.writeDomainLocked(dom, tr.LosingClID)
	if err != nil {
		return nil, nil, err
	}
	all[len(all)-1] = copyTransfer(tr)
	r.queue(entities.NewTransferPollMessages(tr, actor, now))
	return stored, tr, nil
}

// moveHosts moves the losing registrar's hosts under dom to the gaining
// registrar, and re-links the nameservers named in relink to the gaining
// registrar's hosts, creating them. The caller holds the lock.
func (r *Registry) moveHosts(dom *entities.Domain, tr *entities.DomainTransfer, relink []string) {
	losing, gaining := tr.LosingClID.String(), tr.RequestingClID.String()
	for key, h := range r.hosts {
		under := key.name == dom.Name.String() || strings.HasSuffix(key.name, "."+dom.Name.String())
		if key.clid != losing || !under {
			continue
		}
		if _, taken := r.hosts[hostKey{key.name, gaining}]; taken {
			continue
		}
		delete(r.hosts, key)
		h.ClID, h.UpRr = tr.RequestingClID, tr.RequestingClID
		r.hosts[hostKey{key.name, gaining}] = h
	}
	for i, h := range dom.Hosts {
		name := h.Name.String()
		if !slices.Contains(relink, name) {
			continue
		}
		target, ok := r.hosts[hostKey{name, gaining}]
		if !ok {
			r.nextID++
			roid, _ := entities.NewRoidType(r.nextID, entities.RoidTypeHost, r.repository)
			target, _ = entities.NewHost(name, roid.String(), gaining)
			target.CreatedAt = r.now().UTC()
			r.hosts[hostKey{name, gaining}] = target
		}
		dom.Hosts[i] = target.DeepCopy()
	}
}

// relinkPlan names dom's nameservers under dom or under no TLD the
// registry knows. The caller holds the lock.
func (r *Registry) relinkPlan(dom *entities.Domain) []string {
	var out []string
	for _, h := range dom.Hosts {
		name := h.Name.String()
		if name == dom.Name.String() || strings.HasSuffix(name, "."+dom.Name.String()) {
			out = append(out, name)
			continue
		}
		internal := false
		labels := strings.Split(name, ".")
		for i := 1; i < len(labels); i++ {
			if r.knownTLDs[strings.Join(labels[i:], ".")] {
				internal = true
				break
			}
		}
		if !internal {
			out = append(out, name)
		}
	}
	return out
}

// sameNameservers reports whether a and b are the same host objects with
// the same names.
func sameNameservers(a, b []*entities.Host) bool {
	if len(a) != len(b) {
		return false
	}
	names := map[entities.RoidType]entities.DomainName{}
	for _, h := range a {
		names[h.RoID] = h.Name
	}
	for _, h := range b {
		if n, ok := names[h.RoID]; !ok || n != h.Name {
			return false
		}
	}
	return true
}

// recordTransfer stores a new transfer, numbering it, and queues msgs. The
// caller holds the lock.
func (r *Registry) recordTransfer(tr *entities.DomainTransfer, msgs []*entities.PollMessage) {
	r.nextID++
	tr.ID = r.nextID
	r.transfers[tr.DomainRoID] = append(r.transfers[tr.DomainRoID], copyTransfer(tr))
	for _, m := range msgs {
		m.Transfer.ID = tr.ID
	}
	r.queue(msgs)
}

// queue numbers and queues msgs. The caller holds the lock.
func (r *Registry) queue(msgs []*entities.PollMessage) {
	for _, m := range msgs {
		r.nextID++
		m.ID = r.nextID
		r.messages = append(r.messages, m)
	}
}

func copyTransfer(t *entities.DomainTransfer) *entities.DomainTransfer {
	c := *t
	return &c
}

// Queue queues msgs, as a write that causes them would.
func (r *Registry) Queue(msgs ...*entities.PollMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue(msgs)
}

// Messages returns a copy of clid's queue, oldest first.
func (r *Registry) Messages(clid string) []*entities.PollMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*entities.PollMessage
	for _, m := range r.messages {
		if m.ClID.String() == clid {
			c := *m
			out = append(out, &c)
		}
	}
	return out
}

// Transfers is the registry as a repositories.DomainTransferRepository, for
// reading; its writes are RequestTransferAsRead and ResolveTransferAsRead.
func (r *Registry) Transfers() repositories.DomainTransferRepository { return registryTransfers{r: r} }

// Polls is the registry as a repositories.PollMessageRepository.
func (r *Registry) Polls() repositories.PollMessageRepository { return registryPolls{r: r} }

type registryTransfers struct {
	repositories.DomainTransferRepository
	r *Registry
}

func (v registryTransfers) LatestTransfer(_ context.Context, roid entities.RoidType) (*entities.DomainTransfer, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	all := v.r.transfers[roid]
	if len(all) == 0 {
		return nil, entities.ErrTransferNotPending
	}
	return copyTransfer(all[len(all)-1]), nil
}

func (v registryTransfers) ListDueTransfers(_ context.Context, dueBy, afterDate time.Time, afterID int64, limit int) ([]*entities.DomainTransfer, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, v.r.err
	}
	live := map[entities.RoidType]bool{}
	for _, d := range v.r.domains {
		live[d.RoID] = true
	}
	after := func(t *entities.DomainTransfer) bool {
		if c := t.ActionDate.Compare(afterDate); c != 0 {
			return c > 0
		}
		return t.ID > afterID
	}
	var out []*entities.DomainTransfer
	for roid, all := range v.r.transfers {
		if t := all[len(all)-1]; live[roid] && t.Status == entities.TransferStatusPending && !t.ActionDate.After(dueBy) && after(t) {
			out = append(out, copyTransfer(t))
		}
	}
	slices.SortFunc(out, func(a, b *entities.DomainTransfer) int {
		if c := a.ActionDate.Compare(b.ActionDate); c != 0 {
			return c
		}
		return int(a.ID - b.ID)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type registryPolls struct {
	r *Registry
}

func (v registryPolls) OldestPollMessage(_ context.Context, clid entities.ClIDType) (*entities.PollMessage, int64, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return nil, 0, v.r.err
	}
	var first *entities.PollMessage
	var n int64
	for _, m := range v.r.messages {
		if m.ClID == clid {
			if first == nil {
				first = m
			}
			n++
		}
	}
	if first == nil {
		return nil, 0, nil
	}
	c := *first
	return &c, n, nil
}

func (v registryPolls) AckPollMessage(_ context.Context, clid entities.ClIDType, id int64) (int64, error) {
	v.r.mu.Lock()
	defer v.r.mu.Unlock()
	if v.r.err != nil {
		return 0, v.r.err
	}
	i := slices.IndexFunc(v.r.messages, func(m *entities.PollMessage) bool { return m.ID == id && m.ClID == clid })
	if i < 0 {
		return 0, entities.ErrPollMessageNotFound
	}
	v.r.messages = slices.Delete(v.r.messages, i, i+1)
	var n int64
	for _, m := range v.r.messages {
		if m.ClID == clid {
			n++
		}
	}
	return n, nil
}
