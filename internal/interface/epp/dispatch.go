package epp

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
)

// command is one inbound frame being handled. Handlers answer through
// respond, which records the result code for metrics and logs.
type command struct {
	frame   frame
	doc     *etree.Document
	session *session
	svTRID  string
	rw      *epplib.ResponseWriter
	code    int
}

// respond writes the response for this command. msg may be empty for the
// standard text; resData may be nil.
func (c *command) respond(code int, msg string, resData *etree.Element) {
	c.code = code
	doc := responseDocument(code, msg, c.frame.clTRID, c.svTRID, resData)
	_, _ = doc.WriteTo(c.rw) // the connection is gone if this fails; epp-lib reports it on flush
}

// closeAfterResponse ends the session once the response is flushed.
func (c *command) closeAfterResponse() {
	c.session.setState(stateClosing)
	c.rw.CloseAfterWrite()
}

type handlerFunc func(ctx context.Context, cmd *command)

// route is a bound command. requiresLogin is false only for login itself.
type route struct {
	handler       handlerFunc
	requiresLogin bool
}

// routeKey identifies a command: its name, and for object commands the
// object namespace.
type routeKey struct {
	command  string
	objectNS string
}

// rfc5730Commands are the commands RFC 5730 defines. A frame naming one that
// is not bound is answered 2101 (unimplemented); anything else is 2000
// (unknown).
var rfc5730Commands = map[string]bool{
	"login": true, "logout": true, "check": true, "info": true, "poll": true,
	"transfer": true, "create": true, "delete": true, "renew": true, "update": true,
}

func (s *Server) routes() map[routeKey]route {
	return map[routeKey]route{
		{command: "login"}:                     {handler: s.login},
		{command: "logout"}:                    {handler: s.logout, requiresLogin: true},
		{command: "check", objectNS: nsDomain}: {handler: s.domainCheck, requiresLogin: true},
	}
}

// handleCommand is epp-lib's HandleCommand hook: it runs for every frame on
// a session, one at a time. It reads the frame, rejects what fails the
// pre-dispatch checks, enforces the session state machine, and routes.
func (s *Server) handleCommand(ctx context.Context, rw *epplib.ResponseWriter, r io.Reader) {
	start := time.Now()
	cmd := &command{rw: rw, svTRID: s.newID()}

	sessionID, _ := appcontext.ConnectionID(ctx)
	cmd.session = s.sessions.get(sessionID)
	if cmd.session == nil {
		// Only reachable if the connection hook did not register the session.
		s.logger.Error("EPP command on an unregistered session", "connection_id", sessionID)
		rw.CloseAfterWrite()
		return
	}
	cmd.session.commands.Add(1)

	defer func() {
		label := cmd.frame.label()
		s.metrics.commands.WithLabelValues(label, strconv.Itoa(cmd.code)).Inc()
		s.metrics.commandDuration.WithLabelValues(label).Observe(time.Since(start).Seconds())
		s.logger.Debug("EPP command",
			"connection_id", cmd.session.id,
			"command", label,
			"code", cmd.code,
			"cltrid", cmd.frame.clTRID,
			"svtrid", cmd.svTRID,
			"duration_ms", time.Since(start).Milliseconds())
	}()

	body, err := io.ReadAll(r)
	if err != nil {
		// The frame could not be read in full: the connection is failing.
		rw.CloseAfterWrite()
		return
	}
	s.metrics.frameBytes.Observe(float64(len(body)))

	f, err := scanFrame(body, s.cfg.Limits.XMLMaxDepth, s.cfg.Limits.XMLMaxElements)
	if err != nil {
		reason := rejectSyntax
		var fe *frameError
		if errors.As(err, &fe) {
			reason = fe.reason
		}
		s.metrics.framesRejected.WithLabelValues(reason).Inc()
		s.logger.Info("EPP frame rejected", "connection_id", cmd.session.id, "reason", reason, "error", err)
		cmd.respond(epplib.StatusCommandSyntaxError, "", nil)
		return
	}
	cmd.frame = f

	switch f.kind {
	case frameHello:
		cmd.code = 0 // a greeting carries no result code
		_, _ = greetingDocument(time.Now()).WriteTo(rw)
		return
	case frameCommand:
	default:
		cmd.respond(epplib.StatusUnknownCommand, "", nil)
		return
	}

	rt, bound := s.router[routeKey{command: f.command, objectNS: f.objectNS}]
	switch {
	case !bound && rfc5730Commands[f.command]:
		cmd.respond(epplib.StatusUnimplementedCommand, "", nil)
		return
	case !bound:
		cmd.respond(epplib.StatusUnknownCommand, "", nil)
		return
	}

	state := cmd.session.currentState()
	switch {
	case rt.requiresLogin && state != stateAuthenticated:
		cmd.respond(epplib.StatusCommandUseError, "Command use error; log in first", nil)
		return
	case !rt.requiresLogin && state == stateAuthenticated:
		cmd.respond(epplib.StatusCommandUseError, "Command use error; already logged in", nil)
		return
	}

	cmd.doc = etree.NewDocument()
	if err := cmd.doc.ReadFromBytes(body); err != nil {
		// scanFrame accepted it, so this is etree disagreeing with
		// encoding/xml; answer as a syntax error rather than guess.
		s.metrics.framesRejected.WithLabelValues(rejectSyntax).Inc()
		cmd.respond(epplib.StatusCommandSyntaxError, "", nil)
		return
	}
	rt.handler(ctx, cmd)
}

// greeting is epp-lib's Greeting hook, sent once when a session opens.
func (s *Server) greeting(_ context.Context, rw *epplib.ResponseWriter) {
	_, _ = greetingDocument(time.Now()).WriteTo(rw)
}
