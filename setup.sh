#!/bin/bash
# setup.sh - Floating Gateway 一键管理脚本 (Linux/macOS)
# 
# 这是一个交互式脚本，你可以通过输入数字来执行各种管理任务。

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

clear
echo "==============================================="
echo "      Floating Gateway 自动化部署系统"
echo "==============================================="
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
        if [ -f "${SCRIPT_DIR}/scripts/controller.sh" ]; then
            sh "${SCRIPT_DIR}/scripts/controller.sh"
        else
            echo "找不到 ${SCRIPT_DIR}/scripts/controller.sh"
        fi
        ;;
    2)
        if [ -f "${SCRIPT_DIR}/scripts/router.sh" ]; then
            sh "${SCRIPT_DIR}/scripts/router.sh"
        else
            echo "找不到 ${SCRIPT_DIR}/scripts/router.sh"
        fi
        ;;
    0)
        exit 0
        ;;
    *)
        echo "无效选项，退出。"
        exit 1
        ;;
esac
