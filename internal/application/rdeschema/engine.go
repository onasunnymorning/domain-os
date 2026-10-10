package rdeschema

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Engine validates one XML document, supplied as a stream, against the pinned
// deposit schemas.
type Engine interface {
	// Start begins a check. It fails — with an error wrapping ErrUnavailable —
	// when the engine cannot check anything; a caller must treat that as "no
	// decision", never as "no problem".
	Start(ctx context.Context) (Session, error)
}

// Session is one document's check. The document is written to it as it is
// produced; Finish ends the document and returns the verdict.
//
// Write never fails: a session that can no longer accept bytes discards them
// and reports why at Finish, so a caller that is also doing other work on the
// same stream is never stopped by the schema check.
type Session interface {
	io.Writer
	// Finish declares the document complete and returns the verdict.
	Finish() Verdict
	// Abort ends the check and releases its resources. It is the call for a
	// document that was not read to its end, which cannot be judged. The one
	// thing it can still report is ConclusionRejected — the document was
	// refused on its first bytes, whatever came after — and every other
	// result is ConclusionUnavailable.
	Abort() Verdict
}

// ErrUnavailable marks an engine that cannot validate: not installed, schemas
// that do not compile, a failed self-test.
var ErrUnavailable = errors.New("schema validation engine unavailable")

// Conclusion is the engine's decision about a document.
type Conclusion string

const (
	// ConclusionValid: the document is well-formed and conforms to the schemas.
	// It is the only conclusion that lets a deposit pass.
	ConclusionValid Conclusion = "valid"
	// ConclusionInvalid: the document is well-formed and breaks the schemas.
	ConclusionInvalid Conclusion = "invalid"
	// ConclusionNotWellFormed: libxml2 could not parse the document.
	ConclusionNotWellFormed Conclusion = "not-well-formed"
	// ConclusionRejected: the document was refused before it reached libxml2;
	// see Verdict.Reason.
	ConclusionRejected Conclusion = "rejected"
	// ConclusionUnavailable: the engine failed to reach a decision. It says
	// nothing about the document.
	ConclusionUnavailable Conclusion = "unavailable"
)

// Verdict is what a Session concludes.
type Verdict struct {
	Conclusion Conclusion
	// Reason is set when Conclusion is ConclusionRejected.
	Reason RejectReason
	// Violations are the first violations found, up to Config.MaxRetained.
	Violations []Violation
	// Total is how many violations were counted, which is more than
	// len(Violations) when the list was capped.
	Total int
	// Stopped is true when checking was halted after Config.StopAfter
	// violations: Total is then a lower bound and the rest of the document was
	// not checked.
	Stopped bool
	// ParseLine is the line libxml2 gave up at, for ConclusionNotWellFormed.
	ParseLine int
	// Detail explains ConclusionUnavailable in constant words and numbers.
	Detail string
}

// Config tunes an XMLLint engine. The zero value is usable.
type Config struct {
	// Binary is the xmllint executable: a path, or a name looked up on PATH.
	// Empty means "xmllint".
	Binary string
	// MaxRetained bounds how many violations a Verdict carries. Default 1000.
	MaxRetained int
	// StopAfter halts checking once this many violations have been counted.
	// The deposit has failed by then; reading further only costs time. Default
	// 10000.
	StopAfter int
}

const (
	defaultMaxRetained = 1000
	defaultStopAfter   = 10000
	selfTestTimeout    = 30 * time.Second
)

