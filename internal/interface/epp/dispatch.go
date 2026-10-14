package epp

import (
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/interface/epp/schema"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// commandTimeout bounds the registry calls one command makes, so a stalled
// database cannot hold a session forever.
const commandTimeout = 30 * time.Second

// clientSchema is what every command is validated against before dispatch
// (ADR 0016): the EPP envelope, and the object commands that are bound.
var clientSchema = schema.NewSet(schema.Root, schema.Globals...)

// command is one inbound frame being handled. Handlers answer through
// respond. The dispatcher then records the command in the audit log and
// writes the response: a transform's record is written before its response
// is sent.
type command struct {
	frame    frame
	doc      *etree.Document
	session  *session
	svTRID   string
	received time.Time

	code     int
	response *etree.Document
	closing  bool
	// handled is set once the command's handler has run: only then can a
	// transform have changed anything.
	handled bool
	// objects are the object identifiers the command names, for the audit
	// log.
	objects []string
	// attemptedLogin is the login ID a <login> presented, for the audit log
	// of a login that did not succeed.
	attemptedLogin string
}

// respond sets the response to this command. msg may be empty for the
// standard text; resData may be nil. Extensions that are nil are left out.
func (c *command) respond(code int, msg string, resData *etree.Element, extensions ...*etree.Element) {
	c.code = code
	var exts []*etree.Element
	for _, e := range extensions {
		if e != nil {
			exts = append(exts, e)
		}
	}
	c.response = responseDocument(code, msg, c.frame.clTRID, c.svTRID, resData, exts...)
}

// closeAfterResponse ends the session once the response is written.
func (c *command) closeAfterResponse() {
	c.session.setState(stateClosing)
	c.closing = true
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
		{command: "login"}:                      {handler: s.login},
		{command: "logout"}:                     {handler: s.logout, requiresLogin: true},
		{command: "check", objectNS: nsDomain}:  {handler: s.domainCheck, requiresLogin: true},
		{command: "info", objectNS: nsDomain}:   {handler: s.domainInfo, requiresLogin: true},
		{command: "create", objectNS: nsDomain}: {handler: s.domainCreate, requiresLogin: true},
	}
}

// handleCommand is epp-lib's HandleCommand hook: it runs for every frame on
// a session, one at a time. The pipeline is: read and scan the frame
// (size, DTD, depth and element caps), parse it, route it, enforce the
// session state machine and the services negotiated at login, validate it
// against the schema, ask the policy, run the handler; then audit and
// respond.
func (s *Server) handleCommand(ctx context.Context, rw *epplib.ResponseWriter, r io.Reader) {
	cmd := &command{svTRID: s.newID(), received: time.Now()}

	sessionID, _ := appcontext.ConnectionID(ctx)
	cmd.session = s.sessions.get(sessionID)
	if cmd.session == nil {
		// Only reachable if the connection hook did not register the session.
		s.logger.Error("EPP command on an unregistered session", "connection_id", sessionID)
		rw.CloseAfterWrite()
		return
	}
	cmd.session.commands.Add(1)
	cmd.session.lastCommandAt.Store(cmd.received.UnixNano())

	body, err := io.ReadAll(r)
	if err != nil {
		// The frame could not be read in full: the connection is failing.
		rw.CloseAfterWrite()
		return
	}
	s.metrics.frameBytes.Observe(float64(len(body)))
	defer s.finish(ctx, rw, cmd)
	s.dispatch(ctx, cmd, body)
}

func (s *Server) dispatch(ctx context.Context, cmd *command, body []byte) {
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

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		// scanFrame accepted it, so this is etree disagreeing with
		// encoding/xml; answer as a syntax error rather than guess.
		s.metrics.framesRejected.WithLabelValues(rejectSyntax).Inc()
		cmd.respond(epplib.StatusCommandSyntaxError, "", nil)
		return
	}
	cmd.doc = doc

	switch f.kind {
	case frameHello:
		if err := clientSchema.Validate(doc); err != nil {
			s.rejectSchema(cmd, err)
			return
		}
		cmd.response = greetingDocument(time.Now())
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

	if !s.servicesAllowed(cmd, rt) {
		return
	}
	if err := clientSchema.Validate(doc); err != nil {
		s.rejectSchema(cmd, err)
		return
	}
	cmd.describe()
	if !s.applyPolicy(ctx, cmd) {
		return
	}
	cmd.handled = true
	rt.handler(ctx, cmd)
}

// servicesAllowed answers a command that uses an object or extension the
// session did not log in with, and reports whether it may go on. An object
// is answered 2307 (unimplemented object service) and an extension 2103
// (unimplemented extension). A <login> may carry only extensions the server
// offers.
func (s *Server) servicesAllowed(cmd *command, rt route) bool {
	objects, extensions := supportedObjects, supportedExtensions
	if rt.requiresLogin {
		objects, extensions = cmd.session.objects, cmd.session.extensions
	}
	if cmd.frame.objectNS != "" && !slices.Contains(objects, cmd.frame.objectNS) {
		cmd.respond(epplib.StatusUnimplementedObjectService, "Unimplemented object service; "+cmd.frame.objectNS+" was not requested at login", nil)
		return false
	}
	if ext := cmd.doc.FindElement("/epp/command/extension"); ext != nil {
		for _, el := range ext.ChildElements() {
			if ns := el.NamespaceURI(); !slices.Contains(extensions, ns) {
				cmd.respond(epplib.StatusUnimplementedExtension, "Unimplemented extension; "+ns+" was not requested at login", nil)
				return false
			}
		}
	}
	return true
}

