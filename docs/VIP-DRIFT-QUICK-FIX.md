# VIP 漂移失败快速修复指南

## 问题现象

在 Web 控制台点击"验证漂移"后，显示以下错误：

```
验证漂移 (VIP 切换)
✗ 诊断结果：备节点 (openwrt) 未能接管 VIP。当前状态: UNKNOWN。
  可能是 VRRP 组播被拦截 (请检查 PVE/ESXi 网卡防火墙)。
```

## 快速解决方案

### 方案 1: 使用自动修复脚本 (推荐)

在**备节点**上执行以下命令：

```bash
# 下载并运行修复脚本
curl -sSL https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/scripts/fix-vrrp.sh | bash -s -- --fix

# 或者如果已经安装了项目
bash /gateway-agent/scripts/fix-vrrp.sh --fix
```

脚本会自动检查并修复以下问题：
- Keepalived 是否运行
- 配置文件是否正确
- 防火墙是否放行 VRRP
- notify 脚本是否可执行
- 网络连通性

### 方案 2: 手动排查和修复

#### 步骤 1: 检查防火墙 (最常见原因)

**在备节点上执行**：

```bash
# 添加 VRRP 协议放行规则
iptables -I INPUT -p 112 -j ACCEPT
iptables -I OUTPUT -p 112 -j ACCEPT

# OpenWrt 用户还需要配置 UCI
uci set firewall.vrrp=rule
uci set firewall.vrrp.name='Allow-VRRP'
uci set firewall.vrrp.src='lan'
uci set firewall.vrrp.proto='112'
uci set firewall.vrrp.target='ACCEPT'
uci commit firewall
/etc/init.d/firewall reload
```

#### 步骤 2: 检查 Keepalived 状态

```bash
# 检查是否运行
ps | grep keepalived

# 如果未运行，启动它
systemctl start keepalived
# 或 OpenWrt
/etc/init.d/keepalived start

# 查看日志
logread | grep keepalived
# 或 Linux
journalctl -u keepalived -f
```

#### 步骤 3: 验证配置文件

```bash
# 检查配置文件语法
keepalived -t -f /etc/keepalived/keepalived.conf

# 如果有错误，重新生成配置
gateway-agent apply
```

#### 步骤 4: 测试 notify 脚本

```bash
# 手动执行 notify 脚本
/gateway-agent/gateway-agent notify MASTER

# 检查状态文件是否创建
cat /tmp/keepalived.GATEWAY.state
```

#### 步骤 5: 检查网络连通性

```bash
# Ping 主节点 (替换为实际 IP)
ping -c 3 192.168.1.2

# 检查是否能收到 VRRP 包
tcpdump -i br-lan proto 112 -n
```

### 方案 3: 虚拟化平台设置 (PVE/ESXi)

如果你的路由器运行在虚拟化平台上，需要调整网卡设置。

#### Proxmox VE (PVE)

1. 在 PVE Web 界面选择虚拟机
2. 硬件 → 网络设备 → 编辑
3. 高级 → 启用 "混杂模式"
4. 或在命令行执行：

```bash
# 在 PVE 宿主机上执行
qm set <VMID> -net0 virtio,bridge=vmbr0,firewall=0
```

#### VMware ESXi

1. 编辑虚拟机设置
2. 网络适配器 → 安全
3. 设置以下选项为"接受"：
   - 混杂模式: 接受
   - MAC 地址更改: 接受
   - 伪传输: 接受

## 验证修复

修复后，重新在 Web 控制台点击"验证漂移"按钮，应该能看到：

```
验证漂移 (VIP 切换)
✓ VIP 访问正常，漂移成功！
```

## 仍然失败？

如果上述方案都无法解决，请：

1. 查看详细的 [故障排查指南](TROUBLESHOOTING-VIP-DRIFT.md)
2. 收集以下信息并提交 Issue：
   - 两个节点的操作系统和版本
   - `gateway-agent status --json` 输出
   - Keepalived 日志
   - 网络拓扑（是否使用虚拟化、交换机型号等）

## 常见错误信息解读

| 错误信息 | 可能原因 | 解决方案 |
|---------|---------|---------|
| 状态: UNKNOWN | notify 脚本未执行 | 检查 agent 二进制文件权限 |
| 防火墙未发现 VRRP 规则 | 防火墙拦截 | 添加 iptables 规则 |
| 无法 Ping 通主节点 | 网络不通 | 检查路由和防火墙 |
| Keepalived 未运行 | 服务未启动 | 启动 Keepalived 服务 |
| 配置文件无效 | 配置错误 | 重新生成配置 |
| VIP 已分配但无法访问 | ARP 缓存问题 | 清空 ARP 缓存或等待超时 |

## 预防措施

为了避免将来出现问题，建议：

1. **定期测试漂移功能**：每月至少测试一次
2. **监控 Keepalived 日志**：设置日志告警
3. **保持配置同步**：修改配置后及时应用到两个节点
4. **备份配置文件**：定期备份 `/etc/gateway-agent/config.yaml`
5. **更新到最新版本**：新版本通常包含 bug 修复和改进

## 相关文档

- [完整故障排查指南](TROUBLESHOOTING-VIP-DRIFT.md)
- [配置文件说明](../examples/config-primary.yaml)
- [架构设计文档](../README.md)
