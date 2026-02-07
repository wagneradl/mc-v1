# Architecture — Memory Cloud MCP Hub

## 1. System Overview

Memory Cloud is a personal MCP Hub deployed on a VPS that aggregates multiple MCP servers behind a single authenticated endpoint. It exposes tools via Streamable HTTP (primary) and SSE (compatibility) for consumption by Claude Desktop, Claude Code, ChatGPT, Cursor, and other MCP-compatible clients.

The system has three layers:

1. **Edge Layer** — Caddy reverse proxy handling TLS termination and Bearer token authentication
2. **Proxy Layer** — mcp-proxy aggregating multiple MCP servers via named server routing
3. **Service Layer** — Individual MCP servers (off-the-shelf + custom Memory MCP)

## 2. Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENTS                                 │
│                                                                 │
│  Claude Desktop ──stdio──▶ mcp-proxy local ──HTTPS──┐           │
│  Claude Code ────────────────────────HTTP/SSE────┐   │          │
│  ChatGPT ────────────────────────────SSE─────┐   │   │          │
│  Cursor/IDEs ────────────────────────HTTP/SSE┤   │   │          │
└──────────────────────────────────────────────┼───┼───┼──────────┘
                                               ▼   ▼   ▼
┌──────────────────────────────────────────────────────────────────┐
│  VPS (mcp.domain.com)                                            │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Caddy                                                     │  │
│  │  - TLS auto-provisioning (Let's Encrypt)                   │  │
│  │  - Bearer token validation                                 │  │
│  │  - Reverse proxy to mcp-proxy:8080                         │  │
│  └─────────────────────┬──────────────────────────────────────┘  │
│                        ▼                                         │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  mcp-proxy (sparfenyuk/mcp-proxy)                          │  │
│  │  - Named server routing                                    │  │
│  │  - stdio ↔ Streamable HTTP bridge                          │  │
│  │  - SSE compatibility                                       │  │
│  │  - Endpoints: /mcp (Streamable HTTP), /sse (legacy)        │  │
│  └──┬──────┬──────┬──────┬──────┬──────┬──────┬──────┬───────┘  │
│     ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼         │
│  [Fire  [Git] [GitHub][Todo  [Brave [Pupp  [Seq.  [Memory       │
│  crawl]               ist]   Search] eteer] Think] MCP]         │
│  (npx)  (uvx)  (npx)  (npx)  (npx)  (npx)  (npx)  (Go bin)    │
│                                                      │          │
│                                              ┌───────┴───────┐  │
│                                              │  SQLite DBs   │  │
│                                              │  _meta.db     │  │
│                                              │  projects/    │  │
│                                              │  archive/     │  │
│                                              └───────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## 3. Technology Stack

| Component | Technology | Version/Variant | Justification |
|-----------|-----------|-----------------|---------------|
| Reverse Proxy + TLS | Caddy | Latest | Auto-TLS (Let's Encrypt), native Bearer token auth via header matcher, zero-config cert renewal |
| MCP Proxy/Bridge | sparfenyuk/mcp-proxy | Latest | stdio→Streamable HTTP bridge, named server routing, already solves aggregation |
| Memory MCP Server | Go | 1.22+ | Performance, single binary, matches SQLite driver choice |
| Memory MCP SDK | modelcontextprotocol/go-sdk | Latest | Official MCP SDK, supports stdio + Streamable HTTP, auto-schema from Go structs |
| SQLite Driver | ncruces/go-sqlite3 | Latest | Pure Go (no CGO), native WAL shared memory, FTS5 + JSON1 built-in |
| Containerization | Docker Compose | v2 | Declarative orchestration, volume management, service dependencies |
| Off-the-shelf MCPs | npm/uvx packages | Various | Standard MCP servers for Firecrawl, Git, GitHub, Todoist, Brave Search, Puppeteer, Sequential Thinking |

## 4. Transport Strategy

### Protocol Support

The MCP spec (2025-03-26) defines three transports:
- **stdio** — local only, used by Claude Desktop
- **Streamable HTTP** — new standard replacing SSE, single `/mcp` endpoint with optional SSE upgrade
- **HTTP+SSE** — deprecated but still widely used, maintained for backward compatibility

### Client Compatibility Matrix

| Client | Native Transport | How it connects to Memory Cloud |
|--------|-----------------|-------------------------------|
| Claude Desktop | stdio only | Local mcp-proxy converts stdio → HTTPS to VPS |
| Claude Code | stdio + HTTP/SSE | Direct HTTPS to VPS endpoint |
| ChatGPT | SSE (assumed) | SSE endpoint on VPS |
| Cursor | stdio + HTTP/SSE (assumed) | Direct HTTPS or local proxy |
| Windsurf | stdio (assumed) | Local proxy or direct HTTPS |

### Design Decision

mcp-proxy exposes both `/mcp` (Streamable HTTP) and `/sse` (legacy SSE) endpoints, covering all current and foreseeable clients without custom gateway code.

## 5. Data Flow

### Request Flow (Claude Desktop example)

```
1. Claude Desktop spawns local mcp-proxy (stdio)
2. User invokes tool (e.g., search_nodes)
3. Local mcp-proxy sends HTTPS POST to mcp.domain.com/mcp
4. Caddy validates Bearer token in Authorization header
5. Caddy proxies to mcp-proxy container:8080
6. mcp-proxy routes to Memory MCP via stdio
7. Memory MCP executes FTS5 query on current project DB
8. Response flows back: Memory MCP → mcp-proxy → Caddy → HTTPS → local proxy → Claude Desktop
```

### Memory MCP Internal Flow

```
1. Client sends switch_project("project-name")
2. Memory MCP looks up project in _meta.db
3. Opens SQLite connection to projects/{id}.db
4. All subsequent CRUD operations use this connection
5. switch_project("other") closes current, opens new
6. archive_project moves DB to archive/ and updates _meta.db status
```

## 6. Security Model

### Authentication
- **Single-user model**: One Bearer token authenticates all requests
- **Token stored in**: `.env` on VPS, never committed to git
- **Validated at**: Caddy layer (before reaching any MCP server)
- **Client-side**: Token configured per-client (env var or config file)

### Network Security
- Caddy exposes only ports 80/443
- MCP proxy port (8080) is internal only (Docker network)
- Individual MCP servers have no exposed ports
- SSH access restricted by IP via firewall
- fail2ban active for SSH brute-force protection

### Container Security
- Non-root user (`1000:1000`) in all containers
- Minimal base images (alpine/distroless)
- Resource limits per container
- Read-only filesystem where possible
- Volume mounts scoped to minimum necessary paths

## 7. Scalability Considerations

This is a personal/single-user system. The architecture is intentionally simple. If future needs arise:

- **More MCP servers**: Add named server entries to mcp-proxy config
- **More projects**: Database-per-project scales linearly with disk
- **Multiple users**: Would require auth upgrade (JWT/OAuth), session-aware routing, and per-user project isolation — out of scope for v1
- **High availability**: Not needed for personal use; single VPS is sufficient
