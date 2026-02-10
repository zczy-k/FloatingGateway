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
	result := exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "reload")
	if result.Success() {
		return nil
	}
	// Fallback to restart if reload not supported
	result = exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "restart")
	if !result.Success() {
		return fmt.Errorf("openwrt reload/restart failed: %s", result.Combined())
	}
	return nil
}

func (p *OpenWrtPlatform) Start() error {
	result := exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "start")
	if !result.Success() {
		return fmt.Errorf("init.d start failed: %s", result.Combined())
	}
	return nil
}

func (p *OpenWrtPlatform) Stop() error {
	result := exec.RunWithTimeout("/etc/init.d/keepalived", 30*time.Second, "stop")
	if !result.Success() {
		return fmt.Errorf("init.d stop failed: %s", result.Combined())
	}
	return nil
}

func (p *OpenWrtPlatform) Enable() error {
	result := exec.RunWithTimeout("/etc/init.d/keepalived", 10*time.Second, "enable")
	if !result.Success() {
		return fmt.Errorf("init.d enable failed: %s", result.Combined())
	}
	return nil
}
