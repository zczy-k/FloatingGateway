#!/bin/sh
# router.sh - 路由器端 gateway-agent 安装/卸载脚本
# 支持 OpenWrt (procd) 和 Linux (systemd)
# 支持交互模式和非交互模式（用于 controller 远程部署）
#
# 用法:
#   交互模式:  sh router.sh
#   非交互:    sh router.sh --action=install --role=secondary --iface=eth0 --vip=192.168.1.1 --peer-ip=192.168.1.2
#   卸载:      sh router.sh --action=uninstall

set -e

# ============== 配置 ==============
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc/gateway-agent"
CONFIG_FILE="$CONFIG_DIR/config.yaml"
BACKUP_DIR="$CONFIG_DIR/backup"
KEEPALIVED_CONF=""  # 自动探测

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 日志函数
log()   { printf "${GREEN}[+]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[!]${NC} %s\n" "$1"; }
error() { printf "${RED}[-]${NC} %s\n" "$1" >&2; exit 1; }
info()  { printf "${BLUE}[*]${NC} %s\n" "$1"; }

# ============== 参数（非交互模式） ==============
ACTION=""
ARG_ROLE=""
ARG_IFACE=""
ARG_CIDR=""
ARG_VIP=""
ARG_PEER_IP=""
ARG_SELF_IP=""
ARG_VRID="51"
ARG_HEALTH_MODE="internet"
ARG_OPENWRT_DHCP_GW="false"
ARG_AGENT_URL=""
ARG_AGENT_PATH=""
ARG_PURGE_DEPS="false"
INTERACTIVE="true"

# ============== 平台检测 ==============
PLATFORM=""
INIT_SYSTEM=""
ARCH=""
GOARCH=""

detect_platform() {
    if [ -f /etc/openwrt_release ]; then
        PLATFORM="openwrt"
        INIT_SYSTEM="procd"
    elif command -v systemctl >/dev/null 2>&1; then
        PLATFORM="linux"
        INIT_SYSTEM="systemd"
    elif [ -f /etc/init.d/rcS ]; then
        PLATFORM="linux"
        INIT_SYSTEM="sysvinit"
    else
        error "不支持的平台"
    fi
    
    ARCH=$(uname -m)
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
    
    # 探测 keepalived 配置路径
    for path in /etc/keepalived/keepalived.conf /etc/keepalived.conf; do
        if [ -d "$(dirname "$path")" ] || [ -f "$path" ]; then
            KEEPALIVED_CONF="$path"
            break
        fi
    done
    [ -z "$KEEPALIVED_CONF" ] && KEEPALIVED_CONF="/etc/keepalived/keepalived.conf"
    
    log "平台: $PLATFORM ($ARCH), 初始化系统: $INIT_SYSTEM"
}

# ============== 网络工具函数 ==============

# 获取接口列表（优先返回默认路由接口）
list_interfaces() {
    local def_iface
    def_iface=$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++)if($i=="dev")print $(i+1)}' | head -1)
    [ -n "$def_iface" ] && echo "$def_iface"
    ip -o link show | awk -F': ' '{print $2}' | grep -v -E '^(lo|docker|veth|br-|virbr)' | sed 's/@.*//' | grep -v "^$def_iface$"
}

# 获取接口 IPv4 地址
get_iface_ipv4() {
    ip -4 addr show "$1" 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -1
}

# 获取接口 CIDR
get_iface_cidr() {
    ip -4 addr show "$1" 2>/dev/null | awk '/inet /{print $2}' | head -1
}

# 从 CIDR 提取网段
cidr_to_network() {
    echo "$1" | cut -d'/' -f1 | awk -F. '{printf "%s.%s.%s.0/%s", $1, $2, $3, "24"}'
}

# 建议 VIP（默认 .254，冲突则 .253, .252）
suggest_vip() {
    local cidr="$1"
    local base
    base=$(echo "$cidr" | cut -d'/' -f1 | awk -F. '{printf "%s.%s.%s", $1, $2, $3}')
    
    for suffix in 254 253 252 251; do
        local candidate="${base}.${suffix}"
        # 检查是否被占用（快速 ping）
        if ! ping -c 1 -W 1 "$candidate" >/dev/null 2>&1; then
            echo "$candidate"
            return
        fi
    done
    echo "${base}.254"
}

