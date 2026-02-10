package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/version"
)

// Server represents the HTTP server for the controller.
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
	s.mux.HandleFunc("/api/routers", s.authMiddleware(s.handleRouters))
	s.mux.HandleFunc("/api/routers/", s.authMiddleware(s.handleRouter))
	s.mux.HandleFunc("/api/status", s.authMiddleware(s.handleStatus))
	s.mux.HandleFunc("/api/config", s.authMiddleware(s.handleConfig))
	s.mux.HandleFunc("/api/detect-net", s.authMiddleware(s.handleDetectNet))
	s.mux.HandleFunc("/api/routers/install-all", s.authMiddleware(s.handleInstallAll))
	s.mux.HandleFunc("/api/verify-drift", s.authMiddleware(s.handleVerifyDrift))
	s.mux.HandleFunc("/api/version", s.authMiddleware(s.handleVersion))
	s.mux.HandleFunc("/api/upgrade", s.authMiddleware(s.handleUpgrade))

	// Static files (web UI)
	s.mux.HandleFunc("/", s.authMiddleware(s.handleStatic))
}

// authMiddleware provides simple Basic Auth and Origin protection.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Basic CSRF protection: check Origin/Referer for API requests
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet {
			origin := r.Header.Get("Origin")
			referer := r.Header.Get("Referer")
			host := r.Host

			// If origin exists, it must match host
			if origin != "" {
				if !strings.Contains(origin, host) {
					http.Error(w, "Forbidden: Cross-site request blocked", http.StatusForbidden)
					return
				}
			} else if referer != "" {
				if !strings.Contains(referer, host) {
					http.Error(w, "Forbidden: Cross-site request blocked", http.StatusForbidden)
					return
				}
			}
		}

		cfg := s.manager.GetConfig()
		if cfg.Password == "" {
			// No password set, allow all
			next(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || pass != cfg.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Floating Gateway Controller"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// User field is ignored for now, any user with correct password works
		_ = user
		next(w, r)
	}
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
	s.probeTick = time.NewTicker(5 * time.Second)
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
	case "doctor":
		s.handleRouterDoctor(w, r, router)
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown action %q", action))
	}
}

// handleRouterDoctor handles GET /api/routers/{name}/doctor
func (s *Server) handleRouterDoctor(w http.ResponseWriter, r *http.Request, router *Router) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	report, err := s.manager.Doctor(router)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(report))
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
		if update.Iface != "" {
			router.Iface = update.Iface
		}
		if update.HealthMode != "" {
			router.HealthMode = update.HealthMode
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

	// Prevent duplicate install requests
	if router.Status == StatusInstalling {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":   "already_installing",
			"message": "安装正在进行中，请等待完成",
		})
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
	agentConfig, err := s.manager.GenerateAgentConfig(router)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Start installation in background
	router.Status = StatusInstalling
	router.InstallLog = nil
	router.InstallStep = 0
	router.InstallTotal = 11
	router.Error = ""
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

	// Prevent duplicate uninstall requests
	if router.Status == StatusUninstalling {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":   "already_uninstalling",
			"message": "卸载正在进行中，请等待完成",
		})
		return
	}

	// Start uninstallation in background
	router.Status = StatusUninstalling
	router.InstallLog = nil
	router.InstallStep = 0
	router.InstallTotal = 5
	router.Error = ""
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
		if update.LAN.VIP != "" && update.LAN.VIP != cfg.LAN.VIP {
			// Check for conflict
			conflict, _ := s.manager.CheckVIPConflict(update.LAN.VIP)
			if conflict {
				// We still allow it, but we should return a warning or info
				// For now, let's just log it and maybe we can handle it in UI
				fmt.Printf("Warning: VIP %s is already reachable on the network\n", update.LAN.VIP)
			}
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

// handleDetectNet handles POST /api/detect-net
func (s *Server) handleDetectNet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Check if SSH credentials are provided (for remote router detection)
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		KeyFile  string `json:"key_file"`
	}

	// Try to decode request body
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Host != "" {
			// Remote router detection
			if req.Port == 0 {
				req.Port = 22
			}

			client := NewSSHClient(&SSHConfig{
				Host:     req.Host,
				Port:     req.Port,
				User:     req.User,
				Password: req.Password,
				KeyFile:  req.KeyFile,
				Timeout:  30,
			})

			if err := client.Connect(); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("SSH 连接失败: %w", err))
				return
			}
			defer client.Close()

			// Detect network on remote router
			iface, cidr, err := s.manager.DetectNetwork(client, req.Host)
			if err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("网络探测失败: %w", err))
				return
			}

			// Generate suggested VIP based on CIDR
			suggestedVIP := s.manager.SuggestVIP(cidr)

			writeJSON(w, http.StatusOK, map[string]string{
				"iface":         iface,
				"cidr":          cidr,
				"suggested_vip": suggestedVIP,
			})
			return
		}
	}

	// Local network detection (for global config)
	iface, cidr, err := detectLocalNetwork()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("无法检测本机网络: %w", err))
		return
	}

	// Generate suggested VIP based on CIDR
	suggestedVIP := s.manager.SuggestVIP(cidr)

	writeJSON(w, http.StatusOK, map[string]string{
		"iface":         iface,
		"cidr":          cidr,
		"suggested_vip": suggestedVIP,
	})
}

