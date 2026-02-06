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

# ============== 工具函数 ==============
check_port() {
    local port=8080
    if [ -f "$CONFIG_FILE" ]; then
        port=$(grep "listen:" "$CONFIG_FILE" | head -1 | awk '{print $2}' | tr -d '"' | cut -d: -f2)
        port=${port:-8080}
    fi
    if command -v ss >/dev/null 2>&1; then
        ss -tunlp | grep -q ":$port " && return 1
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tunlp 2>/dev/null | grep -q ":$port " && return 1
    elif command -v lsof >/dev/null 2>&1; then
        lsof -i :"$port" -sTCP:LISTEN -t >/dev/null 2>&1 && return 1
    fi
    return 0
}

get_local_ip() {
    local ip=""
    # 优先通过默认路由获取出口 IP
    if command -v ip >/dev/null 2>&1; then
        ip=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K\S+' || true)
    fi
    # 回退: hostname -I (Linux)
    if [ -z "$ip" ] && command -v hostname >/dev/null 2>&1; then
        ip=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
    fi
    # 回退: ifconfig
    if [ -z "$ip" ] && command -v ifconfig >/dev/null 2>&1; then
        ip=$(ifconfig 2>/dev/null | grep -E 'inet [0-9]' | grep -v '127.0.0.1' | head -1 | awk '{print $2}' | sed 's/addr://')
    fi
    # 最终回退
    echo "${ip:-localhost}"
}

# ============== 平台检测 ==============
PLATFORM=""
ARCH=""
GOARCH=""
GOOS=""
INIT_SYSTEM=""
STEP_CURRENT=0
STEP_TOTAL=0

# 带步骤号的日志输出
step() {
    STEP_CURRENT=$((STEP_CURRENT + 1))
    printf "${GREEN}[${STEP_CURRENT}/${STEP_TOTAL}]${NC} %s\n" "$1"
}

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
        mips)
            GOARCH="mips"
            # 自动检测 MIPS 字节序
            if command -v hexdump >/dev/null 2>&1; then
                local endian
                endian=$(echo -n I | hexdump -o | head -n1 | awk '{print $2}')
                if [ "$endian" = "0000049" ]; then
                    GOARCH="mipsle"
                    info "检测到 MIPS 小端序 (little-endian)"
                else
                    info "检测到 MIPS 大端序 (big-endian)"
                fi
            fi
            ;;
        mipsel|mipsle) GOARCH="mipsle" ;;
        *) error "不支持的架构: $ARCH" ;;
    esac
    
    # 下载用的 OS 名称: openwrt 也用 linux 二进制
    case "$os" in
        linux)  GOOS="linux" ;;
        darwin) GOOS="darwin" ;;
    esac
    
    log "平台: $PLATFORM ($ARCH -> ${GOOS}/${GOARCH})"
}

# ============== 版本检测 ==============
get_installed_version() {
    local binary="${INSTALL_DIR}/${CONTROLLER_NAME}"
    if [ -x "$binary" ]; then
        "$binary" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1
    fi
}

