# Infrastructure — VPS, Docker, Caddy, Security

## 1. VPS Requirements

### Minimum Specs
- **CPU:** 2 vCPUs
- **RAM:** 4GB (8GB preferred for Firecrawl)
- **Disk:** 40GB SSD
- **OS:** Ubuntu 22.04+ LTS
- **Network:** Public IPv4, ports 80/443/SSH

### Software Prerequisites
- Docker Engine 24+
- Docker Compose v2
- UFW (firewall)
- fail2ban

## 2. Directory Structure (VPS)

```
/opt/mcp-hub/
├── docker-compose.yml          # Service orchestration (3 services)
├── Dockerfile.mcp-proxy        # Multi-stage: Go build + mcp-proxy extension
├── Caddyfile                   # Reverse proxy + auth routing
├── mcp-servers.json            # MCP server definitions (8 servers)
├── .env                        # Secrets (NEVER committed)
├── .env.example                # Template for .env
├── .gitignore
├── update.sh                   # Deploy script (pull/rebuild/restart)
├── backup.sh                   # SQLite backup script
├── memory-mcp/                 # Custom Memory MCP source (Go)
│   ├── main.go
│   ├── go.mod / go.sum
│   └── internal/
├── oauth-server/               # OAuth 2.1 Authorization Server (Go)
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod / go.sum
├── certs/                      # Cloudflare Origin Certificate
│   ├── origin.pem
│   └── origin-key.pem
├── backups/                    # Daily SQLite backups (30-day retention)
│   └── memory-backup-*.tar.gz
└── docs/                       # Project documentation
```

## 3. Environment Variables (.env)

```bash
# Authentication — Bearer token for HTTPS access
MCP_BEARER_TOKEN=<generate-with-openssl-rand-hex-32>

# Domain — public hostname served by Caddy
DOMAIN=api.yourdomain.com

# MCP Server API Keys
GITHUB_TOKEN=<github-personal-access-token>
FIRECRAWL_API_KEY=<firecrawl-api-key>
BRAVE_API_KEY=<brave-search-api-key>
TODOIST_API_KEY=<todoist-api-key>

# OAuth Authorization Server (Phase 4 — ChatGPT)
OAUTH_AUTHORIZE_PASSWORD=<generate-with-openssl-rand-hex-16>
# MCP_BEARER_TOKEN above is reused as OAuth access_token
```

Generate secrets:
```bash
openssl rand -hex 32   # MCP_BEARER_TOKEN
openssl rand -hex 16   # OAUTH_AUTHORIZE_PASSWORD
```

## 4. Docker Compose

The stack has 3 services: Caddy (edge), OAuth Server (auth), and mcp-proxy (aggregator).

```yaml
services:
  # ── Caddy (TLS + Auth) ─────────────────────────────
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    environment:
      DOMAIN: ${DOMAIN}
      MCP_BEARER_TOKEN: ${MCP_BEARER_TOKEN}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - ./certs:/etc/caddy/certs:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - mcp-proxy
      - oauth-server
    networks:
      - mcp-net

  # ── OAuth Authorization Server (Phase 4) ────────────
  oauth-server:
    build: ./oauth-server
    container_name: mcp-oauth
    restart: unless-stopped
    environment:
      - OAUTH_AUTHORIZE_PASSWORD=${OAUTH_AUTHORIZE_PASSWORD}
      - MCP_BEARER_TOKEN=${MCP_BEARER_TOKEN}
      - OAUTH_SERVER_BASE_URL=https://${DOMAIN}
      - MCP_RESOURCE_URL=https://${DOMAIN}
      - PORT=8090
    expose:
      - "8090"
    networks:
      - mcp-net
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8090/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  # ── MCP Proxy (Aggregator + all MCP servers) ───────
  mcp-proxy:
    build:
      context: .
      dockerfile: Dockerfile.mcp-proxy
    restart: unless-stopped
    expose:
      - "8080"
    environment:
      GITHUB_TOKEN: ${GITHUB_TOKEN}
      GITHUB_PERSONAL_ACCESS_TOKEN: ${GITHUB_TOKEN}
      FIRECRAWL_API_KEY: ${FIRECRAWL_API_KEY}
      FIRECRAWL_ALLOWED_HOSTS: ${FIRECRAWL_ALLOWED_HOSTS:-}
      BRAVE_API_KEY: ${BRAVE_API_KEY}
      TODOIST_API_KEY: ${TODOIST_API_KEY}
    volumes:
      - memory_data:/data/memory
    networks:
      - mcp-net
    deploy:
      resources:
        limits:
          memory: 2G

volumes:
  caddy_data:
  caddy_config:
  memory_data:

networks:
  mcp-net:
    driver: bridge
```

### Key Design Decisions

- **Multi-stage Dockerfile.mcp-proxy:** Builds the Memory MCP Go binary in a `golang:alpine` stage, then extends the `mcp-proxy` base image with Node.js (for npx MCPs), uv (for uvx MCPs), git, and the compiled binary.
- **mcp-servers.json:** Server definitions are in a separate JSON file loaded via `--named-server-config`, not inline `--named-server` flags. This keeps docker-compose.yml clean.
- **`--pass-environment`:** The mcp-proxy passes its environment variables to all child MCP server processes, so API keys defined in `environment:` are available to each server.

