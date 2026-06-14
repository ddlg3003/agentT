package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/vngcloud/agentt/internal/usecase"
	gnruntime "github.com/vngcloud/agentt/pkg/greennode/runtime"
)

// RouterDeps are the dependencies the router needs to wire its handlers.
type RouterDeps struct {
	Chat           *usecase.ChatService
	Digest         *usecase.DigestService
	AllowedOrigins []string
}

// NewRouter builds the application's HTTP handler. It serves two contracts from
// one binary:
//   - the REST API consumed by the frontend (/healthz, /api/v1/*)
//   - the GreenNode AgentBase runtime contract (/invocations, /health) so the
//     same image can be deployed on AgentBase.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	chatHandler := NewChatHandler(deps.Chat)
	digestHandler := NewDigestHandler(deps.Digest)

	// Liveness for the frontend / orchestrators.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Frontend-facing REST API.
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/chat", chatHandler.Handle)

		// Digest product endpoints.
		api.Get("/digests", digestHandler.ListDates)
		api.Get("/digests/{date}", digestHandler.Get)
		api.Post("/digests/{date}/ask", digestHandler.Ask)
		api.Patch("/digests/{date}/flag", digestHandler.Flag)

		// Job triggers (dev) + monthly report.
		api.Post("/jobs/daily", digestHandler.RunDaily)
		api.Post("/jobs/monthly", digestHandler.MonthlyReport)
		api.Get("/report/monthly/{ym}", digestHandler.MonthlyReport)
	})

	// GreenNode AgentBase runtime contract. The agent entrypoint reuses the
	// same chat use case, mapping the runtime payload/headers into it.
	r.Method(http.MethodPost, "/invocations", gnruntime.InvocationsHandler(
		func(req *http.Request, payload json.RawMessage, rc *gnruntime.RequestContext) (any, error) {
			var p struct {
				Query   string `json:"query"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(payload, &p)
			msg := p.Message
			if msg == "" {
				msg = p.Query
			}
			userID := rc.UserID
			if userID == "" {
				userID = "anonymous"
			}
			sessionID := rc.SessionID
			if sessionID == "" {
				sessionID = "default"
			}
			return deps.Chat.Chat(req.Context(), usecase.ChatInput{
				UserID:    userID,
				SessionID: sessionID,
				Message:   msg,
			})
		},
	))
	r.Method(http.MethodGet, "/health", gnruntime.HealthHandler(nil))

	return r
}
