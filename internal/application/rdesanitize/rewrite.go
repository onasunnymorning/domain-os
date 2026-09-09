package rdesanitize

import (
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"
)

// XSIURI carries schema hints rather than deposit data. Its attributes are
// recognised and dropped: keeping a schemaLocation that points at the source
// registry's schemas would be misleading on a derivative.
const XSIURI = "http://www.w3.org/2001/XMLSchema-instance"

// xmlDeclaration is emitted in place of whatever the source declared. The
// derivative is always UTF-8.
const xmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

const heartbeatEvery = 5000

// Rewriter streams an RDE deposit through the profile and writes the
// derivative. It never buffers the document: only one element's text is held at
// a time, bounded by Limits.MaxFieldBytes.
//
// It is driven by xml.Decoder.RawToken rather than Token because RawToken
// leaves namespace prefixes exactly as the source wrote them. Go's encoder
// does not round-trip a namespaced document — it rewrites prefixes into its own
// xmlns attributes — so re-encoding through it would produce a derivative whose
// element names differ from the deposit it claims to mirror. Prefix resolution
// is therefore done here, against an explicit scope stack.
type Rewriter struct {
	Profile   *Profile
	Tokens    *Tokenizer
	Suffix    *SuffixRewriter
	Limits    Limits
	Now       func() time.Time
	Heartbeat func(stage Stage, detail string)
}

type rewriteState struct {
	rw  *Rewriter
	res *Result
	w   *bufio.Writer

	scopes  []map[string]string // prefix -> namespace URI, one frame per open element
	keys    []string            // profile key per open element
	names   []xml.Name          // raw names per open element, to detect a desynchronised document
	actions []Action            // action per open element, restored on close
	elems   int64
	pending []byte // an unflushed start tag, so `<x/>` stays `<x/>`
	value   *strings.Builder
	action  Action
	// skipDepth > 0 means we are inside a dropped subtree; copyDepth > 0 means
	// we are inside a keep-subtree block that is copied without classification.
	skipDepth int
	copyDepth int
}

// Rewrite reads the deposit from src and writes the derivative to dst. It never
// panics: a failure becomes a Finding and Decide picks the outcome. When the
// outcome is not PASS the caller must discard whatever was written — a partial
// derivative is not a derivative.
func (rw *Rewriter) Rewrite(ctx context.Context, src io.Reader, dst io.Writer) (res Result) {
	now := rw.Now
	if now == nil {
		now = time.Now
	}
	res = Result{StageReached: StageRewrite, Findings: []Finding{}, StartedAt: now()}
	defer func() {
		if p := recover(); p != nil {
			res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageRewrite, Message: "sanitization pipeline panicked", At: now()})
		}
		res.finish(now())
	}()
	if err := rw.Limits.Validate(); err != nil {
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageRewrite, Message: "resource limits are misconfigured", At: now()})
		return res
	}
	if rw.Profile == nil || rw.Tokens == nil || rw.Suffix == nil {
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageRewrite, Message: "sanitizer is not fully configured", At: now()})
		return res
	}

	st := &rewriteState{rw: rw, res: &res, w: bufio.NewWriterSize(dst, 64<<10), value: &strings.Builder{}}
	st.emitString(xmlDeclaration)
	dec := xml.NewDecoder(src)
	dec.Strict = true

	for {
		if st.elems%heartbeatEvery == 0 && ctx.Err() != nil {
			res.Add(Finding{Code: CodeSanitizeTimeout, Severity: SeverityError, Stage: StageRewrite, Message: "sanitization deadline reached", At: now()})
			return res
		}
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			st.decodeError(dec, err, now())
			return res
		}
		if !st.token(dec, tok, now()) {
			return res
		}
	}
	if len(st.keys) != 0 {
		res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite, Message: "document ended with unclosed elements", At: now()})
		return res
	}
	st.flushPending()
	if err := st.w.Flush(); err != nil {
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageRewrite, Message: "derivative could not be written", At: now()})
	}
	return res
}

