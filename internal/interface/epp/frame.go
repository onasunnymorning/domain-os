package epp

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	nsEPP     = "urn:ietf:params:xml:ns:epp-1.0"
	nsDomain  = "urn:ietf:params:xml:ns:domain-1.0"
	nsContact = "urn:ietf:params:xml:ns:contact-1.0"
	nsHost    = "urn:ietf:params:xml:ns:host-1.0"

	// RFC 5730 bounds a clTRID to 3..64 characters.
	maxClTRIDLen = 64
)

// Reasons a frame is rejected before dispatch, used as metric labels.
const (
	rejectSyntax   = "syntax"
	rejectDoctype  = "doctype"
	rejectDepth    = "depth"
	rejectElements = "elements"
	rejectClTRID   = "cltrid"
	rejectSchema   = "schema"
)

// frameError is a frame rejected while it is scanned. Every one is answered with
// 2001 (command syntax error); reason says which check failed.
type frameError struct {
	reason string
	err    error
}

func (e *frameError) Error() string { return e.reason + ": " + e.err.Error() }
func (e *frameError) Unwrap() error { return e.err }

func rejectFrame(reason string, format string, args ...any) *frameError {
	return &frameError{reason: reason, err: fmt.Errorf(format, args...)}
}

type frameKind int

const (
	frameUnknown frameKind = iota
	frameHello
	frameCommand
)

// frame is what the dispatcher needs to know about an inbound EPP frame
// before parsing it in full.
type frame struct {
	kind     frameKind
	command  string // <command>'s first child: login, check, info, ...
	objectNS string // namespace of the command's first child, for object commands
	clTRID   string
}

// label names the frame for metrics and logs: "hello", "login",
// "domain:check", and so on. Object commands outside the three standard
// mappings are "other:<command>", which keeps label cardinality bounded.
func (f frame) label() string {
	switch f.kind {
	case frameHello:
		return "hello"
	case frameCommand:
		switch f.objectNS {
		case "":
			return f.command
		case nsDomain:
			return "domain:" + f.command
		case nsContact:
			return "contact:" + f.command
		case nsHost:
			return "host:" + f.command
		default:
			return "other:" + f.command
		}
	default:
		return "unknown"
	}
}

// scanFrame reads an EPP frame as a token stream, enforcing the depth and
// element caps and rejecting any DTD, and records what the dispatcher needs.
// It never builds a tree, so an oversized or hostile document costs no more
// than reading it once.
func scanFrame(b []byte, maxDepth, maxElements int) (frame, error) {
	var (
		f        frame
		depth    int
		elements int
		path     [4]string // local names at depths 1..4, enough to classify
		inClTRID bool
		clTRID   strings.Builder
		sawRoot  bool
	)

	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return frame{}, rejectFrame(rejectSyntax, "%w", err)
		}

		switch t := tok.(type) {
		case xml.Directive:
			return frame{}, rejectFrame(rejectDoctype, "DTDs and other <!...> declarations are not accepted")

		case xml.StartElement:
			depth++
			elements++
			if depth > maxDepth {
				return frame{}, rejectFrame(rejectDepth, "nesting exceeds %d levels", maxDepth)
			}
			if elements > maxElements {
				return frame{}, rejectFrame(rejectElements, "more than %d elements", maxElements)
			}
			if depth <= len(path) {
				path[depth-1] = t.Name.Local
			}

			switch depth {
			case 1:
				if sawRoot {
					return frame{}, rejectFrame(rejectSyntax, "more than one root element")
				}
				sawRoot = true
				if t.Name.Space != nsEPP || t.Name.Local != "epp" {
					return frame{}, rejectFrame(rejectSyntax, "root element is not <epp> in %s", nsEPP)
				}
			case 2:
				if f.kind != frameUnknown || t.Name.Space != nsEPP {
					break
				}
				switch t.Name.Local {
				case "hello":
					f.kind = frameHello
				case "command":
					f.kind = frameCommand
				}
			case 3:
				if f.kind != frameCommand || path[1] != "command" || t.Name.Space != nsEPP {
					break
				}
				switch t.Name.Local {
				case "clTRID":
					inClTRID = true
				case "extension":
				default:
					if f.command == "" {
						f.command = t.Name.Local
					}
				}
			case 4:
				if f.kind == frameCommand && path[1] == "command" && path[2] == f.command && f.objectNS == "" && t.Name.Space != nsEPP {
					f.objectNS = t.Name.Space
				}
			}

		case xml.EndElement:
			if depth == 3 && inClTRID {
				inClTRID = false
			}
			depth--

		case xml.CharData:
			if inClTRID {
				clTRID.Write(t)
				if clTRID.Len() > maxClTRIDLen*4 { // whitespace allowance before trimming
					return frame{}, rejectFrame(rejectClTRID, "clTRID longer than %d characters", maxClTRIDLen)
				}
			}
		}
	}

	if !sawRoot {
		return frame{}, rejectFrame(rejectSyntax, "no root element")
	}
	f.clTRID = strings.TrimSpace(clTRID.String())
	if n := len([]rune(f.clTRID)); n > maxClTRIDLen || (n > 0 && n < 3) {
		return frame{}, rejectFrame(rejectClTRID, "clTRID must be 3 to %d characters", maxClTRIDLen)
	}
	return f, nil
}
