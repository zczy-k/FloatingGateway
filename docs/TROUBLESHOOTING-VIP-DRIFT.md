# VIP 漂移验证失败排查指南

## 问题描述

验证网关漂移时，出现以下错误：
```
验证漂移 (VIP 切换)
诊断结果：备节点 (openwrt) 未能接管 VIP。当前状态: UNKNOWN。
可能是 VRRP 组播被拦截 (请检查 PVE/ESXi 网卡防火墙)。
```

## 根本原因分析

虽然错误提示提到组播，但实际上系统使用的是**单播模式** (unicast)。问题的真正原因可能是：

1. **VRRP 状态未正确更新** - notify 脚本执行失败或权限问题
2. **单播通信失败** - 两个节点之间的 VRRP 单播包被阻止
3. **Keepalived 配置问题** - 配置文件有误或未正确加载
4. **网络层问题** - 防火墙、路由或网卡设置阻止了 VRRP 协议

## 排查步骤

### 1. 检查备节点 Keepalived 状态

在备节点上执行：

```bash
# 检查 Keepalived 是否运行
ps | grep keepalived
# 或
systemctl status keepalived

# 查看 Keepalived 日志
tail -f /var/log/syslog | grep -i keepalived
# OpenWrt 上使用
logread -f | grep -i keepalived
```

### 2. 检查 VRRP 状态文件

```bash
# 查看状态文件
cat /tmp/keepalived.GATEWAY.state

# 如果文件不存在或显示 UNKNOWN，说明 notify 脚本有问题
```

### 3. 手动测试 notify 脚本

```bash
# 找到 agent 二进制文件位置
which gateway-agent
# 或
ls -la /gateway-agent/gateway-agent

# 手动执行 notify 命令
/gateway-agent/gateway-agent notify MASTER

# 检查状态文件是否创建
cat /tmp/keepalived.GATEWAY.state
```

### 4. 检查单播通信

在备节点上检查是否能与主节点通信：

```bash
# Ping 主节点
ping -c 3 <主节点IP>

# 检查 VRRP 包 (协议号 112)
tcpdump -i <网卡名> proto 112 -n

# 检查防火墙规则
iptables -L -n | grep 112
# OpenWrt 上
uci show firewall | grep vrrp
```

### 5. 检查 Keepalived 配置

```bash
# 查看配置文件
cat /etc/keepalived/keepalived.conf
# OpenWrt 上可能在
cat /tmp/keepalived.conf

# 验证配置文件语法
keepalived -t -f /etc/keepalived/keepalived.conf
```

### 6. 检查 VIP 是否被接管

```bash
# 在备节点上检查 VIP 是否已分配
ip addr show | grep <VIP地址>

# 检查 ARP 表
ip neigh show | grep <VIP地址>
```

## 常见问题和解决方案

### 问题 1: notify 脚本权限不足

**症状**: 状态文件不存在或始终为 UNKNOWN

**解决方案**:
```bash
# 确保 agent 二进制文件有执行权限
chmod +x /gateway-agent/gateway-agent

# 确保状态文件目录可写
chmod 777 /tmp

# 手动创建状态文件测试
echo "BACKUP" > /tmp/keepalived.GATEWAY.state
```

### 问题 2: VRRP 协议被防火墙拦截

**症状**: tcpdump 看不到 VRRP 包

**解决方案 (OpenWrt)**:
```bash
# 添加防火墙规则允许 VRRP (协议号 112)
uci set firewall.vrrp=rule
uci set firewall.vrrp.name='Allow-VRRP'
uci set firewall.vrrp.src='lan'
uci set firewall.vrrp.proto='112'
uci set firewall.vrrp.target='ACCEPT'
uci commit firewall
/etc/init.d/firewall reload
```

**解决方案 (Linux iptables)**:
```bash
# 允许 VRRP 协议
iptables -I INPUT -p 112 -j ACCEPT
iptables -I OUTPUT -p 112 -j ACCEPT

# 保存规则
iptables-save > /etc/iptables/rules.v4
```

### 问题 3: 虚拟化平台网卡限制

**症状**: 配置都正确但 VIP 仍然无法漂移

**解决方案 (PVE/Proxmox)**:
1. 编辑虚拟机配置
2. 网络设备 → 高级 → 启用 "混杂模式"
3. 或在命令行修改：
```bash
# 在 PVE 宿主机上
qm set <VMID> -net0 virtio,bridge=vmbr0,firewall=0
```

**解决方案 (ESXi)**:
1. 编辑虚拟机设置
2. 网络适配器 → 安全 → 启用以下选项：
   - 混杂模式: 接受
   - MAC 地址更改: 接受
   - 伪传输: 接受

### 问题 4: Keepalived 配置中缺少 unicast_src_ip

