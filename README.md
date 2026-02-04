# Floating Gateway

双路由器 VIP 浮动网关系统，使用 VRRP (Keepalived) 实现自动故障切换。

## 功能

- **自动故障切换**: Secondary 路由器国际链路故障时，VIP 自动漂移到 Primary
- **健康检查**: 支持 Ping/DNS/TCP/HTTP 多种检查方式，带防抖逻辑
- **集中管理**: Web UI + REST API，通过 SSH 推送配置到路由器
- **多平台支持**: OpenWrt (procd) / Ubuntu (systemd)

## 架构

```
┌─────────────────┐
│   Controller    │  ← 管理端 (Web UI)
└────────┬────────┘
         │ SSH
    ┌────┴────┐
    ↓         ↓
┌───────┐ ┌───────┐
│Primary│ │Second.│  ← 路由器 (gateway-agent + keepalived)
└───┬───┘ └───┬───┘
    │   VIP   │
    └────┬────┘
         ↓
    LAN Clients
```

## 快速开始

### 方式 1: 使用 Controller (推荐)

1. 从 [Releases](../../releases) 下载 `gateway-controller`
2. 创建 `controller.yaml`:
```yaml
version: 1
listen: ":8080"
lan:
  vip: 192.168.1.1
  cidr: 192.168.1.0/24
keepalived:
  vrid: 51
routers:
  - name: openwrt
    host: 192.168.1.2
    user: root
    key_file: ~/.ssh/id_rsa
    role: primary
  - name: ubuntu
    host: 192.168.1.3
    user: root
    key_file: ~/.ssh/id_rsa
    role: secondary
```
3. 启动: `./gateway-controller serve`
4. 打开 http://localhost:8080 点击 "Install" 安装 Agent

### 方式 2: 手动安装 Agent

在路由器上执行:
```bash
# 下载 (替换为实际 URL)
wget https://github.com/youruser/floatip/releases/latest/download/gateway-agent-linux-arm64
chmod +x gateway-agent-linux-arm64
mv gateway-agent-linux-arm64 /usr/bin/gateway-agent

# 配置
mkdir -p /etc/gateway-agent
cat > /etc/gateway-agent/config.yaml << EOF
version: 1
role: secondary
lan:
  iface: eth0
  vip: 192.168.1.1
routers:
  peer_ip: 192.168.1.2
health:
  mode: internet
EOF

# 应用并启动
gateway-agent apply
gateway-agent run
```

## 编译

```bash
# 本地编译
go build -o gateway-agent ./cmd/agent
go build -o gateway-controller ./cmd/controller

# 交叉编译 (Linux ARM64)
GOOS=linux GOARCH=arm64 go build -o gateway-agent-linux-arm64 ./cmd/agent
```

或推送 tag 到 GitHub 自动编译:
```bash
git tag v1.0.0
git push origin v1.0.0
```

## 配置说明

详见 [examples/](examples/) 目录:
- `config-primary.yaml` - Primary 路由器配置
- `config-secondary.yaml` - Secondary 路由器配置  
- `controller.yaml` - Controller 配置

## 命令

### gateway-agent (路由器端)
```
run       运行守护进程
check     单次健康检查 (供 keepalived 调用)
apply     生成并应用 keepalived 配置
doctor    自检
status    状态查看
```

### gateway-controller (管理端)
```
serve     启动 Web UI
probe     探测路由器状态
install   安装 Agent
status    查看状态
```

## License

MIT
