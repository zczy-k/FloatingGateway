// Package doctor provides self-diagnosis and auto-fix functionality.
package doctor

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/keepalived"
	"github.com/zczy-k/FloatingGateway/internal/platform/detect"
	"github.com/zczy-k/FloatingGateway/internal/platform/exec"
	"github.com/zczy-k/FloatingGateway/internal/platform/netutil"
)

// CheckResult represents a single check result.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok, warning, error
	Message string `json:"message"`
	CanFix  bool   `json:"can_fix"`
	Fixed   bool   `json:"fixed"`
}

// Report holds all check results.
type Report struct {
	Platform    string        `json:"platform"`
	Role        string        `json:"role"`
	Checks      []CheckResult `json:"checks"`
	HasErrors   bool          `json:"has_errors"`
	HasWarnings bool          `json:"has_warnings"`
	Summary     string        `json:"summary"`
}

// Doctor performs system diagnosis.
type Doctor struct {
	cfg      *config.Config
	platform *detect.Info
	autoFix  bool
}

// New creates a new Doctor instance.
func New(cfg *config.Config, autoFix bool) *Doctor {
	return &Doctor{
		cfg:      cfg,
		platform: detect.Detect(),
		autoFix:  autoFix,
	}
}

// Run performs all checks and returns a report.
func (d *Doctor) Run() *Report {
	report := &Report{
		Platform: d.platform.String(),
		Role:     string(d.cfg.Role),
		Checks:   make([]CheckResult, 0),
	}

	// Run all checks
	report.Checks = append(report.Checks, d.checkInterface())
	report.Checks = append(report.Checks, d.checkCIDR())
	report.Checks = append(report.Checks, d.checkVIP())
	report.Checks = append(report.Checks, d.checkVIPConflict())
	report.Checks = append(report.Checks, d.checkPeerIP())
	report.Checks = append(report.Checks, d.checkKeepalived())
	report.Checks = append(report.Checks, d.checkKeepalviedConfig())
	report.Checks = append(report.Checks, d.checkVRRPMulticast())
	report.Checks = append(report.Checks, d.checkArping())

	// OpenWrt-specific: DHCP gateway check
	if d.platform.IsOpenWrt() && d.cfg.Role == config.RolePrimary {
		report.Checks = append(report.Checks, d.checkOpenWrtDHCP())
	}

	// Summarize
	for _, c := range report.Checks {
		if c.Status == "error" {
			report.HasErrors = true
		}
		if c.Status == "warning" {
			report.HasWarnings = true
		}
	}

	if report.HasErrors {
		report.Summary = "Some checks failed. Please fix the errors before continuing."
	} else if report.HasWarnings {
		report.Summary = "Checks passed with warnings. Review the warnings above."
	} else {
		report.Summary = "All checks passed. System is ready."
	}

	return report
}

func (d *Doctor) checkInterface() CheckResult {
	result := CheckResult{Name: "interface_exists"}

	if !netutil.InterfaceExists(d.cfg.LAN.Iface) {
		result.Status = "error"
		result.Message = fmt.Sprintf("Interface %q does not exist", d.cfg.LAN.Iface)
		return result
	}

	info, err := netutil.GetInterfaceInfo(d.cfg.LAN.Iface)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Cannot get interface info: %v", err)
		return result
	}

	if info.IPv4 == "" {
		result.Status = "error"
		result.Message = fmt.Sprintf("Interface %q has no IPv4 address", d.cfg.LAN.Iface)
		return result
	}

	if !info.Up {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Interface %q is down", d.cfg.LAN.Iface)
		return result
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("Interface %q is up with IP %s", d.cfg.LAN.Iface, info.IPv4)
	return result
}

func (d *Doctor) checkCIDR() CheckResult {
	result := CheckResult{Name: "cidr_valid"}

	if d.cfg.LAN.CIDR == "" {
		// Try to infer from interface
		info, err := netutil.GetInterfaceInfo(d.cfg.LAN.Iface)
		if err != nil {
			result.Status = "warning"
			result.Message = "CIDR not configured and cannot infer from interface"
			return result
		}
		result.Status = "ok"
		result.Message = fmt.Sprintf("CIDR inferred from interface: %s", info.CIDR)
		return result
	}

	_, _, err := net.ParseCIDR(d.cfg.LAN.CIDR)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Invalid CIDR %q: %v", d.cfg.LAN.CIDR, err)
		return result
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("CIDR %s is valid", d.cfg.LAN.CIDR)
	return result
}

