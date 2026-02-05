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
            <h1>浮动网关 (Floating Gateway)</h1>
            <div id="vip-status">
                <span class="label">虚拟 IP (VIP):</span>
                <span id="vip-address">-</span>
                <span class="label">当前主控:</span>
                <span id="current-master">-</span>
            </div>
        </header>

        <main>
            <section id="routers-section">
                <div class="section-header">
                    <h2>路由器管理</h2>
                    <button id="btn-add-router" class="btn btn-primary">添加路由器</button>
                    <button id="btn-global-config" class="btn">全局设置</button>
                    <button id="btn-refresh" class="btn">刷新状态</button>
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
            <div class="modal-content" style="max-width: 600px;">
                <h3>诊断报告</h3>
                <div id="doctor-report" style="max-height: 400px; overflow-y: auto; font-size: 0.85rem;">
                    <div class="loading">正在获取报告...</div>
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-primary" onclick="closeModal('modal-doctor')">关闭</button>
                </div>
            </div>
        </div>

        <!-- Add Router Modal -->

        <div id="modal-add-router" class="modal">
            <div class="modal-content">
                <h3>添加路由器</h3>
                <form id="form-add-router">
                    <div class="form-group">
                        <label>名称</label>
                        <input type="text" name="name" required placeholder="例如: openwrt-main">
                    </div>
                      <div class="form-group">
                          <label>主机地址 (IP)</label>
                          <div style="display: flex; gap: 0.5rem;">
                              <input type="text" name="host" required placeholder="192.168.1.1">
                              <button type="button" class="btn btn-sm" id="btn-router-probe">探测</button>
                          </div>
                      </div>

                    <div class="form-group">
                        <label>SSH 端口</label>
                        <input type="number" name="port" value="22">
                    </div>
                    <div class="form-group">
                        <label>SSH 用户</label>
                        <input type="text" name="user" required value="root">
                    </div>
                    <div class="form-group">
                        <label>SSH 密码</label>
                        <input type="password" name="password">
                    </div>
                    <div class="form-group">
                        <label>SSH 私钥文件路径</label>
                        <input type="text" name="key_file" placeholder="~/.ssh/id_rsa">
                    </div>
                    <div class="form-group">
                        <label>角色</label>
                        <select name="role" required>
                            <option value="primary">主路由 (Primary - 备用网关)</option>
                            <option value="secondary" selected>旁路由 (Secondary - 首选网关)</option>
                        </select>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn" onclick="closeModal('modal-add-router')">取消</button>
                        <button type="submit" class="btn btn-primary">确定添加</button>
                    </div>
                </form>
            </div>
        </div>

        <!-- Global Config Modal -->
        <div id="modal-global-config" class="modal">
            <div class="modal-content">
                <h3>全局设置</h3>
                <form id="form-global-config">
                    <div class="form-group">
                        <label>虚拟 IP (VIP)</label>
                        <input type="text" name="vip" required placeholder="192.168.1.254">
                    </div>
                    <div class="form-group">
                        <label>网段 (CIDR)</label>
                        <div style="display: flex; gap: 0.5rem;">
                            <input type="text" name="cidr" required placeholder="192.168.1.0/24">
                            <button type="button" class="btn btn-sm" id="btn-detect-net">自动获取</button>
                        </div>
                        <small style="color: var(--text-muted); font-size: 0.7rem;">留空则根据网卡自动推断</small>
                    </div>
                    <div class="form-group">
                        <label>网卡接口 (Interface)</label>
                        <input type="text" name="iface" required placeholder="br-lan 或 eth0">
                    </div>
                    <div class="form-group">
                        <label>虚拟路由标识 (VRID)</label>
                        <input type="number" name="vrid" required value="51" min="1" max="255">
                    </div>
                    <div class="form-group">
                        <label>检测模式 (Health Mode)</label>
                        <select name="health_mode" required>
                            <option value="internet">互联网模式 (检测外网)</option>
                            <option value="basic">基础模式 (仅检测网关)</option>
                        </select>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn" onclick="closeModal('modal-global-config')">取消</button>
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
    --bg: #1a1a2e;
    --bg-card: #16213e;
    --bg-input: #0f1629;
    --text: #eef0f2;
    --text-muted: #8892b0;
    --primary: #4cc9f0;
    --success: #4ade80;
    --warning: #fbbf24;
    --danger: #f87171;
    --border: #2d3a5a;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
    background: var(--bg);
    color: var(--text);
    min-height: 100vh;
}

