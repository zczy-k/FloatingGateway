#!/bin/bash
# controller.sh - Gateway Controller 安装/卸载脚本
# 支持 Linux / macOS / OpenWrt
#
# 用法:
#   安装:   sh controller.sh install
#   卸载:   sh controller.sh uninstall
#   启动:   sh controller.sh start
#   停止:   sh controller.sh stop
#   状态:   sh controller.sh status

set -e

# ============== 配置 ==============
CONTROLLER_NAME="gateway-controller"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="${HOME}/.gateway-controller"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
SYSTEMD_UNIT="/etc/systemd/system/gateway-controller.service"
LAUNCHD_PLIST="${HOME}/Library/LaunchAgents/com.floatip.gateway-controller.plist"

# 下载源（需要替换为实际 URL）
DOWNLOAD_BASE="${DOWNLOAD_BASE:-https://github.com/zczy-k/FloatingGateway/releases/latest/download}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()   { printf "${GREEN}[+]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[!]${NC} %s\n" "$1"; }
error() { printf "${RED}[-]${NC} %s\n" "$1" >&2; exit 1; }
info()  { printf "${BLUE}[*]${NC} %s\n" "$1"; }

# ============== 平台检测 ==============
PLATFORM=""
ARCH=""
GOARCH=""
INIT_SYSTEM=""

detect_platform() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$os" in
        linux)
            if [ -f /etc/openwrt_release ]; then
                PLATFORM="openwrt"
                INIT_SYSTEM="procd"
                INSTALL_DIR="/usr/bin"
                
                # 检查 OpenWrt 资源
                local mem_kb
                mem_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
                if [ "$mem_kb" -lt 65536 ]; then
                    warn "OpenWrt 内存较小 (${mem_kb}KB)，Controller 可能运行不稳定"
                    warn "建议仅使用 CLI 工具或在其他设备运行 Controller"
                fi
            else
                PLATFORM="linux"
                if command -v systemctl >/dev/null 2>&1; then
                    INIT_SYSTEM="systemd"
                else
                    INIT_SYSTEM="none"
                fi
            fi
            ;;
        darwin)
            PLATFORM="darwin"
            INIT_SYSTEM="launchd"
            INSTALL_DIR="/usr/local/bin"
            ;;
        *)
            error "不支持的操作系统: $os"
            ;;
    esac
    
    case "$ARCH" in
        x86_64|amd64) GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        armv7l|armv6l) GOARCH="arm" ;;
        mips) GOARCH="mips" ;;
        mipsel|mipsle) GOARCH="mipsle" ;;
        *) error "不支持的架构: $ARCH" ;;
    esac
    
    log "平台: $PLATFORM ($ARCH)"
}

# ============== 下载 ==============
download_controller() {
    local binary_name="${CONTROLLER_NAME}-${PLATFORM}-${GOARCH}"
    local url="${DOWNLOAD_BASE}/${binary_name}"
    local target="${INSTALL_DIR}/${CONTROLLER_NAME}"
    
    log "下载 $binary_name..."
    
    # 创建安装目录
    if [ ! -d "$INSTALL_DIR" ]; then
        sudo mkdir -p "$INSTALL_DIR" 2>/dev/null || mkdir -p "$INSTALL_DIR"
    fi
    
    # 下载
    if command -v curl >/dev/null 2>&1; then
        sudo curl -sSL "$url" -o "$target" 2>/dev/null || curl -sSL "$url" -o "$target"
    elif command -v wget >/dev/null 2>&1; then
        sudo wget -q "$url" -O "$target" 2>/dev/null || wget -q "$url" -O "$target"
    else
        error "需要 curl 或 wget"
    fi
    
    sudo chmod +x "$target" 2>/dev/null || chmod +x "$target"
    log "已安装到 $target"
}

# ============== 配置文件 ==============
create_default_config() {
    mkdir -p "$CONFIG_DIR"
    
    if [ -f "$CONFIG_FILE" ]; then
        log "配置文件已存在: $CONFIG_FILE"
        return
    fi
    
    cat > "$CONFIG_FILE" << 'EOF'
# Gateway Controller 配置文件
version: 1

# HTTP 服务监听地址
listen: ":8080"

# Agent 二进制路径（用于远程部署）
# 留空则自动从 dist/ 目录查找
agent_bin: ""

# 共享 LAN 配置
lan:
  # 虚拟网关 IP (必填)
  vip: ""
  # 网段 CIDR
  cidr: ""

keepalived:
  # VRRP ID (1-255)
  vrid: 51

# 管理的路由器列表
routers: []
  # - name: openwrt-main
  #   host: 192.168.1.2
  #   port: 22
  #   user: root
  #   password: ""
  #   key_file: ~/.ssh/id_rsa
  #   role: primary
  #
  # - name: ubuntu-gateway
  #   host: 192.168.1.3
  #   port: 22
  #   user: root
  #   password: ""
  #   key_file: ~/.ssh/id_rsa
  #   role: secondary
EOF
    
    chmod 600 "$CONFIG_FILE"
    log "已创建默认配置: $CONFIG_FILE"
    info "请编辑配置文件后启动 Controller"
}

