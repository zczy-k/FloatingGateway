package controller

// getAssets returns the embedded static assets for the web UI.
func getAssets() map[string][]byte {
	return map[string][]byte{
		"/index.html": []byte(indexHTML),
		"/app.js":     []byte(appJS),
		"/style.css":  []byte(styleCSS),
	}
}

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>浮动网关控制台</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <div id="app">
        <header>
            <div class="header-left">
                <svg class="logo-icon" viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>
                <h1>Floating Gateway</h1>
                <span id="controller-version" class="version-tag">v-</span>
            </div>
            <div id="vip-status">
                <div class="status-chip">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 8v4l2 2"/></svg>
                    <span class="label">VIP</span>
                    <span id="vip-address" class="value">-</span>
                </div>
                <div class="status-chip">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                    <span class="label">主控</span>
                    <span id="current-master" class="value">-</span>
                </div>
            </div>
        </header>

        <!-- Setup Progress Wizard -->
        <div id="setup-wizard" class="setup-wizard" style="display:none;">
            <div class="wizard-progress">
                <div class="wizard-step" data-step="1">
                    <div class="step-circle">1</div>
                    <div class="step-label">添加主路由</div>
                </div>
                <div class="wizard-connector"></div>
                <div class="wizard-step" data-step="2">
                    <div class="step-circle">2</div>
                    <div class="step-label">添加旁路由</div>
                </div>
                <div class="wizard-connector"></div>
                <div class="wizard-step" data-step="3">
                    <div class="step-circle">3</div>
                    <div class="step-label">配置 VIP</div>
                </div>
                <div class="wizard-connector"></div>
                <div class="wizard-step" data-step="4">
                    <div class="step-circle">4</div>
                    <div class="step-label">安装 Agent</div>
                </div>
            </div>
            <div class="wizard-hint" id="wizard-hint"></div>
        </div>

        <!-- Context-aware Action Card -->
        <div id="action-card" class="action-card" style="display:none;">
            <div class="action-card-icon" id="action-card-icon"></div>
            <div class="action-card-content">
                <div class="action-card-title" id="action-card-title"></div>
                <div class="action-card-desc" id="action-card-desc"></div>
            </div>
            <div class="action-card-actions" id="action-card-actions"></div>
            <button class="action-card-close" onclick="hideActionCard()">&times;</button>
        </div>

        <main>
            <section id="routers-section">
                <div class="section-header">
                    <h2>路由器管理</h2>
                    <div class="section-actions">
                        <button id="btn-refresh" class="btn btn-icon" title="刷新状态">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"/><path d="M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>
                        </button>
                        <button id="btn-verify-drift" class="btn btn-ghost" title="验证漂移">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                            验证漂移
                        </button>
                        <button id="btn-show-help" class="btn btn-ghost" title="功能说明">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                            使用说明
                        </button>
                        <button id="btn-check-update" class="btn btn-ghost" title="检查更新">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                            检查更新
                        </button>
                        <button id="btn-global-config" class="btn btn-ghost">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>
                            全局设置
                        </button>
                        <button id="btn-add-router" class="btn btn-primary">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                            添加路由器
                        </button>
                    </div>
                </div>
                <div id="routers-grid"></div>
            </section>

            <section id="logs-section">
                <h2>操作日志</h2>
                <div id="logs"></div>
            </section>
        </main>

        <!-- Verify Drift Modal -->
        <div id="modal-verify-drift" class="modal">
            <div class="modal-content modal-sm">
                <div class="modal-header">
                    <h3>验证网关漂移</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-verify-drift')">&times;</button>
                </div>
                <div class="modal-body">
                    <p style="margin-bottom: 1rem; color: var(--text-secondary);">
                        系统将模拟主节点故障（暂停 Keepalived），并验证 VIP 是否能自动漂移到备节点且保持网络连通。
                    </p>
                    <div id="drift-steps" class="drift-steps">
                        <!-- Steps will be inserted here -->
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-primary" id="btn-start-verify">开始验证</button>
                </div>
            </div>
        </div>

        <!-- Help Modal -->
        <div id="modal-help" class="modal">
            <div class="modal-content modal-lg">
                <div class="modal-header">
                    <h3>使用说明与功能指南</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-help')">&times;</button>
                </div>
                <div class="modal-body help-body">
                    <section class="help-section">
                        <h4>🚀 核心概念</h4>
                        <p>Floating Gateway 通过 <strong>Keepalived (VRRP)</strong> 技术，在多台路由器之间共享一个<strong>虚拟 IP (VIP)</strong>。客户端将网关设置为该 VIP，当首选路由器故障时，VIP 会自动漂移到备用路由器，实现无感切换。</p>
                    </section>

                    <section class="help-section">
                        <h4>🛠️ 关键操作按钮说明</h4>
                        <div class="help-grid">
                            <div class="help-card">
                                <div class="help-card-title">🔍 探测网络</div>
                                <p>在“添加/编辑”窗口中。点击后会通过 SSH 登录设备，自动识别网卡名称（如 <code>eth0</code>）和当前子网（如 <code>192.168.1.0/24</code>），避免手动输入错误。</p>
                            </div>
                            <div class="help-card">
                                <div class="help-card-title">🩺 诊断报告</div>
                                <p>执行实时健康检查。检查项包括：网卡状态、VIP 冲突、对端连通性、Keepalived 进程及配置安全审计。<strong>如果 VIP 无法漂移，请先看诊断报告。</strong></p>
                            </div>
                            <div class="help-card">
                                <div class="help-card-title">📦 重装/升级</div>
                                <p>一键完成：停止旧服务 -> 清理残留进程 -> 上传最新 Agent -> 重新生成 Keepalived 配置 -> 启动服务。适用于首次安装或代码更新后同步配置。</p>
                            </div>
                            <div class="help-card">
                                <div class="help-card-title">🔄 切换主备</div>
                                <p>仅在集群正常时显示。手动触发 VIP 漂移，常用于停机维护前主动转移流量。</p>
                            </div>
                        </div>
                    </section>

                    <section class="help-section">
                        <h4>📡 健康检查模式选择</h4>
                        <ul>
                            <li><strong>基础模式 (仅检测网关)</strong>：只要路由器系统没死、网线没断，就认为健康。<strong>建议主路由使用。</strong></li>
                            <li><strong>互联网模式 (检测外网)</strong>：同时探测阿里 DNS、腾讯 DNS 和 114 DNS。只要有 2 个不通，就触发切换。<strong>建议旁路由使用。</strong></li>
                        </ul>
                    </section>

                    <section class="help-section warning">
                        <h4>⚠️ 常见问题与注意事项</h4>
                        <ul>
                            <li><strong>实时状态</strong>：控制台每 5 秒自动同步一次集群状态。当发生 VIP 漂移时，系统会弹出黄色警告通知，并高亮显示当前新的主控设备。</li>
                            <li><strong>PVE/虚拟化环境</strong>：必须在 PVE 网卡设置中<strong>关闭防火墙</strong>或开启 <strong>IP Anti-Spoofing</strong>，否则 VRRP 组播包会被拦截，导致两台路由器都变成 MASTER（抢占冲突）。</li>
                            <li><strong>DHCP 选项 3</strong>：为了让全家设备自动生效，请在 OpenWrt 的 DHCP 选项中添加 <code>3,虚拟IP地址</code>。</li>
                            <li><strong>配置路径</strong>：Agent 会在 <code>/gateway-agent/</code> 下运行，请勿手动删除该目录。</li>
                        </ul>
                    </section>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-primary" onclick="closeModal('modal-help')">我知道了</button>
                </div>
            </div>
        </div>

        <!-- Doctor Report Modal -->
        <div id="modal-doctor" class="modal">
            <div class="modal-content modal-sm">
                <div class="modal-header">
                    <h3>诊断报告</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-doctor')">&times;</button>
                </div>
                <div class="modal-body">
                    <div id="doctor-report">
                        <div class="loading">正在获取报告...</div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-primary" onclick="closeModal('modal-doctor')">关闭</button>
                </div>
            </div>
        </div>

        <!-- Add Router Modal -->
        <div id="modal-add-router" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>添加路由器</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-add-router')">&times;</button>
                </div>
                <form id="form-add-router">
                    <div class="modal-body">
                        <div class="form-group">
                            <label>名称</label>
                            <input type="text" name="name" required placeholder="例如: openwrt-main">
                        </div>
                        <div class="form-group">
                            <label>主机地址 (IP)</label>
                            <input type="text" name="host" required placeholder="192.168.1.1">
                        </div>
                        <div class="form-row">
                            <div class="form-group">
                                <label>SSH 端口</label>
                                <input type="number" name="port" value="22">
                            </div>
                            <div class="form-group">
                                <label>SSH 用户</label>
                                <input type="text" name="user" required value="root">
                            </div>
                        </div>
                        <div class="form-group">
                            <label>SSH 密码</label>
                            <input type="password" name="password" placeholder="留空则使用密钥">
                        </div>
                        <div class="form-group">
                            <label>SSH 私钥文件路径</label>
                            <input type="text" name="key_file" placeholder="~/.ssh/id_rsa">
                        </div>
                        <div class="form-group" style="margin-top: 1rem;">
                            <button type="button" class="btn btn-sm btn-ghost" id="btn-router-probe" style="width: 100%; justify-content: center; border-style: dashed; gap: 0.6rem;">
                                <svg class="probe-icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
                                <span class="probe-text">测试 SSH 连接并探测网络环境</span>
                            </button>
                            <div id="probe-result" class="probe-result" style="display: none;"></div>
                            <small class="form-hint" style="text-align: center;">点击后将尝试使用上方填写的信息登录路由器并自动获取网卡与网段</small>
                        </div>
                        <div class="form-group">
                            <label>角色</label>
                            <select name="role" required>
                                <option value="primary">主路由 (Primary - 备用网关)</option>
                                <option value="secondary" selected>旁路由 (Secondary - 首选网关)</option>
                            </select>
                        </div>
                        <div class="form-row">
                            <div class="form-group">
                                <label>网卡接口</label>
                                <input type="text" name="iface" required placeholder="如 br-lan、eth0、ens18">
                                <small class="form-hint">点击上方"探测"按钮可自动获取</small>
                            </div>
                            <div class="form-group">
                                <label>健康检查模式</label>
                                <select name="health_mode">
                                    <option value="">使用全局设置</option>
                                    <option value="basic">基础模式 (仅检测网关)</option>
                                    <option value="internet">互联网模式 (检测外网)</option>
                                </select>
                                <small class="form-hint">留空则使用全局设置</small>
                            </div>
                        </div>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-ghost" onclick="closeModal('modal-add-router')">取消</button>
                        <button type="submit" class="btn btn-primary">确定添加</button>
                    </div>
                </form>
            </div>
        </div>

        <!-- Global Config Modal -->
        <div id="modal-global-config" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>全局设置</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-global-config')">&times;</button>
                </div>
                <form id="form-global-config">
                    <div class="modal-body">
                        <div class="form-group">
                            <label>虚拟 IP (VIP)</label>
                            <input type="text" name="vip" required placeholder="192.168.1.254">
                        </div>
                        <div class="form-group">
                            <label>网段 (CIDR)</label>
                            <div class="input-row">
                                <input type="text" name="cidr" required placeholder="192.168.1.0/24">
                                <button type="button" class="btn btn-sm btn-ghost" id="btn-detect-net">自动获取</button>
                            </div>
                            <small class="form-hint">点击"自动获取"检测本机网段</small>
                        </div>
                        <div class="form-row">
                            <div class="form-group">
                                <label>虚拟路由标识 (VRID)</label>
                                <input type="number" name="vrid" required value="51" min="1" max="255">
                            </div>
                            <div class="form-group">
                                <label>检测模式</label>
                                <select name="health_mode" required>
                                    <option value="internet">互联网模式 (检测外网)</option>
                                    <option value="basic">基础模式 (仅检测网关)</option>
                                </select>
                            </div>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-ghost" onclick="closeModal('modal-global-config')">取消</button>
                        <button type="submit" class="btn btn-primary">保存设置</button>
                    </div>
                </form>
            </div>
        </div>

        <!-- Version Update Modal -->
        <div id="modal-version" class="modal">
            <div class="modal-content modal-sm">
                <div class="modal-header">
                    <h3>版本信息</h3>
                    <button type="button" class="modal-close" onclick="closeModal('modal-version')">&times;</button>
                </div>
                <div class="modal-body">
                    <div id="version-info">
                        <div class="loading">正在检查更新...</div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-ghost" onclick="closeModal('modal-version')">关闭</button>
                    <a id="version-download-btn" href="#" target="_blank" class="btn btn-primary" style="display:none;">前往下载</a>
                </div>
            </div>
        </div>
    </div>
    <div class="toast-container" id="toast-container"></div>
    <script src="/app.js"></script>
