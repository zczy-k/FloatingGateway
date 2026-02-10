// gateway-agent is the main agent binary for floating gateway management.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/doctor"
	"github.com/zczy-k/FloatingGateway/internal/health/policy"
	"github.com/zczy-k/FloatingGateway/internal/keepalived"
	"github.com/zczy-k/FloatingGateway/internal/platform/detect"
	"github.com/zczy-k/FloatingGateway/internal/platform/netutil"
	"github.com/zczy-k/FloatingGateway/internal/version"
)

const (
	defaultConfigPath = "/etc/gateway-agent/config.yaml"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "check":
		checkCmd(os.Args[2:])
	case "render":
		renderCmd(os.Args[2:])
	case "apply":
		applyCmd(os.Args[2:])
	case "doctor":
		doctorCmd(os.Args[2:])
	case "status":
		statusCmd(os.Args[2:])
	case "notify":
		notifyCmd(os.Args[2:])
	case "detect-iface":
		detectIfaceCmd(os.Args[2:])
	case "version":
		fmt.Printf("gateway-agent %s\n", version.Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gateway-agent - Floating Gateway Agent

Usage:
  gateway-agent <command> [options]

Commands:
  run       Run the agent daemon (for continuous health monitoring)
  check     Perform a single health check (for keepalived track_script)
  render    Output the rendered keepalived configuration
  apply     Write keepalived config and reload the service
  doctor    Run self-diagnosis checks
  status    Show current status
  notify    Handle keepalived state notifications
  detect-iface Detect primary network interface
  version   Print version information

Options:
  -c, --config   Path to config file (default: /etc/gateway-agent/config.yaml)

Examples:
  gateway-agent run
  gateway-agent check --mode=internet
  gateway-agent doctor --fix
  gateway-agent status --json`)
}

func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Fill in self_ip if not set
	if cfg.Routers.SelfIP == "" {
		info, err := netutil.GetInterfaceInfo(cfg.LAN.Iface)
		if err == nil && info.IPv4 != "" {
			cfg.Routers.SelfIP = info.IPv4
		}
	}

	// Fill in CIDR if not set
	if cfg.LAN.CIDR == "" {
		info, err := netutil.GetInterfaceInfo(cfg.LAN.Iface)
		if err == nil && info.CIDR != "" {
			cfg.LAN.CIDR = info.CIDR
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting gateway-agent (version=%s, role=%s, mode=%s)\n", version.Version, cfg.Role, cfg.Health.Mode)

	// Create health policy
	healthPolicy, err := policy.NewPolicy(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating health policy: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Run health check loop
	ticker := time.NewTicker(time.Duration(cfg.Health.IntervalSec) * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial check
	status := healthPolicy.Check(ctx)
	logStatus(status)

	for {
		select {
		case <-ticker.C:
			status := healthPolicy.Check(ctx)
			logStatus(status)

		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				fmt.Println("Received SIGHUP, reloading config...")
				newCfg, err := loadConfig(*configPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Config reload failed: %v\n", err)
					continue
				}
				newPolicy, err := policy.NewPolicy(newCfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Policy reload failed: %v\n", err)
					continue
				}
				cfg = newCfg
				healthPolicy = newPolicy
				fmt.Println("Config reloaded successfully")

			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Println("Shutting down...")
				return
			}
		}
	}
}

func logStatus(status *policy.Status) {
	stateStr := "HEALTHY"
	if !status.Healthy {
		stateStr = "UNHEALTHY"
	}
	fmt.Printf("[%s] %s: %s (%d/%d checks passed)\n",
		time.Now().Format("15:04:05"),
		stateStr,
		status.Reason,
		status.PassedCount,
		status.TotalCount,
	)
}

func checkCmd(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	mode := fs.String("mode", "", "health check mode (basic/internet)")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Override mode if specified
	if *mode != "" {
		cfg.Health.Mode = config.HealthMode(*mode)
	}

	// Create health policy
	healthPolicy, err := policy.NewPolicy(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Run check
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status := healthPolicy.Check(ctx)

	// For keepalived track_script: exit 0 = healthy, exit 1 = unhealthy
	if status.Healthy {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}

func renderCmd(args []string) {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	renderer := keepalived.NewRenderer(cfg)
	content, err := renderer.Render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering config: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(content)
}

func applyCmd(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating and applying keepalived configuration...")
	if err := keepalived.Apply(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration applied to %s\n", keepalived.FindConfigPath())
	fmt.Println("keepalived reloaded successfully")
}

func doctorCmd(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	autoFix := fs.Bool("fix", false, "automatically fix problems")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	doc := doctor.New(cfg, *autoFix)
	report := doc.Run()

	if *jsonOutput {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
	} else {
		doctor.PrintReport(report)
	}

	if report.HasErrors {
		os.Exit(1)
	}
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Gather status information
	status := struct {
		Version    string             `json:"version"`
		Role       string             `json:"role"`
		Platform   string             `json:"platform"`
		Interface  string             `json:"interface"`
		CIDR       string             `json:"cidr"`
		VIP        string             `json:"vip"`
		SelfIP     string             `json:"self_ip"`
		PeerIP     string             `json:"peer_ip"`
		HealthMode string             `json:"health_mode"`
		Keepalived *keepalived.Status `json:"keepalived"`
		Health     *policy.Status     `json:"health,omitempty"`
	}{
		Version:    version.Version,
		Role:       string(cfg.Role),
		Platform:   detect.Detect().String(),
		Interface:  cfg.LAN.Iface,
		CIDR:       cfg.LAN.CIDR,
		VIP:        cfg.LAN.VIP,
		SelfIP:     cfg.Routers.SelfIP,
		PeerIP:     cfg.Routers.PeerIP,
		HealthMode: string(cfg.Health.Mode),
		Keepalived: keepalived.GetStatus(),
	}

	// Run a quick health check
	healthPolicy, err := policy.NewPolicy(cfg)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		status.Health = healthPolicy.Check(ctx)
		cancel()
	}

	if *jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Gateway Agent Status\n")
		fmt.Printf("====================\n")
		fmt.Printf("Version:      %s\n", status.Version)
		fmt.Printf("Role:         %s\n", status.Role)
		fmt.Printf("Platform:     %s\n", status.Platform)
		fmt.Printf("Interface:    %s\n", status.Interface)
		fmt.Printf("CIDR:         %s\n", status.CIDR)
		fmt.Printf("VIP:          %s\n", status.VIP)
		fmt.Printf("Self IP:      %s\n", status.SelfIP)
		fmt.Printf("Peer IP:      %s\n", status.PeerIP)
		fmt.Printf("Health Mode:  %s\n", status.HealthMode)
		fmt.Println()
		fmt.Printf("Keepalived\n")
		fmt.Printf("----------\n")
		fmt.Printf("Running:      %v\n", status.Keepalived.Running)
		fmt.Printf("Config:       %s\n", status.Keepalived.ConfigPath)
		fmt.Printf("Config Valid: %v\n", status.Keepalived.ConfigValid)
		if status.Keepalived.VRRPState != "" {
			fmt.Printf("VRRP State:   %s\n", status.Keepalived.VRRPState)
		}
		if status.Health != nil {
			fmt.Println()
			fmt.Printf("Health Check\n")
			fmt.Printf("------------\n")
			fmt.Printf("Healthy:      %v\n", status.Health.Healthy)
			fmt.Printf("State:        %s\n", status.Health.State)
			fmt.Printf("Passed:       %d/%d\n", status.Health.PassedCount, status.Health.TotalCount)
			fmt.Printf("Reason:       %s\n", status.Health.Reason)
		}
	}
}

func notifyCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: gateway-agent notify <state>\n")
		os.Exit(1)
	}

	state := args[0]
	cfg, _ := loadConfig(defaultConfigPath)

	iface := "eth0"
	vip := ""
	if cfg != nil {
		iface = cfg.LAN.Iface
		vip = cfg.LAN.VIP
	}

	switch state {
	case "master", "MASTER":
		fmt.Printf("Transitioning to MASTER state\n")
		// Send GARP to announce VIP
		if vip != "" {
			if err := netutil.SendGARP(vip, iface); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: GARP failed: %v\n", err)
			} else {
				fmt.Printf("Sent GARP for %s on %s\n", vip, iface)
			}
		}

	case "backup", "BACKUP":
		fmt.Printf("Transitioning to BACKUP state\n")

	case "fault", "FAULT":
		fmt.Printf("Transitioning to FAULT state\n")

	default:
		fmt.Printf("Unknown state: %s\n", state)
	}
}

func detectIfaceCmd(args []string) {
	iface, err := netutil.DetectPrimaryInterface()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(iface)
}