# ============== systemd 服务 (Linux) ==============
setup_systemd_service() {
    [ "$INIT_SYSTEM" != "systemd" ] && return
    
    log "创建 systemd 服务..."
    
    sudo tee "$SYSTEMD_UNIT" > /dev/null << EOF
[Unit]
Description=Gateway Controller - Floating Gateway Management
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=${INSTALL_DIR}/${CONTROLLER_NAME} serve -c ${CONFIG_FILE}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    
    sudo systemctl daemon-reload
    log "systemd 服务已创建"
    info "启动: sudo systemctl start gateway-controller"
    info "开机自启: sudo systemctl enable gateway-controller"
}

remove_systemd_service() {
    [ "$INIT_SYSTEM" != "systemd" ] && return
    [ ! -f "$SYSTEMD_UNIT" ] && return
    
    log "移除 systemd 服务..."
    sudo systemctl stop gateway-controller 2>/dev/null || true
    sudo systemctl disable gateway-controller 2>/dev/null || true
    sudo rm -f "$SYSTEMD_UNIT"
    sudo systemctl daemon-reload
}

# ============== launchd 服务 (macOS) ==============
setup_launchd_service() {
    [ "$INIT_SYSTEM" != "launchd" ] && return
    
    log "创建 launchd 服务..."
    
    mkdir -p "$(dirname "$LAUNCHD_PLIST")"
    
    cat > "$LAUNCHD_PLIST" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.floatip.gateway-controller</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${CONTROLLER_NAME}</string>
        <string>serve</string>
        <string>-c</string>
        <string>${CONFIG_FILE}</string>
    </array>
    <key>RunAtLoad</key>
    <false/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>${CONFIG_DIR}/controller.log</string>
    <key>StandardErrorPath</key>
    <string>${CONFIG_DIR}/controller.err</string>
</dict>
</plist>
EOF
    
    log "launchd 服务已创建"
    info "启动: launchctl load $LAUNCHD_PLIST"
    info "停止: launchctl unload $LAUNCHD_PLIST"
}

remove_launchd_service() {
    [ "$INIT_SYSTEM" != "launchd" ] && return
    [ ! -f "$LAUNCHD_PLIST" ] && return
    
    log "移除 launchd 服务..."
    launchctl unload "$LAUNCHD_PLIST" 2>/dev/null || true
    rm -f "$LAUNCHD_PLIST"
}

# ============== procd 服务 (OpenWrt) ==============
setup_procd_service() {
    [ "$INIT_SYSTEM" != "procd" ] && return
    
    log "创建 procd 服务..."
    
    cat > /etc/init.d/gateway-controller << 'EOF'
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/gateway-controller serve
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}
EOF
    
    chmod +x /etc/init.d/gateway-controller
    log "procd 服务已创建"
    info "启动: /etc/init.d/gateway-controller start"
    info "开机自启: /etc/init.d/gateway-controller enable"
}

remove_procd_service() {
    [ "$INIT_SYSTEM" != "procd" ] && return
    [ ! -f /etc/init.d/gateway-controller ] && return
    
    log "移除 procd 服务..."
    /etc/init.d/gateway-controller stop 2>/dev/null || true
    /etc/init.d/gateway-controller disable 2>/dev/null || true
    rm -f /etc/init.d/gateway-controller
}

# ============== 安装 ==============
do_install() {
    detect_platform
    download_controller
    create_default_config
    
    case "$INIT_SYSTEM" in
        systemd) setup_systemd_service ;;
        launchd) setup_launchd_service ;;
        procd) setup_procd_service ;;
    esac
    
    echo ""
    log "安装完成!"
    echo ""
    info "下一步:"
    info "  1. 编辑配置文件: $CONFIG_FILE"
    info "  2. 启动 Controller: $CONTROLLER_NAME serve -c $CONFIG_FILE"
    info "  3. 打开浏览器访问: http://localhost:8080"
}

# ============== 卸载 ==============
do_uninstall() {
    detect_platform
    
    log "开始卸载 Gateway Controller..."
    
    # 停止并移除服务
    case "$INIT_SYSTEM" in
        systemd) remove_systemd_service ;;
        launchd) remove_launchd_service ;;
        procd) remove_procd_service ;;
    esac
    
    # 删除二进制
    local binary="${INSTALL_DIR}/${CONTROLLER_NAME}"
    if [ -f "$binary" ]; then
        sudo rm -f "$binary" 2>/dev/null || rm -f "$binary"
        log "已删除: $binary"
    fi
    
    # 询问是否删除配置
    if [ -d "$CONFIG_DIR" ]; then
        printf "是否删除配置目录 $CONFIG_DIR? [y/N]: "
        read -r confirm
        case "$confirm" in
            [Yy]*)
                rm -rf "$CONFIG_DIR"
                log "已删除配置目录"
                ;;
            *)
                info "保留配置目录: $CONFIG_DIR"
                ;;
        esac
    fi
    
    log "卸载完成"
}

