package middleware

import (
	"context"
	"net/http"

	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// ctxKey is an unexported type for request-context keys so values stored here
// can't collide with keys set by other packages.
type ctxKey int

const (
	userKey ctxKey = iota
	requestIDKey
)

// WithUser returns a copy of r carrying the authenticated user in its context.
func WithUser(r *http.Request, u *service.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}

// WithRequestID returns a copy of r carrying the request id in its context.
func WithRequestID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
}

// User returns the authenticated user from the context, or nil. Handlers pass
// r.Context(); middleware can pass either the request context or its own.
func User(ctx context.Context) *service.User {
	u, _ := ctx.Value(userKey).(*service.User)
	return u
}

// RequestID returns the request id from the context, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
