#!/bin/bash
# setup.sh - Floating Gateway 一键管理脚本 (Linux/macOS)
# 
# 支持两种运行方式：
#   1. 本地运行:  chmod +x setup.sh && ./setup.sh
#   2. 远程运行:  bash <(curl -sSL https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/setup.sh)
#   3. 国内加速:  bash <(curl -sSL https://gh-proxy.com/https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/setup.sh)

GH_PROXY="${GH_PROXY:-}"  # 可通过环境变量预设，如 GH_PROXY=https://gh-proxy.com/

REPO_RAW_BASE="https://raw.githubusercontent.com/zczy-k/FloatingGateway/main"
REPO_RELEASE_BASE="https://github.com/zczy-k/FloatingGateway/releases/latest/download"

# 判断脚本是否从本地文件运行
SCRIPT_DIR=""
if [ -f "$0" ]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 下载工具封装
download() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -sSL --connect-timeout 10 "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --timeout=10 "$url" -O "$dest"
    else
        printf "${RED}[-]${NC} 需要 curl 或 wget\n"
        return 1
    fi
}

# 检测 GitHub 连通性，自动启用代理
detect_proxy() {
    # 如果已经通过环境变量设置了代理，直接使用
    if [ -n "$GH_PROXY" ]; then
        printf "${BLUE}[*]${NC} 使用预设代理: ${GH_PROXY}\n"
        return
    fi

    printf "${BLUE}[*]${NC} 检测 GitHub 连通性...\n"
    local test_url="https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/README.md"
    local tmp
    tmp=$(mktemp /tmp/fg-test-XXXXXX)

    if download "$test_url" "$tmp" 2>/dev/null && [ -s "$tmp" ]; then
        printf "${GREEN}[+]${NC} GitHub 直连正常\n"
        rm -f "$tmp"
        return
    fi
    rm -f "$tmp"

    printf "${YELLOW}[!]${NC} GitHub 直连失败，尝试使用加速代理...\n"
    GH_PROXY="https://gh-proxy.com/"
    printf "${GREEN}[+]${NC} 已启用代理: ${GH_PROXY}\n"
}

# 获取实际下载 URL（自动加代理前缀）
proxy_url() {
    local url="$1"
    if [ -n "$GH_PROXY" ]; then
        echo "${GH_PROXY}${url}"
    else
        echo "$url"
    fi
}

# 执行子脚本：优先本地文件，回退到远程下载
run_subscript() {
    local name="$1"  # e.g. "scripts/controller.sh"

    # 尝试本地路径
    if [ -n "$SCRIPT_DIR" ] && [ -f "${SCRIPT_DIR}/${name}" ]; then
        # 传递代理和下载地址给子脚本
        export GH_PROXY
        export DOWNLOAD_BASE="$(proxy_url "$REPO_RELEASE_BASE")"
        bash "${SCRIPT_DIR}/${name}"
        return
    fi

    # 远程下载到临时文件后执行（保留 stdin 交互能力）
    local url
    url=$(proxy_url "${REPO_RAW_BASE}/${name}")
    local tmp
    tmp=$(mktemp /tmp/fg-XXXXXX.sh)

    if ! download "$url" "$tmp" || [ ! -s "$tmp" ]; then
        printf "${RED}[-]${NC} 下载失败: ${url}\n"
        rm -f "$tmp"
        exit 1
    fi

    # 传递代理和下载地址给子脚本
    export GH_PROXY
    export DOWNLOAD_BASE="$(proxy_url "$REPO_RELEASE_BASE")"
    bash "$tmp"
    rm -f "$tmp"
}

clear
echo "==============================================="
echo "      Floating Gateway 自动化部署系统"
echo "==============================================="
echo ""

# 自动检测是否需要代理
detect_proxy
echo ""

echo "你想管理哪个部分？"
echo ""
echo "  1) [管理端] Gateway Controller (Web UI, 部署中心)"
echo "     - 适用于你的电脑或长期运行的服务器"
echo ""
echo "  2) [路由器] Gateway Agent (VIP 漂移, 健康检测)"
echo "     - 适用于 OpenWrt 路由器或旁路由服务器"
echo ""
echo "  0) 退出"
echo "-----------------------------------------------"
printf "请选择 [0-2]: "
read -r main_choice

case "$main_choice" in
    1)
        run_subscript "scripts/controller.sh"
        ;;
    2)
        run_subscript "scripts/router.sh"
        ;;
    0)
        exit 0
        ;;
    *)
        echo "无效选项，退出。"
        exit 1
        ;;
esac
