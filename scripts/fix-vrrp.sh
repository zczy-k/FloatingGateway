#!/bin/bash
# fix-vrrp.sh - VRRP 漂移问题快速诊断和修复脚本
# 用法: ./fix-vrrp.sh [--fix]
# 不带参数只诊断，带 --fix 参数会尝试自动修复

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置 (从 gateway-agent 配置读取，或使用默认值)
VIP="${VIP:-192.168.1.1}"
IFACE="${IFACE:-br-lan}"
AUTO_FIX=false

# 解析参数
if [ "$1" = "--fix" ]; then
    AUTO_FIX=true
    echo -e "${YELLOW}自动修复模式已启用${NC}"
fi

echo "==================================="
echo "  VRRP 漂移问题诊断工具"
echo "==================================="
echo ""

# 检查是否为 root
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}✗ 错误: 需要 root 权限运行此脚本${NC}"
    exit 1
fi

ISSUES_FOUND=0

# 1. 检查 Keepalived 运行状态
echo "1. 检查 Keepalived 运行状态..."
if pgrep -x keepalived > /dev/null 2>&1 || pidof keepalived > /dev/null 2>&1; then
    echo -e "   ${GREEN}✓${NC} Keepalived 正在运行"
else
    echo -e "   ${RED}✗${NC} Keepalived 未运行"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
    
    if [ "$AUTO_FIX" = true ]; then
        echo "   → 尝试启动 Keepalived..."
        if systemctl start keepalived 2>/dev/null || /etc/init.d/keepalived start 2>/dev/null; then
            echo -e "   ${GREEN}✓${NC} Keepalived 已启动"
            sleep 2
        else
            echo -e "   ${RED}✗${NC} 启动失败"
        fi
    fi
fi

# 2. 检查配置文件
echo ""
echo "2. 检查 Keepalived 配置文件..."
CONFIG_PATH="/etc/keepalived/keepalived.conf"
if [ ! -f "$CONFIG_PATH" ]; then
    CONFIG_PATH="/tmp/keepalived.conf"
fi

if [ -f "$CONFIG_PATH" ]; then
    echo -e "   ${GREEN}✓${NC} 配置文件存在: $CONFIG_PATH"
    
    # 验证配置语法
    if keepalived -t -f "$CONFIG_PATH" 2>&1 | grep -qi "valid\|successful"; then
        echo -e "   ${GREEN}✓${NC} 配置文件语法正确"
    else
        echo -e "   ${RED}✗${NC} 配置文件语法错误"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
        keepalived -t -f "$CONFIG_PATH" 2>&1 | head -n 5
        
        if [ "$AUTO_FIX" = true ]; then
            echo "   → 尝试重新生成配置..."
            if command -v gateway-agent > /dev/null 2>&1; then
                gateway-agent apply
                echo -e "   ${GREEN}✓${NC} 配置已重新生成"
            else
                echo -e "   ${RED}✗${NC} gateway-agent 命令未找到"
            fi
        fi
    fi
    
    # 检查是否使用单播
    if grep -q "unicast_peer" "$CONFIG_PATH"; then
        echo -e "   ${GREEN}✓${NC} 使用单播模式 (unicast)"
        
        # 提取对端 IP
        PEER_IP=$(grep -A 1 "unicast_peer" "$CONFIG_PATH" | tail -n 1 | tr -d ' {}')
        if [ -n "$PEER_IP" ]; then
            echo "   → 对端 IP: $PEER_IP"
        fi
    else
        echo -e "   ${YELLOW}!${NC} 使用组播模式 (multicast)"
    fi
else
    echo -e "   ${RED}✗${NC} 配置文件不存在"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
fi

# 3. 检查防火墙规则
echo ""
echo "3. 检查防火墙规则 (VRRP 协议112)..."
if iptables -L INPUT -n 2>/dev/null | grep -q "112"; then
    echo -e "   ${GREEN}✓${NC} iptables 已放行 VRRP"
else
    echo -e "   ${RED}✗${NC} iptables 未发现 VRRP 放行规则"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
    
    if [ "$AUTO_FIX" = true ]; then
        echo "   → 添加 iptables 规则..."
        iptables -I INPUT -p 112 -j ACCEPT 2>/dev/null
        iptables -I OUTPUT -p 112 -j ACCEPT 2>/dev/null
        echo -e "   ${GREEN}✓${NC} 规则已添加"
    fi
fi

# OpenWrt 特定检查
if [ -f "/etc/openwrt_release" ] || [ -f "/etc/openwrt_version" ]; then
    if uci get firewall.vrrp.target 2>/dev/null | grep -q "ACCEPT"; then
        echo -e "   ${GREEN}✓${NC} OpenWrt 防火墙已配置 VRRP 规则"
    else
        echo -e "   ${RED}✗${NC} OpenWrt 防火墙未配置 VRRP 规则"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
        
        if [ "$AUTO_FIX" = true ]; then
            echo "   → 配置 OpenWrt 防火墙..."
            uci delete firewall.vrrp 2>/dev/null || true
            uci set firewall.vrrp=rule
            uci set firewall.vrrp.name='Allow-VRRP'
            uci set firewall.vrrp.src='lan'
            uci set firewall.vrrp.proto='112'
            uci set firewall.vrrp.target='ACCEPT'
            uci commit firewall
            /etc/init.d/firewall reload
            echo -e "   ${GREEN}✓${NC} OpenWrt 防火墙已配置"
        fi
    fi
fi

# 4. 检查 VIP 状态
echo ""
echo "4. 检查 VIP 分配状态..."
if ip addr show dev "$IFACE" 2>/dev/null | grep -q "$VIP"; then
    echo -e "   ${GREEN}✓${NC} VIP $VIP 已分配到接口 $IFACE"