</body>
</html>`

const styleCSS = `* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

:root {
    --bg: #0f1117;
    --bg-surface: #161b22;
    --bg-card: #1c2128;
    --bg-card-hover: #21262d;
    --bg-input: #0d1117;
    --bg-overlay: rgba(0, 0, 0, 0.6);
    --text: #e6edf3;
    --text-secondary: #8b949e;
    --text-muted: #6e7681;
    --primary: #58a6ff;
    --primary-hover: #79c0ff;
    --primary-bg: rgba(88, 166, 255, 0.1);
    --success: #3fb950;
    --success-bg: rgba(63, 185, 80, 0.1);
    --warning: #d29922;
    --warning-bg: rgba(210, 153, 34, 0.1);
    --danger: #f85149;
    --danger-bg: rgba(248, 81, 73, 0.1);
    --border: #30363d;
    --border-light: #21262d;
    --radius: 8px;
    --radius-lg: 12px;
    --shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    --shadow-lg: 0 8px 32px rgba(0, 0, 0, 0.4);
    --transition: 0.2s ease;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
    background: var(--bg);
    color: var(--text);
    min-height: 100vh;
    line-height: 1.5;
    font-size: 14px;
}

/* Header */
header {
    background: var(--bg-surface);
    padding: 0 1.5rem;
    height: 56px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    z-index: 100;
    backdrop-filter: blur(12px);
}

.header-left {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}

.logo-icon {
    color: var(--primary);
}

header h1 {
    font-size: 1.1rem;
    font-weight: 600;
    letter-spacing: -0.01em;
}

.version-tag {
    font-size: 0.7rem;
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    color: var(--text-muted);
    background: var(--bg-card);
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    margin-left: 0.2rem;
    align-self: flex-end;
    margin-bottom: 0.2rem;
}

#vip-status {
    display: flex;
    gap: 0.75rem;
    align-items: center;
}

.status-chip {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.75rem;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 20px;
    font-size: 0.8rem;
}

.status-chip .label {
    color: var(--text-muted);
    font-weight: 500;
}

.status-chip .value {
    color: var(--text);
    font-weight: 600;
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
}

.status-chip svg {
    color: var(--text-muted);
}

/* Main */
main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1.5rem;
}

/* Sections */
section {
    margin-bottom: 1.5rem;
}

.section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
    gap: 0.75rem;
}

.section-header h2 {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--text);
}

.section-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

/* Buttons */
.btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 1rem;
    font-size: 0.85rem;
    font-weight: 500;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
    color: var(--text);
    cursor: pointer;
    transition: all var(--transition);
    white-space: nowrap;
    line-height: 1.4;
    font-family: inherit;
}

.btn:hover {
    background: var(--bg-card-hover);
    border-color: var(--text-muted);
}

.btn:active {
    transform: scale(0.97);
}

.btn-primary {
    background: var(--primary);
    color: #fff;
    border-color: var(--primary);
}

.btn-primary:hover {
    background: var(--primary-hover);
    border-color: var(--primary-hover);
}

.btn-ghost {
    background: transparent;
    border-color: transparent;
    color: var(--text-secondary);
}

.btn-ghost:hover {
    background: var(--bg-card);
    border-color: var(--border);
    color: var(--text);
}

.btn-danger {
    color: var(--danger);
    border-color: var(--border);
    background: transparent;
}

.btn-danger:hover {
    background: var(--danger-bg);
    border-color: var(--danger);
}

.btn-sm {
    padding: 0.3rem 0.65rem;
    font-size: 0.78rem;
}

.btn-icon {
    padding: 0.45rem;
    border-color: transparent;
    background: transparent;
    color: var(--text-secondary);
}

.btn-icon:hover {
    background: var(--bg-card);
    color: var(--text);
}

.btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

/* Router Grid */
#routers-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: 0.75rem;
}

/* Router Card */
.router-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    transition: all var(--transition);
}

.router-card:hover {
    border-color: var(--text-muted);
    box-shadow: var(--shadow);
}

.router-card.master {
    border-color: var(--primary);
    box-shadow: 0 0 0 1px var(--primary), 0 0 20px rgba(88, 166, 255, 0.08);
}

.router-card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1rem;
}

.router-name {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
    margin-bottom: 0.25rem;
}

.router-role {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 500;
    padding: 0.15rem 0.5rem;
    border-radius: 10px;
    text-transform: uppercase;
    letter-spacing: 0.03em;
}

.router-role.primary {
    background: var(--primary-bg);
    color: var(--primary);
}

.router-role.secondary {
    background: var(--warning-bg);
    color: var(--warning);
}

.status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.78rem;
    font-weight: 500;
    padding: 0.25rem 0.65rem;
    border-radius: 10px;
}

.status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    display: inline-block;
}

.status-badge.online { background: var(--success-bg); color: var(--success); }
.status-badge.online .status-dot { background: var(--success); box-shadow: 0 0 6px var(--success); }
.status-badge.offline { background: var(--danger-bg); color: var(--danger); }
.status-badge.offline .status-dot { background: var(--danger); }
.status-badge.installing, .status-badge.uninstalling { background: var(--warning-bg); color: var(--warning); }
.status-badge.installing .status-dot, .status-badge.uninstalling .status-dot { background: var(--warning); animation: pulse 1.5s infinite; }
.status-badge.unknown, .status-badge.error { background: rgba(110,118,129,0.1); color: var(--text-muted); }
.status-badge.unknown .status-dot, .status-badge.error .status-dot { background: var(--text-muted); }

