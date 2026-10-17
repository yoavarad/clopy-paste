package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"clopy-paste/internal/messaging"
)

const (
	maxBodyBytes    = 16384
	publishDeadline = 5 * time.Second
	msgBodyTooLarge = "request body too large"
)

type sendRequest struct {
	User json.RawMessage `json:"user"`
	Text json.RawMessage `json:"text"`
}

// decodeField returns the string value of a raw JSON field; ok=false means invalid UTF-8 (reject before
// json.Unmarshal can rewrite it to U+FFFD); a non-string value is an error.
func decodeField(raw json.RawMessage) (s string, validUTF8 bool, err error) {
	if !utf8.Valid(raw) {
		return "", false, nil
	}
	if len(raw) > 0 {
		err = json.Unmarshal(raw, &s)
	}
	return s, true, err
}

// handleSendMessage serves POST /api/messages -- ydk:route:api/send-message: validate, censor, publish, answer after the broker ack.
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusBadRequest, msgBodyTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, msgInvalidJson)
		return
	}
	var raw sendRequest
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidJson)
		return
	}
	user, userOK, err1 := decodeField(raw.User)
	text, textOK, err2 := decodeField(raw.Text)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, msgInvalidJson)
		return
	}
	if !userOK || messaging.ValidateUser(user) != nil {
		writeValidation(w, "user", messaging.MaxUserLen)
		return
	}
	if !textOK || messaging.ValidateText(text) != nil {
		writeValidation(w, "text", messaging.MaxTextLen)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), publishDeadline)
	defer cancel()
	if err := s.deps.Publisher.Publish(ctx, s.deps.Censor.Apply(user), s.deps.Censor.Apply(text)); err != nil {
		s.deps.Logger.Warn("publish failed", "user_len", utf8.RuneCountInString(user), "text_len", utf8.RuneCountInString(text), "timeout", errors.Is(err, context.DeadlineExceeded))
		writeError(w, http.StatusServiceUnavailable, msgKafkaUnavailable)
		return
	}
	writeJSON(w, http.StatusAccepted, struct{}{})
}

func writeValidation(w http.ResponseWriter, field string, max int) {
	writeError(w, http.StatusBadRequest, renderMessage(msgValidationFailed, "${field}", field, "${max}", strconv.Itoa(max)))
}