get_latest_version() {
    local api_url="https://api.github.com/repos/zczy-k/FloatingGateway/releases/latest"
    if [ -n "$GH_PROXY" ]; then
        api_url="${GH_PROXY}${api_url}"
    fi
    local tag=""
    if command -v curl >/dev/null 2>&1; then
        tag=$(curl -sSL --connect-timeout 10 "$api_url" 2>/dev/null | grep '"tag_name"' | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
    elif command -v wget >/dev/null 2>&1; then
        tag=$(wget -q --timeout=10 -O - "$api_url" 2>/dev/null | grep '"tag_name"' | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
    fi
    echo "$tag"
}

# ============== 下载 ==============
# GitHub 加速镜像列表，按优先级排列
GH_PROXIES="https://xuc.xi-xu.me https://gh-proxy.com https://ghfast.top"

# 通用下载函数：尝试加速镜像，最后用官方兜底
# 参数: $1=GitHub原始URL(必须是https://github.com/...), $2=保存路径
download_with_proxy() {
    local url="$1"
    local target="$2"
    local success=0
    
    # 先尝试加速镜像
    for proxy in $GH_PROXIES; do
        # 格式: https://gh-proxy.com/https://github.com/...
        local proxy_url="${proxy}/${url}"
        info "尝试下载: $proxy_url"
        
        # 先删除可能存在的旧文件
        rm -f "$target" 2>/dev/null
        
        if command -v curl >/dev/null 2>&1; then
            if sudo curl -fsSL --connect-timeout 15 --max-time 180 -o "$target" "$proxy_url" 2>/dev/null || \
               curl -fsSL --connect-timeout 15 --max-time 180 -o "$target" "$proxy_url" 2>/dev/null; then
                # 检查文件是否有效（非空且不是 HTML 错误页面）
                if [ -f "$target" ] && [ -s "$target" ]; then
                    # 检查是否为 HTML（错误页面）
                    if file "$target" 2>/dev/null | grep -qiE 'HTML|text'; then
                        warn "下载内容为 HTML 错误页面，跳过: $proxy"
                        rm -f "$target" 2>/dev/null
                        continue
                    fi
                    log "下载成功 (使用加速: $proxy)"
                    success=1
                    break
                fi
            fi
        elif command -v wget >/dev/null 2>&1; then
            if sudo wget -q --timeout=180 -O "$target" "$proxy_url" 2>/dev/null || \
               wget -q --timeout=180 -O "$target" "$proxy_url" 2>/dev/null; then
                if [ -f "$target" ] && [ -s "$target" ]; then
                    if file "$target" 2>/dev/null | grep -qiE 'HTML|text'; then
                        warn "下载内容为 HTML 错误页面，跳过: $proxy"
                        rm -f "$target" 2>/dev/null
                        continue
                    fi
                    log "下载成功 (使用加速: $proxy)"
                    success=1
                    break
                fi
            fi
        fi
        warn "加速镜像失败: $proxy"
    done
    
    # 加速镜像都失败，尝试直连官方
    if [ "$success" -eq 0 ]; then
        warn "所有加速镜像失败，尝试直连 GitHub..."
        info "直连: $url"
        rm -f "$target" 2>/dev/null
        
        if command -v curl >/dev/null 2>&1; then
            if sudo curl -fsSL --connect-timeout 30 --max-time 300 -o "$target" "$url" 2>/dev/null || \
               curl -fsSL --connect-timeout 30 --max-time 300 -o "$target" "$url" 2>/dev/null; then
                if [ -f "$target" ] && [ -s "$target" ]; then
                    if ! file "$target" 2>/dev/null | grep -qiE 'HTML|text'; then
                        log "下载成功 (直连 GitHub)"
                        success=1
                    fi
                fi
            fi
        elif command -v wget >/dev/null 2>&1; then
            if sudo wget -q --timeout=300 -O "$target" "$url" 2>/dev/null || \
               wget -q --timeout=300 -O "$target" "$url" 2>/dev/null; then
                if [ -f "$target" ] && [ -s "$target" ]; then
                    if ! file "$target" 2>/dev/null | grep -qiE 'HTML|text'; then
                        log "下载成功 (直连 GitHub)"
                        success=1
                    fi
                fi
            fi
        fi
    fi
    
    return $((1 - success))
}

download_controller() {
    local binary_name="${CONTROLLER_NAME}-${GOOS}-${GOARCH}"
    # 始终使用原始 GitHub URL，让 download_with_proxy 处理代理
    local github_url="https://github.com/zczy-k/FloatingGateway/releases/latest/download/${binary_name}"
    local target="${INSTALL_DIR}/${CONTROLLER_NAME}"
    
    log "下载 $binary_name..."
    
    # 创建安装目录
    if [ ! -d "$INSTALL_DIR" ]; then
        sudo mkdir -p "$INSTALL_DIR" 2>/dev/null || mkdir -p "$INSTALL_DIR"
    fi
    
    # 使用加速下载（传入原始 GitHub URL）
    if ! download_with_proxy "$github_url" "$target"; then
        error "下载失败: 所有下载源均不可用"
    fi
    
    sudo chmod +x "$target" 2>/dev/null || chmod +x "$target"
    
    # 验证二进制文件是否可执行 (防止 Exec format error)
    if ! "$target" version >/dev/null 2>&1; then
        local file_info=""
        if command -v file >/dev/null 2>&1; then
            file_info=$(file "$target")
        fi
        sudo rm -f "$target" 2>/dev/null || rm -f "$target"
        error "二进制文件验证失败 (Exec format error)!\n  当前系统: $(uname -s) $(uname -m)\n  下载目标: ${GOOS}/${GOARCH}\n  文件信息: ${file_info}\n  请确认下载源中有对应平台的二进制文件"
    fi
    
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
    info "启动后可在 Web 界面中进行配置"
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
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${CONTROLLER_NAME} serve -c ${CONFIG_FILE} --no-browser
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
    STEP_CURRENT=0
    STEP_TOTAL=5
    echo ""
    printf "${BLUE}========== 安装 Gateway Controller ==========${NC}\n"
    echo ""

    step "检测系统平台与架构..."
    detect_platform

    # 版本检查：已安装时比较版本
    step "检查已安装版本..."
    local installed_ver
    installed_ver=$(get_installed_version)
    if [ -n "$installed_ver" ]; then
        info "当前已安装版本: v${installed_ver}"
        info "正在检查最新版本..."
        local latest_ver
        latest_ver=$(get_latest_version)
        if [ -n "$latest_ver" ]; then
            if [ "$installed_ver" = "$latest_ver" ]; then
                log "已是最新版本 (v${latest_ver})，无需更新"
                return 0
            else
                info "发现新版本: v${latest_ver}，开始升级..."
            fi
        else
            warn "无法获取最新版本信息，继续安装/覆盖当前版本"
        fi
    else
        info "未检测到已安装版本，执行全新安装"
    fi

    step "下载并验证 Controller 二进制文件..."
    download_controller

    step "生成默认配置文件..."
    create_default_config
    
    step "配置系统服务..."
    case "$INIT_SYSTEM" in
        systemd) setup_systemd_service ;;
        launchd) setup_launchd_service ;;
        procd) setup_procd_service ;;
        *) warn "未识别的 init 系统，跳过服务配置" ;;
    esac
    
    echo ""
    printf "${GREEN}============================================${NC}\n"
    log "安装完成! (${STEP_TOTAL}/${STEP_TOTAL} 步骤)"
    printf "${GREEN}============================================${NC}\n"
    echo ""
    info "下一步: 在菜单中选择 \"启动 Controller 服务\" 即可"
    info "Agent 二进制文件将在安装到目标设备时按需下载"
}

