// Package pkgmgr provides package manager operations.
package pkgmgr

import (
	"fmt"
	"strings"
	"time"

	"github.com/floatip/gateway/internal/platform/detect"
	"github.com/floatip/gateway/internal/platform/exec"
)

// Manager provides package management operations.
type Manager interface {
	Update() error
	Install(packages ...string) error
	Remove(packages ...string) error
	IsInstalled(pkg string) bool
}

// NewManager creates a package manager for the current platform.
func NewManager() Manager {
	info := detect.Detect()
	switch info.PackageManager {
	case detect.PackageManagerOpkg:
		return &OpkgManager{}
	case detect.PackageManagerApt:
		return &AptManager{}
	case detect.PackageManagerYum:
		return &YumManager{}
	default:
		return &NoopManager{}
	}
}

// OpkgManager manages packages on OpenWrt.
type OpkgManager struct{}

func (m *OpkgManager) Update() error {
	result := exec.RunWithTimeout("opkg", 120*time.Second, "update")
	if !result.Success() {
		return fmt.Errorf("opkg update failed: %s", result.Combined())
	}
	return nil
}

func (m *OpkgManager) Install(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install"}, packages...)
	result := exec.RunWithTimeout("opkg", 300*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("opkg install failed: %s", result.Combined())
	}
	return nil
}

func (m *OpkgManager) Remove(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"remove"}, packages...)
	result := exec.RunWithTimeout("opkg", 120*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("opkg remove failed: %s", result.Combined())
	}
	return nil
}

func (m *OpkgManager) IsInstalled(pkg string) bool {
	result := exec.RunWithTimeout("opkg", 10*time.Second, "list-installed", pkg)
	return result.Success() && strings.Contains(result.Stdout, pkg)
}

// AptManager manages packages on Debian/Ubuntu.
type AptManager struct{}

func (m *AptManager) Update() error {
	result := exec.RunWithTimeout("apt-get", 120*time.Second, "update", "-qq")
	if !result.Success() {
		return fmt.Errorf("apt-get update failed: %s", result.Combined())
	}
	return nil
}

func (m *AptManager) Install(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "-y", "-qq"}, packages...)
	result := exec.RunWithTimeout("apt-get", 600*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("apt-get install failed: %s", result.Combined())
	}
	return nil
}

func (m *AptManager) Remove(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"remove", "-y", "-qq"}, packages...)
	result := exec.RunWithTimeout("apt-get", 120*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("apt-get remove failed: %s", result.Combined())
	}
	return nil
}

func (m *AptManager) IsInstalled(pkg string) bool {
	result := exec.RunWithTimeout("dpkg", 10*time.Second, "-s", pkg)
	return result.Success() && strings.Contains(result.Stdout, "Status: install ok installed")
}

// YumManager manages packages on RHEL/CentOS.
type YumManager struct{}

func (m *YumManager) Update() error {
	result := exec.RunWithTimeout("yum", 120*time.Second, "makecache", "-q")
	if !result.Success() {
		return fmt.Errorf("yum makecache failed: %s", result.Combined())
	}
	return nil
}

func (m *YumManager) Install(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "-y", "-q"}, packages...)
	result := exec.RunWithTimeout("yum", 600*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("yum install failed: %s", result.Combined())
	}
	return nil
}

func (m *YumManager) Remove(packages ...string) error {
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"remove", "-y", "-q"}, packages...)
	result := exec.RunWithTimeout("yum", 120*time.Second, args...)
	if !result.Success() {
		return fmt.Errorf("yum remove failed: %s", result.Combined())
	}
	return nil
}

func (m *YumManager) IsInstalled(pkg string) bool {
	result := exec.RunWithTimeout("rpm", 10*time.Second, "-q", pkg)
	return result.Success()
}

// NoopManager is a no-operation package manager.
type NoopManager struct{}

func (m *NoopManager) Update() error {
	return fmt.Errorf("package management not supported on this platform")
}

func (m *NoopManager) Install(packages ...string) error {
	return fmt.Errorf("package management not supported on this platform")
}

func (m *NoopManager) Remove(packages ...string) error {
	return nil
}

func (m *NoopManager) IsInstalled(pkg string) bool {
	return false
}

// RequiredPackages returns the list of required packages for router operation.
func RequiredPackages(platform detect.Platform) []string {
	switch platform {
	case detect.PlatformOpenWrt:
		return []string{
			"keepalived",
			"ip-full",
			"arping",
			"curl",
		}
	case detect.PlatformLinux:
		return []string{
			"keepalived",
			"iproute2",
			"arping",
			"curl",
		}
	default:
		return nil
	}
}