header {
    background: var(--bg-card);
    padding: 1rem 2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border);
}

.setup-guide {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 2rem;
    max-width: 600px;
    margin: 0 auto;
}

.setup-guide h3 {
    margin-bottom: 1rem;
    color: var(--primary);
}

.setup-guide ol {
    margin-left: 1.5rem;
    margin-bottom: 1rem;
}

.setup-guide li {
    margin-bottom: 0.5rem;
    line-height: 1.5;
}

.loading {
    padding: 2rem;
    text-align: center;
    color: var(--text-muted);
}

.log-entry.error {

    color: var(--danger);
}

.log-entry.success {
    color: var(--success);
}

/* Responsive */
@media (max-width: 640px) {
    header {
        flex-direction: column;
        gap: 1rem;
        text-align: center;
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
async function refreshStatus() {
    try {
        const status = await apiCall('/status');
        
        $('#vip-address').textContent = status.vip || '-';
        $('#current-master').textContent = status.current_master || '无';
        
        routers = status.routers || [];
        renderRouters();
    } catch (e) {
        console.error('刷新状态失败:', e);
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
            '<div class="router-actions">' +
                '<button class="btn btn-sm" onclick="probeRouter(\'' + router.name + '\')">探测</button>' +
                (router.agent_version 
                    ? '<button class="btn btn-sm" onclick="showDoctor(\'' + router.name + '\')">诊断</button>' +
                      '<button class="btn btn-sm btn-danger" onclick="uninstallRouter(\'' + router.name + '\')">卸载 Agent</button>'
                    : '<button class="btn btn-sm btn-primary" onclick="installRouter(\'' + router.name + '\')">安装 Agent</button>') +
                '<button class="btn btn-sm btn-danger" onclick="deleteRouter(\'' + router.name + '\')">删除</button>' +
            '</div>';
        
        grid.appendChild(card);
    });
    
    if (routers.length === 0) {
        grid.innerHTML = '<div class="setup-guide">' +
            '<h3>欢迎使用浮动网关</h3>' +
            '<p>看起来您还没有添加任何路由器。按照以下步骤开始：</p>' +
            '<ol>' +
            '<li>点击右上角的 <b>“添加路由器”</b> 分别添加您的主路由和旁路路由。</li>' +
            '<li>在 <b>“全局设置”</b> 中配置您的虚拟 IP (VIP) 和网卡信息。</li>' +
            '<li>点击路由器卡片上的 <b>“安装 Agent”</b> 一键部署。</li>' +
            '</ol>' +
            '<p style="margin-top: 1rem; font-size: 0.8rem; color: var(--text-muted)">提示：PVE 用户请确保网卡开启了 IP Anti-Spoofing 或关闭防火墙过滤以允许 VIP 通信。</p>' +
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
        let html = '<div style="margin-bottom: 1rem;">' +
            '<div><b>平台:</b> ' + report.platform + '</div>' +
            '<div><b>角色:</b> ' + (report.role === 'primary' ? '主路由' : '旁路路由') + '</div>' +
            '</div>';
        
        report.checks.forEach(check => {
            const statusIcon = check.status === 'ok' ? '✅' : (check.status === 'warning' ? '⚠️' : '❌');
            const statusColor = check.status === 'ok' ? 'var(--success)' : (check.status === 'warning' ? 'var(--warning)' : 'var(--danger)');
            html += '<div style="padding: 0.5rem; border-bottom: 1px solid var(--border);">' +
                '<div style="display: flex; justify-content: space-between;">' +
                    '<b>' + check.name + '</b>' +
                    '<span style="color: ' + statusColor + '">' + statusIcon + ' ' + check.status.toUpperCase() + '</span>' +
                '</div>' +
                '<div style="color: var(--text-muted); font-size: 0.8rem;">' + check.message + '</div>' +
                '</div>';
        });
        
        html += '<div style="margin-top: 1rem; padding: 0.75rem; background: var(--bg-input); border-radius: 4px; font-weight: bold;">' +
            report.summary +
            '</div>';
            
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
        // Poll for completion
        setTimeout(refreshStatus, 5000);
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
        setTimeout(refreshStatus, 3000);
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
        const port = parseInt(form.port.value) || 22;
        
        if (!host) {
            alert('请先输入主机地址');
            return;
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
                    host, user, password, port
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