// XMLLint is an Engine that drives libxml2's xmllint in streaming mode.
//
// xmllint --stream validates with a bounded window — it holds the current
// element and its ancestors, not the document — so a deposit of any size is
// checked in constant memory, at libxml2's speed, while it is still being
// decrypted and unpacked. The document is piped through stdin and never
// touches disk.
//
// A process is the chosen boundary rather than a library binding because the
// worker is a static, cross-compiled binary (CGO_ENABLED=0); the cost is that
// the image must carry xmllint, which the self-test below and the image's own
// build step both enforce.
//
// What xmllint does not do for us, this type does: it resolves external
// entities from a document's DOCTYPE with no option to stop it, so the prolog
// is inspected first and a document with a DTD never reaches it (see
// scanProlog). Everything else is configured out — no network, no catalogs, an
// empty environment, schemas from the embedded copy only (xsi:schemaLocation
// hints are ignored by libxml2 when a schema is supplied, which the tests
// pin).
type XMLLint struct {
	cfg     Config
	bin     string
	dir     string
	vocab   vocabulary
	version string
	err     error
}

// NewXMLLint builds the engine and proves it works: it extracts the schemas,
// finds the binary and validates a known-good and a known-bad document with
// it. It never returns nil. When anything fails the returned engine is
// unavailable — Err reports why and every Start fails with ErrUnavailable — so
// the caller wires one value in either case and a broken install becomes an
// ERROR on every run rather than a skipped check.
func NewXMLLint(cfg Config) *XMLLint {
	x := &XMLLint{cfg: cfg}
	if x.cfg.MaxRetained <= 0 {
		x.cfg.MaxRetained = defaultMaxRetained
	}
	if x.cfg.StopAfter <= 0 {
		x.cfg.StopAfter = defaultStopAfter
	}
	x.err = x.init()
	if x.err != nil && x.dir != "" {
		_ = os.RemoveAll(x.dir)
		x.dir = ""
	}
	return x
}

func (x *XMLLint) init() error {
	var err error
	if x.vocab, err = loadVocabulary(); err != nil {
		return err
	}
	name := x.cfg.Binary
	if name == "" {
		name = "xmllint"
	}
	if x.bin, err = exec.LookPath(name); err != nil {
		return fmt.Errorf("xmllint not found: %w", err)
	}
	if x.dir, err = extractSchemas(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), selfTestTimeout)
	defer cancel()
	x.version = x.probeVersion(ctx)
	return x.selfTest(ctx)
}

// Err reports why the engine is unavailable, or nil when it is healthy.
func (x *XMLLint) Err() error { return x.err }

// Description names the binary and library version in use, for a startup log.
func (x *XMLLint) Description() string {
	if x.err != nil {
		return "unavailable"
	}
	return x.bin + " (" + x.version + ")"
}

// Close removes the extracted schema directory.
func (x *XMLLint) Close() error {
	if x.dir == "" {
		return nil
	}
	err := os.RemoveAll(x.dir)
	x.dir = ""
	return err
}

func (x *XMLLint) probeVersion(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, x.bin, "--version") //nolint:gosec // binary resolved by LookPath at construction, arguments are fixed
	cmd.Env = []string{"LC_ALL=C"}
	out, _ := cmd.CombinedOutput()
	line, _, _ := strings.Cut(string(out), "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "version unknown"
	}
	return cleanVersion(line)
}

func cleanVersion(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= ' ' && r < 0x7f {
			b.WriteRune(r)
		}
	}
	if b.Len() > 80 {
		return b.String()[:80]
	}
	return b.String()
}

const (
	selfTestGood = `<?xml version="1.0" encoding="UTF-8"?>
<rde:deposit xmlns:rde="urn:ietf:params:xml:ns:rde-1.0" type="FULL" id="1">
  <rde:watermark>2026-01-01T00:00:00Z</rde:watermark>
  <rde:rdeMenu><rde:version>1.0</rde:version><rde:objURI>urn:ietf:params:xml:ns:rdeHeader-1.0</rde:objURI></rde:rdeMenu>
</rde:deposit>`
	selfTestBad = `<?xml version="1.0" encoding="UTF-8"?>
<rde:deposit xmlns:rde="urn:ietf:params:xml:ns:rde-1.0" type="FULL" id="1">
  <rde:watermark>2026-01-01T00:00:00Z</rde:watermark>
</rde:deposit>`
)

