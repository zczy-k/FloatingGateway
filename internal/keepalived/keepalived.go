// Package keepalived provides keepalived configuration rendering and management.
package keepalived

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

// ConfigPaths are possible keepalived config file locations.
var ConfigPaths = []string{
	"/etc/keepalived/keepalived.conf",
	"/etc/keepalived.conf",
}

// TemplateData holds data for keepalived config template.
type TemplateData struct {
	Role            string
	Interface       string
	VirtualRouterID int
	Priority        int
	AdvertInt       int
	VIP             string
	PeerIP          string
	SelfIP          string
	Preempt         bool
	PreemptDelay    int
	HealthMode      string
	CheckInterval   int
	CheckScript     string
	TrackWeight     int
	AgentBinary     string
}

const keepalivedTemplate = `# Gateway Agent Keepalived Configuration
# Role: {{ .Role }}
# Generated at: {{ now }}

global_defs {
    script_user root
    enable_script_security
}

vrrp_script chk_gateway {
    script "{{ .AgentBinary }} check --mode={{ .HealthMode }}"
    interval {{ .CheckInterval }}
    weight {{ .TrackWeight }}
    fall 3
    rise 2
    init_fail
}

vrrp_instance GATEWAY {
    state BACKUP
    interface {{ .Interface }}
    virtual_router_id {{ .VirtualRouterID }}
    priority {{ .Priority }}
    advert_int {{ .AdvertInt }}
    {{ if .Preempt }}preempt_delay {{ .PreemptDelay }}{{ else }}nopreempt{{ end }}

    authentication {
        auth_type PASS
        auth_pass gateway
    }

    unicast_src_ip {{ .SelfIP }}
    unicast_peer {
        {{ .PeerIP }}
    }

    virtual_ipaddress {
        {{ .VIP }}/32 dev {{ .Interface }}
    }

    track_script {
        chk_gateway
    }

    notify_master "/bin/sh -c 'PATH=/usr/bin:/usr/local/bin:/bin:/sbin:$PATH {{ .AgentBinary }} notify master'"
    notify_backup "/bin/sh -c 'PATH=/usr/bin:/usr/local/bin:/bin:/sbin:$PATH {{ .AgentBinary }} notify backup'"
    notify_fault  "/bin/sh -c 'PATH=/usr/bin:/usr/local/bin:/bin:/sbin:$PATH {{ .AgentBinary }} notify fault'"
}
`

// Renderer handles keepalived configuration rendering.
type Renderer struct {
	cfg *config.Config
}

// NewRenderer creates a new keepalived config renderer.
func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

