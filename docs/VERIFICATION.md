# 验证步骤文档

本文档说明如何部署和验证 Floating Gateway 系统。

## 1. 环境准备

### 网络拓扑

```
Internet
    │
    ├──── [WAN] Primary (A) - OpenWrt ──── [LAN: br-lan]
    │                                           │
    │                                      ┌────┴────┐
    │                                      │   VIP   │
    │                                      │192.168.1.1│
    │                                      └────┬────┘
    │                                           │
    └──── [WAN] Secondary (B) - Ubuntu ─── [LAN: eth0]
                                                │
                                           LAN Clients
```

### 示例配置（请根据实际情况修改）

| 项目 | Primary (A) | Secondary (B) |
|------|-------------|---------------|
| 平台 | OpenWrt | Ubuntu 22.04 |
| LAN 接口 | br-lan | eth0 |
| 实际 IP | 192.168.1.2 | 192.168.1.3 |
| VIP | 192.168.1.1 | 192.168.1.1 |
| VRID | 51 | 51 |
| 角色 | primary | secondary |
| 健康模式 | basic | internet |

---

## 2. 部署步骤

### 方式 A：使用 Controller（推荐）

#### 2.1 在管理机上安装 Controller

```bash
# Linux/macOS
sh scripts/controller.sh install

# Windows PowerShell
.\scripts\controller.ps1 install
```

#### 2.2 编辑 Controller 配置

编辑 `~/.gateway-controller/config.yaml`：

```yaml
version: 1
listen: ":8080"

lan:
  vip: "192.168.1.1"        # 修改为你的 VIP
  cidr: "192.168.1.0/24"    # 修改为你的网段

keepalived:
  vrid: 51

routers:
  - name: openwrt-main
    host: 192.168.1.2       # Primary 路由器 IP
    port: 22
    user: root
    key_file: ~/.ssh/id_rsa # 或使用 password
    role: primary

  - name: ubuntu-gateway
    host: 192.168.1.3       # Secondary 路由器 IP
    port: 22
    user: root
    key_file: ~/.ssh/id_rsa
    role: secondary
```

#### 2.3 启动 Controller 并部署

```bash
# 启动 Controller
gateway-controller serve

# 打开浏览器访问 http://localhost:8080
# 在 Web UI 中点击 "Install" 按钮部署 Agent
```

### 方式 B：手动安装 Agent

#### 2.4 在 Primary (OpenWrt) 上安装

```bash
# SSH 登录到 Primary
ssh root@192.168.1.2

# 下载并安装（替换为实际 URL）
wget https://github.com/youruser/floatip/releases/latest/download/gateway-agent-linux-mipsle
chmod +x gateway-agent-linux-mipsle
mv gateway-agent-linux-mipsle /usr/bin/gateway-agent

# 运行安装脚本（交互模式）
sh router.sh

# 或非交互模式
sh router.sh --action=install \
  --role=primary \
  --iface=br-lan \
  --vip=192.168.1.1 \
  --peer-ip=192.168.1.3 \
  --health-mode=basic \
  --openwrt-auto-set-dhcp-gw=true \
  --agent-path=/usr/bin/gateway-agent
```

#### 2.5 在 Secondary (Ubuntu) 上安装

```bash
# SSH 登录到 Secondary
ssh root@192.168.1.3

# 下载并安装
wget https://github.com/youruser/floatip/releases/latest/download/gateway-agent-linux-amd64
chmod +x gateway-agent-linux-amd64
mv gateway-agent-linux-amd64 /usr/bin/gateway-agent

# 运行安装脚本
sh router.sh --action=install \
  --role=secondary \
  --iface=eth0 \
  --vip=192.168.1.1 \
  --peer-ip=192.168.1.2 \
  --health-mode=internet \
  --agent-path=/usr/bin/gateway-agent
```

---

## 3. 验证部署

### 3.1 检查服务状态

```bash
# 在两台路由器上分别执行
gateway-agent status
gateway-agent doctor
```

预期输出：
- Primary: `VRRP State: BACKUP`（Secondary 健康时）
- Secondary: `VRRP State: MASTER`（健康时）

### 3.2 检查 VIP 持有者

```bash
# 在任一路由器上
ip addr show | grep 192.168.1.1

# 或从 LAN 客户端
ping 192.168.1.1
arp -n | grep 192.168.1.1
```

正常情况下，VIP 应该在 Secondary 上。

### 3.3 检查 DHCP 网关设置（Primary OpenWrt）

```bash
# 查看 DHCP 配置
uci show dhcp | grep dhcp_option

# 应包含：dhcp_option='3,192.168.1.1'
```

---

## 4. 故障切换测试

