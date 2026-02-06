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

        <main>
            <section id="routers-section">
                <div class="section-header">
                    <h2>路由器管理</h2>
                    <div class="section-actions">
                        <button id="btn-refresh" class="btn btn-icon" title="刷新状态">
                            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"/><path d="M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>
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
                                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
                                测试 SSH 连接并探测网络环境
                            </button>
                            <small class="form-hint" style="text-align: center;">点击后将尝试使用上方填写的信息登录路由器并自动获取网卡与网段</small>
                        </div>
                        <div class="form-group">
                            <label>角色</label>
                            <select name="role" required>
                                <option value="primary">主路由 (Primary - 备用网关)</option>
                                <option value="secondary" selected>旁路由 (Secondary - 首选网关)</option>
                            </select>
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
                            <small class="form-hint">留空则根据网卡自动推断</small>
                        </div>
                        <div class="form-group">
                            <label>网卡接口 (Interface)</label>
                            <input type="text" name="iface" required placeholder="br-lan 或 eth0">
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
    </div>
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

.modal-sm {
    max-width: 560px;
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
}`

const appJS = `// Gateway Controller UI
const API_BASE = '/api';
let routers = [];

// Utility functions
function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

function formatTime(date) {
    return new Date(date).toLocaleTimeString();
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
async function refreshStatus() {
    try {
        const status = await apiCall('/status');
        
        $('#vip-address').textContent = status.vip || '-';
        $('#current-master').textContent = status.current_master || '无';
        
        routers = status.routers || [];
        renderRouters();

        // If any router is installing/uninstalling, poll faster (every 2s)
        const isBusy = routers.some(r => r.status === 'installing' || r.status === 'uninstalling');
        const interval = isBusy ? 2000 : 30000;
        
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
        
        let progressHtml = '';
        if ((statusClass === 'installing' || statusClass === 'uninstalling' || (statusClass === 'error' && router.install_log)) && router.install_log) {
            const logs = router.install_log.map(line => '<div class="install-log-item">' + line + '</div>').join('');
            progressHtml = 
                '<div class="install-progress">' +
                    '<div class="install-progress-header">' +
                        '<span>执行进度</span>' +
                        '<span class="loading-dots">...</span>' +
                    '</div>' +
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
                '<div><span class="label">Agent:</span> <span class="value">' + (router.agent_version || '未安装') + '</span></div>' +
                '<div><span class="label">VRRP状态:</span> ' + (vrrpHtml || '<span class="value">-</span>') + '</div>' +
                '<div><span class="label">健康状态:</span> ' + (healthHtml || '<span class="value">-</span>') + '</div>' +
            '</div>' +
            progressHtml +
            '<div class="router-actions">' +
                '<button class="btn btn-sm" onclick="probeRouter(\'' + router.name + '\')">探测</button>' +
                (router.agent_version 
                    ? '<button class="btn btn-sm" onclick="showDoctor(\'' + router.name + '\')">诊断</button>' +
                      '<button class="btn btn-sm btn-danger" onclick="uninstallRouter(\'' + router.name + '\')">卸载 Agent</button>'
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
        const report = await apiCall('/routers/' + name + '/doctor');
        let html = '<div style="display:flex;gap:1rem;margin-bottom:0.75rem;font-size:0.85rem;">' +
            '<div><span style="color:var(--text-muted)">平台:</span> ' + report.platform + '</div>' +
            '<div><span style="color:var(--text-muted)">角色:</span> ' + (report.role === 'primary' ? '主路由' : '旁路路由') + '</div>' +
            '</div>';
        
        report.checks.forEach(check => {
            html += '<div class="doctor-item">' +
                '<div class="doctor-item-header">' +
                    '<span class="doctor-item-name">' + check.name + '</span>' +
                    '<span class="doctor-status ' + check.status + '">' + check.status.toUpperCase() + '</span>' +
                '</div>' +
                '<div class="doctor-message">' + check.message + '</div>' +
                '</div>';
        });
        
        html += '<div class="doctor-summary">' + report.summary + '</div>';
            
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

async function installRouter(name) {
    if (!confirm('确定要在 ' + name + ' 上安装 gateway-agent 吗？')) return;
    
    log('正在 ' + name + ' 上安装 Agent...');
    try {
        await apiCall('/routers/' + name + '/install', { method: 'POST' });
        log('已开始安装: ' + name, 'success');
        // Immediate refresh and trigger fast polling
        if (refreshTimer) clearTimeout(refreshTimer);
        refreshStatus();
    } catch (e) {
        log('安装失败: ' + e.message, 'error');
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
    
    // Auto refresh every 30s
    setInterval(refreshStatus, 30000);
    
    // Add router button
    $('#btn-add-router').addEventListener('click', () => {
        openModal('modal-add-router');
    });
    
    // Global config button
    $('#btn-global-config').addEventListener('click', async () => {
        try {
            const cfg = await apiCall('/config');
            const form = $('#form-global-config');
            form.vip.value = cfg.lan.vip || '';
            form.cidr.value = cfg.lan.cidr || '';
            form.iface.value = cfg.lan.iface || '';
            form.vrid.value = cfg.keepalived.vrid || 51;
            form.health_mode.value = cfg.health.mode || 'internet';
            openModal('modal-global-config');
        } catch (e) {
            log('获取配置失败: ' + e.message, 'error');
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
            form.iface.value = result.iface;
            log('自动探测成功: ' + result.iface + ' (' + result.cidr + ')', 'success');
        } catch (e) {
            log('自动探测失败: ' + e.message, 'error');
            alert('探测失败: ' + e.message + '\n\n请确保已添加至少一个路由器且网络连接正常。');
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
        
        if (!host) {
            alert('请先输入主机地址');
            return;
        }
        if (!password && !key_file) {
            if (!confirm('你没有填写 SSH 密码或私钥路径，探测可能会因为权限不足而失败。是否继续？')) {
                return;
            }
        }

        const btn = $('#btn-router-probe');
        btn.disabled = true;
        const originalText = btn.textContent;
        btn.textContent = '探测中...';
        
        try {
            log('正在探测 ' + host + '...');
            const result = await apiCall('/detect-net', { 
                method: 'POST',
                body: JSON.stringify({
                    host, user, password, key_file, port
                })
            });
            log('探测成功: ' + result.iface + ' (' + result.cidr + ')', 'success');
            
            // Suggest filling global config if empty
            const currentCfg = await apiCall('/config');
            if (!currentCfg.lan.iface || !currentCfg.lan.cidr) {
                if (confirm('探测到网段: ' + result.cidr + '\n网卡: ' + result.iface + '\n\n是否将其设为全局默认配置？')) {
                    await apiCall('/config', {
                        method: 'PUT',
                        body: JSON.stringify({
                            lan: {
                                iface: result.iface,
                                cidr: result.cidr,
                                vip: currentCfg.lan.vip || result.cidr.split('.').slice(0, 3).join('.') + '.254'
                            }
                        })
                    });
                    log('已自动更新全局网络配置', 'success');
                }
            } else {
                alert('探测成功！\n网卡: ' + result.iface + '\n网段: ' + result.cidr);
            }
        } catch (e) {
            log('探测失败: ' + e.message, 'error');
            alert('探测失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = originalText;
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
            role: form.role.value
        };
        
        try {
            await apiCall('/routers', {
                method: 'POST',
                body: JSON.stringify(router)
            });
            log('已添加路由器: ' + router.name, 'success');
            closeModal('modal-add-router');
            form.reset();
            await refreshStatus();
        } catch (e) {
            log('添加路由器失败: ' + e.message, 'error');
        }
    });

    // Global config form
    $('#form-global-config').addEventListener('submit', async (e) => {
        e.preventDefault();
        const form = e.target;
        const update = {
            lan: {
                vip: form.vip.value,
                cidr: form.cidr.value,
                iface: form.iface.value
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
            closeModal('modal-global-config');
            await refreshStatus();
        } catch (e) {
            log('更新配置失败: ' + e.message, 'error');
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
});`
