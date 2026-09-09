package rdevalidate

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unpack(t *testing.T, payload []byte, lim Limits) (*XMLEntry, *Finding, *budget) {
	t.Helper()
	b := &budget{remaining: lim.MaxUnpackedBytes}
	e, f := SafeUnpack(bytes.NewReader(payload), lim, b, testNow)
	return e, f, b
}

func TestSafeUnpack_Layouts(t *testing.T) {
	lim := DefaultLimits()
	xml := rdetest.BuildXML(rdetest.DepositOpts{})
	cases := []struct {
		name   string
		opts   rdetest.DepositOpts
		layout Layout
		index  int
	}{
		{"raw xml", rdetest.DepositOpts{Layout: rdetest.LayoutXML}, LayoutXML, 0},
		{"gzip xml", rdetest.DepositOpts{Layout: rdetest.LayoutGzip}, LayoutGzip, 0},
		{"tar", rdetest.DepositOpts{Layout: rdetest.LayoutTar}, LayoutTar, 1},
		{"tar with gzipped entry", rdetest.DepositOpts{Layout: rdetest.LayoutTar, EntryGzip: true}, LayoutTar, 1},
		{"gzip(tar)", rdetest.DepositOpts{Layout: rdetest.LayoutTar, GzipWraps: 1}, LayoutGzip, 1},
		{"tar with a harmless leading entry", rdetest.DepositOpts{Layout: rdetest.LayoutTar, Before: []rdetest.TarEntry{{Name: "README", Type: tar.TypeReg, Body: []byte("hello")}}}, LayoutTar, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := rdetest.BuildPayload(t, c.opts, xml)
			e, f, _ := unpack(t, payload, lim)
			if c.name == "tar with a harmless leading entry" {
				// a non-XML regular file before the payload is an unexpected layout
				require.NotNil(t, f)
				assert.Equal(t, CodeArchiveUnsupportedLayout, f.Code)
				return
			}
			require.Nil(t, f)
			assert.Equal(t, c.layout, e.Layout)
			assert.Equal(t, c.index, e.Index)
			got, err := io.ReadAll(e.Reader)
			require.NoError(t, err)
			assert.Equal(t, xml, got)
			assert.Nil(t, e.Drain())
		})
	}
}

func TestSafeUnpack_Rejections(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxFiles = 3
	lim.MaxNesting = 2
	xml := rdetest.BuildXML(rdetest.DepositOpts{})
	after := func(es ...rdetest.TarEntry) rdetest.DepositOpts {
		return rdetest.DepositOpts{Layout: rdetest.LayoutTar, After: es}
	}
	cases := []struct {
		name    string
		payload []byte
		code    Code
		onDrain bool
	}{
		{"not xml, gzip or tar", []byte("PK\x03\x04 zip?"), CodeArchiveUnsupportedLayout, false},
		{"empty", nil, CodeArchiveUnsupportedLayout, false},
		{"empty tar (no ustar magic)", func() []byte { var b bytes.Buffer; tw := tar.NewWriter(&b); _ = tw.Close(); return b.Bytes() }(), CodeArchiveUnsupportedLayout, false},
		{"symlink entry", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, EntryType: tar.TypeSymlink, EntryName: "deposit.xml"}, xml), CodeArchiveUnsafeEntry, false},
		{"directory before payload", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, Before: []rdetest.TarEntry{{Name: "dir/", Type: tar.TypeDir}}}, xml), CodeArchiveUnsafeEntry, false},
		{"path traversal", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, EntryName: "../../etc/deposit.xml"}, xml), CodeArchiveUnsafeEntry, false},
		{"absolute path", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, EntryName: "/tmp/deposit.xml"}, xml), CodeArchiveUnsafeEntry, false},
		{"second xml payload", rdetest.BuildPayload(t, after(rdetest.TarEntry{Name: "part2.xml", Type: tar.TypeReg, Body: xml}), xml), CodeArchiveUnsupportedLayout, true},
		{"unexpected trailing entry", rdetest.BuildPayload(t, after(rdetest.TarEntry{Name: "notes.txt", Type: tar.TypeReg, Body: []byte("x")}), xml), CodeArchiveUnsupportedLayout, true},
		{"trailing traversal entry", rdetest.BuildPayload(t, after(rdetest.TarEntry{Name: "../x", Type: tar.TypeReg, Body: []byte("x")}), xml), CodeArchiveUnsafeEntry, true},
		{"nesting too deep", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, EntryGzip: true, GzipWraps: 2}, xml), CodeArchiveLimitNesting, false},
		{"non-xml entry", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar}, []byte("just text")), CodeArchiveUnsupportedLayout, false},
		{"truncated tar header", rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar}, xml)[:300], CodeArchiveUnsupportedLayout, false},
		{"truncated tar body", func() []byte {
			p := rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar}, xml)
			return p[:len(p)-2048]
		}(), CodeArchiveUnsupportedLayout, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, f, _ := unpack(t, c.payload, lim)
			if !c.onDrain {
				require.NotNil(t, f, "expected a finding at unpack time")
				assert.Equal(t, c.code, f.Code)
				assert.NotContains(t, f.Message, "etc", "entry names never leak into messages")
				return
			}
			require.Nil(t, f)
			_, _ = io.ReadAll(e.Reader)
			df := e.Drain()
			require.NotNil(t, df, "expected a finding while draining")
			assert.Equal(t, c.code, df.Code)
		})
	}
}

func TestSafeUnpack_Budget(t *testing.T) {
	xml := rdetest.BuildXML(rdetest.DepositOpts{Domains: 50, Contacts: 50, Hosts: 50, Registrars: 1})
	lim := DefaultLimits()
	lim.MaxUnpackedBytes = int64(len(xml) / 2)

	t.Run("gzip bomb-shaped payload stops at the budget", func(t *testing.T) {
		e, f, _ := unpack(t, rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutGzip}, xml), lim)
		require.Nil(t, f)
		_, err := io.ReadAll(e.Reader)
		require.ErrorIs(t, err, errBudgetExceeded)
	})
	t.Run("tar entry larger than the remaining budget is rejected up front", func(t *testing.T) {
		_, f, _ := unpack(t, rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar}, xml), lim)
		require.NotNil(t, f)
		assert.Equal(t, CodeArchiveLimitUnpackedSize, f.Code)
	})
	t.Run("budget is sticky", func(t *testing.T) {
		b := &budget{remaining: 3}
		r := &budgetReader{r: bytes.NewReader([]byte("abcdef")), b: b}
		_, err := io.ReadAll(r)
		require.ErrorIs(t, err, errBudgetExceeded)
		_, err = r.Read(make([]byte, 1))
		require.ErrorIs(t, err, errBudgetExceeded)
	})
}

func TestSafeUnpack_FileCountLimit(t *testing.T) {
	// The entry-count check runs before any content check, so it is the
	// finding that fires when an archive carries more members than allowed.
	lim := DefaultLimits()
	lim.MaxFiles = 1
	xml := rdetest.BuildXML(rdetest.DepositOpts{})
	payload := rdetest.BuildPayload(t, rdetest.DepositOpts{Layout: rdetest.LayoutTar, After: []rdetest.TarEntry{{Name: "extra", Type: tar.TypeReg, Body: []byte("x")}}}, xml)
	e, f, _ := unpack(t, payload, lim)
	require.Nil(t, f)
	_, _ = io.ReadAll(e.Reader)
	df := e.Drain()
	require.NotNil(t, df)
	assert.Equal(t, CodeArchiveLimitFileCount, df.Code)
}
