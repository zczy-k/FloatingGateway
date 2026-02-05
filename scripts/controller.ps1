# controller.ps1 - Gateway Controller Windows 安装/卸载脚本
# 用法:
#   安装:   .\controller.ps1 install
#   卸载:   .\controller.ps1 uninstall
#   启动:   .\controller.ps1 start
#   停止:   .\controller.ps1 stop
#   状态:   .\controller.ps1 status

param(
    [Parameter(Position=0)]
    [ValidateSet("install", "uninstall", "start", "stop", "status", "help")]
    [string]$Action = "help"
)

# ============== 配置 ==============
$ControllerName = "gateway-controller"
$InstallDir = "$env:LOCALAPPDATA\GatewayController"
$ConfigDir = "$env:USERPROFILE\.gateway-controller"
$ConfigFile = "$ConfigDir\config.yaml"
$BinaryPath = "$InstallDir\$ControllerName.exe"

# 下载源
$DownloadBase = if ($env:DOWNLOAD_BASE) { $env:DOWNLOAD_BASE } else { "https://github.com/youruser/floatip/releases/latest/download" }

# ============== 工具函数 ==============
function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $color = switch ($Level) {
        "INFO"    { "Green" }
        "WARN"    { "Yellow" }
        "ERROR"   { "Red" }
        "DEBUG"   { "Cyan" }
        default   { "White" }
    }
    $prefix = switch ($Level) {
        "INFO"    { "[+]" }
        "WARN"    { "[!]" }
        "ERROR"   { "[-]" }
        "DEBUG"   { "[*]" }
        default   { "[?]" }
    }
    Write-Host "$prefix $Message" -ForegroundColor $color
}

function Get-Architecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { return "amd64" }
    }
}

# ============== 下载 ==============
function Install-Controller {
    $arch = Get-Architecture
    $binaryName = "$ControllerName-windows-$arch.exe"
    $url = "$DownloadBase/$binaryName"
    
    Write-Log "平台: Windows ($arch)"
    Write-Log "下载 $binaryName..."
    
    # 创建安装目录
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # 下载
    try {
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $url -OutFile $BinaryPath -UseBasicParsing
        Write-Log "已下载到 $BinaryPath"
    }
    catch {
        Write-Log "下载失败: $_" "ERROR"
        exit 1
    }
    
    # 创建配置目录
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    
    # 创建默认配置
    if (-not (Test-Path $ConfigFile)) {
        $defaultConfig = @"
# Gateway Controller 配置文件
version: 1

# HTTP 服务监听地址
listen: ":8080"

# Agent 二进制路径（用于远程部署）
agent_bin: ""

# 共享 LAN 配置
lan:
  vip: ""
  cidr: ""

keepalived:
  vrid: 51

# 管理的路由器列表
routers: []
  # - name: openwrt-main
  #   host: 192.168.1.2
  #   port: 22
  #   user: root
  #   password: ""
  #   key_file: ~/.ssh/id_rsa
  #   role: primary
"@
        $defaultConfig | Out-File -FilePath $ConfigFile -Encoding UTF8
        Write-Log "已创建默认配置: $ConfigFile"
    }
    else {
        Write-Log "配置文件已存在: $ConfigFile"
    }
    
    # 添加到 PATH (当前用户)
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("PATH", "$userPath;$InstallDir", "User")
        Write-Log "已添加到用户 PATH"
    }
    
    Write-Host ""
    Write-Log "安装完成!"
    Write-Host ""
    Write-Log "下一步:" "DEBUG"
    Write-Log "  1. 编辑配置文件: $ConfigFile" "DEBUG"
    Write-Log "  2. 启动 Controller: $ControllerName serve -c $ConfigFile" "DEBUG"
    Write-Log "  3. 或使用: .\controller.ps1 start" "DEBUG"
    Write-Log "  4. 打开浏览器访问: http://localhost:8080" "DEBUG"
}