# 验证 IPv4 格式
validate_ipv4() {
    echo "$1" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}$'
}

# 检查 IP 是否在 CIDR 内
ip_in_cidr() {
    local ip="$1"
    local cidr="$2"
    # 简化检查：比较前三段
    local ip_prefix cidr_prefix
    ip_prefix=$(echo "$ip" | awk -F. '{printf "%s.%s.%s", $1, $2, $3}')
    cidr_prefix=$(echo "$cidr" | cut -d'/' -f1 | awk -F. '{printf "%s.%s.%s", $1, $2, $3}')
    [ "$ip_prefix" = "$cidr_prefix" ]
}

# ============== 依赖安装 ==============
install_dependencies() {
    log "安装依赖..."
    
    case "$PLATFORM" in
        openwrt)
            opkg update || warn "opkg update 失败"
            opkg install keepalived ip-full arping curl 2>/dev/null || true
            ;;
        linux)
            if command -v apt-get >/dev/null 2>&1; then
                apt-get update -qq
                apt-get install -y -qq keepalived iproute2 arping curl dnsutils 2>/dev/null || true
            elif command -v yum >/dev/null 2>&1; then
                yum install -y -q keepalived iproute arping curl bind-utils 2>/dev/null || true
            elif command -v apk >/dev/null 2>&1; then
                apk add keepalived iproute2 arping curl bind-tools 2>/dev/null || true
            else
                warn "未找到包管理器，请手动安装 keepalived"
            fi
            ;;
    esac
}

# ============== 版本检测 ==============
get_installed_agent_version() {
    if [ -x "$INSTALL_DIR/gateway-agent" ]; then
        "$INSTALL_DIR/gateway-agent" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1
    fi
}

get_latest_agent_version() {
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

# 检查是否需要更新，返回 0 表示已是最新不需要更新
check_agent_up_to_date() {
    local installed_ver
    installed_ver=$(get_installed_agent_version)
    if [ -z "$installed_ver" ]; then
        return 1  # 未安装，需要安装
    fi

    info "当前已安装版本: v${installed_ver}"
    info "正在检查最新版本..."
    local latest_ver
    latest_ver=$(get_latest_agent_version)
    if [ -n "$latest_ver" ]; then
        if [ "$installed_ver" = "$latest_ver" ]; then
            log "已是最新版本 (v${latest_ver})，无需更新"
            return 0
        else
            info "发现新版本: v${latest_ver}，开始升级..."
            return 1
        fi
    else
        warn "无法获取最新版本信息，继续安装/覆盖当前版本"
        return 1
    fi
}

# ============== Agent 安装 ==============
install_agent() {
    log "安装 gateway-agent..."
    
    mkdir -p "$INSTALL_DIR"
    
    if [ -n "$ARG_AGENT_PATH" ] && [ -f "$ARG_AGENT_PATH" ]; then
        # 从本地路径复制
        cp "$ARG_AGENT_PATH" "$INSTALL_DIR/gateway-agent"
    elif [ -n "$ARG_AGENT_URL" ]; then
        # 从 URL 下载
        if command -v curl >/dev/null 2>&1; then
            curl -sSL "$ARG_AGENT_URL" -o "$INSTALL_DIR/gateway-agent"
        elif command -v wget >/dev/null 2>&1; then
            wget -q "$ARG_AGENT_URL" -O "$INSTALL_DIR/gateway-agent"
        else
            error "需要 curl 或 wget 来下载 agent"
        fi
    else
        error "需要指定 --agent-url 或 --agent-path"
    fi
    
    chmod +x "$INSTALL_DIR/gateway-agent"
    log "Agent 已安装到 $INSTALL_DIR/gateway-agent"
}

# ============== 配置生成 ==============
generate_config() {
    log "生成配置文件..."
    
    mkdir -p "$CONFIG_DIR"
    
    # 备份已有配置
    if [ -f "$CONFIG_FILE" ]; then
        mkdir -p "$BACKUP_DIR"
        cp "$CONFIG_FILE" "$BACKUP_DIR/config.yaml.$(date +%Y%m%d%H%M%S)"
    fi
    
    cat > "$CONFIG_FILE" << EOF
# Gateway Agent 配置文件
# 生成时间: $(date)
version: 1

role: ${ARG_ROLE}

lan:
  iface: ${ARG_IFACE}
  cidr: ${ARG_CIDR}
  vip: ${ARG_VIP}

routers:
  self_ip: ${ARG_SELF_IP}
  peer_ip: ${ARG_PEER_IP}

keepalived:
  vrid: ${ARG_VRID}
  advert_int: 1
  priority:
    primary: 100
    secondary: 150

failover:
  prefer: secondary
  preempt: true
  preempt_delay_sec: 30

health:
  mode: ${ARG_HEALTH_MODE}
  interval_sec: 2
  fail_count: 3
  recover_count: 5
  hold_down_sec: 10
  
  basic:
    checks:
      - type: ping
        target: 223.5.5.5
        timeout: 3
      - type: dns
        resolver: 223.5.5.5
        domain: baidu.com
        timeout: 3

  internet:
    checks:
      - type: ping
        target: 1.1.1.1
        timeout: 3
      - type: dns
        resolver: 1.1.1.1
        domain: google.com
        timeout: 3
      - type: tcp
        target: 1.1.1.1
        port: 443
        timeout: 3

openwrt:
  dhcp:
    auto_set_gateway: ${ARG_OPENWRT_DHCP_GW}
EOF
    
    log "配置已写入 $CONFIG_FILE"
}

# ============== 服务设置 ==============
setup_procd_service() {
    log "创建 procd 服务..."
    
    cat > /etc/init.d/gateway-agent << 'INITEOF'
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/gateway-agent run
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}

reload_service() {
    stop
    start
}
INITEOF
    
    chmod +x /etc/init.d/gateway-agent
    /etc/init.d/gateway-agent enable
}

