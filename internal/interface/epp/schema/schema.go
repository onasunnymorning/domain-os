// Package schema validates inbound EPP frames against Go models of the EPP
// XML schemas (RFC 5730, RFC 5731 and the extensions the server supports).
//
// Go has no complete XSD validator, and the EPP server stays a static,
// cgo-free binary (ADR 0016). So each XSD the server accepts frames for is
// mirrored here as a content model: element names and order, occurrence
// bounds, attribute declarations, and the simple-type facets of every text
// value. The models are only as faithful as their tests: a differential test
// validates a corpus of frames, and systematic mutations of it, with both
// these models and xmllint on the schemas pinned from ICANN's RST release
// (../testdata/rst/xsd), and fails on any disagreement.
//
// A validation error names the element or attribute and the rule it broke,
// never the value: values include passwords.
package schema

import (
	"fmt"
	"strings"

	"github.com/beevik/etree"
)

// Unbounded is a maxOccurs of "unbounded".
const Unbounded = -1

// Namespaces the models use.
const (
	NSEPP    = "urn:ietf:params:xml:ns:epp-1.0"
	NSDomain = "urn:ietf:params:xml:ns:domain-1.0"
	NSHost   = "urn:ietf:params:xml:ns:host-1.0"
	NSEPPCom = "urn:ietf:params:xml:ns:eppcom-1.0"
	NSSecDNS = "urn:ietf:params:xml:ns:secDNS-1.1"
	NSXSI    = "http://www.w3.org/2001/XMLSchema-instance"
)

// prefixes name namespaces in error paths, so a path reads like the frame.
var prefixes = map[string]string{
	NSEPP:    "",
	NSDomain: "domain:",
	NSHost:   "host:",
	NSSecDNS: "secDNS:",
}

// Error is a frame that does not match the schema. Path locates the element
// or attribute; Reason says which rule failed. Neither includes a value.
type Error struct {
	Path   string
	Reason string
}

func (e *Error) Error() string { return e.Path + ": " + e.Reason }

// Element is an element declaration. Exactly one of Simple, Content and
// AnyType describes its content; if none does, the element must be empty.
type Element struct {
	NS, Name string
	// Min and Max are minOccurs and maxOccurs where the element is used as
	// a particle. Zero values mean 1: set Min to Optional for minOccurs 0.
	Min, Max int
	Attrs    []Attribute
	// Simple is the type of the element's text, for simple content.
	Simple *Simple
	// Content is the element-only content model.
	Content Particle
	// AnyType is xs:anyType: any attributes, text and children, with
	// children validated only where a global declaration exists (lax).
	AnyType bool
}

// Optional is a Min of minOccurs="0"; a zero Min means 1.
const Optional = -1

func (e *Element) occurs() (lo, hi int) { return occurs(e.Min, e.Max) }

// Attribute is an attribute declaration. Attributes are unqualified, as in
// every EPP schema.
type Attribute struct {
	Name     string
	Type     *Simple
	Required bool
}

// Particle is a piece of an element-only content model: an element, a
// sequence, a choice or a wildcard.
type Particle interface {
	// match consumes the children of parent from index i that this
	// particle accounts for, and returns the index after them.
	match(v *validator, parent string, kids []*etree.Element, i int) (int, error)
	// startsWith reports whether el can be the first element this particle
	// consumes.
	startsWith(v *validator, el *etree.Element) bool
	// emptiable reports whether the particle may consume nothing.
	emptiable() bool
}

// Seq is xs:sequence, occurring once.
type Seq []Particle

// Choice is xs:choice. Zero Min and Max mean once.
type Choice struct {
	Alts     []Particle
	Min, Max int
}

// Any is xs:any namespace="##other" processContents="strict": elements from
// any namespace but OtherThan (and not unqualified), each of which must have
// a global declaration. Zero Min and Max mean once.
type Any struct {
	OtherThan string
	Min, Max  int
}

// Set is the global element declarations a frame may use: the root, and
// what wildcards in it may resolve to.
type Set struct {
	root    *Element
	globals map[qname]*Element
}

type qname struct{ ns, local string }

// NewSet returns a set rooted at root, with globals available to wildcards.
func NewSet(root *Element, globals ...*Element) *Set {
	s := &Set{root: root, globals: map[qname]*Element{}}
	for _, g := range globals {
		s.globals[qname{g.NS, g.Name}] = g
	}
	return s
}