@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
}

@keyframes pulse-drift {
    0% { transform: scale(1); box-shadow: 0 0 0 0 rgba(210, 153, 34, 0.7); }
    70% { transform: scale(1.05); box-shadow: 0 0 0 10px rgba(210, 153, 34, 0); }
    100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(210, 153, 34, 0); }
}

.router-info {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem 1rem;
    margin-bottom: 1rem;
    font-size: 0.82rem;
}

.router-info .label {
    color: var(--text-muted);
}

.router-info .value {
    color: var(--text-secondary);
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    font-size: 0.8rem;
}

.router-info .value.outdated {
    color: var(--warning);
    font-weight: 600;
}

.vrrp-state {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 600;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

.vrrp-state.master { background: var(--primary-bg); color: var(--primary); }
.vrrp-state.backup { background: rgba(110,118,129,0.15); color: var(--text-secondary); }
.vrrp-state.fault  { background: var(--danger-bg); color: var(--danger); }

.health-indicator { font-size: 0.8rem; font-weight: 500; }
.health-indicator.healthy { color: var(--success); }
.health-indicator.unhealthy { color: var(--danger); }

.router-actions {
    display: flex;
    gap: 0.4rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border-light);
    flex-wrap: wrap;
}

/* Installation Progress */
.install-progress {
    margin-top: 0.75rem;
    padding: 0.75rem;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    font-size: 0.75rem;
}

.install-progress-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
    color: var(--text-secondary);
    font-weight: 600;
}

.install-progress-bar {
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    margin-bottom: 0.5rem;
    overflow: hidden;
}

.install-progress-fill {
    height: 100%;
    background: var(--primary);
    border-radius: 2px;
    transition: width 0.4s ease;
}

.install-log-list {
    max-height: 120px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
}

.install-log-item {
    color: var(--text-muted);
    white-space: pre-wrap;
}

.install-log-item:last-child {
    color: var(--primary);
    font-weight: 500;
}

/* Setup Guide (empty state) */
.setup-guide {
    background: var(--bg-card);
    border: 1px dashed var(--border);
    border-radius: var(--radius-lg);
    padding: 2.5rem 2rem;
    max-width: 520px;
    margin: 2rem auto;
    text-align: center;
}

.setup-guide .guide-icon {
    width: 48px;
    height: 48px;
    margin: 0 auto 1rem;
    color: var(--text-muted);
}

.setup-guide h3 {
    margin-bottom: 0.75rem;
    color: var(--text);
    font-size: 1.1rem;
    font-weight: 600;
}

.setup-guide p {
    color: var(--text-secondary);
    font-size: 0.85rem;
    margin-bottom: 0.5rem;
}

.setup-guide ol {
    text-align: left;
    margin: 1rem 0 1rem 1.5rem;
    color: var(--text-secondary);
    font-size: 0.85rem;
}

.setup-guide li {
    margin-bottom: 0.5rem;
    line-height: 1.6;
}

.setup-guide .hint {
    margin-top: 1rem;
    font-size: 0.78rem;
    color: var(--text-muted);
    padding: 0.75rem;
    background: var(--bg);
    border-radius: var(--radius);
}

/* Logs */
#logs-section h2 {
    font-size: 1.1rem;
    font-weight: 600;
    margin-bottom: 0.75rem;
}

#logs {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 0.75rem;
    max-height: 220px;
    overflow-y: auto;
}

.log-entry {
    padding: 0.35rem 0.5rem;
    font-size: 0.8rem;
    border-radius: 4px;
    color: var(--text-secondary);
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
}

.log-entry:hover {
    background: var(--bg-card-hover);
}

.log-entry .log-time {
    color: var(--text-muted);
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    font-size: 0.75rem;
    flex-shrink: 0;
}

.log-entry.error { color: var(--danger); }
.log-entry.success { color: var(--success); }

/* Loading */
.loading {
    padding: 2rem;
    text-align: center;
    color: var(--text-muted);
    font-size: 0.85rem;
}

/* Modals */
.modal {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--bg-overlay);
    z-index: 1000;
    align-items: center;
    justify-content: center;
    backdrop-filter: blur(4px);
}

.modal.active {
    display: flex;
}

.modal-content {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    width: 90%;
    max-width: 480px;
    max-height: 85vh;
    overflow-y: auto;
    box-shadow: var(--shadow-lg);
    animation: modalIn 0.2s ease;
}

.modal-content.modal-sm { max-width: 560px; }
.modal-content.modal-lg { max-width: 800px; }

/* Help Content Styles */
.help-body {
    padding: 1.5rem;
    max-height: 70vh;
    overflow-y: auto;
}

.help-section {
    margin-bottom: 2rem;
}

.help-section h4 {
    color: var(--primary);
    margin-bottom: 0.75rem;
    font-size: 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.help-section p {
    color: var(--text-secondary);
    font-size: 0.9rem;
    line-height: 1.6;
}

.help-section ul {
    padding-left: 1.25rem;
    color: var(--text-secondary);
}

.help-section li {
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
    line-height: 1.5;
}

.help-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1rem;
    margin-top: 1rem;
}

.help-card {
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 1rem;
}

.help-card-title {
    font-weight: 600;
    color: var(--text);
    font-size: 0.85rem;
    margin-bottom: 0.5rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
}

.help-card p {
    font-size: 0.8rem;
    margin: 0;
}

.help-section.warning {
    background: var(--warning-bg);
    border-radius: var(--radius);
    padding: 1rem;
    border: 1px solid rgba(210, 153, 34, 0.2);
}

.help-section.warning h4 {
    color: var(--warning);
}

@keyframes modalIn {
    from { opacity: 0; transform: scale(0.95) translateY(8px); }
    to   { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--border);
}

.modal-header h3 {
    font-size: 1rem;
    font-weight: 600;
}

.modal-close {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 1.3rem;
    cursor: pointer;
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
    line-height: 1;
    transition: all var(--transition);
}

.modal-close:hover {
    background: var(--bg-card-hover);
    color: var(--text);
}

.modal-body {
    padding: 1.25rem;
}

.modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    padding: 1rem 1.25rem;
    border-top: 1px solid var(--border);
}

/* Forms */
.form-group {
    margin-bottom: 1rem;
}

.form-group:last-child {
    margin-bottom: 0;
}

.form-group label {
    display: block;
    font-size: 0.82rem;
    font-weight: 500;
    color: var(--text-secondary);
    margin-bottom: 0.35rem;
}

.form-hint {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-top: 0.3rem;
}

.form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
}

.input-row {
    display: flex;
    gap: 0.5rem;
}

.input-row input {
    flex: 1;
}

input[type="text"],
input[type="number"],
input[type="password"],
select {
    width: 100%;
    padding: 0.55rem 0.75rem;
    font-size: 0.85rem;
    font-family: inherit;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text);
    transition: border-color var(--transition);
    outline: none;
}

input:focus,
select:focus {
    border-color: var(--primary);
    box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.15);
}

input::placeholder {
    color: var(--text-muted);
}

select {
    cursor: pointer;
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%236e7681' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 0.75rem center;
    padding-right: 2rem;
}

/* Drift Verification Styles */
.drift-steps {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}

.drift-step {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem;
    border-radius: var(--radius);
    background: var(--bg-input);
    border: 1px solid var(--border);
    transition: all 0.3s ease;
}

.drift-step.running {
    border-color: var(--primary);
    background: rgba(59, 130, 246, 0.1);
}

.drift-step.success {
    border-color: var(--success);
    background: rgba(16, 185, 129, 0.1);
}

.drift-step.error {
    border-color: var(--danger);
    background: rgba(239, 68, 68, 0.1);
}

