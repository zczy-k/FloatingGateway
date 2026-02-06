package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"gopkg.in/yaml.v3"
)

// RouterStatus represents the state of a managed router.
type RouterStatus string

const (
	StatusUnknown      RouterStatus = "unknown"
	StatusOnline       RouterStatus = "online"
	StatusOffline      RouterStatus = "offline"
	StatusInstalling   RouterStatus = "installing"
	StatusUninstalling RouterStatus = "uninstalling"
	StatusError        RouterStatus = "error"
)

// Platform represents the detected remote platform.
type Platform string

const (
	PlatformUnknown Platform = "unknown"
	PlatformLinux   Platform = "linux"
	PlatformOpenWrt Platform = "openwrt"
)

// Router represents a managed router.
type Router struct {
	Name       string       `yaml:"name" json:"name"`
	Host       string       `yaml:"host" json:"host"`
	Port       int          `yaml:"port" json:"port"`
	User       string       `yaml:"user" json:"user"`
	Password   string       `yaml:"password,omitempty" json:"password,omitempty"`
	KeyFile    string       `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	Passphrase string       `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`
	Role       config.Role  `yaml:"role" json:"role"`
	Status     RouterStatus `yaml:"-" json:"status"`
	Platform   Platform     `yaml:"-" json:"platform"`
	LastSeen   time.Time    `yaml:"-" json:"last_seen,omitempty"`
	AgentVer   string       `yaml:"-" json:"agent_version,omitempty"`
	VRRPState  string       `yaml:"-" json:"vrrp_state,omitempty"`
	Healthy    *bool        `yaml:"-" json:"healthy,omitempty"`
	Error      string       `yaml:"-" json:"error,omitempty"`
}

// MarshalJSON customizes the JSON output to hide sensitive fields.
func (r *Router) MarshalJSON() ([]byte, error) {
	type Alias Router
	return json.Marshal(&struct {
		*Alias
		Password   string `json:"password,omitempty"`
		Passphrase string `json:"passphrase,omitempty"`
	}{
		Alias:      (*Alias)(r),
		Password:   "", // Hide sensitive data in API responses
		Passphrase: "",
	})
}

// ControllerConfig holds controller configuration.
type ControllerConfig struct {
	Version  int       `yaml:"version" json:"version"`
	Listen   string    `yaml:"listen" json:"listen"`
	Routers  []*Router `yaml:"routers" json:"routers"`
	AgentBin string    `yaml:"agent_bin" json:"agent_bin"` // Path to gateway-agent binary
	LAN      struct {
		VIP   string `yaml:"vip" json:"vip"`
		CIDR  string `yaml:"cidr" json:"cidr"`
		Iface string `yaml:"iface" json:"iface"`
	} `yaml:"lan" json:"lan"`
	Keepalived struct {
		VRID int `yaml:"vrid" json:"vrid"`
	} `yaml:"keepalived" json:"keepalived"`
	Health struct {
		Mode config.HealthMode `yaml:"mode" json:"mode"`
	} `yaml:"health" json:"health"`
}

// Manager handles router management operations.
type Manager struct {
	config     *ControllerConfig
	configPath string
	mu         sync.RWMutex
}

// NewManager creates a new router manager.
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{configPath: configPath}
	if err := m.loadConfig(); err != nil {
		return nil, err
	}
	return m, nil
}

// loadConfig loads the controller configuration.
func (m *Manager) loadConfig() error {
	cfg := &ControllerConfig{
		Version: 1,
		Listen:  ":8080",
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			m.config = cfg
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Set defaults
	for _, r := range cfg.Routers {
		if r.Port == 0 {
			r.Port = 22
		}
		r.Status = StatusUnknown
	}

	m.config = cfg
	return nil
}

// SaveConfig saves the current configuration.
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

// GetRouters returns all configured routers.
func (m *Manager) GetRouters() []*Router {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Routers
}

// GetRouter returns a router by name.
func (m *Manager) GetRouter(name string) *Router {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.config.Routers {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// AddRouter adds a new router to manage.
func (m *Manager) AddRouter(r *Router) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.config.Routers {
		if existing.Name == r.Name {
			return fmt.Errorf("router %q already exists", r.Name)
		}
	}

	if r.Port == 0 {
		r.Port = 22
	}
	r.Status = StatusUnknown

	m.config.Routers = append(m.config.Routers, r)
	return nil
}

// RemoveRouter removes a router from management.
func (m *Manager) RemoveRouter(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, r := range m.config.Routers {
		if r.Name == name {
			m.config.Routers = append(m.config.Routers[:i], m.config.Routers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("router %q not found", name)
}

// sshConfig returns SSH config for a router.
func (m *Manager) sshConfig(r *Router) *SSHConfig {
	return &SSHConfig{
		Host:       r.Host,
		Port:       r.Port,
		User:       r.User,
		Password:   r.Password,
		KeyFile:    r.KeyFile,
		Passphrase: r.Passphrase,
		Timeout:    30,
	}
}

// Probe checks a router's status and updates its state.
func (m *Manager) Probe(r *Router) error {
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		r.Status = StatusOffline
		r.Error = err.Error()
		return err
	}
	defer client.Close()

	r.Status = StatusOnline
	r.LastSeen = time.Now()
	r.Error = ""

	// Detect platform
	r.Platform = m.detectPlatform(client)

	// Auto-detect interface and CIDR if not set in global config
	if m.config.LAN.Iface == "" || m.config.LAN.CIDR == "" {
		if iface, cidr, err := m.DetectNetwork(client, r.Host); err == nil {
			m.mu.Lock()
			if m.config.LAN.Iface == "" {
				m.config.LAN.Iface = iface
			}
			if m.config.LAN.CIDR == "" {
				m.config.LAN.CIDR = cidr
			}
			m.mu.Unlock()
			m.SaveConfig()
		}
	}

	// Check agent version
	if ver, err := client.RunCombined("gateway-agent version 2>/dev/null"); err == nil {
		r.AgentVer = strings.TrimSpace(ver)
	}

	// Get agent status if installed
	if r.AgentVer != "" {
		if output, err := client.RunCombined("gateway-agent status --json 2>/dev/null"); err == nil {
			var status struct {
				Keepalived struct {
					VRRPState string `json:"vrrp_state"`
				} `json:"keepalived"`
				Health struct {
					Healthy bool `json:"healthy"`
				} `json:"health"`
			}
			if json.Unmarshal([]byte(output), &status) == nil {
				r.VRRPState = status.Keepalived.VRRPState
				healthy := status.Health.Healthy
				r.Healthy = &healthy
			}
		}
	}

	return nil
}

// detectPlatform detects the remote platform.
func (m *Manager) detectPlatform(client *SSHClient) Platform {
	// Check for OpenWrt
	if out, err := client.RunCombined("cat /etc/openwrt_release 2>/dev/null"); err == nil && out != "" {
		return PlatformOpenWrt
	}

	// Check for generic Linux
	if out, err := client.RunCombined("uname -s"); err == nil && strings.Contains(strings.ToLower(out), "linux") {
		return PlatformLinux
	}

	return PlatformUnknown
}

// DetectNetwork tries to find the interface and CIDR for a given IP on the remote host.
func (m *Manager) DetectNetwork(client *SSHClient, targetIP string) (iface, cidr string, err error) {
	// Try to get default interface
	out, err := client.RunCombined("ip route get 8.8.8.8")
	if err != nil || out == "" {
		out, err = client.RunCombined("ip route show default | head -n 1")
	}

	if err == nil && out != "" {
		fields := strings.Fields(out)
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
				break
			}
		}
	}

	// If still empty, try to find interface containing the target IP
	if iface == "" {
		out, _ = client.RunCombined(fmt.Sprintf("ip -4 addr show to %s", targetIP))
		if out != "" {
			re := regexp.MustCompile(`\d+:\s+(\S+):?`)
			matches := re.FindStringSubmatch(out)
			if len(matches) > 1 {
				iface = matches[1]
			}
		}
	}

	if iface != "" {
		// Get CIDR for this interface
		out, err = client.RunCombined(fmt.Sprintf("ip -4 addr show dev %s", iface))
		if err == nil && out != "" {
			re := regexp.MustCompile(`inet\s+([0-9./]+)`)
			matches := re.FindStringSubmatch(out)
			if len(matches) > 1 {
				cidrFull := matches[1]
				parts := strings.Split(cidrFull, "/")
				if len(parts) == 2 {
					ipDots := strings.Split(parts[0], ".")
					if len(ipDots) == 4 {
						cidr = fmt.Sprintf("%s.%s.%s.0/%s", ipDots[0], ipDots[1], ipDots[2], parts[1])
					}
				}
			}
		}
	}

	if iface == "" || cidr == "" {
		return "", "", fmt.Errorf("自动探测失败，请手动输入")
	}

	return iface, cidr, nil
}

// ProbeAll probes all routers concurrently.
func (m *Manager) ProbeAll() {
	var wg sync.WaitGroup
	for _, r := range m.GetRouters() {
		wg.Add(1)
		go func(router *Router) {
			defer wg.Done()
			m.Probe(router)
		}(r)
	}
	wg.Wait()
}

// Install installs the agent on a router.
func (m *Manager) Install(r *Router, agentConfig *config.Config) error {
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	r.Status = StatusInstalling

	// Detect platform
	platform := m.detectPlatform(client)
	r.Platform = platform

	// Determine target architecture
	arch, err := client.RunCombined("uname -m")
	if err != nil {
		return fmt.Errorf("detect arch: %w", err)
	}
	arch = strings.TrimSpace(arch)

	// Find appropriate binary
	binPath, err := m.findAgentBinary(platform, arch)
	if err != nil {
		return fmt.Errorf("find binary: %w", err)
	}

	// Read binary
	binData, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("read binary: %w", err)
	}

	// Create config directory
	if err := client.MkdirAll("/etc/gateway-agent"); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Upload binary
	if err := client.WriteFile("/usr/bin/gateway-agent", binData, 0755); err != nil {
		return fmt.Errorf("upload binary: %w", err)
	}

	// Generate and upload config
	agentConfig.Role = r.Role
	configData, err := agentConfig.ToYAML()
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	if err := client.WriteFile("/etc/gateway-agent/config.yaml", configData, 0644); err != nil {
		return fmt.Errorf("upload config: %w", err)
	}

	// Install keepalived
	if err := m.installKeepalived(client, platform); err != nil {
		return fmt.Errorf("install keepalived: %w", err)
	}

	// Apply agent config (generates keepalived.conf)
	if _, err := client.RunCombined("gateway-agent apply"); err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	// Setup service
	if err := m.setupService(client, platform); err != nil {
		return fmt.Errorf("setup service: %w", err)
	}

	// Setup firewall
	if err := m.setupFirewall(client, platform); err != nil {
		// Log but don't fail, as some platforms might not have firewall enabled
		fmt.Printf("Warning: setup firewall for %s failed: %v\n", r.Name, err)
	}

	r.Status = StatusOnline
	return nil
}

// setupFirewall configures firewall to allow VRRP.
func (m *Manager) setupFirewall(client *SSHClient, platform Platform) error {
	switch platform {
	case PlatformOpenWrt:
		// Add traffic rule for VRRP (protocol 112)
		rules := []string{
			"uci delete firewall.vrrp 2>/dev/null",
			"uci set firewall.vrrp=rule",
			"uci set firewall.vrrp.name='Allow-VRRP'",
			"uci set firewall.vrrp.src='lan'",
			"uci set firewall.vrrp.dest='*'",
			"uci set firewall.vrrp.proto='112'",
			"uci set firewall.vrrp.target='ACCEPT'",
			"uci commit firewall",
			"/etc/init.d/firewall restart",
		}
		for _, cmd := range rules {
			client.RunCombined(cmd)
		}
		return nil

	case PlatformLinux:
		// Try UFW
		if _, err := client.RunCombined("which ufw"); err == nil {
			client.RunCombined("ufw allow proto vrrp")
			return nil
		}
		// Try firewalld
		if _, err := client.RunCombined("which firewall-cmd"); err == nil {
			client.RunCombined("firewall-cmd --permanent --add-rich-rule='rule protocol value=\"vrrp\" accept'")
			client.RunCombined("firewall-cmd --reload")
			return nil
		}
		// Try iptables directly
		client.RunCombined("iptables -I INPUT -p vrrp -j ACCEPT")
		return nil
	}
	return nil
}

// Doctor runs a remote diagnostic on the router and returns the report.
func (m *Manager) Doctor(r *Router) (string, error) {
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	// Run doctor and get JSON output
	output, err := client.RunCombined("gateway-agent doctor --json")
	if err != nil {
		return "", fmt.Errorf("run doctor: %w (output: %s)", err, output)
	}

	return output, nil
}

// CheckVIPConflict checks if the VIP is currently reachable on the network.
func (m *Manager) CheckVIPConflict(vip string) (bool, error) {
	// Use ping to check if VIP is reachable
	// We use -c 1 and a short timeout
	cmd := "ping"
	args := []string{"-c", "1", "-W", "1", vip}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", "1000", vip}
	}

	_, err := exec.Command(cmd, args...).CombinedOutput()
	// If err is nil, it means ping succeeded (VIP is reachable)
	return err == nil, nil
}

// findAgentBinary finds the appropriate agent binary.
func (m *Manager) findAgentBinary(platform Platform, arch string) (string, error) {
	// Normalize architecture
	goarch := arch
	switch arch {
	case "x86_64", "amd64":
		goarch = "amd64"
	case "aarch64", "arm64":
		goarch = "arm64"
	case "armv7l", "armv6l":
		goarch = "arm"
	case "mips", "mipsel":
		goarch = arch
	}

	goos := "linux"

	// Check configured path first
	if m.config.AgentBin != "" {
		if _, err := os.Stat(m.config.AgentBin); err == nil {
			return m.config.AgentBin, nil
		}
	}

	// Look for platform-specific binary
	patterns := []string{
		fmt.Sprintf("gateway-agent-%s-%s", goos, goarch),
		fmt.Sprintf("gateway-agent_%s_%s", goos, goarch),
		"gateway-agent",
	}

	searchDirs := []string{
		".",
		"./bin",
		"./dist",
	}

	for _, dir := range searchDirs {
		for _, pattern := range patterns {
			path := fmt.Sprintf("%s/%s", dir, pattern)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("no binary found for %s/%s (local GOOS=%s)", goos, goarch, runtime.GOOS)
}

// installKeepalived installs keepalived on the remote system.
func (m *Manager) installKeepalived(client *SSHClient, platform Platform) error {
	// Check if already installed
	if _, err := client.RunCombined("which keepalived"); err == nil {
		return nil
	}

	switch platform {
	case PlatformOpenWrt:
		// Try install directly first, then try update if it fails
		if _, err := client.RunCombined("opkg install keepalived"); err == nil {
			return nil
		}
		_, err := client.RunCombined("opkg update && opkg install keepalived")
		return err
	case PlatformLinux:
		// Try apt first, then yum
		if _, err := client.RunCombined("apt-get install -y keepalived"); err == nil {
			return nil
		}
		if _, err := client.RunCombined("apt-get update && apt-get install -y keepalived"); err == nil {
			return nil
		}
		if _, err := client.RunCombined("yum install -y keepalived"); err == nil {
			return nil
		}
		return fmt.Errorf("failed to install keepalived")
	}

	return fmt.Errorf("unsupported platform: %s", platform)
}

// setupService sets up the gateway-agent service.
func (m *Manager) setupService(client *SSHClient, platform Platform) error {
	switch platform {
	case PlatformOpenWrt:
		return m.setupProcdService(client)
	case PlatformLinux:
		return m.setupSystemdService(client)
	}
	return fmt.Errorf("unsupported platform: %s", platform)
}

// setupProcdService sets up a procd service on OpenWrt.
func (m *Manager) setupProcdService(client *SSHClient) error {
	initScript := `#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/gateway-agent run
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
`
	if err := client.WriteFile("/etc/init.d/gateway-agent", []byte(initScript), 0755); err != nil {
		return err
	}

	// Enable and start
	cmds := []string{
		"/etc/init.d/gateway-agent enable",
		"/etc/init.d/keepalived enable",
		"/etc/init.d/keepalived start",
	}
	for _, cmd := range cmds {
		client.RunCombined(cmd)
	}

	return nil
}

// setupSystemdService sets up a systemd service on Linux.
func (m *Manager) setupSystemdService(client *SSHClient) error {
	unitFile := `[Unit]
Description=Gateway Agent
After=network.target keepalived.service
Wants=keepalived.service

[Service]
Type=simple
ExecStart=/usr/bin/gateway-agent run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	if err := client.WriteFile("/etc/systemd/system/gateway-agent.service", []byte(unitFile), 0644); err != nil {
		return err
	}

	// Reload and start
	cmds := []string{
		"systemctl daemon-reload",
		"systemctl enable keepalived",
		"systemctl start keepalived",
		"systemctl enable gateway-agent",
		"systemctl start gateway-agent",
	}
	for _, cmd := range cmds {
		if _, err := client.RunCombined(cmd); err != nil {
			return fmt.Errorf("run %q: %w", cmd, err)
		}
	}

	return nil
}

