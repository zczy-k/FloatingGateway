// Package keepalived provides keepalived configuration rendering and management.
package keepalived

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
)

var currentPlatform Platform

func init() {
	// Detect platform at startup
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		currentPlatform = &OpenWrtPlatform{}
	} else if _, err := os.Stat("/etc/openwrt_version"); err == nil {
		currentPlatform = &OpenWrtPlatform{}
	} else {
		currentPlatform = &LinuxPlatform{}
	}
}

// FindConfigPath finds the keepalived config file path.
func FindConfigPath() string {
	return currentPlatform.FindConfigPath()
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
	return currentPlatform.Reload()
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

	status.Running = IsRunning()

	// Check config validity
	if _, err := os.Stat(status.ConfigPath); err == nil {
		result := exec.RunWithTimeout("keepalived", 5*time.Second, "-t", "-f", status.ConfigPath)
		status.ConfigValid = result.Success()
		if !status.ConfigValid {
			status.Error = result.Combined()
		}
	}

	// Try to get VRRP state
	// 1. Check state file (updated by notify scripts)
	// Only trust if not UNKNOWN (as we initialize it to UNKNOWN)
	result := exec.RunWithTimeout("cat", 2*time.Second, "/tmp/keepalived.GATEWAY.state")
	if result.Success() {
		state := strings.TrimSpace(result.Stdout)
		if state != "" && state != "UNKNOWN" {
			status.VRRPState = state
		}
	}

	// 2. Fallback: Check if VIP is actually assigned to the interface
	// This provides a reliable source of truth even if notify scripts fail
	if status.VRRPState == "" || status.VRRPState == "UNKNOWN" {
		cfgFile := FindConfigPath()
		if data, err := os.ReadFile(cfgFile); err == nil {
			content := string(data)
			// Extract VIP and Interface from config
			vip := ""
			iface := ""
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "virtual_ipaddress {") {
					// Next line usually contains VIP
					continue
				}
				if strings.Contains(line, "/32 dev") {
					parts := strings.Fields(line)
					if len(parts) >= 3 {
						vip = strings.Split(parts[0], "/")[0]
						iface = parts[2]
					}
				}
			}

			if vip != "" && iface != "" {
				// Check if IP exists on interface
				// On OpenWrt/BusyBox, grep might behave differently, so we check for exact match or subnet
				checkCmd := fmt.Sprintf("ip addr show dev %s | grep -F '%s/'", iface, vip)
				res := exec.RunWithTimeout("sh", 2*time.Second, "-c", checkCmd)
				if res.Success() {
					status.VRRPState = "MASTER"
				} else if status.Running {
					// If running but no VIP, we are definitely BACKUP (or FAULT)
					// But only if we are sure service is running
					status.VRRPState = "BACKUP"
				}
			}
		}
	}

	return status
}

// IsRunning checks if keepalived is running.
func IsRunning() bool {
	// Try pgrep first
	result := exec.RunWithTimeout("pgrep", 5*time.Second, "-x", "keepalived")
	if result.Success() {
		return true
	}

	// Fallback to pidof (Standard on OpenWrt/BusyBox)
	result = exec.RunWithTimeout("pidof", 5*time.Second, "keepalived")
	return result.Success()
}

// Start starts the keepalived service.
func Start() error {
	return currentPlatform.Start()
}

// Stop stops the keepalived service.
func Stop() error {
	return currentPlatform.Stop()
}

// Enable enables the keepalived service at boot.
func Enable() error {
	return currentPlatform.Enable()
}