# ============== 启动/停止/状态 ==============
do_start() {
    detect_platform
    
    case "$INIT_SYSTEM" in
        systemd)
            sudo systemctl start gateway-controller
            ;;
        launchd)
            launchctl load "$LAUNCHD_PLIST"
            ;;
        procd)
            /etc/init.d/gateway-controller start
            ;;
        *)
            # 直接运行
            "${INSTALL_DIR}/${CONTROLLER_NAME}" serve -c "$CONFIG_FILE" &
            echo $! > "${CONFIG_DIR}/controller.pid"
            ;;
    esac
    
    log "Controller 已启动"
}

do_stop() {
    detect_platform
    
    case "$INIT_SYSTEM" in
        systemd)
            sudo systemctl stop gateway-controller
            ;;
        launchd)
            launchctl unload "$LAUNCHD_PLIST"
            ;;
        procd)
            /etc/init.d/gateway-controller stop
            ;;
        *)
            if [ -f "${CONFIG_DIR}/controller.pid" ]; then
                kill "$(cat "${CONFIG_DIR}/controller.pid")" 2>/dev/null || true
                rm -f "${CONFIG_DIR}/controller.pid"
            fi
            pkill -f "gateway-controller serve" 2>/dev/null || true
            ;;
    esac
    
    log "Controller 已停止"
}

do_status() {
    detect_platform
    
    local binary="${INSTALL_DIR}/${CONTROLLER_NAME}"
    
    echo "=== Gateway Controller 状态 ==="
    
    # 检查安装
    if [ -x "$binary" ]; then
        log "二进制: 已安装 ($binary)"
        "$binary" version 2>/dev/null || true
    else
        warn "二进制: 未安装"
    fi
    
    # 检查配置
    if [ -f "$CONFIG_FILE" ]; then
        log "配置: $CONFIG_FILE"
    else
        warn "配置: 未找到"
    fi
    
    # 检查运行状态
    case "$INIT_SYSTEM" in
        systemd)
            if systemctl is-active gateway-controller >/dev/null 2>&1; then
                log "服务: 运行中 (systemd)"
            else
                warn "服务: 未运行"
            fi
            ;;
        launchd)
            if launchctl list | grep -q "com.floatip.gateway-controller"; then
                log "服务: 运行中 (launchd)"
            else
                warn "服务: 未运行"
            fi
            ;;
        procd)
            if /etc/init.d/gateway-controller status 2>/dev/null | grep -q running; then
                log "服务: 运行中 (procd)"
            else
                warn "服务: 未运行"
            fi
            ;;
        *)
            if pgrep -f "gateway-controller serve" >/dev/null 2>&1; then
                log "服务: 运行中"
            else
                warn "服务: 未运行"
            fi
            ;;
    esac
}

# ============== 帮助 ==============
show_help() {
    cat << 'EOF'
Gateway Controller 安装脚本

用法: controller.sh <命令>

命令:
  install     安装 Controller
  uninstall   卸载 Controller
  start       启动服务
  stop        停止服务
  status      查看状态
  help        显示帮助

环境变量:
  DOWNLOAD_BASE   下载基础 URL (默认: GitHub Releases)

示例:
  # 安装
  sh controller.sh install

  # 自定义下载源安装
  DOWNLOAD_BASE=https://myserver.com/releases sh controller.sh install

  # 卸载
  sh controller.sh uninstall
EOF
}

# ============== 主入口 ==============
show_menu() {
    clear
    printf "${BLUE}===============================================${NC}\n"
      printf "${BLUE}      Gateway Controller 管理工具 (v1.0.5)      ${NC}\n"

    printf "${BLUE}===============================================${NC}\n"
    printf "  1) 安装 / 升级 Controller\n"
    printf "  2) 启动 Controller 服务\n"
    printf "  3) 停止 Controller 服务\n"
    printf "  4) 查看服务状态 / 诊断\n"
    printf "  5) 卸载 Controller (清理残留)\n"
    printf "  0) 退出\n"
    printf "${BLUE}-----------------------------------------------${NC}\n"
    printf "请选择数字 [0-5]: "
    read -r choice

    case "$choice" in
        1) do_install ;;
        2) do_start ;;
        3) do_stop ;;
        4) do_status ;;
        5) do_uninstall ;;
        0) exit 0 ;;
        *) warn "无效选项，请重试"; sleep 1; show_menu ;;
    esac
}

main() {
    if [ -z "$1" ]; then
        show_menu
        return
    fi

    case "$1" in
        install)
            do_install
            ;;
        uninstall)
            do_uninstall
            ;;
        start)
            do_start
            ;;
        stop)
            do_stop
            ;;
        status)
            do_status
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            error "未知命令: $1\n运行 'controller.sh help' 查看帮助"
            ;;
    esac
}

main "$@"
