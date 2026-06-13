package runtime

import (
	"encoding/json"
	"net/http"
)

// PingStatus mirrors the AgentBase health states.
type PingStatus string

const (
	StatusHealthy     PingStatus = "Healthy"
	StatusHealthyBusy PingStatus = "HealthyBusy"
)

// Handler is the user agent entrypoint. payload is the raw JSON body of the
// /invocations request; rc carries the runtime-injected request metadata. The
// returned value is JSON-encoded as the response.
type Handler func(r *http.Request, payload json.RawMessage, rc *RequestContext) (any, error)

// InvocationsHandler returns an http.HandlerFunc implementing the AgentBase
// /invocations contract. Mount it at POST /invocations.
//
// It is decoupled from any router: the application can mount it alongside its
// own REST API so the same binary serves both the frontend and the AgentBase
// runtime.
func InvocationsHandler(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON in request"})
				return
			}
		}

		rc := FromHeaders(r.Header)
		r = r.WithContext(WithContext(r.Context(), rc))

		result, err := h(r, payload, rc)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// HealthHandler returns an http.HandlerFunc implementing the AgentBase /health
// contract. If status is nil it always reports Healthy.
func HealthHandler(status func() PingStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := StatusHealthy
		if status != nil {
			s = status()
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": string(s)})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	// Disable nginx buffering, matching the upstream runtime behaviour.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