.step-icon {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.step-content {
    flex: 1;
}

.step-title {
    font-weight: 600;
    font-size: 0.9rem;
    margin-bottom: 0.2rem;
}

.step-desc {
    font-size: 0.8rem;
    color: var(--text-secondary);
}

/* Spinner for running state */
.spinner {
    width: 16px;
    height: 16px;
    border: 2px solid var(--primary);
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

/* Doctor report */
.doctor-item {
    padding: 0.65rem 0.75rem;
    border-bottom: 1px solid var(--border-light);
}

.doctor-item:last-child {
    border-bottom: none;
}

.doctor-item-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.2rem;
}

.doctor-item-name {
    font-weight: 500;
    font-size: 0.85rem;
}

.doctor-status {
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    text-transform: uppercase;
}

.doctor-status.ok { background: var(--success-bg); color: var(--success); }
/* Doctor report */
.doctor-report {
    background: var(--bg-console);
    color: #e2e8f0;
    padding: 1rem;
    border-radius: var(--radius);
    font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
    font-size: 0.85rem;
    white-space: pre-wrap;
    max-height: 60vh;
    overflow-y: auto;
    border: 1px solid var(--border);
    line-height: 1.5;
}

.doctor-report-item {
    margin-bottom: 0.5rem;
    display: flex;
    gap: 0.5rem;
}

.doctor-status-ok { color: var(--success); }
.doctor-status-warn { color: var(--warning); }
.doctor-status-error { color: var(--danger); }
.doctor-fixed { color: var(--primary); margin-left: 0.5rem; font-size: 0.8rem; }
.doctor-meta { color: #64748b; font-size: 0.8rem; margin-top: 1rem; border-top: 1px solid #334155; padding-top: 0.5rem; }

.doctor-status.warning { background: var(--warning-bg); color: var(--warning); }
.doctor-status.error { background: var(--danger-bg); color: var(--danger); }

.doctor-message {
    color: var(--text-muted);
    font-size: 0.78rem;
}

.doctor-summary {
    margin-top: 0.75rem;
    padding: 0.75rem;
    background: var(--bg-input);
    border-radius: var(--radius);
    font-weight: 600;
    font-size: 0.85rem;
    text-align: center;
}

/* Scrollbar */
::-webkit-scrollbar {
    width: 6px;
}

::-webkit-scrollbar-track {
    background: transparent;
}

::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
}

/* Responsive */
@media (max-width: 640px) {
    header {
        flex-direction: column;
        height: auto;
        padding: 0.75rem 1rem;
        gap: 0.5rem;
    }

    #vip-status {
        flex-wrap: wrap;
        justify-content: center;
    }

    main {
        padding: 1rem;
    }

    .section-header {
        flex-wrap: wrap;
    }

    .section-actions {
        flex-wrap: wrap;
    }

    #routers-grid {
        grid-template-columns: 1fr;
    }

    .router-info {
        grid-template-columns: 1fr;
    }

    .form-row {
        grid-template-columns: 1fr;
    }

    .modal-content {
        width: 95%;
        max-height: 90vh;
    }
}

/* Setup Wizard */
.setup-wizard {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
    margin: 0 auto 1rem;
    max-width: 1200px;
}

.wizard-progress {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    margin-bottom: 0.75rem;
}

.wizard-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
    min-width: 90px;
}

.step-circle {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--bg-input);
    border: 2px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-muted);
    transition: all var(--transition);
}

.wizard-step.active .step-circle {
    background: var(--primary);
    border-color: var(--primary);
    color: #fff;
    box-shadow: 0 0 12px rgba(88, 166, 255, 0.4);
}

.wizard-step.completed .step-circle {
    background: var(--success);
    border-color: var(--success);
    color: #fff;
}

.wizard-step.completed .step-circle::after {
    content: '\u2713';
}

.step-label {
    font-size: 0.75rem;
    color: var(--text-muted);
    text-align: center;
    transition: color var(--transition);
}

.wizard-step.active .step-label {
    color: var(--primary);
    font-weight: 500;
}

.wizard-step.completed .step-label {
    color: var(--success);
}

.wizard-connector {
    flex: 1;
    max-width: 80px;
    height: 2px;
    background: var(--border);
    margin: 0 0.5rem;
    margin-bottom: 1.5rem;
}

.wizard-hint {
    text-align: center;
    font-size: 0.85rem;
    color: var(--text-secondary);
    padding-top: 0.5rem;
    border-top: 1px solid var(--border-light);
}

/* Action Card */
.action-card {
    background: linear-gradient(135deg, var(--bg-card) 0%, var(--bg-surface) 100%);
    border: 1px solid var(--primary);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
    margin: 0 auto 1.5rem;
    max-width: 1200px;
    display: flex;
    align-items: center;
    gap: 1rem;
    position: relative;
    box-shadow: 0 0 20px rgba(88, 166, 255, 0.08);
}

.action-card-icon {
    flex-shrink: 0;
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-input);
    border-radius: 12px;
}

.action-card-content {
    flex: 1;
}

.action-card-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text);
    margin-bottom: 0.25rem;
}

.action-card-desc {
    font-size: 0.85rem;
    color: var(--text-secondary);
}

.action-card-actions {
    display: flex;
    gap: 0.5rem;
    flex-shrink: 0;
}

.action-card-close {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 1.2rem;
    cursor: pointer;
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
    line-height: 1;
    transition: all var(--transition);
}

.action-card-close:hover {
    background: var(--bg-card-hover);
    color: var(--text);
}

