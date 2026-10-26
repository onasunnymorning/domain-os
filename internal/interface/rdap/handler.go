package rdap

import (
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

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			rec.Header().Set("Allow", "GET, HEAD")
			writeError(rec, http.StatusMethodNotAllowed, "RDAP is read-only: only GET and HEAD are served")
			return
		}
		path, ok := strings.CutPrefix(r.URL.Path, s.basePath)
		if !ok {
			writeError(rec, http.StatusNotFound, "this path is outside the RDAP service's base URL")
			return
		}
		q := parseQuery(path)
		route = q.kind.route()
		s.serve(rec, r, q)
	})
}

// serve answers a parsed query.
func (s *Server) serve(w http.ResponseWriter, r *http.Request, q query) {
	switch q.kind {
	case queryHelp:
		writeJSON(w, http.StatusOK, helpResponse{RDAPConformance: conformance, Notices: s.notices(r)})
	case querySearch:
		writeError(w, http.StatusNotImplemented, "searches are not offered")
	case queryDomain:
		// A domain outside the TLD whose base URL was used is not this
		// TLD's to answer for.
		if q.tld != "" && !strings.HasSuffix(q.arg, "."+q.tld) {
			writeError(w, http.StatusNotFound)
			return
		}
		rec, err := s.lookups.Domain(r.Context(), q.arg)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildDomain(r, rec, rdappolicy.Anonymous()))
			return
		}
		s.answerLookup(w, routeDomain, errors.Is(err, entities.ErrDomainNotFound), err)
	case queryNameserver:
		rec, err := s.lookups.Nameserver(r.Context(), q.arg)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildNameserver(r, rec))
			return
		}
		s.answerLookup(w, routeNameserver, errors.Is(err, entities.ErrHostNotFound), err)
	case queryEntity:
		// Only registrars are served directly, by IANA ID. A contact
		// appears only inside a domain's response (ADR 0020 decision 5).
		ianaID, isRegistrar := registrarHandle(q.arg)
		if !isRegistrar {
			writeError(w, http.StatusNotFound)
			return
		}
		rr, err := s.lookups.Registrar(r.Context(), ianaID)
		if err == nil {
			writeJSON(w, http.StatusOK, s.buildRegistrar(r, rr))
			return
		}
		s.answerLookup(w, routeEntity, errors.Is(err, entities.ErrRegistrarNotFound), err)
	default:
		writeError(w, http.StatusBadRequest, q.problem)
	}
}

// answerLookup answers a lookup that found nothing: 404 if the object does
// not exist, 500 if the registry could not be read.
func (s *Server) answerLookup(w http.ResponseWriter, route string, notFound bool, err error) {
	if notFound {
		writeError(w, http.StatusNotFound)
		return
	}
	s.metrics.lookupErrors.WithLabelValues(route).Inc()
	s.logger.Error("RDAP lookup failed reading the registry; answered 500", "route", route, "error", err)
	writeError(w, http.StatusInternalServerError)
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
// response.
func (s *Server) requestURL(r *http.Request) string {
	return s.baseURL + strings.TrimPrefix(r.URL.EscapedPath(), s.escapedBasePath)
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
