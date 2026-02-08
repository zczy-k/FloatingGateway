@echo off
cd /d "%~dp0"
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o gateway-agent-linux-amd64 ./cmd/agent
if %ERRORLEVEL% EQU 0 (
    echo Build successful: gateway-agent-linux-amd64
    dir gateway-agent-linux-amd64
) else (
    echo Build failed with error %ERRORLEVEL%
)