/* Toast notifications */
.toast-container {
    position: fixed;
    bottom: 1.5rem;
    right: 1.5rem;
    z-index: 2000;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.toast {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.75rem 1rem;
    font-size: 0.85rem;
    box-shadow: var(--shadow-lg);
    animation: toastIn 0.3s ease;
    max-width: 320px;
}

.toast.success { border-color: var(--success); }
.toast.error { border-color: var(--danger); }
.toast.warning { border-color: var(--warning); }

@keyframes toastIn {
    from { opacity: 0; transform: translateX(20px); }
    to { opacity: 1; transform: translateX(0); }
}

/* Probe result in modal */
.probe-result {
    margin-top: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: var(--radius);
    font-size: 0.85rem;
    animation: fadeIn 0.3s ease;
}

.probe-result.loading {
    background: var(--bg-surface);
    border: 1px dashed var(--border);
    color: var(--text-muted);
    text-align: center;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
}

.probe-result.success {
    background: rgba(16, 185, 129, 0.1);
    border: 1px solid var(--success);
}

.probe-result.success .probe-header {
    color: var(--success);
    font-weight: 600;
    margin-bottom: 0.5rem;
}

.probe-result.error {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid var(--danger);
    color: var(--danger);
}

.probe-result.warning {
    background: rgba(210, 153, 34, 0.1);
    border: 1px solid var(--warning);
    color: var(--warning);
}

.probe-result .probe-detail {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-top: 0.5rem;
    font-size: 0.82rem;
    color: var(--text);
    background: var(--bg-card);
    padding: 0.6rem 0.75rem;
    border-radius: var(--radius);
}

.probe-result .probe-detail span {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.probe-result .probe-detail .probe-label {
    color: var(--text-muted);
    font-size: 0.78rem;
}

.probe-result .probe-detail code {
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    background: var(--bg-input);
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    color: var(--primary);
    font-weight: 500;
}

.probe-result .probe-hint {
    margin-top: 0.5rem;
    font-size: 0.78rem;
    color: var(--text-secondary);
    padding: 0.5rem;
    background: var(--bg-card);
    border-radius: var(--radius);
}

.probe-spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid var(--border);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

/* Responsive for wizard */
@media (max-width: 640px) {
    .wizard-progress {
        flex-wrap: wrap;
    }
    
    .wizard-connector {
        display: none;
    }
    
    .wizard-step {
        min-width: 70px;
    }
    
    .action-card {
        flex-direction: column;
        text-align: center;
    }
    
    .action-card-actions {
        width: 100%;
        justify-content: center;
    }
}

/* Version info styles */
.version-info-container {
    text-align: center;
}

.version-current {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    margin-bottom: 1rem;
    padding: 1rem;
    background: var(--bg-input);
    border-radius: var(--radius);
}

.version-current .version-label {
    color: var(--text-muted);
    font-size: 0.85rem;
}

.version-current .version-value {
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--primary);
}

.version-status {
    padding: 1rem;
    border-radius: var(--radius);
    margin-bottom: 1rem;
}

.version-status.up-to-date {
    background: var(--success-bg);
    border: 1px solid var(--success);
}

.version-status.up-to-date .status-icon {
    color: var(--success);
    font-size: 2rem;
    margin-bottom: 0.5rem;
}

.version-status.up-to-date .status-text {
    color: var(--success);
    font-weight: 600;
}

.version-status.has-update {
    background: var(--warning-bg);
    border: 1px solid var(--warning);
}

.version-status.has-update .status-icon {
    color: var(--warning);
    font-size: 2rem;
    margin-bottom: 0.5rem;
}

.version-status.has-update .status-text {
    color: var(--warning);
    font-weight: 600;
}

.version-status.has-update .new-version {
    margin-top: 0.5rem;
    font-size: 0.9rem;
    color: var(--text);
}

.version-status.has-update .new-version code {
    font-family: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
    background: var(--bg-card);
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    color: var(--warning);
    font-weight: 600;
}

.version-notes {
    text-align: left;
    padding: 0.75rem;
    background: var(--bg-card);
    border-radius: var(--radius);
    font-size: 0.82rem;
    color: var(--text-secondary);
    max-height: 150px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-word;
}

.version-notes-title {
    font-size: 0.78rem;
    color: var(--text-muted);
    margin-bottom: 0.5rem;
    text-align: left;
}

.version-error {
    padding: 1rem;
    background: var(--danger-bg);
    border: 1px solid var(--danger);
    border-radius: var(--radius);
    color: var(--danger);
    font-size: 0.85rem;
}`

const appJS = `// Gateway Controller UI
const API_BASE = '/api';
let routers = [];
let globalConfig = null;

// Utility functions
function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

function formatTime(date) {
    return new Date(date).toLocaleTimeString();
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function log(msg, type = 'info') {
    const logs = $('#logs');
    const entry = document.createElement('div');
    entry.className = 'log-entry ' + type;
    entry.innerHTML = '<span class="log-time">' + formatTime(new Date()) + '</span>' + msg;
    logs.insertBefore(entry, logs.firstChild);
    
    // Keep only last 50 entries
    while (logs.children.length > 50) {
        logs.removeChild(logs.lastChild);
    }
}

// Toast notifications
function showToast(msg, type = 'info', duration = 3500) {
    const container = $('#toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast ' + type;
    toast.textContent = msg;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(20px)';
        toast.style.transition = 'all 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

// ============ Setup Wizard Logic ============
function getSetupStep() {
    const hasPrimary = routers.some(r => r.role === 'primary');
    const hasSecondary = routers.some(r => r.role === 'secondary');
    const hasVIP = globalConfig && globalConfig.lan && globalConfig.lan.vip;
    const allInstalled = routers.length >= 2 && routers.every(r => r.agent_version);
    
    if (!hasPrimary) return 1;
    if (!hasSecondary) return 2;
    if (!hasVIP) return 3;
    if (!allInstalled) return 4;
    return 0; // Setup complete
}

function updateWizard() {
    const wizard = $('#setup-wizard');
    const step = getSetupStep();
    
    if (step === 0) {
        wizard.style.display = 'none';
        hideActionCard();
        return;
    }
    
    wizard.style.display = 'block';
    
    // Update step circles
    $$('.wizard-step').forEach((el, idx) => {
        const stepNum = idx + 1;
        el.classList.remove('active', 'completed');
        if (stepNum < step) {
            el.classList.add('completed');
        } else if (stepNum === step) {
            el.classList.add('active');
        }
    });
    
    // Update hint text
    const hints = {
        1: '请点击「添加路由器」添加一台主路由器（Primary），这将作为故障时的备用网关',
        2: '请添加一台旁路由器（Secondary），这将作为默认首选网关',
        3: '请点击「全局设置」配置虚拟 IP (VIP) 地址',
        4: '配置已完成，点击下方按钮一键安装所有 Agent'
    };
    $('#wizard-hint').textContent = hints[step] || '';
    
    // Show action card with context-aware suggestion
    showActionCardForStep(step);
}

function showActionCardForStep(step) {
    const card = $('#action-card');
    const iconEl = $('#action-card-icon');
    const titleEl = $('#action-card-title');
    const descEl = $('#action-card-desc');
    const actionsEl = $('#action-card-actions');
    
    card.style.display = 'flex';
    
    const icons = {
        1: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="var(--primary)" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>',
        2: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="var(--warning)" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>',
        3: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="var(--success)" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 8v4l2 2"/></svg>',
        4: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="var(--success)" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22,4 12,14.01 9,11.01"/></svg>'
    };
    
    const configs = {
        1: {
            title: '第 1 步：添加主路由器',
            desc: '主路由器（Primary）将作为故障时的备用网关，优先级较低',
            actions: '<button class="btn btn-primary" onclick="openAddRouterWithRole(\'primary\')">添加主路由器</button>'
        },
        2: {
            title: '第 2 步：添加旁路由器',
            desc: '旁路由器（Secondary）将作为默认首选网关，优先级较高',
            actions: '<button class="btn btn-primary" onclick="openAddRouterWithRole(\'secondary\')">添加旁路由器</button>'
        },
        3: {
            title: '第 3 步：配置虚拟 IP',
            desc: '设置 VIP 地址，这是客户端实际使用的网关地址',
            actions: '<button class="btn btn-primary" onclick="openGlobalConfigWithSuggestion()">配置 VIP</button>'
        },
        4: {
            title: '准备就绪',
            desc: '所有配置已完成，点击按钮一键在所有路由器上安装 Agent',
            actions: '<button class="btn btn-primary" onclick="installAll()">一键安装所有 Agent</button>'
        }
    };
    
    const cfg = configs[step];
    iconEl.innerHTML = icons[step];
    titleEl.textContent = cfg.title;
    descEl.textContent = cfg.desc;
    actionsEl.innerHTML = cfg.actions;
}

function hideActionCard() {
    $('#action-card').style.display = 'none';
}

function openAddRouterWithRole(role) {
    const form = $('#form-add-router');
    form.reset();
    form.role.value = role;
    // Reset probe result area
    const probeResult = $('#probe-result');
    probeResult.style.display = 'none';
    probeResult.className = 'probe-result';
    probeResult.innerHTML = '';
    openModal('modal-add-router');
}

async function openGlobalConfigWithSuggestion() {
    try {
        const cfg = await apiCall('/config');
        const form = $('#form-global-config');
        form.vip.value = cfg.lan.vip || '';
        form.cidr.value = cfg.lan.cidr || '';
        form.iface.value = cfg.lan.iface || '';
        form.vrid.value = cfg.keepalived.vrid || 51;
        form.health_mode.value = cfg.health.mode || 'internet';
        
        // Auto detect if missing
        if (!cfg.lan.cidr || !cfg.lan.iface) {
            openModal('modal-global-config');
            // Trigger auto-detect
            setTimeout(() => $('#btn-detect-net').click(), 300);
        } else {
            openModal('modal-global-config');
        }
    } catch (e) {
        log('获取配置失败: ' + e.message, 'error');
    }
}

async function installAll() {
    if (!confirm('确定要在所有路由器上安装 gateway-agent 吗？')) return;
    
    log('正在所有路由器上安装 Agent...');
    try {
        await apiCall('/routers/install-all', { method: 'POST' });
        log('已开始批量安装', 'success');
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshStatus();
    } catch (e) {
        log('批量安装失败: ' + e.message, 'error');
        showToast('安装失败: ' + e.message, 'error');
    }
}

// API functions
async function apiCall(endpoint, options = {}) {
    try {
        const resp = await fetch(API_BASE + endpoint, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            }
        });
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error || resp.statusText);
        }
        if (resp.status === 204) return null;
        return await resp.json();
    } catch (e) {
        log('接口错误: ' + e.message, 'error');
        throw e;
    }
}

// Status update
let refreshTimer = null;
let previousRouterStates = {}; // Track previous states to detect changes
let previousMaster = null;     // Track previous master to detect drift
let controllerVersion = 'dev';

function normalizeVersion(v) {
    if (!v) return '';
    return v.replace('gateway-agent', '').replace('v', '').trim();
}

async function refreshStatus() {
    try {
        const [status, cfg, versionInfo] = await Promise.all([
            apiCall('/status'),
            apiCall('/config'),
            apiCall('/version').catch(() => ({ current_version: 'dev' }))
        ]);
        
        globalConfig = cfg;
        controllerVersion = versionInfo.current_version;
        $('#controller-version').textContent = controllerVersion.startsWith('v') ? controllerVersion : 'v' + controllerVersion;
        
        $('#vip-address').textContent = status.vip || '-';
        $('#current-master').textContent = status.current_master || '无';
        
        // Detect Master Drift
        if (previousMaster !== null && status.current_master && previousMaster !== status.current_master) {
            const msg = '🔄 发生 VIP 漂移：' + (previousMaster || '未知') + ' ➜ ' + status.current_master;
            showToast(msg, 'warning', 8000);
            log(msg, 'warning');
            
            // Add a temporary visual flash effect to the header chip
            const chip = $('#current-master').parentElement;
            chip.style.animation = 'pulse-drift 2s infinite';
            setTimeout(() => { chip.style.animation = ''; }, 6000);
        }
        previousMaster = status.current_master;
        
        const newRouters = status.routers || [];
        
        // Detect state changes and show notifications
        newRouters.forEach(router => {
            const prevState = previousRouterStates[router.name];
            const currState = router.status;
            
            if (prevState && prevState !== currState) {
                // State changed - check for completion
                if (prevState === 'installing' && currState === 'online') {
                    showToast('✓ ' + router.name + ' Agent 安装成功！', 'success', 5000);
                    log(router.name + ' Agent 安装成功', 'success');
                } else if (prevState === 'installing' && currState === 'error') {
                    showToast('✗ ' + router.name + ' 安装失败: ' + (router.error || '未知错误'), 'error', 6000);
                    log(router.name + ' 安装失败: ' + (router.error || '未知错误'), 'error');
                } else if (prevState === 'uninstalling' && currState === 'online') {
                    showToast('✓ ' + router.name + ' Agent 已卸载', 'success', 5000);
                    log(router.name + ' Agent 卸载成功', 'success');
                } else if (prevState === 'uninstalling' && currState === 'error') {
                    showToast('✗ ' + router.name + ' 卸载失败: ' + (router.error || '未知错误'), 'error', 6000);
                    log(router.name + ' 卸载失败: ' + (router.error || '未知错误'), 'error');
                }
            }
            
            // Update previous state
            previousRouterStates[router.name] = currState;
        });
        
        routers = newRouters;
        renderRouters();
        updateWizard();

        // If any router is installing/uninstalling, poll faster (every 2s)
        // Otherwise poll every 5s for near-realtime drift detection
        const isBusy = routers.some(r => r.status === 'installing' || r.status === 'uninstalling');
        const interval = isBusy ? 2000 : 5000;
        
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshTimer = setTimeout(refreshStatus, interval);
    } catch (e) {
        console.error('刷新状态失败:', e);
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshTimer = setTimeout(refreshStatus, 30000);
    }
}