### 4.1 模拟 Secondary 国际链路故障

方法 1：阻断检测目标
```bash
# 在 Secondary 上
iptables -A OUTPUT -d 1.1.1.1 -j DROP
```

方法 2：停止 Agent 健康检查
```bash
# 在 Secondary 上
systemctl stop gateway-agent
```

方法 3：修改健康检查配置为必定失败的目标
```yaml
# /etc/gateway-agent/config.yaml
health:
  internet:
    checks:
      - type: ping
        target: 203.0.113.1  # 不可达地址
        timeout: 1
```

### 4.2 观察 VIP 漂移

```bash
# 在 Primary 上观察
watch -n 1 'gateway-agent status --json | grep -E "(vrrp_state|healthy)"'

# 在 LAN 客户端观察
watch -n 1 'arp -n | grep 192.168.1.1'
```

预期结果：
1. Secondary 健康检查连续失败 3 次后标记为 unhealthy
2. keepalived 检测到 track_script 失败，降低 Secondary 优先级
3. Primary 检测到自己优先级更高，成为 MASTER
4. VIP 漂移到 Primary，触发 GARP
5. LAN 客户端 ARP 表更新，流量切换到 Primary

### 4.3 验证流量走向

```bash
# 在 LAN 客户端执行
traceroute 8.8.8.8
# 或
mtr 8.8.8.8

# 应看到第一跳是 VIP，第二跳是 Primary 的 WAN 网关
```

### 4.4 恢复测试

```bash
# 恢复 Secondary 国际链路
# 方法 1：删除 iptables 规则
iptables -D OUTPUT -d 1.1.1.1 -j DROP

# 方法 2：重启 Agent
systemctl start gateway-agent

# 方法 3：恢复正确配置并 apply
gateway-agent apply
```

预期结果：
1. Secondary 健康检查连续成功 5 次后标记为 healthy
2. 等待 preempt_delay (30秒) 后
3. Secondary 抢占成为 MASTER
4. VIP 漂移回 Secondary

---

## 5. 卸载步骤

### 5.1 卸载 Agent

```bash
# 在路由器上执行
sh router.sh --action=uninstall

# 如需同时卸载 keepalived
sh router.sh --action=uninstall --purge-deps
```

### 5.2 卸载 Controller

```bash
# Linux/macOS
sh scripts/controller.sh uninstall

# Windows
.\scripts\controller.ps1 uninstall
```

### 5.3 验证卸载

```bash
# 检查服务已停止
systemctl status gateway-agent  # 应显示 not found
systemctl status keepalived     # 应显示 inactive（除非 --purge-deps）

# 检查文件已删除
ls /usr/bin/gateway-agent       # 应显示 No such file
ls /etc/gateway-agent/          # 应显示 No such file

# 检查 VIP 已释放
ip addr show | grep 192.168.1.1  # 应无输出
```

### 5.4 回滚 DHCP 网关设置（如需）

```bash
# 查看备份
ls /tmp/gateway-agent-backup-*/

# 恢复 DHCP 配置
cp /tmp/gateway-agent-backup-*/dhcp.backup.* /etc/config/dhcp
/etc/init.d/dnsmasq restart

# 或手动修改
uci delete dhcp.lan.dhcp_option
uci add_list dhcp.lan.dhcp_option='3,192.168.1.2'  # 改回 Primary 实际 IP
uci commit dhcp
/etc/init.d/dnsmasq restart
```

---

## 6. 常见问题

### Q1: VIP 没有漂移

检查：
1. 两台路由器的 VRID 是否相同
2. keepalived 是否运行：`systemctl status keepalived`
3. track_script 是否正常：`gateway-agent check --mode=internet; echo $?`
4. 防火墙是否阻止 VRRP 协议（IP 协议 112）

### Q2: LAN 客户端无法上网

检查：
1. VIP 是否正确配置：`ip addr show`
2. 默认网关是否为 VIP：客户端执行 `route -n`
3. NAT/转发是否正确配置

### Q3: 健康检查总是失败

检查：
1. 检测目标是否可达：`ping 1.1.1.1`
2. DNS 是否正常：`nslookup google.com 1.1.1.1`
3. 配置是否正确：`gateway-agent doctor`

### Q4: 两台都是 MASTER

检查：
1. VRRP 通告是否能互通：检查防火墙
2. 接口是否正确：`ip link show`
3. VRID 是否冲突：确保网络中唯一

---

## 7. 日志查看

```bash
# Agent 日志
# OpenWrt
logread | grep gateway-agent

# Ubuntu
journalctl -u gateway-agent -f

# keepalived 日志
journalctl -u keepalived -f
# 或
tail -f /var/log/syslog | grep -i keepalived
```