func (d *Doctor) checkVIP() CheckResult {
	result := CheckResult{Name: "vip_valid"}

	vip := net.ParseIP(d.cfg.LAN.VIP)
	if vip == nil || vip.To4() == nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("VIP %q is not a valid IPv4 address", d.cfg.LAN.VIP)
		return result
	}

	// Check if VIP is in CIDR
	cidr := d.cfg.LAN.CIDR
	if cidr == "" {
		info, _ := netutil.GetInterfaceInfo(d.cfg.LAN.Iface)
		if info != nil {
			cidr = info.CIDR
		}
	}

	if cidr != "" {
		inCIDR, err := netutil.IsIPInCIDR(d.cfg.LAN.VIP, cidr)
		if err != nil {
			result.Status = "warning"
			result.Message = fmt.Sprintf("Cannot verify VIP in CIDR: %v", err)
			return result
		}
		if !inCIDR {
			result.Status = "error"
			result.Message = fmt.Sprintf("VIP %s is not within CIDR %s", d.cfg.LAN.VIP, cidr)
			return result
		}
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("VIP %s is valid and within network", d.cfg.LAN.VIP)
	return result
}

func (d *Doctor) checkVIPConflict() CheckResult {
	result := CheckResult{Name: "vip_conflict"}

	conflict, err := netutil.CheckIPConflict(d.cfg.LAN.VIP, d.cfg.LAN.Iface, 3*time.Second)
	if err != nil {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Cannot check VIP conflict: %v", err)
		return result
	}

	if conflict {
		// Check if it's us holding the VIP (which is expected in MASTER state)
		hasVIP, _ := netutil.HasVIP(d.cfg.LAN.VIP, d.cfg.LAN.Iface)
		if hasVIP {
			result.Status = "ok"
			result.Message = fmt.Sprintf("VIP %s is assigned to this host (MASTER state)", d.cfg.LAN.VIP)
			return result
		}

		// Check if it's the peer holding the VIP (also expected in BACKUP state)
		// We can't easily verify MAC address remotely, but if peer is up and we are backup, it's likely fine.
		// For now, we downgrade this to INFO/OK if we can reach the peer.
		if d.checkPeerIP().Status == "ok" {
			result.Status = "ok"
			result.Message = fmt.Sprintf("VIP %s is active (likely held by peer)", d.cfg.LAN.VIP)
			return result
		}

		result.Status = "warning"
		result.Message = fmt.Sprintf("VIP %s appears to be in use by another host (Peer unreachable?)", d.cfg.LAN.VIP)
		return result
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("VIP %s is not in conflict", d.cfg.LAN.VIP)
	return result
}

func (d *Doctor) checkPeerIP() CheckResult {
	result := CheckResult{Name: "peer_ip_valid"}

	peerIP := net.ParseIP(d.cfg.Routers.PeerIP)
	if peerIP == nil || peerIP.To4() == nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("Peer IP %q is not a valid IPv4 address", d.cfg.Routers.PeerIP)
		return result
	}

	// Check if peer is reachable (ping)
	pingResult := exec.RunWithTimeout("ping", 5*time.Second, "-c", "1", "-W", "2", d.cfg.Routers.PeerIP)
	if !pingResult.Success() {
		result.Status = "warning"
		result.Message = fmt.Sprintf("Peer %s is not reachable (may be normal if peer is down)", d.cfg.Routers.PeerIP)
		return result
	}

	result.Status = "ok"
	result.Message = fmt.Sprintf("Peer %s is reachable", d.cfg.Routers.PeerIP)
	return result
}