**症状**: 配置文件中有 unicast_peer 但没有 unicast_src_ip

**解决方案**:
```bash
# 重新生成配置
gateway-agent apply

# 或手动编辑配置文件，确保包含：
# unicast_src_ip <本机IP>
# unicast_peer {
#     <对端IP>
# }
```

### 问题 5: 健康检查脚本失败导致优先级过低

**症状**: 备节点 Keepalived 运行但优先级不足以接管

**解决方案**:
```bash
# 手动测试健康检查
gateway-agent check --mode=basic
# 或
gateway-agent check --mode=internet

# 查看健康检查日志
journalctl -u gateway-agent -f
# OpenWrt 上
logread -f | grep gateway-agent
```

## 改进建议

### 1. 增强诊断信息

修改 `internal/controller/api.go` 中的诊断逻辑，添加更详细的检查：

```go
// 检查 Keepalived 是否运行
out, _ := sshBackup.RunCombined("ps | grep keepalived | grep -v grep")
isRunning := strings.Contains(out, "keepalived")

// 检查配置文件
configOut, _ := sshBackup.RunCombined("keepalived -t -f /etc/keepalived/keepalived.conf 2>&1")

// 检查 notify 脚本权限
notifyOut, _ := sshBackup.RunCombined("ls -la /gateway-agent/gateway-agent")

// 检查防火墙规则
fwOut, _ := sshBackup.RunCombined("iptables -L -n | grep 112")
```

### 2. 自动修复功能

在验证失败时，自动尝试修复常见问题：

```go
// 尝试重启 Keepalived
sshBackup.RunCombined("systemctl restart keepalived || /etc/init.d/keepalived restart")

// 尝试手动触发 notify
sshBackup.RunCombined("/gateway-agent/gateway-agent notify MASTER")

// 检查并修复防火墙
sshBackup.RunCombined("iptables -I INPUT -p 112 -j ACCEPT 2>/dev/null")
```

### 3. 使用更可靠的状态检测

不依赖状态文件，直接检查 VIP 是否在接口上：

```go
// 检查 VIP 是否在接口上
out, _ := sshBackup.RunCombined(fmt.Sprintf("ip addr show dev %s | grep %s", cfg.LAN.Iface, cfg.LAN.VIP))
hasVIP := strings.Contains(out, cfg.LAN.VIP)
```

## 快速修复脚本

创建一个快速诊断和修复脚本：

```bash
#!/bin/bash
# fix-vrrp.sh - 快速修复 VRRP 漂移问题

VIP="192.168.1.1"
IFACE="br-lan"

echo "=== VRRP 漂移问题诊断 ==="

# 1. 检查 Keepalived
echo "1. 检查 Keepalived 状态..."
if pgrep keepalived > /dev/null; then
    echo "   ✓ Keepalived 正在运行"
else
    echo "   ✗ Keepalived 未运行，尝试启动..."
    systemctl start keepalived || /etc/init.d/keepalived start
fi

# 2. 检查配置
echo "2. 检查配置文件..."
if keepalived -t -f /etc/keepalived/keepalived.conf 2>&1 | grep -q "Configuration is valid"; then
    echo "   ✓ 配置文件有效"
else
    echo "   ✗ 配置文件无效，尝试重新生成..."
    gateway-agent apply
fi

# 3. 检查防火墙
echo "3. 检查防火墙规则..."
if iptables -L -n | grep -q "112"; then
    echo "   ✓ 防火墙已放行 VRRP"
else
    echo "   ✗ 防火墙未放行 VRRP，添加规则..."
    iptables -I INPUT -p 112 -j ACCEPT
    iptables -I OUTPUT -p 112 -j ACCEPT
fi

# 4. 检查 VIP
echo "4. 检查 VIP 状态..."
if ip addr show dev $IFACE | grep -q $VIP; then
    echo "   ✓ VIP 已分配到本机"
else
    echo "   ✗ VIP 未分配"
fi

# 5. 测试 notify 脚本
echo "5. 测试 notify 脚本..."
if /gateway-agent/gateway-agent notify TEST 2>&1; then
    echo "   ✓ notify 脚本可执行"
    cat /tmp/keepalived.GATEWAY.state
else
    echo "   ✗ notify 脚本执行失败"
fi

echo "=== 诊断完成 ==="
```

## 总结

VIP 漂移失败通常是由以下原因造成的（按概率排序）：

1. **防火墙拦截 VRRP 协议** (最常见)
2. **虚拟化平台网卡限制** (PVE/ESXi)
3. **notify 脚本权限或路径问题**
4. **Keepalived 配置错误**
5. **网络连通性问题**

建议按照上述排查步骤逐一检查，大多数问题都可以通过添加防火墙规则或调整虚拟化平台设置解决。
