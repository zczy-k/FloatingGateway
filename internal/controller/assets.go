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
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Floating Gateway Controller</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <div id="app">
        <header>
            <h1>Floating Gateway</h1>
            <div id="vip-status">
                <span class="label">VIP:</span>
                <span id="vip-address">-</span>
                <span class="label">Master:</span>
                <span id="current-master">-</span>
            </div>
        </header>

        <main>
            <section id="routers-section">
                <div class="section-header">
                    <h2>Routers</h2>
                    <button id="btn-add-router" class="btn btn-primary">Add Router</button>
                    <button id="btn-refresh" class="btn">Refresh</button>
                </div>
                <div id="routers-grid"></div>
            </section>

            <section id="logs-section">
                <h2>Activity Log</h2>
                <div id="logs"></div>
            </section>
        </main>

        <!-- Add Router Modal -->
        <div id="modal-add-router" class="modal">
            <div class="modal-content">
                <h3>Add Router</h3>
                <form id="form-add-router">
                    <div class="form-group">
                        <label>Name</label>
                        <input type="text" name="name" required placeholder="e.g., openwrt-main">
                    </div>
                    <div class="form-group">
                        <label>Host</label>
                        <input type="text" name="host" required placeholder="192.168.1.1">
                    </div>
                    <div class="form-group">
                        <label>Port</label>
                        <input type="number" name="port" value="22">
                    </div>
                    <div class="form-group">
                        <label>User</label>
                        <input type="text" name="user" required value="root">
                    </div>
                    <div class="form-group">
                        <label>Password</label>
                        <input type="password" name="password">
                    </div>
                    <div class="form-group">
                        <label>SSH Key File</label>
                        <input type="text" name="key_file" placeholder="~/.ssh/id_rsa">
                    </div>
                    <div class="form-group">
                        <label>Role</label>
                        <select name="role" required>
                            <option value="primary">Primary (Backup Gateway)</option>
                            <option value="secondary" selected>Secondary (Preferred Gateway)</option>
                        </select>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn" onclick="closeModal('modal-add-router')">Cancel</button>
                        <button type="submit" class="btn btn-primary">Add</button>
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
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
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

header h1 {
    font-size: 1.5rem;
    font-weight: 600;
}

#vip-status {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.9rem;
}

#vip-status .label {
    color: var(--text-muted);
}

#vip-address, #current-master {
    font-family: monospace;
    background: var(--bg-input);
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
}

main {
    max-width: 1400px;
    margin: 0 auto;
    padding: 2rem;
}

section {
    margin-bottom: 2rem;
}

.section-header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
}

.section-header h2 {
    font-size: 1.25rem;
    font-weight: 500;
}

.btn {
    background: var(--bg-input);
    color: var(--text);
    border: 1px solid var(--border);
    padding: 0.5rem 1rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.875rem;
    transition: all 0.2s;
}

.btn:hover {
    background: var(--border);
}

.btn-primary {
    background: var(--primary);
    color: var(--bg);
    border-color: var(--primary);
}

.btn-primary:hover {
    opacity: 0.9;
}

.btn-danger {
    background: var(--danger);
    color: white;
    border-color: var(--danger);
}

.btn-sm {
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
}

#routers-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 1rem;
}

.router-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1.25rem;
}

.router-card.master {
    border-color: var(--success);
    box-shadow: 0 0 0 1px var(--success);
}

.router-card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1rem;
}

.router-name {
    font-size: 1.1rem;
    font-weight: 500;
}

.router-role {
    font-size: 0.75rem;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    background: var(--bg-input);
}

.router-role.primary {
    color: var(--warning);
}

.router-role.secondary {
    color: var(--primary);
}

.status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.75rem;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
}

.status-badge.online {
    background: rgba(74, 222, 128, 0.15);
    color: var(--success);
}

.status-badge.offline {
    background: rgba(248, 113, 113, 0.15);
    color: var(--danger);
}

.status-badge.unknown {
    background: rgba(136, 146, 176, 0.15);
    color: var(--text-muted);
}

.status-badge.installing, .status-badge.uninstalling {
    background: rgba(251, 191, 36, 0.15);
    color: var(--warning);
}

.status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
}

.router-info {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin-bottom: 1rem;
}

.router-info div {
    margin-bottom: 0.35rem;
}

.router-info .label {
    display: inline-block;
    width: 80px;
}

.router-info .value {
    color: var(--text);
    font-family: monospace;
}

.vrrp-state {
    display: inline-block;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 500;
}

.vrrp-state.master {
    background: var(--success);
    color: var(--bg);
}

.vrrp-state.backup {
    background: var(--text-muted);
    color: var(--bg);
}

.health-indicator {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
}

.health-indicator.healthy {
    color: var(--success);
}

.health-indicator.unhealthy {
    color: var(--danger);
}

.router-actions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-top: 1rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
}

/* Modal */
.modal {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    align-items: center;
    justify-content: center;
    z-index: 100;
}

.modal.active {
    display: flex;
}

.modal-content {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1.5rem;
    width: 100%;
    max-width: 400px;
}

.modal-content h3 {
    margin-bottom: 1.25rem;
}

.form-group {
    margin-bottom: 1rem;
}

.form-group label {
    display: block;
    font-size: 0.85rem;
    color: var(--text-muted);
    margin-bottom: 0.35rem;
}

