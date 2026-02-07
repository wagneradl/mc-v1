# Client Configuration Guide

## 1. Overview

Memory Cloud exposes MCP tools via HTTPS on `mcp.yourdomain.com`. Different clients connect using different transport methods.

| Client | Transport | Connection Method |
|--------|-----------|-------------------|
| Claude Desktop | stdio → HTTPS | Local mcp-proxy bridge |
| Claude Code | HTTP/SSE direct | URL + Bearer token |
| ChatGPT | SSE | SSE endpoint URL |
| Cursor | HTTP/SSE | URL + Bearer token |
| Windsurf | stdio/HTTP | Local proxy or direct |

## 2. Claude Desktop

Claude Desktop only speaks stdio natively. You need a local `mcp-proxy` that bridges stdio→HTTPS.

### Install mcp-proxy locally

```bash
# Via pip
pip install mcp-proxy

# Or via npx
npx mcp-proxy --help
```

### Configure `claude_desktop_config.json`

Location:
- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
    "mcpServers": {
        "memory-cloud": {
            "command": "mcp-proxy",
            "args": [
                "--transport", "sse",
                "--sse-url", "https://mcp.yourdomain.com/sse"
            ],
            "env": {
                "AUTHORIZATION": "Bearer YOUR_BEARER_TOKEN"
            }
        }
    }
}
```

> **Note:** The exact args/env syntax depends on the mcp-proxy version. Check
> `mcp-proxy --help` for the current interface. Some versions use `--header`
> for auth instead of env vars.

### Alternative: Streamable HTTP (if supported by mcp-proxy version)

```json
{
    "mcpServers": {
        "memory-cloud": {
            "command": "mcp-proxy",
            "args": [
                "--transport", "streamable-http",
                "--url", "https://mcp.yourdomain.com/mcp"
            ],
            "env": {
                "AUTHORIZATION": "Bearer YOUR_BEARER_TOKEN"
            }
        }
    }
}
```

### Verification

After configuring, restart Claude Desktop. You should see all Memory Cloud tools available in the tools panel (search_nodes, create_entities, switch_project, etc.).

## 3. Claude Code

Claude Code supports HTTP/SSE transports directly.

### Configuration

In your Claude Code MCP settings (IDE-dependent):

```json
{
    "mcpServers": {
        "memory-cloud": {
            "transport": "sse",
            "url": "https://mcp.yourdomain.com/sse",
            "headers": {
                "Authorization": "Bearer YOUR_BEARER_TOKEN"
            }
        }
    }
}
```

Or via CLI:
```bash
claude mcp add memory-cloud --transport sse \
    --url "https://mcp.yourdomain.com/sse" \
    --header "Authorization: Bearer YOUR_BEARER_TOKEN"
```

## 4. ChatGPT

ChatGPT MCP support is evolving. Current approach uses SSE endpoint:

### Configuration

In ChatGPT's MCP/tools configuration:
- **URL:** `https://mcp.yourdomain.com/sse`
- **Authentication:** Bearer token in Authorization header

> **Note:** ChatGPT's MCP integration details may differ from the standard spec.
> Verify the current interface in OpenAI's documentation.

## 5. Cursor

### Configuration

In Cursor settings, add MCP server:

```json
{
    "mcpServers": {
        "memory-cloud": {
            "transport": "sse",
            "url": "https://mcp.yourdomain.com/sse",
            "headers": {
                "Authorization": "Bearer YOUR_BEARER_TOKEN"
            }
        }
    }
}
```

Or use the Cursor MCP settings UI to add a remote server with the SSE URL and auth header.

## 6. Windsurf

Similar to Cursor — add via MCP settings:

```json
{
    "mcpServers": {
        "memory-cloud": {
            "transport": "sse",
            "url": "https://mcp.yourdomain.com/sse",
            "headers": {
                "Authorization": "Bearer YOUR_BEARER_TOKEN"
            }
        }
    }
}
```

## 7. Testing Connection

### Quick test with curl

```bash
# Test auth (should return 401 without token)
curl -s -o /dev/null -w "%{http_code}" https://mcp.yourdomain.com/mcp
# Expected: 401

# Test with token (should return MCP response or 200)
curl -s -H "Authorization: Bearer YOUR_TOKEN" https://mcp.yourdomain.com/mcp
# Expected: MCP protocol response

# Test SSE endpoint
curl -N -H "Authorization: Bearer YOUR_TOKEN" https://mcp.yourdomain.com/sse
# Expected: SSE stream connection
```

### Test with MCP Inspector

```bash
npx @modelcontextprotocol/inspector \
    --transport sse \
    --url "https://mcp.yourdomain.com/sse" \
    --header "Authorization: Bearer YOUR_TOKEN"
```

## 8. Troubleshooting

| Problem | Likely Cause | Fix |
|---------|-------------|-----|
| 401 Unauthorized | Wrong/missing Bearer token | Check token in .env matches client config |
| Connection timeout | Firewall blocking 443 | Check UFW rules on VPS |
| SSL error | DNS not pointing to VPS | Verify A record, wait for propagation |
| Tools not appearing | mcp-proxy not routing correctly | Check mcp-proxy logs: `docker compose logs mcp-proxy` |
| Memory tools error "no active project" | No project selected | Call `switch_project` first |
| SSE connection drops | Caddy buffering responses | Verify `flush_interval -1` in Caddyfile |
