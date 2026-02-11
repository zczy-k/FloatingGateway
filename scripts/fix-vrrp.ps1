# fix-vrrp.ps1 - VRRP 漂移问题远程诊断脚本 (Windows PowerShell)
# 用法: .\fix-vrrp.ps1 -RouterIP 192.168.1.2 -Username root [-Password "password"]

param(
    [Parameter(Mandatory=$true)]
    [string]$RouterIP,
    
    [Parameter(Mandatory=$true)]
    [string]$Username,
    
    [Parameter(Mandatory=$false)]
    [string]$Password = "",
    
    [Parameter(Mandatory=$false)]
    [string]$VIP = "192.168.1.1",
    
    [Parameter(Mandatory=$false)]
    [string]$Interface = "br-lan",
    
    [Parameter(Mandatory=$false)]
    [switch]$AutoFix
)

# 颜色输出函数
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Success {
    param([string]$Message)
    Write-ColorOutput "   ✓ $Message" "Green"
}

function Write-Error {
    param([string]$Message)
    Write-ColorOutput "   ✗ $Message" "Red"
}

function Write-Warning {
    param([string]$Message)
    Write-ColorOutput "   ! $Message" "Yellow"
}

function Write-Info {
    param([string]$Message)
    Write-ColorOutput "   → $Message" "Cyan"
}

# SSH 命令执行函数
function Invoke-SSHCommand {
    param(
        [string]$Command
    )
    
    if ($Password -ne "") {
        $sshCmd = "sshpass -p '$Password' ssh -o StrictHostKeyChecking=no $Username@$RouterIP '$Command' 2>&1"
    } else {
        $sshCmd = "ssh -o StrictHostKeyChecking=no $Username@$RouterIP '$Command' 2>&1"
    }
    
    try {
        $result = Invoke-Expression $sshCmd
        return @{
            Success = $LASTEXITCODE -eq 0
            Output = $result
        }
    } catch {
        return @{
            Success = $false
            Output = $_.Exception.Message
        }
    }
}

# 主程序
Write-Host ""
Write-ColorOutput "===================================" "Cyan"
Write-ColorOutput "  VRRP 漂移问题远程诊断工具" "Cyan"
Write-ColorOutput "===================================" "Cyan"
Write-Host ""

Write-ColorOutput "目标路由器: $RouterIP" "White"
Write-ColorOutput "用户名: $Username" "White"
Write-ColorOutput "VIP: $VIP" "White"
Write-ColorOutput "接口: $Interface" "White"
if ($AutoFix) {
    Write-ColorOutput "模式: 自动修复" "Yellow"
} else {
    Write-ColorOutput "模式: 仅诊断" "White"
}
Write-Host ""

$issuesFound = 0

# 测试 SSH 连接
Write-ColorOutput "0. 测试 SSH 连接..." "White"
$testResult = Invoke-SSHCommand "echo 'OK'"
if ($testResult.Success) {
    Write-Success "SSH 连接成功"
} else {
    Write-Error "SSH 连接失败: $($testResult.Output)"
    Write-Host ""
    Write-ColorOutput "请确保:" "Yellow"
    Write-ColorOutput "  1. 路由器 IP 地址正确" "Yellow"
    Write-ColorOutput "  2. SSH 服务已启用" "Yellow"
    Write-ColorOutput "  3. 用户名和密码正确" "Yellow"
    Write-ColorOutput "  4. 如果使用密钥认证，请不要指定 -Password 参数" "Yellow"
    exit 1
}
Write-Host ""

# 1. 检查 Keepalived 运行状态
Write-ColorOutput "1. 检查 Keepalived 运行状态..." "White"
$result = Invoke-SSHCommand "pgrep -x keepalived || pidof keepalived"
if ($result.Success -and $result.Output -ne "") {
    Write-Success "Keepalived 正在运行 (PID: $($result.Output.Trim()))"
} else {
    Write-Error "Keepalived 未运行"
    $issuesFound++
    
    if ($AutoFix) {
        Write-Info "尝试启动 Keepalived..."
        $startResult = Invoke-SSHCommand "systemctl start keepalived 2>/dev/null || /etc/init.d/keepalived start 2>/dev/null"
        if ($startResult.Success) {
            Write-Success "Keepalived 已启动"
            Start-Sleep -Seconds 2
        } else {
            Write-Error "启动失败: $($startResult.Output)"
        }
    }
}
Write-Host ""

# 2. 检查配置文件
Write-ColorOutput "2. 检查 Keepalived 配置文件..." "White"
$configPath = "/etc/keepalived/keepalived.conf"
$result = Invoke-SSHCommand "test -f $configPath && echo 'EXISTS' || echo 'NOT_FOUND'"
if ($result.Output.Trim() -eq "NOT_FOUND") {
    $configPath = "/tmp/keepalived.conf"
    $result = Invoke-SSHCommand "test -f $configPath && echo 'EXISTS' || echo 'NOT_FOUND'"
}

