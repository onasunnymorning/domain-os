package rdevalidate

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"strings"
	"time"
)

// Layout is the payload shape detected inside the decrypted deposit.
type Layout string

const (
	LayoutTar  Layout = "tar"
	LayoutGzip Layout = "gzip"
	LayoutXML  Layout = "xml"
)

// errBudgetExceeded is returned by budgetReader once the cumulative unpacked
// byte budget is spent. It is sticky: every subsequent Read fails too.
var errBudgetExceeded = errors.New("rdevalidate: unpacked byte budget exceeded")

// budget is the shared cumulative plaintext allowance for one run. Every
// decompression layer's *output* is charged against it, so a small archive
// that inflates enormously (a bomb) is stopped at the budget regardless of
// how many layers it hides behind.
type budget struct {
	remaining int64
	exceeded  bool
}

type budgetReader struct {
	r io.Reader
	b *budget
}

func (br *budgetReader) Read(p []byte) (int, error) {
	if br.b.exceeded {
		return 0, errBudgetExceeded
	}
	n, err := br.r.Read(p)
	br.b.remaining -= int64(n)
	if br.b.remaining < 0 {
		br.b.exceeded = true
		return n, errBudgetExceeded
	}
	return n, err
}

// XMLEntry is the single XML payload found in the deposit, exposed as a
// stream so nothing is ever written to disk.
type XMLEntry struct {
	Reader io.Reader
	Index  int    // tar entry index (1-based) or 0 for raw/gzip layouts
	Layout Layout // outermost layout

	drain func() *Finding
}

// Drain consumes everything after the XML payload — the rest of the XML
// entry and every remaining archive entry — applying the same entry rules and
// limits, so that a second payload or an unsafe trailing entry is still
// detected even though the XML has already been validated. It must be called
// after the XML has been read.
func (e *XMLEntry) Drain() *Finding {
	if e.drain == nil {
		return nil
	}
	return e.drain()
}

// SafeUnpack detects the payload layout by magic bytes and returns the one XML
// entry as a stream. Accepted layouts: raw XML; gzip-compressed XML; a tar
// archive (optionally gzip-compressed) holding exactly one XML file, which may
// itself be gzip-compressed. Anything else fails closed.
//
// Tar rules (design constraint "safe unpack"): regular files only; no absolute
// paths, no `..` segments, no NUL in names; entry names are never retained —
// findings locate entries by index. Every decompression layer's output is
// charged to the shared budget.
func SafeUnpack(plaintext io.Reader, lim Limits, b *budget, now time.Time) (*XMLEntry, *Finding) {
	fail := func(code Code, locator, msg string) (*XMLEntry, *Finding) {
		return nil, &Finding{Code: code, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive", Locator: locator, Message: msg, At: now}
	}

	layers := 0
	outer := Layout("")
	cur := bufio.NewReaderSize(&budgetReader{r: plaintext, b: b}, 64<<10)

	for {
		head, err := cur.Peek(512)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
			if errors.Is(err, errBudgetExceeded) {
				return fail(CodeArchiveLimitUnpackedSize, "", "unpacked size limit exceeded while reading the payload")
			}
			return fail(CodeArchiveUnsupportedLayout, "", "payload could not be read")
		}
		switch {
		case isGzip(head):
			layers++
			if outer == "" {
				outer = LayoutGzip
			}
			if layers > lim.MaxNesting {
				return fail(CodeArchiveLimitNesting, "", "decompression nesting exceeds limit ("+itoa(lim.MaxNesting)+")")
			}
			gz, err := gzip.NewReader(cur)
			if err != nil {
				return fail(CodeArchiveUnsupportedLayout, "", "gzip stream could not be opened")
			}
			cur = bufio.NewReaderSize(&budgetReader{r: gz, b: b}, 64<<10)
			continue
		case isTar(head):
			if outer == "" {
				outer = LayoutTar
			}
			return unpackTar(cur, lim, b, layers, outer, now)
		case isXML(head):
			if outer == "" {
				outer = LayoutXML
			}
			return &XMLEntry{
				Reader: cur, Index: 0, Layout: outer,
				drain: func() *Finding { return discard(cur, now) },
			}, nil
		default:
			return fail(CodeArchiveUnsupportedLayout, "", "payload is not XML, gzip or tar")
		}
	}
}

