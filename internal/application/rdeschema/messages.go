package rdeschema

import (
	"regexp"
	"strconv"
	"strings"
)

// Class is the kind of schema violation, in a vocabulary this package owns and
// that does not change when libxml2 rewords a message. The validator turns a
// Class into the text an operator reads; the libxml2 sentence itself is never
// passed on, because it quotes the offending value.
type Class string

const (
	ClassUnexpectedElement   Class = "unexpected-element"   // an element the schema does not allow at this position
	ClassMissingElement      Class = "missing-element"      // required content ended before a required element
	ClassMissingAttribute    Class = "missing-attribute"    // a required attribute is absent
	ClassUnexpectedAttribute Class = "unexpected-attribute" // an attribute the element does not declare
	ClassInvalidValue        Class = "invalid-value"        // element or attribute text not valid for its type
	ClassUnexpectedText      Class = "unexpected-text"      // character content where only elements are allowed
	ClassUndeclaredElement   Class = "undeclared-element"   // an element no schema in the set declares
	ClassIdentity            Class = "identity-constraint"  // a key, unique or keyref constraint
	ClassOther               Class = "other"                // anything this table does not recognise
)

// Name is an element, attribute or type name taken from libxml2's output.
//
// A name that the pinned schemas themselves declare — rde:watermark, an
// attribute called "type" — is vocabulary and can be repeated freely. A name
// the deposit supplied is untrusted markup, and is kept apart: Known is false
// and Text is only safe for the one field the finding model reserves for
// deposit-sourced text.
type Name struct {
	Known bool
	Text  string
}

// Violation is one schema violation, stripped of everything the deposit said.
type Violation struct {
	Class Class
	// Line is the 1-based line of the offending element's start tag. Stream
	// validation does not report columns.
	Line int
	// Element is the element the violation is reported against: the offender
	// for most classes, the parent for a missing child.
	Element Name
	// Attribute is set for attribute violations and for an invalid attribute
	// value.
	Attribute Name
	// Facet names the restriction facet an invalid value broke, from a closed
	// set (pattern, enumeration, maxLength …); empty when none applies.
	Facet string
	// Type is the schema type the value was checked against, when the schema
	// names one (xs:dateTime, rdeDomain:…).
	Type string
	// Expected lists the elements the schema would have accepted instead, as
	// prefix:local, and only those the schema set declares.
	Expected []string
}

// lineKind is what one line of xmllint's stderr turned out to be.
type lineKind int

const (
	lineIgnored    lineKind = iota // context echo, summary, anything unrecognised
	lineViolation                  // a schema validity error
	lineParseError                 // the document is not well-formed (libxml2's view)
	lineParseFail                  // xmllint's own "failed to parse" verdict
)

var (
	reValidity   = regexp.MustCompile(`^-:(\d+): Schemas validity error : (.*)$`)
	reParseError = regexp.MustCompile(`^-:(\d+): (?:parser|namespace) error : `)
	reElement    = regexp.MustCompile(`^Element '([^']*)'(?:, attribute '([^']*)')?: (.*)$`)
	reQName      = regexp.MustCompile(`\{([^}]*)\}([^\s,()]+)`)
	reFacet      = regexp.MustCompile(`\[facet '([A-Za-z]+)'\]`)
	reTypeName   = regexp.MustCompile(`(?:atomic|list|union) type '([^']*)'`)
	reAttrInMsg  = regexp.MustCompile(`^The attribute '([^']*)' is`)
)

// knownFacets is the closed set of XSD facets libxml2 names. A facet outside it
// is reported as an unnamed one rather than repeated.
var knownFacets = map[string]bool{
	"pattern": true, "enumeration": true, "length": true, "minLength": true, "maxLength": true,
	"minInclusive": true, "maxInclusive": true, "minExclusive": true, "maxExclusive": true,
	"totalDigits": true, "fractionDigits": true, "whiteSpace": true,
}

const maxNameLen = 96

