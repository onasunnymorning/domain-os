package rdeschema

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// schemaFS is the pinned schema set. It is embedded rather than read from the
// source tree or from the image filesystem so that a worker cannot be deployed
// without the schemas it enforces: the binary either has them or does not
// exist.
//
//go:embed xsd/*.xsd
var schemaFS embed.FS

const (
	xsdNamespace = "http://www.w3.org/2001/XMLSchema"
	// wrapperNamespace is the target namespace of the wrapper schemas. They
	// only import; they declare nothing a deposit can use.
	wrapperNamespace = "urn:domain-os:deposit-schemas"
)

// vocabulary is what the embedded schemas say about themselves: the namespaces
// they define and the names they declare in them. It is what lets a finding
// repeat a name from a schema verbatim while refusing to repeat one that came
// from the deposit — see Vocabulary.Known.
type vocabulary struct {
	namespaces map[string]string // namespace URI -> conventional prefix
	names      map[string]bool   // declared local names, across all namespaces
}

var (
	vocabOnce sync.Once
	vocab     vocabulary
	vocabErr  error
)

func loadVocabulary() (vocabulary, error) {
	vocabOnce.Do(func() {
		v := vocabulary{namespaces: map[string]string{}, names: map[string]bool{}}
		entries, err := fs.ReadDir(schemaFS, "xsd")
		if err != nil {
			vocabErr = fmt.Errorf("rdeschema: read embedded schemas: %w", err)
			return
		}
		for _, e := range entries {
			raw, err := schemaFS.ReadFile("xsd/" + e.Name())
			if err != nil {
				vocabErr = fmt.Errorf("rdeschema: read %s: %w", e.Name(), err)
				return
			}
			if err := v.scan(raw); err != nil {
				vocabErr = fmt.Errorf("rdeschema: scan %s: %w", e.Name(), err)
				return
			}
		}
		vocab = v
	})
	return vocab, vocabErr
}

// scan adds one schema document's target namespace and declared names.
func (v *vocabulary) scan(raw []byte) error {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	first := true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Space != xsdNamespace {
			continue
		}
		if first {
			first = false
			for _, a := range se.Attr {
				if a.Name.Local == "targetNamespace" && a.Value != wrapperNamespace {
					v.namespaces[a.Value] = prefixFor(a.Value)
				}
			}
		}
		switch se.Name.Local {
		case "element", "attribute", "complexType", "simpleType", "group", "attributeGroup":
			for _, a := range se.Attr {
				if a.Name.Local == "name" && a.Name.Space == "" {
					v.names[a.Value] = true
				}
			}
		}
	}
}

var versionSuffix = regexp.MustCompile(`-[0-9]+(\.[0-9]+)*$`)

// prefixFor derives the conventional prefix from a namespace URI of the form
// urn:ietf:params:xml:ns:rdeDomain-1.0, the shape every published schema in the
// set uses, and which is also the prefix the RFCs use in their examples.
func prefixFor(ns string) string {
	seg := ns
	if i := strings.LastIndex(seg, ":"); i >= 0 {
		seg = seg[i+1:]
	}
	return versionSuffix.ReplaceAllString(seg, "")
}

// Namespaces returns the namespace URIs of the pinned schema set, sorted.
func Namespaces() ([]string, error) {
	v, err := loadVocabulary()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(v.namespaces))
	for ns := range v.namespaces {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out, nil
}

// SetDigest identifies the pinned schema set: a SHA-256 over every embedded
// schema's name and content, in name order. It is recorded wherever an operator
// might need to say which schemas a deposit was judged against.
func SetDigest() (string, error) {
	entries, err := fs.ReadDir(schemaFS, "xsd")
	if err != nil {
		return "", fmt.Errorf("rdeschema: read embedded schemas: %w", err)
	}
	h := sha256.New()
	for _, e := range entries { // ReadDir is sorted by filename
		raw, err := schemaFS.ReadFile("xsd/" + e.Name())
		if err != nil {
			return "", fmt.Errorf("rdeschema: read %s: %w", e.Name(), err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", e.Name(), len(raw))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractSchemas writes the embedded set into a fresh private directory and
// returns it. libxml2 reads schemas from files, so a copy on disk is the cost
// of embedding; the copy is schemas only — never deposit content.
func extractSchemas() (dir string, err error) {
	dir, err = os.MkdirTemp("", "rde-schemas-")
	if err != nil {
		return "", fmt.Errorf("rdeschema: create schema directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	entries, err := fs.ReadDir(schemaFS, "xsd")
	if err != nil {
		return "", fmt.Errorf("rdeschema: read embedded schemas: %w", err)
	}
	for _, e := range entries {
		raw, err := schemaFS.ReadFile("xsd/" + e.Name())
		if err != nil {
			return "", fmt.Errorf("rdeschema: read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), raw, 0o400); err != nil {
			return "", fmt.Errorf("rdeschema: write %s: %w", e.Name(), err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, DepositSchemas)); err != nil {
		return "", fmt.Errorf("rdeschema: %s is not in the embedded set: %w", DepositSchemas, err)
	}
	return dir, nil
}