setup_systemd_service() {
    log "创建 systemd 服务..."
    
    cat > /etc/systemd/system/gateway-agent.service << 'UNITEOF'
[Unit]
Description=Gateway Agent - Floating Gateway Manager
After=network.target keepalived.service
Wants=keepalived.service

[Service]
Type=simple
ExecStart=/usr/bin/gateway-agent run
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNITEOF
    
    systemctl daemon-reload
    systemctl enable gateway-agent
}

# ============== 启动服务 ==============
start_services() {
    log "启动服务..."
    
    # 先应用 keepalived 配置
    if [ -x "$INSTALL_DIR/gateway-agent" ]; then
        "$INSTALL_DIR/gateway-agent" apply -c "$CONFIG_FILE" || warn "应用配置失败"
    fi
    
    case "$INIT_SYSTEM" in
        procd)
            /etc/init.d/keepalived enable 2>/dev/null || true
            /etc/init.d/keepalived start 2>/dev/null || /etc/init.d/keepalived restart
            /etc/init.d/gateway-agent start
            ;;
        systemd)
            systemctl enable keepalived
            systemctl start keepalived || systemctl restart keepalived
            systemctl start gateway-agent
            ;;
    esac
    
    log "服务已启动"
}

# ============== OpenWrt DHCP 网关配置 ==============
setup_openwrt_dhcp_gateway() {
    [ "$PLATFORM" != "openwrt" ] && return
    [ "$ARG_OPENWRT_DHCP_GW" != "true" ] && return
    [ "$ARG_ROLE" != "primary" ] && return
    
    log "配置 OpenWrt DHCP 网关为 VIP..."
    
    # 备份当前 DHCP 配置
    mkdir -p "$BACKUP_DIR"
    cp /etc/config/dhcp "$BACKUP_DIR/dhcp.backup.$(date +%Y%m%d%H%M%S)"
    
    # 使用 UCI 设置 DHCP 选项
    # 查找 lan 接口的 dhcp section
    local dhcp_section
    dhcp_section=$(uci show dhcp | grep "dhcp\..*\.interface='lan'" | cut -d'.' -f2 | head -1)
    
    if [ -n "$dhcp_section" ]; then
        # 获取现有 dhcp_option
        local existing_opts
        existing_opts=$(uci get "dhcp.${dhcp_section}.dhcp_option" 2>/dev/null || echo "")
        
        # 检查是否已有 option 3（网关）
        if echo "$existing_opts" | grep -q "^3,"; then
            # 替换现有网关选项
            uci del_list "dhcp.${dhcp_section}.dhcp_option=$(echo "$existing_opts" | grep "^3,")" 2>/dev/null || true
        fi
        
        # 添加新网关选项
        uci add_list "dhcp.${dhcp_section}.dhcp_option=3,${ARG_VIP}"
        uci commit dhcp
        
        # 重启 dnsmasq
        /etc/init.d/dnsmasq restart
        
        log "DHCP 网关已设置为 $ARG_VIP"
        info "备份已保存到 $BACKUP_DIR/dhcp.backup.*"
        info "如需回滚，执行: cp $BACKUP_DIR/dhcp.backup.* /etc/config/dhcp && /etc/init.d/dnsmasq restart"
    else
        warn "未找到 lan 接口的 DHCP 配置，请手动设置"
    fi
}

