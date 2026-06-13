// Package runtime ports the GreenNode AgentBase runtime HTTP contract: the
// request headers it injects, the per-request context, and the /invocations +
// /health endpoints an agent must expose to be deployable on AgentBase.
package runtime

import (
	"context"
	"net/http"
	"strings"
)

// Headers injected by the AgentBase runtime into every /invocations request.
const (
	SessionHeader           = "X-GreenNode-AgentBase-Session-Id"
	RequestIDHeader         = "X-GreenNode-AgentBase-Request-Id"
	AccessTokenHeader       = "X-GreenNode-AgentBase-Access-Token"
	UserIDHeader            = "X-GreenNode-AgentBase-User-Id"
	OAuth2CallbackURLHeader = "X-GreenNode-AgentBase-OAuth2-Callback-Url"
	AuthorizationHeader     = "Authorization"
	CustomHeaderPrefix      = "X-GreenNode-AgentBase-Custom-"
)

// RequestContext carries request-scoped metadata extracted from the runtime
// headers. It is the Go analogue of the Python RequestContext.
type RequestContext struct {
	RequestID         string
	SessionID         string
	UserID            string
	WorkloadToken     string
	OAuth2CallbackURL string
	// Headers holds the Authorization header plus any X-GreenNode-AgentBase-Custom-* headers.
	Headers map[string]string
}

// FromHeaders builds a RequestContext from an http.Header set.
func FromHeaders(h http.Header) *RequestContext {
	rc := &RequestContext{
		RequestID:         h.Get(RequestIDHeader),
		SessionID:         h.Get(SessionHeader),
		UserID:            h.Get(UserIDHeader),
		WorkloadToken:     h.Get(AccessTokenHeader),
		OAuth2CallbackURL: h.Get(OAuth2CallbackURLHeader),
		Headers:           map[string]string{},
	}
	if auth := h.Get(AuthorizationHeader); auth != "" {
		rc.Headers[AuthorizationHeader] = auth
	}
	for name, values := range h {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(CustomHeaderPrefix)) && len(values) > 0 {
			rc.Headers[name] = values[0]
		}
	}
	return rc
}

type ctxKey struct{}

// WithContext stores a RequestContext in a context.Context.
func WithContext(ctx context.Context, rc *RequestContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, rc)
}

// FromContext retrieves the RequestContext, or nil if absent.
func FromContext(ctx context.Context) *RequestContext {
	rc, _ := ctx.Value(ctxKey{}).(*RequestContext)
	return rc
}
