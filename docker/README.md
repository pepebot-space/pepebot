# Docker Deployment Guide

## 🐳 Features

The Pepebot Docker image includes:
- ✅ **Cron daemon** - Schedule periodic tasks
- ✅ **Tmux** - Terminal multiplexer for session management
- ✅ **Systemctl replacement** - Service management in containers
- ✅ **Ubuntu 24.04 LTS** - Stable and familiar base image
- ✅ **Common utilities** - vim, nano, htop, curl, ping, etc.

## 🚀 Quick Start

### Using Docker Compose (Recommended)

```bash
# Copy environment template
cp .env.example .env

# Edit configuration
vim .env

# Start service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop service
docker-compose down
```

### Using Docker CLI

```bash
docker run -d \
  --name pepebot \
  -p 18790:18790 \
  -v $(pwd)/data/.pepebot:/root/.pepebot \
  -e PEPEBOT_AGENTS_DEFAULTS_MODEL=maia/gemini-3-pro-preview \
  -e PEPEBOT_PROVIDERS_MAIAROUTER_API_KEY=your-api-key \
  ghcr.io/pepebot-space/pepebot:latest
```

## ⏰ Cron Jobs

The container includes a running cron daemon. You can add cron jobs in two ways:

### Method 1: Mount Crontab File

Create a crontab file:

```bash
# crontab.txt
# Run pepebot cron every 15 minutes
*/15 * * * * /usr/local/bin/pepebot cron >> /var/log/pepebot-cron.log 2>&1

# Custom job example
0 */6 * * * echo "Pepebot is running" >> /var/log/heartbeat.log
```

Mount it in docker-compose.yml:

```yaml
services:
  pepebot:
    volumes:
      - ./crontab.txt:/var/spool/cron/crontabs/root:ro
```

### Method 2: Execute Inside Container

```bash
# Enter container
docker exec -it pepebot sh

# Edit crontab
crontab -e

# Add your jobs
*/15 * * * * /usr/local/bin/pepebot cron >> /var/log/pepebot-cron.log 2>&1

# List current crontab
crontab -l

# View cron logs
tail -f /var/log/pepebot-cron.log
```

### Pepebot Built-in Cron Support

Pepebot has a `cron` command that processes scheduled tasks:

```bash
# Run cron jobs manually
docker exec pepebot pepebot cron

# Schedule with system cron (in container)
*/15 * * * * /usr/local/bin/pepebot cron
```

## 🖥️ Tmux Usage

The container includes tmux for terminal multiplexing:

```bash
# Enter container with bash
docker exec -it pepebot bash

# Start tmux session
tmux new -s pepebot

# List sessions
tmux ls

# Attach to session
tmux attach -t pepebot

# Detach: Press Ctrl+B, then D
```

## 🔧 Service Management

The container includes systemctl replacement for service management:

```bash
# Check service status
docker exec pepebot systemctl status

# View all services
docker exec pepebot systemctl list-units

# Note: This is a docker-systemctl-replacement
# Full systemd features may not be available
```

## 📊 Health Checks

The container includes health check support:

```bash
# Check container health
docker inspect --format='{{.State.Health.Status}}' pepebot

# View health check logs
docker inspect --format='{{json .State.Health}}' pepebot | jq
```

## 🔍 Troubleshooting

### View All Logs

```bash
# Pepebot logs
docker-compose logs -f pepebot

# Cron logs
docker exec pepebot tail -f /var/log/cron.log

# Gateway logs
docker exec pepebot tail -f /var/log/pepebot-gateway.log
```

### Debug Cron Issues

```bash
# Check if cron is running
docker exec pepebot ps aux | grep cron

# Check cron service status (Ubuntu)
docker exec pepebot service cron status

# Verify crontab
docker exec pepebot crontab -l

# Test cron job manually
docker exec pepebot /usr/local/bin/pepebot cron
```

### Restart Services

```bash
# Restart container
docker-compose restart

# Restart only cron daemon (inside container - Ubuntu)
docker exec pepebot service cron restart

# Or kill and restart manually
docker exec pepebot sh -c "service cron stop && service cron start"
```

## 🐛 Common Issues

### Cron Jobs Not Running

1. Check cron daemon (Ubuntu):
   ```bash
   docker exec pepebot ps aux | grep cron
   docker exec pepebot service cron status
   ```

2. Verify crontab syntax:
   ```bash
   docker exec pepebot crontab -l
   ```

3. Check cron logs:
   ```bash
   docker exec pepebot tail -f /var/log/cron.log
   docker exec pepebot tail -f /var/log/pepebot-cron.log
   ```

4. Restart cron service:
   ```bash
   docker exec pepebot service cron restart
   ```

### Permission Issues

Ensure mounted volumes have correct permissions:

```bash
chmod -R 755 data/.pepebot
chmod -R 755 data/workspace
```

## 📦 Building Custom Image

To build with custom modifications:

```bash
# Build
docker build -t pepebot:custom .

# Run
docker run -d --name pepebot pepebot:custom
```

## 🔐 Security Notes

- Store API keys in `.env` file (not in docker-compose.yml)
- Use Docker secrets for production deployments
- Limit container resources with deploy.resources
- Keep base image updated: `docker pull alpine:latest`

## 📚 Additional Resources

- [Pepebot Documentation](https://github.com/pepebot-space/pepebot)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Cron Format Guide](https://crontab.guru/)
- [Tmux Cheat Sheet](https://tmuxcheatsheet.com/)
