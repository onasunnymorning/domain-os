package rdevalidate

import (
	"hash/fnv"
	"sort"
	"strings"
)

// MaxCrossReferenceObjects bounds how many identifiers the referential check
// holds at once. The check has to see the whole deposit before it can say
// anything — a host may be declared forty thousand objects before the domain
// that uses it, and this deposit's header is the last element in the file — so
// unlike every other check here it cannot work in constant space. Past the
// bound it gives up and says so rather than growing without limit inside a
// worker that is also streaming a multi-gigabyte deposit.
//
// An entry is a uint64 key and an int value, so a Go map costs roughly 24
// bytes for each once buckets and load factor are counted: five million of
// them is about 120 MB. A TLD large enough to pass that can raise the bound,
// but it is a memory decision and should be made deliberately.
const MaxCrossReferenceObjects = 5_000_000

// MaxCrossReferenceNames bounds how many of those identifiers the check also
// keeps verbatim, so it can name the object a finding is about rather than
// only its ordinal. A name costs a string header and its bytes on top of the
// entry — call it seventy bytes for a host name — so keeping every one would
// roughly triple the ceiling above. This bound is deliberately the smaller of
// the two: past it the index keeps counting and comparing, and findings
// degrade to an ordinal and a byte offset, which is what they carried before
// Finding.Object existed. A deposit that large is already being read with the
// file in hand.
const MaxCrossReferenceNames = 1_000_000

// crossRef answers two questions about a FULL deposit, which differ in what
// they mean and so in what they cost:
//
// Does every contact and host in it belong to a domain in it? An orphan is
// data a successor registry would inherit with nothing pointing at it, and for
// a contact it is personal data with nothing left to justify keeping it. RFC
// 9022 does not forbid one, so it is a WARNING.
//
// Does every reference a domain makes resolve inside the deposit? A reference
// that does not is an ERROR: the deposit is not integral and the domain that
// made it cannot be imported, which is the thing escrow exists to guarantee.
//
// It compares 64-bit hashes rather than the identifiers themselves, which
// halves the memory the comparison costs. The cost is that two identifiers
// could collide and an orphan go unreported: at five million entries the odds
// are about one in a million, and the failure is a missing warning.
//
// It does keep the identifiers verbatim, in one bounded side table, purely so
// a finding can say which contact or host it means (Finding.Object). Nothing
// else reads them: every message, rule and locator the check emits is still
// built from constant text and ordinals.
type crossRef struct {
	// enabled is false for a DIFF or INCR deposit, where a contact or host
	// belonging to a domain deposited earlier is correct, not an orphan.
	enabled bool
	// bailiwick is the TLD the deposit was bound to at intake, with a leading
	// dot. A domain may point at a nameserver under another TLD, which RFC 9022
	// does not ask the deposit to carry; only in-bailiwick hosts must resolve.
	// Empty disables the dangling-host check rather than guessing.
	bailiwick string

	contacts    map[uint64]int // declared contact id -> its ordinal in the deposit
	hosts       map[uint64]int // declared host name  -> its ordinal
	usedContact map[uint64]int // referenced contact id -> ordinal of the first domain to use it
	usedHost    map[uint64]int // every referenced nameserver -> same

	// usedHostInBailiwick is the subset of usedHost under the bound TLD, and
	// only it can be dangling. The two are kept apart because the questions
	// differ: a registry running the host-object model declares an object for
	// every nameserver its domains use, most of them under other TLDs, and
	// judging those by the in-bailiwick references alone called 2,168 hosts
	// orphans in a real deposit that its own domains delegate to.
	usedHostInBailiwick map[uint64]int

	// names maps a key back to the identifier that produced it, for the
	// findings alone. Bounded separately by MaxCrossReferenceNames; a key with
	// no entry here yields a finding with no Object, not a wrong one.
	names map[uint64]string

	entries    int
	overflowed bool
}

func newCrossRef(boundTLD string) *crossRef {
	x := &crossRef{
		contacts:    map[uint64]int{},
		hosts:       map[uint64]int{},
		usedContact: map[uint64]int{},
		usedHost:    map[uint64]int{},

		usedHostInBailiwick: map[uint64]int{},
		names:               map[uint64]string{},
	}
	if tld := strings.Trim(strings.ToLower(strings.TrimSpace(boundTLD)), "."); tld != "" {
		x.bailiwick = "." + tld
	}
	return x
}

// key hashes an identifier for comparison. Host names are DNS names and EPP
// contact ids are tokens matched without regard to case, so both are folded
// before hashing; comparing them as written would report a deposit that
// capitalises one reference differently as broken.
func key(id string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(id))))
	return h.Sum64()
}

