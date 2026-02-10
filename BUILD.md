# Build Guide

This document explains how to build Pepebot from source and how our CI/CD pipeline works.

## 📋 Table of Contents

- [Local Build](#local-build)
- [Cross-Platform Build](#cross-platform-build)
- [Docker Build](#docker-build)
- [GitHub Actions](#github-actions)
- [Supported Architectures](#supported-architectures)
- [Release Process](#release-process)

## 🔨 Local Build

### Prerequisites

- Go 1.21 or higher
- Git
- Make (optional, for using Makefile)

### Build for Current Platform

```bash
# Using Make
make build

# Or using Go directly
go build -v -o pepebot ./cmd/pepebot
```

### Install to System

```bash
# Install to ~/.local/bin (default)
make install

# Or specify custom prefix
make install INSTALL_PREFIX=/usr/local
```

### Clean Build Artifacts

```bash
make clean
```

## 🌍 Cross-Platform Build

### Build All Supported Platforms

```bash
make build-all
```

This creates binaries for:
- Linux (amd64, arm64, riscv64)
- macOS (amd64, arm64)
- Windows (amd64)

### Build Specific Platform

```bash
# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o pepebot-linux-arm64 ./cmd/pepebot

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o pepebot-darwin-arm64 ./cmd/pepebot

# Windows
GOOS=windows GOARCH=amd64 go build -o pepebot-windows-amd64.exe ./cmd/pepebot

# Linux RISC-V
GOOS=linux GOARCH=riscv64 go build -o pepebot-linux-riscv64 ./cmd/pepebot

# Linux MIPS
GOOS=linux GOARCH=mips go build -o pepebot-linux-mips ./cmd/pepebot
```

## 🐳 Docker Build

### Build Docker Image

```bash
# Build for current architecture
docker build -t pepebot:latest .

# Build with specific version
docker build \
  --build-arg VERSION=v0.1.0 \
  --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t pepebot:v0.1.0 .
```

### Multi-Architecture Build

```bash
# Set up buildx
docker buildx create --use

# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7,linux/riscv64 \
  -t pepebot:latest \
  --push .
```

### Using Docker Compose

```bash
# Copy environment file
cp .env.example .env

# Edit .env with your settings
nano .env

# Start services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## ⚙️ GitHub Actions

We use GitHub Actions for automated builds and releases.

### CI Workflow (`.github/workflows/ci.yml`)

**Triggers:**
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop`

**Jobs:**
- **Test**: Run unit tests with coverage
- **Lint**: Run golangci-lint
- **Build**: Build on Ubuntu, macOS, and Windows
- **Build Cross**: Test cross-compilation
- **Docker**: Test Docker image build

### Release Workflow (`.github/workflows/release.yml`)

**Triggers:**
- Push tags matching `v*` (e.g., `v0.1.0`)
- Manual workflow dispatch

**Jobs:**

1. **Build** - Creates binaries for all supported platforms:
   - Linux: amd64, arm64, armv7, armv6, riscv64, mips, mipsle, mips64, mips64le
   - macOS: amd64, arm64
   - Windows: amd64, arm64
   - FreeBSD: amd64, arm64

2. **Release** - Creates GitHub release with:
   - All binary archives (`.tar.gz`)
   - SHA256 checksums
   - Release notes
   - Installation instructions

3. **Docker** - Builds and pushes multi-arch Docker images to GitHub Container Registry:
   - Platforms: linux/amd64, linux/arm64, linux/arm/v7, linux/riscv64
   - Tags: `latest`, `v0.1.0`, `v0.1`, `v0`

## 🏗️ Supported Architectures

| OS | Architecture | GOARCH | Notes |
|---|---|---|---|
| **Linux** | x86_64 | amd64 | Standard PC/Server |
| | ARM64 | arm64 | Raspberry Pi 4/5, ARM servers |
| | ARMv7 | arm (GOARM=7) | Raspberry Pi 3/4 (32-bit) |
| | ARMv6 | arm (GOARM=6) | Raspberry Pi 1/Zero |
| | RISC-V 64 | riscv64 | SiFive, StarFive boards |
| | MIPS | mips | Big-endian routers |
| | MIPSle | mipsle | Little-endian routers |
| | MIPS64 | mips64 | 64-bit MIPS |
| | MIPS64le | mips64le | 64-bit MIPS (LE) |
| **macOS** | Intel | amd64 | Intel Macs |
| | Apple Silicon | arm64 | M1/M2/M3/M4 Macs |
| **Windows** | x86_64 | amd64 | Standard Windows PC |
| | ARM64 | arm64 | ARM Windows devices |
| **FreeBSD** | x86_64 | amd64 | FreeBSD servers |
| | ARM64 | arm64 | FreeBSD on ARM |

## 🚀 Release Process

### Creating a New Release

1. **Update Version**
   ```bash
   # Update version in code if needed
   nano cmd/pepebot/main.go
   ```

2. **Commit Changes**
   ```bash
   git add .
   git commit -m "Prepare release v0.1.0"
   git push origin main
   ```

3. **Create and Push Tag**
   ```bash
   # Create tag
   git tag -a v0.1.0 -m "Release v0.1.0"

   # Push tag (this triggers the release workflow)
   git push origin v0.1.0
   ```

4. **Monitor GitHub Actions**
   - Go to Actions tab on GitHub
   - Watch the release workflow
   - Verify all builds complete successfully

5. **Verify Release**
   - Check Releases page
   - Download and test a binary
   - Verify checksums

### Manual Release (Workflow Dispatch)

If you need to trigger a release manually:

1. Go to Actions → Release
2. Click "Run workflow"
3. Enter version (e.g., `v0.1.0`)
4. Click "Run workflow"

## 🔍 Build Optimization

### Reducing Binary Size

Our builds use these optimizations:

```bash
go build \
  -trimpath \
  -ldflags="-s -w" \
  -o pepebot \
  ./cmd/pepebot
```

Flags:
- `-trimpath`: Remove file system paths
- `-ldflags="-s -w"`: Strip debug info and symbol table
- `CGO_ENABLED=0`: Static linking (for Linux)

### Build Info

Version and build time are embedded:

```bash
go build \
  -ldflags="-X main.version=v0.1.0 -X main.buildTime=2026-01-01T00:00:00Z" \
  ./cmd/pepebot
```

Check build info:
```bash
./pepebot version
```

## 🧪 Testing Builds

### Test Current Platform

```bash
make build
./build/pepebot version
./build/pepebot onboard
```

### Test Cross-Compilation

```bash
# Build all platforms
make build-all

# Check binaries
ls -lh build/
```

### Test Docker Image

```bash
# Build and run
docker build -t pepebot:test .
docker run --rm pepebot:test version

# Test with compose
docker-compose up -d
docker-compose exec pepebot pepebot status
docker-compose down
```

## 📦 Binary Size Comparison

Approximate sizes after optimization:

| Platform | Size (MB) | Notes |
|---|---|---|
| Linux amd64 | ~8-10 | Standard |
| Linux arm64 | ~8-10 | Same as amd64 |
| Linux riscv64 | ~8-10 | Same as amd64 |
| macOS amd64 | ~9-11 | Slightly larger |
| macOS arm64 | ~9-11 | Same as amd64 |
| Windows amd64 | ~9-11 | .exe overhead |

## 🐛 Troubleshooting

### Build Fails on macOS

```bash
# Install Xcode Command Line Tools
xcode-select --install
```

### Cross-compilation Issues

```bash
# Clean and retry
make clean
go clean -cache
make build-all
```

### Docker Build Slow

```bash
# Use BuildKit
export DOCKER_BUILDKIT=1
docker build -t pepebot:latest .
```

### GOPATH Issues

```bash
# Ensure Go is properly configured
go env GOPATH
go env GOROOT

# Clean module cache
go clean -modcache
```

## 📚 Additional Resources

- [Go Cross Compilation](https://go.dev/doc/install/source#environment)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Multi-Platform](https://docs.docker.com/build/building/multi-platform/)
- [Makefile Documentation](https://www.gnu.org/software/make/manual/)

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on contributing to Pepebot.

---

Built with 🐸 by Pepebot Contributors