// Render routers
function renderRouters() {
    const grid = $('#routers-grid');
    grid.innerHTML = '';
    
    routers.forEach(router => {
        const card = document.createElement('div');
        card.className = 'router-card';
        if (router.vrrp_state === 'MASTER') {
            card.classList.add('master');
        }
        
        const statusClass = router.status || 'unknown';
        const roleClass = router.role || 'unknown';
        
        let healthHtml = '';
        if (router.healthy !== undefined && router.healthy !== null) {
            const healthClass = router.healthy ? 'healthy' : 'unhealthy';
            const healthIcon = router.healthy ? '✓' : '✗';
            const healthText = router.healthy ? '健康' : '异常';
            healthHtml = '<span class="health-indicator ' + healthClass + '">' + healthIcon + ' ' + healthText + '</span>';
        }
        
        let vrrpHtml = '';
        if (router.vrrp_state) {
            const vrrpClass = router.vrrp_state.toLowerCase();
            vrrpHtml = '<span class="vrrp-state ' + vrrpClass + '">' + router.vrrp_state + '</span>';
        }
        
        const roleText = router.role === 'primary' ? '主路由' : '旁路由';
        const statusTextMap = {
            'online': '在线',
            'offline': '离线',
            'installing': '正在安装',
            'uninstalling': '正在卸载',
            'unknown': '未知',
            'error': '错误'
        };
        const statusText = statusTextMap[statusClass] || statusClass;
        
        const normAgentVer = normalizeVersion(router.agent_version);
        const normCtrlVer = normalizeVersion(controllerVersion);
        const isOutdated = normAgentVer && normCtrlVer !== 'dev' && normAgentVer !== normCtrlVer;
        
        const displayAgentVer = router.agent_version ? router.agent_version.replace('gateway-agent ', '') : '未安装';
        const agentVerHtml = isOutdated 
            ? '<span class="value outdated" title="版本与控制端不一致 (' + controllerVersion + ')，建议重新安装">' + displayAgentVer + ' ⚠</span>'
            : '<span class="value">' + displayAgentVer + '</span>';
        
        let progressHtml = '';
         const showProgress = statusClass === 'installing' || statusClass === 'uninstalling' || (statusClass === 'error' && router.install_log && router.install_log.length > 0);
         if (showProgress) {
             const step = router.install_step || 0;
             const total = router.install_total || 1;
             const pct = Math.round((step / total) * 100);
             const hasLogs = router.install_log && router.install_log.length > 0;
             const logs = hasLogs ? router.install_log.map(line => '<div class="install-log-item">' + line + '</div>').join('') : '<div class="install-log-item" style="color:var(--warning)">正在准备...</div>';
             const actionText = statusClass === 'uninstalling' ? '卸载' : '安装';
             progressHtml = 
                 '<div class="install-progress">' +
                     '<div class="install-progress-header">' +
                         '<span>' + actionText + '进度 ' + step + '/' + total + '</span>' +
                         (statusClass === 'error' ? '<span style="color:var(--danger)">失败</span>' : '<span class="loading-dots">...</span>') +
                     '</div>' +
                     '<div class="install-progress-bar"><div class="install-progress-fill" style="width:' + pct + '%;' + (statusClass === 'error' ? 'background:var(--danger)' : '') + '"></div></div>' +
                     '<div class="install-log-list" id="log-list-' + router.name + '">' +
                         logs +
                     '</div>' +
                 '</div>';
         }

        card.innerHTML = 
            '<div class="router-card-header">' +
                '<div>' +
                    '<div class="router-name">' + router.name + '</div>' +
                    '<span class="router-role ' + roleClass + '">' + roleText + '</span>' +
                '</div>' +
                '<span class="status-badge ' + statusClass + '">' +
                    '<span class="status-dot"></span>' +
                    statusText +
                '</span>' +
            '</div>' +
            '<div class="router-info">' +
                '<div><span class="label">主机:</span> <span class="value">' + router.host + ':' + router.port + '</span></div>' +
                '<div><span class="label">系统:</span> <span class="value">' + (router.platform || '-') + '</span></div>' +
                '<div><span class="label">网卡:</span> <span class="value">' + (router.iface || '使用全局') + '</span></div>' +
                '<div><span class="label">健康模式:</span> <span class="value">' + (router.health_mode || '使用全局') + '</span></div>' +
                '<div><span class="label">Agent:</span> ' + agentVerHtml + '</div>' +
                '<div><span class="label">VRRP状态:</span> ' + (vrrpHtml || '<span class="value">-</span>') + '</div>' +
                '<div><span class="label">健康状态:</span> ' + (healthHtml || '<span class="value">-</span>') + '</div>' +
            '</div>' +
            progressHtml +
            '<div class="router-actions">' +
                '<button class="btn btn-sm" onclick="probeRouter(\'' + router.name + '\')">探测</button>' +
                (router.agent_version 
                    ? '<button class="btn btn-sm" onclick="showDoctor(\'' + router.name + '\')">诊断</button>' +
                      '<button class="btn btn-sm btn-primary" onclick="installRouter(\'' + router.name + '\', true)" ' + (statusClass === 'installing' ? 'disabled' : '') + '>重装/升级</button>' +
                      '<button class="btn btn-sm btn-danger" onclick="uninstallRouter(\'' + router.name + '\')" ' + (statusClass === 'uninstalling' ? 'disabled' : '') + '>卸载</button>'
                    : '<button class="btn btn-sm btn-primary" onclick="installRouter(\'' + router.name + '\')" ' + (statusClass === 'installing' ? 'disabled' : '') + '>安装 Agent</button>') +
                '<button class="btn btn-sm btn-danger" onclick="deleteRouter(\'' + router.name + '\')">删除</button>' +
            '</div>';
        
        grid.appendChild(card);
        
        // Auto scroll logs to bottom
        const logList = document.getElementById('log-list-' + router.name);
        if (logList) logList.scrollTop = logList.scrollHeight;
    });
    
    if (routers.length === 0) {
        grid.innerHTML = '<div class="setup-guide">' +
            '<svg class="guide-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>' +
            '<h3>欢迎使用 Floating Gateway</h3>' +
            '<p>按照以下步骤快速开始配置：</p>' +
            '<ol>' +
            '<li>点击右上角 <b>"添加路由器"</b> 分别添加主路由和旁路由</li>' +
            '<li>在 <b>"全局设置"</b> 中配置虚拟 IP (VIP) 和网卡信息</li>' +
            '<li>点击路由器卡片上的 <b>"安装 Agent"</b> 一键部署</li>' +
            '</ol>' +
            '<div class="hint">PVE 用户请确保网卡开启了 IP Anti-Spoofing 或关闭防火墙过滤以允许 VIP 通信</div>' +
            '</div>';
    }
}

// Router actions
async function showDoctor(name) {
    const reportDiv = $('#doctor-report');
    reportDiv.innerHTML = '<div class="loading">正在获取诊断报告...</div>';
    openModal('modal-doctor');
    
    try {
        // Use a timestamp to force fresh report and avoid cache
        const report = await apiCall('/routers/' + name + '/doctor?t=' + Date.now());
        
        // Check name translations
        const checkNames = {
            'interface_exists': '网卡接口',
            'cidr_valid': '网段配置',
            'vip_valid': 'VIP 配置',
            'vip_conflict': 'VIP 冲突检测',
            'peer_ip_valid': '对端路由器',
            'keepalived_running': 'Keepalived 服务',
            'keepalived_config': 'Keepalived 配置',
            'arping_available': 'ARP 工具'
        };
        
        let html = '<div style="display:flex;gap:1rem;margin-bottom:0.75rem;font-size:0.85rem;">' +
            '<div><span style="color:var(--text-muted)">平台:</span> ' + report.platform + '</div>' +
            '<div><span style="color:var(--text-muted)">角色:</span> ' + (report.role === 'primary' ? '主路由' : '旁路由') + '</div>' +
            '</div>';
        
        report.checks.forEach(check => {
            const displayName = checkNames[check.name] || check.name;
            const statusIcon = check.status === 'ok' ? '✓' : (check.status === 'warning' ? '⚠' : '✗');
            
            html += '<div class="doctor-item">' +
                '<div class="doctor-item-header">' +
                    '<span class="doctor-item-name">' + statusIcon + ' ' + displayName + '</span>' +
                    '<span class="doctor-status ' + check.status + '">' + 
                        (check.status === 'ok' ? '正常' : (check.status === 'warning' ? '警告' : '错误')) + 
                    '</span>' +
                '</div>' +
                '<div class="doctor-message">' + check.message + '</div>' +
                '</div>';
        });
        
        // Summary with color
        const summaryClass = report.has_errors ? 'error' : (report.has_warnings ? 'warning' : 'success');
        html += '<div class="doctor-summary ' + summaryClass + '">' + report.summary + '</div>';
            
        reportDiv.innerHTML = html;
    } catch (e) {
        reportDiv.innerHTML = '<div class="log-entry error">诊断失败: ' + e.message + '</div>';
    }
}

