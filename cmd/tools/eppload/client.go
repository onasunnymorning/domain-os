package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// maxFrame bounds a response this client reads.
const maxFrame = 4 << 20

// conn is one EPP session over TLS (RFC 5734): each frame is a 4-byte
// big-endian length that counts itself, then the XML.
type conn struct {
	c       net.Conn
	timeout time.Duration
}

// dialEPP connects, reads the greeting, and returns the session.
func dialEPP(addr string, cfg *tls.Config, timeout time.Duration) (*conn, error) {
	d := &net.Dialer{Timeout: timeout}
	c, err := tls.DialWithDialer(d, "tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	e := &conn{c: c, timeout: timeout}
	if _, err := e.read(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("read the greeting: %w", err)
	}
	return e, nil
}

func (e *conn) write(frame []byte) error {
	if err := e.c.SetWriteDeadline(time.Now().Add(e.timeout)); err != nil {
		return err
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(frame)+4)) //nolint:gosec // G115: frames this tool builds are a few KB
	if _, err := e.c.Write(hdr[:]); err != nil {
		return err
	}
	_, err := e.c.Write(frame)
	return err
}

func (e *conn) read() ([]byte, error) {
	if err := e.c.SetReadDeadline(time.Now().Add(e.timeout)); err != nil {
		return nil, err
	}
	var hdr [4]byte
	if _, err := io.ReadFull(e.c, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n < 4 || n > maxFrame {
		return nil, fmt.Errorf("frame length %d", n)
	}
	body := make([]byte, n-4)
	_, err := io.ReadFull(e.c, body)
	return body, err
}

// result is what the tool reads from a response.
type result struct {
	Code    int
	Msg     string
	Reason  string
	Latency time.Duration
	// Available is each checked name's availability, for a check.
	Available map[string]bool
}

// closing reports whether the server ends the session after this result.
func (r result) closing() bool {
	return r.Code == 1500 || r.Code == 2500 || r.Code == 2501 || r.Code == 2502
}

type eppResponse struct {
	Results []struct {
		Code     int    `xml:"code,attr"`
		Msg      string `xml:"msg"`
		ExtValue struct {
			Reason string `xml:"reason"`
		} `xml:"extValue"`
	} `xml:"response>result"`
	Checks []struct {
		Name struct {
			Avail string `xml:"avail,attr"`
			Value string `xml:",chardata"`
		} `xml:"name"`
	} `xml:"response>resData>chkData>cd"`
}

// roundTrip sends frame and reads its response.
func (e *conn) roundTrip(frame []byte) (result, error) {
	start := time.Now()
	if err := e.write(frame); err != nil {
		return result{}, err
	}
	body, err := e.read()
	if err != nil {
		return result{}, err
	}
	r := result{Latency: time.Since(start)}
	var resp eppResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return r, fmt.Errorf("parse the response: %w", err)
	}
	if len(resp.Results) == 0 {
		return r, errors.New("the response has no result")
	}
	r.Code, r.Msg, r.Reason = resp.Results[0].Code, strings.TrimSpace(resp.Results[0].Msg), strings.TrimSpace(resp.Results[0].ExtValue.Reason)
	if len(resp.Checks) > 0 {
		r.Available = make(map[string]bool, len(resp.Checks))
		for _, cd := range resp.Checks {
			r.Available[strings.TrimSpace(cd.Name.Value)] = cd.Name.Avail == "1" || cd.Name.Avail == "true"
		}
	}
	return r, nil
}

func (e *conn) close() { _ = e.c.Close() }

// ---------------------------------------------------------------------------
// Frames
// ---------------------------------------------------------------------------

const (
	nsEPP    = "urn:ietf:params:xml:ns:epp-1.0"
	nsDomain = "urn:ietf:params:xml:ns:domain-1.0"
)

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func command(body, clTRID string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><epp xmlns="` + nsEPP + `"><command>` + body +
		`<clTRID>` + esc(clTRID) + `</clTRID></command></epp>`)
}

func loginFrame(login, password, clTRID string) []byte {
	return command(`<login><clID>`+esc(login)+`</clID><pw>`+esc(password)+`</pw>`+
		`<options><version>1.0</version><lang>en</lang></options>`+
		`<svcs><objURI>`+nsDomain+`</objURI></svcs></login>`, clTRID)
}

func logoutFrame(clTRID string) []byte { return command(`<logout/>`, clTRID) }

func checkFrame(names []string, clTRID string) []byte {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(`<domain:name>` + esc(n) + `</domain:name>`)
	}
	return command(`<check><domain:check xmlns:domain="`+nsDomain+`">`+b.String()+`</domain:check></check>`, clTRID)
}

// createSpec is what a <domain:create> carries besides the name.
type createSpec struct {
	Registrant string
	Contacts   map[string]string // type: id
	AuthInfo   string
}

func createFrame(name string, s createSpec, clTRID string) []byte {
	var b strings.Builder
	b.WriteString(`<domain:name>` + esc(name) + `</domain:name><domain:period unit="y">1</domain:period>`)
	if s.Registrant != "" {
		b.WriteString(`<domain:registrant>` + esc(s.Registrant) + `</domain:registrant>`)
	}
	for _, typ := range []string{"admin", "tech", "billing"} {
		if id := s.Contacts[typ]; id != "" {
			b.WriteString(`<domain:contact type="` + typ + `">` + esc(id) + `</domain:contact>`)
		}
	}
	b.WriteString(`<domain:authInfo><domain:pw>` + esc(s.AuthInfo) + `</domain:pw></domain:authInfo>`)
	return command(`<create><domain:create xmlns:domain="`+nsDomain+`">`+b.String()+`</domain:create></create>`, clTRID)
}
