package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/version"
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

	// DefaultAgentPath is the standard installation path for the agent on remote routers.
	DefaultAgentPath = "/gateway-agent/gateway-agent"
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
	Name         string       `yaml:"name" json:"name"`
	Host         string       `yaml:"host" json:"host"`
	Port         int          `yaml:"port" json:"port"`
	User         string       `yaml:"user" json:"user"`
	Password     string       `yaml:"password,omitempty" json:"password,omitempty"`
	KeyFile      string       `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	Passphrase   string       `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`
	Role         config.Role  `yaml:"role" json:"role"`
	Iface        string       `yaml:"iface,omitempty" json:"iface,omitempty"`             // Per-router interface override
	HealthMode   string       `yaml:"health_mode,omitempty" json:"health_mode,omitempty"` // Per-router health mode (basic/internet)
	Status       RouterStatus `yaml:"-" json:"status"`
	Platform     Platform     `yaml:"-" json:"platform"`
	LastSeen     time.Time    `yaml:"-" json:"last_seen,omitempty"`
	AgentVer     string       `yaml:"-" json:"agent_version,omitempty"`
	VRRPState    string       `yaml:"-" json:"vrrp_state,omitempty"`
	Healthy      *bool        `yaml:"-" json:"healthy,omitempty"`
	Error        string       `yaml:"-" json:"error,omitempty"`
	InstallLog   []string     `yaml:"-" json:"install_log"`
	InstallStep  int          `yaml:"-" json:"install_step"`
	InstallTotal int          `yaml:"-" json:"install_total"`
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
	Version      int       `yaml:"version" json:"version"`
	Listen       string    `yaml:"listen" json:"listen"`
	Password     string    `yaml:"password" json:"password"` // Basic Auth password
	Routers      []*Router `yaml:"routers" json:"routers"`
	AgentBin     string    `yaml:"agent_bin" json:"agent_bin"`         // Path to gateway-agent binary (manual override)
	DownloadBase string    `yaml:"download_base" json:"download_base"` // Release download base URL
	GHProxy      string    `yaml:"gh_proxy" json:"gh_proxy"`           // GitHub acceleration proxy for China
	LAN          struct {
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

const defaultDownloadBase = "https://github.com/zczy-k/FloatingGateway/releases/latest/download"

// GitHub 加速镜像列表，按优先级排列
var ghProxies = []string{
	"https://gh-proxy.com/",
	"https://ghproxy.net/",
	"https://mirror.ghproxy.com/",
	"https://ghfast.top/",
}

// Manager handles router management operations.
type Manager struct {
	config     *ControllerConfig
	configPath string
	mu         sync.RWMutex
	dlMu       sync.Mutex // Mutex for binary downloads
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

	// Decrypt sensitive fields
	if cfg.Password != "" {
		if dec, err := decrypt(cfg.Password); err == nil {
			cfg.Password = dec
		}
	}

	// Set defaults and decrypt router fields
	for _, r := range cfg.Routers {
		if r.Port == 0 {
			r.Port = 22
		}
		r.Status = StatusUnknown

		if r.Password != "" {
			if dec, err := decrypt(r.Password); err == nil {
				r.Password = dec
			}
		}
		if r.Passphrase != "" {
			if dec, err := decrypt(r.Passphrase); err == nil {
				r.Passphrase = dec
			}
		}
	}

	m.config = cfg
	return nil
}

// SaveConfig saves the current configuration.
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to encrypt without affecting the in-memory state
	cfgCopy := *m.config
	cfgCopy.Routers = make([]*Router, len(m.config.Routers))
	for i, r := range m.config.Routers {
		rCopy := *r
		if rCopy.Password != "" {
			if enc, err := encrypt(rCopy.Password); err == nil {
				rCopy.Password = enc
			}
		}
		if rCopy.Passphrase != "" {
			if enc, err := encrypt(rCopy.Passphrase); err == nil {
				rCopy.Passphrase = enc
			}
		}
		cfgCopy.Routers[i] = &rCopy
	}

	if cfgCopy.Password != "" {
		if enc, err := encrypt(cfgCopy.Password); err == nil {
			cfgCopy.Password = enc
		}
	}

	data, err := yaml.Marshal(&cfgCopy)
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

	r.LastSeen = time.Now()
	r.Error = ""

	// Do not overwrite installing/uninstalling status
	if r.Status != StatusInstalling && r.Status != StatusUninstalling {
		r.Status = StatusOnline
	}

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

	// Check agent version (try absolute path first, then PATH)
	if ver, err := client.RunStdout(fmt.Sprintf("%s version 2>/dev/null || gateway-agent version 2>/dev/null", DefaultAgentPath)); err == nil {
		r.AgentVer = strings.TrimSpace(ver)
	}

	// Get agent status if installed
	if r.AgentVer != "" {
		if output, err := client.RunStdout(fmt.Sprintf("%s status --json 2>/dev/null || gateway-agent status --json 2>/dev/null", DefaultAgentPath)); err == nil {
			var status struct {
				Keepalived struct {
					VRRPState string `json:"vrrp_state"`
				} `json:"keepalived"`
				Health struct {
					Healthy bool `json:"healthy"`
				} `json:"health"`
			}
			if json.Unmarshal([]byte(extractJSON(output)), &status) == nil {
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

// verifyInterface checks if the network interface exists on the remote system
func (m *Manager) verifyInterface(client *SSHClient, iface string) error {
	_, err := client.RunCombined(fmt.Sprintf("ip link show %s", iface))
	if err != nil {
		return fmt.Errorf("网卡接口 %s 不存在", iface)
	}
	return nil
}

// getInterfaceIP gets the IPv4 address of the specified interface
func (m *Manager) getInterfaceIP(client *SSHClient, iface string) (string, error) {
	// Get the first IPv4 address from the interface
	cmd := fmt.Sprintf("ip -4 addr show dev %s | grep inet | awk '{print $2}' | cut -d/ -f1 | head -n1", iface)
	output, err := client.RunCombined(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get IP: %w", err)
	}

	ip := strings.TrimSpace(output)
	if ip == "" {
		return "", fmt.Errorf("网卡 %s 没有配置 IPv4 地址", iface)
	}

	// Validate IP format
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("无效的 IP 地址: %s", ip)
	}

	return ip, nil
}

// DetectNetwork tries to find the interface and CIDR for a given IP on the remote host.
func (m *Manager) DetectNetwork(client *SSHClient, targetIP string) (iface, cidr string, err error) {
	// First, try to find interface containing the target IP (most accurate)
	out, _ := client.RunCombined(fmt.Sprintf("ip -4 addr show to %s 2>&1", targetIP))
	if out != "" {
		re := regexp.MustCompile(`\d+:\s+(\S+?):`)
		matches := re.FindStringSubmatch(out)
		if len(matches) > 1 {
			iface = matches[1]
		}
	}

	// If not found, try to get the interface with a private IP (likely LAN)
	if iface == "" {
		// Look for interfaces with private IP addresses (10.x, 172.16-31.x, 192.168.x)
		out, _ = client.RunCombined("ip -4 addr show 2>&1 | grep -E 'inet (10\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.|192\\.168\\.)' -B 2")
		if out != "" {
			// Find the interface name from the output
			re := regexp.MustCompile(`\d+:\s+(\S+?):`)
			matches := re.FindStringSubmatch(out)
			if len(matches) > 1 {
				iface = matches[1]
			}
		}
	}

	// If still not found, try common LAN interface names
	if iface == "" {
		commonLANIfaces := []string{"br-lan", "eth0", "ens18", "ens33", "enp0s3", "lan"}
		for _, testIface := range commonLANIfaces {
			out, err := client.RunCombined(fmt.Sprintf("ip -4 addr show dev %s 2>&1", testIface))
			if err == nil && out != "" && strings.Contains(out, "inet ") {
				iface = testIface
				break
			}
		}
	}

	// Last resort: get default route interface (but skip if it's a WAN interface)
	if iface == "" {
		out, err := client.RunCombined("ip route get 8.8.8.8 2>&1")
		if err == nil && out != "" {
			fields := strings.Fields(out)
			for i, f := range fields {
				if f == "dev" && i+1 < len(fields) {
					testIface := fields[i+1]
					// Skip common WAN interface names
					if !strings.Contains(testIface, "pppoe") &&
						!strings.Contains(testIface, "wan") &&
						testIface != "eth1" {
						iface = testIface
					}
					break
				}
			}
		}
	}

	if iface == "" {
		return "", "", fmt.Errorf("无法找到 LAN 网络接口，请手动输入")
	}

	// Get CIDR for this interface
	out, err = client.RunCombined(fmt.Sprintf("ip -4 addr show dev %s 2>&1", iface))
	if err != nil || out == "" {
		return "", "", fmt.Errorf("无法获取接口 %s 的地址信息: %v", iface, err)
	}

	re := regexp.MustCompile(`inet\s+([0-9./]+)`)
	matches := re.FindStringSubmatch(out)
	if len(matches) > 1 {
		cidrFull := matches[1]
		// Use proper CIDR calculation
		cidr = cidrToNetwork(cidrFull)
	}

	if cidr == "" {
		return "", "", fmt.Errorf("接口 %s 没有配置 IPv4 地址", iface)
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

// AddLog adds a log entry to the router's installation log.
func (r *Router) AddLog(msg string) {
	r.InstallLog = append(r.InstallLog, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	// Keep only last 30 entries
	if len(r.InstallLog) > 30 {
		r.InstallLog = r.InstallLog[len(r.InstallLog)-30:]
	}
}

// StepLog advances the step counter and adds a log entry.
func (r *Router) StepLog(msg string) {
	r.InstallStep++
	r.AddLog(fmt.Sprintf("(%d/%d) %s", r.InstallStep, r.InstallTotal, msg))
}

// Install installs the agent on a router.
func (m *Manager) Install(r *Router, agentConfig *config.Config) error {
	r.InstallLog = nil
	r.InstallStep = 0
	r.InstallTotal = 13
	r.Error = ""
	r.Status = StatusInstalling

	// Cleanup function for failure
	var cleanup []func()
	defer func() {
		if r.Status == StatusError {
			r.AddLog("!! 检测到安装失败，正在执行部分清理...")
			for i := len(cleanup) - 1; i >= 0; i-- {
				cleanup[i]()
			}
		}
	}()

	r.StepLog("正在连接到 " + r.Host + ":" + fmt.Sprintf("%d", r.Port) + "...")
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		r.AddLog("!! 连接失败: " + err.Error())
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	r.AddLog("   连接成功")

	// Detect platform
	r.StepLog("探测目标系统平台...")
	platform := m.detectPlatform(client)
	r.Platform = platform
	r.AddLog("   平台: " + string(platform))

	// Verify interface exists
	r.StepLog("验证网卡接口...")
	if err := m.verifyInterface(client, agentConfig.LAN.Iface); err != nil {
		r.AddLog("!! 网卡接口验证失败: " + err.Error())
		return fmt.Errorf("interface verification: %w", err)
	}
	r.AddLog("   网卡接口 " + agentConfig.LAN.Iface + " 存在")

	// Get SelfIP from interface
	r.StepLog("获取网卡 IP 地址...")
	selfIP, err := m.getInterfaceIP(client, agentConfig.LAN.Iface)
	if err != nil {
		r.AddLog("!! 获取 IP 失败: " + err.Error())
		return fmt.Errorf("get interface IP: %w", err)
	}
	agentConfig.Routers.SelfIP = selfIP
	r.AddLog("   网卡 IP: " + selfIP)

	// Determine target architecture
	r.StepLog("探测系统架构...")
	arch, err := client.RunCombined("uname -m")
	if err != nil {
		r.AddLog("!! 架构探测失败: " + err.Error())
		return fmt.Errorf("detect arch: %w", err)
	}
	arch = strings.TrimSpace(arch)

	// ... (endianness detection logic stays same)
	// (Go arch normalization logic stays same)

	goarch := normalizeArch(arch)
	goos := "linux"
	r.AddLog(fmt.Sprintf("   架构: %s (目标: %s/%s)", arch, goos, goarch))

	// Try to find agent binary locally first
	r.StepLog("查找适配的 Agent 二进制文件...")
	binPath, err := m.findAgentBinary(goos, goarch)
	if err != nil {
		// Not found locally, download from remote
		r.AddLog("   本地未找到，从远程仓库下载...")
		r.StepLog("从远程仓库下载 Agent 二进制文件...")
		binPath, err = m.downloadAgentBinary(r, goos, goarch)
		if err != nil {
			r.AddLog("!! 下载失败: " + err.Error())
			return fmt.Errorf("download binary: %w", err)
		}
	} else {
		r.AddLog("   使用本地文件: " + binPath)
	}

	// Read and upload binary
	r.StepLog("停止旧版本 Agent 并上传新版本...")
	// Stop existing services to avoid "text file busy" errors during upload
	switch platform {
	case PlatformOpenWrt:
		client.RunCombined("/etc/init.d/gateway-agent stop 2>/dev/null")
	case PlatformLinux:
		client.RunCombined("systemctl stop gateway-agent 2>/dev/null")
	}
	// Kill any stray agent processes
	client.RunCombined("pkill -9 gateway-agent || (ps -w | grep gateway-agent | grep -v grep | awk '{print $1}' | xargs kill -9) 2>/dev/null")
	time.Sleep(500 * time.Millisecond)

	binData, err := os.ReadFile(binPath)
	if err != nil {
		r.AddLog("!! 读取二进制文件失败: " + err.Error())
		return fmt.Errorf("read binary: %w", err)
	}

	// Create agent directory
	r.StepLog("上传 Agent 二进制文件...")
	agentDir := filepath.Dir(DefaultAgentPath)
	if err := client.MkdirAll(agentDir); err != nil {
		r.AddLog("!! 创建 Agent 目录失败: " + err.Error())
		return fmt.Errorf("create agent dir: %w", err)
	}
	cleanup = append(cleanup, func() { client.RunCombined(fmt.Sprintf("rm -rf %s", agentDir)) })

	// CRITICAL: Ensure root ownership on our directory to satisfy Keepalived security audits.
	// Since / and /gateway-agent will be root-owned, this path is secure even if /etc or /usr are not.
	client.RunCombined(fmt.Sprintf("chown root:root %s && chmod 0755 %s", agentDir, agentDir))

	// Create config directory
	if err := client.MkdirAll("/etc/gateway-agent"); err != nil {
		r.AddLog("!! 创建配置目录失败: " + err.Error())
		return fmt.Errorf("create config dir: %w", err)
	}
	cleanup = append(cleanup, func() { client.RunCombined("rm -rf /etc/gateway-agent") })

	// Fix ownership for config directory as well
	client.RunCombined("chown root:root /etc/gateway-agent && chmod 0755 /etc/gateway-agent")

	if err := client.WriteFile(DefaultAgentPath, binData, 0755); err != nil {
		r.AddLog("!! 上传失败: " + err.Error())
		// Try to diagnose why it failed
		if diagOut, _ := client.RunCombined(fmt.Sprintf("ls -ld %s && mount | grep ' / '", agentDir)); diagOut != "" {
			r.AddLog("   调试信息: " + diagOut)
		}
		return fmt.Errorf("upload binary: %w", err)
	}
	cleanup = append(cleanup, func() { client.RemoveFile(DefaultAgentPath) })

	// Ensure root ownership and strict permissions to satisfy Keepalived security checks
	client.RunCombined(fmt.Sprintf("chown root:root %s && chmod 0755 %s", DefaultAgentPath, DefaultAgentPath))

	// Verify upload size immediately
	expectedSize := len(binData)
	if sizeOut, err := client.RunCombined(fmt.Sprintf("stat -c %%s %s 2>/dev/null || stat -f %%z %s 2>/dev/null", DefaultAgentPath, DefaultAgentPath)); err != nil {
		r.AddLog("!! 无法验证上传: " + err.Error())
		return fmt.Errorf("verify upload size: %w", err)
	} else {
		actualSize := strings.TrimSpace(sizeOut)
		r.AddLog(fmt.Sprintf("   上传成功 (预期: %d 字节, 实际: %s 字节)", expectedSize, actualSize))
		if actualSize != fmt.Sprintf("%d", expectedSize) {
			r.AddLog("!! 文件大小不匹配，上传可能不完整")
			return fmt.Errorf("upload size mismatch: expected %d, got %s", expectedSize, actualSize)
		}
	}

	// Generate and upload config
	r.StepLog("生成并上传配置文件...")
	agentConfig.Role = r.Role
	configData, err := agentConfig.ToYAML()
	if err != nil {
		r.AddLog("!! 生成配置失败: " + err.Error())
		return fmt.Errorf("generate config: %w", err)
	}
	if err := client.WriteFile("/etc/gateway-agent/config.yaml", configData, 0600); err != nil {
		r.AddLog("!! 上传配置失败: " + err.Error())
		return fmt.Errorf("upload config: %w", err)
	}
	client.RunCombined("chown root:root /etc/gateway-agent/config.yaml")
	r.AddLog("   配置已写入")

	// Install keepalived
	r.StepLog("安装 Keepalived 依赖...")
	if err := m.installKeepalived(client, platform); err != nil {
		r.AddLog("!! 安装 Keepalived 失败: " + err.Error())
		return fmt.Errorf("install keepalived: %w", err)
	}
	r.AddLog("   Keepalived 就绪")

	// Install arping for GARP announcements
	r.StepLog("安装 ARP 工具...")
	if err := m.installArping(client, platform); err != nil {
		r.AddLog("   警告: ARP 工具安装失败 (不影响核心功能)")
	} else {
		r.AddLog("   ARP 工具已安装")
	}

	// Verify binary was uploaded correctly and is executable
	r.StepLog("验证 Agent 二进制文件...")
	// Check version and ensure it matches what we expect
	verifyScript := fmt.Sprintf(`ls -la %s 2>&1
