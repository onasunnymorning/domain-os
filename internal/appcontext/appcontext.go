// Package appcontext holds the typed keys used to carry request- and
// session-scoped values on a context.Context.
//
// Keys are a private type so they cannot collide with keys set by any other
// package, including third-party libraries sharing the same context. Callers
// go through the accessors below rather than calling context.WithValue or
// ctx.Value directly, so the value type is checked at compile time instead of
// at every read site.
//
// See INV-15 in docs/INVARIANTS.md.
package appcontext

import "context"

// contextKey is unexported so no package outside this one can construct a key
// that collides with these.
type contextKey string

const (
	// Request-path keys, populated by the REST middleware.
	keyUserID        contextKey = "userid"
	keyTraceID       contextKey = "trace_id"
	keyCorrelationID contextKey = "correlation_id"

	// EPP session keys, populated per connection by the EPP server.
	keyConnectionID contextKey = "cid"
	keyClientIP     contextKey = "clientIP"
	keyRegistrarID  contextKey = "registrarID"
)

// WithUserID returns a copy of ctx carrying the acting user's identity.
func WithUserID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyUserID, v)
}

// UserID reports the acting user's identity, and whether one was set.
func UserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyUserID).(string)
	return v, ok
}

// WithTraceID returns a copy of ctx carrying the trace ID — the identifier for
// this single execution. On a request originating from a Temporal activity this
// is the Temporal Run ID.
func WithTraceID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyTraceID, v)
}

// TraceID reports the trace ID, and whether one was set.
func TraceID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyTraceID).(string)
	return v, ok
}

// WithCorrelationID returns a copy of ctx carrying the correlation ID — the
// identifier for the logical operation, stable across retries. On work
// originating from Temporal this is the Workflow ID.
func WithCorrelationID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyCorrelationID, v)
}

// CorrelationID reports the correlation ID, and whether one was set.
func CorrelationID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyCorrelationID).(string)
	return v, ok
}

// WithConnectionID returns a copy of ctx carrying the EPP connection ID.
func WithConnectionID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyConnectionID, v)
}

// ConnectionID reports the EPP connection ID, and whether one was set.
func ConnectionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyConnectionID).(string)
	return v, ok
}

// WithClientIP returns a copy of ctx carrying the client IP of an EPP connection.
func WithClientIP(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyClientIP, v)
}

// ClientIP reports the client IP, and whether one was set.
func ClientIP(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyClientIP).(string)
	return v, ok
}

// WithRegistrarID returns a copy of ctx carrying the authenticated registrar's
// ClID for an EPP session.
func WithRegistrarID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, keyRegistrarID, v)
}

// RegistrarID reports the authenticated registrar's ClID, and whether one was set.
func RegistrarID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyRegistrarID).(string)
	return v, ok
}