# ============== 卸载 ==============
function Uninstall-Controller {
    Write-Log "开始卸载 Gateway Controller..."
    
    # 停止进程
    Stop-Controller -Silent
    
    # 删除二进制
    if (Test-Path $BinaryPath) {
        Remove-Item $BinaryPath -Force
        Write-Log "已删除: $BinaryPath"
    }
    
    # 删除安装目录（如果为空）
    if ((Test-Path $InstallDir) -and ((Get-ChildItem $InstallDir | Measure-Object).Count -eq 0)) {
        Remove-Item $InstallDir -Force
    }
    
    # 从 PATH 移除
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -like "*$InstallDir*") {
        $newPath = ($userPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Log "已从 PATH 移除"
    }
    
    # 询问是否删除配置
    if (Test-Path $ConfigDir) {
        $confirm = Read-Host "是否删除配置目录 $ConfigDir? [y/N]"
        if ($confirm -match "^[Yy]") {
            Remove-Item $ConfigDir -Recurse -Force
            Write-Log "已删除配置目录"
        }
        else {
            Write-Log "保留配置目录: $ConfigDir" "DEBUG"
        }
    }
    
    Write-Log "卸载完成"
}

# ============== 启动 ==============
function Start-Controller {
    if (-not (Test-Path $BinaryPath)) {
        Write-Log "Controller 未安装，请先运行: .\controller.ps1 install" "ERROR"
        exit 1
    }
    
    # 检查是否已运行
    $process = Get-Process -Name $ControllerName -ErrorAction SilentlyContinue
    if ($process) {
        Write-Log "Controller 已在运行 (PID: $($process.Id))" "WARN"
        return
    }
    
    # 启动
    $configArg = if (Test-Path $ConfigFile) { "-c `"$ConfigFile`"" } else { "" }
    
    Write-Log "启动 Controller..."
    Start-Process -FilePath $BinaryPath -ArgumentList "serve $configArg" -WindowStyle Hidden
    
    Start-Sleep -Seconds 1
    
    $process = Get-Process -Name $ControllerName -ErrorAction SilentlyContinue
    if ($process) {
        Write-Log "Controller 已启动 (PID: $($process.Id))"
        Write-Log "访问: http://localhost:8080" "DEBUG"
    }
    else {
        Write-Log "启动失败，请检查日志" "ERROR"
    }
}

# ============== 停止 ==============
function Stop-Controller {
    param([switch]$Silent)
    
    $process = Get-Process -Name $ControllerName -ErrorAction SilentlyContinue
    if ($process) {
        Stop-Process -Name $ControllerName -Force
        if (-not $Silent) {
            Write-Log "Controller 已停止"
        }
    }
    else {
        if (-not $Silent) {
            Write-Log "Controller 未在运行" "WARN"
        }
    }
}

# ============== 状态 ==============
function Get-ControllerStatus {
    Write-Host "=== Gateway Controller 状态 ==="
    
    # 检查安装
    if (Test-Path $BinaryPath) {
        Write-Log "二进制: 已安装 ($BinaryPath)"
        & $BinaryPath version 2>$null
    }
    else {
        Write-Log "二进制: 未安装" "WARN"
    }
    
    # 检查配置
    if (Test-Path $ConfigFile) {
        Write-Log "配置: $ConfigFile"
    }
    else {
        Write-Log "配置: 未找到" "WARN"
    }
    
    # 检查运行状态
    $process = Get-Process -Name $ControllerName -ErrorAction SilentlyContinue
    if ($process) {
        Write-Log "服务: 运行中 (PID: $($process.Id))"
    }
    else {
        Write-Log "服务: 未运行" "WARN"
    }
}

# ============== 帮助 ==============
function Show-Help {
    Write-Host @"
Gateway Controller Windows 安装脚本

用法: .\controller.ps1 <命令>

命令:
  install     安装 Controller
  uninstall   卸载 Controller
  start       启动 Controller
  stop        停止 Controller
  status      查看状态
  help        显示帮助

环境变量:
  DOWNLOAD_BASE   下载基础 URL (默认: GitHub Releases)

示例:
  # 安装
  .\controller.ps1 install

  # 自定义下载源
  `$env:DOWNLOAD_BASE = "https://myserver.com/releases"
  .\controller.ps1 install

  # 启动
  .\controller.ps1 start

  # 卸载
  .\controller.ps1 uninstall
"@
}

# ============== 主入口 ==============
switch ($Action) {
    "install"   { Install-Controller }
    "uninstall" { Uninstall-Controller }
    "start"     { Start-Controller }
    "stop"      { Stop-Controller }
    "status"    { Get-ControllerStatus }
    "help"      { Show-Help }
    default     { Show-Help }
}