# ============== 卸载 ==============
do_uninstall() {
    log "开始卸载 gateway-agent..."
    
    detect_platform
    
    # 停止服务
    case "$INIT_SYSTEM" in
        procd)
            /etc/init.d/gateway-agent stop 2>/dev/null || true
            /etc/init.d/gateway-agent disable 2>/dev/null || true
            rm -f /etc/init.d/gateway-agent
            /etc/init.d/keepalived stop 2>/dev/null || true
            ;;
        systemd)
            systemctl stop gateway-agent 2>/dev/null || true
            systemctl disable gateway-agent 2>/dev/null || true
            rm -f /etc/systemd/system/gateway-agent.service
            systemctl daemon-reload
            systemctl stop keepalived 2>/dev/null || true
            ;;
    esac
    
    # 删除文件
    rm -f "$INSTALL_DIR/gateway-agent"
    
    # 备份并删除配置目录
    if [ -d "$CONFIG_DIR" ]; then
        if [ -d "$BACKUP_DIR" ]; then
            info "保留备份目录: $BACKUP_DIR"
            # 移动备份到 /tmp
            mv "$BACKUP_DIR" /tmp/gateway-agent-backup-$(date +%Y%m%d%H%M%S) 2>/dev/null || true
        fi
        rm -rf "$CONFIG_DIR"
    fi
    
    # OpenWrt DHCP 回滚提示
    if [ "$PLATFORM" = "openwrt" ]; then
        if [ -f /tmp/gateway-agent-backup-*/dhcp.backup.* ] 2>/dev/null; then
            warn "检测到 DHCP 配置备份，如需回滚网关设置:"
            warn "  cp /tmp/gateway-agent-backup-*/dhcp.backup.* /etc/config/dhcp"
            warn "  /etc/init.d/dnsmasq restart"
        fi
    fi
    
    # 可选：卸载依赖
    if [ "$ARG_PURGE_DEPS" = "true" ]; then
        warn "正在卸载 keepalived..."
        case "$PLATFORM" in
            openwrt) opkg remove keepalived 2>/dev/null || true ;;
            linux)
                if command -v apt-get >/dev/null 2>&1; then
                    apt-get remove -y keepalived 2>/dev/null || true
                elif command -v yum >/dev/null 2>&1; then
                    yum remove -y keepalived 2>/dev/null || true
                fi
                ;;
        esac
    fi
    
    log "卸载完成"
}

# ============== 状态检查 ==============
do_status() {
    detect_platform
    
    echo "=== Gateway Agent 状态 ==="
    
    # 检查 agent
    if [ -x "$INSTALL_DIR/gateway-agent" ]; then
        log "Agent: 已安装"
        "$INSTALL_DIR/gateway-agent" version 2>/dev/null || true
    else
        warn "Agent: 未安装"
    fi
    
    # 检查配置
    if [ -f "$CONFIG_FILE" ]; then
        log "配置: $CONFIG_FILE 存在"
    else
        warn "配置: $CONFIG_FILE 不存在"
    fi
    
    # 检查服务
    case "$INIT_SYSTEM" in
        procd)
            if /etc/init.d/gateway-agent status 2>/dev/null | grep -q running; then
                log "服务: 运行中"
            else
                warn "服务: 未运行"
            fi
            ;;
        systemd)
            if systemctl is-active gateway-agent >/dev/null 2>&1; then
                log "服务: 运行中"
            else
                warn "服务: 未运行"
            fi
            ;;
    esac
    
    # 运行 agent status
    if [ -x "$INSTALL_DIR/gateway-agent" ] && [ -f "$CONFIG_FILE" ]; then
        echo ""
        "$INSTALL_DIR/gateway-agent" status -c "$CONFIG_FILE" 2>/dev/null || true
    fi
}

