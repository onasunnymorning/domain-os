package rdeschema

import (
	"bytes"
)

// RejectReason says why the engine refused to hand a document to libxml2.
type RejectReason string

const (
	// RejectDocType: the prolog carries a document type declaration. xmllint
	// resolves external entities and external subsets declared there — from
	// the local filesystem as well as, unless --nonet is given, the network —
	// and has no option that turns that off. A DTD is the only way a
	// well-formed document can name an entity, so refusing the declaration
	// refuses every route to it.
	RejectDocType RejectReason = "doctype"
	// RejectProlog: the document does not start as UTF-8 XML the engine can
	// inspect — a UTF-16/32 signature, a declared encoding other than UTF-8,
	// text that is not XML at all, or a prolog too long to be one. Without
	// being able to read the prolog the DOCTYPE check above proves nothing.
	RejectProlog RejectReason = "prolog"
)

// maxPrologBytes bounds how much of a document is buffered before the root
// element starts. A real prolog is an XML declaration and, at most, a few
// comments.
const maxPrologBytes = 64 << 10

type prologStatus int

const (
	prologNeedMore prologStatus = iota
	prologOK
	prologRejected
)

// scanProlog decides whether the bytes so far are a prolog libxml2 may be
// given: UTF-8, no document type declaration. It is a decision, not a parse —
// anything it cannot account for is refused.
//
// It sees only the prolog; once the root element has started it returns
// prologOK and stops looking, because a document type declaration after the
// root start tag is not well-formed and cannot declare anything.
func scanProlog(buf []byte, atEOF bool) (prologStatus, RejectReason) {
	need := func() (prologStatus, RejectReason) {
		if atEOF {
			// The document ended inside its prolog. There is no root and no
			// entity reference, so nothing here can load anything; libxml2
			// will say the document is empty or truncated.
			return prologOK, ""
		}
		if len(buf) > maxPrologBytes {
			return prologRejected, RejectProlog
		}
		return prologNeedMore, ""
	}

	i := 0
	// UTF-16 and UTF-32 signatures are refused before anything else is read
	// as ASCII. XML Appendix F: a BOM, or "<" in 16/32-bit form, or EBCDIC.
	for _, sig := range [][]byte{
		{0xFE, 0xFF}, {0xFF, 0xFE}, {0x00, 0x00, 0xFE, 0xFF}, {0xFF, 0xFE, 0x00, 0x00},
		{0x00, 0x3C}, {0x3C, 0x00}, {0x00, 0x00, 0x00, 0x3C}, {0x3C, 0x00, 0x00, 0x00},
		{0x4C, 0x6F, 0xA7, 0x94},
	} {
		if bytes.HasPrefix(buf, sig) {
			return prologRejected, RejectProlog
		}
	}
	if len(buf) < 4 {
		for _, sig := range [][]byte{{0xFE}, {0xFF}, {0x00}, {0x3C, 0x00}, {0x4C}} {
			if bytes.HasPrefix(buf, sig) && !atEOF {
				return need()
			}
		}
	}
	if bytes.HasPrefix(buf, []byte{0xEF, 0xBB, 0xBF}) {
		i = 3
	}

	for {
		for i < len(buf) && isXMLSpace(buf[i]) {
			i++
		}
		if i >= len(buf) {
			return need()
		}
		if buf[i] != '<' {
			// Not markup: either not XML or not UTF-8 text. Refuse rather
			// than pass libxml2 something whose encoding was guessed.
			return prologRejected, RejectProlog
		}
		rest := buf[i:]
		switch {
		case bytes.HasPrefix(rest, []byte("<?")):
			end := bytes.Index(rest, []byte("?>"))
			if end < 0 {
				return need()
			}
			if isXMLDecl(rest) {
				if enc, found := declaredEncoding(rest[:end]); found && !isUTF8Label(enc) {
					return prologRejected, RejectProlog
				}
			}
			i += end + 2
		case bytes.HasPrefix(rest, []byte("<!--")):
			end := bytes.Index(rest[4:], []byte("-->"))
			if end < 0 {
				return need()
			}
			i += 4 + end + 3
		case bytes.HasPrefix(rest, []byte("<!")):
			if len(rest) < 4 && !atEOF {
				return need()
			}
			// "<!" that is not a comment in the prolog is a document type
			// declaration, or something no parser would accept.
			return prologRejected, RejectDocType
		case len(rest) < 2:
			return need()
		case isNameStart(rest[1]):
			return prologOK, ""
		default:
			// "<" followed by something that cannot start a name. Malformed;
			// the parsers will say so, and it cannot declare an entity.
			return prologOK, ""
		}
		if i > maxPrologBytes {
			return prologRejected, RejectProlog
		}
	}
}

func isXMLSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// isNameStart reports whether b can begin an XML name. Any byte >= 0x80 is the
// start of a multi-byte UTF-8 name character.
func isNameStart(b byte) bool {
	return b == '_' || b == ':' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= 0x80
}

// isXMLDecl reports whether the processing instruction at the start of rest is
// an XML declaration (<?xml followed by whitespace), as opposed to some other
// PI whose target merely begins with "xml".
func isXMLDecl(rest []byte) bool {
	return len(rest) > 5 && bytes.HasPrefix(rest, []byte("<?xml")) && isXMLSpace(rest[5])
}

// declaredEncoding extracts the encoding pseudo-attribute from the text of an
// XML declaration (everything before the closing "?>").
func declaredEncoding(decl []byte) (value string, found bool) {
	i := bytes.Index(decl, []byte("encoding"))
	if i < 0 {
		return "", false
	}
	j := i + len("encoding")
	for j < len(decl) && isXMLSpace(decl[j]) {
		j++
	}
	if j >= len(decl) || decl[j] != '=' {
		return "", false
	}
	j++
	for j < len(decl) && isXMLSpace(decl[j]) {
		j++
	}
	if j >= len(decl) || (decl[j] != '"' && decl[j] != '\'') {
		return "", false
	}
	q := decl[j]
	j++
	k := bytes.IndexByte(decl[j:], q)
	if k < 0 {
		return "", false
	}
	return string(decl[j : j+k]), true
}

func isUTF8Label(s string) bool {
	return len(s) == 5 && (s[0]|0x20) == 'u' && (s[1]|0x20) == 't' && (s[2]|0x20) == 'f' && s[3] == '-' && s[4] == '8'
}