// put records id in m under ordinal, unless the bound has been reached, and
// remembers the identifier itself while there is room for it.
func (x *crossRef) put(m map[uint64]int, id string, ordinal int) {
	if !x.enabled || x.overflowed {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	k := key(id)
	if _, seen := m[k]; seen {
		return
	}
	if x.entries >= MaxCrossReferenceObjects {
		x.overflowed = true
		return
	}
	x.entries++
	m[k] = ordinal
	// Recorded on first sight from either direction, so a domain that
	// references an object the deposit never declares still has a name to
	// report. The first spelling wins; the rest differ only in case.
	if _, named := x.names[k]; !named && len(x.names) < MaxCrossReferenceNames {
		x.names[k] = id
	}
}

func (x *crossRef) declareContact(id string, ordinal int) { x.put(x.contacts, id, ordinal) }
func (x *crossRef) declareHost(name string, ordinal int)  { x.put(x.hosts, name, ordinal) }

// useContact records that the domain at this ordinal points at a contact.
func (x *crossRef) useContact(id string, domainOrdinal int) {
	x.put(x.usedContact, id, domainOrdinal)
}

// useHost records a domain's nameserver. Every one counts towards whether a
// declared host is used; only an in-bailiwick one counts towards whether the
// deposit was obliged to carry it, because a nameserver under another TLD is
// somebody else's host object.
func (x *crossRef) useHost(name string, domainOrdinal int) {
	n := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(name, ".")))
	x.put(x.usedHost, n, domainOrdinal)
	if x.bailiwick != "" && strings.HasSuffix(n, x.bailiwick) {
		x.put(x.usedHostInBailiwick, n, domainOrdinal)
	}
}

// report emits the findings. Orphans are reported per object so an operator
// can go and look at one; the ordinals are sorted so two runs over the same
// deposit produce the same findings in the same order, which map iteration
// alone would not give.
//
// One finding per orphan is what makes the tally exact — Result.Add counts
// every finding and retains only a few — but it does mean the caller's slice
// holds one entry per orphan until the pipeline funnels it in. A deposit whose
// every host is an orphan therefore costs about 130 bytes each here, on top of
// the index. MaxCrossReferenceObjects is what bounds it.
func (x *crossRef) report(add func(code Code, sev Severity, stage Stage, objType, object, locator, rule, msg string)) {
	if !x.enabled {
		return
	}
	if x.overflowed {
		// A WARNING rather than a note: the check that did not run is the one
		// that decides whether the deposit is integral, so a reader has to be
		// told that nothing here says it is.
		add(CodeRDECrossReferenceSkipped, SeverityWarning, StageRDE, "", "", "",
			"deposit exceeds "+itoa(MaxCrossReferenceObjects)+" cross-referenced identifiers",
			"contact and host references were not checked: the deposit carries more identifiers than the check holds")
		return
	}

	orphans := func(declared, used map[uint64]int, objType, rule, msg string) {
		at := x.unmatched(declared, used)
		for _, o := range at {
			add(CodeRDEObjectNotReferenced, SeverityWarning, StageRDE, objType,
				o.name, objType+"#"+itoa(o.ordinal), rule, msg)
		}
	}
	orphans(x.contacts, x.usedContact, "contact",
		"no domain in this FULL deposit references this contact",
		"contact belongs to no domain in the deposit and would be imported with nothing pointing at it")
	orphans(x.hosts, x.usedHost, "host",
		"no domain in this FULL deposit references this host",
		"host belongs to no domain in the deposit and would be imported with nothing pointing at it")

	// A reference that resolves to nothing is an ERROR, unlike an orphan. An
	// orphan is data nobody asked for; a broken reference means the deposit is
	// not integral and a successor registry cannot import the domain that made
	// it, which is the whole thing escrow exists to guarantee.
	// The Object here is the identifier that could not be resolved, not the
	// domain that named it: that identifier is what an operator has to go and
	// find, and the locator already says which domain wanted it.
	dangling := func(used, declared map[uint64]int, objType, rule, msg string) {
		for _, o := range x.unmatched(used, declared) {
			add(CodeRDEReferenceNotInDeposit, SeverityError, StageRDE, objType,
				o.name, "domain#"+itoa(o.ordinal), rule, msg)
		}
	}
	dangling(x.usedContact, x.contacts, "contact",
		"a domain references a contact this FULL deposit does not carry",
		"domain points at a contact that is not in the deposit, so the reference cannot be resolved")
	dangling(x.usedHostInBailiwick, x.hosts, "host",
		"a domain references an in-bailiwick host this FULL deposit does not carry",
		"domain points at a nameserver under this TLD that is not in the deposit, so the reference cannot be resolved")
}

// unmatchedEntry is one key of have that miss does not carry, carrying the
// ordinal a finding locates it by and the identifier it names.
type unmatchedEntry struct {
	ordinal int
	name    string
}

// unmatched is every key in have that is absent from miss, ordered by
// ordinal. The order is what makes two runs over the same deposit emit the
// same findings in the same sequence; ranging a map alone would not.
func (x *crossRef) unmatched(have, miss map[uint64]int) []unmatchedEntry {
	var out []unmatchedEntry
	for k, ordinal := range have {
		if _, present := miss[k]; !present {
			out = append(out, unmatchedEntry{ordinal: ordinal, name: x.names[k]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ordinal < out[j].ordinal })
	return out
}

// declare folds one decoded object into the index. The ordinal is the object's
// position among others of its kind, which is what a finding's locator names.
func (x *crossRef) declare(objectType string, out objectResult, ordinal int) {
	if !x.enabled {
		return
	}
	switch objectType {
	case "contact":
		x.declareContact(out.Name, ordinal)
	case "host":
		x.declareHost(out.Name, ordinal)
	case "domain":
		for _, id := range out.Refs {
			x.useContact(id, ordinal)
		}
		for _, h := range out.RefHosts {
			x.useHost(h, ordinal)
		}
	}
}
