package schema

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The differential test: the models must agree with xmllint, on the schemas
// pinned from ICANN's RST release, about every frame in testdata/frames and
// every mutation of them. It is what makes the models trustworthy (ADR 0016).

// pinnedSchemas is the entry point for the XSDs pinned from ICANN's RST
// release; see ../testdata/rst/PROVENANCE.md.
const pinnedSchemas = "../testdata/rst/xsd/epp-schemas.xsd"

// testSet is the models under test: every global declaration the server
// validates against.
var testSet = NewSet(Root, Globals...)

var xmllint = sync.OnceValues(func() (string, error) { return exec.LookPath("xmllint") })

// requireXmllint returns xmllint's path. CI installs it and fails without
// it; a developer machine without it skips the test and says so.
func requireXmllint(t testing.TB) string {
	t.Helper()
	bin, err := xmllint()
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("xmllint is required in CI for the schema differential test: %v", err)
		}
		t.Skipf("xmllint not found; the schema differential test needs it")
	}
	return bin
}

// xmllintVerdict validates frame with xmllint. ok is its schema verdict;
// wellFormed is false when libxml2 could not parse the frame at all, in
// which case there is no schema verdict to compare.
func xmllintVerdict(t testing.TB, bin string, frame []byte) (ok, wellFormed bool, out string) {
	t.Helper()
	cmd := exec.Command(bin, "--noout", "--nonet", "--schema", pinnedSchemas, "-") // #nosec G204 -- test-only: fixed arguments, binary from PATH
	cmd.Stdin = bytes.NewReader(frame)
	b, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, true, string(b)
	case errors.As(err, &exit) && exit.ExitCode() == 3: // XMLLINT_ERR_VALID
		return false, true, string(b)
	case errors.As(err, &exit) && exit.ExitCode() == 1: // XMLLINT_ERR_UNCLASS: not well-formed
		return false, false, string(b)
	}
	t.Fatalf("xmllint failed to run: %v\n%s", err, b)
	return false, false, ""
}

func loadCorpus(t testing.TB) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob("testdata/frames/*.xml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	corpus := map[string][]byte{}
	for _, p := range paths {
		b, err := os.ReadFile(p) // #nosec G304 -- test fixture
		require.NoError(t, err)
		corpus[filepath.Base(p)] = b
	}
	return corpus
}

func validateBytes(b []byte) error {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(b); err != nil {
		return err
	}
	return testSet.Validate(doc)
}

func TestCorpusIsValid(t *testing.T) {
	bin := requireXmllint(t)
	for name, frame := range loadCorpus(t) {
		t.Run(name, func(t *testing.T) {
			ok, _, out := xmllintVerdict(t, bin, frame)
			require.True(t, ok, "xmllint rejects a corpus frame:\n%s", out)
			require.NoError(t, validateBytes(frame))
		})
	}
}

// mutant is one mutation of a corpus frame.
type mutant struct {
	name  string
	frame []byte
}

func TestModelsAgreeWithXmllint(t *testing.T) {
	bin := requireXmllint(t)
	var mutants []mutant
	for name, frame := range loadCorpus(t) {
		mutants = append(mutants, mutate(t, name, frame)...)
	}

	type disagreement struct {
		m             mutant
		ours, theirs  bool
		why, lintText string
	}
	var (
		mu       sync.Mutex
		failures []disagreement
		invalid  int
		wg       sync.WaitGroup
		next     = make(chan mutant)
	)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range next {
				theirs, wellFormed, out := xmllintVerdict(t, bin, m.frame)
				if !wellFormed {
					continue
				}
				err := validateBytes(m.frame)
				mu.Lock()
				if !theirs {
					invalid++
				}
				if (err == nil) != theirs {
					why := "accepted"
					if err != nil {
						why = err.Error()
					}
					failures = append(failures, disagreement{m: m, ours: err == nil, theirs: theirs, why: why, lintText: out})
				}
				mu.Unlock()
			}
		}()
	}
	for _, m := range mutants {
		next <- m
	}
	close(next)
	wg.Wait()

	t.Logf("%d mutants, %d invalid per xmllint", len(mutants), invalid)
	require.Greater(t, invalid, len(mutants)/3, "many mutations should break a frame; the mutator has stopped mutating")
	for i, f := range failures {
		if i == 20 {
			t.Errorf("... and %d more", len(failures)-20)
			break
		}
		t.Errorf("%s: models say valid=%v (%s), xmllint says valid=%v\n%s\nframe:\n%s", f.m.name, f.ours, f.why, f.theirs, f.lintText, f.m.frame)
	}
}