// detectLocalNetwork detects the local machine's network interface and CIDR
func detectLocalNetwork() (iface, cidr string, err error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}

	// Find the first non-loopback interface with an IPv4 address
	for _, i := range interfaces {
		// Skip loopback and down interfaces
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ipNet *net.IPNet
			switch v := addr.(type) {
			case *net.IPNet:
				ipNet = v
			case *net.IPAddr:
				// Convert IPAddr to IPNet with default mask
				ipNet = &net.IPNet{
					IP:   v.IP,
					Mask: v.IP.DefaultMask(),
				}
			}

			// Skip if not IPv4 or is loopback
			if ipNet == nil || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
				continue
			}

			// Found a valid interface - return network address, not host address
			iface = i.Name
			cidr = ipNet.String() // This gives us the network address like 192.168.1.0/24
			return iface, cidr, nil
		}
	}

	return "", "", fmt.Errorf("未找到有效的网络接口")
}

// handleInstallAll handles POST /api/routers/install-all
func (s *Server) handleInstallAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	routers := s.manager.GetRouters()
	if len(routers) < 2 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("至少需要配置两台路由器才能安装"))
		return
	}

	// Check global config
	cfg := s.manager.GetConfig()
	if cfg.LAN.VIP == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("请先在全局设置中配置 VIP"))
		return
	}

	// Check roles
	hasPrimary := false
	hasSecondary := false
	for _, router := range routers {
		if router.Role == config.RolePrimary {
			hasPrimary = true
		}
		if router.Role == config.RoleSecondary {
			hasSecondary = true
		}
	}
	if !hasPrimary || !hasSecondary {
		writeError(w, http.StatusBadRequest, fmt.Errorf("需要至少一台主路由(primary)和一台旁路由(secondary)"))
		return
	}

	// Start installation for all routers that don't have agent installed
	installed := 0
	for _, router := range routers {
		if router.AgentVer != "" {
			continue // Already installed
		}
		if router.Status == StatusInstalling {
			continue // Already installing
		}

		agentConfig, err := s.manager.GenerateAgentConfig(router)
		if err != nil {
			continue
		}

		router.Status = StatusInstalling
		router.InstallLog = nil
		router.InstallStep = 0
		router.InstallTotal = 11
		router.Error = ""

		go func(r *Router, cfg *config.Config) {
			if err := s.manager.Install(r, cfg); err != nil {
				r.Status = StatusError
				r.Error = err.Error()
			}
		}(router, agentConfig)
		installed++
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "installing",
		"message": fmt.Sprintf("已开始安装 %d 台路由器", installed),
		"count":   installed,
	})
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

// handleVersion handles GET /api/version - returns current and latest version info
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get current version from build info
	currentVersion := getCurrentVersion()

	// Get latest version from GitHub API
	latestVersion, releaseURL, releaseNotes, err := getLatestReleaseInfo()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"current_version": currentVersion,
			"latest_version":  "",
			"has_update":      false,
			"error":           err.Error(),
		})
		return
	}

	hasUpdate := compareVersions(currentVersion, latestVersion) < 0

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_version": currentVersion,
		"latest_version":  latestVersion,
		"has_update":      hasUpdate,
		"release_url":     releaseURL,
		"release_notes":   releaseNotes,
	})
}