// rejectSchema answers a frame that fails schema validation: 2001, with the
// element and the rule it broke, but never the value.
func (s *Server) rejectSchema(cmd *command, err error) {
	s.metrics.framesRejected.WithLabelValues(rejectSchema).Inc()
	s.logger.Info("EPP frame rejected", "connection_id", cmd.session.id, "reason", rejectSchema, "error", err)
	cmd.respond(epplib.StatusCommandSyntaxError, "Command syntax error; "+err.Error(), nil)
}

// finish records the command in the audit log and the metrics, then writes
// its response.
func (s *Server) finish(ctx context.Context, rw *epplib.ResponseWriter, cmd *command) {
	var response []byte
	if cmd.response != nil {
		var err error
		if response, err = cmd.response.WriteToBytes(); err != nil {
			s.logger.Error("EPP response not serialised", "connection_id", cmd.session.id, "svtrid", cmd.svTRID, "error", err)
			response = nil
			cmd.closing = true
		}
	}
	s.audit(ctx, cmd)

	label := cmd.frame.label()
	elapsed := time.Since(cmd.received)
	s.metrics.commands.WithLabelValues(cmd.session.registrar(), label, strconv.Itoa(cmd.code)).Inc()
	s.metrics.commandDuration.WithLabelValues(label).Observe(elapsed.Seconds())
	s.logger.Debug("EPP command",
		"connection_id", cmd.session.id,
		"clid", cmd.session.registrar(),
		"command", label,
		"code", cmd.code,
		"cltrid", cmd.frame.clTRID,
		"svtrid", cmd.svTRID,
		"duration_ms", elapsed.Milliseconds())

	if response != nil {
		_, _ = rw.Write(response) // the connection is gone if this fails; epp-lib reports it on flush
	}
	if cmd.closing {
		rw.CloseAfterWrite()
	}
}

// audit records cmd in the audit log. A transform that ran is written
// before its response is sent; if that write fails, the record is queued
// for a retry instead, and the response still goes out: the registrar must
// learn the result of a change that has happened. Everything else is
// queued.
func (s *Server) audit(ctx context.Context, cmd *command) {
	label := cmd.frame.label()
	query := entities.EPPCommandIsQuery(label, cmd.isPollRequest())
	tx := &entities.EPPTransaction{
		SvTRID:     cmd.svTRID,
		ClTRID:     cmd.frame.clTRID,
		SessionID:  cmd.session.id,
		Node:       s.cfg.Sessions.NodeID,
		ClID:       entities.ClIDType(cmd.session.registrar()),
		LoginID:    entities.ClIDType(cmd.session.loggedInAs()),
		ClientIP:   cmd.session.source,
		Command:    label,
		Objects:    cmd.objects,
		ResultCode: cmd.code,
		ReceivedAt: cmd.received,
		Duration:   time.Since(cmd.received),
	}
	if tx.LoginID == "" {
		tx.LoginID = entities.ClIDType(cmd.attemptedLogin)
	}
	if cmd.doc != nil {
		tx.Request = redactedFrame(cmd.doc)
	}
	if cmd.response != nil && (!query || s.cfg.Audit.QueryResponses) {
		tx.Response = redactedFrame(cmd.response)
	}

	// The session's context may already be cancelled (a logout, a kill);
	// the record is written on its own, bounded context.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedStoreTimeout*4)
	defer cancel()
	if cmd.handled && !query {
		err := s.auditLog.Record(auditCtx, tx)
		if err == nil {
			return
		}
		s.metrics.auditErrors.WithLabelValues(auditStageSync).Inc()
		s.logger.Error("EPP audit record not written before the response; queued for retry", "svtrid", cmd.svTRID, "command", label, "error", err)
	}
	if err := s.auditLog.Enqueue(auditCtx, tx); err != nil {
		s.metrics.auditErrors.WithLabelValues(auditStageLost).Inc()
		s.logger.Error("EPP audit record lost", "svtrid", cmd.svTRID, "command", label, "error", err)
	}
}

// describe records what a validated command names, for the audit log and
// for its handler: the login ID a <login> presents, and the identifiers
// (<name> or <id>) an object command is about, whitespace-collapsed as the
// schema's token type defines them. It runs before the policy, so a command
// the policy refuses is still recorded with what it asked about.
func (c *command) describe() {
	if c.frame.command == "login" {
		c.attemptedLogin = elementText(c.doc, "/epp/command/login/clID")
		return
	}
	if c.frame.objectNS == "" {
		return
	}
	obj := c.doc.FindElement("/epp/command/" + c.frame.command + "/*")
	if obj == nil {
		return
	}
	for _, el := range obj.ChildElements() {
		if el.Tag == "name" || el.Tag == "id" {
			c.objects = append(c.objects, schema.TokenValue(el.Text()))
		}
	}
}

// isPollRequest reports whether the command is <poll op="req">.
func (c *command) isPollRequest() bool {
	if c.doc == nil || c.frame.command != "poll" {
		return false
	}
	if p := c.doc.FindElement("/epp/command/poll"); p != nil {
		return p.SelectAttrValue("op", "") == "req"
	}
	return false
}

// greeting is epp-lib's Greeting hook, sent once when a session opens.
func (s *Server) greeting(_ context.Context, rw *epplib.ResponseWriter) {
	_, _ = greetingDocument(time.Now()).WriteTo(rw)
}