// Validate reports whether doc is valid against the set's root.
func (s *Set) Validate(doc *etree.Document) error {
	root := doc.Root()
	if root == nil {
		return &Error{Path: "/", Reason: "no root element"}
	}
	v := &validator{set: s}
	if !v.is(root, s.root) {
		return &Error{Path: "/" + v.name(root), Reason: "unexpected root element"}
	}
	return v.element(root, s.root, "")
}

type validator struct{ set *Set }

func (v *validator) is(el *etree.Element, decl *Element) bool {
	return el.Tag == decl.Name && el.NamespaceURI() == decl.NS
}

func (v *validator) name(el *etree.Element) string {
	if p, ok := prefixes[el.NamespaceURI()]; ok {
		return p + el.Tag
	}
	return "{" + el.NamespaceURI() + "}" + el.Tag
}

func (v *validator) errorf(path, format string, args ...any) error {
	return &Error{Path: path, Reason: fmt.Sprintf(format, args...)}
}

// element validates el, already matched to decl, at parent's path.
func (v *validator) element(el *etree.Element, decl *Element, parent string) error {
	path := parent + "/" + v.name(el)
	if decl.AnyType {
		return v.lax(el, path)
	}
	if err := v.attributes(el, decl, path); err != nil {
		return err
	}
	kids := el.ChildElements()
	text, hasText := textOf(el)
	switch {
	case decl.Simple != nil:
		if len(kids) > 0 {
			return v.errorf(path, "element content is not allowed here; only text")
		}
		if err := decl.Simple.validate(text); err != nil {
			return v.errorf(path, "%s", err)
		}
		return nil
	case decl.Content != nil:
		if hasText {
			return v.errorf(path, "text is not allowed here; only elements")
		}
		i, err := decl.Content.match(v, path, kids, 0)
		if err != nil {
			return err
		}
		if i < len(kids) {
			return v.errorf(path+"/"+v.name(kids[i]), "element not expected here")
		}
		return nil
	default:
		if len(kids) > 0 {
			return v.errorf(path+"/"+v.name(kids[0]), "element not expected here; the content must be empty")
		}
		if text != "" {
			return v.errorf(path, "text is not allowed here; the content must be empty")
		}
		return nil
	}
}