func unpackTar(r io.Reader, lim Limits, b *budget, layers int, outer Layout, now time.Time) (*XMLEntry, *Finding) {
	tr := tar.NewReader(r)
	count := 0
	fail := func(code Code, msg string) (*XMLEntry, *Finding) {
		return nil, &Finding{Code: code, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive-entry", Locator: "entry[" + itoa(count) + "]", Message: msg, At: now}
	}

	for {
		hdr, err := tr.Next()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				return fail(CodeArchiveNoXML, "archive contains no XML payload")
			case errors.Is(err, errBudgetExceeded):
				return fail(CodeArchiveLimitUnpackedSize, "unpacked size limit exceeded while reading the archive")
			default:
				return fail(CodeArchiveUnsupportedLayout, "archive is truncated or malformed")
			}
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		count++
		if count > lim.MaxFiles {
			return fail(CodeArchiveLimitFileCount, "archive entry count exceeds limit ("+itoa(lim.MaxFiles)+")")
		}
		if code, msg := checkEntry(hdr, b); code != "" {
			return fail(code, msg)
		}

		content := bufio.NewReaderSize(tr, 64<<10)
		head, _ := content.Peek(512)
		var payload io.Reader = content
		switch {
		case isGzip(head):
			if layers+1 > lim.MaxNesting {
				return fail(CodeArchiveLimitNesting, "decompression nesting exceeds limit ("+itoa(lim.MaxNesting)+")")
			}
			gz, err := gzip.NewReader(content)
			if err != nil {
				return fail(CodeArchiveUnsupportedLayout, "entry gzip stream could not be opened")
			}
			inner := bufio.NewReaderSize(&budgetReader{r: gz, b: b}, 64<<10)
			ih, _ := inner.Peek(512)
			if !isXML(ih) {
				return fail(CodeArchiveUnsupportedLayout, "compressed entry is not XML")
			}
			payload = inner
		case isXML(head):
		default:
			return fail(CodeArchiveUnsupportedLayout, "entry is not an XML payload")
		}

		idx := count
		entry := &XMLEntry{Reader: payload, Index: idx, Layout: outer}
		entry.drain = func() *Finding {
			if f := discard(payload, now); f != nil {
				return f
			}
			return drainTar(tr, lim, b, &count, now)
		}
		return entry, nil
	}
}

// drainTar walks the remaining entries after the XML payload. A second
// payload-shaped entry or any unsafe entry still fails the run.
func drainTar(tr *tar.Reader, lim Limits, b *budget, count *int, now time.Time) *Finding {
	fail := func(code Code, msg string) *Finding {
		return &Finding{Code: code, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive-entry", Locator: "entry[" + itoa(*count) + "]", Message: msg, At: now}
	}
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, errBudgetExceeded) {
				return fail(CodeArchiveLimitUnpackedSize, "unpacked size limit exceeded while reading the archive")
			}
			return fail(CodeArchiveUnsupportedLayout, "archive is truncated or malformed after the payload")
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		*count++
		if *count > lim.MaxFiles {
			return fail(CodeArchiveLimitFileCount, "archive entry count exceeds limit ("+itoa(lim.MaxFiles)+")")
		}
		if code, msg := checkEntry(hdr, b); code != "" {
			return fail(code, msg)
		}
		// Stage 1 accepts exactly one XML payload (user decision 4).
		content := bufio.NewReader(tr)
		head, _ := content.Peek(512)
		if isXML(head) || isGzip(head) {
			return fail(CodeArchiveUnsupportedLayout, "archive holds more than one payload; split deposits are not supported")
		}
		return fail(CodeArchiveUnsupportedLayout, "archive holds an unexpected additional entry")
	}
}

// checkEntry applies the safety rules to one tar header. It returns a code
// and message, or ("", "") if the entry is acceptable. Names are inspected
// but never copied into the message.
func checkEntry(hdr *tar.Header, b *budget) (Code, string) {
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck // TypeRegA is still emitted by old tar writers
	default:
		return CodeArchiveUnsafeEntry, "entry is not a regular file (directory, link or device)"
	}
	name := hdr.Name
	if name == "" || strings.ContainsRune(name, 0) {
		return CodeArchiveUnsafeEntry, "entry has an empty or NUL-containing name"
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || (len(name) > 1 && name[1] == ':') {
		return CodeArchiveUnsafeEntry, "entry has an absolute path"
	}
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return CodeArchiveUnsafeEntry, "entry path traverses upward"
		}
	}
	if hdr.Size < 0 {
		return CodeArchiveUnsafeEntry, "entry declares a negative size"
	}
	if hdr.Size > b.remaining {
		return CodeArchiveLimitUnpackedSize, "entry declared size (" + i64toa(hdr.Size) + " bytes) exceeds the remaining unpacked budget"
	}
	return "", ""
}

func discard(r io.Reader, now time.Time) *Finding {
	if _, err := io.Copy(io.Discard, r); err != nil {
		if errors.Is(err, errBudgetExceeded) {
			return &Finding{Code: CodeArchiveLimitUnpackedSize, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive", Message: "unpacked size limit exceeded while draining the payload", At: now}
		}
		return &Finding{Code: CodeArchiveUnsupportedLayout, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive", Message: "payload is truncated or malformed after the XML", At: now}
	}
	return nil
}

func isGzip(head []byte) bool {
	return len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b
}

func isTar(head []byte) bool {
	return len(head) >= 263 && string(head[257:262]) == "ustar"
}

func isXML(head []byte) bool {
	h := bytes.TrimPrefix(head, []byte{0xEF, 0xBB, 0xBF})
	h = bytes.TrimLeft(h, " \t\r\n")
	return len(h) > 0 && h[0] == '<'
}
