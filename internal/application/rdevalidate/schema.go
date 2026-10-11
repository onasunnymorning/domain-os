package rdevalidate

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
)

// schemaTee hands every byte the XML validator reads to the schema session.
//
// The two checks look at one stream at one time, which is the only way to
// schema-validate a signed deposit without reading it a third time or writing
// plaintext anywhere: the bytes are produced once, by decrypt-and-unpack, and
// are consumed twice, in parallel — by the Go checks and by libxml2.
//
// A failing or slow session is the session's problem. Write never fails, so
// the Go checks are never cut short by it; and because the tee is what the Go
// checks read, the schema check is back-pressured by them rather than the
// other way round.
type schemaTee struct {
	r io.Reader
	s rdeschema.Session
	// eof is set when the underlying reader reported the end of the document.
	// Only a document the Go checks read to its end has been handed to the
	// schema session in full; anything else is not a document it can judge.
	eof bool
}

func (t *schemaTee) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		_, _ = t.s.Write(p[:n])
	}
	if errors.Is(err, io.EOF) {
		t.eof = true
	}
	return n, err
}

// Rule texts for XML_SCHEMA_INVALID, one per violation class. They are the
// finding's Rule, so the tally counts each class separately, and they are
// constants: libxml2's own sentence is never passed on, because it quotes the
// offending value.
var schemaRules = map[rdeschema.Class]string{
	rdeschema.ClassUnexpectedElement:   "schema: element is not allowed at this position",
	rdeschema.ClassMissingElement:      "schema: a required element is missing",
	rdeschema.ClassMissingAttribute:    "schema: a required attribute is missing",
	rdeschema.ClassUnexpectedAttribute: "schema: attribute is not declared for this element",
	rdeschema.ClassInvalidValue:        "schema: value is not valid for its type",
	rdeschema.ClassUnexpectedText:      "schema: character content is not allowed here",
	rdeschema.ClassUndeclaredElement:   "schema: element is not declared by any pinned schema",
	rdeschema.ClassIdentity:            "schema: identity constraint is violated",
	rdeschema.ClassOther:               "schema: the deposit breaks a schema rule this validator cannot name yet",
}

// schemaFinding renders one violation.
//
// What may appear in the message is fenced by what libxml2 was quoting. The
// names of elements and attributes are used only when the pinned schemas
// declare them (Name.Known), and the elements the schema expected are always
// the schema's own. A name the deposit invented is carried in Object — the one
// field the finding model reserves for deposit-sourced text — cleaned and
// bounded, and never in Message, Rule or Locator.
func schemaFinding(v rdeschema.Violation, now time.Time) Finding {
	rule, ok := schemaRules[v.Class]
	if !ok {
		rule = schemaRules[rdeschema.ClassOther]
	}
	f := Finding{
		Code: CodeXMLSchemaInvalid, Severity: SeverityError, Stage: StageXML,
		Rule: rule, At: now,
	}
	if v.Line > 0 {
		f.Locator = "line=" + itoa(v.Line)
	}
	if v.Element.Text != "" {
		f.Object = v.Element.Text
		if v.Attribute.Text != "" {
			f.Object += "@" + v.Attribute.Text
		}
	}
	f.Message = schemaMessage(v)
	return f
}

func schemaMessage(v rdeschema.Violation) string {
	where := "an element"
	if v.Element.Known {
		where = "element " + v.Element.Text
	}
	attr := "an attribute"
	if v.Attribute.Known {
		attr = "attribute " + v.Attribute.Text
	}
	expected := ""
	if len(v.Expected) > 0 {
		expected = "; the schema allows: " + strings.Join(v.Expected, ", ")
	}
	switch v.Class {
	case rdeschema.ClassUnexpectedElement:
		if len(v.Expected) == 0 {
			expected = "; the schema allows no further element here"
		}
		return where + " is not allowed at this position" + expected
	case rdeschema.ClassMissingElement:
		if len(v.Expected) == 0 {
			return where + " is missing required content"
		}
		return where + " is missing a required child element; the schema requires: " + strings.Join(v.Expected, ", ")
	case rdeschema.ClassMissingAttribute:
		if v.Attribute.Known {
			return where + " is missing its required attribute " + v.Attribute.Text
		}
		return where + " is missing a required attribute"
	case rdeschema.ClassUnexpectedAttribute:
		return where + " carries " + attr + " the schema does not declare for it"
	case rdeschema.ClassInvalidValue:
		what := where
		if v.Attribute.Text != "" {
			what = attr + " of " + where
		}
		msg := what + " has a value that is not valid"
		if v.Type != "" {
			msg += " for the type " + v.Type
		}
		if v.Facet != "" {
			msg += " (facet " + v.Facet + ")"
		}
		return msg
	case rdeschema.ClassUnexpectedText:
		return where + " contains character content where only elements are allowed"
	case rdeschema.ClassUndeclaredElement:
		return where + " is not declared by any schema in the pinned set"
	case rdeschema.ClassIdentity:
		return where + " violates a uniqueness or key constraint of the schema"
	default:
		return where + " breaks a schema rule"
	}
}