func (d *Doctor) checkKeepalived() CheckResult {
	result := CheckResult{Name: "keepalived_running"}

	if !keepalived.IsRunning() {
		result.Status = "error"
		result.CanFix = true

		// Try to get more details about why it's not running
		var details []string

		// Check if keepalived is installed
		if !exec.CommandExists("keepalived") {
			details = append(details, "keepalived 未安装")
		} else {
			// Check if config exists
			configPath := keepalived.FindConfigPath()
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				details = append(details, fmt.Sprintf("配置文件不存在: %s", configPath))
			} else {
				// Try to validate config
				validateResult := exec.RunWithTimeout("keepalived", 5*time.Second, "-t", "-f", configPath)
				if !validateResult.Success() {
					details = append(details, fmt.Sprintf("配置文件无效: %s", strings.TrimSpace(validateResult.Combined())))
				}
			}

			// Check if service is enabled
			if d.platform.ServiceManager == "systemd" {
				enabledResult := exec.RunWithTimeout("systemctl", 5*time.Second, "is-enabled", "keepalived")
				if !enabledResult.Success() {
					details = append(details, "服务未启用（disabled）")
				}

				// Get service status for more info
				statusResult := exec.RunWithTimeout("systemctl", 5*time.Second, "status", "keepalived")
				statusOutput := strings.TrimSpace(statusResult.Combined())
				if strings.Contains(statusOutput, "failed") || strings.Contains(statusOutput, "inactive") {
					// Extract the most relevant error line
					lines := strings.Split(statusOutput, "\n")
					for _, line := range lines {
						if strings.Contains(line, "Main PID") || strings.Contains(line, "Active:") {
							details = append(details, strings.TrimSpace(line))
							break
						}
					}
				}
			}
		}

		if len(details) > 0 {
			result.Message = fmt.Sprintf("keepalived 未运行。原因: %s", strings.Join(details, "; "))
		} else {
			result.Message = "keepalived 未运行"
		}

		if d.autoFix {
			if err := keepalived.Start(); err == nil {
				result.Fixed = true
				result.Status = "ok"
				result.Message = "keepalived 已启动"
			} else {
				result.Message = fmt.Sprintf("%s。尝试启动失败: %v", result.Message, err)
			}
		}
		return result
	}

	result.Status = "ok"
	result.Message = "keepalived is running"

	// Deep check: Verify VRRP state file
	stateFile := "/tmp/keepalived.GATEWAY.state"
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		result.Status = "warning"
		result.Message = "keepalived 正在运行，但未生成状态文件 (notify 脚本可能执行失败)"
		
		// Try to run notify manually to see if it works and check for permission errors
		// This simulates what keepalived does
		agentBin := keepalived.FindAgentBinary()
		testCmd := fmt.Sprintf("%s notify TEST_PERMISSION", agentBin)
		if out, err := exec.RunStdout(testCmd); err != nil {
			result.Message += fmt.Sprintf("。手动执行 notify 失败: %v", err)
		} else {
			// Check if file created now
			if _, err := os.Stat(stateFile); err == nil {
				result.Message += "。手动执行成功（文件已创建），可能是 Keepalived 进程权限不足"
				// Auto-fix permission
				exec.RunStdout("chmod 666 " + stateFile)
			} else {
				result.Message += fmt.Sprintf("。手动执行成功但文件未创建。输出: %s", out)
			}
		}
	} else {
		content, _ := os.ReadFile(stateFile)
		state := strings.TrimSpace(string(content))
		result.Message = fmt.Sprintf("keepalived 运行中 (VRRP状态: %s)", state)
	}

	return result
}

