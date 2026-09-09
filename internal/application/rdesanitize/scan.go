package rdesanitize

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"time"
)

// ScanOptions configures the regression scan over a finished derivative.
type ScanOptions struct {
	Profile *Profile
	Suffix  *SuffixRewriter
	Limits  Limits
	Now     func() time.Time
}

// Scan re-reads a derivative and asserts the properties the ticket makes
// acceptance criteria. It exists because the rewriter and the scanner can fail
// in different ways: the rewriter is driven by the profile, and this check is
// driven by the outcome, so a classification bug that let something through is
// caught before anything is published.
//
// It asserts that:
//   - every element is classified by the profile, and no element the profile
//     drops (authInfo and anything else credential-shaped) is present;
//   - no text or attribute value still carries the original registry suffix;
//   - every synthesized or tokenized field has the synthetic shape, so a
//     missed substitution cannot pass as a real value.
//
// Findings never quote what they found: a scan that printed the leaked value
// would leak it into the run record and the logs.
func Scan(ctx context.Context, r io.Reader, o ScanOptions) []Finding {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	var findings []Finding
	add := func(code Code, locator, msg string) {
		if len(findings) >= MaxFindings {
			return
		}
		findings = append(findings, Finding{Code: code, Severity: SeverityError, Stage: StageVerify, Locator: locator, Message: msg, At: now()})
	}
	if o.Profile == nil || o.Suffix == nil {
		add(CodeInternal, "", "derivative scan is not fully configured")
		return findings
	}

	dec := xml.NewDecoder(r)
	dec.Strict = true
	var scopes []map[string]string
	var keys []string
	var actions []Action
	var elems int64
	// A keep-subtree block (protocol policy) is copied without classification
	// by the rewriter, so the scan does not classify it either. Its text and
	// attributes are still checked for a leaked suffix.
	keepSubtree := 0

	for {
		if elems%heartbeatEvery == 0 && ctx.Err() != nil {
			add(CodeSanitizeTimeout, "", "verification deadline reached")
			return findings
		}
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var se *xml.SyntaxError
			if errors.As(err, &se) {
				add(CodeDerivativeInvalid, "line="+itoa(se.Line), "derivative is not well-formed XML")
			} else {
				add(CodeDerivativeInvalid, "", "derivative could not be read")
			}
			return findings
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elems++
			scope := map[string]string{}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					scope[a.Name.Local] = a.Value
				} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
					scope[""] = a.Value
				}
			}
			scopes = append(scopes, scope)
			locator := "element[" + i64toa(elems) + "]"

			uri, ok := resolvePrefix(scopes, t.Name.Space)
			if !ok {
				add(CodeDerivativeInvalid, locator, "derivative element is in an unresolved namespace")
				return findings
			}
			alias, ok := Alias(uri)
			if !ok {
				add(CodeDerivativePIIDetected, locator, "derivative element is in a namespace the profile does not classify")
				return findings
			}
			key := alias + ":" + t.Name.Local
			parent := ""
			if len(keys) > 0 {
				parent = keys[len(keys)-1]
			}
			action, classified := o.Profile.Action(parent, key)
			if keepSubtree > 0 {
				keepSubtree++
				action = Action{Kind: ActKeepSubtree}
			} else {
				switch {
				case !classified:
					add(CodeDerivativePIIDetected, locator, "derivative carries an element the profile does not classify")
				case action.Kind == ActDropSubtree:
					add(CodeDerivativePIIDetected, locator, "derivative carries an element the profile removes")
				case action.Kind == ActKeepSubtree:
					keepSubtree = 1
				}
			}
			keys = append(keys, key)
			actions = append(actions, action)

			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				if o.Suffix.CarriesSuffix(a.Value) {
					add(CodeDerivativePIIDetected, locator, "an attribute value still carries the original registry suffix")
				}
			}

		case xml.EndElement:
			if keepSubtree > 0 {
				keepSubtree--
			}
			if len(keys) > 0 {
				keys = keys[:len(keys)-1]
				actions = actions[:len(actions)-1]
			}
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text == "" {
				continue
			}
			locator := "element[" + i64toa(elems) + "]"
			if o.Suffix.CarriesSuffix(text) {
				add(CodeDerivativePIIDetected, locator, "element text still carries the original registry suffix")
			}
			if len(actions) > 0 {
				if msg, bad := wrongSyntheticShape(actions[len(actions)-1], text); bad {
					add(CodeDerivativePIIDetected, locator, msg)
				}
			}

		case xml.Directive:
			add(CodeDerivativeInvalid, "", "derivative carries a document type declaration")
			return findings
		}
	}
	return findings
}

// wrongSyntheticShape reports a value that should have been replaced but does
// not look replaced. It checks shape, never content.
func wrongSyntheticShape(a Action, text string) (string, bool) {
	switch {
	case a.Kind == ActSynthesize && a.Value == ValEmail:
		if !strings.HasSuffix(text, "@example.invalid") {
			return "an email field was not replaced with a synthetic address", true
		}
	case a.Kind == ActSynthesize && a.Value == ValPersonName:
		if !strings.HasPrefix(text, "Contact ") {
			return "a name field was not replaced with a synthetic name", true
		}
	case a.Kind == ActTokenize && a.Value == ValContactID:
		if len(text) != 16 || !strings.HasPrefix(text, "C") {
			return "a contact identifier was not tokenised", true
		}
	case a.Kind == ActTokenize && a.Value == ValContactROID:
		if !strings.HasSuffix(text, "-RDE") {
			return "a contact ROID was not tokenised", true
		}
	}
	return "", false
}

func resolvePrefix(scopes []map[string]string, prefix string) (string, bool) {
	for i := len(scopes) - 1; i >= 0; i-- {
		if uri, ok := scopes[i][prefix]; ok {
			return uri, true
		}
	}
	return "", false
}