# ============== 交互模式 ==============
interactive_install() {
    echo ""
    echo "=========================================="
    echo "    Gateway Agent 交互式安装向导"
    echo "=========================================="
    echo ""
    
    detect_platform

    # 版本检查：已安装时比较版本
    if check_agent_up_to_date; then
        return 0
    fi
    
    # 选择角色
    echo "选择角色:"
    echo "  1) primary  - 主路由 (OpenWrt, 备份网关)"
    echo "  2) secondary - 旁路由 (优先网关, 国际出口)"
    printf "请选择 [1/2]: "
    read -r role_choice
    case "$role_choice" in
        1) ARG_ROLE="primary" ;;
        2|"") ARG_ROLE="secondary" ;;
        *) ARG_ROLE="secondary" ;;
    esac
    echo ""
    
    # 选择接口
    echo "可用网络接口:"
    local idx=1
    local ifaces
    ifaces=$(list_interfaces)
    for iface in $ifaces; do
        local ip
        ip=$(get_iface_ipv4 "$iface")
        echo "  $idx) $iface $([ -n "$ip" ] && echo "($ip)")"
        idx=$((idx + 1))
    done
    printf "选择 LAN 接口 [默认第一个]: "
    read -r iface_choice
    if [ -z "$iface_choice" ]; then
        ARG_IFACE=$(echo "$ifaces" | head -1)
    else
        ARG_IFACE=$(echo "$ifaces" | sed -n "${iface_choice}p")
    fi
    [ -z "$ARG_IFACE" ] && ARG_IFACE=$(echo "$ifaces" | head -1)
    echo ""
    
    # 获取 CIDR
    ARG_CIDR=$(get_iface_cidr "$ARG_IFACE")
    if [ -z "$ARG_CIDR" ]; then
        printf "输入网段 CIDR (如 192.168.1.0/24): "
        read -r ARG_CIDR
    else
        printf "网段 CIDR [%s]: " "$ARG_CIDR"
        read -r input_cidr
        [ -n "$input_cidr" ] && ARG_CIDR="$input_cidr"
    fi
    echo ""
    
    # 获取 self_ip
    ARG_SELF_IP=$(get_iface_ipv4 "$ARG_IFACE")
    printf "本机 IP [%s]: " "$ARG_SELF_IP"
    read -r input_self_ip
    [ -n "$input_self_ip" ] && ARG_SELF_IP="$input_self_ip"
    echo ""
    
    # 建议 VIP
    local suggested_vip
    suggested_vip=$(suggest_vip "$ARG_CIDR")
    printf "虚拟网关 VIP [%s]: " "$suggested_vip"
    read -r ARG_VIP
    [ -z "$ARG_VIP" ] && ARG_VIP="$suggested_vip"
    echo ""
    
    # 输入 peer_ip
    printf "对端路由器 IP (必填): "
    read -r ARG_PEER_IP
    while [ -z "$ARG_PEER_IP" ]; do
        printf "对端 IP 不能为空，请输入: "
        read -r ARG_PEER_IP
    done
    echo ""
    
    # VRID
    printf "VRRP ID (1-255) [51]: "
    read -r input_vrid
    [ -n "$input_vrid" ] && ARG_VRID="$input_vrid"
    echo ""
    
    # Health mode (仅 secondary)
    if [ "$ARG_ROLE" = "secondary" ]; then
        echo "健康检查模式:"
        echo "  1) internet - 检测国际链路 (推荐)"
        echo "  2) basic    - 仅检测基本连通性"
        printf "选择 [1/2, 默认 1]: "
        read -r health_choice
        case "$health_choice" in
            2) ARG_HEALTH_MODE="basic" ;;
            *) ARG_HEALTH_MODE="internet" ;;
        esac
        echo ""
    else
        ARG_HEALTH_MODE="basic"
    fi
    
    # OpenWrt DHCP 网关 (仅 primary)
    if [ "$ARG_ROLE" = "primary" ] && [ "$PLATFORM" = "openwrt" ]; then
        printf "是否自动将 DHCP 默认网关设为 VIP? [Y/n]: "
        read -r dhcp_choice
        case "$dhcp_choice" in
            [Nn]*) ARG_OPENWRT_DHCP_GW="false" ;;
            *) ARG_OPENWRT_DHCP_GW="true" ;;
        esac
        echo ""
    fi
    
    # Agent 来源
    if [ ! -x "$INSTALL_DIR/gateway-agent" ]; then
        printf "gateway-agent 下载 URL (或本地路径): "
        read -r agent_src
        if [ -f "$agent_src" ]; then
            ARG_AGENT_PATH="$agent_src"
        else
            ARG_AGENT_URL="$agent_src"
        fi
    fi
    
    # 确认
    echo ""
    echo "=========================================="
    echo "  配置确认"
    echo "=========================================="
    echo "  角色:       $ARG_ROLE"
    echo "  接口:       $ARG_IFACE"
    echo "  CIDR:       $ARG_CIDR"
    echo "  本机 IP:    $ARG_SELF_IP"
    echo "  VIP:        $ARG_VIP"
    echo "  对端 IP:    $ARG_PEER_IP"
    echo "  VRID:       $ARG_VRID"
    echo "  健康模式:   $ARG_HEALTH_MODE"
    [ "$PLATFORM" = "openwrt" ] && echo "  DHCP网关:   $ARG_OPENWRT_DHCP_GW"
    echo "=========================================="
    printf "确认安装? [Y/n]: "
    read -r confirm
    case "$confirm" in
        [Nn]*) echo "已取消"; exit 0 ;;
    esac
    
    # 执行安装
    do_install
}

