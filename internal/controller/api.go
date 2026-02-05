package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/floatip/gateway/internal/config"
)

// Server is the HTTP API server for the controller.
type Server struct {
	manager   *Manager
	mux       *http.ServeMux
	server    *http.Server
	probeTick *time.Ticker
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewServer creates a new HTTP server.
func NewServer(manager *Manager) *Server {
	s := &Server{
		manager: manager,
		mux:     http.NewServeMux(),
		stopCh:  make(chan struct{}),
	}
	s.setupRoutes()
	return s
}

// setupRoutes configures HTTP handlers.
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/routers", s.handleRouters)
	s.mux.HandleFunc("/api/routers/", s.handleRouter)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/config", s.handleConfig)

	// Static files (web UI)
	s.mux.HandleFunc("/", s.handleStatic)
}

// Start starts the HTTP server.
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	// Start background probe
	s.startProbing()

	// Initial probe
	go s.manager.ProbeAll()

	return s.server.ListenAndServe()
}

// Stop stops the HTTP server.
func (s *Server) Stop() error {
	close(s.stopCh)
	s.wg.Wait()
	return s.server.Close()
}

// startProbing starts periodic router probing.
func (s *Server) startProbing() {
	s.probeTick = time.NewTicker(30 * time.Second)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.probeTick.C:
				s.manager.ProbeAll()
			case <-s.stopCh:
				s.probeTick.Stop()
				return
			}
		}
	}()
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// handleRouters handles /api/routers
func (s *Server) handleRouters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		routers := s.manager.GetRouters()
		writeJSON(w, http.StatusOK, routers)

	case http.MethodPost:
		var router Router
		if err := json.NewDecoder(r.Body).Decode(&router); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.manager.AddRouter(&router); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		s.manager.SaveConfig()
		writeJSON(w, http.StatusCreated, router)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleRouter handles /api/routers/{name}/*
func (s *Server) handleRouter(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/routers/{name}[/action]
	path := strings.TrimPrefix(r.URL.Path, "/api/routers/")
	parts := strings.SplitN(path, "/", 2)
	name := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	router := s.manager.GetRouter(name)
	if router == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("router %q not found", name))
		return
	}

	switch action {
	case "":
		s.handleRouterCRUD(w, r, router)
	case "probe":
		s.handleRouterProbe(w, r, router)
	case "install":
		s.handleRouterInstall(w, r, router)
	case "uninstall":
		s.handleRouterUninstall(w, r, router)
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown action %q", action))
	}
}

// handleRouterCRUD handles GET/PUT/DELETE on a router.
func (s *Server) handleRouterCRUD(w http.ResponseWriter, r *http.Request, router *Router) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, router)

	case http.MethodPut:
		var update Router
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		// Update fields
		if update.Host != "" {
			router.Host = update.Host
		}
		if update.Port != 0 {
			router.Port = update.Port
		}
		if update.User != "" {
			router.User = update.User
		}
		if update.Password != "" {
			router.Password = update.Password
		}
		if update.KeyFile != "" {
			router.KeyFile = update.KeyFile
		}
		if update.Role != "" {
			router.Role = update.Role
		}
		s.manager.SaveConfig()
		writeJSON(w, http.StatusOK, router)

	case http.MethodDelete:
		if err := s.manager.RemoveRouter(router.Name); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.manager.SaveConfig()
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleRouterProbe handles POST /api/routers/{name}/probe
func (s *Server) handleRouterProbe(w http.ResponseWriter, r *http.Request, router *Router) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := s.manager.Probe(router); err != nil {
		writeJSON(w, http.StatusOK, router) // Still return status even on error
		return
	}
	writeJSON(w, http.StatusOK, router)
}

// handleRouterInstall handles POST /api/routers/{name}/install
func (s *Server) handleRouterInstall(w http.ResponseWriter, r *http.Request, router *Router) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse optional config override
	var configOverride config.Config
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&configOverride); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	// Generate config for this router
	agentConfig := s.manager.GenerateAgentConfig(router)

	// Start installation in background
	go func() {
		if err := s.manager.Install(router, agentConfig); err != nil {
			router.Status = StatusError
			router.Error = err.Error()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "installing",
		"message": "Installation started in background",
	})
}

// handleRouterUninstall handles POST /api/routers/{name}/uninstall
func (s *Server) handleRouterUninstall(w http.ResponseWriter, r *http.Request, router *Router) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Start uninstallation in background
	go func() {
		if err := s.manager.Uninstall(router); err != nil {
			router.Status = StatusError
			router.Error = err.Error()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "uninstalling",
		"message": "Uninstallation started in background",
	})
}

// handleStatus handles /api/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg := s.manager.GetConfig()
	routers := s.manager.GetRouters()

	// Find current master
	var currentMaster string
	for _, router := range routers {
		if router.VRRPState == "MASTER" {
			currentMaster = router.Name
			break
		}
	}

	status := map[string]interface{}{
		"vip":            cfg.LAN.VIP,
		"cidr":           cfg.LAN.CIDR,
		"iface":          cfg.LAN.Iface,
		"current_master": currentMaster,
		"routers":        routers,
	}

	writeJSON(w, http.StatusOK, status)
}

// handleConfig handles /api/config
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.manager.GetConfig()
		writeJSON(w, http.StatusOK, cfg)

	case http.MethodPut:
		var update struct {
			LAN struct {
				VIP   string `json:"vip"`
				CIDR  string `json:"cidr"`
				Iface string `json:"iface"`
			} `json:"lan"`
			Keepalived struct {
				VRID int `json:"vrid"`
			} `json:"keepalived"`
			Health struct {
				Mode string `json:"mode"`
			} `json:"health"`
		}
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		cfg := s.manager.GetConfig()
		if update.LAN.VIP != "" {
			cfg.LAN.VIP = update.LAN.VIP
		}
		if update.LAN.CIDR != "" {
			cfg.LAN.CIDR = update.LAN.CIDR
		}
		if update.LAN.Iface != "" {
			cfg.LAN.Iface = update.LAN.Iface
		}
		if update.Keepalived.VRID != 0 {
			cfg.Keepalived.VRID = update.Keepalived.VRID
		}
		if update.Health.Mode != "" {
			cfg.Health.Mode = config.HealthMode(update.Health.Mode)
		}

		s.manager.SaveConfig()
		writeJSON(w, http.StatusOK, cfg)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleStatic serves the web UI.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Serve embedded UI
	content, contentType, ok := getEmbeddedAsset(path)
	if !ok {
		// Fallback to index.html for SPA routing
		content, contentType, ok = getEmbeddedAsset("/index.html")
		if !ok {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

// getEmbeddedAsset returns embedded static file content.
func getEmbeddedAsset(path string) ([]byte, string, bool) {
	assets := getAssets()
	content, ok := assets[path]
	if !ok {
		return nil, "", false
	}

	contentType := "application/octet-stream"
	if strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(path, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(path, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(path, ".ico") {
		contentType = "image/x-icon"
	}

	return content, contentType, true
}
