// Package config handles configuration loading, validation, and defaults.
package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Role represents the router role in the floating gateway setup.
type Role string

const (
	RolePrimary   Role = "primary"
	RoleSecondary Role = "secondary"
)

// HealthMode represents the health check mode.
type HealthMode string

const (
	HealthModeBasic    HealthMode = "basic"
	HealthModeInternet HealthMode = "internet"
)

// Config is the main configuration structure.
type Config struct {
	Version   int             `yaml:"version"`
	Role      Role            `yaml:"role"`
	LAN       LANConfig       `yaml:"lan"`
	Routers   RoutersConfig   `yaml:"routers"`
	Keepalived KeepalivedConfig `yaml:"keepalived"`
	Failover  FailoverConfig  `yaml:"failover"`
	Health    HealthConfig    `yaml:"health"`
	OpenWrt   OpenWrtConfig   `yaml:"openwrt"`
}

// LANConfig holds LAN interface configuration.
type LANConfig struct {
	Iface string `yaml:"iface"`
	CIDR  string `yaml:"cidr"`  // Optional, inferred from iface if empty
	VIP   string `yaml:"vip"`
}

// RoutersConfig holds router peer information.
type RoutersConfig struct {
	SelfIP string `yaml:"self_ip"` // Optional, inferred from iface
	PeerIP string `yaml:"peer_ip"` // Required
}

// KeepalivedConfig holds VRRP configuration.
type KeepalivedConfig struct {
	VRID      int                      `yaml:"vrid"`
	AdvertInt int                      `yaml:"advert_int"`
	Priority  KeepalivedPriorityConfig `yaml:"priority"`
}

// KeepalivedPriorityConfig holds priority values for each role.
type KeepalivedPriorityConfig struct {
	Primary   int `yaml:"primary"`
	Secondary int `yaml:"secondary"`
}

// FailoverConfig holds failover behavior settings.
type FailoverConfig struct {
	Prefer         string `yaml:"prefer"`
	Preempt        bool   `yaml:"preempt"`
	PreemptDelaySec int   `yaml:"preempt_delay_sec"`
}

// HealthConfig holds health check configuration.
type HealthConfig struct {
	Mode         HealthMode     `yaml:"mode"`
	IntervalSec  int            `yaml:"interval_sec"`
	FailCount    int            `yaml:"fail_count"`
	RecoverCount int            `yaml:"recover_count"`
	HoldDownSec  int            `yaml:"hold_down_sec"`
	KOfN         string         `yaml:"k_of_n"` // e.g., "2/3"
	Basic        ChecksConfig   `yaml:"basic"`
	Internet     ChecksConfig   `yaml:"internet"`
}

// ChecksConfig holds a list of check configurations.
type ChecksConfig struct {
	Checks []CheckConfig `yaml:"checks"`
}

// CheckConfig represents a single health check.
type CheckConfig struct {
	Type     string `yaml:"type"`     // ping, dns, tcp, http
	Target   string `yaml:"target"`   // IP, hostname, URL depending on type
	Port     int    `yaml:"port"`     // For tcp type
	Resolver string `yaml:"resolver"` // For dns type
	Domain   string `yaml:"domain"`   // For dns type
	URL      string `yaml:"url"`      // For http type
	Timeout  int    `yaml:"timeout"`  // Timeout in seconds, default 5
}

// OpenWrtConfig holds OpenWrt-specific settings.
type OpenWrtConfig struct {
	DHCP OpenWrtDHCPConfig `yaml:"dhcp"`
}

// OpenWrtDHCPConfig holds DHCP auto-configuration settings.
type OpenWrtDHCPConfig struct {
	AutoSetGateway bool `yaml:"auto_set_gateway"`
}

// DefaultConfig returns a Config with default values filled in.
func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		Keepalived: KeepalivedConfig{
			VRID:      51,
			AdvertInt: 1,
			Priority: KeepalivedPriorityConfig{
				Primary:   100,
				Secondary: 150,
			},
		},
		Failover: FailoverConfig{
			Prefer:         "secondary",
			Preempt:        true,
			PreemptDelaySec: 30,
		},
		Health: HealthConfig{
			Mode:         HealthModeInternet,
			IntervalSec:  2,
			FailCount:    3,
			RecoverCount: 5,
			KOfN:         "2/3",
			Basic: ChecksConfig{
				Checks: []CheckConfig{
					{Type: "ping", Target: "223.5.5.5", Timeout: 3},
					{Type: "dns", Resolver: "223.5.5.5", Domain: "baidu.com", Timeout: 3},
				},
			},
			Internet: ChecksConfig{
				Checks: []CheckConfig{
					{Type: "ping", Target: "223.5.5.5", Timeout: 3},
					{Type: "dns", Resolver: "119.29.29.29", Domain: "baidu.com", Timeout: 3},
					{Type: "tcp", Target: "114.114.114.114", Port: 53, Timeout: 3},
				},
			},
		},
		OpenWrt: OpenWrtConfig{
			DHCP: OpenWrtDHCPConfig{
				AutoSetGateway: false,
			},
		},
	}
}

