#!/bin/sh
# router.sh - Standalone installation script for gateway-agent
# Usage: curl -sSL https://example.com/router.sh | sh -s -- [options]

set -e

AGENT_URL="${AGENT_URL:-}"
CONFIG_URL="${CONFIG_URL:-}"
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc/gateway-agent"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo "${GREEN}[+]${NC} $1"; }
warn() { echo "${YELLOW}[!]${NC} $1"; }
error() { echo "${RED}[-]${NC} $1" >&2; exit 1; }

# Detect platform
detect_platform() {
    if [ -f /etc/openwrt_release ]; then
        PLATFORM="openwrt"
        INIT_SYSTEM="procd"
    elif command -v systemctl >/dev/null 2>&1; then
        PLATFORM="linux"
        INIT_SYSTEM="systemd"
    else
        error "Unsupported platform"
    fi
    
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        armv7l|armv6l) GOARCH="arm" ;;
        mips) GOARCH="mips" ;;
        mipsel) GOARCH="mipsel" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac
    
    log "Detected: $PLATFORM ($ARCH)"
}

# Install keepalived
install_keepalived() {
    if command -v keepalived >/dev/null 2>&1; then
        log "keepalived already installed"
        return
    fi
    
    log "Installing keepalived..."
    case "$PLATFORM" in
        openwrt)
            opkg update
            opkg install keepalived
            ;;
        linux)
            if command -v apt-get >/dev/null 2>&1; then
                apt-get update -qq
                apt-get install -y -qq keepalived
            elif command -v yum >/dev/null 2>&1; then
                yum install -y -q keepalived
            elif command -v apk >/dev/null 2>&1; then
                apk add keepalived
            else
                error "Cannot install keepalived: no supported package manager found"
            fi
            ;;
    esac
}

# Download agent binary
download_agent() {
    if [ -z "$AGENT_URL" ]; then
        error "AGENT_URL not set. Provide the URL to gateway-agent binary."
    fi
    
    log "Downloading gateway-agent..."
    mkdir -p "$INSTALL_DIR"
    
    if command -v curl >/dev/null 2>&1; then
        curl -sSL "$AGENT_URL" -o "$INSTALL_DIR/gateway-agent"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$AGENT_URL" -O "$INSTALL_DIR/gateway-agent"
    else
        error "Neither curl nor wget available"
    fi
    
    chmod +x "$INSTALL_DIR/gateway-agent"
    log "Agent installed to $INSTALL_DIR/gateway-agent"
}

# Download or create config
setup_config() {
    mkdir -p "$CONFIG_DIR"
    
    if [ -n "$CONFIG_URL" ]; then
        log "Downloading config..."
        if command -v curl >/dev/null 2>&1; then
            curl -sSL "$CONFIG_URL" -o "$CONFIG_FILE"
        else
            wget -q "$CONFIG_URL" -O "$CONFIG_FILE"
        fi
    elif [ ! -f "$CONFIG_FILE" ]; then
        warn "No config file found at $CONFIG_FILE"
        warn "Create config manually or use gateway-controller to push config"
        
        # Create minimal template
        cat > "$CONFIG_FILE" << 'EOF'
version: 1
# role: primary or secondary
role: secondary

lan:
  iface: eth0
  # cidr: auto-detected if empty
  vip: ""  # REQUIRED: Virtual IP address

routers:
  # self_ip: auto-detected if empty  
  peer_ip: ""  # REQUIRED: IP of the other router

keepalived:
  vrid: 51
  advert_int: 1
  priority:
    primary: 100
    secondary: 150

health:
  mode: internet
  interval_sec: 2
  fail_count: 3
  recover_count: 5
EOF
        warn "Edit $CONFIG_FILE before starting the agent"
    fi
}

# Setup procd service (OpenWrt)
setup_procd_service() {
    cat > /etc/init.d/gateway-agent << 'EOF'
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
EOF
    chmod +x /etc/init.d/gateway-agent
    /etc/init.d/gateway-agent enable
    log "procd service created"
}

# Setup systemd service
setup_systemd_service() {
    cat > /etc/systemd/system/gateway-agent.service << 'EOF'
[Unit]
Description=Gateway Agent
After=network.target keepalived.service
Wants=keepalived.service

[Service]
Type=simple
ExecStart=/usr/bin/gateway-agent run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable gateway-agent
    log "systemd service created"
}

# Apply configuration
apply_config() {
    if [ -f "$CONFIG_FILE" ]; then
        log "Applying keepalived configuration..."
        gateway-agent apply -c "$CONFIG_FILE" || warn "Apply failed - check config"
    fi
}

# Start services
start_services() {
    log "Starting services..."
    case "$INIT_SYSTEM" in
        procd)
            /etc/init.d/keepalived enable
            /etc/init.d/keepalived start
            ;;
        systemd)
            systemctl enable keepalived
            systemctl start keepalived
            ;;
    esac
    
    if [ -f "$CONFIG_FILE" ] && grep -q "vip:" "$CONFIG_FILE" | grep -v '""'; then
        case "$INIT_SYSTEM" in
            procd)
                /etc/init.d/gateway-agent start
                ;;
            systemd)
                systemctl start gateway-agent
                ;;
        esac
        log "Services started"
    else
        warn "Config incomplete - services not started"
        warn "Run 'gateway-agent doctor' after configuring"
    fi
}

# Uninstall
uninstall() {
    log "Uninstalling gateway-agent..."
    
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
    
    rm -f "$INSTALL_DIR/gateway-agent"
    rm -rf "$CONFIG_DIR"
    
    log "Uninstall complete"
}

# Main
main() {
    ACTION="install"
    
    while [ $# -gt 0 ]; do
        case "$1" in
            --uninstall) ACTION="uninstall" ;;
            --agent-url=*) AGENT_URL="${1#*=}" ;;
            --config-url=*) CONFIG_URL="${1#*=}" ;;
            --help|-h)
                echo "Usage: $0 [options]"
                echo ""
                echo "Options:"
                echo "  --agent-url=URL     URL to download gateway-agent binary"
                echo "  --config-url=URL    URL to download config.yaml"
                echo "  --uninstall         Remove gateway-agent"
                echo ""
                echo "Environment variables:"
                echo "  AGENT_URL           Same as --agent-url"
                echo "  CONFIG_URL          Same as --config-url"
                exit 0
                ;;
            *) warn "Unknown option: $1" ;;
        esac
        shift
    done
    
    detect_platform
    
    if [ "$ACTION" = "uninstall" ]; then
        uninstall
        exit 0
    fi
    
    install_keepalived
    download_agent
    setup_config
    
    case "$INIT_SYSTEM" in
        procd) setup_procd_service ;;
        systemd) setup_systemd_service ;;
    esac
    
    apply_config
    start_services
    
    log "Installation complete!"
    log "Run 'gateway-agent status' to check status"
    log "Run 'gateway-agent doctor' to verify configuration"
}

main "$@"