// schemaOutcome turns the session's verdict into findings on res.
//
// It is the place the "never PASS on a check that did not run" rule is kept:
// every verdict other than ConclusionValid produces an ERROR-severity finding,
// and the ones that mean "could not decide" produce the error-class code that
// makes the run's outcome ERROR rather than FAIL.
func schemaOutcome(res *Result, vd rdeschema.Verdict, now func() time.Time) {
	switch vd.Conclusion {
	case rdeschema.ConclusionValid:
		return

	case rdeschema.ConclusionInvalid:
		for _, v := range vd.Violations {
			res.Add(schemaFinding(v, now()))
		}
		if len(vd.Violations) == 0 {
			// An engine that says "invalid" and cannot say where must still
			// not let the deposit through.
			res.Add(schemaFinding(rdeschema.Violation{Class: rdeschema.ClassOther}, now()))
		}
		switch {
		case vd.Stopped:
			res.Add(Finding{Code: CodeXMLSchemaCheckTruncated, Severity: SeverityInfo, Stage: StageXML,
				Message: "schema checking stopped after " + itoa(vd.Total) + " violations; the rest of the deposit was not schema-checked",
				At:      now()})
		case vd.Total > len(vd.Violations):
			res.Add(Finding{Code: CodeXMLSchemaCheckTruncated, Severity: SeverityInfo, Stage: StageXML,
				Message: itoa(vd.Total) + " schema violations were found; the first " + itoa(len(vd.Violations)) + " are listed",
				At:      now()})
		}

	case rdeschema.ConclusionNotWellFormed:
		// The Go checks usually say this first; do not say it twice.
		if res.Has(CodeXMLMalformed) {
			return
		}
		f := Finding{Code: CodeXMLMalformed, Severity: SeverityError, Stage: StageXML,
			Message: "deposit XML is not well-formed or exceeds the XML parser's limits", At: now()}
		if vd.ParseLine > 0 {
			f.Locator = "line=" + itoa(vd.ParseLine)
		}
		res.Add(f)

	case rdeschema.ConclusionRejected:
		switch vd.Reason {
		case rdeschema.RejectDocType:
			res.Add(Finding{Code: CodeXMLDTDNotSupported, Severity: SeverityError, Stage: StageXML,
				Message: "the deposit carries a document type declaration, which is not supported: an RDE deposit is a plain XML document and its entities are never resolved",
				At:      now()})
		default:
			res.Add(Finding{Code: CodeXMLPrologNotSupported, Severity: SeverityError, Stage: StageXML,
				Message: "the deposit does not start as UTF-8 XML (an XML declaration naming UTF-8, or the root element)",
				At:      now()})
		}

	default:
		res.Add(Finding{Code: CodeSchemaEngineUnavailable, Severity: SeverityError, Stage: StageXML,
			Message: "the schema check could not be completed, so the deposit's conformance is undecided: " + engineDetail(vd.Detail),
			At:      now()})
	}
}

// engineDetail keeps a verdict's detail to what the engine is allowed to say:
// constant words and numbers.
func engineDetail(d string) string {
	if d == "" {
		return "schema engine failed"
	}
	return d
}

// runSchema owns the session for one run. It is created before the XML is read
// and settled after.
type runSchema struct {
	sess rdeschema.Session
	tee  *schemaTee
}

// startSchema opens the schema session, or records that there is none.
//
// There is deliberately no path on which a missing engine is quietly skipped:
// an Input without one, an engine that is installed wrongly and an engine that
// fails to start all end in the same ERROR-class finding, so a worker that
// cannot enforce the schemas cannot pass a deposit.
func startSchema(ctx context.Context, in Input, res *Result, now func() time.Time) *runSchema {
	unavailable := func(why string) *runSchema {
		res.Add(Finding{Code: CodeSchemaEngineUnavailable, Severity: SeverityError, Stage: StageXML,
			Message: "the schema check could not be completed, so the deposit's conformance is undecided: " + why, At: now()})
		return nil
	}
	if in.Schema == nil {
		return unavailable("no schema validation engine is configured")
	}
	sess, err := in.Schema.Start(ctx)
	if err != nil {
		return unavailable("the schema engine could not be started")
	}
	return &runSchema{sess: sess}
}

// wrap returns the reader the XML checks must read, so that the session sees
// exactly what they see.
func (r *runSchema) wrap(src io.Reader) io.Reader {
	if r == nil {
		return src
	}
	r.tee = &schemaTee{r: src, s: r.sess}
	return r.tee
}

// settle concludes the check once the XML checks have finished with the stream.
// It must be called exactly once.
func (r *runSchema) settle(ctx context.Context, res *Result, now func() time.Time) {
	if r == nil {
		return
	}
	if ctx.Err() != nil {
		// The deadline is reported by the caller; there is nothing more to
		// learn from a half-read document.
		r.sess.Abort()
		return
	}
	if !r.tee.eof {
		// The XML checks stopped before the end of the document, which they do
		// only after recording an ERROR of their own. Judging the prefix would
		// report a truncation as a schema violation. A document the engine
		// refused on its first bytes is still refused, and says so.
		if vd := r.sess.Abort(); vd.Conclusion == rdeschema.ConclusionRejected {
			schemaOutcome(res, vd, now)
		}
		if !hasErrorFinding(res) {
			res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageXML,
				Message: "the XML checks ended before the end of the document without recording why, so the schema check could not be completed",
				At:      now()})
		}
		return
	}
	schemaOutcome(res, r.sess.Finish(), now)
}

// hasErrorFinding reports whether the run already carries an ERROR-severity
// finding.
func hasErrorFinding(res *Result) bool {
	for _, t := range res.tallySnapshot() {
		if t.Count > 0 && t.Severity == SeverityError {
			return true
		}
	}
	return false
}
