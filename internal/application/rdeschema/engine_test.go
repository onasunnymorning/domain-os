package rdeschema_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validDoc() string {
	return string(rdetest.BuildXML(rdetest.DepositOpts{TLD: "example", Domains: 2, Contacts: 2, Hosts: 2, Registrars: 1}))
}

func verdictFor(t *testing.T, eng rdeschema.Engine, doc string) rdeschema.Verdict {
	t.Helper()
	s, err := eng.Start(context.Background())
	require.NoError(t, err)
	_, err = s.Write([]byte(doc))
	require.NoError(t, err)
	return s.Finish()
}

func TestEngine_IsHealthyAndDescribesItself(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	require.NoError(t, eng.Err())
	assert.Contains(t, eng.Description(), "xmllint")
	d, err := rdeschema.SetDigest()
	require.NoError(t, err)
	assert.Len(t, d, 64)
	ns, err := rdeschema.Namespaces()
	require.NoError(t, err)
	assert.Contains(t, ns, "urn:ietf:params:xml:ns:rde-1.0")
	assert.Contains(t, ns, "urn:ietf:params:xml:ns:secDNS-1.1")
}

func TestEngine_Conclusions(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	cases := []struct {
		name string
		doc  string
		want rdeschema.Conclusion
	}{
		{"valid", validDoc(), rdeschema.ConclusionValid},
		{"invalid", strings.Replace(validDoc(), "<rde:watermark>", "<rde:nope/><rde:watermark>", 1), rdeschema.ConclusionInvalid},
		{"truncated", validDoc()[:700], rdeschema.ConclusionNotWellFormed},
		{"trailing content after the root", validDoc() + "<extra/>", rdeschema.ConclusionNotWellFormed},
		{"mismatched tags", strings.Replace(validDoc(), "</rdeHost:name>", "</rdeHost:nome>", 1), rdeschema.ConclusionNotWellFormed},
		{"empty", "", rdeschema.ConclusionNotWellFormed},
		{"doctype", `<!DOCTYPE a><a/>`, rdeschema.ConclusionRejected},
		{"utf-16 signature", "\xFF\xFE<\x00a\x00/\x00>\x00", rdeschema.ConclusionRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vd := verdictFor(t, eng, tc.doc)
			assert.Equal(t, tc.want, vd.Conclusion, "verdict: %+v", vd)
		})
	}
}

// The valid result must come from the exit status and nothing else: xmllint
// prints "validates" for a document it then fails to parse (observed on the
// libxml2 that ships with macOS), so the text is never evidence.
func TestEngine_ValidIsNotInferredFromOutputText(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	vd := verdictFor(t, eng, validDoc()+"<trailing/>")
	assert.NotEqual(t, rdeschema.ConclusionValid, vd.Conclusion)
}

func TestEngine_ChunkedWritesGiveTheSameVerdict(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	doc := strings.Replace(validDoc(), "<rde:watermark>", "<rde:nope/><rde:watermark>", 1)
	whole := verdictFor(t, eng, doc)
	for _, size := range []int{1, 7, 64, 4096} {
		s, err := eng.Start(context.Background())
		require.NoError(t, err)
		for i := 0; i < len(doc); i += size {
			end := min(i+size, len(doc))
			_, _ = s.Write([]byte(doc[i:end]))
		}
		vd := s.Finish()
		assert.Equal(t, whole.Conclusion, vd.Conclusion, "chunk size %d", size)
		assert.Equal(t, whole.Total, vd.Total)
	}
}

// A document with a DTD is refused on its first bytes: the entity it names is
// never handed to a process that would resolve it, even when the document goes
// on for a long time afterwards.
func TestEngine_DocTypeIsRefusedBeforeAnythingIsForwarded(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	s, err := eng.Start(context.Background())
	require.NoError(t, err)
	_, _ = s.Write([]byte(`<?xml version="1.0"?><!DOCTYPE a [ <!ENTITY x SYSTEM "file:///etc/hosts"> ]>` + "\n<a>&x;</a>"))
	_, _ = s.Write([]byte(strings.Repeat("<b/>", 10000)))
	vd := s.Finish()
	assert.Equal(t, rdeschema.ConclusionRejected, vd.Conclusion)
	assert.Equal(t, rdeschema.RejectDocType, vd.Reason)
}

