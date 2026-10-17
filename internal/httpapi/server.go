// Package httpapi wires ydk:route manifests onto a stdlib ServeMux.
package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"clopy-paste/internal/messaging"
)

// Deps are the collaborators handlers use (one field per ydk:contract).
type Deps struct {
	Logger    *slog.Logger
	Censor    messaging.Censor
	Publisher messaging.Publisher
	Store     messaging.Store

	// InstanceName is the label shown in the page header (INSTANCE_NAME).
	InstanceName string
}

// Server holds handler dependencies and readiness state.
type Server struct {
	deps  Deps
	ready atomic.Bool
}

// NewServer returns a ready Server. It panics when Censor, Publisher or Store is nil: a
// missing collaborator is a wiring bug that must fail at startup, not on the first request.
func NewServer(deps Deps) *Server {
	switch {
	case deps.Censor == nil:
		panic("httpapi: NewServer: nil Censor")
	case deps.Publisher == nil:
		panic("httpapi: NewServer: nil Publisher")
	case deps.Store == nil:
		panic("httpapi: NewServer: nil Store")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	s := &Server{deps: deps}
	s.ready.Store(true)
	return s
}

// SetReady toggles readiness; graceful shutdown sets it to false first.
func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

// Handler returns the root handler: routes, JSON 404/405 bodies, panic recovery, request logging.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/messages", s.handleGetMessages)  // ydk:route:api/get-messages
	mux.HandleFunc("GET /healthz", s.handleHealthz)           // ydk:route:api/healthz
	mux.HandleFunc("GET /{$}", s.handleIndex)                 // ydk:route:api/index
	mux.HandleFunc("GET /readyz", s.handleReadyz)             // ydk:route:api/readyz
	mux.HandleFunc("POST /api/messages", s.handleSendMessage) // ydk:route:api/send-message
	return s.logRequests(s.recoverPanics(jsonFallback(mux)))
}

// recoverPanics turns a handler panic into a JSON 500 (TODO handlers panic until implemented).
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				s.deps.Logger.Error("internal error", "route", routeOf(r), "method", r.Method, "error", fmt.Sprint(rec))
				writeError(w, http.StatusInternalServerError, msgInternalError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// jsonFallback replaces the ServeMux's plain-text 404/405 bodies with JSON, keeping Allow.
func jsonFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		capture := &statusCapture{header: http.Header{}}
		mux.ServeHTTP(capture, r)
		switch capture.status {
		case http.StatusNotFound:
			writeError(w, http.StatusNotFound, msgNotFound)
		case http.StatusMethodNotAllowed:
			w.Header().Set("Allow", capture.header.Get("Allow"))
			writeError(w, http.StatusMethodNotAllowed, renderMessage(msgMethodNotAllowed, "${method}", r.Method))
		default:
			mux.ServeHTTP(w, r)
		}
	})
}

// statusCapture records the status and headers the mux would send, discarding the body.
type statusCapture struct {
	header http.Header
	status int
}

func (c *statusCapture) Header() http.Header { return c.header }

func (c *statusCapture) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return len(b), nil
}

func (c *statusCapture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
