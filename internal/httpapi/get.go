package httpapi

import (
	"net/http"
	"time"

	"clopy-paste/internal/messaging"
)

type messageJSON struct {
	Text string `json:"text"`
	Time string `json:"time"`
}

// handleGetMessages serves GET /api/messages -- ydk:route:api/get-messages: Returns messages received from the other side for one username, newest first, at most 100.
func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	// ValidateUser also rejects invalid UTF-8, so the censor only sees valid input.
	if err := messaging.ValidateUser(user); err != nil {
		writeError(w, http.StatusBadRequest, renderMessage(msgValidationFailed, "${field}", "user", "${max}", "128"))
		return
	}
	msgs := s.deps.Store.Get(s.deps.Censor.Apply(user))
	out := make([]messageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = messageJSON{Text: m.Text, Time: m.Time.UTC().Format(time.RFC3339)}
	}
	writeJSON(w, http.StatusOK, out)
}
