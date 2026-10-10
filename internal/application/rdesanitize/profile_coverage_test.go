package rdesanitize

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/stretchr/testify/require"
)

// The sanitisation profile has to classify everything a standard-conformant
// deposit can contain. A deposit that passes validation and then quarantines
// because the profile has no entry for an element RFC 9022 defines is a gap in
// the profile, not a property of the deposit — and unless something compares
// the two, the gap is found by whichever registry's deposit hits it first.
//
// This test derives "everything a conformant deposit can contain" from the
// pinned XSDs themselves, walking the type graph from rde:deposit, and asks the
// profile to classify every element and attribute it finds.

type xnode struct {
	local string
	attrs map[string]string
	kids  []*xnode
}

type xsdDoc struct {
	targetNS string
	ns       map[string]string // prefix -> URI; "" is the default namespace
	root     *xnode
}

func parseXSD(t *testing.T, path string) *xsdDoc {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // the path is built from the schema directory this package owns
	require.NoError(t, err)
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	d := &xsdDoc{ns: map[string]string{}}
	var stack []*xnode
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch e := tok.(type) {
		case xml.StartElement:
			n := &xnode{local: e.Name.Local, attrs: map[string]string{}}
			for _, a := range e.Attr {
				switch {
				case a.Name.Space == "xmlns":
					d.ns[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					d.ns[""] = a.Value
				case a.Name.Space == "":
					n.attrs[a.Name.Local] = a.Value
				}
			}
			if len(stack) == 0 {
				d.root = n
				d.targetNS = n.attrs["targetNamespace"]
			} else {
				p := stack[len(stack)-1]
				p.kids = append(p.kids, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	require.NotNil(t, d.root, path)
	return d
}

func (d *xsdDoc) resolve(qn string) string {
	pfx, local, ok := strings.Cut(qn, ":")
	if !ok {
		pfx, local = "", qn
	}
	return "{" + d.ns[pfx] + "}" + local
}

type declRef struct {
	doc  *xsdDoc
	node *xnode
}

type schemaWalker struct {
	elements   map[string]declRef
	types      map[string]declRef
	groups     map[string]declRef
	attrGroups map[string]declRef
	subst      map[string][]string // head -> direct members

	edges     map[string]map[string]bool // parent -> child element QNames
	edgeDecls map[string][]*xnode        // "parent>child" -> the declarations that edge refers to
	declAttrs map[*xnode]map[string]bool // element declaration -> its attribute names
	wildcard  map[string]bool            // elements whose content has an xs:any
	seen      map[*xnode]bool
}

func newWalker(t *testing.T) *schemaWalker {
	t.Helper()
	w := &schemaWalker{
		elements: map[string]declRef{}, types: map[string]declRef{}, groups: map[string]declRef{},
		attrGroups: map[string]declRef{}, subst: map[string][]string{},
		edges: map[string]map[string]bool{}, edgeDecls: map[string][]*xnode{},
		declAttrs: map[*xnode]map[string]bool{}, wildcard: map[string]bool{}, seen: map[*xnode]bool{},
	}
	dir := filepath.Dir(rdeschema.Path(rdeschema.DepositSchemas))
	wrapper := parseXSD(t, rdeschema.Path(rdeschema.DepositSchemas))
	for _, k := range wrapper.root.kids {
		if k.local != "import" {
			continue
		}
		d := parseXSD(t, filepath.Join(dir, k.attrs["schemaLocation"]))
		for _, n := range d.root.kids {
			name := n.attrs["name"]
			if name == "" {
				continue
			}
			q := "{" + d.targetNS + "}" + name
			ref := declRef{d, n}
			switch n.local {
			case "element":
				w.elements[q] = ref
				if sg := n.attrs["substitutionGroup"]; sg != "" {
					h := d.resolve(sg)
					w.subst[h] = append(w.subst[h], q)
				}
			case "complexType":
				w.types[q] = ref
			case "group":
				w.groups[q] = ref
			case "attributeGroup":
				w.attrGroups[q] = ref
			}
		}
	}
	return w
}

// members returns the concrete elements that can stand where q is declared.
func (w *schemaWalker) members(q string, out map[string]bool) {
	if out[q] {
		return
	}
	if decl, ok := w.elements[q]; ok && decl.node.attrs["abstract"] != "true" {
		out[q] = true
	}
	for _, m := range w.subst[q] {
		w.members(m, out)
	}
}

func (w *schemaWalker) edge(parent, child string) {
	if w.edges[parent] == nil {
		w.edges[parent] = map[string]bool{}
	}
	w.edges[parent][child] = true
}

// element handles an <element> particle found in doc, owned by parent.
func (w *schemaWalker) element(doc *xsdDoc, n *xnode, parent string) {
	if ref := n.attrs["ref"]; ref != "" {
		set := map[string]bool{}
		w.members(doc.resolve(ref), set)
		for q := range set {
			decl := w.elements[q]
			w.edge(parent, q)
			w.edgeDecls[parent+">"+q] = append(w.edgeDecls[parent+">"+q], decl.node)
			w.declared(decl.doc, decl.node, q)
		}
		return
	}
	q := "{" + doc.targetNS + "}" + n.attrs["name"]
	w.edge(parent, q)
	w.edgeDecls[parent+">"+q] = append(w.edgeDecls[parent+">"+q], n)
	w.declared(doc, n, q)
}

// declared walks the content model of one element declaration once.
func (w *schemaWalker) declared(doc *xsdDoc, n *xnode, q string) {
	if w.seen[n] {
		return
	}
	w.seen[n] = true
	sink := map[string]bool{}
	w.declAttrs[n] = sink
	if t := n.attrs["type"]; t != "" {
		if ref, ok := w.types[doc.resolve(t)]; ok {
			w.complexType(ref.doc, ref.node, q, sink)
		}
		return
	}
	for _, k := range n.kids {
		if k.local == "complexType" {
			w.complexType(doc, k, q, sink)
			return
		}
	}
	// No type of its own: a member of a substitution group takes the type of
	// the group's head (rdeContact:contact is declared as nothing but that).
	if sg := n.attrs["substitutionGroup"]; sg != "" {
		if head, ok := w.elements[doc.resolve(sg)]; ok {
			w.declaredAs(head.doc, head.node, q, sink)
		}
	}
}

// declaredAs walks head's content model on behalf of owner q.
func (w *schemaWalker) declaredAs(doc *xsdDoc, head *xnode, q string, sink map[string]bool) {
	if t := head.attrs["type"]; t != "" {
		if ref, ok := w.types[doc.resolve(t)]; ok {
			w.complexType(ref.doc, ref.node, q, sink)
		}
		return
	}
	for _, k := range head.kids {
		if k.local == "complexType" {
			w.complexType(doc, k, q, sink)
			return
		}
	}
	if sg := head.attrs["substitutionGroup"]; sg != "" {
		if h, ok := w.elements[doc.resolve(sg)]; ok {
			w.declaredAs(h.doc, h.node, q, sink)
		}
	}
}

func (w *schemaWalker) complexType(doc *xsdDoc, ct *xnode, owner string, sink map[string]bool) {
	for _, k := range ct.kids {
		switch k.local {
		case "sequence", "choice", "all":
			w.particle(doc, k, owner)
		case "group":
			if ref, ok := w.groups[doc.resolve(k.attrs["ref"])]; ok {
				w.particle(ref.doc, ref.node, owner)
			}
		case "attribute":
			w.attribute(k, sink)
		case "attributeGroup":
			if ref, ok := w.attrGroups[doc.resolve(k.attrs["ref"])]; ok {
				w.complexType(ref.doc, ref.node, owner, sink)
			}
		case "anyAttribute":
			w.wildcard[owner] = true
		case "complexContent", "simpleContent":
			for _, d := range k.kids {
				if d.local != "extension" && d.local != "restriction" {
					continue
				}
				if base, ok := w.types[doc.resolve(d.attrs["base"])]; ok && d.local == "extension" {
					w.complexType(base.doc, base.node, owner, sink)
				}
				w.complexType(doc, d, owner, sink)
			}
		}
	}
}

func (w *schemaWalker) attribute(a *xnode, sink map[string]bool) {
	if a.attrs["use"] == "prohibited" {
		return
	}
	name := a.attrs["name"]
	if name == "" {
		_, name, _ = strings.Cut(a.attrs["ref"], ":")
	}
	sink[name] = true
}

func (w *schemaWalker) particle(doc *xsdDoc, n *xnode, owner string) {
	for _, k := range n.kids {
		switch k.local {
		case "element":
			w.element(doc, k, owner)
		case "sequence", "choice", "all":
			w.particle(doc, k, owner)
		case "group":
			if ref, ok := w.groups[doc.resolve(k.attrs["ref"])]; ok {
				w.particle(ref.doc, ref.node, owner)
			}
		case "any":
			w.wildcard[owner] = true
		}
	}
}

// keyOf renders a "{uri}local" QName as the profile's alias:local key.
func keyOf(q string) (string, bool) {
	i := strings.Index(q, "}")
	alias, ok := Alias(q[1:i])
	return alias + ":" + q[i+1:], ok
}

func TestProfileClassifiesEverythingAConformantDepositCanContain(t *testing.T) {
	w := newWalker(t)
	root := "{urn:ietf:params:xml:ns:rde-1.0}deposit"
	w.edge("", root)
	decl, ok := w.elements[root]
	require.True(t, ok)
	w.declared(decl.doc, decl.node, root)
	// Every attribute of the root is declared on a complexType the walker
	// reached; the root element's own is covered by the same pass.

	p := BaselineProfile()
	var gaps []string
	checked := map[[2]string]bool{}

	var visit func(parent, parentKey, q string)
	visit = func(parent, parentKey, q string) {
		if checked[[2]string{parentKey, q}] {
			return
		}
		checked[[2]string{parentKey, q}] = true
		key, known := keyOf(q)
		if !known {
			gaps = append(gaps, "namespace not in the profile: "+q+" (under "+parentKey+")")
			return
		}
		act, ok := p.Action(parentKey, key)
		if !ok {
			gaps = append(gaps, "element: "+parentKey+" > "+key)
			return
		}
		attrs := map[string]bool{}
		for _, d := range w.edgeDecls[parent+">"+q] {
			for a := range w.declAttrs[d] {
				attrs[a] = true
			}
		}
		for a := range attrs {
			if !p.AllowsAttribute(parentKey, key, a) && !p.DropsAttribute(parentKey, key, a) {
				gaps = append(gaps, "attribute: "+parentKey+" > "+key+" @"+a)
			}
		}
		if act.Kind == ActDropSubtree || act.Kind == ActKeepSubtree {
			return // the rewriter never classifies what is inside these
		}
		for c := range w.edges[q] {
			visit(q, key, c)
		}
	}
	visit("", "", root)

	sort.Strings(gaps)
	require.Empty(t, gaps, "the sanitisation profile does not classify content the pinned schemas allow in a deposit:\n  %s", strings.Join(gaps, "\n  "))
}