else
    echo -e "   ${YELLOW}!${NC} VIP $VIP 未分配到接口 $IFACE (可能处于 BACKUP 状态)"
fi

# 5. 检查 VRRP 状态文件
echo ""
echo "5. 检查 VRRP 状态文件..."
STATE_FILE="/tmp/keepalived.GATEWAY.state"
if [ -f "$STATE_FILE" ]; then
    STATE=$(cat "$STATE_FILE")
    echo -e "   ${GREEN}✓${NC} 状态文件存在: $STATE"
else
    echo -e "   ${RED}✗${NC} 状态文件不存在 (notify 脚本可能未执行)"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
    
    if [ "$AUTO_FIX" = true ]; then
        echo "   → 测试 notify 脚本..."
        AGENT_BIN=$(command -v gateway-agent 2>/dev/null || echo "/gateway-agent/gateway-agent")
        
        if [ -x "$AGENT_BIN" ]; then
            "$AGENT_BIN" notify TEST 2>&1
            if [ -f "$STATE_FILE" ]; then
                echo -e "   ${GREEN}✓${NC} notify 脚本执行成功"
            else
                echo -e "   ${RED}✗${NC} notify 脚本执行失败"
            fi
        else
            echo -e "   ${RED}✗${NC} gateway-agent 二进制文件不存在或无执行权限: $AGENT_BIN"
        fi
    fi
fi

# 6. 检查网络接口
echo ""
echo "6. 检查网络接口..."
if ip link show "$IFACE" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✓${NC} 接口 $IFACE 存在"
    
    # 检查接口状态
    if ip link show "$IFACE" | grep -q "UP"; then
        echo -e "   ${GREEN}✓${NC} 接口 $IFACE 已启用"
    else
        echo -e "   ${RED}✗${NC} 接口 $IFACE 未启用"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
    
    # 检查组播标志
    if ip link show "$IFACE" | grep -q "MULTICAST"; then
        echo -e "   ${GREEN}✓${NC} 接口支持组播"
    else
        echo -e "   ${YELLOW}!${NC} 接口不支持组播 (单播模式下可忽略)"
    fi
else
    echo -e "   ${RED}✗${NC} 接口 $IFACE 不存在"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
fi

# 7. 检查对端连通性 (如果是单播模式)
if [ -n "$PEER_IP" ]; then
    echo ""
    echo "7. 检查对端连通性 (单播模式)..."
    if ping -c 1 -W 2 "$PEER_IP" > /dev/null 2>&1; then
        echo -e "   ${GREEN}✓${NC} 可以 Ping 通对端 $PEER_IP"
    else
        echo -e "   ${RED}✗${NC} 无法 Ping 通对端 $PEER_IP"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
fi

# 8. 检查最近的日志
echo ""
echo "8. 检查最近的 Keepalived 日志..."
if command -v logread > /dev/null 2>&1; then
    # OpenWrt
    LOG_OUTPUT=$(logread | grep -i keepalived | tail -n 3)
elif [ -f "/var/log/syslog" ]; then
    # Debian/Ubuntu
    LOG_OUTPUT=$(tail -n 50 /var/log/syslog | grep -i keepalived | tail -n 3)
elif command -v journalctl > /dev/null 2>&1; then
    # systemd
    LOG_OUTPUT=$(journalctl -u keepalived -n 3 --no-pager 2>/dev/null)
else
    LOG_OUTPUT=""
fi

if [ -n "$LOG_OUTPUT" ]; then
    echo "$LOG_OUTPUT" | while IFS= read -r line; do
        if echo "$line" | grep -qi "error\|fail\|warn"; then
            echo -e "   ${YELLOW}!${NC} $line"
        else
            echo "   $line"
        fi
    done
else
    echo -e "   ${YELLOW}!${NC} 未找到日志"
fi

# 9. 测试 VRRP 包捕获 (可选)
echo ""
echo "9. VRRP 包捕获测试 (5秒)..."
if command -v tcpdump > /dev/null 2>&1; then
    echo "   → 监听 VRRP 协议包..."
    VRRP_PACKETS=$(timeout 5 tcpdump -i "$IFACE" -c 5 proto 112 -n 2>&1 | grep -c "VRRP" || echo "0")
    if [ "$VRRP_PACKETS" -gt 0 ]; then
        echo -e "   ${GREEN}✓${NC} 捕获到 $VRRP_PACKETS 个 VRRP 包"
    else
        echo -e "   ${RED}✗${NC} 未捕获到 VRRP 包 (可能被拦截或使用单播)"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
else
    echo -e "   ${YELLOW}!${NC} tcpdump 未安装，跳过"
fi

# 总结
echo ""
echo "==================================="
echo "  诊断总结"
echo "==================================="

if [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${GREEN}✓ 未发现明显问题${NC}"
    echo ""
    echo "如果 VIP 漂移仍然失败，请检查："
    echo "  1. 虚拟化平台网卡设置 (PVE/ESXi 混杂模式)"
    echo "  2. 交换机是否支持 VRRP"
    echo "  3. 两个节点是否在同一个二层网络"
else
    echo -e "${RED}✗ 发现 $ISSUES_FOUND 个问题${NC}"
    
    if [ "$AUTO_FIX" = false ]; then
        echo ""
        echo "运行 '$0 --fix' 尝试自动修复"
    fi
fi

echo ""
echo "详细排查指南: docs/TROUBLESHOOTING-VIP-DRIFT.md"
echo "==================================="

exit $ISSUES_FOUND