file %s 2>/dev/null || echo 'file command not available'
%s version 2>&1 || echo 'version check failed'`, DefaultAgentPath, DefaultAgentPath, DefaultAgentPath)
	if output, err := client.RunCombined(verifyScript); err != nil {
		r.AddLog("!! 验证失败: " + err.Error())
		r.AddLog("   输出: " + output)
		return fmt.Errorf("verify binary: %w", err)
	} else {
		r.AddLog("   " + strings.TrimSpace(output))
		// Version check
		if !strings.Contains(output, version.Version) && version.Version != "dev" {
			r.AddLog(fmt.Sprintf("   警告: Agent 版本 (%s) 与控制端版本 (%s) 不匹配", output, version.Version))
		}
	}

	// Apply agent config (use absolute path and explicit config path)
	r.StepLog("初始化 Agent 配置...")
	// Set PATH explicitly to ensure the command can find dependencies
	applyCmd := fmt.Sprintf("PATH=/usr/bin:/usr/local/bin:/bin:/sbin:$PATH %s apply -c /etc/gateway-agent/config.yaml", DefaultAgentPath)
	if output, err := client.RunCombined(applyCmd); err != nil {
		r.AddLog("!! 初始化失败: " + err.Error())
		if output != "" {
			r.AddLog("   输出: " + output)
		}
		// Try to get more debug info
		if debugOut, _ := client.RunCombined(fmt.Sprintf("echo PATH=$PATH && ls -la %s && cat /etc/gateway-agent/config.yaml | head -20", DefaultAgentPath)); debugOut != "" {
			r.AddLog("   调试信息: " + debugOut)
		}
		return fmt.Errorf("apply config: %w", err)
	}

	// Setup service
	r.StepLog("配置并启动系统服务...")
	if err := m.setupService(client, platform); err != nil {
		r.AddLog("!! 服务配置失败: " + err.Error())
		return fmt.Errorf("setup service: %w", err)
	}
	cleanup = append(cleanup, func() {
		switch platform {
		case PlatformOpenWrt:
			client.RunCombined("/etc/init.d/gateway-agent stop; /etc/init.d/gateway-agent disable; rm /etc/init.d/gateway-agent")
		case PlatformLinux:
			client.RunCombined("systemctl stop gateway-agent; systemctl disable gateway-agent; rm /etc/systemd/system/gateway-agent.service; systemctl daemon-reload")
		}
	})

	// Wait a moment for services to start
	time.Sleep(2 * time.Second)

	// Ensure root ownership and strict permissions for keepalived config
	client.RunCombined("chown root:root /etc/keepalived/keepalived.conf && chmod 0644 /etc/keepalived/keepalived.conf")

	// Verify keepalived is running
	// Use both pgrep and pidof for better compatibility across different systems/OpenWrt
	checkRunningCmd := "pgrep -x keepalived || pidof keepalived"
	if output, err := client.RunCombined(checkRunningCmd); err != nil || output == "" {
		r.AddLog("   警告: keepalived 未能启动，尝试重新启动...")
		// Try to restart keepalived
		switch platform {
		case PlatformOpenWrt:
			client.RunCombined("/etc/init.d/keepalived restart")
		case PlatformLinux:
			client.RunCombined("systemctl restart keepalived")
		}
		time.Sleep(3 * time.Second) // Give it more time to start

		// Check again
		if output, err := client.RunCombined(checkRunningCmd); err != nil || output == "" {
			r.AddLog("!! 错误: Keepalived 服务未能启动")
			// Try to get more error info
			var logCmd string
			switch platform {
			case PlatformOpenWrt:
				logCmd = "logread | grep keepalived | tail -n 20"
			case PlatformLinux:
				logCmd = "journalctl -u keepalived -n 20 --no-pager"
			}
			if logOut, _ := client.RunCombined(logCmd); logOut != "" {
				r.AddLog("   系统日志:\n" + logOut)
			}
			// One last attempt: check if config file actually exists and has content
			if confCheck, _ := client.RunCombined("ls -l /etc/keepalived/keepalived.conf && cat /etc/keepalived/keepalived.conf | head -n 5"); confCheck != "" {
				r.AddLog("   当前配置文件状态:\n" + confCheck)
			}
			return fmt.Errorf("keepalived failed to start")
		} else {
			r.AddLog("   keepalived 已成功启动")
		}
	} else {
		r.AddLog("   服务已启动")
	}

	// Setup firewall
	r.StepLog("配置防火墙规则...")
	if err := m.setupFirewall(client, platform); err != nil {
		r.AddLog("   警告: 防火墙配置失败 (不影响核心功能)")
	} else {
		r.AddLog("   防火墙规则已配置")
	}

	r.InstallStep = r.InstallTotal
	r.AddLog("安装全部完成!")
	r.AddLog("")
	r.AddLog("=== 重要提示 ===")
	r.AddLog("请在主路由的 DHCP 设置中将默认网关改为 VIP: " + agentConfig.LAN.VIP)
	if platform == PlatformOpenWrt {
		r.AddLog("OpenWrt 路径: 网络 -> 接口 -> LAN -> 修改 -> DHCP 服务器")
		r.AddLog("在 DHCP 选项中添加: 3," + agentConfig.LAN.VIP)
	}
	r.AddLog("===============")
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

	// Run doctor with absolute path and get JSON output
	// Note: doctor may return non-zero exit code if there are errors, but output is still valid JSON
	docCmd := fmt.Sprintf("%s doctor --json 2>/dev/null || gateway-agent doctor --json 2>/dev/null", DefaultAgentPath)
	output, err := client.RunStdout(docCmd)

	// If there's output (even with error), try to return it as it's likely valid JSON
	if output != "" {
		return extractJSON(output), nil
	}

	// Only return error if there's no output at all
	if err != nil {
		return "", fmt.Errorf("run doctor: %w", err)
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

// normalizeArch converts uname -m output to Go's GOARCH naming.
func normalizeArch(arch string) string {
	switch arch {
	case "x86_64", "amd64":
		return "amd64"
	case "i386", "i686", "i586":
		return "386"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv8l", "armv6l", "armv5l", "arm":
		return "arm"
	case "mips":
		return "mips"
	case "mipsel", "mipsle":
		return "mipsle"
	case "mips64":
		return "mips64"
	case "mips64el", "mips64le":
		return "mips64le"
	case "riscv64":
		return "riscv64"
	case "loongarch64":
		return "loong64"
	case "ppc64le":
		return "ppc64le"
	case "ppc64":
		return "ppc64"
	case "s390x":
		return "s390x"
	default:
		return arch
	}
}

// agentCacheDir returns the directory for cached agent binaries.
func (m *Manager) agentCacheDir() string {
	return filepath.Join(filepath.Dir(m.configPath), "agents")
}

// findAgentBinary finds the appropriate agent binary from local cache or configured path.
// It does NOT download; downloading is a separate step in the install flow.
func (m *Manager) findAgentBinary(goos, goarch string) (string, error) {
	// 1. Check configured path first
	if m.config.AgentBin != "" {
		if _, err := os.Stat(m.config.AgentBin); err == nil {
			return m.config.AgentBin, nil
		}
	}

	// 2. Look for platform-specific binary
	patterns := []string{
		fmt.Sprintf("gateway-agent-%s-%s-%s", goos, goarch, version.Version),
		fmt.Sprintf("gateway-agent-%s-%s", goos, goarch),
		fmt.Sprintf("gateway-agent_%s_%s", goos, goarch),
		"gateway-agent",
	}

	// Search dirs: cache dir, working dir, exe dir, system paths
	searchDirs := []string{
		m.agentCacheDir(),
		".",
		"./bin",
		"./dist",
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs,
			exeDir,
			filepath.Join(exeDir, "bin"),
			filepath.Join(exeDir, "dist"),
		)
	}

	searchDirs = append(searchDirs, "/usr/local/bin", "/usr/bin")

	for _, dir := range searchDirs {
		for _, pattern := range patterns {
			path := filepath.Join(dir, pattern)
			if info, err := os.Stat(path); err == nil && info.Size() > 0 {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("未找到 %s/%s 的 Agent 二进制文件", goos, goarch)
}

// downloadAgentBinary downloads the agent binary for the given platform/arch from GitHub Releases.
// It tries acceleration proxies first (for China users), then falls back to direct download.
// Returns the path to the downloaded binary.
func (m *Manager) downloadAgentBinary(r *Router, goos, goarch string) (string, error) {
	// Remote asset name (as uploaded to GitHub Release)
	remoteAssetName := fmt.Sprintf("gateway-agent-%s-%s", goos, goarch)
	// Local cache name (includes version to ensure matching versions)
	localCacheName := fmt.Sprintf("%s-%s", remoteAssetName, version.Version)

	cacheDir := m.agentCacheDir()
	destPath := filepath.Join(cacheDir, localCacheName)

	// Check if already cached (without lock first for speed)
	if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
		r.AddLog("   使用缓存: " + localCacheName)
		return destPath, nil
	}

	// Acquire download lock to avoid redundant downloads and file conflicts
	m.dlMu.Lock()
	defer m.dlMu.Unlock()

	// Re-check cache after acquiring lock (double-checked locking)
	if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
		return destPath, nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// Build download URL
	// Try versioned release first, then fall back to latest
	var downloadBases []string
	if m.config.DownloadBase != "" {
		downloadBases = append(downloadBases, m.config.DownloadBase)
	} else {
		// Priority 1: Specific version
		if version.Version != "" && version.Version != "dev" {
			v := version.Version
			if !strings.HasPrefix(v, "v") {
				v = "v" + v
			}
			downloadBases = append(downloadBases, fmt.Sprintf("https://github.com/zczy-k/FloatingGateway/releases/download/%s", v))
		}
		// Priority 2: Latest release
		downloadBases = append(downloadBases, defaultDownloadBase)
	}

	// Build candidate URLs: user-configured proxy, then built-in proxies, then direct
	var urls []string

	for _, base := range downloadBases {
		directURL := base + "/" + remoteAssetName

		if m.config.GHProxy != "" {
			proxy := m.config.GHProxy
			if !strings.HasSuffix(proxy, "/") {
				proxy += "/"
			}
			urls = append(urls, proxy+directURL)
		}

		for _, proxy := range ghProxies {
			if !strings.HasSuffix(proxy, "/") {
				proxy += "/"
			}
			urls = append(urls, proxy+directURL)
		}

		urls = append(urls, directURL)
	}

	client := &http.Client{
		Timeout: 180 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	for i, url := range urls {
		if i > 0 {
			r.AddLog(fmt.Sprintf("   尝试备用下载源 (%d/%d)...", i+1, len(urls)))
		}

		r.AddLog("   下载: " + url)
		r.AddLog("   提示: 如果下载缓慢，请耐心等待或在全局设置中配置加速镜像")

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			r.AddLog("   !! 请求构建失败: " + err.Error())
			continue
		}
		req.Header.Set("User-Agent", "FloatingGateway-Controller")

		resp, err := client.Do(req)
		if err != nil {
			r.AddLog("   !! 连接失败: " + err.Error())
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			r.AddLog(fmt.Sprintf("   !! HTTP %d", resp.StatusCode))
			continue
		}

		// Ensure cache directory still exists
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("创建缓存目录失败: %w", err)
		}

		// Create a truly unique temp file to avoid concurrent install conflicts
		f, err := os.CreateTemp(cacheDir, localCacheName+".*.tmp")
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("创建临时文件失败: %w", err)
		}
		tmpPath := f.Name()

		written, err := io.Copy(f, resp.Body)
		f.Close()
		resp.Body.Close()

		if err != nil {
			os.Remove(tmpPath)
			r.AddLog("   !! 下载中断: " + err.Error())
			continue
		}

		if written == 0 {
			os.Remove(tmpPath)
			r.AddLog("   !! 下载的文件为空")
			continue
		}

		// Rename temp to final (use copy if rename fails due to cross-device)
		if err := os.Rename(tmpPath, destPath); err != nil {
			// If rename fails (e.g. cross-device or permission), try copy + delete
			if err := copyFile(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return "", fmt.Errorf("移动文件失败: %w", err)
			}
			os.Remove(tmpPath)
		}

		if err := os.Chmod(destPath, 0755); err != nil {
			return "", fmt.Errorf("设置权限失败: %w", err)
		}

		r.AddLog(fmt.Sprintf("   下载完成 (%.1f MB)", float64(written)/1024/1024))
		return destPath, nil
	}

	return "", fmt.Errorf("所有下载源均失败，请检查网络连接或手动将 %s 放到 %s 目录", localCacheName, cacheDir)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
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

// installArping installs arping tool for GARP announcements
func (m *Manager) installArping(client *SSHClient, platform Platform) error {
	// Check if already installed
	if _, err := client.RunCombined("which arping"); err == nil {
		return nil
	}

	switch platform {
	case PlatformOpenWrt:
		// Try multiple arping providers
		providers := []string{"iputils-arping", "arping", "busybox"}
		for _, p := range providers {
			if _, err := client.RunCombined("opkg install " + p); err == nil {
				// Verify it works
				if _, err := client.RunCombined("which arping"); err == nil {
					return nil
				}
			}
		}
		// If opkg failed, check if busybox already has it
		if out, err := client.RunCombined("arping 2>&1"); err == nil || strings.Contains(out, "Usage: arping") {
			return nil
		}
		return fmt.Errorf("failed to install any arping provider")
	case PlatformLinux:
		// Try iputils-arping (Debian/Ubuntu) or iputils (RHEL/CentOS)
		if _, err := client.RunCombined("apt-get install -y iputils-arping"); err == nil {
			return nil
		}
		if _, err := client.RunCombined("yum install -y iputils"); err == nil {
			return nil
		}
		if _, err := client.RunCombined("yum install -y arping"); err == nil {
			return nil
		}
		return fmt.Errorf("failed to install arping")
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
	initScript := fmt.Sprintf(`#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command %s run
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
`, DefaultAgentPath)
	if err := client.WriteFile("/etc/init.d/gateway-agent", []byte(initScript), 0755); err != nil {
		return err
	}

	// Enable services
	client.RunCombined("/etc/init.d/gateway-agent enable")
	client.RunCombined("/etc/init.d/keepalived enable")

	// Start gateway-agent
	client.RunCombined("/etc/init.d/gateway-agent restart")

	// Stop keepalived first (in case it's already running with old config or conflicting config)
	client.RunCombined("/etc/init.d/keepalived stop")
	// Kill any stray keepalived processes (especially on iStoreOS/OpenWrt)
	// Use multiple ways to ensure it's killed, even if killall is missing
	client.RunCombined("pkill -9 keepalived || killall -9 keepalived || (ps -w | grep keepalived | grep -v grep | awk '{print $1}' | xargs kill -9) 2>/dev/null")
	time.Sleep(1 * time.Second)

	// Force keepalived to use our config file via UCI if possible
	client.RunCombined("uci set keepalived.globals=keepalived 2>/dev/null")
	client.RunCombined("uci set keepalived.globals.config_file='/etc/keepalived/keepalived.conf' 2>/dev/null")
	client.RunCombined("uci commit keepalived 2>/dev/null")

	// Start keepalived
	if output, err := client.RunCombined("/etc/init.d/keepalived start"); err != nil {
		return fmt.Errorf("start keepalived: %w (output: %s)", err, output)
	}

	return nil
}

// setupSystemdService sets up a systemd service on Linux.
func (m *Manager) setupSystemdService(client *SSHClient) error {
	unitFile := fmt.Sprintf(`[Unit]
