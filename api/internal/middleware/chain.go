package middleware

import "net/http"

// Middleware decorates an http.Handler. Composing these replaces gin's
// engine/group Use() stack.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with mws so the first listed middleware runs outermost: it sees
// the request first and the response last, matching the order of gin's r.Use().
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