async function probeRouter(name) {
    log('正在探测 ' + name + '...');
    try {
        await apiCall('/routers/' + name + '/probe', { method: 'POST' });
        log('探测完成: ' + name, 'success');
        await refreshStatus();
    } catch (e) {
        log('探测失败: ' + e.message, 'error');
    }
}

async function installRouter(name, isUpgrade = false) {
    const actionText = isUpgrade ? '重装/升级' : '安装';
    if (!confirm('确定要在 ' + name + ' 上' + actionText + ' gateway-agent 吗？')) return;
    
    log('正在 ' + name + ' 上' + actionText + ' Agent...');
    try {
        await apiCall('/routers/' + name + '/install', { method: 'POST' });
        log('已开始' + actionText + ': ' + name, 'success');
        // Immediate refresh and trigger fast polling
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshStatus();
    } catch (e) {
        log(actionText + '失败: ' + e.message, 'error');
    }
}

async function uninstallRouter(name) {
    if (!confirm('确定要从 ' + name + ' 上卸载 gateway-agent 吗？')) return;
    
    log('正在从 ' + name + ' 上卸载 Agent...');
    try {
        await apiCall('/routers/' + name + '/uninstall', { method: 'POST' });
        log('已开始卸载: ' + name, 'success');
        // Immediate refresh and trigger fast polling
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshStatus();
    } catch (e) {
        log('卸载失败: ' + e.message, 'error');
    }
}

async function deleteRouter(name) {
    if (!confirm('确定要移除路由器 ' + name + ' 吗？')) return;
    
    try {
        await apiCall('/routers/' + name, { method: 'DELETE' });
        log('已移除: ' + name, 'success');
        await refreshStatus();
    } catch (e) {
        log('移除失败: ' + e.message, 'error');
    }
}

// Modal functions
function openModal(id) {
    document.getElementById(id).classList.add('active');
}

function closeModal(id) {
    document.getElementById(id).classList.remove('active');
}

