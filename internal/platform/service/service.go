// Package service provides system service management.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/platform/detect"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

// Manager provides service management operations.
type Manager interface {
	Install(name string, config ServiceConfig) error
	Uninstall(name string) error
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Reload(name string) error
	Enable(name string) error
	Disable(name string) error
	Status(name string) (ServiceStatus, error)
	IsRunning(name string) bool
}

// ServiceConfig holds service configuration.
type ServiceConfig struct {
	Description string
	ExecPath    string
	Args        []string
	WorkingDir  string
	User        string
	Group       string
	Restart     string // always, on-failure, no
	ConfigFile  string
}

// ServiceStatus holds service status information.
type ServiceStatus struct {
	Name    string
	Running bool
	Enabled bool
	PID     int
	Error   string
}

// NewManager creates a service manager for the current platform.
func NewManager() Manager {
	info := detect.Detect()
	switch info.ServiceManager {
	case detect.ServiceManagerSystemd:
		return &SystemdManager{}
	case detect.ServiceManagerProcd:
		return &ProcdManager{}
	default:
		return &NoopManager{}
	}
}

// SystemdManager manages systemd services.
type SystemdManager struct{}

func (m *SystemdManager) Install(name string, config ServiceConfig) error {
	unitContent := m.generateUnit(name, config)
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	// Reload systemd
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "daemon-reload")
	if !result.Success() {
		return fmt.Errorf("daemon-reload failed: %s", result.Combined())
	}

	return nil
}

func (m *SystemdManager) generateUnit(name string, config ServiceConfig) string {
	var sb strings.Builder
	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=%s\n", config.Description))
	sb.WriteString("After=network.target\n")
	sb.WriteString("Wants=network-online.target\n\n")
	
	sb.WriteString("[Service]\n")
	sb.WriteString("Type=simple\n")
	
	execStart := config.ExecPath
	if len(config.Args) > 0 {
		execStart += " " + strings.Join(config.Args, " ")
	}
	sb.WriteString(fmt.Sprintf("ExecStart=%s\n", execStart))
	
	if config.WorkingDir != "" {
		sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", config.WorkingDir))
	}
	if config.User != "" {
		sb.WriteString(fmt.Sprintf("User=%s\n", config.User))
	}
	if config.Group != "" {
		sb.WriteString(fmt.Sprintf("Group=%s\n", config.Group))
	}
	
	restart := config.Restart
	if restart == "" {
		restart = "on-failure"
	}
	sb.WriteString(fmt.Sprintf("Restart=%s\n", restart))
	sb.WriteString("RestartSec=5\n")
	
	// Environment
	if config.ConfigFile != "" {
		sb.WriteString(fmt.Sprintf("Environment=CONFIG_FILE=%s\n", config.ConfigFile))
	}
	
	sb.WriteString("\n[Install]\n")
	sb.WriteString("WantedBy=multi-user.target\n")
	
	return sb.String()
}

func (m *SystemdManager) Uninstall(name string) error {
	// Stop first
	m.Stop(name)
	m.Disable(name)
	
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", name)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	
	exec.RunWithTimeout("systemctl", 10*time.Second, "daemon-reload")
	return nil
}

func (m *SystemdManager) Start(name string) error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "start", name)
	if !result.Success() {
		return fmt.Errorf("start failed: %s", result.Combined())
	}
	return nil
}

func (m *SystemdManager) Stop(name string) error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "stop", name)
	if !result.Success() && !strings.Contains(result.Stderr, "not loaded") {
		return fmt.Errorf("stop failed: %s", result.Combined())
	}
	return nil
}

func (m *SystemdManager) Restart(name string) error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "restart", name)
	if !result.Success() {
		return fmt.Errorf("restart failed: %s", result.Combined())
	}
	return nil
}

func (m *SystemdManager) Reload(name string) error {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "reload", name)
	if !result.Success() {
		// Try restart if reload not supported
		return m.Restart(name)
	}
	return nil
}

func (m *SystemdManager) Enable(name string) error {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "enable", name)
	if !result.Success() {
		return fmt.Errorf("enable failed: %s", result.Combined())
	}
	return nil
}

func (m *SystemdManager) Disable(name string) error {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "disable", name)
	if !result.Success() && !strings.Contains(result.Stderr, "not loaded") {
		return fmt.Errorf("disable failed: %s", result.Combined())
	}
	return nil
}

func (m *SystemdManager) Status(name string) (ServiceStatus, error) {
	status := ServiceStatus{Name: name}
	
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "is-active", name)
	status.Running = result.ExitCode == 0
	
	result = exec.RunWithTimeout("systemctl", 10*time.Second, "is-enabled", name)
	status.Enabled = result.ExitCode == 0
	
	// Get PID
	result = exec.RunWithTimeout("systemctl", 10*time.Second, "show", "-p", "MainPID", name)
	if result.Success() {
		fmt.Sscanf(strings.TrimPrefix(result.Stdout, "MainPID="), "%d", &status.PID)
	}
	
	return status, nil
}

func (m *SystemdManager) IsRunning(name string) bool {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "is-active", name)
	return result.ExitCode == 0
}

// ProcdManager manages procd services (OpenWrt).
type ProcdManager struct{}