// Render generates the keepalived configuration.
func (r *Renderer) Render() (string, error) {
	data := r.buildTemplateData()
	
	funcMap := template.FuncMap{
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
	}

	tmpl, err := template.New("keepalived").Funcs(funcMap).Parse(keepalivedTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func (r *Renderer) buildTemplateData() *TemplateData {
	// Determine track weight based on role
	// Secondary uses negative weight to drop priority when unhealthy
	trackWeight := 0
	if r.cfg.Role == config.RoleSecondary {
		trackWeight = -200 // Enough to drop below primary's priority
	}

	// Find agent binary path
	agentBinary := FindAgentBinary()

	return &TemplateData{
		Role:            string(r.cfg.Role),
		Interface:       r.cfg.LAN.Iface,
		VirtualRouterID: r.cfg.Keepalived.VRID,
		Priority:        r.cfg.GetPriority(),
		AdvertInt:       r.cfg.Keepalived.AdvertInt,
		VIP:             r.cfg.LAN.VIP,
		PeerIP:          r.cfg.Routers.PeerIP,
		SelfIP:          r.cfg.Routers.SelfIP,
		Preempt:         r.cfg.Failover.Preempt,
		PreemptDelay:    r.cfg.Failover.PreemptDelaySec,
		HealthMode:      string(r.cfg.Health.Mode),
		CheckInterval:   r.cfg.Health.IntervalSec,
		CheckScript:     fmt.Sprintf("%s check --mode=%s", agentBinary, r.cfg.Health.Mode),
		TrackWeight:     trackWeight,
		AgentBinary:     agentBinary,
	}
}

func FindAgentBinary() string {
	// Standard installation paths for gateway-agent
	paths := []string{
		"/usr/bin/gateway-agent",
		"/usr/local/bin/gateway-agent",
		"/etc/gateway-agent/gateway-agent", // Possible custom location
	}
	// Try to get executable path of current running process if we are the agent
	if exe, err := os.Executable(); err == nil {
		if strings.Contains(exe, "gateway-agent") {
			return exe
		}
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Check in PATH as fallback
	if path, err := exec.Which("gateway-agent"); err == nil {
		return path
	}
	// Default to standard location
	return "/usr/bin/gateway-agent"
}

// FindConfigPath finds the keepalived config file path.
func FindConfigPath() string {
	for _, p := range ConfigPaths {
		dir := filepath.Dir(p)
		if _, err := os.Stat(dir); err == nil {
			return p
		}
	}
	// Default to first path
	return ConfigPaths[0]
}

// Apply writes the config and reloads keepalived.
func Apply(cfg *config.Config) error {
	renderer := NewRenderer(cfg)
	content, err := renderer.Render()
	if err != nil {
		return fmt.Errorf("render config: %w", err)
	}

	configPath := FindConfigPath()
	
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Write to temp file first for atomic operation
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename config: %w", err)
	}

	// Reload keepalived
	if err := Reload(); err != nil {
		return fmt.Errorf("reload keepalived: %w", err)
	}

	return nil
}

// Reload reloads the keepalived service.
func Reload() error {
	// Try systemctl first
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "reload", "keepalived")
	if result.Success() {
		return nil
	}

	// Try init.d
	result = exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "reload")
	if result.Success() {
		return nil
	}

	// Try HUP signal
	result = exec.RunWithTimeout("killall", 5*time.Second, "-HUP", "keepalived")
	if result.Success() {
		return nil
	}

	// Last resort: restart
	result = exec.RunWithTimeout("systemctl", 30*time.Second, "restart", "keepalived")
	if result.Success() {
		return nil
	}

	result = exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "restart")
	if result.Success() {
		return nil
	}

	return fmt.Errorf("failed to reload keepalived")
}

// Status returns the keepalived service status.
type Status struct {
	Running     bool   `json:"running"`
	ConfigPath  string `json:"config_path"`
	ConfigValid bool   `json:"config_valid"`
	VRRPState   string `json:"vrrp_state"`
	Priority    int    `json:"priority"`
	Error       string `json:"error,omitempty"`
}

// GetStatus returns the current keepalived status.
func GetStatus() *Status {
	status := &Status{
		ConfigPath: FindConfigPath(),
	}

	// Check if running
	result := exec.RunWithTimeout("pgrep", 5*time.Second, "-x", "keepalived")
	status.Running = result.Success()

	// Check config validity
	if _, err := os.Stat(status.ConfigPath); err == nil {
		result = exec.RunWithTimeout("keepalived", 5*time.Second, "-t", "-f", status.ConfigPath)
		status.ConfigValid = result.Success()
		if !status.ConfigValid {
			status.Error = result.Combined()
		}
	}

	// Try to get VRRP state
	// This is platform-specific and may not always work
	result = exec.RunWithTimeout("cat", 5*time.Second, "/tmp/keepalived.GATEWAY.state")
	if result.Success() {
		state := strings.TrimSpace(result.Stdout)
		status.VRRPState = state
	}

	return status
}

// IsRunning checks if keepalived is running.
func IsRunning() bool {
	result := exec.RunWithTimeout("pgrep", 5*time.Second, "-x", "keepalived")
	return result.Success()
}

// Start starts the keepalived service.
func Start() error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "start", "keepalived")
	if result.Success() {
		return nil
	}

	result = exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "start")
	if result.Success() {
		return nil
	}

	return fmt.Errorf("failed to start keepalived: %s", result.Combined())
}

// Stop stops the keepalived service.
func Stop() error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "stop", "keepalived")
	if result.Success() {
		return nil
	}

	result = exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "stop")
	if result.Success() {
		return nil
	}

	return fmt.Errorf("failed to stop keepalived: %s", result.Combined())
}

// Enable enables the keepalived service at boot.
func Enable() error {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "enable", "keepalived")
	if result.Success() {
		return nil
	}

	result = exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "enable")
	if result.Success() {
		return nil
	}

	return fmt.Errorf("failed to enable keepalived: %s", result.Combined())
}