// Event handlers
document.addEventListener('DOMContentLoaded', () => {
    // Initial load
    refreshStatus();
    log('控制台已加载');
    
    // Add router button
    $('#btn-add-router').addEventListener('click', () => {
        const form = $('#form-add-router');
        form.reset();
        // Reset probe result area
        const probeResult = $('#probe-result');
        probeResult.style.display = 'none';
        probeResult.className = 'probe-result';
        probeResult.innerHTML = '';
        openModal('modal-add-router');
    });
    
    // Global config button
    $('#btn-global-config').addEventListener('click', async () => {
        try {
            const cfg = await apiCall('/config');
            const form = $('#form-global-config');
            form.vip.value = cfg.lan.vip || '';
            form.cidr.value = cfg.lan.cidr || '';
            form.vrid.value = cfg.keepalived.vrid || 51;
            form.health_mode.value = cfg.health.mode || 'internet';
            openModal('modal-global-config');
        } catch (e) {
            log('获取配置失败: ' + e.message, 'error');
        }
    });

    // Verify Drift button
    $('#btn-verify-drift').addEventListener('click', () => {
        const stepsContainer = $('#drift-steps');
        stepsContainer.innerHTML = ''; // Clear previous
        
        // Add initial steps placeholders
        const steps = [
            {id: 'init', title: '环境检查'},
            {id: 'ping_vip', title: '初始连通性测试'},
            {id: 'trigger_drift', title: '模拟故障 (暂停主节点)'},
            {id: 'verify_drift', title: '验证漂移 (VIP 切换)'},
            {id: 'restore', title: '恢复环境'},
            {id: 'finish', title: '最终结果'}
        ];
        
        steps.forEach(step => {
            const div = document.createElement('div');
            div.className = 'drift-step';
            div.id = 'step-' + step.id;
            div.innerHTML = ` + "`" + `
                <div class="step-icon">
                    <div class="step-dot" style="width: 8px; height: 8px; background: var(--text-secondary); border-radius: 50%;"></div>
                </div>
                <div class="step-content">
                    <div class="step-title">${"${step.title}"}</div>
                    <div class="step-desc">等待开始...</div>
                </div>
            ` + "`" + `;
            stepsContainer.appendChild(div);
        });
        
        $('#btn-start-verify').disabled = false;
        $('#btn-start-verify').textContent = '开始验证';
        openModal('modal-verify-drift');
    });

    $('#btn-start-verify').addEventListener('click', async () => {
        const btn = $('#btn-start-verify');
        btn.disabled = true;
        btn.textContent = '验证进行中...';
        
        try {
            const response = await fetch('/api/verify-drift', {
                method: 'POST',
                headers: { 'Authorization': 'Bearer ' + token }
            });
            
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;
                
                const text = decoder.decode(value);
                const lines = text.split('\n');
                
                for (const line of lines) {
                    if (!line.trim()) continue;
                    try {
                        const event = JSON.parse(line);
                        updateDriftStep(event);
                    } catch (e) {
                        console.error('Failed to parse event:', line);
                    }
                }
            }
        } catch (error) {
            log('验证请求失败: ' + error.message, 'error');
            updateDriftStep({step: 'finish', status: 'error', message: '网络请求失败'});
        }
        
        btn.textContent = '验证完成';
        btn.disabled = false;
    });

    function updateDriftStep(event) {
        const el = $('#step-' + event.step);
        if (!el) return;
        
        el.className = 'drift-step ' + event.status;
        const icon = el.querySelector('.step-icon');
        const desc = el.querySelector('.step-desc');
        
        desc.textContent = event.message;
        
        if (event.status === 'running') {
            icon.innerHTML = '<div class="spinner"></div>';
        } else if (event.status === 'success') {
            icon.innerHTML = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="var(--success)" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>';
        } else if (event.status === 'error') {
            icon.innerHTML = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="var(--danger)" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';
        }
    }

    // Help button
    $('#btn-show-help').addEventListener('click', () => {
        openModal('modal-help');
    });

    // Check update button
    $('#btn-check-update').addEventListener('click', async () => {
        const infoDiv = $('#version-info');
        const downloadBtn = $('#version-download-btn');
        infoDiv.innerHTML = '<div class="loading">正在检查更新...</div>';
        downloadBtn.style.display = 'none';
        openModal('modal-version');
        
        try {
            const result = await apiCall('/version');
            let html = '<div class="version-info-container">';
            
            // Current version
            html += '<div class="version-current">' +
                '<span class="version-label">当前版本:</span>' +
                '<span class="version-value">' + (result.current_version || 'unknown') + '</span>' +
                '</div>';
            
            if (result.error) {
                html += '<div class="version-error">检查更新失败: ' + result.error + '</div>';
            } else if (result.has_update) {
                html += '<div class="version-status has-update">' +
                    '<div class="status-icon">⬆</div>' +
                    '<div class="status-text">发现新版本!</div>' +
                    '<div class="new-version">最新版本: <code>' + result.latest_version + '</code></div>' +
                    '</div>';
                
                if (result.release_notes) {
                    html += '<div class="version-notes-title">更新说明:</div>' +
                        '<div class="version-notes">' + escapeHtml(result.release_notes) + '</div>';
                }
                
                // Add auto-upgrade button
                html += '<div class="version-actions">' +
                    '<button id="btn-auto-upgrade" class="btn-primary" style="margin-right: 10px;">自动升级</button>' +
                    '<a href="' + result.release_url + '" target="_blank" class="btn-secondary">手动下载</a>' +
                    '</div>';
                
                downloadBtn.style.display = 'none'; // Hide the old download button
            } else {
                html += '<div class="version-status up-to-date">' +
                    '<div class="status-icon">✓</div>' +
                    '<div class="status-text">已是最新版本</div>' +
                    '</div>';
            }
            
            html += '</div>';
            infoDiv.innerHTML = html;
            log('版本检查完成: 当前 ' + result.current_version + ', 最新 ' + (result.latest_version || 'N/A'));
            
            // Add auto-upgrade button handler if update available
            if (result.has_update) {
                const upgradeBtn = document.getElementById('btn-auto-upgrade');
                if (upgradeBtn) {
                    upgradeBtn.addEventListener('click', async () => {
                        if (!confirm('确定要自动升级到 ' + result.latest_version + ' 吗？\\n\\n升级过程中服务会短暂中断。')) {
                            return;
                        }
                        
                        upgradeBtn.disabled = true;
                        upgradeBtn.textContent = '升级中...';
                        
                        try {
                            log('开始自动升级到 ' + result.latest_version + '...');
                            const upgradeResult = await apiCall('/upgrade', {
                                method: 'POST',
                                body: JSON.stringify({ version: result.latest_version })
                            });
                            
                            log('升级成功！服务将在 5 秒后重启...', 'success');
                            alert('升级成功！\\n\\n服务将在 5 秒后自动重启。\\n请稍后刷新页面。');
                            
                            // Wait and reload
                            setTimeout(() => {
                                window.location.reload();
                            }, 10000);
                        } catch (e) {
                            log('自动升级失败: ' + e.message, 'error');
                            alert('自动升级失败: ' + e.message + '\\n\\n请尝试手动下载升级。');
                            upgradeBtn.disabled = false;
                            upgradeBtn.textContent = '自动升级';
                        }
                    });
                }
            }
        } catch (e) {
            infoDiv.innerHTML = '<div class="version-error">检查更新失败: ' + e.message + '</div>';
            log('检查更新失败: ' + e.message, 'error');
        }
    });

    // Detect network button
    $('#btn-detect-net').addEventListener('click', async () => {
        const btn = $('#btn-detect-net');
        const originalText = btn.textContent;
        btn.disabled = true;
        btn.textContent = '获取中...';
        
        try {
            log('正在尝试自动探测网络配置...');
            const result = await apiCall('/detect-net', { method: 'POST' });
            const form = $('#form-global-config');
            form.cidr.value = result.cidr;
            // Auto-fill suggested VIP if not already set
            if (!form.vip.value && result.suggested_vip) {
                form.vip.value = result.suggested_vip;
                log('自动探测成功: 网段 ' + result.cidr + '，建议 VIP: ' + result.suggested_vip, 'success');
            } else {
                log('自动探测成功: 网段 ' + result.cidr, 'success');
            }
        } catch (e) {
            log('自动探测失败: ' + e.message, 'error');
              showToast('探测失败: ' + e.message + '。请确保网络正常。', 'error', 5000);
        } finally {
            btn.disabled = false;
            btn.textContent = originalText;
        }
    });

    // Router probe in modal
    $('#btn-router-probe').addEventListener('click', async () => {
        const form = $('#form-add-router');
        const host = form.host.value;
        const user = form.user.value;
        const password = form.password.value;
        const key_file = form.key_file.value;
        const port = parseInt(form.port.value) || 22;
        const probeResult = $('#probe-result');
        
        if (!host) {
            probeResult.style.display = 'block';
            probeResult.className = 'probe-result error';
            probeResult.innerHTML = '✗ 请先输入主机地址';
            return;
        }
        if (!user) {
            probeResult.style.display = 'block';
            probeResult.className = 'probe-result error';
            probeResult.innerHTML = '✗ 请先输入 SSH 用户名';
            return;
        }
        if (!password && !key_file) {
            probeResult.style.display = 'block';
            probeResult.className = 'probe-result warning';
            probeResult.innerHTML = '⚠ 未填写 SSH 密码或私钥路径，探测可能会失败';
        }

        const btn = $('#btn-router-probe');
        const probeText = btn.querySelector('.probe-text');
        btn.disabled = true;
        const originalText = probeText.textContent;
        probeText.textContent = '正在连接...';
        
        // Show loading state in result area
        probeResult.style.display = 'block';
        probeResult.className = 'probe-result loading';
        probeResult.innerHTML = '<span class="probe-spinner"></span> 正在探测 ' + host + ' 的网络环境...';
        
        try {
            log('正在探测 ' + host + '...');
            const result = await apiCall('/detect-net', { 
                method: 'POST',
                body: JSON.stringify({
                    host, user, password, key_file, port
                })
            });
            log('探测成功: ' + result.iface + ' (' + result.cidr + ')', 'success');
            
            // Auto-fill interface to form
            form.iface.value = result.iface;

            // Auto-select best health mode based on role
            const role = form.role.value;
            if (role === 'primary') {
                form.health_mode.value = 'basic';
                log('已根据角色建议: 主路由默认使用基础模式 (更稳定)');
            } else if (role === 'secondary') {
                form.health_mode.value = 'internet';
                log('已根据角色建议: 旁路由默认使用互联网模式 (更可靠)');
            }
            
            // Store result for later use
            window._lastProbeResult = result;
            
            // Show success in result area with apply button
            const currentCfg = await apiCall('/config');
            const needsConfig = !currentCfg.lan.iface || !currentCfg.lan.cidr;
            
            probeResult.className = 'probe-result success';
            probeResult.innerHTML = '<div class="probe-header">✓ SSH 连接成功，网络探测完成</div>' +
                '<div class="probe-detail">' +
                    '<span><span class="probe-label">网卡接口</span> <code>' + result.iface + '</code></span>' +
                    '<span><span class="probe-label">网段</span> <code>' + result.cidr + '</code></span>' +
                    (result.suggested_vip ? '<span><span class="probe-label">建议 VIP</span> <code>' + result.suggested_vip + '</code></span>' : '') +
                '</div>' +
                (needsConfig ? '<button type="button" class="btn btn-sm btn-primary probe-apply-btn" id="btn-apply-probe-config" style="margin-top: 0.75rem; width: 100%;">应用到全局配置</button>' : '');
            
            // Add click handler for apply button if shown
            if (needsConfig) {
                setTimeout(() => {
                    const applyBtn = document.getElementById('btn-apply-probe-config');
                    if (applyBtn) {
                        applyBtn.addEventListener('click', async () => {
                            applyBtn.disabled = true;
                            applyBtn.textContent = '正在应用...';
                            try {
                                const cfg = await apiCall('/config');
                                await apiCall('/config', {
                                    method: 'PUT',
                                    body: JSON.stringify({
                                        lan: {
                                            cidr: result.cidr,
                                            vip: cfg.lan.vip || result.suggested_vip || ''
                                        }
                                    })
                                });
                                log('已自动更新全局网络配置', 'success');
                                showToast('全局网络配置已更新', 'success');
                                applyBtn.textContent = '✓ 已应用';
                                applyBtn.classList.remove('btn-primary');
                                applyBtn.classList.add('btn-ghost');
                            } catch (e) {
                                log('应用配置失败: ' + e.message, 'error');
                                showToast('应用失败: ' + e.message, 'error');
                                applyBtn.disabled = false;
                                applyBtn.textContent = '应用到全局配置';
                            }
                        });
                    }
                }, 0);
            }
            
            showToast('探测成功! 网卡: ' + result.iface + ' 网段: ' + result.cidr, 'success');
        } catch (e) {
            log('探测失败: ' + e.message, 'error');
            // Show error in result area with more details
            probeResult.className = 'probe-result error';
            let errorHint = '';
            if (e.message.includes('connection refused')) {
                errorHint = '<div class="probe-hint">提示: 请检查 SSH 端口是否正确，目标主机是否已启动 SSH 服务</div>';
            } else if (e.message.includes('authentication')) {
                errorHint = '<div class="probe-hint">提示: 请检查 SSH 用户名和密码/私钥是否正确</div>';
            } else if (e.message.includes('timeout') || e.message.includes('i/o timeout')) {
                errorHint = '<div class="probe-hint">提示: 连接超时，请检查网络连通性和防火墙设置</div>';
            } else if (e.message.includes('no route')) {
                errorHint = '<div class="probe-hint">提示: 无法到达主机，请检查 IP 地址是否正确</div>';
            }
            probeResult.innerHTML = '✗ 探测失败: ' + e.message + errorHint;
        } finally {
            btn.disabled = false;
            probeText.textContent = originalText;
        }
    });
    
    // Refresh button
    $('#btn-refresh').addEventListener('click', () => {
        log('正在刷新...');
        refreshStatus();
    });
    
    // Add router form
    $('#form-add-router').addEventListener('submit', async (e) => {
        e.preventDefault();
        const form = e.target;
        const router = {
            name: form.name.value,
            host: form.host.value,
            port: parseInt(form.port.value) || 22,
            user: form.user.value,
            password: form.password.value,
            key_file: form.key_file.value,
            role: form.role.value,
            iface: form.iface.value || '',
            health_mode: form.health_mode.value || ''
        };
        
        try {
              await apiCall('/routers', {
                  method: 'POST',
                  body: JSON.stringify(router)
              });
              log('已添加路由器: ' + router.name, 'success');
              showToast('已添加路由器: ' + router.name, 'success');
              closeModal('modal-add-router');
              form.reset();
              await refreshStatus();
              // Auto-probe the newly added router
              probeRouter(router.name);
          } catch (e) {
              log('添加路由器失败: ' + e.message, 'error');
              showToast('添加失败: ' + e.message, 'error');
          }
    });

    // Global config form
    $('#form-global-config').addEventListener('submit', async (e) => {
        e.preventDefault();
        const form = e.target;
        const update = {
            lan: {
                vip: form.vip.value,
                cidr: form.cidr.value
            },
            keepalived: {
                vrid: parseInt(form.vrid.value)
            },
            health: {
                mode: form.health_mode.value
            }
        };
        
        try {
              await apiCall('/config', {
                  method: 'PUT',
                  body: JSON.stringify(update)
              });
              log('全局配置已更新', 'success');
              showToast('全局配置已保存', 'success');
              closeModal('modal-global-config');
              await refreshStatus();
          } catch (e) {
              log('更新配置失败: ' + e.message, 'error');
              showToast('保存失败: ' + e.message, 'error');
          }
    });
    
    // Close modal on background click
    $$('.modal').forEach(modal => {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.classList.remove('active');
            }
        });
    });

    // Close modal on ESC key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            $$('.modal.active').forEach(m => m.classList.remove('active'));
        }
    });
});`
