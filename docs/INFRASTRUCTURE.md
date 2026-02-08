# Infrastructure — VPS, Docker, Caddy, Security

## 1. VPS Requirements

### Minimum Specs
- **CPU:** 2 vCPUs
- **RAM:** 4GB (8GB preferred for Puppeteer/Firecrawl)
- **Disk:** 40GB SSD
- **OS:** Ubuntu 24.04 LTS
- **Network:** Public IPv4, ports 80/443/SSH

### Software Prerequisites
- Docker Engine 24+
- Docker Compose v2
- UFW (firewall)
- fail2ban

## 2. Directory Structure (VPS)

```
/opt/mcp-hub/
├── docker-compose.yml          # Service orchestration
├── Caddyfile                   # Reverse proxy config
├── .env                        # Secrets (NEVER committed)
├── memory-mcp/                 # Custom Memory MCP source
│   ├── Dockerfile
│   ├── main.go
│   ├── go.mod / go.sum
│   └── internal/
├── data/                       # Persistent data (volume-mounted)
│   └── memory/
│       ├── _meta.db
│       ├── projects/
│       └── archive/
└── caddy/
    └── data/                   # Caddy TLS certificates
```

## 3. Environment Variables (.env)

```bash
# Authentication
MCP_BEARER_TOKEN=<generate-strong-random-token>

# MCP Server API Keys
GITHUB_TOKEN=<github-personal-access-token>
FIRECRAWL_API_KEY=<firecrawl-api-key>
BRAVE_API_KEY=<brave-search-api-key>
TODOIST_API_TOKEN=<todoist-api-token>

# Domain
DOMAIN=mcp.yourdomain.com

# OAuth Authorization Server (Phase 4 — ChatGPT)
OAUTH_AUTHORIZE_PASSWORD=<generate-with-openssl-rand-hex-16>
# MCP_BEARER_TOKEN above is reused as OAuth access_token
```

Generate bearer token:
```bash
openssl rand -hex 32
```

## 4. Docker Compose

```yaml
version: "3.9"

services:
  # ── Caddy (TLS + Auth) ─────────────────────────────
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - mcp-proxy
    networks:
      - mcp-net

  # ── MCP Proxy (Aggregator) ─────────────────────────
  mcp-proxy:
    image: ghcr.io/sparfenyuk/mcp-proxy:latest
    restart: unless-stopped
    expose:
      - "8080"
    command: >
      --port 8080
      --named-server memory "./memory-mcp --transport stdio --data-dir /data/memory"
      --named-server github "npx -y @modelcontextprotocol/server-github"
      --named-server brave "npx -y @anthropic/mcp-brave-search"
      --named-server todoist "npx -y todoist-mcp-server"
      --named-server git "uvx mcp-server-git"
      --named-server firecrawl "npx -y firecrawl-mcp"
      --named-server puppeteer "npx -y @anthropic/mcp-puppeteer"
      --named-server thinking "npx -y @anthropic/mcp-sequential-thinking"
    environment:
      GITHUB_TOKEN: ${GITHUB_TOKEN}
      FIRECRAWL_API_KEY: ${FIRECRAWL_API_KEY}
      BRAVE_API_KEY: ${BRAVE_API_KEY}
      TODOIST_API_TOKEN: ${TODOIST_API_TOKEN}
    volumes:
      - memory_data:/data/memory
      - ./memory-mcp/memory-mcp:/usr/local/bin/memory-mcp:ro
    networks:
      - mcp-net
    deploy:
      resources:
        limits:
          memory: 2G

  # ── Memory MCP (Build) ─────────────────────────────
  memory-mcp-build:
    build:
      context: ./memory-mcp
      dockerfile: Dockerfile
    image: memory-mcp:local
    # This service only builds the image; the binary is mounted into mcp-proxy
    # Alternatively, use a multi-stage approach where mcp-proxy contains the binary
    profiles:
      - build

volumes:
  caddy_data:
  caddy_config:
  memory_data:

networks:
  mcp-net:
    driver: bridge
```

> **Note:** The exact mcp-proxy command syntax may need adjustment based on the version used.
> The Memory MCP binary can either be:
> (a) Built separately and volume-mounted into the mcp-proxy container
> (b) Run as its own container with stdio piped to mcp-proxy
> This will be finalized during Phase 2 development based on mcp-proxy's actual behavior.

## 5. Caddyfile

```
{$DOMAIN} {
    # Bearer token authentication
    @authorized {
        header Authorization "Bearer {$MCP_BEARER_TOKEN}"
    }

    # Authorized requests → proxy to MCP
    handle @authorized {
        reverse_proxy mcp-proxy:8080 {
            # SSE-friendly settings
            flush_interval -1
            transport http {
                read_timeout 0
                write_timeout 0
            }
        }
    }

    # Unauthorized → 401
    handle {
        respond "Unauthorized" 401
    }

    # Logging
    log {
        output file /var/log/caddy/access.log
        format json
    }
}
```

### Key Caddy Settings for SSE/Streaming
- `flush_interval -1` — disables response buffering (critical for SSE)
- `read_timeout 0` / `write_timeout 0` — no timeout on streaming connections
- These ensure SSE streams and Streamable HTTP long-polls work correctly

## 6. Security Hardening

### 6.1 Firewall (UFW)

```bash
# Reset and set defaults
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (restrict to your IP if possible)
sudo ufw allow from YOUR_IP to any port 22
# Or more broadly: sudo ufw allow 22/tcp

# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable
sudo ufw enable
```

### 6.2 SSH Hardening

```bash
# /etc/ssh/sshd_config
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
AllowUsers youruser
```

### 6.3 fail2ban

```bash
sudo apt install fail2ban
sudo systemctl enable fail2ban

# /etc/fail2ban/jail.local
[sshd]
enabled = true
port = ssh
maxretry = 3
bantime = 3600
```

### 6.4 Docker Security

- All custom containers run as non-root user (UID 1000)
- Resource limits set in compose (`memory: 2G`)
- No privileged containers
- Internal Docker network for service-to-service communication
- Only Caddy exposes ports to host
- Regular image updates (`docker compose pull`)

### 6.5 Regular Maintenance

```bash
# Update system packages
sudo apt update && sudo apt upgrade -y

# Update Docker images
cd /opt/mcp-hub
docker compose pull
docker compose up -d

# Check logs
docker compose logs -f caddy
docker compose logs -f mcp-proxy

# Backup memory data
tar -czf backup-$(date +%Y%m%d).tar.gz data/memory/
```

## 7. DNS Configuration

Point your domain to the VPS IP:

```
Type: A
Name: mcp (or whatever subdomain)
Value: YOUR_VPS_IP
TTL: 300
```

Caddy will automatically provision a Let's Encrypt TLS certificate on first request.

## 8. Deployment Checklist

- [ ] VPS provisioned with Ubuntu 24.04
- [ ] Docker + Docker Compose installed
- [ ] SSH key configured, root login disabled
- [ ] UFW configured (80, 443, SSH only)
- [ ] fail2ban installed and configured
- [ ] DNS A record pointing to VPS IP
- [ ] `.env` file created with all secrets
- [ ] Memory MCP binary built and tested
- [ ] `docker compose up -d` successful
- [ ] TLS certificate provisioned (check Caddy logs)
- [ ] Bearer token auth verified (`curl -H "Authorization: Bearer TOKEN" https://mcp.domain.com/mcp`)
- [ ] All MCP servers responding via proxy
- [ ] Client configurations tested (Claude Desktop, Code)
- [ ] Backup script in place
