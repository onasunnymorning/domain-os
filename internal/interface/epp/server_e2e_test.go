package epp

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerSession drives a real TLS session through NewServer: greeting,
// login, domain:check, logout. It checks the wiring of the command mux, not
// the behaviour of any one handler.
func TestServerSession(t *testing.T) {
	serverCert, err := SelfSignedCertificate()
	require.NoError(t, err)
	clientCert, err := SelfSignedCertificate()
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := NewServer(serverCert, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()
	t.Cleanup(func() {
		_ = listener.Close()
		<-served
	})

	conn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: true, // #nosec G402 -- test-only: the server presents a certificate generated in this test
		MinVersion:         tls.VersionTLS12,
	})
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))

	greeting := readFrame(t, conn)
	require.NotNil(t, greeting.FindElement("//greeting"), "the server greets on connect")

	writeFrame(t, conn, `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><login>
<clID>registrar1</clID><pw>secret</pw>
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>urn:ietf:params:xml:ns:domain-1.0</objURI></svcs>
</login><clTRID>T1</clTRID></command></epp>`)
	assert.Equal(t, "1000", resultCode(t, readFrame(t, conn)), "login")

	writeFrame(t, conn, `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><check>
<domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>example.test</domain:name></domain:check>
</check><clTRID>T2</clTRID></command></epp>`)
	check := readFrame(t, conn)
	assert.Equal(t, "1000", resultCode(t, check), "check")
	assert.NotNil(t, check.FindElement("//chkData"), "check returns chkData")

	writeFrame(t, conn, `<?xml version="1.0" encoding="UTF-8"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><logout/><clTRID>T3</clTRID></command></epp>`)
	assert.Equal(t, "1500", resultCode(t, readFrame(t, conn)), "logout")

	_, err = conn.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF, "the server closes the session after logout")
}

// writeFrame sends an RFC 5734 data unit: a 4-byte big-endian length that
// counts itself, then the XML.
func writeFrame(t *testing.T, w io.Writer, xml string) {
	t.Helper()
	frame := make([]byte, 4+len(xml))
	binary.BigEndian.PutUint32(frame, uint32(len(frame))) // #nosec G115 -- test frames are a few hundred bytes
	copy(frame[4:], xml)
	_, err := w.Write(frame)
	require.NoError(t, err)
}

func readFrame(t *testing.T, r io.Reader) *etree.Document {
	t.Helper()
	header := make([]byte, 4)
	_, err := io.ReadFull(r, header)
	require.NoError(t, err)
	body := make([]byte, binary.BigEndian.Uint32(header)-4)
	_, err = io.ReadFull(r, body)
	require.NoError(t, err)
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromBytes(body))
	return doc
}

func resultCode(t *testing.T, doc *etree.Document) string {
	t.Helper()
	result := doc.FindElement("//result")
	require.NotNil(t, result, "response has a result")
	return result.SelectAttrValue("code", "")
}
