package rdap

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// queryKind is what a request path asks for (RFC 9082 §3).
type queryKind int

const (
	queryInvalid    queryKind = iota // a path that is not an RDAP query: 400
	queryHelp                        // help
	queryDomain                      // domain/{name}
	queryNameserver                  // nameserver/{name}
	queryEntity                      // entity/{handle}
	querySearch                      // domains, nameservers, entities: not offered, 501
)

// route is the metrics label for the kind.
func (k queryKind) route() string {
	switch k {
	case queryHelp:
		return routeHelp
	case queryDomain:
		return routeDomain
	case queryNameserver:
		return routeNameserver
	case queryEntity:
		return routeEntity
	case querySearch:
		return routeSearch
	default:
		return routeInvalid
	}
}

// query is a parsed request path.
type query struct {
	kind queryKind
	// tld is the TLD whose base URL the request came in under (an A-label),
	// or empty for the registry-wide base.
	tld string
	// arg is the domain or nameserver name (an A-label) or the entity
	// handle, as given.
	arg string
	// problem says why a path is invalid, for the 400's description.
	problem string
}

// pathWords are the first segments of every RDAP query path. A first
// segment that is none of these is a TLD's base URL.
var pathWords = map[string]queryKind{
	"help":        queryHelp,
	"domain":      queryDomain,
	"nameserver":  queryNameserver,
	"entity":      queryEntity,
	"domains":     querySearch,
	"nameservers": querySearch,
	"entities":    querySearch,
}

// maxNameLength is the longest DNS name (RFC 1035 §3.1), and the longest
// entity handle accepted; a handle is far shorter in practice.
const maxNameLength = 253

// maxLabelLength is the longest DNS label.
const maxLabelLength = 63

// maxRawNameLength bounds a name before IDNA conversion. A U-label is at
// most a few times longer in UTF-8 than its A-label.
const maxRawNameLength = 1024

// parseQuery reads path, which has had the base path removed: either the
// query itself (domain/example.tld) or the query under a TLD's base URL
// (tld/domain/example.tld).
//
// A TLD can be named like a query keyword (.help, .domains), so a path is
// read as TLD-prefixed whenever its second segment starts a query that fits
// the rest of the path: /help/help is help under .help, /domains/help help
// under .domains. Only otherwise is a first segment that is a keyword the
// registry-wide query.
func parseQuery(path string) query {
	segments := strings.Split(path, "/")
	if len(segments) >= 2 && fitsQuery(segments[1:]) {
		if tld, err := normalizeName(segments[0]); err == nil {
			q := parseSegments(segments[1:])
			if q.kind != queryInvalid {
				q.tld = tld
				return q
			}
		}
	}
	if _, isWord := pathWords[segments[0]]; isWord {
		return parseSegments(segments)
	}
	if segments[0] == "" {
		return query{problem: "no query in the path"}
	}
	tld, err := normalizeName(segments[0])
	if err != nil {
		return query{problem: "the path names neither an RDAP query nor a TLD"}
	}
	q := parseSegments(segments[1:])
	if q.kind != queryInvalid {
		q.tld = tld
	}
	return q
}

// fitsQuery reports whether segments have the shape of a query: a keyword
// and the number of arguments it takes.
func fitsQuery(segments []string) bool {
	kind, ok := pathWords[segments[0]]
	if !ok {
		return false
	}
	switch kind {
	case queryHelp:
		return len(segments) == 1
	case querySearch:
		return true
	default:
		return len(segments) == 2 && segments[1] != ""
	}
}

// parseSegments reads a query without a TLD prefix.
func parseSegments(segments []string) query {
	if len(segments) == 0 || segments[0] == "" {
		return query{problem: "no query in the path"}
	}
	kind, ok := pathWords[segments[0]]
	if !ok {
		return query{problem: "unknown query " + strconv.Quote(segments[0])}
	}
	q := query{kind: kind}
	switch kind {
	case queryHelp:
		if len(segments) != 1 {
			return query{problem: "help takes no argument"}
		}
	case querySearch:
		// Answered 501 whatever follows.
	case queryDomain, queryNameserver:
		if len(segments) != 2 || segments[1] == "" {
			return query{problem: segments[0] + " takes one name"}
		}
		name, err := normalizeName(segments[1])
		if err != nil {
			return query{problem: "not a valid " + segments[0] + " name: " + err.Error()}
		}
		if kind == queryDomain && !strings.Contains(name, ".") {
			return query{problem: "a domain name has at least two labels"}
		}
		q.arg = name
	case queryEntity:
		if len(segments) != 2 || segments[1] == "" || len(segments[1]) > maxNameLength {
			return query{problem: "entity takes one handle"}
		}
		q.arg = segments[1]
	}
	return q
}

// normalizeName returns the A-label form of a domain or host name given as
// A-labels or U-labels (RFC 9082 §3.1.3), lower-cased and without a trailing
// dot. DNS's limits (RFC 1035 §2.3.4) apply to the A-label form, so they are
// checked after conversion: a Unicode name longer than 253 bytes can still
// be a valid name.
func normalizeName(raw string) (string, error) {
	name := strings.TrimSuffix(raw, ".")
	// Bounds the work on a hostile path; any real name is far shorter.
	if name == "" || len(name) > maxRawNameLength {
		return "", errors.New("empty or far too long")
	}
	ascii, err := idna.Lookup.ToASCII(name)
	if err != nil {
		return "", err
	}
	ascii = strings.ToLower(ascii)
	if len(ascii) > maxNameLength {
		return "", fmt.Errorf("longer than %d characters as A-labels", maxNameLength)
	}
	for _, label := range strings.Split(ascii, ".") {
		if label == "" || len(label) > maxLabelLength {
			return "", fmt.Errorf("a label is empty or longer than %d characters", maxLabelLength)
		}
	}
	return ascii, nil
}

// registrarHandle reads an entity handle as a registrar's IANA ID, the
// handle the gTLD profile gives registrars. ok is false for any other
// handle.
func registrarHandle(handle string) (ianaID int, ok bool) {
	if handle == "" || len(handle) > 10 || strings.TrimLeft(handle, "0123456789") != "" {
		return 0, false
	}
	id, err := strconv.Atoi(handle)
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}
