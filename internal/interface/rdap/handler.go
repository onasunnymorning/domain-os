package rdap

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// handler answers RDAP requests: it records each request's metrics, refuses
// what is not a lookup, parses the path and dispatches the query.
func (s *Server) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.inFlight.Inc()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		route := routeInvalid
		defer func() {
			s.metrics.inFlight.Dec()
			s.metrics.requests.WithLabelValues(route, strconv.Itoa(rec.status)).Inc()
			s.metrics.requestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		}()

		select {
		case s.inFlight <- struct{}{}:
			defer func() { <-s.inFlight }()
		default:
			// The node is full. A 503 is an outage to the SLA monitors,
			// so this is alerted on; it is not how clients are limited.
			s.metrics.shed.WithLabelValues(shedInFlight).Inc()
			rec.Header().Set("Retry-After", "1")
			s.fail(rec, r, http.StatusServiceUnavailable, "the server is busy; retry shortly")
			return
		}

		chargeNotFound, admitted := s.admit(rec, r)
		if !admitted {
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			rec.Header().Set("Allow", "GET, HEAD")
			s.fail(rec, r, http.StatusMethodNotAllowed, "RDAP is read-only: only GET and HEAD are served")
			return
		}
		path, ok := strings.CutPrefix(r.URL.Path, s.basePath)
		if !ok {
			s.fail(rec, r, http.StatusNotFound, "this path is outside the RDAP service's base URL")
			return
		}
		q := parseQuery(path)
		route = q.kind.route()
		// Lookups must end before the response's write deadline, with
		// time left to answer with an RDAP error.
		ctx, cancel := context.WithTimeout(r.Context(), s.lookupDeadline())
		defer cancel()
		s.serve(rec, r.WithContext(ctx), q)
		// A lookup that found nothing costs more, to slow enumeration. The
		// charge runs after the handler returns, so it adds nothing to the
		// client's wait on any protocol (an HTTP/2 stream ends only when the
		// handler does). Shutdown waits for it.
		if rec.status == http.StatusNotFound && (q.kind == queryDomain || q.kind == queryNameserver || q.kind == queryEntity) {
			s.charges.Add(1)
			go func() {
				defer s.charges.Done()
				chargeNotFound()
			}()
		}
	})
}

// serve answers a parsed query.
func (s *Server) serve(w http.ResponseWriter, r *http.Request, q query) {
	switch q.kind {
	case queryHelp:
		writeJSON(w, http.StatusOK, helpResponse{RDAPConformance: conformance, Notices: s.notices(r)})
	case querySearch:
		s.fail(w, r, http.StatusNotImplemented, "searches are not offered")
	case queryDomain:
		// A domain outside the TLD whose base URL was used is not this
		// TLD's to answer for.
		if q.tld != "" && !strings.HasSuffix(q.arg, "."+q.tld) {
			s.fail(w, r, http.StatusNotFound)
			return
		}
		rec, err := s.lookups.Domain(r.Context(), q.arg)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildDomain(r, rec, rdappolicy.Anonymous()))
			return
		}
		s.answerLookup(w, r, routeDomain, errors.Is(err, entities.ErrDomainNotFound), err)
	case queryNameserver:
		rec, err := s.lookups.Nameserver(r.Context(), q.arg)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildNameserver(r, rec))
			return
		}
		s.answerLookup(w, r, routeNameserver, errors.Is(err, entities.ErrHostNotFound), err)
	case queryEntity:
		// Only registrars are served directly, by IANA ID. A contact
		// appears only inside a domain's response (ADR 0020 decision 5).
		ianaID, isRegistrar := registrarHandle(q.arg)
		if !isRegistrar {
			s.fail(w, r, http.StatusNotFound)
			return
		}
		rr, err := s.lookups.Registrar(r.Context(), ianaID)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildRegistrar(r, rr))
			return
		}
		s.answerLookup(w, r, routeEntity, errors.Is(err, entities.ErrRegistrarNotFound), err)
	default:
		s.fail(w, r, http.StatusBadRequest, q.problem)
	}
}

// answerLookup answers a lookup that found nothing: 404 if the object does
// not exist, 500 if the registry could not be read.
func (s *Server) answerLookup(w http.ResponseWriter, r *http.Request, route string, notFound bool, err error) {
	if notFound {
		s.fail(w, r, http.StatusNotFound)
		return
	}
	s.metrics.lookupErrors.WithLabelValues(route).Inc()
	s.logger.Error("RDAP lookup failed reading the registry; answered 500", "route", route, "error", err)
	s.fail(w, r, http.StatusInternalServerError)
}

// notices are the gTLD profile's notices that every top-level response
// carries. Domain responses add the Status Codes and RDDS Inaccuracy
// notices (domainNotices).
func (s *Server) notices(r *http.Request) []notice {
	if s.termsOfService == "" {
		return nil
	}
	return []notice{{
		Title:       "Terms of Use",
		Description: []string{"Service subject to the terms of use."},
		Links:       []link{{Value: s.requestURL(r), Rel: "terms-of-service", Href: s.termsOfService, Type: "text/html"}},
	}}
}

// requestURL is the public URL of r, the context of every link in its
// response: the queried URI, query string included (the gTLD profile's
// tool checks this).
func (s *Server) requestURL(r *http.Request) string {
	// The public origin and the request's own escaped path: exactly the
	// queried URI, however the path was spelled and wherever it routed.
	u := s.origin + r.URL.EscapedPath()
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		u += "?" + r.URL.RawQuery
	}
	return u
}

// fail answers r with an RDAP error object carrying the notices.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, description ...string) {
	writeErrorObject(w, status, s.notices(r), description...)
}

// lookupDeadline is how long a request's lookups may take: three quarters
// of the write timeout, leaving the rest to answer.
func (s *Server) lookupDeadline() time.Duration {
	if d := s.cfg.Timeouts.Write; d > 0 {
		return d * 3 / 4
	}
	return 10 * time.Second
}

// statusRecorder keeps the status a handler wrote, for metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
