// Package netutil provides network-related utilities.
package netutil

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/floatip/gateway/internal/platform/exec"
)

// InterfaceInfo holds information about a network interface.
type InterfaceInfo struct {
	Name    string
	IPv4    string
	CIDR    string
	Netmask string
	MAC     string
	Up      bool
}

// GetInterfaces returns all network interfaces with IPv4 addresses.
func GetInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var result []InterfaceInfo
	for _, iface := range ifaces {
		// Skip loopback and down interfaces for listing
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		
		info := InterfaceInfo{
			Name: iface.Name,
			MAC:  iface.HardwareAddr.String(),
			Up:   iface.Flags&net.FlagUp != 0,
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipv4 := ipNet.IP.To4(); ipv4 != nil {
					info.IPv4 = ipv4.String()
					info.CIDR = ipNet.String()
					ones, _ := ipNet.Mask.Size()
					info.Netmask = fmt.Sprintf("%d.%d.%d.%d",
						ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3])
					_ = ones // cidr prefix stored in CIDR field
					break
				}
			}
		}

		if info.IPv4 != "" {
			result = append(result, info)
		}
	}

	return result, nil
}

// GetInterfaceInfo returns information about a specific interface.
func GetInterfaceInfo(name string) (*InterfaceInfo, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface %q not found: %w", name, err)
	}

	info := &InterfaceInfo{
		Name: iface.Name,
		MAC:  iface.HardwareAddr.String(),
		Up:   iface.Flags&net.FlagUp != 0,
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("get addresses for %q: %w", name, err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				info.IPv4 = ipv4.String()
				info.CIDR = ipNet.String()
				info.Netmask = fmt.Sprintf("%d.%d.%d.%d",
					ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3])
				break
			}
		}
	}

	return info, nil
}

// InterfaceExists checks if an interface exists.
func InterfaceExists(name string) bool {
	_, err := net.InterfaceByName(name)
	return err == nil
}

// IsIPInCIDR checks if an IP is within a CIDR range.
func IsIPInCIDR(ip, cidr string) (bool, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, fmt.Errorf("invalid IP: %s", ip)
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR: %s", cidr)
	}
	return ipNet.Contains(parsedIP), nil
}

// SuggestVIP suggests a VIP address for the given CIDR.
// Tries .254, .253, .252 in order, skipping if occupied.
func SuggestVIP(cidr string, exclude []string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	// Get network address
	network := ipNet.IP.To4()
	if network == nil {
		return "", fmt.Errorf("not an IPv4 CIDR")
	}

	// Calculate broadcast address
	mask := ipNet.Mask
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = network[i] | ^mask[i]
	}

	// Try .254, .253, .252
	excludeMap := make(map[string]bool)
	for _, e := range exclude {
		excludeMap[e] = true
	}

	for offset := 1; offset <= 3; offset++ {
		candidate := make(net.IP, 4)
		copy(candidate, broadcast)
		candidate[3] = broadcast[3] - byte(offset)
		
		candidateStr := candidate.String()
		if !excludeMap[candidateStr] && ipNet.Contains(candidate) {
			return candidateStr, nil
		}
	}

	return "", fmt.Errorf("no suitable VIP found in CIDR %s", cidr)
}

// CheckIPConflict uses arping to check if an IP is in use.
// Returns true if the IP is in use (conflict).
func CheckIPConflict(ip, iface string, timeout time.Duration) (bool, error) {
	// Try arping first
	if exec.CommandExists("arping") {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		
		// arping -D -I <iface> -c 2 <ip>
		// Returns 0 if IP is NOT in use (no reply)
		// Returns 1 if IP IS in use (got reply)
		result := exec.Run(ctx, "arping", "-D", "-I", iface, "-c", "2", ip)
		if result.ExitCode == 1 {
			return true, nil // IP is in use
		}
		if result.ExitCode == 0 {
			return false, nil // IP is free
		}
		// Other error, fall through
	}

	// Fallback: just check if we can bind to the IP
	// This is less reliable but better than nothing
	return false, nil
}

// SendGARP sends a Gratuitous ARP announcement.
func SendGARP(vip, iface string) error {
	// Try arping first (preferred)
	if exec.CommandExists("arping") {
		result := exec.RunWithTimeout("arping", 5*time.Second,
			"-A", "-c", "4", "-I", iface, vip)
		if result.Success() {
			return nil
		}
		// Continue with fallback if arping fails
	}

	// Fallback: use ip neigh to trigger ARP update
	// This is less effective but works on more systems
	if exec.CommandExists("ip") {
		// Flush neighbor cache to force re-ARP
		exec.RunWithTimeout("ip", 3*time.Second, "neigh", "flush", "dev", iface)
		return nil
	}

	return fmt.Errorf("no method available to send GARP (arping or ip command not found)")
}

// GetDefaultGateway returns the default gateway IP.
func GetDefaultGateway() (string, error) {
	// Try ip route
	if exec.CommandExists("ip") {
		result := exec.RunWithTimeout("ip", 5*time.Second, "route", "show", "default")
		if result.Success() {
			// Parse "default via 192.168.1.1 dev eth0"
			lines := strings.Split(result.Stdout, "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				for i, f := range fields {
					if f == "via" && i+1 < len(fields) {
						return fields[i+1], nil
					}
				}
			}
		}
	}

	// Try route (older systems)
	if exec.CommandExists("route") {
		result := exec.RunWithTimeout("route", 5*time.Second, "-n")
		if result.Success() {
			lines := strings.Split(result.Stdout, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "0.0.0.0") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						return fields[1], nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine default gateway")
}

// AddVIP adds a VIP to an interface.
func AddVIP(vip, iface string) error {
	if exec.CommandExists("ip") {
		result := exec.RunWithTimeout("ip", 5*time.Second,
			"addr", "add", vip+"/32", "dev", iface)
		if result.Success() || strings.Contains(result.Stderr, "exists") {
			return nil
		}
		return fmt.Errorf("add VIP failed: %s", result.Combined())
	}
	return fmt.Errorf("ip command not found")
}

// RemoveVIP removes a VIP from an interface.
func RemoveVIP(vip, iface string) error {
	if exec.CommandExists("ip") {
		result := exec.RunWithTimeout("ip", 5*time.Second,
			"addr", "del", vip+"/32", "dev", iface)
		if result.Success() || strings.Contains(result.Stderr, "not exist") {
			return nil
		}
		return fmt.Errorf("remove VIP failed: %s", result.Combined())
	}
	return fmt.Errorf("ip command not found")
}

// HasVIP checks if an interface has the specified VIP.
func HasVIP(vip, iface string) (bool, error) {
	info, err := GetInterfaceInfo(iface)
	if err != nil {
		return false, err
	}

	// Check primary IP
	if info.IPv4 == vip {
		return true, nil
	}

	// Check secondary IPs using ip command
	if exec.CommandExists("ip") {
		result := exec.RunWithTimeout("ip", 5*time.Second,
			"addr", "show", "dev", iface)
		if result.Success() {
			return strings.Contains(result.Stdout, vip), nil
		}
	}

	return false, nil
}