// selfTest proves the installed engine accepts a valid document and rejects an
// invalid one. A binary that exits 0 for everything, or a schema set that does
// not compile, is caught here and not on a customer's deposit.
func (x *XMLLint) selfTest(ctx context.Context) error {
	good, err := x.check(ctx, selfTestGood)
	if err != nil {
		return fmt.Errorf("self-test: %w", err)
	}
	if good.Conclusion != ConclusionValid {
		return fmt.Errorf("self-test: a valid document was judged %s (%s)", good.Conclusion, good.Detail)
	}
	bad, err := x.check(ctx, selfTestBad)
	if err != nil {
		return fmt.Errorf("self-test: %w", err)
	}
	if bad.Conclusion != ConclusionInvalid || len(bad.Violations) == 0 {
		return fmt.Errorf("self-test: an invalid document was judged %s (%s)", bad.Conclusion, bad.Detail)
	}
	return nil
}

func (x *XMLLint) check(ctx context.Context, doc string) (Verdict, error) {
	s, err := x.start(ctx)
	if err != nil {
		return Verdict{}, err
	}
	_, _ = s.Write([]byte(doc))
	return s.Finish(), nil
}

// Start implements Engine.
func (x *XMLLint) Start(ctx context.Context) (Session, error) {
	if x.err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, x.err)
	}
	s, err := x.start(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return s, nil
}

