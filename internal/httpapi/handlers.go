package httpapi

import (
	"html"
	"net/http"
	"strings"

	"clopy-paste/web"
)

// instancePlaceholder is replaced with the html-escaped INSTANCE_NAME in web/main.html.
const instancePlaceholder = "{{INSTANCE_NAME}}"

// handleGetMessages serves GET /api/messages -- ydk:route:api/get-messages: Returns messages received from the other side for one username, newest first, at most 100.
func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	panic("not implemented") // ydk:route:api/get-messages: GET /api/messages
}

// handleHealthz serves GET /healthz -- ydk:route:api/healthz: Liveness endpoint (ydk:crosscut:system/health-checks). Returns 200 whenever the HTTP server
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleIndex serves GET / -- ydk:route:api/index: Serves the single page ydk:page:ui/main; the page is one embedded index.html (go:embed, no
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	body, err := web.Pages.ReadFile("main.html")
	if err != nil {
		panic(err)
	}
	page := strings.ReplaceAll(string(body), instancePlaceholder, html.EscapeString(s.deps.InstanceName))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
}

// handleReadyz serves GET /readyz -- ydk:route:api/readyz: Readiness endpoint (ydk:crosscut:system/health-checks, ADR-016). Returns 200 once the HTTP
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeError(w, http.StatusServiceUnavailable, msgNotReady)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleSendMessage serves POST /api/messages -- ydk:route:api/send-message: Accepts a message typed on this instance and publishes it to the output topic.
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	panic("not implemented") // ydk:route:api/send-message: POST /api/messages
}
