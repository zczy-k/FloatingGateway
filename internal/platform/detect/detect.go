// Package detect provides platform detection utilities.
package detect

import (
	"os"
	"runtime"
	"strings"
)

// Platform represents the detected platform type.
type Platform string

const (
	PlatformOpenWrt Platform = "openwrt"
	PlatformLinux   Platform = "linux"
	PlatformMacOS   Platform = "macos"
	PlatformWindows Platform = "windows"
	PlatformUnknown Platform = "unknown"
)

// ServiceManager represents the system service manager type.
type ServiceManager string

const (
	ServiceManagerProcd   ServiceManager = "procd"
	ServiceManagerSystemd ServiceManager = "systemd"
	ServiceManagerLaunchd ServiceManager = "launchd"
	ServiceManagerNone    ServiceManager = "none"
)

// PackageManager represents the package manager type.
type PackageManager string

const (
	PackageManagerOpkg     PackageManager = "opkg"
	PackageManagerApt      PackageManager = "apt"
	PackageManagerYum      PackageManager = "yum"
	PackageManagerBrew     PackageManager = "brew"
	PackageManagerNone     PackageManager = "none"
)

// Info holds detected platform information.
type Info struct {
	OS             string
	Arch           string
	Platform       Platform
	ServiceManager ServiceManager
	PackageManager PackageManager
	IsRouter       bool // true if running on OpenWrt or Linux (potential router)
}

// Detect returns information about the current platform.
func Detect() *Info {
	info := &Info{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		info.detectLinux()
	case "darwin":
		info.Platform = PlatformMacOS
		info.ServiceManager = ServiceManagerLaunchd
		info.PackageManager = PackageManagerBrew
		info.IsRouter = false
	case "windows":
		info.Platform = PlatformWindows
		info.ServiceManager = ServiceManagerNone
		info.PackageManager = PackageManagerNone
		info.IsRouter = false
	default:
		info.Platform = PlatformUnknown
		info.ServiceManager = ServiceManagerNone
		info.PackageManager = PackageManagerNone
		info.IsRouter = false
	}

	return info
}

func (info *Info) detectLinux() {
	info.IsRouter = true // Linux can be a router

	// Check for OpenWrt
	if isOpenWrt() {
		info.Platform = PlatformOpenWrt
		info.ServiceManager = ServiceManagerProcd
		info.PackageManager = PackageManagerOpkg
		return
	}

	// Regular Linux
	info.Platform = PlatformLinux
	info.ServiceManager = detectServiceManager()
	info.PackageManager = detectPackageManager()
}

func isOpenWrt() bool {
	// Check /etc/openwrt_release
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		return true
	}
	// Check /etc/os-release for OpenWrt
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		content := strings.ToLower(string(data))
		if strings.Contains(content, "openwrt") {
			return true
		}
	}
	return false
}

func detectServiceManager() ServiceManager {
	// Check for systemd
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return ServiceManagerSystemd
	}
	// Check for systemctl binary
	if _, err := os.Stat("/bin/systemctl"); err == nil {
		return ServiceManagerSystemd
	}
	if _, err := os.Stat("/usr/bin/systemctl"); err == nil {
		return ServiceManagerSystemd
	}
	return ServiceManagerNone
}

func detectPackageManager() PackageManager {
	// Check for apt
	if _, err := os.Stat("/usr/bin/apt-get"); err == nil {
		return PackageManagerApt
	}
	if _, err := os.Stat("/usr/bin/apt"); err == nil {
		return PackageManagerApt
	}
	// Check for yum
	if _, err := os.Stat("/usr/bin/yum"); err == nil {
		return PackageManagerYum
	}
	return PackageManagerNone
}

// IsOpenWrt returns true if running on OpenWrt.
func (info *Info) IsOpenWrt() bool {
	return info.Platform == PlatformOpenWrt
}

// IsLinux returns true if running on Linux (non-OpenWrt).
func (info *Info) IsLinux() bool {
	return info.Platform == PlatformLinux
}

// HasSystemd returns true if systemd is available.
func (info *Info) HasSystemd() bool {
	return info.ServiceManager == ServiceManagerSystemd
}

// HasProcd returns true if procd is available (OpenWrt).
func (info *Info) HasProcd() bool {
	return info.ServiceManager == ServiceManagerProcd
}

// CanBeRouter returns true if this platform can be a router node.
func (info *Info) CanBeRouter() bool {
	return info.IsRouter
}

// String returns a human-readable description.
func (info *Info) String() string {
	return string(info.Platform) + "/" + info.Arch + " (service: " + string(info.ServiceManager) + ", pkg: " + string(info.PackageManager) + ")"
}
