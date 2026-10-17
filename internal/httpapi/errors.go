package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Error messages from ydk:error manifests. Every non-2xx body is {"error": "<message>"} (ydk:crosscut:system/error-format).
const (
	msgInternalError    = "internal error"                        // ydk:error:api/internal-error (500)
	msgInvalidJson      = "invalid JSON body"                     // ydk:error:api/invalid-json (400)
	msgKafkaUnavailable = "kafka unavailable, message not sent"   // ydk:error:api/kafka-unavailable (503)
	msgMethodNotAllowed = "method ${method} not allowed"          // ydk:error:api/method-not-allowed (405)
	msgNotFound         = "not found"                             // ydk:error:api/not-found (404)
	msgNotReady         = "shutting down"                         // ydk:error:api/not-ready (503)
	msgValidationFailed = "${field} must be 1..${max} characters" // ydk:error:api/validation-failed (400)
)

// writeJSON writes v as a JSON body with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError is the single error helper every handler uses.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// renderMessage fills ${name} placeholders: renderMessage(tmpl, "${method}", r.Method).
func renderMessage(tmpl string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
