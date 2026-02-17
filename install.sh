#!/bin/bash
# Pepebot Installation Script
# Automatically detects system architecture and installs the latest release
# Supports Linux, macOS, FreeBSD with optional systemd/launchd setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="pepebot-space/pepebot"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
PEPEBOT_HOME="${PEPEBOT_HOME:-$HOME/.pepebot}"

# Functions
print_info() {
    echo -e "${BLUE}ℹ ${NC}$1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_header() {
    echo ""
    echo "     ___"
    echo "    (o o)"
    echo "   (  >  )"
    echo "   /|   |\\"
    echo "  (_|   |_)"
    echo ""
    echo "  🐸 PEPEBOT INSTALLER"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
}

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "darwin";;
        FreeBSD*)   echo "freebsd";;
        *)          echo "unknown";;
    esac
}

detect_arch() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)   echo "amd64";;
        aarch64|arm64)  echo "arm64";;
        armv7l)         echo "armv7";;
        armv6l)         echo "armv6";;
        riscv64)        echo "riscv64";;
        mips)           echo "mips";;
        mipsle)         echo "mipsle";;
        mips64)         echo "mips64";;
        mips64le)       echo "mips64le";;
        *)              echo "unknown";;
    esac
}

check_dependencies() {
    local missing=()

    for cmd in curl tar; do
        if ! command -v "$cmd" &> /dev/null; then
            missing+=("$cmd")
        fi
    done

    if [ ${#missing[@]} -gt 0 ]; then
        print_error "Missing required dependencies: ${missing[*]}"
        print_info "Please install them and try again"
        exit 1
    fi
}

get_latest_version() {
    print_info "Fetching latest release version..."
    local version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        print_error "Failed to fetch latest version"
        exit 1
    fi

    echo "$version"
}

download_release() {
    local version=$1
    local os=$2
    local arch=$3

    local filename="pepebot-${os}-${arch}.tar.gz"
    local url="https://github.com/${REPO}/releases/download/${version}/${filename}"
    local tmp_dir=$(mktemp -d)

    print_info "Downloading ${filename}..."

    if ! curl -fsSL "$url" -o "${tmp_dir}/${filename}"; then
        print_error "Failed to download ${filename}"
        print_error "URL: $url"
        rm -rf "$tmp_dir"
        exit 1
    fi

    print_info "Extracting archive..."
    tar -xzf "${tmp_dir}/${filename}" -C "$tmp_dir"

    # Find the binary (it should be the only executable file)
    local binary=$(find "$tmp_dir" -type f -name "pepebot-*" ! -name "*.tar.gz" ! -name "*.sha256" ! -name "*.txt")

    if [ -z "$binary" ]; then
        print_error "Binary not found in archive"
        rm -rf "$tmp_dir"
        exit 1
    fi

    echo "$binary"
}

install_binary() {
    local binary=$1

    print_info "Installing to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"

    cp "$binary" "${INSTALL_DIR}/pepebot"
    chmod +x "${INSTALL_DIR}/pepebot"

    print_success "Binary installed to ${INSTALL_DIR}/pepebot"
}

setup_systemd() {
    print_info "Setting up systemd service..."

    local service_file="$HOME/.config/systemd/user/pepebot.service"
    mkdir -p "$(dirname "$service_file")"

    cat > "$service_file" << EOF
[Unit]
Description=Pepebot Personal AI Assistant
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/pepebot gateway
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF

    print_success "Systemd service created at $service_file"
    print_info "To enable and start the service:"
    echo "  systemctl --user daemon-reload"
    echo "  systemctl --user enable pepebot"
    echo "  systemctl --user start pepebot"
    echo ""
    print_info "To view logs:"
    echo "  journalctl --user -u pepebot -f"
}

setup_launchd() {
    print_info "Setting up launchd service..."

    local plist_file="$HOME/Library/LaunchAgents/com.pepebot.agent.plist"
    mkdir -p "$(dirname "$plist_file")"

    cat > "$plist_file" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pepebot.agent</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/pepebot</string>
        <string>gateway</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>StandardOutPath</key>
    <string>${PEPEBOT_HOME}/pepebot.log</string>

    <key>StandardErrorPath</key>
    <string>${PEPEBOT_HOME}/pepebot.error.log</string>

    <key>WorkingDirectory</key>
    <string>${PEPEBOT_HOME}</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:${INSTALL_DIR}</string>
    </dict>
</dict>
</plist>
EOF

    print_success "Launchd service created at $plist_file"
    print_info "To load and start the service:"
    echo "  launchctl load $plist_file"
    echo "  launchctl start com.pepebot.agent"
    echo ""
    print_info "To view logs:"
    echo "  tail -f ${PEPEBOT_HOME}/pepebot.log"
    echo "  tail -f ${PEPEBOT_HOME}/pepebot.error.log"
}

add_to_path() {
    local shell_rc=""

    if [ -n "$BASH_VERSION" ]; then
        shell_rc="$HOME/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
        shell_rc="$HOME/.zshrc"
    else
        shell_rc="$HOME/.profile"
    fi

    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        print_info "Adding ${INSTALL_DIR} to PATH in $shell_rc"
        echo "" >> "$shell_rc"
        echo "# Pepebot" >> "$shell_rc"
        echo "export PATH=\"\$PATH:${INSTALL_DIR}\"" >> "$shell_rc"
        print_success "PATH updated in $shell_rc"
        print_warning "Please run: source $shell_rc"
    else
        print_success "${INSTALL_DIR} is already in PATH"
    fi
}

prompt_service_setup() {
    local os=$1

    echo ""
    print_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    if [ "$os" = "linux" ]; then
        echo ""
        print_info "Would you like to set up systemd service?"
        print_info "This allows pepebot gateway to run automatically on boot."
        echo ""
        read -p "Setup systemd service? (y/N): " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            setup_systemd
        else
            print_info "Skipped systemd setup"
            print_info "You can run pepebot manually with: pepebot gateway"
        fi
    elif [ "$os" = "darwin" ]; then
        echo ""
        print_info "Would you like to set up launchd service?"
        print_info "This allows pepebot gateway to run automatically on login."
        echo ""
        read -p "Setup launchd service? (y/N): " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            setup_launchd
        else
            print_info "Skipped launchd setup"
            print_info "You can run pepebot manually with: pepebot gateway"
        fi
    fi
}

main() {
    print_header

    # Check dependencies
    check_dependencies

    # Detect system
    local os=$(detect_os)
    local arch=$(detect_arch)

    print_info "Detected system: ${os}/${arch}"

    if [ "$os" = "unknown" ] || [ "$arch" = "unknown" ]; then
        print_error "Unsupported system: ${os}/${arch}"
        exit 1
    fi

    # Get latest version
    local version=$(get_latest_version)
    print_success "Latest version: $version"

    # Download and install
    local binary=$(download_release "$version" "$os" "$arch")
    install_binary "$binary"

    # Cleanup
    rm -rf "$(dirname "$binary")"

    # Add to PATH
    add_to_path

    # Prompt for service setup
    prompt_service_setup "$os"

    # Print success message
    echo ""
    print_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "     ___"
    echo "    (^ ^)"
    echo "   (  v  )   🎉 INSTALLATION COMPLETE!"
    echo "   /|   |\\"
    echo "  (_|   |_)"
    echo ""
    print_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    print_info "Next steps:"
    echo ""
    echo "  1. Run setup wizard:"
    echo "     pepebot onboard"
    echo ""
    echo "  2. Start chatting:"
    echo "     pepebot agent"
    echo ""
    echo "  3. Or start gateway:"
    echo "     pepebot gateway"
    echo ""
    echo "  4. Check status:"
    echo "     pepebot status"
    echo ""
    print_info "Documentation: https://github.com/${REPO}"
    echo ""
}

# Run main function
main "$@"