func (s *rewriteState) decodeError(dec *xml.Decoder, err error, at time.Time) {
	var se *xml.SyntaxError
	switch {
	case errors.As(err, &se):
		s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite,
			Locator: "line=" + itoa(se.Line), Message: "source XML is malformed", At: at})
	case strings.Contains(err.Error(), "CharsetReader"):
		// encoding/xml reports a declared non-UTF-8 encoding this way. Naming it
		// beats a generic parse failure: the fix is to re-export as UTF-8.
		s.res.Add(Finding{Code: CodeXMLUnsupportedEncoding, Severity: SeverityError, Stage: StageRewrite,
			Message: "source declares an encoding other than UTF-8", At: at})
	default:
		_ = dec
		s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite,
			Message: "source XML could not be read", At: at})
	}
}

// token processes one raw token. It returns false to stop the run.
func (s *rewriteState) token(dec *xml.Decoder, tok xml.Token, at time.Time) bool {
	if s.skipDepth > 0 {
		return s.skipToken(tok, at)
	}
	if s.copyDepth > 0 {
		return s.copyToken(tok, at)
	}
	switch t := tok.(type) {
	case xml.StartElement:
		return s.startElement(t, at)
	case xml.EndElement:
		return s.endElement(t, at)
	case xml.CharData:
		return s.charData(t, at)
	case xml.Comment:
		// Comments carry no deposit data and may carry operator notes; the
		// derivative does not reproduce them.
		return true
	case xml.ProcInst:
		if strings.EqualFold(t.Target, "xml") {
			return s.checkDeclaration(string(t.Inst), at)
		}
		return true // a non-XML processing instruction is dropped, not copied
	case xml.Directive:
		// A DOCTYPE is refused outright. Go never resolves external entities or
		// internal-subset declarations, so this is defence in depth rather than
		// the only barrier — but a deposit that ships a DTD is not one we
		// transform silently.
		if s.isDoctype(t) {
			s.res.Add(Finding{Code: CodeXMLDTDPresent, Severity: SeverityError, Stage: StageRewrite,
				Message: "source carries a document type declaration", At: at})
			return false
		}
		s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite,
			Message: "source carries an unsupported XML directive", At: at})
		return false
	}
	return true
}

func (s *rewriteState) isDoctype(d xml.Directive) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(string(d))), "DOCTYPE")
}

func (s *rewriteState) checkDeclaration(inst string, at time.Time) bool {
	lower := strings.ToLower(inst)
	i := strings.Index(lower, "encoding")
	if i < 0 {
		return true
	}
	rest := lower[i+len("encoding"):]
	if j := strings.IndexAny(rest, "\"'"); j >= 0 {
		rest = rest[j+1:]
		if k := strings.IndexAny(rest, "\"'"); k >= 0 {
			enc := strings.TrimSpace(rest[:k])
			if enc != "" && enc != "utf-8" {
				s.res.Add(Finding{Code: CodeXMLUnsupportedEncoding, Severity: SeverityError, Stage: StageRewrite,
					Message: "source declares an encoding other than UTF-8", At: at})
				return false
			}
		}
	}
	return true
}

