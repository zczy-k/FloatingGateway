#!/bin/bash
# build.sh - Cross-compile gateway-agent and gateway-controller for multiple platforms

set -e

VERSION="${VERSION:-1.0.4}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
MODULE="github.com/zczy-k/FloatingGateway"

# Target platforms
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "linux/mips"
    "linux/mipsle"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Build flags
LDFLAGS="-s -w -X main.version=$VERSION"

log() { echo "[+] $1"; }
error() { echo "[-] $1" >&2; exit 1; }

# Check Go installation
command -v go >/dev/null 2>&1 || error "Go is not installed"

mkdir -p "$OUTPUT_DIR"

# Build agent
build_agent() {
    local os=$1
    local arch=$2
    local ext=""
    
    [ "$os" = "windows" ] && ext=".exe"
    
    output="$OUTPUT_DIR/gateway-agent-$os-$arch$ext"
    
    log "Building gateway-agent for $os/$arch..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags "$LDFLAGS" \
        -o "$output" \
        "$MODULE/cmd/agent"
    
    echo "  -> $output"
}

# Build controller
build_controller() {
    local os=$1
    local arch=$2
    local ext=""
    
    [ "$os" = "windows" ] && ext=".exe"
    
    output="$OUTPUT_DIR/gateway-controller-$os-$arch$ext"
    
    log "Building gateway-controller for $os/$arch..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags "$LDFLAGS" \
        -o "$output" \
        "$MODULE/cmd/controller"
    
    echo "  -> $output"
}

# Main
main() {
    cd "$(dirname "$0")/.."
    
    log "Building version $VERSION"
    log "Output directory: $OUTPUT_DIR"
    echo
    
    # Build for all targets
    for target in "${TARGETS[@]}"; do
        os="${target%/*}"
        arch="${target#*/}"
        
        # Agent: only Linux targets (routers)
        if [ "$os" = "linux" ]; then
            build_agent "$os" "$arch"
        fi
        
        # Controller: all platforms (management machine)
        build_controller "$os" "$arch"
    done
    
    echo
    log "Build complete!"
    ls -la "$OUTPUT_DIR"
}

main "$@"
