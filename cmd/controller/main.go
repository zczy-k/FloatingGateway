// gateway-controller is the central management tool for the floating gateway system.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/controller"
	"os/exec"
	"runtime"
	"time"
)

const (
	defaultConfigPath = "controller.yaml"
	version           = "1.0.5"
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
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gateway-controller - 浮动网关控制台

用法:
  gateway-controller <命令> [选项]

命令:
  serve       启动 Web 界面和 API 服务 (自动打开浏览器)
  probe       探测所有路由器状态
  install     在路由器上安装 Agent
  uninstall   从路由器上卸载 Agent
  status      显示总体状态
  version     打印版本信息

选项:
  -c, --config   配置文件路径 (默认: controller.yaml)

示例:
  gateway-controller serve -c controller.yaml
  gateway-controller probe
  gateway-controller install --router=openwrt-main
  gateway-controller status`)
}

func loadManager(configPath string) (*controller.Manager, error) {
	manager, err := controller.NewManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	return manager, nil
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	
	// Try to find a real LAN IP first
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// Prefer common private IP ranges
				if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
					return ip
				}
			}
		}
	}
	
	// Fallback to first non-loopback IPv4
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	
	return "localhost"
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
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// If config file doesn't exist, it will be created on first save
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Printf("配置文件 %s 未找到。将以设置模式启动 (默认监听: :8080)\n", *configPath)
	}

	// Determine listen address
	addr := manager.GetConfig().Listen
	if *listen != "" {
		addr = *listen
	}
	if addr == "" {
		addr = ":8080"
	}

	fmt.Printf("正在启动浮动网关控制台，监听: %s\n", addr)

	// Determine the URL to open
	host := "localhost"
	port := addr
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if parts[0] != "" && parts[0] != "0.0.0.0" {
			host = parts[0]
		} else {
			// Listen on all interfaces, try to find local IP
			host = getLocalIP()
		}
		port = ":" + parts[1]
	}
	url := fmt.Sprintf("http://%s%s", host, port)
	fmt.Printf("请在浏览器中打开: %s\n", url)

	// Open browser in background if requested
	if !*noBrowser {
		go func() {
			time.Sleep(1 * time.Second) // Wait a bit for server to start
			openBrowser(url)
		}()
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	server := controller.NewServer(manager)

	go func() {
		<-sigCh
		fmt.Println("\n正在关闭...")
		server.Stop()
	}()

	if err := server.Start(addr); err != nil {
		fmt.Fprintf(os.Stderr, "服务器错误: %v\n", err)
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
		err = fmt.Errorf("不支持的平台")
	}
	if err != nil {
		fmt.Printf("无法自动打开浏览器: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "未找到路由器 %q\n", *routerName)
			os.Exit(1)
		}
		fmt.Printf("正在探测 %s...\n", router.Name)
		if err := manager.Probe(router); err != nil {
			fmt.Printf("  状态: 离线 (%v)\n", err)
		} else {
			fmt.Printf("  状态: %s\n", translateStatus(router.Status))
			fmt.Printf("  平台: %s\n", router.Platform)
			fmt.Printf("  Agent 版本: %s\n", router.AgentVer)
			if router.VRRPState != "" {
				fmt.Printf("  VRRP 状态: %s\n", router.VRRPState)
			}
		}
	} else {
		fmt.Println("正在探测所有路由器...")
		manager.ProbeAll()
		for _, router := range manager.GetRouters() {
			fmt.Printf("\n%s (%s):\n", router.Name, translateRole(router.Role))
			fmt.Printf("  主机: %s:%d\n", router.Host, router.Port)
			fmt.Printf("  状态: %s\n", translateStatus(router.Status))
			if router.Status == controller.StatusOnline {
				fmt.Printf("  平台: %s\n", router.Platform)
				if router.AgentVer != "" {
					fmt.Printf("  Agent 版本: %s\n", router.AgentVer)
					if router.VRRPState != "" {
						fmt.Printf("  VRRP 状态: %s\n", router.VRRPState)
					}
					if router.Healthy != nil {
						healthStr := "异常"
						if *router.Healthy {
							healthStr = "健康"
						}
						fmt.Printf("  健康状态: %s\n", healthStr)
					}
				} else {
					fmt.Printf("  Agent: 未安装\n")
				}
			} else if router.Error != "" {
				fmt.Printf("  错误: %s\n", router.Error)
			}
		}
	}
}

func translateStatus(s controller.RouterStatus) string {
	switch s {
	case controller.StatusOnline:
		return "在线"
	case controller.StatusOffline:
		return "离线"
	case controller.StatusInstalling:
		return "安装中"
	case controller.StatusUninstalling:
		return "卸载中"
	case controller.StatusError:
		return "错误"
	default:
		return "未知"
	}
}

func translateRole(r config.Role) string {
	switch r {
	case config.RolePrimary:
		return "主路由 (Primary)"
	case config.RoleSecondary:
		return "旁路由 (Secondary)"
	default:
		return string(r)
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
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	var routers []*controller.Router
	if *all {
		routers = manager.GetRouters()
	} else if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "未找到路由器 %q\n", *routerName)
			os.Exit(1)
		}
		routers = []*controller.Router{router}
	} else {
		fmt.Fprintf(os.Stderr, "错误: 需要提供 --router 或 --all 参数\n")
		os.Exit(1)
	}

	for _, router := range routers {
		fmt.Printf("正在 %s 上安装 Agent...\n", router.Name)
		agentConfig := manager.GenerateAgentConfig(router)
		if err := manager.Install(router, agentConfig); err != nil {
			fmt.Fprintf(os.Stderr, "  失败: %v\n", err)
		} else {
			fmt.Printf("  成功!\n")
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
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	var routers []*controller.Router
	if *all {
		routers = manager.GetRouters()
	} else if *routerName != "" {
		router := manager.GetRouter(*routerName)
		if router == nil {
			fmt.Fprintf(os.Stderr, "未找到路由器 %q\n", *routerName)
			os.Exit(1)
		}
		routers = []*controller.Router{router}
	} else {
		fmt.Fprintf(os.Stderr, "错误: 需要提供 --router 或 --all 参数\n")
		os.Exit(1)
	}

	for _, router := range routers {
		fmt.Printf("正在从 %s 卸载 Agent...\n", router.Name)
		if err := manager.Uninstall(router); err != nil {
			fmt.Fprintf(os.Stderr, "  失败: %v\n", err)
		} else {
			fmt.Printf("  成功!\n")
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
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	cfg := manager.GetConfig()

	fmt.Println("浮动网关状态汇总")
	fmt.Println("=======================")
	fmt.Printf("VIP 地址: %s\n", cfg.LAN.VIP)
	fmt.Printf("网段 (CIDR): %s\n", cfg.LAN.CIDR)
	fmt.Printf("VRID 标识: %d\n", cfg.Keepalived.VRID)
	fmt.Println()

	// Probe all routers
	fmt.Println("正在探测路由器状态...")
	manager.ProbeAll()

	// Find master
	var master string
	for _, router := range manager.GetRouters() {
		if router.VRRPState == "MASTER" {
			master = router.Name
			break
		}
	}

	if master == "" {
		fmt.Println("当前主控 (Master): 无")
	} else {
		fmt.Printf("当前主控 (Master): %s\n", master)
	}
	fmt.Println()

	fmt.Println("路由器列表:")
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
				healthStr = " (健康)"
			} else {
				healthStr = " (异常)"
			}
		}

		fmt.Printf("  [%s] %s (%s) - %s:%d%s%s\n",
			statusIcon, router.Name, translateRole(router.Role), router.Host, router.Port, vrrpStr, healthStr)
	}
}