func (s *rewriteState) startElement(t xml.StartElement, at time.Time) bool {
	s.flushPending()
	s.elems++
	s.res.SourceElements = s.elems
	if s.elems > s.rw.Limits.MaxElements {
		s.res.Add(Finding{Code: CodeLimitElementCount, Severity: SeverityError, Stage: StageRewrite,
			Message: "source exceeds the element limit (" + i64toa(s.rw.Limits.MaxElements) + ")", At: at})
		return false
	}
	if len(s.keys) >= s.rw.Limits.MaxXMLDepth {
		s.res.Add(Finding{Code: CodeLimitXMLDepth, Severity: SeverityError, Stage: StageRewrite,
			Locator: s.locator(), Message: "source exceeds the nesting depth limit (" + itoa(s.rw.Limits.MaxXMLDepth) + ")", At: at})
		return false
	}
	if s.rw.Heartbeat != nil && s.elems%heartbeatEvery == 0 {
		s.rw.Heartbeat(StageRewrite, "rewriting element "+i64toa(s.elems))
	}

	scope := s.newScope(t.Attr)
	key, ok := s.key(scope, t.Name)
	if !ok {
		s.res.Add(Finding{Code: CodePolicyUnknownNamespace, Severity: SeverityError, Stage: StageRewrite,
			Locator: s.locator(), Message: "element is in a namespace the profile does not classify", At: at})
		return false
	}
	action, ok := s.rw.Profile.Action(s.parentKey(), key)
	if !ok {
		s.res.Add(Finding{Code: CodePolicyUnknownElement, Severity: SeverityError, Stage: StageRewrite,
			Locator: s.locator(), Message: "element is not classified by profile " + s.rw.Profile.Version(), At: at})
		return false
	}

	if action.Kind == ActDropSubtree {
		// The element and everything under it is removed. Nothing inside is
		// classified, emitted, buffered or counted beyond the drop tally: the
		// point of dropping authInfo is that its text never exists here.
		s.res.Counts.Dropped++
		s.skipDepth = 1
		return true
	}

	attrs, ok := s.attributes(scope, key, t.Attr, at)
	if !ok {
		return false
	}
	if uri, isObject := objectRoots[key]; isObject {
		if s.res.Counts.ObjectsByType == nil {
			s.res.Counts.ObjectsByType = map[string]int64{}
		}
		s.res.Counts.ObjectsByType[uri]++
	}
	s.push(scope, key, t.Name, action)
	s.action = action
	s.value.Reset()
	s.pending = s.startTag(t.Name, attrs)
	if action.Kind == ActKeep || action.Kind == ActKeepSubtree {
		s.res.Counts.Kept++
	}
	if action.Kind == ActKeepSubtree {
		s.copyDepth = 1
	}
	return true
}

// skipToken consumes a dropped subtree without classifying or emitting it.
func (s *rewriteState) skipToken(tok xml.Token, at time.Time) bool {
	switch tok.(type) {
	case xml.StartElement:
		s.elems++
		s.res.SourceElements = s.elems
		s.res.Counts.Dropped++
		if s.elems > s.rw.Limits.MaxElements {
			s.res.Add(Finding{Code: CodeLimitElementCount, Severity: SeverityError, Stage: StageRewrite,
				Message: "source exceeds the element limit (" + i64toa(s.rw.Limits.MaxElements) + ")", At: at})
			return false
		}
		s.skipDepth++
	case xml.EndElement:
		s.skipDepth--
	}
	return true
}

// copyToken reproduces a keep-subtree block verbatim. It is reserved for
// protocol policy (rdeEppParams:dcp), which carries no object data, so the
// descendants are copied rather than classified one by one.
func (s *rewriteState) copyToken(tok xml.Token, at time.Time) bool {
	switch t := tok.(type) {
	case xml.StartElement:
		s.flushPending()
		s.elems++
		s.res.SourceElements = s.elems
		s.res.Counts.Kept++
		if s.elems > s.rw.Limits.MaxElements {
			s.res.Add(Finding{Code: CodeLimitElementCount, Severity: SeverityError, Stage: StageRewrite,
				Message: "source exceeds the element limit (" + i64toa(s.rw.Limits.MaxElements) + ")", At: at})
			return false
		}
		s.push(s.newScope(t.Attr), "", t.Name, Action{Kind: ActKeepSubtree})
		s.pending = s.startTag(t.Name, t.Attr)
		s.copyDepth++
	case xml.EndElement:
		if len(s.names) == 0 {
			s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite, Message: "source XML is malformed", At: at})
			return false
		}
		if s.pending != nil {
			s.emit(s.pending[:len(s.pending)-1])
			s.emitString("/>")
			s.pending = nil
		} else {
			s.emitString(s.endTag(t.Name))
		}
		s.pop()
		s.copyDepth--
		s.action = s.currentAction()
	case xml.CharData:
		s.flushPending()
		writeText(s.w, string(t))
	}
	return true
}