// TestMutatorCoversEveryElementButTheRoot pins the mutator's reach: the
// required child of <epp> is removed and duplicated like any other element,
// and only <epp> itself is left in place.
func TestMutatorCoversEveryElementButTheRoot(t *testing.T) {
	names := map[string]bool{}
	for name, frame := range loadCorpus(t) {
		for _, m := range mutate(t, name, frame) {
			names[m.name] = true
		}
	}
	for _, want := range []string{
		"login.xml: #1 <command> removed",
		"login.xml: #1 <command> duplicated",
		"hello.xml: #1 <hello> removed",
		"hello.xml: #1 <hello> duplicated",
	} {
		assert.True(t, names[want], "mutants include %q", want)
	}
	assert.False(t, names["login.xml: #0 <epp> removed"], "the root is never removed: there would be no document")
}

// FuzzModelsAgreeWithXmllint extends the differential test to inputs the
// mutator does not generate. `go test` runs only the seed corpus; run it for
// longer with `go test -fuzz FuzzModelsAgreeWithXmllint ./internal/interface/epp/schema`.
func FuzzModelsAgreeWithXmllint(f *testing.F) {
	for _, frame := range loadCorpus(f) {
		f.Add(frame)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		doc := etree.NewDocument()
		if doc.ReadFromBytes(frame) != nil {
			return
		}
		bin := requireXmllint(t)
		theirs, wellFormed, out := xmllintVerdict(t, bin, frame)
		if !wellFormed {
			return
		}
		if err := testSet.Validate(doc); (err == nil) != theirs {
			t.Fatalf("models say %v, xmllint says valid=%v\n%s\nframe:\n%s", err, theirs, out, frame)
		}
	})
}

// mutationValues replace the text of every leaf element and attribute: the
// boundaries of every length facet in the models, enumeration near-misses,
// numbers around each range, and whitespace the collapse rules must handle.
var mutationValues = []string{
	"", " ", "a", "ab", "abc", "abcde", "abcdef", " abcdef ", "a  b  c",
	strings.Repeat("x", 16), strings.Repeat("x", 17), "\t" + strings.Repeat("x", 16) + "\n",
	strings.Repeat("x", 32), strings.Repeat("x", 33),
	strings.Repeat("x", 64), strings.Repeat("x", 65),
	strings.Repeat("x", 255), strings.Repeat("x", 256),
	"1.0", "1.1", "2.0", "01.0", " 1.0 ", "en", "en-GB", "e1", "english-language", "-en",
	"0", "1", "+1", "99", "100", "65535", "65536", "-1", "y", "m", "Y",
	"true", "false", "TRUE", "all", "none", "del", "sub", "req", "ack", "request", "query",
	"admin", "tech", "billing", "owner",
	"v4", "v6", "v5", "192.0.2.1", "2001:db8::1", "12", "120", "y m",
	"http://h:ab/", "a%zz", "a%2f", "urn:x y", ":x", "a:b", "1a:b", "x/y:z", "http://[::1]/", "?a:b", "#frag",
	"http://u:p@h:80/x", "http://h:/", "http://[::1]:x/", "//h:9", "//h:z", "a://h:1:2/", "//[::1]:", "//[::1]:8", "//u@h:7?q", "a:/h:x",
	"255", "256", "-0", "+5", "2147483647", "2147483648", "-2147483648",
	"AB", "ABC", "abcd", "0g", "QUJD", "QUJDRA==", "QUJD RA==", "QUJDRA=", "Q===", "====", "QQ==\n", "QR==", "QUI=", "QUJ=", "Q=Q=", "QQ=a", "Q Q = =", "!!!!QUJD",
	"1_DOM-APEX", "1-ABCDEFGHI", "-APEX", "a b-APEX", "x\u00e9_1-R\u00e9P",
	"2026-10-16", "2024-02-29", "2026-02-29", "2000-02-29", "1900-02-29", "2026-04-31", "2026-13-01", "2026-00-10", "2026-01-00",
	"0000-01-01", "0001-01-01", "-0001-01-01", "-0004-02-29", "-0005-02-29", "12026-01-01", "02026-01-01", "26-01-01", "2026-1-1",
	"2026-01-01Z", "2026-01-01+14:00", "2026-01-01+14:01", "2026-01-01-12:59", "2026-01-01+15:00", "2026-01-01+01:60", "2026-01-01T00:00:00",
	" 2026-01-01 ", "2026-01-01+1:00",
}