func (m *ProcdManager) Install(name string, config ServiceConfig) error {
	scriptContent := m.generateInitScript(name, config)
	scriptPath := fmt.Sprintf("/etc/init.d/%s", name)
	
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("write init script: %w", err)
	}
	
	return nil
}

func (m *ProcdManager) generateInitScript(name string, config ServiceConfig) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh /etc/rc.common\n\n")
	sb.WriteString("START=99\n")
	sb.WriteString("STOP=01\n")
	sb.WriteString("USE_PROCD=1\n\n")
	
	sb.WriteString(fmt.Sprintf("PROG=%s\n", config.ExecPath))
	if config.ConfigFile != "" {
		sb.WriteString(fmt.Sprintf("CONFIG=%s\n", config.ConfigFile))
	}
	sb.WriteString("\n")
	
	sb.WriteString("start_service() {\n")
	sb.WriteString("    procd_open_instance\n")
	sb.WriteString("    procd_set_param command $PROG")
	for _, arg := range config.Args {
		sb.WriteString(fmt.Sprintf(" %s", arg))
	}
	sb.WriteString("\n")
	sb.WriteString("    procd_set_param respawn ${respawn_threshold:-3600} ${respawn_timeout:-5} ${respawn_retry:-5}\n")
	sb.WriteString("    procd_set_param stderr 1\n")
	sb.WriteString("    procd_set_param stdout 1\n")
	if config.ConfigFile != "" {
		sb.WriteString("    procd_set_param file $CONFIG\n")
	}
	sb.WriteString("    procd_close_instance\n")
	sb.WriteString("}\n\n")
	
	sb.WriteString("service_triggers() {\n")
	sb.WriteString("    procd_add_reload_trigger \"gateway-agent\"\n")
	sb.WriteString("}\n")
	
	return sb.String()
}

func (m *ProcdManager) Uninstall(name string) error {
	m.Stop(name)
	m.Disable(name)
	
	scriptPath := fmt.Sprintf("/etc/init.d/%s", name)
	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove init script: %w", err)
	}
	
	return nil
}

func (m *ProcdManager) Start(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 30*time.Second, "start")
	if !result.Success() {
		return fmt.Errorf("start failed: %s", result.Combined())
	}
	return nil
}

func (m *ProcdManager) Stop(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 30*time.Second, "stop")
	if !result.Success() {
		return fmt.Errorf("stop failed: %s", result.Combined())
	}
	return nil
}

func (m *ProcdManager) Restart(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 30*time.Second, "restart")
	if !result.Success() {
		return fmt.Errorf("restart failed: %s", result.Combined())
	}
	return nil
}

func (m *ProcdManager) Reload(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 10*time.Second, "reload")
	if !result.Success() {
		return m.Restart(name)
	}
	return nil
}

func (m *ProcdManager) Enable(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 10*time.Second, "enable")
	if !result.Success() {
		return fmt.Errorf("enable failed: %s", result.Combined())
	}
	return nil
}

func (m *ProcdManager) Disable(name string) error {
	result := exec.RunWithTimeout(filepath.Join("/etc/init.d", name), 10*time.Second, "disable")
	if !result.Success() {
		return fmt.Errorf("disable failed: %s", result.Combined())
	}
	return nil
}

func (m *ProcdManager) Status(name string) (ServiceStatus, error) {
	status := ServiceStatus{Name: name}
	
	// Check if running via pgrep
	result := exec.RunWithTimeout("pgrep", 5*time.Second, "-f", name)
	status.Running = result.ExitCode == 0
	if status.Running && result.Stdout != "" {
		fmt.Sscanf(strings.TrimSpace(result.Stdout), "%d", &status.PID)
	}
	
	// Check if enabled
	linkPath := fmt.Sprintf("/etc/rc.d/S99%s", name)
	if _, err := os.Stat(linkPath); err == nil {
		status.Enabled = true
	}
	
	return status, nil
}

func (m *ProcdManager) IsRunning(name string) bool {
	result := exec.RunWithTimeout("pgrep", 5*time.Second, "-f", name)
	return result.ExitCode == 0
}

// NoopManager is a no-operation service manager.
type NoopManager struct{}

func (m *NoopManager) Install(name string, config ServiceConfig) error {
	return fmt.Errorf("service management not supported on this platform")
}

func (m *NoopManager) Uninstall(name string) error {
	return nil
}

func (m *NoopManager) Start(name string) error {
	return fmt.Errorf("service management not supported on this platform")
}

func (m *NoopManager) Stop(name string) error {
	return nil
}

func (m *NoopManager) Restart(name string) error {
	return fmt.Errorf("service management not supported on this platform")
}

func (m *NoopManager) Reload(name string) error {
	return fmt.Errorf("service management not supported on this platform")
}

func (m *NoopManager) Enable(name string) error {
	return fmt.Errorf("service management not supported on this platform")
}

func (m *NoopManager) Disable(name string) error {
	return nil
}

func (m *NoopManager) Status(name string) (ServiceStatus, error) {
	return ServiceStatus{Name: name}, nil
}

func (m *NoopManager) IsRunning(name string) bool {
	return false
}