Description=Gateway Agent
After=network.target keepalived.service
Wants=keepalived.service

[Service]
Type=simple
ExecStart=%s run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, DefaultAgentPath)
	if err := client.WriteFile("/etc/systemd/system/gateway-agent.service", []byte(unitFile), 0644); err != nil {
		return err
	}

	// Reload systemd
	if _, err := client.RunCombined("systemctl daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	// Enable services
	client.RunCombined("systemctl enable keepalived")
	client.RunCombined("systemctl enable gateway-agent")

	// Start gateway-agent
	client.RunCombined("systemctl restart gateway-agent")

	// Stop keepalived first (in case it's already running with old config or conflicting config)
	client.RunCombined("systemctl stop keepalived")
	// Use multiple ways to ensure it's killed, even if killall is missing
	client.RunCombined("pkill -9 keepalived || killall -9 keepalived || (ps -w | grep keepalived | grep -v grep | awk '{print $1}' | xargs kill -9) 2>/dev/null")
	time.Sleep(1 * time.Second)

	// Start keepalived
	if output, err := client.RunCombined("systemctl start keepalived"); err != nil {
		return fmt.Errorf("start keepalived: %w (output: %s)", err, output)
	}

	return nil
}

// Uninstall removes the agent from a router.
func (m *Manager) Uninstall(r *Router) error {
	r.InstallLog = nil
	r.InstallStep = 0
	r.InstallTotal = 9
	r.Error = ""
	r.Status = StatusUninstalling

	r.StepLog(fmt.Sprintf("正在连接到 %s:%d...", r.Host, r.Port))
	client := NewSSHClient(m.sshConfig(r))
	if err := client.Connect(); err != nil {
		r.AddLog("!! 连接失败: " + err.Error())
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	r.AddLog("   连接成功")

	r.StepLog("探测目标系统平台...")
	platform := m.detectPlatform(client)
	r.AddLog("   平台: " + string(platform))

	// Stop and disable gateway-agent service
	r.StepLog("停止并禁用 Agent 服务...")
	switch platform {
	case PlatformOpenWrt:
		client.RunCombined("/etc/init.d/gateway-agent stop 2>/dev/null")
		client.RunCombined("/etc/init.d/gateway-agent disable 2>/dev/null")
		client.RemoveFile("/etc/init.d/gateway-agent")
	case PlatformLinux:
		client.RunCombined("systemctl stop gateway-agent 2>/dev/null")
		client.RunCombined("systemctl disable gateway-agent 2>/dev/null")
		client.RemoveFile("/etc/systemd/system/gateway-agent.service")
		client.RunCombined("systemctl daemon-reload")
	}
	r.AddLog("   Agent 服务已停止并移除")

	// Cleanup Firewall Rules
	r.StepLog("清理防火墙规则...")
	switch platform {
	case PlatformOpenWrt:
		client.RunCombined("uci delete firewall.vrrp 2>/dev/null")
		client.RunCombined("uci commit firewall")
		client.RunCombined("/etc/init.d/firewall restart 2>/dev/null")
	case PlatformLinux:
		if _, err := client.RunCombined("which ufw"); err == nil {
			client.RunCombined("ufw delete allow proto vrrp 2>/dev/null")
		}
		if _, err := client.RunCombined("which firewall-cmd"); err == nil {
			client.RunCombined("firewall-cmd --permanent --remove-rich-rule='rule protocol value=\"vrrp\" accept' 2>/dev/null")
			client.RunCombined("firewall-cmd --reload 2>/dev/null")
		}
		client.RunCombined("iptables -D INPUT -p vrrp -j ACCEPT 2>/dev/null")
	}
	r.AddLog("   防火墙规则已清理")

	// Stop and disable keepalived service
	r.StepLog("停止并禁用 Keepalived 服务...")
	switch platform {
	case PlatformOpenWrt:
		client.RunCombined("/etc/init.d/keepalived stop 2>/dev/null")
		client.RunCombined("/etc/init.d/keepalived disable 2>/dev/null")
	case PlatformLinux:
		client.RunCombined("systemctl stop keepalived 2>/dev/null")
		client.RunCombined("systemctl disable keepalived 2>/dev/null")
	}
	r.AddLog("   Keepalived 服务已停止并禁用")

	// Remove VIP from interface if still present
	r.StepLog("清理 VIP 地址...")
	if m.config.LAN.VIP != "" && r.Iface != "" {
		vipCmd := fmt.Sprintf("ip addr del %s/32 dev %s 2>/dev/null || true", m.config.LAN.VIP, r.Iface)
		client.RunCombined(vipCmd)
		r.AddLog("   VIP 已清理")
	} else {
		r.AddLog("   跳过 VIP 清理（配置不完整）")
	}

	// Remove keepalived configuration
	r.StepLog("清理 Keepalived 配置...")
	client.RemoveFile("/etc/keepalived/keepalived.conf")
	client.RemoveFile("/etc/keepalived.conf")
	r.AddLog("   Keepalived 配置已删除")

	// Remove agent files and configuration
	r.StepLog("清理 Agent 文件和配置...")
	client.RemoveFile(DefaultAgentPath)
	client.RemoveFile("/usr/bin/gateway-agent")
	client.RemoveFile("/usr/local/bin/gateway-agent")
	client.RunCombined("rm -rf /etc/gateway-agent")
	client.RunCombined("rm -rf /var/log/gateway-agent*")
	client.RunCombined("rm -rf /tmp/gateway-agent*")
	r.AddLog("   Agent 文件已清理")

	// Clean up any remaining state files
	r.StepLog("清理状态文件...")
	client.RunCombined("rm -f /tmp/keepalived.*.state 2>/dev/null")
	client.RunCombined("rm -f /var/run/keepalived.pid 2>/dev/null")
	r.AddLog("   状态文件已清理")

	r.InstallStep = r.InstallTotal
	r.AddLog("卸载全部完成!")
	r.AddLog("")
	r.AddLog("=== 提示 ===")
	r.AddLog("如果之前修改了 DHCP 网关设置，请记得改回原来的网关")
	r.AddLog("===========")
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
// Returns an error if no peer router is found (at least 2 routers required for HA).
func (m *Manager) GenerateAgentConfig(r *Router) (*config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.Role = r.Role
	cfg.LAN.VIP = m.config.LAN.VIP
	cfg.LAN.CIDR = m.config.LAN.CIDR

	// Router must have its own interface configured
	if r.Iface == "" {
		return nil, fmt.Errorf("路由器 %s 未配置网卡接口，请在路由器设置中指定", r.Name)
	}
	cfg.LAN.Iface = r.Iface

	cfg.Keepalived.VRID = m.config.Keepalived.VRID

	// Logic Optimization: Set default health mode based on role
	if r.HealthMode != "" {
		cfg.Health.Mode = config.HealthMode(r.HealthMode)
	} else {
		// Primary focuses on basic connectivity, secondary on internet/proxy connectivity
		if r.Role == config.RolePrimary {
			cfg.Health.Mode = config.HealthModeBasic
		} else {
			cfg.Health.Mode = config.HealthModeInternet
		}
	}

	// SelfIP will be auto-detected from interface during installation
	// ... (rest of the code stays same)
	// Find peer
	found := false
	for _, other := range m.config.Routers {
		if other.Name != r.Name {
			cfg.Routers.PeerIP = other.Host
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("未找到对端路由器，安装 Agent 需要至少配置两台路由器 (primary + secondary)")
	}

	return cfg, nil
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

	// Validate each router and check for VIP conflict
	hasPrimary := false
	hasSecondary := false
	for _, r := range m.config.Routers {
		if r.Name == "" {
			return fmt.Errorf("router name is required")
		}
		if r.Host == "" {
			return fmt.Errorf("router %s: host is required", r.Name)
		}
		// Security Check: VIP must not conflict with any router's Host IP
		if r.Host == m.config.LAN.VIP {
			return fmt.Errorf("VIP (%s) 冲突: 不能与路由器 %s 的 Host IP 相同", m.config.LAN.VIP, r.Name)
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

// cidrToNetwork converts an IP/prefix (like 192.168.1.100/24) to network address (192.168.1.0/24).
func cidrToNetwork(cidrFull string) string {
	_, ipNet, err := net.ParseCIDR(cidrFull)
	if err != nil {
		// Fallback to simple calculation for /24
		parts := strings.Split(cidrFull, "/")
		if len(parts) == 2 {
			ipDots := strings.Split(parts[0], ".")
			if len(ipDots) == 4 {
				return fmt.Sprintf("%s.%s.%s.0/%s", ipDots[0], ipDots[1], ipDots[2], parts[1])
			}
		}
		return ""
	}
	return ipNet.String()
}

// SuggestVIP generates a suggested VIP address based on the CIDR.
// It tries .254, .253, .252 etc. until finding an available one.
func (m *Manager) SuggestVIP(cidr string) string {
	if cidr == "" {
		return ""
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}

	// Get network address as 4 bytes
	ip := ipNet.IP.To4()
	if ip == nil {
		return ""
	}

	// Get mask to determine valid host range
	mask := ipNet.Mask
	ones, bits := mask.Size()
	if bits != 32 {
		return ""
	}

	// Calculate number of host bits
	hostBits := bits - ones
	if hostBits < 2 {
		return "" // Too small network
	}

	// Try common gateway addresses: .254, .253, .252, .251, .1
	// For a /24 network, these would be x.x.x.254, x.x.x.253, etc.
	candidates := []int{254, 253, 252, 251, 1}

	// Get existing router IPs to avoid conflicts
	existingIPs := make(map[string]bool)
	for _, r := range m.config.Routers {
		existingIPs[r.Host] = true
	}

	for _, lastOctet := range candidates {
		candidateIP := net.IPv4(ip[0], ip[1], ip[2], byte(lastOctet))

		// Check if this IP is within the network
		if !ipNet.Contains(candidateIP) {
			continue
		}

		// Check if it's not already used by a router
		candidateStr := candidateIP.String()
		if existingIPs[candidateStr] {
			continue
		}

		return candidateStr
	}

	// Default fallback
	return fmt.Sprintf("%d.%d.%d.254", ip[0], ip[1], ip[2])
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