// mutate returns systematic mutations of frame: each element removed,
// duplicated, swapped with its next sibling, renamed and moved to another
// namespace; unexpected children, text and attributes added; each leaf's
// text and each attribute's value replaced with every mutation value.
func mutate(t testing.TB, name string, frame []byte) []mutant {
	t.Helper()
	base := etree.NewDocument()
	require.NoError(t, base.ReadFromBytes(frame))

	var out []mutant
	emit := func(what string, doc *etree.Document) {
		b, err := doc.WriteToBytes()
		require.NoError(t, err)
		out = append(out, mutant{name: name + ": " + what, frame: b})
	}
	// Each mutation works on a fresh copy, addressing the element by its
	// position in a depth-first walk.
	count := len(walk(base.Root()))
	for i := range count {
		with := func(fn func(doc *etree.Document, el *etree.Element) string) {
			doc := base.Copy()
			el := walk(doc.Root())[i]
			if what := fn(doc, el); what != "" {
				emit(fmt.Sprintf("#%d <%s> %s", i, el.Tag, what), doc)
			}
		}
		with(func(_ *etree.Document, el *etree.Element) string {
			if isRoot(el) {
				return ""
			}
			el.Parent().RemoveChild(el)
			return "removed"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			if isRoot(el) {
				return ""
			}
			el.Parent().InsertChildAt(el.Index()+1, el.Copy())
			return "duplicated"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			if isRoot(el) {
				return ""
			}
			p := el.Parent()
			sibs := p.ChildElements()
			for j, s := range sibs {
				if s == el && j+1 < len(sibs) {
					p.RemoveChild(el)
					p.InsertChildAt(sibs[j+1].Index()+1, el)
					return "swapped with the next sibling"
				}
			}
			return ""
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			el.Tag += "x"
			return "renamed"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			el.Space = "zz"
			el.CreateAttr("xmlns:zz", "urn:example:unknown")
			return "moved to an unknown namespace"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			el.InsertChildAt(0, etree.NewElement("unexpected"))
			return "given an unexpected first child in its default namespace"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			kid := el.CreateElement("zz:unexpected")
			kid.CreateAttr("xmlns:zz", "urn:example:unknown")
			return "given an unexpected last child in an unknown namespace"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			if len(el.ChildElements()) == 0 {
				return ""
			}
			el.InsertChildAt(0, etree.NewText("text"))
			return "given text among its children"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			el.CreateAttr("unexpected", "1")
			return "given an unexpected attribute"
		})
		with(func(_ *etree.Document, el *etree.Element) string {
			el.CreateAttr("xml:lang", "en")
			return "given xml:lang"
		})
		for ai, a := range walk(base.Root())[i].Attr {
			if a.Space == "xmlns" || a.Key == "xmlns" {
				continue
			}
			with(func(_ *etree.Document, el *etree.Element) string {
				el.RemoveAttr(el.Attr[ai].FullKey())
				return "lost attribute " + a.Key
			})
			for vi, v := range mutationValues {
				with(func(_ *etree.Document, el *etree.Element) string {
					el.Attr[ai].Value = v
					return fmt.Sprintf("attribute %s set to value #%d", a.Key, vi)
				})
			}
		}
		if len(walk(base.Root())[i].ChildElements()) == 0 {
			for vi, v := range mutationValues {
				with(func(_ *etree.Document, el *etree.Element) string {
					el.SetText(v)
					return fmt.Sprintf("text set to value #%d", vi)
				})
			}
		}
	}
	return out
}

// isRoot reports whether el is the document element. In etree the root's
// parent is the Document's own element, not nil, so the root is the
// element whose parent has no parent. Every other element, including
// <command> and <hello> under <epp>, is removed, duplicated and swapped.
func isRoot(el *etree.Element) bool {
	return el.Parent() == nil || el.Parent().Parent() == nil
}

// walk lists el and its descendants depth first.
func walk(el *etree.Element) []*etree.Element {
	out := []*etree.Element{el}
	for _, kid := range el.ChildElements() {
		out = append(out, walk(kid)...)
	}
	return out
}