func (s *rewriteState) endElement(t xml.EndElement, at time.Time) bool {
	if len(s.keys) == 0 {
		s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite,
			Message: "source XML is malformed", At: at})
		return false
	}
	open := s.names[len(s.names)-1]
	if open.Local != t.Name.Local || open.Space != t.Name.Space {
		s.res.Add(Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageRewrite,
			Locator: s.locator(), Message: "source XML is malformed: mismatched end element", At: at})
		return false
	}

	if s.pending != nil && s.action.Kind != ActKeep && s.action.Kind != ActKeepSubtree {
		// A value element with substitution: the text was buffered, so the
		// replacement is written between the tags.
		out, ok := s.substitute(at)
		if !ok {
			return false
		}
		s.emit(s.pending)
		s.pending = nil
		writeText(s.w, out)
		s.emitString(s.endTag(t.Name))
	} else if s.pending != nil {
		// Nothing between the tags: keep the source's self-closing form.
		s.emit(s.pending[:len(s.pending)-1])
		s.emitString("/>")
		s.pending = nil
	} else {
		s.emitString(s.endTag(t.Name))
	}
	s.pop()
	s.action = s.currentAction()
	s.value.Reset()
	return true
}

// currentAction is the action of the element now on top of the stack, so a
// parent's classification survives its children.
func (s *rewriteState) currentAction() Action {
	if len(s.actions) == 0 {
		return Action{Kind: ActKeep}
	}
	return s.actions[len(s.actions)-1]
}

func (s *rewriteState) charData(t xml.CharData, at time.Time) bool {
	switch s.action.Kind {
	case ActKeep, ActKeepSubtree:
		s.flushPending()
		writeText(s.w, string(t))
		return true
	default:
		// A substituted element's text is buffered and never written, never
		// logged and never placed in a finding.
		if s.value.Len()+len(t) > s.rw.Limits.MaxFieldBytes {
			s.res.Add(Finding{Code: CodeLimitFieldBytes, Severity: SeverityError, Stage: StageRewrite,
				Locator: s.locator(), Message: "field exceeds the length limit (" + itoa(s.rw.Limits.MaxFieldBytes) + " bytes)", At: at})
			return false
		}
		s.value.Write(t)
		return true
	}
}

// substitute applies the current element's action to its buffered text.
func (s *rewriteState) substitute(at time.Time) (string, bool) {
	original := s.value.String()
	switch s.action.Kind {
	case ActTokenize:
		if strings.TrimSpace(original) == "" {
			return original, true
		}
		s.res.Counts.Tokenized++
		return s.rw.Tokens.Value(s.action.Value, strings.TrimSpace(original)), true
	case ActSynthesize:
		s.res.Counts.Synthesized++
		return s.rw.Tokens.Value(s.action.Value, strings.TrimSpace(original)), true
	case ActRewriteFQDN:
		out, changed, ok := s.rw.Suffix.Rewrite(original)
		if !ok {
			s.res.Add(Finding{Code: CodeFQDNUnrewritable, Severity: SeverityError, Stage: StageRewrite,
				Locator: s.locator(), Message: "a name carrying the registry suffix could not be rewritten", At: at})
			return "", false
		}
		if changed {
			s.res.Counts.Rewritten++
		} else {
			s.res.Counts.Kept++
		}
		return out, true
	default:
		return original, true
	}
}

// ---------------------------------------------------------------------------
// namespaces, keys and serialisation
// ---------------------------------------------------------------------------

func (s *rewriteState) newScope(attrs []xml.Attr) map[string]string {
	var scope map[string]string
	for _, a := range attrs {
		switch {
		case a.Name.Space == "xmlns":
			if scope == nil {
				scope = map[string]string{}
			}
			scope[a.Name.Local] = a.Value
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			if scope == nil {
				scope = map[string]string{}
			}
			scope[""] = a.Value
		}
	}
	return scope
}

// resolve walks the scope stack outwards, newest frame first.
func (s *rewriteState) resolve(local map[string]string, prefix string) (string, bool) {
	if uri, ok := local[prefix]; ok {
		return uri, true
	}
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if uri, ok := s.scopes[i][prefix]; ok {
			return uri, true
		}
	}
	return "", false
}

// key returns the profile key ("alias:local") for an element name.
func (s *rewriteState) key(local map[string]string, name xml.Name) (string, bool) {
	uri, ok := s.resolve(local, name.Space)
	if !ok {
		return "", false
	}
	alias, ok := Alias(uri)
	if !ok {
		return "", false
	}
	return alias + ":" + name.Local, true
}