// lax validates xs:anyType content: anything goes, except that a child with
// a global declaration must match it.
func (v *validator) lax(el *etree.Element, path string) error {
	for _, kid := range el.ChildElements() {
		if decl, ok := v.set.globals[qname{kid.NamespaceURI(), kid.Tag}]; ok {
			if err := v.element(kid, decl, path); err != nil {
				return err
			}
			continue
		}
		if err := v.lax(kid, path+"/"+v.name(kid)); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) attributes(el *etree.Element, decl *Element, path string) error {
	seen := map[string]bool{}
	for i := range el.Attr {
		a := &el.Attr[i]
		switch {
		case a.Space == "xmlns" || (a.Space == "" && a.Key == "xmlns"):
			continue // a namespace declaration, not an attribute
		case a.Space != "":
			if a.NamespaceURI() == NSXSI && (a.Key == "schemaLocation" || a.Key == "noNamespaceSchemaLocation") {
				continue // location hints, which validation ignores
			}
			return v.errorf(path+"/@"+a.Space+":"+a.Key, "attribute not allowed")
		}
		ad := findAttr(decl.Attrs, a.Key)
		if ad == nil {
			return v.errorf(path+"/@"+a.Key, "attribute not allowed")
		}
		if err := ad.Type.validate(a.Value); err != nil {
			return v.errorf(path+"/@"+a.Key, "%s", err)
		}
		seen[a.Key] = true
	}
	for _, ad := range decl.Attrs {
		if ad.Required && !seen[ad.Name] {
			return v.errorf(path+"/@"+ad.Name, "required attribute missing")
		}
	}
	return nil
}

func findAttr(attrs []Attribute, name string) *Attribute {
	for i := range attrs {
		if attrs[i].Name == name {
			return &attrs[i]
		}
	}
	return nil
}

// textOf returns el's character data, and whether any of it is not
// whitespace. Comments and processing instructions are not content.
func textOf(el *etree.Element) (string, bool) {
	var b strings.Builder
	nonSpace := false
	for _, tok := range el.Child {
		if cd, ok := tok.(*etree.CharData); ok {
			b.WriteString(cd.Data)
			nonSpace = nonSpace || strings.Trim(cd.Data, " \t\r\n") != ""
		}
	}
	return b.String(), nonSpace
}

// --- particles ---------------------------------------------------------------

func (e *Element) match(v *validator, parent string, kids []*etree.Element, i int) (int, error) {
	lo, hi := e.occurs()
	n := 0
	for i < len(kids) && (hi == Unbounded || n < hi) && v.is(kids[i], e) {
		if err := v.element(kids[i], e, parent); err != nil {
			return i, err
		}
		i++
		n++
	}
	if n < lo {
		if i < len(kids) {
			return i, v.errorf(parent+"/"+v.name(kids[i]), "element not expected here; expected <%s>", e.Name)
		}
		return i, v.errorf(parent, "missing <%s>", e.Name)
	}
	return i, nil
}

func (e *Element) startsWith(v *validator, el *etree.Element) bool { return v.is(el, e) }

func (e *Element) emptiable() bool { lo, _ := e.occurs(); return lo == 0 }

func (s Seq) match(v *validator, parent string, kids []*etree.Element, i int) (int, error) {
	var err error
	for _, p := range s {
		if i, err = p.match(v, parent, kids, i); err != nil {
			return i, err
		}
	}
	return i, nil
}

func (s Seq) startsWith(v *validator, el *etree.Element) bool {
	for _, p := range s {
		if p.startsWith(v, el) {
			return true
		}
		if !p.emptiable() {
			return false
		}
	}
	return false
}

func (s Seq) emptiable() bool {
	for _, p := range s {
		if !p.emptiable() {
			return false
		}
	}
	return true
}

// occurs turns Min and Max fields into minOccurs and maxOccurs: zero means
// 1, Optional means 0.
func occurs(lo, hi int) (int, int) {
	switch lo {
	case 0:
		lo = 1
	case Optional:
		lo = 0
	}
	if hi == 0 {
		hi = 1
	}
	return lo, hi
}

func (c Choice) match(v *validator, parent string, kids []*etree.Element, i int) (int, error) {
	lo, hi := occurs(c.Min, c.Max)
	n := 0
	for i < len(kids) && (hi == Unbounded || n < hi) {
		var alt Particle
		for _, a := range c.Alts {
			if a.startsWith(v, kids[i]) {
				alt = a
				break
			}
		}
		if alt == nil {
			break
		}
		var err error
		if i, err = alt.match(v, parent, kids, i); err != nil {
			return i, err
		}
		n++
	}
	if n < lo && !c.altEmptiable() {
		if i < len(kids) {
			return i, v.errorf(parent+"/"+v.name(kids[i]), "element not expected here")
		}
		return i, v.errorf(parent, "missing a required element")
	}
	return i, nil
}

func (c Choice) altEmptiable() bool {
	for _, a := range c.Alts {
		if a.emptiable() {
			return true
		}
	}
	return false
}

func (c Choice) startsWith(v *validator, el *etree.Element) bool {
	for _, a := range c.Alts {
		if a.startsWith(v, el) {
			return true
		}
	}
	return false
}

func (c Choice) emptiable() bool {
	lo, _ := occurs(c.Min, c.Max)
	return lo == 0 || c.altEmptiable()
}

func (a Any) match(v *validator, parent string, kids []*etree.Element, i int) (int, error) {
	lo, hi := occurs(a.Min, a.Max)
	n := 0
	for i < len(kids) && (hi == Unbounded || n < hi) && a.startsWith(v, kids[i]) {
		decl, ok := v.set.globals[qname{kids[i].NamespaceURI(), kids[i].Tag}]
		if !ok {
			return i, v.errorf(parent+"/"+v.name(kids[i]), "no schema for this element")
		}
		if err := v.element(kids[i], decl, parent); err != nil {
			return i, err
		}
		i++
		n++
	}
	if n < lo {
		if i < len(kids) {
			return i, v.errorf(parent+"/"+v.name(kids[i]), "element not expected here")
		}
		return i, v.errorf(parent, "missing a required element")
	}
	return i, nil
}

func (a Any) startsWith(_ *validator, el *etree.Element) bool {
	ns := el.NamespaceURI()
	return ns != "" && ns != a.OtherThan
}

func (a Any) emptiable() bool { lo, _ := occurs(a.Min, a.Max); return lo == 0 }