if ($result.Output.Trim() -eq "EXISTS") {
    Write-Success "配置文件存在: $configPath"
    
    # 验证配置语法
    $validateResult = Invoke-SSHCommand "keepalived -t -f $configPath 2>&1"
    if ($validateResult.Output -match "valid|successful") {
        Write-Success "配置文件语法正确"
    } else {
        Write-Error "配置文件语法错误"
        Write-Host $validateResult.Output
        $issuesFound++
        
        if ($AutoFix) {
            Write-Info "尝试重新生成配置..."
            $applyResult = Invoke-SSHCommand "gateway-agent apply"
            if ($applyResult.Success) {
                Write-Success "配置已重新生成"
            } else {
                Write-Error "重新生成失败: $($applyResult.Output)"
            }
        }
    }
    
    # 检查是否使用单播
    $unicastResult = Invoke-SSHCommand "grep 'unicast_peer' $configPath"
    if ($unicastResult.Success) {
        Write-Success "使用单播模式 (unicast)"
        
        # 提取对端 IP
        $peerResult = Invoke-SSHCommand "grep -A 1 'unicast_peer' $configPath | tail -n 1 | tr -d ' {}'"
        if ($peerResult.Success -and $peerResult.Output.Trim() -ne "") {
            $peerIP = $peerResult.Output.Trim()
            Write-Info "对端 IP: $peerIP"
        }
    } else {
        Write-Warning "使用组播模式 (multicast)"
    }
} else {
    Write-Error "配置文件不存在"
    $issuesFound++
}
Write-Host ""

# 3. 检查防火墙规则
Write-ColorOutput "3. 检查防火墙规则 (VRRP 协议112)..." "White"
$result = Invoke-SSHCommand "iptables -L INPUT -n 2>/dev/null | grep 112"
if ($result.Success -and $result.Output.Trim() -ne "") {
    Write-Success "iptables 已放行 VRRP"
} else {
    Write-Error "iptables 未发现 VRRP 放行规则"
    $issuesFound++
    
    if ($AutoFix) {
        Write-Info "添加 iptables 规则..."
        Invoke-SSHCommand "iptables -I INPUT -p 112 -j ACCEPT 2>/dev/null" | Out-Null
        Invoke-SSHCommand "iptables -I OUTPUT -p 112 -j ACCEPT 2>/dev/null" | Out-Null
        Write-Success "规则已添加"
    }
}

# OpenWrt 特定检查
$openwrtResult = Invoke-SSHCommand "test -f /etc/openwrt_release && echo 'OPENWRT' || echo 'NOT_OPENWRT'"
if ($openwrtResult.Output.Trim() -eq "OPENWRT") {
    $uciResult = Invoke-SSHCommand "uci get firewall.vrrp.target 2>/dev/null"
    if ($uciResult.Success -and $uciResult.Output.Trim() -eq "ACCEPT") {
        Write-Success "OpenWrt 防火墙已配置 VRRP 规则"
    } else {
        Write-Error "OpenWrt 防火墙未配置 VRRP 规则"
        $issuesFound++
        
        if ($AutoFix) {
            Write-Info "配置 OpenWrt 防火墙..."
            Invoke-SSHCommand "uci delete firewall.vrrp 2>/dev/null" | Out-Null
            Invoke-SSHCommand "uci set firewall.vrrp=rule" | Out-Null
            Invoke-SSHCommand "uci set firewall.vrrp.name='Allow-VRRP'" | Out-Null
            Invoke-SSHCommand "uci set firewall.vrrp.src='lan'" | Out-Null
            Invoke-SSHCommand "uci set firewall.vrrp.proto='112'" | Out-Null
            Invoke-SSHCommand "uci set firewall.vrrp.target='ACCEPT'" | Out-Null
            Invoke-SSHCommand "uci commit firewall" | Out-Null
            Invoke-SSHCommand "/etc/init.d/firewall reload" | Out-Null
            Write-Success "OpenWrt 防火墙已配置"
        }
    }
}
Write-Host ""

# 4. 检查 VIP 状态
Write-ColorOutput "4. 检查 VIP 分配状态..." "White"
$result = Invoke-SSHCommand "ip addr show dev $Interface 2>/dev/null | grep '$VIP'"
if ($result.Success -and $result.Output.Trim() -ne "") {
    Write-Success "VIP $VIP 已分配到接口 $Interface"
} else {
    Write-Warning "VIP $VIP 未分配到接口 $Interface (可能处于 BACKUP 状态)"
}
Write-Host ""