# ============== 安装执行 ==============
do_install() {
    log "开始安装..."
    
    # 验证参数
    [ -z "$ARG_ROLE" ] && error "缺少参数: --role"
    [ -z "$ARG_IFACE" ] && error "缺少参数: --iface"
    [ -z "$ARG_VIP" ] && error "缺少参数: --vip"
    [ -z "$ARG_PEER_IP" ] && error "缺少参数: --peer-ip"
    
    # 验证 IP 格式
    validate_ipv4 "$ARG_VIP" || error "无效的 VIP: $ARG_VIP"
    validate_ipv4 "$ARG_PEER_IP" || error "无效的 peer_ip: $ARG_PEER_IP"
    
    # 自动获取 CIDR 和 self_ip
    [ -z "$ARG_CIDR" ] && ARG_CIDR=$(get_iface_cidr "$ARG_IFACE")
    [ -z "$ARG_CIDR" ] && error "无法获取 CIDR，请手动指定 --cidr"
    
    [ -z "$ARG_SELF_IP" ] && ARG_SELF_IP=$(get_iface_ipv4 "$ARG_IFACE")
    
    # 验证 VIP 在 CIDR 内
    ip_in_cidr "$ARG_VIP" "$ARG_CIDR" || warn "VIP $ARG_VIP 可能不在 CIDR $ARG_CIDR 内"
    
    # 执行安装步骤
    install_dependencies
    
    if [ -n "$ARG_AGENT_URL" ] || [ -n "$ARG_AGENT_PATH" ]; then
        install_agent
    elif [ ! -x "$INSTALL_DIR/gateway-agent" ]; then
        error "需要指定 --agent-url 或 --agent-path"
    fi
    
    generate_config
    
    case "$INIT_SYSTEM" in
        procd) setup_procd_service ;;
        systemd) setup_systemd_service ;;
    esac
    
    setup_openwrt_dhcp_gateway
    start_services
    
    # 运行 doctor
    log "运行自检..."
    "$INSTALL_DIR/gateway-agent" doctor -c "$CONFIG_FILE" || warn "自检发现问题"
    
    echo ""
    log "安装完成!"
    log "运行 'gateway-agent status' 查看状态"
    log "运行 'gateway-agent doctor' 进行自检"
}