// parseLine classifies one line of xmllint's stderr.
func parseLine(line string, v vocabulary) (Violation, lineKind) {
	line = strings.TrimRight(line, "\r\n")
	if strings.HasSuffix(line, "failed to parse") {
		return Violation{}, lineParseFail
	}
	if m := reParseError.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[1])
		return Violation{Class: ClassOther, Line: n}, lineParseError
	}
	m := reValidity.FindStringSubmatch(line)
	if m == nil {
		return Violation{}, lineIgnored
	}
	n, _ := strconv.Atoi(m[1])
	viol := Violation{Class: ClassOther, Line: n}
	em := reElement.FindStringSubmatch(m[2])
	if em == nil {
		return viol, lineViolation
	}
	viol.Element = v.name(em[1])
	if em[2] != "" {
		viol.Attribute = v.attrName(em[2])
	}
	msg := em[3]

	switch {
	case strings.Contains(msg, "This element is not expected"):
		viol.Class = ClassUnexpectedElement
		viol.Expected = v.expected(msg)
	case strings.Contains(msg, "Missing child element(s)"):
		viol.Class = ClassMissingElement
		viol.Expected = v.expected(msg)
	case strings.Contains(msg, "is required but missing"):
		viol.Class = ClassMissingAttribute
		if am := reAttrInMsg.FindStringSubmatch(msg); am != nil {
			viol.Attribute = v.attrName(am[1])
		}
	case strings.Contains(msg, "is not allowed") && viol.Attribute.Text != "":
		viol.Class = ClassUnexpectedAttribute
	case strings.Contains(msg, "[facet ") || strings.Contains(msg, "is not a valid value of"):
		viol.Class = ClassInvalidValue
		if fm := reFacet.FindStringSubmatch(msg); len(fm) == 2 && knownFacets[fm[1]] {
			viol.Facet = fm[1]
		}
		if tm := reTypeName.FindStringSubmatch(msg); tm != nil {
			viol.Type = v.typeName(tm[1])
		}
	case strings.Contains(msg, "Character content") && strings.Contains(msg, "not allowed"):
		viol.Class = ClassUnexpectedText
	case strings.Contains(msg, "No matching global") || strings.Contains(msg, "strict wildcard"):
		viol.Class = ClassUndeclaredElement
	case strings.Contains(msg, "key-sequence") || strings.Contains(msg, "identity-constraint"):
		viol.Class = ClassIdentity
	}
	return viol, lineViolation
}

// splitQName splits libxml2's "{namespace}local" form.
func splitQName(s string) (ns, local string) {
	if strings.HasPrefix(s, "{") {
		if i := strings.Index(s, "}"); i > 0 {
			return s[1:i], s[i+1:]
		}
	}
	return "", s
}

// name classifies an element name from libxml2's output. It is Known only when
// the namespace and the local name are both ones the pinned schemas declare.
func (v vocabulary) name(qn string) Name {
	ns, local := splitQName(qn)
	prefix, nsKnown := v.namespaces[ns]
	if nsKnown && v.names[local] && cleanName(local) == local {
		return Name{Known: true, Text: prefix + ":" + local}
	}
	switch {
	case ns == "":
		return Name{Text: cleanName(local)}
	case nsKnown:
		// A declared namespace with a name the schema does not declare there:
		// the prefix is vocabulary, the local name is not.
		return Name{Text: prefix + ":" + cleanName(local)}
	default:
		return Name{Text: "{?}" + cleanName(local)}
	}
}

// attrName classifies an attribute name. Attributes are unqualified in this
// schema set, so Known means the name is one the schemas declare.
func (v vocabulary) attrName(a string) Name {
	_, local := splitQName(a)
	if v.names[local] && cleanName(local) == local {
		return Name{Known: true, Text: local}
	}
	return Name{Text: cleanName(local)}
}

// typeName returns the schema type libxml2 named, or "" when it is not one the
// schemas (or XSD itself) declare — a type name is part of the schema, but
// "the schema" is only trusted to the extent this table can check it.
func (v vocabulary) typeName(t string) string {
	if strings.HasPrefix(t, "xs:") {
		if b := strings.TrimPrefix(t, "xs:"); b != "" && cleanName(b) == b {
			return t
		}
		return ""
	}
	n := v.name(t)
	if n.Known {
		return n.Text
	}
	return ""
}

// expected extracts the "Expected is ( … )" list, keeping only declared names.
func (v vocabulary) expected(msg string) []string {
	i := strings.Index(msg, "Expected is")
	if i < 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range reQName.FindAllStringSubmatch(msg[i:], -1) {
		n := v.name("{" + m[1] + "}" + m[2])
		if n.Known && !seen[n.Text] {
			seen[n.Text] = true
			out = append(out, n.Text)
		}
	}
	return out
}

// cleanName reduces a name to the characters an XML name can sensibly contain
// and bounds its length, so that whatever the deposit used as an element name
// cannot smuggle markup, control characters or an unbounded string into a
// finding.
func cleanName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxNameLen {
			b.WriteString("…")
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('?')
		}
		n++
	}
	return b.String()
}