# 5. 检查 VRRP 状态文件
Write-ColorOutput "5. 检查 VRRP 状态文件..." "White"
$stateFile = "/tmp/keepalived.GATEWAY.state"
$result = Invoke-SSHCommand "cat $stateFile 2>/dev/null"
if ($result.Success -and $result.Output.Trim() -ne "") {
    $state = $result.Output.Trim()
    Write-Success "状态文件存在: $state"
} else {
    Write-Error "状态文件不存在 (notify 脚本可能未执行)"
    $issuesFound++
    
    if ($AutoFix) {
        Write-Info "测试 notify 脚本..."
        $agentResult = Invoke-SSHCommand "which gateway-agent 2>/dev/null || echo '/gateway-agent/gateway-agent'"
        $agentBin = $agentResult.Output.Trim()
        
        $notifyResult = Invoke-SSHCommand "$agentBin notify TEST 2>&1"
        $checkResult = Invoke-SSHCommand "test -f $stateFile && echo 'EXISTS' || echo 'NOT_FOUND'"
        
        if ($checkResult.Output.Trim() -eq "EXISTS") {
            Write-Success "notify 脚本执行成功"
        } else {
            Write-Error "notify 脚本执行失败: $($notifyResult.Output)"
        }
    }
}
Write-Host ""

# 6. 检查网络接口
Write-ColorOutput "6. 检查网络接口..." "White"
$result = Invoke-SSHCommand "ip link show $Interface 2>&1"
if ($result.Success) {
    Write-Success "接口 $Interface 存在"
    
    # 检查接口状态
    if ($result.Output -match "UP") {
        Write-Success "接口 $Interface 已启用"
    } else {
        Write-Error "接口 $Interface 未启用"
        $issuesFound++
    }
    
    # 检查组播标志
    if ($result.Output -match "MULTICAST") {
        Write-Success "接口支持组播"
    } else {
        Write-Warning "接口不支持组播 (单播模式下可忽略)"
    }
} else {
    Write-Error "接口 $Interface 不存在"
    $issuesFound++
}
Write-Host ""

# 7. 检查对端连通性 (如果是单播模式)
if ($peerIP) {
    Write-ColorOutput "7. 检查对端连通性 (单播模式)..." "White"
    $result = Invoke-SSHCommand "ping -c 1 -W 2 $peerIP 2>&1"
    if ($result.Success -and $result.Output -match "1 received|1 packets received") {
        Write-Success "可以 Ping 通对端 $peerIP"
    } else {
        Write-Error "无法 Ping 通对端 $peerIP"
        $issuesFound++
    }
    Write-Host ""
}

# 8. 检查最近的日志
Write-ColorOutput "8. 检查最近的 Keepalived 日志..." "White"
$logResult = Invoke-SSHCommand "logread 2>/dev/null | grep -i keepalived | tail -n 3"
if ($logResult.Output.Trim() -eq "") {
    $logResult = Invoke-SSHCommand "tail -n 50 /var/log/syslog 2>/dev/null | grep -i keepalived | tail -n 3"
}
if ($logResult.Output.Trim() -eq "") {
    $logResult = Invoke-SSHCommand "journalctl -u keepalived -n 3 --no-pager 2>/dev/null"
}

if ($logResult.Output.Trim() -ne "") {
    $logResult.Output -split "`n" | ForEach-Object {
        if ($_ -match "error|fail|warn") {
            Write-Warning $_
        } else {
            Write-Host "   $_"
        }
    }
} else {
    Write-Warning "未找到日志"
}
Write-Host ""

# 总结
Write-ColorOutput "===================================" "Cyan"
Write-ColorOutput "  诊断总结" "Cyan"
Write-ColorOutput "===================================" "Cyan"
Write-Host ""

if ($issuesFound -eq 0) {
    Write-Success "未发现明显问题"
    Write-Host ""
    Write-ColorOutput "如果 VIP 漂移仍然失败，请检查:" "Yellow"
    Write-ColorOutput "  1. 虚拟化平台网卡设置 (PVE/ESXi 混杂模式)" "Yellow"
    Write-ColorOutput "  2. 交换机是否支持 VRRP" "Yellow"
    Write-ColorOutput "  3. 两个节点是否在同一个二层网络" "Yellow"
} else {
    Write-Error "发现 $issuesFound 个问题"
    
    if (-not $AutoFix) {
        Write-Host ""
        Write-ColorOutput "运行 '.\fix-vrrp.ps1 -RouterIP $RouterIP -Username $Username -AutoFix' 尝试自动修复" "Yellow"
    }
}

Write-Host ""
Write-ColorOutput "详细排查指南: docs/TROUBLESHOOTING-VIP-DRIFT.md" "Cyan"
Write-ColorOutput "===================================" "Cyan"
Write-Host ""

exit $issuesFound
