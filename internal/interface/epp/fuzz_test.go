package epp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Fuzz tests for what reads untrusted bytes (docs/epp/ARCHITECTURE.md §4).
// The seeds are the schema corpus (schema/testdata/frames) and a few frames
// made to be rejected; `make fuzz-epp` runs each for FUZZTIME.

// fuzzSeeds adds the corpus frames and some hostile ones.
func fuzzSeeds(f *testing.F) {
	f.Helper()
	files, err := filepath.Glob("schema/testdata/frames/*.xml")
	require.NoError(f, err)
	require.NotEmpty(f, files)
	for _, name := range files {
		b, err := os.ReadFile(name)
		require.NoError(f, err)
		f.Add(b)
	}
	for _, s := range []string{
		``,
		`<epp/>`,
		`<?xml version="1.0"?><!DOCTYPE epp [<!ENTITY x "y">]><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"/>`,
		`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><clTRID>ab</clTRID></command></epp>`,
		`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>a.test</domain:name></domain:check></check></command></epp><epp/>`,
		`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`,
		`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><delete><host:info xmlns:host="urn:ietf:params:xml:ns:host-1.0"/></delete></command></epp>`,
	} {
		f.Add([]byte(s))
	}
}

// depthAndElements recounts what scanFrame bounds, with its own decoder.
func depthAndElements(b []byte) (maxDepth, elements int, err error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	dec.Strict = true
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return maxDepth, elements, nil
		}
		if err != nil {
			return 0, 0, err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			elements++
			maxDepth = max(maxDepth, depth)
		case xml.EndElement:
			depth--
		}
	}
}

// A frame scanFrame accepts is well-formed XML within its caps, with one
// <epp> root and a clTRID of 3 to 64 characters or none; whatever it is
// given, it does not panic.
func FuzzScanFrame(f *testing.F) {
	fuzzSeeds(f)
	const maxDepth, maxElements = 12, 200
	f.Fuzz(func(t *testing.T, b []byte) {
		fr, err := scanFrame(b, maxDepth, maxElements)
		if err != nil {
			var fe *frameError
			if !errors.As(err, &fe) {
				t.Fatalf("a refusal that is not a frameError: %v", err)
			}
			return
		}
		depth, elements, err := depthAndElements(b)
		if err != nil {
			t.Fatalf("accepted a frame encoding/xml cannot read: %v", err)
		}
		if depth > maxDepth || elements > maxElements {
			t.Fatalf("accepted depth %d, %d elements, over %d/%d", depth, elements, maxDepth, maxElements)
		}
		if n := len([]rune(fr.clTRID)); n != 0 && (n < 3 || n > maxClTRIDLen) {
			t.Fatalf("accepted a clTRID of %d characters", n)
		}
		if fr.kind != frameCommand && (fr.command != "" || fr.objectNS != "") {
			t.Fatalf("a %v frame with command %q", fr.kind, fr.command)
		}
		_ = fr.label()
	})
}

// Every frame a logged-in session sends gets an answer and does not panic
// the server, from the frame scanner through the handlers.
func FuzzDispatch(f *testing.F) {
	fuzzSeeds(f)
	creds := newTestCredentials(f)
	open := entities.DefaultEPPPolicyLimits()
	open.Rates = entities.EPPCommandRates{
		Check: entities.EPPRate{PerSecond: 1_000_000, Burst: 1_000_000}, Info: entities.EPPRate{PerSecond: 1_000_000, Burst: 1_000_000},
		Transform: entities.EPPRate{PerSecond: 1_000_000, Burst: 1_000_000}, Poll: entities.EPPRate{PerSecond: 1_000_000, Burst: 1_000_000},
	}
	open.FailedCreates = entities.EPPFailedCreateBudget{Failures: 1_000_000, RatioPercent: 100}
	creds.policies.assign(testRegistrar, "fuzz", open)
	ts := startServerWith(f, testConfig(), listenLoopback(f), creds)
	c := dial(f, ts.addr)
	requireCode(f, c.roundTrip(objectsLoginXML(testLogin, testPassword)), "1000")
	var sess *session
	require.Eventually(f, func() bool {
		for _, s := range ts.sessions.all() {
			if s.auth.Load() != nil {
				sess = s
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)

	f.Fuzz(func(t *testing.T, b []byte) {
		cmd := &command{session: sess, svTRID: "FUZZ-1", received: time.Now()}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ts.dispatch(ctx, cmd, b)
		if cmd.response == nil {
			t.Fatalf("no answer to %q", b)
		}
		// Only <logout> (1500) and the policy's closes (2502, or 2400 for a
		// held command whose session ended) end a logged-in session.
		if cmd.closing && cmd.code != 1500 && cmd.code != 2502 && cmd.code != 2400 {
			t.Fatalf("the session would close for %d", cmd.code)
		}
	})
}
