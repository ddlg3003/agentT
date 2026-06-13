// Package http is the delivery layer: it translates HTTP requests into use-case
// calls and back. It depends on the use case, never the other way around.
package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vngcloud/agentt/internal/usecase"
)

// ChatHandler exposes the chat use case over HTTP.
type ChatHandler struct {
	chat *usecase.ChatService
}

// NewChatHandler builds a ChatHandler.
func NewChatHandler(chat *usecase.ChatService) *ChatHandler {
	return &ChatHandler{chat: chat}
}

type chatRequest struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// Handle serves POST /api/v1/chat.
func (h *ChatHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.UserID == "" {
		req.UserID = "anonymous"
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}

	out, err := h.chat.Chat(r.Context(), usecase.ChatInput{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Message:   req.Message,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
