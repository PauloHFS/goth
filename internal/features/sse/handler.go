package sse

import (
	"fmt"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
	"github.com/alexedwards/scs/v2"
)

type Handler struct {
	broker  Broker
	session *scs.SessionManager
}

func NewHandler(broker Broker, session *scs.SessionManager) *Handler {
	return &Handler{
		broker:  broker,
		session: session,
	}
}

// ServeHTTP estabelece conexão SSE para notificações em tempo real
// @Summary SSE notifications
// @Description Establishes Server-Sent Events connection for real-time notifications
// @Tags Real-time
// @Produce text/event-stream
// @Success 200 {string} string "SSE stream"
// @Failure 401 {object} map[string]string
// @Router /events [get]
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to get user from context first (if auth middleware ran)
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		// Fallback: get user_id from session
		userID := h.session.GetInt64(r.Context(), "user_id")
		if userID == 0 {
			httpErr.HandleError(w, r, httpErr.NewUnauthorizedError(""), "sse_auth")
			return
		}
		// Create minimal user object
		user = db.User{ID: userID}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpErr.HandleError(w, r, httpErr.NewInternalError("streaming unsupported", nil), "sse_flusher")
		return
	}

	messageChan := make(chan string, 10)
	RegisterClient(h.broker, user.ID, messageChan)

	defer func() {
		UnregisterClient(h.broker, user.ID, messageChan)
		// Drain and close channel to prevent leaks
		for range messageChan {
		}
	}()

	flusher.Flush()

	for {
		select {
		case msg := <-messageChan:
			_, _ = fmt.Fprint(w, msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