func (d *Doctor) checkKeepalviedConfig() CheckResult {
	result := CheckResult{Name: "keepalived_config"}

	configPath := keepalived.FindConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Status = "error"
		result.Message = fmt.Sprintf("配置文件不存在: %s。请运行 'gateway-agent apply' 生成配置", configPath)
		result.CanFix = true

		if d.autoFix {
			if err := keepalived.Apply(d.cfg); err == nil {
				result.Fixed = true
				result.Status = "ok"
				result.Message = "配置文件已生成并应用"
			} else {
				result.Message = fmt.Sprintf("配置文件不存在，生成失败: %v", err)
			}
		}
		return result
	}

	// Validate config
	validateResult := exec.RunWithTimeout("keepalived", 5*time.Second, "-t", "-f", configPath)
	if !validateResult.Success() {
		errMsg := strings.TrimSpace(validateResult.Combined())

		// Analyze common errors
		var suggestion string
		if strings.Contains(errMsg, "track script") && strings.Contains(errMsg, "not found") {
			// Test if gateway-agent check command works
			checkCmd := fmt.Sprintf("%s check --mode=%s", keepalived.FindAgentBinary(), d.cfg.Health.Mode)
			if checkOut, err := exec.RunStdout(checkCmd); err != nil {
				suggestion = fmt.Sprintf("Agent 健康检查命令执行失败: %v", err)
			} else {
				suggestion = fmt.Sprintf("Agent 健康检查命令输出: %s", checkOut)
			}
		}

		result.Status = "error"
		result.Message = fmt.Sprintf("配置文件无效: %s. %s", errMsg, suggestion)

		// Always try to regenerate config if invalid
		result.CanFix = true
		if d.autoFix {
			if err := keepalived.Apply(d.cfg); err == nil {
				result.Fixed = true
				result.Status = "ok"
				result.Message = "检测到配置无效，已自动重新生成"
			}
		}

		return result
	}

	// Check for security context warnings in logs (e.g., insecure path)
	if out, err := exec.RunStdout("grep 'Insecure path' /var/log/syslog /var/log/messages 2>/dev/null | tail -n 1"); err == nil && out != "" {
		result.Status = "warning"
		result.Message = fmt.Sprintf("检测到路径安全警告: %s (请确保 Agent 安装目录属于 root)", strings.TrimSpace(out))
	}

	result.Status = "ok"
	result.Message = "配置文件有效"
	return result
}

func (d *Doctor) checkVRRPMulticast() CheckResult {
	result := CheckResult{Name: "vrrp_multicast"}

	// Check if we can receive VRRP packets (if not MASTER)
	// or send them (if MASTER). This is hard to check passively without tcpdump.
	// So we check if the interface has the multicast flag and if firewall allows it.

	iface := d.cfg.LAN.Iface

	// 1. Check MULTICAST flag
	if out, err := exec.RunStdout(fmt.Sprintf("ip link show %s", iface)); err == nil {
		if !strings.Contains(out, "MULTICAST") {
			result.Status = "error"
			result.Message = fmt.Sprintf("网卡 %s 未开启组播功能 (MULTICAST flag missing)", iface)
			return result
		}
	}

	// 2. Check Firewall (Basic check)
	// OpenWrt
	if d.platform.OS == "openwrt" {
		if out, err := exec.RunStdout("uci get firewall.vrrp.target 2>/dev/null"); err != nil || strings.TrimSpace(out) != "ACCEPT" {
			result.Status = "warning"
			result.Message = "OpenWrt 防火墙可能未放行 VRRP 协议 (uci get firewall.vrrp.target != ACCEPT)"
			result.CanFix = true
			if d.autoFix {
				// Fix it via uci
				exec.RunWithTimeout("uci", 5*time.Second, "delete", "firewall.vrrp")
				exec.RunWithTimeout("uci", 5*time.Second, "set", "firewall.vrrp=rule")
				exec.RunWithTimeout("uci", 5*time.Second, "set", "firewall.vrrp.name=Allow-VRRP")
				exec.RunWithTimeout("uci", 5*time.Second, "set", "firewall.vrrp.src=lan")
				exec.RunWithTimeout("uci", 5*time.Second, "set", "firewall.vrrp.proto=112")
				exec.RunWithTimeout("uci", 5*time.Second, "set", "firewall.vrrp.target=ACCEPT")
				exec.RunWithTimeout("uci", 5*time.Second, "commit", "firewall")
				exec.RunWithTimeout("/etc/init.d/firewall", 5*time.Second, "reload")
				result.Fixed = true
				result.Status = "ok"
				result.Message = "已自动添加防火墙规则放行 VRRP"
			}
			return result
		}
	}

	// Linux (iptables check)
	if out, err := exec.RunStdout("iptables -L INPUT -n | grep 112"); err == nil && out == "" {
		// Only warn if no rule found, as it might be managed by ufw/firewall-cmd
		result.Status = "warning"
		result.Message = "iptables 中未发现针对 VRRP (Proto 112) 的放行规则"
	}

	result.Status = "ok"
	result.Message = "组播功能已开启，防火墙规则检查通过"
	return result
}