func TestEngine_AbortStillReportsARefusal(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	s, err := eng.Start(context.Background())
	require.NoError(t, err)
	_, _ = s.Write([]byte(`<!DOCTYPE a><a/>`))
	vd := s.Abort()
	assert.Equal(t, rdeschema.ConclusionRejected, vd.Conclusion)

	s, err = eng.Start(context.Background())
	require.NoError(t, err)
	_, _ = s.Write([]byte(validDoc()[:300]))
	vd = s.Abort()
	assert.Equal(t, rdeschema.ConclusionUnavailable, vd.Conclusion, "an abandoned prefix is never judged")
}

func TestEngine_CountsAreBoundedAndCheckingStops(t *testing.T) {
	eng := rdeschema.NewXMLLint(rdeschema.Config{MaxRetained: 5, StopAfter: 12})
	require.NoError(t, eng.Err())
	t.Cleanup(func() { _ = eng.Close() })

	var b strings.Builder
	b.WriteString(`<rde:deposit xmlns:rde="urn:ietf:params:xml:ns:rde-1.0" xmlns:rdeIDN="urn:ietf:params:xml:ns:rdeIDN-1.0" type="FULL" id="1"><rde:watermark>2026-01-01T00:00:00Z</rde:watermark><rde:rdeMenu><rde:version>1.0</rde:version><rde:objURI>urn:x</rde:objURI></rde:rdeMenu><rde:contents>`)
	for i := 0; i < 200; i++ {
		// libxml2 reports a broken content model once per parent, so the
		// violations must be in separate elements to be counted separately.
		b.WriteString(`<rdeIDN:idnTableRef><rdeIDN:url>u</rdeIDN:url><rdeIDN:urlPolicy>p</rdeIDN:urlPolicy></rdeIDN:idnTableRef>`)
	}
	b.WriteString(`</rde:contents></rde:deposit>`)
	vd := verdictFor(t, eng, b.String())

	assert.Equal(t, rdeschema.ConclusionInvalid, vd.Conclusion)
	assert.Len(t, vd.Violations, 5)
	assert.True(t, vd.Stopped)
	assert.Equal(t, 12, vd.Total, "checking stopped at the bound rather than counting every violation")
}

func TestEngine_VeryLongValueIsNotBufferedOrEchoed(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	doc := strings.Replace(validDoc(), "<rde:version>1.0</rde:version>", "<rde:version>"+strings.Repeat("A", 300000)+"</rde:version>", 1)
	vd := verdictFor(t, eng, doc)
	assert.Equal(t, rdeschema.ConclusionInvalid, vd.Conclusion)
	require.NotEmpty(t, vd.Violations)
	for _, v := range vd.Violations {
		assert.NotContains(t, v.Element.Text+v.Attribute.Text+v.Type+v.Facet, "AAAA")
	}
}

func TestEngine_ConcurrentSessions(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	var wg sync.WaitGroup
	bad := strings.Replace(validDoc(), "<rde:watermark>", "<rde:nope/><rde:watermark>", 1)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			doc, want := validDoc(), rdeschema.ConclusionValid
			if i%2 == 1 {
				doc, want = bad, rdeschema.ConclusionInvalid
			}
			assert.Equal(t, want, verdictFor(t, eng, doc).Conclusion)
		}(i)
	}
	wg.Wait()
}

func TestEngine_CancellationEndsTheProcess(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := eng.Start(ctx)
	require.NoError(t, err)
	_, _ = s.Write([]byte(validDoc()[:200]))
	cancel()
	done := make(chan rdeschema.Verdict, 1)
	go func() { done <- s.Finish() }()
	select {
	case vd := <-done:
		assert.Equal(t, rdeschema.ConclusionUnavailable, vd.Conclusion)
	case <-time.After(20 * time.Second):
		t.Fatal("a cancelled check did not end")
	}
}

func TestEngine_UnavailableEnginesNeverStart(t *testing.T) {
	for name, cfg := range map[string]rdeschema.Config{
		"missing binary":          {Binary: "/nonexistent/xmllint"},
		"binary that accepts all": {Binary: "true"},
		"binary that rejects all": {Binary: "false"},
	} {
		t.Run(name, func(t *testing.T) {
			eng := rdeschema.NewXMLLint(cfg)
			t.Cleanup(func() { _ = eng.Close() })
			require.Error(t, eng.Err())
			_, err := eng.Start(context.Background())
			require.Error(t, err)
			assert.True(t, errors.Is(err, rdeschema.ErrUnavailable))
			assert.Equal(t, "unavailable", eng.Description())
		})
	}
}