# ============== 参数解析 ==============
parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --action=*) ACTION="${1#*=}" ;;
            --role=*) ARG_ROLE="${1#*=}"; INTERACTIVE="false" ;;
            --iface=*) ARG_IFACE="${1#*=}"; INTERACTIVE="false" ;;
            --cidr=*) ARG_CIDR="${1#*=}" ;;
            --vip=*) ARG_VIP="${1#*=}"; INTERACTIVE="false" ;;
            --peer-ip=*) ARG_PEER_IP="${1#*=}"; INTERACTIVE="false" ;;
            --self-ip=*) ARG_SELF_IP="${1#*=}" ;;
            --vrid=*) ARG_VRID="${1#*=}" ;;
            --health-mode=*) ARG_HEALTH_MODE="${1#*=}" ;;
            --openwrt-auto-set-dhcp-gw=*) ARG_OPENWRT_DHCP_GW="${1#*=}" ;;
            --agent-url=*) ARG_AGENT_URL="${1#*=}" ;;
            --agent-path=*) ARG_AGENT_PATH="${1#*=}" ;;
            --purge-deps) ARG_PURGE_DEPS="true" ;;
            -h|--help)
                cat << 'HELPEOF'
用法: router.sh [选项]

操作:
  --action=install     安装 (默认)
  --action=uninstall   卸载
  --action=status      查看状态

安装参数 (非交互模式必须):
  --role=<primary|secondary>   角色
  --iface=<name>               LAN 接口名
  --vip=<ip>                   虚拟网关 IP
  --peer-ip=<ip>               对端路由器 IP

安装参数 (可选):
  --cidr=<cidr>                网段 (默认自动检测)
  --self-ip=<ip>               本机 IP (默认自动检测)
  --vrid=<1-255>               VRRP ID (默认 51)
  --health-mode=<internet|basic>  健康检查模式 (默认 internet)
  --openwrt-auto-set-dhcp-gw=<true|false>  自动设置 DHCP 网关 (OpenWrt)
  --agent-url=<url>            Agent 下载 URL
  --agent-path=<path>          Agent 本地路径

卸载参数:
  --purge-deps                 同时卸载 keepalived

示例:
  # 交互模式
  sh router.sh

  # 非交互安装 secondary
  sh router.sh --action=install --role=secondary --iface=eth0 \
    --vip=192.168.1.1 --peer-ip=192.168.1.2 \
    --agent-url=https://example.com/gateway-agent-linux-amd64

  # 卸载
  sh router.sh --action=uninstall
HELPEOF
                exit 0
                ;;
            *)
                warn "未知参数: $1"
                ;;
        esac
        shift
    done
}

# ============== 主入口 ==============
show_menu() {
    while true; do
        echo ""
        echo "=========================================="
        echo "    Gateway Agent 管理工具 (v1.0.5)"
        echo "=========================================="
        echo "  1) 安装 / 升级 Gateway Agent"
        echo "  2) 查看运行状态 / 诊断 (status)"
        echo "  3) 运行自检 (doctor)"
        echo "  4) 卸载 Gateway Agent (清理残留)"
        echo "  0) 退出"
        echo "------------------------------------------"
        printf "请选择 [0-4]: "
        read -r choice
        echo ""

        case "$choice" in
            1) interactive_install || warn "操作未成功完成" ;;
            2) do_status || warn "操作未成功完成" ;;
            3) 
                if [ -x "$INSTALL_DIR/gateway-agent" ]; then
                    "$INSTALL_DIR/gateway-agent" doctor -c "$CONFIG_FILE" || warn "操作未成功完成"
                else
                    warn "Agent 未安装"
                fi
                ;;
            4) do_uninstall || warn "操作未成功完成" ;;
            0) echo "再见!"; exit 0 ;;
            *) warn "无效选项，请重试" ;;
        esac

        echo ""
        printf "${YELLOW}按 Enter 返回菜单...${NC}"
        read -r _
    done
}

main() {
    if [ $# -eq 0 ] && [ -t 0 ]; then
        show_menu
        return
    fi

    parse_args "$@"
    
    detect_platform
    
    case "$ACTION" in
        uninstall)
            do_uninstall
            ;;
        status)
            do_status
            ;;
        install|"")
            do_install
            ;;
        *)
            error "未知操作: $ACTION"
            ;;
    esac
}

main "$@"