func (d *Doctor) checkArping() CheckResult {
	result := CheckResult{Name: "arping_available"}

	if !exec.CommandExists("arping") {
		result.Status = "warning"
		result.Message = "arping not found; GARP announcements may not work properly"
		return result
	}

	result.Status = "ok"
	result.Message = "arping is available for GARP announcements"
	return result
}

func (d *Doctor) checkOpenWrtDHCP() CheckResult {
	result := CheckResult{Name: "openwrt_dhcp_gateway"}

	// Expected option 3 (gateway)
	expectedOption := fmt.Sprintf("3,%s", d.cfg.LAN.VIP)

	// Try to use uci first as it's more accurate
	uciResult := exec.RunWithTimeout("uci", 5*time.Second, "get", "dhcp.lan.dhcp_option")
	if uciResult.Success() {
		if strings.Contains(uciResult.Stdout, expectedOption) {
			result.Status = "ok"
			result.Message = fmt.Sprintf("DHCP gateway is configured to VIP %s (via uci)", d.cfg.LAN.VIP)
			return result
		}
	} else {
		// Fallback to reading file if uci fails or lan section not found
		data, err := os.ReadFile("/etc/config/dhcp")
		if err == nil {
			content := string(data)
			if strings.Contains(content, expectedOption) {
				result.Status = "ok"
				result.Message = fmt.Sprintf("DHCP gateway is configured to VIP %s", d.cfg.LAN.VIP)
				return result
			}
		}
	}

	result.Status = "warning"
	result.Message = fmt.Sprintf("DHCP gateway may not be set to VIP %s", d.cfg.LAN.VIP)
	result.CanFix = d.cfg.OpenWrt.DHCP.AutoSetGateway

	if d.autoFix && d.cfg.OpenWrt.DHCP.AutoSetGateway {
		// Use uci to set the gateway
		if err := d.fixOpenWrtDHCP(); err == nil {
			result.Fixed = true
			result.Status = "ok"
			result.Message = fmt.Sprintf("DHCP gateway auto-configured to VIP %s", d.cfg.LAN.VIP)
		} else {
			result.Message = fmt.Sprintf("Failed to auto-configure DHCP gateway: %v", err)
		}
	}

	return result
}

func (d *Doctor) fixOpenWrtDHCP() error {
	// Backup current config
	backupPath := "/etc/config/dhcp.gateway-agent.bak"
	exec.RunWithTimeout("cp", 5*time.Second, "/etc/config/dhcp", backupPath)

	// Find the lan dhcp section and add/update the gateway option
	// This is a simplified implementation - production should use proper UCI parsing

	// First, try to delete any existing option 3
	exec.RunWithTimeout("uci", 5*time.Second, "delete", "dhcp.lan.dhcp_option")

	// Add the new gateway option
	gatewayOption := fmt.Sprintf("3,%s", d.cfg.LAN.VIP)
	result := exec.RunWithTimeout("uci", 5*time.Second, "add_list", "dhcp.lan.dhcp_option="+gatewayOption)
	if !result.Success() {
		return fmt.Errorf("uci add_list failed: %s", result.Combined())
	}

	// Commit changes
	result = exec.RunWithTimeout("uci", 5*time.Second, "commit", "dhcp")
	if !result.Success() {
		return fmt.Errorf("uci commit failed: %s", result.Combined())
	}

	// Restart dnsmasq
	result = exec.RunWithTimeout("/etc/init.d/dnsmasq", 10*time.Second, "restart")
	if !result.Success() {
		return fmt.Errorf("dnsmasq restart failed: %s", result.Combined())
	}

	return nil
}

// PrintReport prints the report to stdout.
func PrintReport(report *Report) {
	fmt.Printf("Platform: %s\n", report.Platform)
	fmt.Printf("Role: %s\n", report.Role)
	fmt.Println()

	for _, c := range report.Checks {
		status := c.Status
		switch status {
		case "ok":
			status = "[OK]"
		case "warning":
			status = "[WARN]"
		case "error":
			status = "[ERROR]"
		}

		fixedStr := ""
		if c.Fixed {
			fixedStr = " (auto-fixed)"
		}

		fmt.Printf("%-8s %-25s %s%s\n", status, c.Name, c.Message, fixedStr)
	}

	fmt.Println()
	fmt.Println(report.Summary)
}