.form-group input,
.form-group select {
    width: 100%;
    padding: 0.5rem;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text);
    font-size: 0.9rem;
}

.form-group input:focus,
.form-group select:focus {
    outline: none;
    border-color: var(--primary);
}

.form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1.5rem;
}

/* Logs */
#logs {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1rem;
    max-height: 200px;
    overflow-y: auto;
    font-family: monospace;
    font-size: 0.8rem;
}

.log-entry {
    padding: 0.25rem 0;
    border-bottom: 1px solid var(--border);
}

.log-entry:last-child {
    border-bottom: none;
}

.log-time {
    color: var(--text-muted);
    margin-right: 0.5rem;
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
        log('API Error: ' + e.message, 'error');
        throw e;
    }
}

// Status update
async function refreshStatus() {
    try {
        const status = await apiCall('/status');
        
        $('#vip-address').textContent = status.vip || '-';
        $('#current-master').textContent = status.current_master || 'None';
        
        routers = status.routers || [];
        renderRouters();
    } catch (e) {
        console.error('Failed to refresh status:', e);
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
            healthHtml = '<span class="health-indicator ' + healthClass + '">' + healthIcon + ' Health</span>';
        }
        
        let vrrpHtml = '';
        if (router.vrrp_state) {
            const vrrpClass = router.vrrp_state.toLowerCase();
            vrrpHtml = '<span class="vrrp-state ' + vrrpClass + '">' + router.vrrp_state + '</span>';
        }
        
        card.innerHTML = 
            '<div class="router-card-header">' +
                '<div>' +
                    '<div class="router-name">' + router.name + '</div>' +
                    '<span class="router-role ' + roleClass + '">' + router.role + '</span>' +
                '</div>' +
                '<span class="status-badge ' + statusClass + '">' +
                    '<span class="status-dot"></span>' +
                    statusClass +
                '</span>' +
            '</div>' +
            '<div class="router-info">' +
                '<div><span class="label">Host:</span> <span class="value">' + router.host + ':' + router.port + '</span></div>' +
                '<div><span class="label">Platform:</span> <span class="value">' + (router.platform || '-') + '</span></div>' +
                '<div><span class="label">Agent:</span> <span class="value">' + (router.agent_version || 'Not installed') + '</span></div>' +
                '<div><span class="label">VRRP:</span> ' + (vrrpHtml || '<span class="value">-</span>') + '</div>' +
                '<div><span class="label">Health:</span> ' + (healthHtml || '<span class="value">-</span>') + '</div>' +
            '</div>' +
            '<div class="router-actions">' +
                '<button class="btn btn-sm" onclick="probeRouter(\'' + router.name + '\')">Probe</button>' +
                (router.agent_version 
                    ? '<button class="btn btn-sm btn-danger" onclick="uninstallRouter(\'' + router.name + '\')">Uninstall</button>'
                    : '<button class="btn btn-sm btn-primary" onclick="installRouter(\'' + router.name + '\')">Install</button>') +
                '<button class="btn btn-sm btn-danger" onclick="deleteRouter(\'' + router.name + '\')">Delete</button>' +
            '</div>';
        
        grid.appendChild(card);
    });
    
    if (routers.length === 0) {
        grid.innerHTML = '<p style="color: var(--text-muted)">No routers configured. Click "Add Router" to get started.</p>';
    }
}

// Router actions
async function probeRouter(name) {
    log('Probing ' + name + '...');
    try {
        await apiCall('/routers/' + name + '/probe', { method: 'POST' });
        log('Probed ' + name, 'success');
        await refreshStatus();
    } catch (e) {
        log('Probe failed: ' + e.message, 'error');
    }
}

async function installRouter(name) {
    if (!confirm('Install gateway-agent on ' + name + '?')) return;
    
    log('Installing agent on ' + name + '...');
    try {
        await apiCall('/routers/' + name + '/install', { method: 'POST' });
        log('Installation started on ' + name, 'success');
        // Poll for completion
        setTimeout(refreshStatus, 5000);
    } catch (e) {
        log('Install failed: ' + e.message, 'error');
    }
}

async function uninstallRouter(name) {
    if (!confirm('Uninstall gateway-agent from ' + name + '?')) return;
    
    log('Uninstalling agent from ' + name + '...');
    try {
        await apiCall('/routers/' + name + '/uninstall', { method: 'POST' });
        log('Uninstallation started on ' + name, 'success');
        setTimeout(refreshStatus, 3000);
    } catch (e) {
        log('Uninstall failed: ' + e.message, 'error');
    }
}

async function deleteRouter(name) {
    if (!confirm('Remove router ' + name + ' from management?')) return;
    
    try {
        await apiCall('/routers/' + name, { method: 'DELETE' });
        log('Removed ' + name, 'success');
        await refreshStatus();
    } catch (e) {
        log('Delete failed: ' + e.message, 'error');
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
    log('Controller UI loaded');
    
    // Auto refresh every 30s
    setInterval(refreshStatus, 30000);
    
    // Add router button
    $('#btn-add-router').addEventListener('click', () => {
        openModal('modal-add-router');
    });
    
    // Refresh button
    $('#btn-refresh').addEventListener('click', () => {
        log('Refreshing...');
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
            log('Added router ' + router.name, 'success');
            closeModal('modal-add-router');
            form.reset();
            await refreshStatus();
        } catch (e) {
            log('Failed to add router: ' + e.message, 'error');
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
