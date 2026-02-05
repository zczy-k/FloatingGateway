// gateway-controller is the central management tool for the floating gateway system.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/floatip/gateway/internal/controller"
	"os/exec"
	"runtime"
	"time"
)

const (
	defaultConfigPath = "controller.yaml"
	version           = "1.0.1"
)

func main() {
	if len(os.Args) < 2 {
		// Default to serve for Windows double-click support
		serveCmd([]string{})
		return
	}

	switch os.Args[1] {
	case "serve":
		serveCmd(os.Args[2:])
	case "probe":
		probeCmd(os.Args[2:])
	case "install":
		installCmd(os.Args[2:])
	case "uninstall":
		uninstallCmd(os.Args[2:])
	case "status":
		statusCmd(os.Args[2:])
	case "version":
		fmt.Printf("gateway-controller %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gateway-controller - Floating Gateway Controller

Usage:
  gateway-controller <command> [options]

Commands:
  serve       Start the web UI and API server (opens browser automatically)
  probe       Probe all routers for status
  install     Install agent on routers
  uninstall   Uninstall agent from routers
  status      Show overall status
  version     Print version information

Options:
  -c, --config   Path to config file (default: controller.yaml)

Examples:
  gateway-controller serve -c controller.yaml
  gateway-controller probe
  gateway-controller install --router=openwrt-main
  gateway-controller status`)
}

func loadManager(configPath string) (*controller.Manager, error) {
	manager, err := controller.NewManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return manager, nil
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	listen := fs.String("listen", "", "listen address (overrides config)")
	noBrowser := fs.Bool("no-browser", false, "don't open browser automatically")
	fs.Parse(args)

	manager, err := loadManager(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// If config file doesn't exist, it will be created on first save
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Printf("Config file %s not found. Starting in setup mode (defaults: :8080)\n", *configPath)
	}

	// Determine listen address
	addr := manager.GetConfig().Listen
	if *listen != "" {
		addr = *listen
	}
	if addr == "" {
		addr = ":8080"
	}

	fmt.Printf("Starting gateway-controller on %s\n", addr)

	// Determine the URL to open
	host := "localhost"
	port := addr
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if parts[0] != "" {
			host = parts[0]
		}
		port = ":" + parts[1]
	}
	url := fmt.Sprintf("http://%s%s", host, port)
	fmt.Printf("Open %s in your browser\n", url)

	// Open browser in background if requested
	if !*noBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond) // Wait a bit for server to start
			openBrowser(url)
		}()
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	server := controller.NewServer(manager)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Stop()
	}()

	if err := server.Start(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}
}

func probeCmd(args []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	routerName := fs.String("router", "", "specific router to probe (default: all)")
	fs.Parse(args)

	manager, err := loadManager(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "Router %q not found\n", *routerName)
			os.Exit(1)
		}
		fmt.Printf("Probing %s...\n", router.Name)
		if err := manager.Probe(router); err != nil {
			fmt.Printf("  Status: OFFLINE (%v)\n", err)
		} else {
			fmt.Printf("  Status: %s\n", router.Status)
			fmt.Printf("  Platform: %s\n", router.Platform)
			fmt.Printf("  Agent: %s\n", router.AgentVer)
			if router.VRRPState != "" {
				fmt.Printf("  VRRP: %s\n", router.VRRPState)
			}
		}
	} else {
		fmt.Println("Probing all routers...")
		manager.ProbeAll()
		for _, router := range manager.GetRouters() {
			fmt.Printf("\n%s (%s):\n", router.Name, router.Role)
			fmt.Printf("  Host: %s:%d\n", router.Host, router.Port)
			fmt.Printf("  Status: %s\n", router.Status)
			if router.Status == controller.StatusOnline {
				fmt.Printf("  Platform: %s\n", router.Platform)
				if router.AgentVer != "" {
					fmt.Printf("  Agent: %s\n", router.AgentVer)
					if router.VRRPState != "" {
						fmt.Printf("  VRRP: %s\n", router.VRRPState)
					}
					if router.Healthy != nil {
						fmt.Printf("  Healthy: %v\n", *router.Healthy)
					}
				} else {
					fmt.Printf("  Agent: Not installed\n")
				}
			} else if router.Error != "" {
				fmt.Printf("  Error: %s\n", router.Error)
			}
		}
	}
}

func installCmd(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	routerName := fs.String("router", "", "router to install on (required)")
	all := fs.Bool("all", false, "install on all routers")
	fs.Parse(args)

	manager, err := loadManager(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var routers []*controller.Router
	if *all {
		routers = manager.GetRouters()
	} else if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "Router %q not found\n", *routerName)
			os.Exit(1)
		}
		routers = []*controller.Router{router}
	} else {
		fmt.Fprintf(os.Stderr, "Error: --router or --all is required\n")
		os.Exit(1)
	}

	for _, router := range routers {
		fmt.Printf("Installing agent on %s...\n", router.Name)
		agentConfig := manager.GenerateAgentConfig(router)
		if err := manager.Install(router, agentConfig); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed: %v\n", err)
		} else {
			fmt.Printf("  Success!\n")
		}
	}
}

func uninstallCmd(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	routerName := fs.String("router", "", "router to uninstall from (required)")
	all := fs.Bool("all", false, "uninstall from all routers")
	fs.Parse(args)

	manager, err := loadManager(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var routers []*controller.Router
	if *all {
		routers = manager.GetRouters()
	} else if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "Router %q not found\n", *routerName)
			os.Exit(1)
		}
		routers = []*controller.Router{router}
	} else {
		fmt.Fprintf(os.Stderr, "Error: --router or --all is required\n")
		os.Exit(1)
	}

	for _, router := range routers {
		fmt.Printf("Uninstalling agent from %s...\n", router.Name)
		if err := manager.Uninstall(router); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed: %v\n", err)
		} else {
			fmt.Printf("  Success!\n")
		}
	}
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("c", defaultConfigPath, "config file path")
	fs.StringVar(configPath, "config", defaultConfigPath, "config file path")
	fs.Parse(args)

	manager, err := loadManager(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg := manager.GetConfig()

	fmt.Println("Floating Gateway Status")
	fmt.Println("=======================")
	fmt.Printf("VIP: %s\n", cfg.LAN.VIP)
	fmt.Printf("CIDR: %s\n", cfg.LAN.CIDR)
	fmt.Printf("VRID: %d\n", cfg.Keepalived.VRID)
	fmt.Println()

	// Probe all routers
	fmt.Println("Probing routers...")
	manager.ProbeAll()

	// Find master
	var master string
	for _, router := range manager.GetRouters() {
		if router.VRRPState == "MASTER" {
			master = router.Name
			break
		}
	}

	fmt.Printf("\nCurrent Master: %s\n", master)
	fmt.Println()

	fmt.Println("Routers:")
	for _, router := range manager.GetRouters() {
		statusIcon := "?"
		switch router.Status {
		case controller.StatusOnline:
			statusIcon = "+"
		case controller.StatusOffline:
			statusIcon = "-"
		}

		vrrpStr := ""
		if router.VRRPState != "" {
			vrrpStr = fmt.Sprintf(" [%s]", router.VRRPState)
		}

		healthStr := ""
		if router.Healthy != nil {
			if *router.Healthy {
				healthStr = " (healthy)"
			} else {
				healthStr = " (unhealthy)"
			}
		}

		fmt.Printf("  [%s] %s (%s) - %s:%d%s%s\n",
			statusIcon, router.Name, router.Role, router.Host, router.Port, vrrpStr, healthStr)
	}
}
