package rdeschema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanProlog(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		eof    bool
		status prologStatus
		reason RejectReason
	}{
		{"declaration then root", `<?xml version="1.0" encoding="UTF-8"?><rde:deposit>`, false, prologOK, ""},
		{"bare root", `<rde:deposit>`, false, prologOK, ""},
		{"utf-8 bom", "\xEF\xBB\xBF<?xml version=\"1.0\"?><a>", false, prologOK, ""},
		{"comments and whitespace", "<?xml version=\"1.0\"?>\n<!-- a -->\n<!-- b -->\n<a>", false, prologOK, ""},
		{"lowercase utf-8 label", `<?xml version='1.0' encoding='utf-8'?><a>`, false, prologOK, ""},
		{"encoding label with spaces around =", `<?xml version="1.0" encoding = "UTF-8" ?><a>`, false, prologOK, ""},

		{"internal subset", `<?xml version="1.0"?><!DOCTYPE a [ <!ENTITY e "x"> ]><a>&e;</a>`, false, prologRejected, RejectDocType},
		{"external subset", `<!DOCTYPE a SYSTEM "http://evil/a.dtd"><a/>`, false, prologRejected, RejectDocType},
		{"external entity", `<!DOCTYPE a [ <!ENTITY x SYSTEM "file:///etc/passwd"> ]><a>&x;</a>`, false, prologRejected, RejectDocType},
		{"doctype after a comment", `<!-- hide --><!DOCTYPE a><a/>`, false, prologRejected, RejectDocType},
		{"lowercase doctype is still refused", `<!doctype a><a/>`, false, prologRejected, RejectDocType},
		{"CDATA before the root is not XML", `<![CDATA[x]]><a/>`, false, prologRejected, RejectDocType},

		{"utf-16 le bom", "\xFF\xFE<\x00a\x00>\x00", false, prologRejected, RejectProlog},
		{"utf-16 be bom", "\xFE\xFF\x00<\x00a\x00>", false, prologRejected, RejectProlog},
		{"utf-16 le without bom", "<\x00?\x00x\x00m\x00l\x00", false, prologRejected, RejectProlog},
		{"utf-16 be without bom", "\x00<\x00?\x00x\x00m\x00l", false, prologRejected, RejectProlog},
		{"utf-32", "\x00\x00\x00<\x00\x00\x00?", false, prologRejected, RejectProlog},
		{"ebcdic", "\x4C\x6F\xA7\x94\x93", false, prologRejected, RejectProlog},
		{"declared utf-16", `<?xml version="1.0" encoding="UTF-16"?><a/>`, false, prologRejected, RejectProlog},
		{"declared latin-1", `<?xml version="1.0" encoding="ISO-8859-1"?><a/>`, false, prologRejected, RejectProlog},
		{"not xml", `hello`, false, prologRejected, RejectProlog},
		{"a prolog that never ends", "<!--" + strings.Repeat("x", maxPrologBytes+10), false, prologRejected, RejectProlog},

		{"undecided: only the start", `<?xml ver`, false, prologNeedMore, ""},
		{"undecided: lone <", `<`, false, prologNeedMore, ""},
		{"undecided: partial doctype", `<!DO`, false, prologRejected, RejectDocType},
		{"ended inside the prolog", `<?xml ver`, true, prologOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, reason := scanProlog([]byte(tc.in), tc.eof)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.reason, reason)
		})
	}
}

// The decision must not depend on how the stream happens to be chunked.
func TestScanProlog_ChunkingDoesNotChangeTheDecision(t *testing.T) {
	docs := []string{
		`<?xml version="1.0" encoding="UTF-8"?><!-- c --><rde:deposit xmlns:rde="x">`,
		`<?xml version="1.0"?><!DOCTYPE a [ <!ENTITY e "x"> ]><a>&e;</a>`,
		`<?xml version="1.0" encoding="UTF-16"?><a/>`,
	}
	for _, d := range docs {
		whole, wr := scanProlog([]byte(d), false)
		for size := 1; size <= 7; size++ {
			var buf []byte
			var got prologStatus
			var why RejectReason
			for i := 0; i < len(d); i += size {
				end := i + size
				if end > len(d) {
					end = len(d)
				}
				buf = append(buf, d[i:end]...)
				got, why = scanProlog(buf, false)
				if got != prologNeedMore {
					break
				}
			}
			assert.Equal(t, whole, got, "doc %q chunk %d", d, size)
			assert.Equal(t, wr, why)
		}
	}
}