# ============== 卸载 ==============
do_uninstall() {
    STEP_CURRENT=0
    STEP_TOTAL=4
    echo ""
    printf "${BLUE}========== 卸载 Gateway Controller ==========${NC}\n"
    echo ""

    step "检测系统平台..."
    detect_platform
    
    # 停止并移除服务
    step "停止并移除系统服务..."
    case "$INIT_SYSTEM" in
        systemd) remove_systemd_service ;;
        launchd) remove_launchd_service ;;
        procd) remove_procd_service ;;
    esac
    
    # 删除二进制
    step "删除程序文件..."
    local binary="${INSTALL_DIR}/${CONTROLLER_NAME}"
    if [ -f "$binary" ]; then
        sudo rm -f "$binary" 2>/dev/null || rm -f "$binary"
        log "已删除: $binary"
    else
        info "程序文件不存在，跳过"
    fi
    
    # 询问是否删除配置
    step "清理配置文件..."
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
    else
        info "配置目录不存在，跳过"
    fi
    
    echo ""
    printf "${GREEN}============================================${NC}\n"
    log "卸载完成! (${STEP_TOTAL}/${STEP_TOTAL} 步骤)"
    printf "${GREEN}============================================${NC}\n"
}

# ============== 启动/停止/状态 ==============
do_start() {
    STEP_CURRENT=0
    STEP_TOTAL=3
    echo ""

    step "检测系统平台..."
    detect_platform
    
    # 检查二进制是否存在
    local binary="${INSTALL_DIR}/${CONTROLLER_NAME}"
    if [ ! -x "$binary" ]; then
        error "Controller 未安装，请先选择 \"安装 / 升级\" 选项"
    fi

    step "检查端口占用..."
    # 检查端口冲突
    if ! check_port; then
        warn "端口已被占用，请检查是否有其他服务（或旧的 Controller 实例）正在运行"
    fi

    step "启动服务..."

    case "$INIT_SYSTEM" in
        systemd)
            sudo systemctl start gateway-controller
            sleep 1
            if ! systemctl is-active gateway-controller >/dev/null 2>&1; then
                error "服务启动失败。可能原因：配置错误、端口占用。请运行 'journalctl -u gateway-controller' 查看详细日志"
            fi
            ;;
        launchd)
            launchctl load "$LAUNCHD_PLIST"
            ;;
        procd)
            /etc/init.d/gateway-controller start
            sleep 1
            if ! /etc/init.d/gateway-controller status 2>/dev/null | grep -q running; then
                error "服务启动失败。请查看 /var/log/messages 获取日志"
            fi
            ;;
        *)
            # 直接运行
            if pgrep -f "gateway-controller serve" >/dev/null 2>&1; then
                warn "Controller 已经在运行中"
                return
            fi
            "${INSTALL_DIR}/${CONTROLLER_NAME}" serve -c "$CONFIG_FILE" --no-browser > "${CONFIG_DIR}/controller.log" 2>&1 &
            echo $! > "${CONFIG_DIR}/controller.pid"
            sleep 1
            if ! kill -0 $(cat "${CONFIG_DIR}/controller.pid") 2>/dev/null; then
                error "服务启动失败。请查看 ${CONFIG_DIR}/controller.log 获取日志"
            fi
            ;;
    esac
    
    local ip
    ip=$(get_local_ip)
    log "Controller 已成功启动"
    info "打开浏览器访问: http://${ip}:8080"
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
                warn "服务: 未运行 (systemd)"
                if ! check_port; then
                    info "诊断: 检测到端口冲突，可能有其他服务占用了端口"
                fi
                info "诊断: 运行 'journalctl -u gateway-controller -n 20' 查看最近日志"
            fi
            ;;
        launchd)
            if launchctl list | grep -q "com.floatip.gateway-controller"; then
                log "服务: 运行中 (launchd)"
            else
                warn "服务: 未运行 (launchd)"
            fi
            ;;
        procd)
            if /etc/init.d/gateway-controller status 2>/dev/null | grep -q running; then
                log "服务: 运行中 (procd)"
            else
                warn "服务: 未运行 (procd)"
                info "诊断: 运行 'logread -e gateway-controller' 查看日志"
            fi
            ;;
        *)
            if pgrep -f "gateway-controller serve" >/dev/null 2>&1; then
                log "服务: 运行中"
            else
                warn "服务: 未运行"
                if [ -f "${CONFIG_DIR}/controller.log" ]; then
                    info "诊断: 最近日志内容 (${CONFIG_DIR}/controller.log):"
                    tail -n 5 "${CONFIG_DIR}/controller.log"
                fi
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
    while true; do
        echo ""
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
            1) do_install || warn "操作未成功完成" ;;
            2) do_start || warn "操作未成功完成" ;;
            3) do_stop || warn "操作未成功完成" ;;
            4) do_status || warn "操作未成功完成" ;;
            5) do_uninstall || warn "操作未成功完成" ;;
            0) echo "再见!"; exit 0 ;;
            *) warn "无效选项，请重试" ;;
        esac

        echo ""
        printf "${YELLOW}按 Enter 返回菜单...${NC}"
        read -r _
    done
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
