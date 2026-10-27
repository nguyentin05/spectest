package ui

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/adapter/cli/cmdutil"
)

//go:embed dist/*
var embeddedAssets embed.FS

// Server hosts the SpecTest Web UI and REST API.
type Server struct {
	addr       string
	workDir    string
	factory    *cmdutil.Factory
	sessionMgr *SessionManager
	httpServer *http.Server
}

// NewServer creates a new Web UI Server instance.
func NewServer(addr string, workDir string, f *cmdutil.Factory) *Server {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &Server{
		addr:       addr,
		workDir:    workDir,
		factory:    f,
		sessionMgr: NewSessionManager(),
	}
}

// Start boots up the HTTP server listener.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.sessionMgr.AppendLog("SpecTest Web UI Server listening on " + s.addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.sessionMgr.CancelActive()
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Routes configures and returns the HTTP mux for the server.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// 1. Health and Status
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"time":     time.Now().Format(time.RFC3339),
			"work_dir": s.workDir,
		})
	})

	// 2. Logs
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"logs": s.sessionMgr.Logs(),
		})
	})

	// 3. Configuration
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleGetConfig(w, r)
		case http.MethodPost:
			s.handleUpdateConfig(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/config/test", s.handleTestLLM)

	// 4. Target Server Probing
	mux.HandleFunc("/api/server/healthcheck", s.handleHealthCheck)

	// 5. Specs & Workspace
	mux.HandleFunc("/api/specs/workspace", s.handleListWorkspace)
	mux.HandleFunc("/api/specs/upload", s.handleUploadSpec)
	mux.HandleFunc("/api/specs/plan", s.handleGetPlan)
	mux.HandleFunc("/api/specs/history", s.handleGetHistory)

	// 6. Test Generation
	mux.HandleFunc("/api/generate", s.handleGenerate)

	// 7. Runner & SSE
	mux.HandleFunc("/api/run", s.handleRunStart)
	mux.HandleFunc("/api/run/stream", s.handleRunStream)
	mux.HandleFunc("/api/run/cancel", s.handleRunCancel)
	mux.HandleFunc("/api/run/report", s.handleRunReport)

	// 8. Markdown Viewer
	mux.HandleFunc("/api/markdown/read", s.handleReadMarkdown)

	// 9. Static Assets (SPA)
	distFS, err := fs.Sub(embeddedAssets, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			if f, err := distFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// Do not fall back to index.html for static assets under /assets/
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				http.NotFound(w, r)
				return
			}
			// Fallback to index.html for client-side routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error":   true,
		"message": message,
	})
}