func (x *XMLLint) start(ctx context.Context) (*session, error) {
	cmd := exec.CommandContext(ctx, x.bin, //nolint:gosec // binary resolved by LookPath at construction, arguments are fixed
		"--stream", "--noout", "--nonet", "--nocatalogs",
		"--schema", DepositSchemas, "-")
	// The schemas are found relative to the directory they were written to,
	// and the process is given nothing else: no inherited environment (which
	// is where libxml2 looks for catalogs) and nothing to read but stdin.
	cmd.Dir = x.dir
	cmd.Env = []string{"LC_ALL=C"}
	cmd.WaitDelay = 5 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	s := &session{x: x, ctx: ctx, cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go s.scan(stderr)
	return s, nil
}

// session is one xmllint process and the document flowing through it.
type session struct {
	x     *XMLLint
	ctx   context.Context
	cmd   *exec.Cmd
	stdin io.WriteCloser

	// prolog holds the document's opening bytes until scanProlog has decided
	// them. Nothing is sent to xmllint before that.
	prolog      []byte
	prologDone  bool
	rejected    RejectReason
	writeFailed bool

	// Written by scan, read only after done is closed.
	done      chan struct{}
	retained  []Violation
	total     int
	stopped   bool
	parseFail bool
	parseLine int

	once    sync.Once
	verdict Verdict
}

// Write implements io.Writer. It never returns an error.
func (s *session) Write(p []byte) (int, error) {
	n := len(p)
	if s.rejected != "" || s.writeFailed {
		return n, nil
	}
	if s.prologDone {
		s.forward(p)
		return n, nil
	}
	s.prolog = append(s.prolog, p...)
	switch status, why := scanProlog(s.prolog, false); status {
	case prologNeedMore:
	case prologRejected:
		s.rejected = why
		s.prolog = nil
		s.kill()
	case prologOK:
		s.prologDone = true
		buf := s.prolog
		s.prolog = nil
		s.forward(buf)
	}
	return n, nil
}

func (s *session) forward(p []byte) {
	if _, err := s.stdin.Write(p); err != nil {
		s.writeFailed = true
	}
}

func (s *session) kill() {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

// Abort implements Session.
func (s *session) Abort() Verdict {
	s.once.Do(func() {
		s.kill()
		_ = s.stdin.Close()
		<-s.done
		_ = s.cmd.Wait()
		s.verdict = Verdict{Conclusion: ConclusionUnavailable, Detail: "check aborted"}
		if s.rejected != "" {
			s.verdict = Verdict{Conclusion: ConclusionRejected, Reason: s.rejected}
		}
	})
	return s.verdict
}

// Finish implements Session.
func (s *session) Finish() Verdict {
	s.once.Do(func() { s.verdict = s.finish() })
	return s.verdict
}

func (s *session) finish() Verdict {
	if s.rejected == "" && !s.prologDone {
		// The document ended while its prolog was still undecided.
		status, why := scanProlog(s.prolog, true)
		if status == prologRejected {
			s.rejected = why
			s.kill()
		} else {
			s.prologDone = true
			s.forward(s.prolog)
		}
		s.prolog = nil
	}
	_ = s.stdin.Close()
	<-s.done
	waitErr := s.cmd.Wait()

	if s.rejected != "" {
		return Verdict{Conclusion: ConclusionRejected, Reason: s.rejected}
	}
	if s.ctx.Err() != nil {
		return Verdict{Conclusion: ConclusionUnavailable, Detail: "validation deadline reached"}
	}
	base := Verdict{Violations: s.retained, Total: s.total, Stopped: s.stopped, ParseLine: s.parseLine}

	if s.stopped {
		// We killed it ourselves; the exit status means nothing, and the
		// violations are what decided the document.
		base.Conclusion = ConclusionInvalid
		return base
	}
	exit := 0
	var ee *exec.ExitError
	switch {
	case waitErr == nil:
	case errors.As(waitErr, &ee):
		exit = ee.ExitCode() // -1 when it was killed by a signal
	default:
		base.Conclusion = ConclusionUnavailable
		base.Detail = "schema engine could not be waited on"
		return base
	}

	switch {
	case s.writeFailed && exit == 0:
		// It stopped reading and still reported success: it cannot have seen
		// the whole document.
		base.Conclusion = ConclusionUnavailable
		base.Detail = "schema engine stopped reading the document early"
	case exit == 0 && s.total == 0 && !s.parseFail:
		base.Conclusion = ConclusionValid
	case exit == 3 || (exit == 0 && s.total > 0):
		base.Conclusion = ConclusionInvalid
		if s.total == 0 {
			// It says the document is invalid but nothing it printed was
			// recognised. Still a failure of the document, with nothing
			// specific to say.
			base.Violations = []Violation{{Class: ClassOther}}
			base.Total = 1
		}
	case exit == 1 && s.parseFail:
		base.Conclusion = ConclusionNotWellFormed
	default:
		base.Conclusion = ConclusionUnavailable
		base.Detail = "schema engine exited with status " + strconv.Itoa(exit)
		if exit == 5 {
			base.Detail = "schema set failed to compile (exit status 5)"
		}
		if exit < 0 {
			base.Detail = "schema engine was terminated by a signal"
		}
	}
	return base
}

// scan reads xmllint's stderr to the end, classifying each line. It holds at
// most one line at a time and never more than Config.MaxRetained violations,
// whatever the document does.
func (s *session) scan(r io.Reader) {
	defer close(s.done)
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		chunk, err := br.ReadSlice('\n')
		// Copied before the next read, which reuses the buffer.
		line := string(chunk)
		// A line longer than the buffer is a long value echoed in a message.
		// Its head is all that is ever read; the rest is dropped.
		for errors.Is(err, bufio.ErrBufferFull) {
			_, err = br.ReadSlice('\n')
		}
		if line != "" {
			s.take(line)
		}
		if err != nil {
			return
		}
	}
}

func (s *session) take(line string) {
	if s.stopped {
		// Output that was already in flight when the process was killed. The
		// count is a lower bound by definition once checking has stopped.
		return
	}
	v, kind := parseLine(line, s.x.vocab)
	switch kind {
	case lineViolation:
		s.total++
		if len(s.retained) < s.x.cfg.MaxRetained {
			s.retained = append(s.retained, v)
		}
		if s.total >= s.x.cfg.StopAfter && !s.stopped {
			s.stopped = true
			s.kill()
		}
	case lineParseError:
		if !s.parseFail || s.parseLine == 0 {
			s.parseLine = v.Line
		}
		s.parseFail = true
	case lineParseFail:
		s.parseFail = true
	}
}
