# 🏠 Floating Gateway (浮动网关)

[![GitHub Release](https://img.shields.io/github/v/release/zczy-k/FloatingGateway)](../../releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Floating Gateway** 是一个为家庭网络设计的双路由器高可用方案。它通过 **VRRP (虚拟路由冗余协议)** 技术，在你的主路由（Primary）和旁路由（Secondary）之间建立一个 **VIP (虚拟网关 IP)**。

> [!TIP]
> 推荐在具有高可用要求的家庭网络环境中使用，支持 OpenWrt 和主流 Linux 发行版。

当你的旁路由（出口网关）健康时，它持有 VIP 并处理所有流量；一旦检测到旁路由的国际链路故障或宕机，VIP 会自动秒级漂移到主路由，确保网络始终可用。

---

## 🌟 核心特性

- **🚀 自动故障切换**: 无需手动修改任何设备网关，故障时自动切换，恢复时自动抢占。
- **🔍 智能健康检测**: 内置 Ping、DNS、TCP、HTTP 四种检测，支持 `basic`（连通性）和 `internet`（国际链路）模式。
- **🛡️ 极致稳定性**: 基于成熟的 Keepalived 核心，结合 Go 语言编写的防抖策略（k-of-n 判定）。
- **🖥️ 可视化管理**: 提供跨平台 Web 控制台，支持 Windows、macOS、Linux 甚至 OpenWrt。
- **🛠️ 零配置部署**: 
    - **智能网卡识别**: 自动探测路由器的 LAN 网卡和网段。
    - **一键环境安装**: 自动安装 Keepalived 及其依赖。
    - **Windows 双击即用**: 自动识别局域网 IP 并打开浏览器。
- **🔄 自动更新 (New!)**: 
    - **自动发布**: 代码推送后自动构建并发布新版本。
    - **一键升级**: Web UI 内置版本检查和自动升级功能。
    - **智能下载**: 自动选择最快的下载源（支持 GitHub 加速镜像）。

---

## 🚀 快速安装 (新手推荐)

### 1. 启动管理控制台
控制台是管理中心，运行在你的常用电脑或服务器上。

#### **Windows**
1. 从 [Releases](../../releases) 下载 `gateway-controller-windows-amd64.exe`。
2. **直接双击运行**，程序会自动打开浏览器访问管理界面。

#### **Linux / macOS / OpenWrt**
一键运行交互式安装脚本（无需手动下载或赋权）：
```bash
bash <(curl -sSL https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/setup.sh)
```

**国内用户加速**（如果上面的命令下载缓慢或超时）：
```bash
bash <(curl -sSL https://gh-proxy.com/https://raw.githubusercontent.com/zczy-k/FloatingGateway/main/setup.sh)
```
> 脚本会自动检测 GitHub 连通性，国内网络下会自动启用代理加速，后续下载也会走加速通道。

选择 `1) [管理端] Gateway Controller` 即可。

### 2. 在 Web UI 中部署 (三步走)
1. **添加路由器**: 输入路由器的 IP 和 SSH 账号。点击 **“探测”**，系统会自动识别网卡并建议全局配置。
2. **全局设置**: 确认 VIP（虚拟网关 IP）、网段和网卡接口。
3. **一键安装**: 点击路由器卡片上的 **“安装 Agent”**。系统将自动完成所有配置。

### 3. 修改 DHCP 设置 (最后一步)
在你的主路由 (OpenWrt) 中，将 DHCP 服务器的 **“默认网关”** 设置为你刚才配置的 **VIP**。
- *路径: 网络 -> 接口 -> LAN -> 修改 -> DHCP 服务器 -> 高级设置 -> DHCP 选项: `3,192.168.1.254`*

---

## 🛠️ 进阶：命令行管理

如果你是高级用户，可以直接在路由器上使用 `gateway-agent`：

```bash
# 自动探测网卡
gateway-agent detect-iface

# 运行自检
gateway-agent doctor --fix

# 查看状态
gateway-agent status --json
```

---

## 📊 常见问题 (FAQ)

- **Q: 如何升级到最新版本？**
  A: 在 Web UI 中点击"检查更新"按钮，如果有新版本会显示"自动升级"按钮，点击即可一键升级。
- **Q: VIP 漂移验证失败怎么办？**
  A: 这是最常见的问题。请参考 [VIP 漂移故障排查指南](docs/TROUBLESHOOTING-VIP-DRIFT.md)，或在备节点上运行 `bash scripts/fix-vrrp.sh --fix` 进行自动诊断和修复。
- **Q: PVE 环境下 VIP 无法漂移？**
  A: 请在 PVE 的网卡设置中关闭 "IP Anti-Spoofing" 或允许 MAC 地址欺骗。同时确保防火墙允许 VRRP 协议 (112)。
- **Q: 防火墙需要开什么端口？**
  A: 必须允许 **VRRP 协议 (112)** 在 LAN 内通行。可以运行 `iptables -I INPUT -p 112 -j ACCEPT` 添加规则。
- **Q: 为什么显示 Unhealthy？**
  A: 检查你的旁路由是否真的能访问国际互联网（如果你开启了 internet 模式）。

---

## 🏗️ 开发者指南

### 编译项目
```bash
# 交叉编译所有平台
./scripts/build.sh
```

---

## 📜 许可证

本项目采用 [MIT License](LICENSE) 开源。

---
*如果有任何问题，欢迎在 [zczy-k/FloatingGateway](https://github.com/zczy-k/FloatingGateway) 提交 Issue！*
