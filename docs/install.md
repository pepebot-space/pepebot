# Pepebot Installation Guide

This guide covers all installation methods for Pepebot across different platforms.

## Table of Contents

- [Quick Install](#quick-install)
- [Manual Installation](#manual-installation)
- [Build from Source](#build-from-source)
- [Service Setup](#service-setup)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Quick Install

The automated installer is the easiest way to install Pepebot on Linux, macOS, or FreeBSD.

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

### Download and Inspect

For security-conscious users, download and review the script first:

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh -o install.sh
chmod +x install.sh
./install.sh
```

### What the Installer Does

1. **Detects System**: Automatically identifies your OS and CPU architecture
2. **Downloads Binary**: Fetches the latest release from GitHub
3. **Installs Binary**: Places `pepebot` in `~/.local/bin/`
4. **Updates PATH**: Adds installation directory to your shell PATH
5. **Optional Service Setup**: Prompts to configure systemd (Linux) or launchd (macOS)

### Supported Systems

| OS | Architectures |
|---|---|
| **Linux** | x86_64, ARM64, ARMv7, ARMv6, RISC-V 64, MIPS (all variants) |
| **macOS** | x86_64 (Intel), ARM64 (Apple Silicon) |
| **FreeBSD** | x86_64, ARM64 |

### Custom Installation Directory

Set `INSTALL_DIR` to change the installation location:

```bash
INSTALL_DIR=/usr/local/bin ./install.sh
```

## Package Managers

### Homebrew (macOS and Linux)

Homebrew provides the easiest installation on macOS and Linux systems.

**Prerequisites:**
- [Homebrew](https://brew.sh/) installed

**Installation:**

```bash
# Add Pepebot tap (first time only)
brew tap pepebot-space/tap https://github.com/pepebot-space/homebrew-tap

# Install pepebot
brew install pepebot

# Verify installation
pepebot version
```

**Start as service:**

```bash
# Start service now and on login
brew services start pepebot

# Check service status
brew services list | grep pepebot

# Stop service
brew services stop pepebot

# View logs
tail -f /usr/local/var/log/pepebot.log  # Intel Mac
tail -f /opt/homebrew/var/log/pepebot.log  # Apple Silicon
```

**Update:**

```bash
brew update
brew upgrade pepebot
```

**Uninstall:**

```bash
brew uninstall pepebot
brew untap pepebot-space/tap
```

### Nix (NixOS, Linux, macOS)

Nix provides reproducible builds and easy package management.

**Prerequisites:**
- [Nix](https://nixos.org/download.html) installed

**Installation:**

**Option 1: Direct install from GitHub**

```bash
# Install to user profile
nix-env -if https://github.com/pepebot-space/pepebot/archive/main.tar.gz

# Verify installation
pepebot version
```

**Option 2: Add to NixOS configuration**

Edit `/etc/nixos/configuration.nix`:

```nix
{ config, pkgs, ... }:

let
  pepebot = pkgs.callPackage (pkgs.fetchFromGitHub {
    owner = "pepebot-space";
    repo = "pepebot";
    rev = "v0.4.0";  # Use specific version
    sha256 = "...";  # Fill with correct hash
  }) {};
in
{
  environment.systemPackages = with pkgs; [
    pepebot
  ];
}
```

Then rebuild:

```bash
sudo nixos-rebuild switch
```

**Option 3: Add to home-manager**

Edit `~/.config/nixpkgs/home.nix`:

```nix
{ config, pkgs, ... }:

let
  pepebot = pkgs.callPackage (fetchGit {
    url = "https://github.com/pepebot-space/pepebot";
    ref = "main";
  }) {};
in
{
  home.packages = [ pepebot ];
}
```

Then apply:

```bash
home-manager switch
```

**Update:**

```bash
# Update user profile installation
nix-env -u pepebot

# Or for NixOS/home-manager, update the rev/sha256 and rebuild
```

**Uninstall:**

```bash
nix-env -e pepebot
```

### Docker

Docker provides isolated containerized installation.

**Prerequisites:**
- [Docker](https://docs.docker.com/get-docker/) installed

**Installation:**

**Pull the image:**

```bash
# Pull latest version
docker pull ghcr.io/pepebot-space/pepebot:latest

# Or pull specific version
docker pull ghcr.io/pepebot-space/pepebot:0.4.0

# Verify image
docker images | grep pepebot
```

**Run interactively:**

```bash
# Run setup wizard
docker run -it --rm \
  -v ~/.pepebot:/root/.pepebot \
  ghcr.io/pepebot-space/pepebot:latest onboard

# Interactive chat mode
docker run -it --rm \
  -v ~/.pepebot:/root/.pepebot \
  ghcr.io/pepebot-space/pepebot:latest agent

# One-off command
docker run --rm \
  -v ~/.pepebot:/root/.pepebot \
  ghcr.io/pepebot-space/pepebot:latest status
```

**Run as daemon (gateway mode):**

```bash
# Start gateway
docker run -d \
  --name pepebot \
  --restart unless-stopped \
  -v ~/.pepebot:/root/.pepebot \
  -p 18790:18790 \
  ghcr.io/pepebot-space/pepebot:latest gateway

# Check logs
docker logs -f pepebot

# Stop container
docker stop pepebot

# Remove container
docker rm pepebot
```

**Using Docker Compose:**

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  pepebot:
    image: ghcr.io/pepebot-space/pepebot:latest
    container_name: pepebot
    restart: unless-stopped
    volumes:
      - ~/.pepebot:/root/.pepebot
    ports:
      - "18790:18790"
    command: gateway
    environment:
      - TZ=Asia/Jakarta
    healthcheck:
      test: ["CMD", "pepebot", "status"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Or download the official compose file:

```bash
curl -O https://raw.githubusercontent.com/pepebot-space/pepebot/main/docker-compose.yml
```

Run with Docker Compose:

```bash
# Start service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop service
docker-compose down

# Update to latest version
docker-compose pull
docker-compose up -d
```

**Custom Docker build:**

```bash
# Clone repository
git clone https://github.com/pepebot-space/pepebot.git
cd pepebot

# Build image
docker build -t pepebot:local .

# Run local build
docker run -it --rm pepebot:local version
```

**Available tags:**
- `latest` - Latest stable release
- `0.4.0`, `0.4`, `0` - Semantic version tags
- `main` - Latest main branch build (may be unstable)

**Supported platforms:**
- `linux/amd64` - x86_64 Linux
- `linux/arm64` - ARM64 Linux (Raspberry Pi 4, etc.)

## Manual Installation

### Step 1: Download Binary

Visit [GitHub Releases](https://github.com/pepebot-space/pepebot/releases/latest) and download the appropriate archive for your system.

**Linux x86_64:**
```bash
wget https://github.com/pepebot-space/pepebot/releases/latest/download/pepebot-linux-amd64.tar.gz
```

**macOS Apple Silicon:**
```bash
curl -LO https://github.com/pepebot-space/pepebot/releases/latest/download/pepebot-darwin-arm64.tar.gz
```

**Linux ARM64:**
```bash
wget https://github.com/pepebot-space/pepebot/releases/latest/download/pepebot-linux-arm64.tar.gz
```

### Step 2: Extract Archive

```bash
tar xzf pepebot-*.tar.gz
```

### Step 3: Install Binary

**System-wide installation (requires sudo):**
```bash
sudo mv pepebot-* /usr/local/bin/pepebot
sudo chmod +x /usr/local/bin/pepebot
```

**User installation:**
```bash
mkdir -p ~/.local/bin
mv pepebot-* ~/.local/bin/pepebot
chmod +x ~/.local/bin/pepebot
```

### Step 4: Add to PATH

If using user installation, ensure `~/.local/bin` is in your PATH:

**For bash:**
```bash
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
source ~/.bashrc
```

**For zsh:**
```bash
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.zshrc
source ~/.zshrc
```

### Step 5: Verify Installation

```bash
pepebot version
```

## Build from Source

### Prerequisites

- Go 1.21 or higher
- Git
- Make (optional, but recommended)

### Clone Repository

```bash
git clone https://github.com/pepebot-space/pepebot.git
cd pepebot
```

### Build Binary

**Using Make:**
```bash
make build
```

**Using Go directly:**
```bash
go build -v -o pepebot ./cmd/pepebot
```

### Install Binary

**Using Make:**
```bash
# Install to ~/.local/bin (default)
make install

# Install to custom location
make install INSTALL_PREFIX=/usr/local
```

**Manual installation:**
```bash
# User installation
cp pepebot ~/.local/bin/

# System-wide installation
sudo cp pepebot /usr/local/bin/
```

### Build for Other Platforms

```bash
# Build for all platforms
make build-all

# Build for specific platform
GOOS=linux GOARCH=arm64 go build -o pepebot-linux-arm64 ./cmd/pepebot
```

## Service Setup

Run Pepebot as a background service to ensure it starts automatically.

### Linux (systemd)

Create a systemd user service:

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/pepebot.service << 'EOF'
[Unit]
Description=Pepebot Personal AI Assistant
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/pepebot gateway
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF
```

Enable and start the service:

```bash
systemctl --user daemon-reload
systemctl --user enable pepebot
systemctl --user start pepebot
```

Check status:

```bash
systemctl --user status pepebot
```

View logs:

```bash
journalctl --user -u pepebot -f
```

### macOS (launchd)

Create a launchd agent:

```bash
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.pepebot.agent.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pepebot.agent</string>

    <key>ProgramArguments</key>
    <array>
        <string>/Users/YOUR_USERNAME/.local/bin/pepebot</string>
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
    <string>/Users/YOUR_USERNAME/.pepebot/pepebot.log</string>

    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/.pepebot/pepebot.error.log</string>

    <key>WorkingDirectory</key>
    <string>/Users/YOUR_USERNAME/.pepebot</string>
</dict>
</plist>
EOF
```

**Note:** Replace `YOUR_USERNAME` with your actual username.

Load and start the service:

```bash
launchctl load ~/Library/LaunchAgents/com.pepebot.agent.plist
launchctl start com.pepebot.agent
```

Check logs:

```bash
tail -f ~/.pepebot/pepebot.log
```

### FreeBSD (rc.d)

Create an rc.d script:

```bash
sudo tee /usr/local/etc/rc.d/pepebot << 'EOF'
#!/bin/sh
#
# PROVIDE: pepebot
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pepebot"
rcvar="pepebot_enable"

load_rc_config $name

: ${pepebot_enable:="NO"}
: ${pepebot_user:="pepebot"}
: ${pepebot_config:="/home/${pepebot_user}/.pepebot/config.json"}

pidfile="/var/run/pepebot.pid"
command="/usr/local/bin/pepebot"
command_args="gateway"

run_rc_command "$1"
EOF

sudo chmod +x /usr/local/etc/rc.d/pepebot
```

Enable and start:

```bash
sudo sysrc pepebot_enable="YES"
sudo service pepebot start
```

## Verification

After installation, verify Pepebot is working:

### Check Version

```bash
pepebot version
```

Expected output:
```
🐸 pepebot vX.Y.Z
```

### Run Setup Wizard

```bash
pepebot onboard
```

This will guide you through:
1. Choosing an AI provider
2. Configuring API keys
3. Setting up chat channels
4. Configuring workspace

### Test Interactive Mode

```bash
pepebot agent -m "Hello!"
```

### Check Status

```bash
pepebot status
```

## Troubleshooting

### Binary Not Found

**Problem:** `command not found: pepebot`

**Solution:**
1. Verify installation directory: `ls ~/.local/bin/pepebot`
2. Check PATH includes installation directory: `echo $PATH`
3. Reload shell configuration: `source ~/.bashrc` or `source ~/.zshrc`
4. Try full path: `~/.local/bin/pepebot version`

### Permission Denied

**Problem:** `Permission denied` when running pepebot

**Solution:**
```bash
chmod +x ~/.local/bin/pepebot
# or
chmod +x /usr/local/bin/pepebot
```

### Service Won't Start

**Linux (systemd):**
```bash
# Check status
systemctl --user status pepebot

# View detailed logs
journalctl --user -u pepebot --since "5 minutes ago"

# Restart service
systemctl --user restart pepebot
```

**macOS (launchd):**
```bash
# Check if loaded
launchctl list | grep pepebot

# View logs
cat ~/.pepebot/pepebot.error.log

# Reload service
launchctl unload ~/Library/LaunchAgents/com.pepebot.agent.plist
launchctl load ~/Library/LaunchAgents/com.pepebot.agent.plist
```

### Wrong Architecture

**Problem:** `exec format error` or binary won't run

**Solution:**
1. Check your architecture: `uname -m`
2. Download the correct binary for your system
3. Verify downloaded file: `file pepebot`

Common architectures:
- `x86_64` → download `*-amd64`
- `aarch64` or `arm64` → download `*-arm64`
- `armv7l` → download `*-armv7`
- `riscv64` → download `*-riscv64`

### Installer Fails to Detect System

**Problem:** Installer says "Unsupported system"

**Solution:**
1. Check your OS: `uname -s`
2. Check your architecture: `uname -m`
3. Use manual installation if your system is unusual
4. Report unsupported system at: https://github.com/pepebot-space/pepebot/issues

### curl or tar Not Found

**Problem:** Installer requires `curl` and `tar`

**Solution:**

**Debian/Ubuntu:**
```bash
sudo apt-get install curl tar
```

**RHEL/CentOS/Fedora:**
```bash
sudo yum install curl tar
# or
sudo dnf install curl tar
```

**macOS:**
```bash
# curl and tar are pre-installed
# If missing, install via Homebrew:
brew install curl
```

**FreeBSD:**
```bash
sudo pkg install curl gtar
```

## Uninstallation

### Remove Binary

```bash
# User installation
rm ~/.local/bin/pepebot

# System-wide installation
sudo rm /usr/local/bin/pepebot
```

### Remove Service

**Linux (systemd):**
```bash
systemctl --user stop pepebot
systemctl --user disable pepebot
rm ~/.config/systemd/user/pepebot.service
systemctl --user daemon-reload
```

**macOS (launchd):**
```bash
launchctl stop com.pepebot.agent
launchctl unload ~/Library/LaunchAgents/com.pepebot.agent.plist
rm ~/Library/LaunchAgents/com.pepebot.agent.plist
```

### Remove Configuration and Data

```bash
# WARNING: This deletes all your configuration, skills, and data
rm -rf ~/.pepebot
```

## Next Steps

After installation:

1. **Run Setup:** `pepebot onboard`
2. **Install Skills:** `pepebot skills install-builtin`
3. **Start Chatting:** `pepebot agent`
4. **Start Gateway:** `pepebot gateway`

For more information:
- [README](../README.md) - Overview and features
- [Configuration Guide](../README.md#configuration) - Detailed configuration
- [Workflow Documentation](./workflows.md) - Automation workflows
- [Android Setup](./android.md) - Android/Termux installation

---

**Need Help?**
- Report issues: https://github.com/pepebot-space/pepebot/issues
- Discussions: https://github.com/pepebot-space/pepebot/discussions