// Uninstall removes the agent from a router.
func (m *Manager) Uninstall(r *Router) error {
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	r.Status = StatusUninstalling

	platform := m.detectPlatform(client)

	// Stop services
	switch platform {
	case PlatformOpenWrt:
		client.RunCombined("/etc/init.d/gateway-agent stop")
		client.RunCombined("/etc/init.d/gateway-agent disable")
		client.RemoveFile("/etc/init.d/gateway-agent")
	case PlatformLinux:
		client.RunCombined("systemctl stop gateway-agent")
		client.RunCombined("systemctl disable gateway-agent")
		client.RemoveFile("/etc/systemd/system/gateway-agent.service")
		client.RunCombined("systemctl daemon-reload")
	}

	// Remove files
	client.RemoveFile("/usr/bin/gateway-agent")
	client.RunCombined("rm -rf /etc/gateway-agent")

	// Restore keepalived default config
	client.RunCombined("systemctl stop keepalived 2>/dev/null || /etc/init.d/keepalived stop 2>/dev/null")

	r.Status = StatusOnline
	r.AgentVer = ""
	r.VRRPState = ""
	r.Healthy = nil

	return nil
}

// GetConfig returns the controller configuration.
func (m *Manager) GetConfig() *ControllerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GenerateAgentConfig creates a config.Config for a router.
func (m *Manager) GenerateAgentConfig(r *Router) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Role = r.Role
	cfg.LAN.VIP = m.config.LAN.VIP
	cfg.LAN.CIDR = m.config.LAN.CIDR
	cfg.LAN.Iface = m.config.LAN.Iface
	cfg.Keepalived.VRID = m.config.Keepalived.VRID
	if m.config.Health.Mode != "" {
		cfg.Health.Mode = m.config.Health.Mode
	}

	// Find peer
	for _, other := range m.config.Routers {
		if other.Name != r.Name {
			cfg.Routers.PeerIP = other.Host
			break
		}
	}

	return cfg
}

// ValidateConfig validates the controller configuration.
func (m *Manager) ValidateConfig() error {
	if len(m.config.Routers) == 0 {
		return fmt.Errorf("no routers configured")
	}

	if m.config.LAN.VIP == "" {
		return fmt.Errorf("lan.vip is required")
	}

	// Check VIP format
	if matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, m.config.LAN.VIP); !matched {
		return fmt.Errorf("invalid VIP format: %s", m.config.LAN.VIP)
	}

	// Validate each router
	hasPrimary := false
	hasSecondary := false
	for _, r := range m.config.Routers {
		if r.Name == "" {
			return fmt.Errorf("router name is required")
		}
		if r.Host == "" {
			return fmt.Errorf("router %s: host is required", r.Name)
		}
		if r.User == "" {
			return fmt.Errorf("router %s: user is required", r.Name)
		}
		if r.Role == config.RolePrimary {
			hasPrimary = true
		}
		if r.Role == config.RoleSecondary {
			hasSecondary = true
		}
	}

	if !hasPrimary {
		return fmt.Errorf("no primary router configured")
	}
	if !hasSecondary {
		return fmt.Errorf("no secondary router configured")
	}

	return nil
}
