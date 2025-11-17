package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Vova4o/VPSSetup/internal/config"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Server represents the API server
type Server struct {
	router   *mux.Router
	config   *config.Config
	port     int
	upgrader websocket.Upgrader
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, port int) *Server {
	s := &Server{
		router: mux.NewRouter(),
		config: cfg,
		port:   port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in local dev
			},
		},
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Serve static files
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))
	s.router.HandleFunc("/", s.handleIndex).Methods("GET")

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// VPS management
	api.HandleFunc("/vps/setup", s.handleVPSSetup).Methods("POST")
	api.HandleFunc("/vps/status", s.handleVPSStatus).Methods("GET")
	api.HandleFunc("/vps/destroy", s.handleVPSDestroy).Methods("DELETE")
	api.HandleFunc("/vps/restart", s.handleVPSRestart).Methods("POST")
	api.HandleFunc("/vps/connect", s.handleVPSConnect).Methods("POST")

	// DNS management
	api.HandleFunc("/dns/domains", s.handleGetDomains).Methods("GET")
	api.HandleFunc("/dns/records", s.handleDNSList).Methods("GET")
	api.HandleFunc("/dns/records", s.handleDNSCreate).Methods("POST")
	api.HandleFunc("/dns/records/{id}", s.handleDNSUpdate).Methods("PUT")
	api.HandleFunc("/dns/records/{id}", s.handleDNSRemove).Methods("DELETE")

	// NGINX management
	api.HandleFunc("/nginx/setup", s.handleNginxSetup).Methods("POST")
	api.HandleFunc("/nginx/test", s.handleNginxTest).Methods("POST")
	api.HandleFunc("/nginx/reload", s.handleNginxReload).Methods("POST")

	// SSL management
	api.HandleFunc("/ssl/install", s.handleSSLInstall).Methods("POST")
	api.HandleFunc("/ssl/renew", s.handleSSLRenew).Methods("POST")

	// Deployment
	api.HandleFunc("/deploy", s.handleDeploy).Methods("POST")

	// Logs (WebSocket)
	api.HandleFunc("/logs/stream", s.handleLogsStream)
	api.HandleFunc("/logs/{type}", s.handleLogs).Methods("GET")

	// Security
	api.HandleFunc("/harden", s.handleHarden).Methods("POST")

	// Provider data
	api.HandleFunc("/providers/regions", s.handleGetRegions).Methods("GET")
	api.HandleFunc("/providers/sizes", s.handleGetSizes).Methods("GET")

	// Config
	api.HandleFunc("/config/check", s.handleCheckConfig).Methods("GET")
	api.HandleFunc("/config/init", s.handleInitConfig).Methods("POST")
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config", s.handleUpdateConfig).Methods("PUT")

	// SSH
	api.HandleFunc("/ssh/generate-key", s.handleGenerateSSHKey).Methods("POST")
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("🚀 API server starting on http://localhost%s\n", addr)
	fmt.Printf("📱 Web UI: http://localhost%s\n", addr)
	fmt.Printf("📚 API Docs: http://localhost%s/api/docs\n", addr)
	return http.ListenAndServe(addr, s.router)
}

// handleIndex serves the main web UI
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/index.html")
}

// respondJSON sends a JSON response
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondSuccess sends a success response
func (s *Server) respondSuccess(w http.ResponseWriter, data interface{}) {
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// respondError sends an error response
func (s *Server) respondError(w http.ResponseWriter, status int, err error) {
	errorMsg := "Unknown error"
	if err != nil {
		errorMsg = err.Error()
	} else if status == http.StatusNotImplemented {
		errorMsg = "Feature not implemented"
	}
	s.respondJSON(w, status, Response{
		Success: false,
		Error:   errorMsg,
	})
}

// getProfile gets the active profile from config
func (s *Server) getProfile(r *http.Request) (*config.Profile, error) {
	profileName := r.URL.Query().Get("profile")
	return s.config.GetProfile(profileName)
}

// withTimeout wraps a context with timeout
func withTimeout(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, duration)
}
