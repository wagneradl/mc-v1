# Client Configuration Guide

## 1. Overview

Memory Cloud exposes 8 MCP servers via HTTPS on `api.wagnerlima.cc`. Each server has its own endpoint at `/servers/<name>/mcp`.

| Server | Endpoint | Tools |
|--------|----------|-------|
| memory | `/servers/memory/mcp` | 16 (knowledge graph CRUD) |
| github | `/servers/github/mcp` | 26 (repos, issues, PRs) |
| brave | `/servers/brave/mcp` | 6 (web + local search) |
| todoist | `/servers/todoist/mcp` | 33 (task management) |
| git | `/servers/git/mcp` | 12 (git operations) |
| firecrawl | `/servers/firecrawl/mcp` | 8 (web scraping) |
| puppeteer | `/servers/puppeteer/mcp` | browser automation |
| thinking | `/servers/thinking/mcp` | 1 (sequential thinking) |

### Authentication

All endpoints require Bearer token:
```
Authorization: Bearer <MCP_BEARER_TOKEN>
```

### Transport

- **Streamable HTTP (recommended):** `POST https://api.wagnerlima.cc/servers/<name>/mcp`
- **SSE (legacy):** `GET https://api.wagnerlima.cc/servers/<name>/sse`

## 2. Claude Desktop

Claude Desktop speaks only stdio. Use `uvx mcp-proxy` as a bridge (stdio → streamable HTTP).

### Prerequisites

```bash
# uv/uvx must be installed (pip install uv, or brew install uv)
uvx mcp-proxy --version  # verify it works
```

### Configuration

Location: `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS)

Each remote server needs its own entry:

```json
{
  "mcpServers": {
    "memory-cloud": {
      "command": "uvx",
      "args": [
        "mcp-proxy",
        "--transport", "streamablehttp",
        "-H", "Authorization", "Bearer YOUR_BEARER_TOKEN",
        "https://api.wagnerlima.cc/servers/memory/mcp"
      ]
    },
    "brave-search": {
      "command": "uvx",
      "args": [
        "mcp-proxy",
        "--transport", "streamablehttp",
        "-H", "Authorization", "Bearer YOUR_BEARER_TOKEN",
        "https://api.wagnerlima.cc/servers/brave/mcp"
      ]
    }
  }
}
```

Repeat the pattern for each server, changing only the server name in the URL.

### Verification

Restart Claude Desktop after editing. All remote tools should appear in the tools panel.

## 3. Claude Code

Claude Code supports HTTP transport natively — no bridge needed.

### Add servers via CLI

```bash
TOKEN="Bearer YOUR_BEARER_TOKEN"
BASE="https://api.wagnerlima.cc/servers"

claude mcp add memory-cloud "$BASE/memory/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add github-remote "$BASE/github/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add brave-search "$BASE/brave/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add todoist-remote "$BASE/todoist/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add git-remote "$BASE/git/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add firecrawl-remote "$BASE/firecrawl/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add puppeteer-remote "$BASE/puppeteer/mcp" --transport http --scope user --header "Authorization: $TOKEN"
claude mcp add thinking-remote "$BASE/thinking/mcp" --transport http --scope user --header "Authorization: $TOKEN"
```

### Verification

```bash
claude mcp list
```

## 4. Cursor / Windsurf / Other Clients

Most modern MCP clients support HTTP or SSE transport:

```json
{
  "mcpServers": {
    "memory-cloud": {
      "transport": "streamable-http",
      "url": "https://api.wagnerlima.cc/servers/memory/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_BEARER_TOKEN"
      }
    }
  }
}
```

## 5. Testing Connection

```bash
# Test auth (should return 401)
curl -s -o /dev/null -w "%{http_code}" https://api.wagnerlima.cc/servers/memory/mcp

# Test with token (should return MCP initialize response)
curl -s -X POST https://api.wagnerlima.cc/servers/memory/mcp \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```

## 6. Troubleshooting

| Problem | Likely Cause | Fix |
|---------|-------------|-----|
| 401 Unauthorized | Wrong/missing Bearer token | Check token matches `.env` on VPS |
| Connection timeout | Firewall or Cloudflare issue | Check UFW rules, Cloudflare DNS |
| SSL error | Cert mismatch | Verify Cloudflare SSL mode is "Full (strict)" |
| Tools not appearing | mcp-proxy bridge not running | Check `uvx mcp-proxy` is installed |
| Memory tools error "no active project" | No project selected | Call `switch_project` first |
| SSE connection drops | Cloudflare buffering | Use Streamable HTTP instead of SSE |

## 7. Operations

### Update procedure

SSH to VPS and run:
```bash
ssh deploy@46.225.69.233
cd /opt/mcp-hub

# Standard update (pull latest base images):
./update.sh

# After code changes (rebuild mcp-proxy image):
./update.sh --rebuild
```

### Backup & Restore

Backups run daily at 03:00 UTC via cron:
```bash
# Manual backup:
./backup.sh

# List backups:
ls -la backups/

# Restore from backup:
docker compose down
docker run --rm -v mcp-hub_memory_data:/data -v /opt/mcp-hub/backups:/backup alpine \
  sh -c "rm -rf /data/* && tar -xzf /backup/memory-backup-YYYYMMDD_HHMMSS.tar.gz -C /data"
docker compose up -d
```