### MCP Server Definitions (mcp-servers.json)

```json
{
  "mcpServers": {
    "memory":    { "command": "/usr/local/bin/memory-mcp", "args": ["--transport", "stdio", "--data-dir", "/data/memory"], "transportType": "stdio" },
    "github":    { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"], "transportType": "stdio" },
    "brave":     { "command": "npx", "args": ["-y", "@brave/brave-search-mcp-server"], "transportType": "stdio" },
    "todoist":   { "command": "npx", "args": ["-y", "@doist/todoist-ai"], "transportType": "stdio" },
    "git":       { "command": "uvx", "args": ["mcp-server-git"], "transportType": "stdio" },
    "firecrawl": { "command": "npx", "args": ["-y", "firecrawl-mcp"], "transportType": "stdio" },
    "thinking":  { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"], "transportType": "stdio" }
  }
}
```

## 5. Caddyfile

Uses `route {}` directive for explicit first-match-wins ordering. OAuth/discovery endpoints are unauthenticated; MCP endpoints require Bearer token.

```
{$DOMAIN} {
    tls /etc/caddy/certs/origin.pem /etc/caddy/certs/origin-key.pem

    route {
        # 1. OAuth discovery — NO auth
        reverse_proxy /.well-known/oauth-protected-resource oauth-server:8090
        reverse_proxy /.well-known/oauth-authorization-server oauth-server:8090

        # 2. OAuth flow — NO auth
        reverse_proxy /oauth/* oauth-server:8090

        # 3. MCP endpoints — Bearer auth required
        @authorized {
            header Authorization "Bearer {$MCP_BEARER_TOKEN}"
        }
        reverse_proxy @authorized mcp-proxy:8080 {
            flush_interval -1
            transport http {
                read_timeout 0
                write_timeout 0
            }
        }

        # 4. Catch-all — 401
        respond "Unauthorized" 401
    }

    log {
        output file /var/log/caddy/access.log
        format json
    }
}
```

### Key Caddy Settings

- **`route {}`** — Explicit first-match-wins ordering (required for coexisting auth/no-auth paths). Do NOT use `handle` blocks — they don't guarantee evaluation order with named matchers.
- **`flush_interval -1`** — Disables response buffering (critical for SSE)
- **`read_timeout 0` / `write_timeout 0`** — No timeout on streaming connections
- **Cloudflare Origin Certificate** — TLS terminated at Cloudflare edge; Caddy uses origin cert for Cloudflare-to-origin encryption (not Let's Encrypt).

## 6. Security Hardening

### 6.1 Firewall (UFW)

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow from YOUR_IP to any port 22
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 6.2 SSH Hardening

```bash
# /etc/ssh/sshd_config
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
AllowUsers deploy
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

## 7. DNS Configuration

Domain is behind Cloudflare (proxied):

```
Type: A
Name: api
Value: <VPS_IP>
Proxy: Enabled (orange cloud)
TTL: Auto
```

Cloudflare handles edge TLS (client → Cloudflare). Caddy uses a Cloudflare Origin Certificate for the Cloudflare → origin connection (`certs/origin.pem`).

## 8. Operations

### Deploy (from local Mac)

```bash
# 1. Push changes to GitHub
git add -A && git commit -m "feat: ..." && git push origin main

# 2. Sync to VPS (excludes secrets and data)
rsync -avz --exclude='.git' --exclude='.env' --exclude='data/' --exclude='certs/' \
  /Users/wagner/Projetos/memory-cloud/ deploy@46.225.69.233:/opt/mcp-hub/

# 3. Rebuild on VPS
ssh deploy@46.225.69.233 "cd /opt/mcp-hub && ./update.sh --rebuild"
```

### update.sh

```bash
./update.sh              # Pull latest base images, restart
./update.sh --rebuild    # Rebuild mcp-proxy image (after code changes), restart
```

### Backup

Daily backup of Memory MCP SQLite databases. 30-day retention.

```bash
./backup.sh              # Manual backup
# Automated via root crontab: 0 3 * * * /opt/mcp-hub/backup.sh
```

### Logs

```bash
docker compose logs -f caddy       # Caddy access logs
docker compose logs -f mcp-proxy   # MCP server output
docker compose logs -f oauth-server # OAuth flow logs
```

## 9. Deployment Checklist

- [x] VPS provisioned (Ubuntu 22.04, 8GB RAM)
- [x] Docker + Docker Compose installed
- [x] SSH key configured, `deploy` user created
- [x] Cloudflare DNS A record (proxied)
- [x] Cloudflare Origin Certificate in `certs/`
- [x] `.env` file created with all secrets
- [x] `docker compose up -d` successful (3 services)
- [x] Bearer token auth verified
- [x] All 8 MCP servers responding via proxy
- [x] OAuth discovery endpoints working (`.well-known`)
- [x] OAuth flow tested (DCR → Authorize → Token → MCP access)
- [x] Claude Desktop configured
- [x] Claude Code configured
- [x] ChatGPT Apps configured (8 apps)
- [x] Backup script running daily
