package http

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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
	// StaticDir, if non-empty, serves the built frontend with SPA fallback.
	StaticDir string
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

	// Timeouts are scoped by workload. Quick reads/writes get a short deadline;
	// endpoints that run a multi-turn agent loop (daily/monthly synthesis,
	// follow-up Q&A, chat) get a generous one — a 60s blanket timeout was
	// cutting the daily loop off mid-run ("context deadline exceeded" + a
	// superfluous WriteHeader from the timeout middleware).
	const (
		quickTimeout = 30 * time.Second
		agentTimeout = 5 * time.Minute
	)

	// Frontend-facing REST API.
	r.Route("/api/v1", func(api chi.Router) {
		// Quick, non-LLM endpoints.
		api.Group(func(q chi.Router) {
			q.Use(middleware.Timeout(quickTimeout))
			q.Get("/digests", digestHandler.ListDates)
			q.Get("/digests/{date}", digestHandler.Get)
			q.Get("/digests/{date}/history", digestHandler.GetHistory)
			q.Patch("/digests/{date}/flag", digestHandler.Flag)
		})

		// Agent-loop endpoints — long deadline (many sequential LLM turns).
		api.Group(func(loop chi.Router) {
			loop.Use(middleware.Timeout(agentTimeout))
			loop.Post("/chat", chatHandler.Handle)
			loop.Post("/digests/{date}/ask", digestHandler.Ask)
			loop.Post("/jobs/daily", digestHandler.RunDaily)
			loop.Post("/jobs/monthly", digestHandler.MonthlyReport)
			loop.Get("/report/monthly/{ym}", digestHandler.MonthlyReport)
		})
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

	// Serve built frontend (SPA). Registered last so API routes take priority.
	if deps.StaticDir != "" {
		r.Handle("/*", spaHandler(deps.StaticDir))
	}

	return r
}

// spaHandler serves static files from dir, falling back to index.html for
// any path that doesn't match a real file (React Router / SPA routing).
func spaHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