func (s *rewriteState) parentKey() string {
	if len(s.keys) == 0 {
		return ""
	}
	return s.keys[len(s.keys)-1]
}

// locator names a position without naming content: the ancestor chain is made
// of profile keys, which are fixed vocabulary, plus the element index.
func (s *rewriteState) locator() string {
	if len(s.keys) == 0 {
		return "element[" + i64toa(s.elems) + "]"
	}
	return s.keys[len(s.keys)-1] + " element[" + i64toa(s.elems) + "]"
}

func (s *rewriteState) push(scope map[string]string, key string, name xml.Name, action Action) {
	if scope == nil {
		scope = map[string]string{}
	}
	s.scopes = append(s.scopes, scope)
	s.keys = append(s.keys, key)
	s.names = append(s.names, name)
	s.actions = append(s.actions, action)
}

func (s *rewriteState) pop() {
	s.scopes = s.scopes[:len(s.scopes)-1]
	s.keys = s.keys[:len(s.keys)-1]
	s.names = s.names[:len(s.names)-1]
	s.actions = s.actions[:len(s.actions)-1]
}

// attributes validates every attribute against the profile and returns the ones
// to emit. Namespace declarations always pass; schema hints are dropped.
func (s *rewriteState) attributes(scope map[string]string, key string, attrs []xml.Attr, at time.Time) ([]xml.Attr, bool) {
	out := make([]xml.Attr, 0, len(attrs))
	for _, a := range attrs {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			out = append(out, a)
			continue
		}
		if a.Name.Space != "" {
			uri, ok := s.resolve(scope, a.Name.Space)
			if ok && uri == XSIURI {
				continue // schema hint: recognised, deliberately not reproduced
			}
			s.res.Add(Finding{Code: CodePolicyUnknownAttribute, Severity: SeverityError, Stage: StageRewrite,
				Locator: s.locator(), Message: "attribute is in a namespace the profile does not classify", At: at})
			return nil, false
		}
		if !s.rw.Profile.AllowsAttribute(s.parentKey(), key, a.Name.Local) {
			s.res.Add(Finding{Code: CodePolicyUnknownAttribute, Severity: SeverityError, Stage: StageRewrite,
				Locator: s.locator(), Message: "attribute is not classified by profile " + s.rw.Profile.Version(), At: at})
			return nil, false
		}
		out = append(out, a)
	}
	return out, true
}

func rawName(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return n.Space + ":" + n.Local
}

func (s *rewriteState) startTag(name xml.Name, attrs []xml.Attr) []byte {
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(rawName(name))
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(rawName(a.Name))
		b.WriteString(`="`)
		b.WriteString(escapeAttr(a.Value))
		b.WriteByte('"')
	}
	b.WriteByte('>')
	return []byte(b.String())
}

func (s *rewriteState) endTag(name xml.Name) string {
	return "</" + rawName(name) + ">"
}

func (s *rewriteState) flushPending() {
	if s.pending != nil {
		s.emit(s.pending)
		s.pending = nil
	}
}

// textEscaper escapes only what character data must escape. encoding/xml's
// EscapeText also turns newlines and tabs into character references, which
// would mangle the source's layout and, outside the root element, turn
// whitespace into text and make the derivative unparseable.
var textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#xD;")

func writeText(w *bufio.Writer, s string) { _, _ = textEscaper.WriteString(w, s) }

// emit and emitString write to the buffered output. bufio.Writer latches its
// first error and reports it from Flush, which Rewrite checks once at the end,
// so the individual writes are deliberately unchecked: branching on each would
// add a dozen paths reachable only after the single checked one has failed.
func (s *rewriteState) emit(b []byte)       { _, _ = s.w.Write(b) }
func (s *rewriteState) emitString(x string) { _, _ = s.w.WriteString(x) }

var attrEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;",
	"\t", "&#x9;", "\n", "&#xA;", "\r", "&#xD;",
)

func escapeAttr(v string) string { return attrEscaper.Replace(v) }
