package rdap

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// contentType is the RDAP media type (RFC 7480 §4.2).
const contentType = "application/rdap+json"

// conformance is every response's rdapConformance: RDAP itself and the
// February 2024 gTLD RDAP Profile, its technical implementation guide and its
// response profile. "redacted" (RFC 9537) joins it with the first response
// that can redact (Phase 2).
var conformance = []string{
	"rdap_level_0",
	"icann_rdap_technical_implementation_guide_1",
	"icann_rdap_response_profile_1",
}

// link is an RFC 9083 §4.2 link.
type link struct {
	Value string `json:"value,omitempty"`
	Rel   string `json:"rel"`
	Href  string `json:"href"`
	Type  string `json:"type,omitempty"`
}

// notice is an RFC 9083 §4.3 notice.
type notice struct {
	Title       string   `json:"title,omitempty"`
	Description []string `json:"description"`
	Links       []link   `json:"links,omitempty"`
}

// helpResponse is the answer to /help (RFC 9083 §7).
type helpResponse struct {
	RDAPConformance []string `json:"rdapConformance"`
	Notices         []notice `json:"notices,omitempty"`
}

// errorResponse is an RFC 9083 §6 error. Like every top-level response it
// carries the Terms of Use notice.
type errorResponse struct {
	RDAPConformance []string `json:"rdapConformance"`
	ErrorCode       int      `json:"errorCode"`
	Title           string   `json:"title"`
	Description     []string `json:"description,omitempty"`
	Notices         []notice `json:"notices,omitempty"`
}

// cachePublic is how long a shared cache may keep a public answer
// (ADR 0020 decision 7). Nothing is per-requester yet; Vary: Authorization is
// set now so that adding an authenticated tier cannot leak into a cache set
// up before it existed.
const cachePublic = "public, max-age=60"

// writeJSON writes v with the headers every RDAP response carries. For a
// HEAD request net/http sends the headers and drops the body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Access-Control-Allow-Origin", "*")
	// A browser reads only safelisted headers from a cross-origin answer
	// unless told otherwise: the 429 and 503 backoff is in Retry-After.
	h.Set("Access-Control-Expose-Headers", "Retry-After")
	h.Set("Vary", "Authorization")
	switch status {
	case http.StatusOK, http.StatusNotFound:
		h.Set("Cache-Control", cachePublic)
	default:
		h.Set("Cache-Control", "no-store")
	}
	body, err := json.Marshal(v)
	if err != nil {
		// Every value here is built from plain structs; this cannot happen.
		status, body = http.StatusInternalServerError, []byte(`{"errorCode":500,"title":"Internal Server Error"}`)
	}
	// The length lets a client take the response as complete as soon as
	// it is flushed, before the handler has returned.
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeErrorObject writes an RDAP error object with the status as its code.
// Handlers use Server.fail, which adds the notices.
func writeErrorObject(w http.ResponseWriter, status int, notices []notice, description ...string) {
	writeJSON(w, status, errorResponse{
		RDAPConformance: conformance,
		ErrorCode:       status,
		Title:           http.StatusText(status),
		Description:     description,
		Notices:         notices,
	})
}
