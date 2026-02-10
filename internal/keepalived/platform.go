// Package keepalived provides keepalived configuration rendering and management.
package keepalived

import (
	"fmt"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

// Platform defines the interface for OS-specific keepalived operations.
type Platform interface {
	FindConfigPath() string
	Reload() error
	Start() error
	Stop() error
	Enable() error
}

// LinuxPlatform implements Platform for standard Linux (Systemd).
type LinuxPlatform struct{}

func (p *LinuxPlatform) FindConfigPath() string {
	paths := []string{
		"/etc/keepalived/keepalived.conf",
		"/etc/keepalived.conf",
	}
	for _, path := range paths {
		return path // Return first standard path for Linux
	}
	return "/etc/keepalived/keepalived.conf"
}

func (p *LinuxPlatform) Reload() error {
	// Try reload first
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "reload", "keepalived")
	if result.Success() {
		return nil
	}
	
	// If reload failed, check if service is running. If not, start it.
	// Or simply try restart which handles both cases
	result = exec.RunWithTimeout("systemctl", 30*time.Second, "restart", "keepalived")
	if result.Success() {
		return nil
	}
	
	// Last resort: try manual signal
	result = exec.RunWithTimeout("killall", 5*time.Second, "-HUP", "keepalived")
	if result.Success() {
		return nil
	}
	
	return fmt.Errorf("linux reload/restart failed: %s", result.Combined())
}

func (p *LinuxPlatform) Start() error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "start", "keepalived")
	if !result.Success() {
		return fmt.Errorf("systemctl start failed: %s", result.Combined())
	}
	return nil
}

func (p *LinuxPlatform) Stop() error {
	result := exec.RunWithTimeout("systemctl", 30*time.Second, "stop", "keepalived")
	if !result.Success() {
		return fmt.Errorf("systemctl stop failed: %s", result.Combined())
	}
	return nil
}

func (p *LinuxPlatform) Enable() error {
	result := exec.RunWithTimeout("systemctl", 10*time.Second, "enable", "keepalived")
	if !result.Success() {
		return fmt.Errorf("systemctl enable failed: %s", result.Combined())
	}
	return nil
}

// OpenWrtPlatform implements Platform for OpenWrt (Procd/Init.d).
type OpenWrtPlatform struct{}

func (p *OpenWrtPlatform) FindConfigPath() string {
	return "/tmp/keepalived.conf"
}

func (p *OpenWrtPlatform) Reload() error {
	// Directly restart process to ensure config is picked up
	// We cannot use /etc/init.d/keepalived because it overwrites our config
	
	// 1. Kill existing process
	exec.RunWithTimeout("killall", 5*time.Second, "keepalived")
	
	// 2. Start new process with our config
	// -n: don't fork (let agent manage it? no, better let it fork for now to avoid blocking)
	// Actually, we want it to run in background.
	// -D: log to syslog
	// -f: config file
	
	// Note: We use nohup or & equivalent to detach, but exec.Command waits.
	// Keepalived forks by default without -n.
	
	result := exec.RunWithTimeout("/usr/sbin/keepalived", 5*time.Second, "-f", "/tmp/keepalived.conf", "-D")
	if !result.Success() {
		return fmt.Errorf("failed to start keepalived binary: %s", result.Combined())
	}
	
	return nil
}

func (p *OpenWrtPlatform) Start() error {
	// Same as Reload for now - just ensure it's running
	return p.Reload()
}

func (p *OpenWrtPlatform) Stop() error {
	// Stop system service first to be safe
	exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "stop")
	
	// Kill process
	result := exec.RunWithTimeout("killall", 5*time.Second, "keepalived")
	if !result.Success() && result.ExitCode != 1 { // 1 means no process found
		return fmt.Errorf("killall failed: %s", result.Combined())
	}
	return nil
}

func (p *OpenWrtPlatform) Enable() error {
	// Disable system service to prevent it from interfering on reboot
	// Agent should be responsible for starting it
	exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "disable")
	return nil
}