// Load reads configuration from file and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	return Parse(data)
}

// Parse parses YAML data into Config with defaults.
func Parse(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return cfg, nil
}

// Validate checks configuration for errors.
func (c *Config) Validate() error {
	// Validate role
	if c.Role != RolePrimary && c.Role != RoleSecondary {
		return fmt.Errorf("invalid role %q: must be 'primary' or 'secondary'", c.Role)
	}

	// Validate LAN interface
	if c.LAN.Iface == "" {
		return fmt.Errorf("lan.iface is required")
	}

	// Validate VIP
	if c.LAN.VIP == "" {
		return fmt.Errorf("lan.vip is required")
	}
	vip := net.ParseIP(c.LAN.VIP)
	if vip == nil || vip.To4() == nil {
		return fmt.Errorf("lan.vip %q is not a valid IPv4 address", c.LAN.VIP)
	}

	// Validate CIDR if provided
	if c.LAN.CIDR != "" {
		_, ipNet, err := net.ParseCIDR(c.LAN.CIDR)
		if err != nil {
			return fmt.Errorf("lan.cidr %q is not valid: %w", c.LAN.CIDR, err)
		}
		// Check VIP is within CIDR
		if !ipNet.Contains(vip) {
			return fmt.Errorf("lan.vip %q is not within lan.cidr %q", c.LAN.VIP, c.LAN.CIDR)
		}
	}

	// Validate peer_ip
	if c.Routers.PeerIP == "" {
		return fmt.Errorf("routers.peer_ip is required")
	}
	peerIP := net.ParseIP(c.Routers.PeerIP)
	if peerIP == nil || peerIP.To4() == nil {
		return fmt.Errorf("routers.peer_ip %q is not a valid IPv4 address", c.Routers.PeerIP)
	}

	// Check peer_ip != vip
	if c.Routers.PeerIP == c.LAN.VIP {
		return fmt.Errorf("routers.peer_ip cannot be the same as lan.vip")
	}

	// Validate self_ip if provided
	if c.Routers.SelfIP != "" {
		selfIP := net.ParseIP(c.Routers.SelfIP)
		if selfIP == nil || selfIP.To4() == nil {
			return fmt.Errorf("routers.self_ip %q is not a valid IPv4 address", c.Routers.SelfIP)
		}
		if c.Routers.SelfIP == c.LAN.VIP {
			return fmt.Errorf("routers.self_ip cannot be the same as lan.vip")
		}
		if c.Routers.SelfIP == c.Routers.PeerIP {
			return fmt.Errorf("routers.self_ip cannot be the same as routers.peer_ip")
		}
	}

	// Validate VRID
	if c.Keepalived.VRID < 1 || c.Keepalived.VRID > 255 {
		return fmt.Errorf("keepalived.vrid must be between 1 and 255, got %d", c.Keepalived.VRID)
	}

	// Validate health mode
	if c.Health.Mode != HealthModeBasic && c.Health.Mode != HealthModeInternet {
		return fmt.Errorf("health.mode must be 'basic' or 'internet', got %q", c.Health.Mode)
	}

	// Validate k_of_n if provided
	if c.Health.KOfN != "" {
		if err := validateKOfN(c.Health.KOfN); err != nil {
			return fmt.Errorf("health.k_of_n: %w", err)
		}
	}

	return nil
}

// validateKOfN checks the k/n format.
func validateKOfN(s string) error {
	re := regexp.MustCompile(`^(\d+)/(\d+)$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return fmt.Errorf("invalid format %q, expected 'k/n' (e.g., '2/3')", s)
	}
	k, _ := strconv.Atoi(matches[1])
	n, _ := strconv.Atoi(matches[2])
	if k < 1 || n < 1 {
		return fmt.Errorf("k and n must be positive integers")
	}
	if k > n {
		return fmt.Errorf("k (%d) cannot be greater than n (%d)", k, n)
	}
	return nil
}

// ParseKOfN parses k/n string into (k, n) values.
func ParseKOfN(s string) (k, n int, err error) {
	if s == "" {
		return 0, 0, nil // all-of-n mode
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid k_of_n format")
	}
	k, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	n, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return k, n, nil
}

// GetChecks returns the checks for the configured health mode.
func (c *Config) GetChecks() []CheckConfig {
	switch c.Health.Mode {
	case HealthModeBasic:
		return c.Health.Basic.Checks
	case HealthModeInternet:
		return c.Health.Internet.Checks
	default:
		return c.Health.Internet.Checks
	}
}

// GetPriority returns the priority for the current role.
func (c *Config) GetPriority() int {
	switch c.Role {
	case RolePrimary:
		return c.Keepalived.Priority.Primary
	case RoleSecondary:
		return c.Keepalived.Priority.Secondary
	default:
		return 100
	}
}

// ToYAML serializes the config to YAML.
func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// SaveTo writes the config to a file.
func (c *Config) SaveTo(path string) error {
	data, err := c.ToYAML()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
