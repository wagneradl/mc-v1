# Adding a New MCP Server to the Hub

Guide for adding new MCP servers to the Memory Cloud hub infrastructure.

---

## Overview

The hub uses `mcp-proxy` with `--named-server` flags to aggregate MCPs. Adding a new one requires **zero changes** to Caddy, OAuth, or TLS — the proxy routes automatically by server name.

After adding, the new MCP is accessible at:
```
https://api.wagnerlima.cc/servers/{name}/sse
```

---

## Step-by-step

### 1. Local — Configure the MCP

**`docker-compose.yml`** — Add a `--named-server` entry to the mcp-proxy command:

```yaml
command: >
  --port 8080
  --named-server memory "./memory-mcp --transport stdio --data-dir /data/memory"
  --named-server github "npx -y @modelcontextprotocol/server-github"
  ...
  --named-server novo-mcp "npx -y @pacote/novo-mcp"
```

**`.env`** — If the MCP requires an API key:

```bash
NOVO_MCP_API_KEY=<your-api-key>
```

**`docker-compose.yml`** — Reference the env var in the mcp-proxy `environment` block:

```yaml
environment:
  GITHUB_TOKEN: ${GITHUB_TOKEN}
  ...
  NOVO_MCP_API_KEY: ${NOVO_MCP_API_KEY}
```

**`.env.example`** — Document the new variable:

```bash
NOVO_MCP_API_KEY=   # API key for Novo MCP (get from https://...)
```

### 2. Local — Test

```bash
# Rebuild and start
docker compose up -d --build

# Test the new MCP endpoint
curl -H "Authorization: Bearer $MCP_BEARER_TOKEN" \
  https://localhost/servers/novo-mcp/sse
```

Verify the MCP initializes and responds. Check logs if needed:

```bash
docker compose logs mcp-proxy | grep novo-mcp
```

### 3. Deploy — VPS

```bash
# SSH into VPS
ssh deploy@46.225.69.233

# Add API key to production .env
cd /opt/mcp-hub
nano .env  # Add NOVO_MCP_API_KEY=<production-key>

# Pull latest code and rebuild
./update.sh --rebuild

# Test in production
curl -H "Authorization: Bearer $MCP_BEARER_TOKEN" \
  https://api.wagnerlima.cc/servers/novo-mcp/sse
```

### 4. Configure Clients

**Claude Desktop** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "novo-mcp": {
      "command": "uvx",
      "args": [
        "mcp-proxy",
        "--transport", "streamablehttp",
        "https://api.wagnerlima.cc/servers/novo-mcp/mcp"
      ],
      "env": {
        "AUTHORIZATION": "Bearer <MCP_BEARER_TOKEN>"
      }
    }
  }
}
```

**Claude Code:**

```bash
claude mcp add --transport http novo-mcp \
  https://api.wagnerlima.cc/servers/novo-mcp/mcp \
  --header "Authorization: Bearer <MCP_BEARER_TOKEN>"
```

**ChatGPT:**

| Field | Value |
|-------|-------|
| Name | Novo MCP |
| Description | Brief description of what the MCP does |
| MCP Server URL | https://api.wagnerlima.cc/servers/novo-mcp/sse |
| Authentication | OAuth |

No need to configure OAuth URLs — ChatGPT discovers everything via `.well-known`. Same credentials as all other Apps.

### 5. Update Documentation

1. **`docs/CLIENT-CONFIG.md`** — Add the new MCP to the client configuration tables
2. **`docs/INFRASTRUCTURE.md`** — Add new env var to the environment variables section
3. **Commit and push:**

```bash
git add -A
git commit -m "feat: add novo-mcp to hub"
git push origin main
```

### 6. (Optional) Register in Memory Cloud

If the MCP is relevant to a project's knowledge graph:

```
create_entities([{
  name: "Novo MCP",
  entity_type: "component",
  observations: ["Description of what it does", "npm package: @pacote/novo-mcp"]
}])
create_relations([{
  from: "mcp-proxy",
  to: "Novo MCP",
  relation_type: "aggregates"
}])
```

---

## What does NOT need to change

| Component | Change needed? | Why |
|-----------|---------------|-----|
| Caddyfile | ❌ No | Bearer auth applies to all `/servers/*` paths already |
| OAuth Server | ❌ No | OAuth protects the whole hub, not individual MCPs |
| TLS/DNS | ❌ No | Same domain, same certificate |
| mcp-proxy image | ❌ No | Just add a `--named-server` flag |

---

## Checklist

- [ ] `docker-compose.yml` — `--named-server` added
- [ ] `.env` / `.env.example` — API key added (if needed)
- [ ] Local test passing
- [ ] VPS `.env` updated with production key
- [ ] VPS deployed and tested
- [ ] Claude Desktop configured
- [ ] Claude Code configured
- [ ] ChatGPT App created
- [ ] `docs/CLIENT-CONFIG.md` updated
- [ ] `docs/INFRASTRUCTURE.md` updated
- [ ] Committed and pushed