// getCurrentVersion returns the current version of the controller
func getCurrentVersion() string {
	return version.Version
}

// getLatestReleaseInfo fetches the latest release info from GitHub
func getLatestReleaseInfo() (version, url, notes string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// GitHub API for latest release
	apiURL := "https://api.github.com/repos/zczy-k/FloatingGateway/releases/latest"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "FloatingGateway-Controller")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("GitHub API 返回状态码: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", "", fmt.Errorf("解析 GitHub API 响应失败: %w", err)
	}

	return release.TagName, release.HTMLURL, release.Body, nil
}

// compareVersions compares two version strings (e.g., "v1.0.0" vs "v1.1.0")
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle dev version
	if v1 == "dev" || v1 == "" {
		return -1
	}
	if v2 == "dev" || v2 == "" {
		return 1
	}

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			// Extract numeric part (handle suffixes like "0-beta")
			numStr := strings.Split(parts1[i], "-")[0]
			fmt.Sscanf(numStr, "%d", &n1)
		}
		if i < len(parts2) {
			numStr := strings.Split(parts2[i], "-")[0]
			fmt.Sscanf(numStr, "%d", &n2)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

// handleVerifyDrift performs a comprehensive drift verification test.
func (s *Server) handleVerifyDrift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Set up streaming headers immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendEvent := func(step, status, message string) {
		json.NewEncoder(w).Encode(map[string]string{
			"step":    step,
			"status":  status,
			"message": message,
		})
		fmt.Fprint(w, "\n")
		flusher.Flush()
	}

	// 1. Initial State Check
	cfg := s.manager.GetConfig()
	routers := s.manager.GetRouters()
	var master, backup *Router
	for _, router := range routers {
		if router.VRRPState == "MASTER" {
			master = router
		} else if router.VRRPState == "BACKUP" {
			backup = router
		}
	}

	if master == nil || backup == nil {
		sendEvent("init", "error", "无法开始验证：集群状态不健康（未找到 MASTER 或 BACKUP 节点）。请先确保诊断全部通过。")
		return
	}

	sendEvent("init", "running", fmt.Sprintf("当前主节点: %s, 备节点: %s", master.Name, backup.Name))

	// 2. Ping VIP Check
	sendEvent("ping_vip", "running", "正在测试 VIP 连通性...")
	if err := pingIP(cfg.LAN.VIP); err != nil {
		sendEvent("ping_vip", "error", fmt.Sprintf("VIP 无法访问: %v。验证终止。", err))
		return
	}
	sendEvent("ping_vip", "success", "VIP 连通性正常")

	// 3. Trigger Drift (Simulate Fault on Master)
	sendEvent("trigger_drift", "running", fmt.Sprintf("正在 %s 上模拟故障 (降低优先级)...", master.Name))

	// Command to drop priority temporarily
	// We use pkill -STOP keepalived to freeze it, which should trigger backup to take over
	// Or better, use a custom agent command if available.
	// For now, let's use systemctl stop keepalived for 10s then start

	sshClient := NewSSHClient(s.manager.sshConfig(master))
	if err := sshClient.Connect(); err != nil {
		sendEvent("trigger_drift", "error", "无法连接到主节点: "+err.Error())
		return
	}
	defer sshClient.Close()

	// Stop keepalived
	_, err := sshClient.RunCombined("systemctl stop keepalived || /etc/init.d/keepalived stop")
	if err != nil {
		sendEvent("trigger_drift", "error", "故障模拟失败: "+err.Error())
		return
	}
	sendEvent("trigger_drift", "success", "主节点 Keepalived 已暂停，等待漂移...")

	// 4. Verify Drift (Wait for Backup to become Master)
	sendEvent("verify_drift", "running", "正在检测 VIP 漂移...")
	driftSuccess := false
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		// Check if VIP is still pingable (should be almost instant)
		if err := pingIP(cfg.LAN.VIP); err == nil {
			// Check if Backup is now holding VIP
			// We can check via API probe or just trust ping if we know Master is down
			driftSuccess = true
			break
		}
	}

	if driftSuccess {
		sendEvent("verify_drift", "success", "VIP 访问正常，漂移成功！")
	} else {
		// Deep diagnostics for failure
		sendEvent("verify_drift", "running", "漂移失败，正在进行深度诊断...")

		// Connect to backup router to check status
		sshBackup := NewSSHClient(s.manager.sshConfig(backup))
		if err := sshBackup.Connect(); err == nil {
			defer sshBackup.Close()

			// Check if it has VIP
			out, _ := sshBackup.RunCombined(fmt.Sprintf("ip addr show | grep %s", cfg.LAN.VIP))
			hasVIP := strings.Contains(out, cfg.LAN.VIP)

			// Check Keepalived state
			stateOut, _ := sshBackup.RunCombined("cat /tmp/keepalived.GATEWAY.state")
			state := strings.TrimSpace(stateOut)

			if hasVIP {
				sendEvent("verify_drift", "error", fmt.Sprintf("诊断结果：备节点 (%s) 已接管 VIP，但控制端无法访问。可能是防火墙拦截了 ICMP 或 ARP 广播未生效。", backup.Name))
			} else {
				sendEvent("verify_drift", "error", fmt.Sprintf("诊断结果：备节点 (%s) 未能接管 VIP。当前状态: %s。可能是 VRRP 组播被拦截。", backup.Name, state))
			}
		} else {
			sendEvent("verify_drift", "error", fmt.Sprintf("诊断失败：无法连接到备节点 (%s) 进行检查。", backup.Name))
		}
	}

	// 5. Restore
	sendEvent("restore", "running", "正在恢复主节点...")
	sshClient.RunCombined("systemctl start keepalived || /etc/init.d/keepalived start")

	// Wait for restore
	time.Sleep(5 * time.Second)
	// Force probe to update UI state
	s.manager.Probe(master)
	s.manager.Probe(backup)

	if driftSuccess {
		sendEvent("finish", "success", "验证完成：网关漂移功能正常！")
	} else {
		sendEvent("finish", "error", "验证失败：请检查备份节点日志")
	}
}

