package httpapi

import (
	"html"
	"net/http"
	"strings"

	"clopy-paste/web"
)

// instancePlaceholder is replaced with the html-escaped INSTANCE_NAME in web/main.html.
const instancePlaceholder = "{{INSTANCE_NAME}}"

// splitPlaceholder is replaced with the embedded web/split.js (ADR-019).
const splitPlaceholder = "{{SPLIT_JS}}"

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
	page := strings.Replace(string(body), splitPlaceholder, web.SplitJS, 1)
	page = strings.ReplaceAll(page, instancePlaceholder, html.EscapeString(s.deps.InstanceName))
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
