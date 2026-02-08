# Memory Cloud

Self-hosted MCP Hub that aggregates 8 MCP servers behind a single authenticated endpoint, with a custom Knowledge Graph server built in Go + SQLite.

**Production:** `https://api.wagnerlima.cc`

---

## What it does

Memory Cloud gives AI assistants persistent, cross-platform memory. A conversation started on Claude Desktop can be continued on ChatGPT mobile — the knowledge graph is the same, accessible from anywhere.

**8 MCP servers, 1 endpoint:**

| Server | Package | Purpose |
|--------|---------|---------|
| **Memory** | Custom (Go) | Knowledge graph with entities, relations, observations, project isolation |
| **GitHub** | @modelcontextprotocol/server-github | Repository management, issues, PRs |
| **Brave Search** | @anthropic/mcp-brave-search | Web search |
| **Todoist** | todoist-mcp-server | Task management |
| **Git** | mcp-server-git | Local git operations |
| **Firecrawl** | firecrawl-mcp | Web scraping and crawling |
| **Puppeteer** | @anthropic/mcp-puppeteer | Browser automation |
| **Sequential Thinking** | @anthropic/mcp-sequential-thinking | Structured reasoning |

---

## Architecture

```
Clients                          VPS (api.wagnerlima.cc)
─────────                        ────────────────────────
Claude Desktop ─stdio─┐
Claude Code ───HTTP────┤         ┌─────────────────────┐
ChatGPT ───────SSE─────┼────────▶│  Caddy (TLS + Auth) │
Cursor/IDEs ───HTTP────┘         └──┬──────────────┬───┘
                                    │              │
                              .well-known    Bearer auth
                              /oauth/*            │
                                    │              ▼
                              ┌─────┴─────┐  ┌─────────────┐
                              │   OAuth    │  │  mcp-proxy  │
                              │  Server    │  │  (8 servers) │
                              │  (Go)     │  └──────┬──────┘
                              └───────────┘         │
                                          ┌─────────┼─────────┐
                                          ▼         ▼         ▼
                                       Memory    GitHub    Brave ...
                                       (Go+SQLite)
```

**Four layers:**
1. **Edge** — Caddy: TLS termination, Bearer token validation
2. **Auth** — OAuth 2.1 Server: Authorization Code + PKCE + DCR (for ChatGPT)
3. **Proxy** — mcp-proxy: named server routing, stdio↔HTTP bridge
4. **Services** — 8 MCP servers (1 custom + 7 off-the-shelf)

---

## Connected Clients

| Client | Transport | Auth |
|--------|-----------|------|
| Claude Desktop | stdio → mcp-proxy local → HTTPS | Bearer token |
| Claude Code | HTTP direct | Bearer token |
| ChatGPT (desktop + mobile) | SSE via OAuth | OAuth 2.1 (PKCE) |
| Cursor / Windsurf | HTTP or stdio | Bearer token |

---

## Quick Start

### Prerequisites

- Docker Engine 24+ & Docker Compose v2
- VPS with public IPv4, ports 80/443
- Domain pointing to VPS (A record)

### Deploy

```bash
# Clone
git clone https://github.com/wagneradl/mc-v1.git
cd mc-v1

# Configure secrets
cp .env.example .env
# Edit .env with your API keys and tokens

# Build Memory MCP binary
cd memory-mcp && docker build -t memory-mcp:local . && cd ..

# Start everything
docker compose up -d

# Verify
curl -H "Authorization: Bearer $MCP_BEARER_TOKEN" \
  https://your-domain.com/servers/memory/sse
```

### Client Setup

See [docs/CLIENT-CONFIG.md](docs/CLIENT-CONFIG.md) for detailed configuration of each client.

---

## Memory MCP

The custom knowledge graph server supports:

- **Entities** with typed observations
- **Directed relations** between entities
- **Project isolation** — each project has its own SQLite database
- **FTS5 full-text search** across entities and observations
- **Archive/restore** for project lifecycle management
- **Soft delete** with audit trail

### Tools

| Tool | Description |
|------|-------------|
| `create_entities` | Create entities with type and observations |
| `search_nodes` | Full-text search (FTS5 syntax: AND, OR, NOT, prefix*) |
| `open_nodes` | Retrieve entities by exact name |
| `read_graph` | Dump entire project graph |
| `create_relations` | Directed relations between entities |
| `add_observations` | Append observations to entities |
| `delete_entities/relations/observations` | Soft delete |
| `create/switch/list/archive/restore/delete_project` | Project management |
| `get_current_project` | Show active project context |

---

## Project Structure

```
memory-cloud/
├── docker-compose.yml          # Service orchestration
├── Caddyfile                   # Reverse proxy + auth routes
├── .env.example                # Environment template
├── memory-mcp/                 # Custom Memory MCP (Go)
│   ├── main.go
│   ├── Dockerfile
│   └── internal/
├── oauth-server/               # OAuth 2.1 AS (Go)
│   ├── main.go
│   └── Dockerfile
├── MEMORY-USER-GUIDE.md        # System prompt for AI agents
└── docs/
    ├── ARCHITECTURE.md         # System design + diagrams
    ├── INFRASTRUCTURE.md       # VPS, Docker, Caddy, security
    ├── CLIENT-CONFIG.md        # Client setup guides
    ├── MEMORY-MCP-SPEC.md      # Memory MCP specification
    ├── ADDING-NEW-MCP.md       # How to add new MCP servers
    ├── DECISIONS.md            # Architecture Decision Records
    ├── DEVELOPMENT-PLAN.md     # Phased development plan
    └── RESEARCH-SUMMARY.md     # Research findings
```

---

## Adding New MCPs

Adding a new MCP server requires only:

1. Add `--named-server` flag to mcp-proxy in `docker-compose.yml`
2. Add API key to `.env` (if needed)
3. Deploy and configure clients

No changes to Caddy, OAuth, or TLS. See [docs/ADDING-NEW-MCP.md](docs/ADDING-NEW-MCP.md) for the full procedure.

---

## Security

- **TLS** — Caddy with auto Let's Encrypt certificates
- **Auth** — Bearer token validated at edge (Caddy layer)
- **OAuth** — Authorization Code + PKCE + DCR for OAuth-only clients
- **Network** — Only ports 80/443 exposed; MCP servers on internal Docker network
- **Containers** — Non-root (UID 1000), resource limits, minimal base images
- **SSH** — Key-only, fail2ban, IP-restricted

---

## Development History

| Phase | Status | Deliverable |
|-------|--------|-------------|
| Phase 1 | ✅ Complete | Memory MCP server (Go + SQLite + FTS5) |
| Phase 2 | ✅ Complete | Docker infrastructure + mcp-proxy aggregation |
| Phase 3 | ✅ Complete | VPS deployment, Caddy, TLS, Claude Desktop + Code |
| Phase 4 | ✅ Complete | OAuth 2.1 server, ChatGPT integration, agent system prompt |

---

## License

Private repository. © Wagner Lima / Logos AI Solutions.