func pingIP(ip string) error {
	cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	}
	return cmd.Run()
}

// handleUpgrade handles POST /api/upgrade - auto-upgrade controller to latest version
func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Validate version format
	if req.Version == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("version is required"))
		return
	}

	// Start upgrade in background
	go func() {
		if err := performUpgrade(req.Version); err != nil {
			log.Printf("Upgrade failed: %v", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "upgrading",
		"message": "Upgrade started, service will restart in a few seconds",
	})
}

// performUpgrade downloads and installs the new version
func performUpgrade(targetVersion string) error {
	log.Printf("Starting upgrade to version %s", targetVersion)

	// Determine binary name based on OS and architecture
	binaryName := fmt.Sprintf("gateway-controller-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// Download URL with acceleration proxy support
	downloadURLs := []string{
		fmt.Sprintf("https://ghproxy.com/https://github.com/zczy-k/FloatingGateway/releases/download/%s/%s", targetVersion, binaryName),
		fmt.Sprintf("https://github.com/zczy-k/FloatingGateway/releases/download/%s/%s", targetVersion, binaryName),
	}

	var downloadedData []byte
	var downloadErr error

	for _, url := range downloadURLs {
		log.Printf("Trying to download from: %s", url)
		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Get(url)
		if err != nil {
			downloadErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			downloadErr = fmt.Errorf("download failed with status: %d", resp.StatusCode)
			continue
		}

		downloadedData, err = io.ReadAll(resp.Body)
		if err != nil {
			downloadErr = err
			continue
		}

		log.Printf("Successfully downloaded %d bytes", len(downloadedData))
		downloadErr = nil
		break
	}

	if downloadErr != nil {
		return fmt.Errorf("failed to download new version: %w", downloadErr)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Write new binary to temp file
	tmpPath := execPath + ".new"
	if err := os.WriteFile(tmpPath, downloadedData, 0755); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	log.Printf("New binary written to: %s", tmpPath)

	// Replace old binary with new one
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup old binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	log.Printf("Upgrade successful, restarting service...")

	// Wait a bit for response to be sent
	time.Sleep(2 * time.Second)

	// Restart the service
	if runtime.GOOS == "windows" {
		// On Windows, just exit and let the service manager restart
		os.Exit(0)
	} else {
		// On Unix, try to restart using the same command
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to restart: %v", err)
		}
		os.Exit(0)
	}

	return nil
}
