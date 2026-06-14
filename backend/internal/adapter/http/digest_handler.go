package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vngcloud/agentt/internal/domain/digest"
	"github.com/vngcloud/agentt/internal/usecase"
)

// DigestHandler exposes the digest use cases over HTTP for the frontend.
type DigestHandler struct {
	svc *usecase.DigestService
}

// NewDigestHandler builds a DigestHandler.
func NewDigestHandler(svc *usecase.DigestService) *DigestHandler {
	return &DigestHandler{svc: svc}
}

// ListDates serves GET /api/v1/digests — the dates that have a digest.
func (h *DigestHandler) ListDates(w http.ResponseWriter, r *http.Request) {
	dates, err := h.svc.ListDates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dates": dates})
}

// Get serves GET /api/v1/digests/{date} — the full digest.
func (h *DigestHandler) Get(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	d, err := h.svc.Get(r.Context(), date)
	if errors.Is(err, digest.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no digest for "+date)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type askRequest struct {
	UserID   string `json:"userId"`
	Question string `json:"question"`
}

// Ask serves POST /api/v1/digests/{date}/ask — a follow-up question (which may
// also apply a PO correction via the loop's update_digest tool).
func (h *DigestHandler) Ask(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.UserID == "" {
		req.UserID = "anonymous"
	}

	out, err := h.svc.AskFollowup(r.Context(), usecase.FollowupInput{
		Date: date, UserID: req.UserID, Question: req.Question,
	})
	if errors.Is(err, digest.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no digest for "+date)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type flagRequest struct {
	UserID string `json:"userId"`
	Note   string `json:"note"`
}

// Flag serves PATCH /api/v1/digests/{date}/flag — PO marks a digest incorrect.
func (h *DigestHandler) Flag(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	var req flagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.UserID == "" {
		req.UserID = "anonymous"
	}
	d, err := h.svc.FlagDigest(r.Context(), date, req.UserID, req.Note)
	if errors.Is(err, digest.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no digest for "+date)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type dailyJobRequest struct {
	Date string `json:"date"`
}

// RunDaily serves POST /api/v1/jobs/daily — trigger a daily run (dev only).
func (h *DigestHandler) RunDaily(w http.ResponseWriter, r *http.Request) {
	var req dailyJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Date == "" {
		writeError(w, http.StatusBadRequest, "date is required (YYYY-MM-DD)")
		return
	}
	d, err := h.svc.GenerateDaily(r.Context(), req.Date)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// MonthlyReport serves both POST /api/v1/jobs/monthly and
// GET /api/v1/report/monthly/{ym}. The rollup runs synchronously for the MVP.
func (h *DigestHandler) MonthlyReport(w http.ResponseWriter, r *http.Request) {
	ym := chi.URLParam(r, "ym")
	if ym == "" {
		var req struct {
			Month string `json:"month"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		ym = req.Month
	}
	if ym == "" {
		writeError(w, http.StatusBadRequest, "month is required (YYYY-MM)")
		return
	}
	report, err := h.svc.GenerateMonthly(r.Context(), ym)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}
