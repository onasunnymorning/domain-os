package rdap

import (
	"errors"
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
// argument accepted; an entity handle is far shorter in practice.
const maxNameLength = 253

// parseQuery reads path, which has had the base path removed: either the
// query itself (domain/example.tld) or the query under a TLD's base URL
// (tld/domain/example.tld).
func parseQuery(path string) query {
	segments := strings.Split(path, "/")
	q := query{}
	if len(segments) > 0 {
		if _, isWord := pathWords[segments[0]]; !isWord && segments[0] != "" {
			tld, err := normalizeName(segments[0])
			if err != nil || strings.Contains(tld, ".") {
				return query{problem: "the path names neither an RDAP query nor a TLD"}
			}
			q.tld = tld
			segments = segments[1:]
		}
	}
	if len(segments) == 0 {
		return query{problem: "no query in the path"}
	}
	kind, ok := pathWords[segments[0]]
	if !ok {
		return query{problem: "unknown query " + strconv.Quote(segments[0])}
	}
	q.kind = kind
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
// dot.
func normalizeName(raw string) (string, error) {
	name := strings.TrimSuffix(raw, ".")
	if name == "" || len(name) > maxNameLength {
		return "", errors.New("empty or longer than 253 characters")
	}
	ascii, err := idna.Lookup.ToASCII(name)
	if err != nil {
		return "", err
	}
	return strings.ToLower(ascii), nil
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
