# Research Summary

Consolidated findings from 6 research phases conducted via Perplexity Pro (February 2025).

## Research 1 — MCP Transport Evolution

### Key Findings
- MCP spec (2025-03-26) defines three transports: **stdio**, **Streamable HTTP** (new), and **HTTP+SSE** (deprecated)
- **Streamable HTTP** replaces HTTP+SSE as the recommended network transport
- Single `/mcp` endpoint replaces separate `/sse` and request endpoints
- Optional SSE upgrade within Streamable HTTP for streaming responses
- Session IDs introduced for reconnection and recovery
- Works cleanly with CDNs, API gateways, serverless (unlike persistent SSE)

### Client Transport Support
- **Claude Desktop**: stdio only; remote requires local proxy bridge
- **Claude Code**: stdio + HTTP/SSE confirmed; Streamable HTTP likely roadmap
- **ChatGPT**: No official MCP transport matrix published; SSE assumed
- **Cursor/Windsurf**: stdio assumed as baseline; HTTP support vendor-specific

### Impact on Architecture
- VPS must expose both Streamable HTTP and SSE for maximum compatibility
- Claude Desktop needs local mcp-proxy (stdio→HTTPS)
- Forward-compatible design: Streamable HTTP primary, SSE as fallback

---

## Research 2 — MCP Gateway/Proxy Solutions

### Key Findings
- **sparfenyuk/mcp-proxy**: stdio↔Streamable HTTP bridge, named server support, good for aggregation
- **Supergateway**: stdio↔SSE/WS bridge, mature but no auth
- **Microsoft mcp-gateway**: Kubernetes-native, stateful routing, enterprise
- **Envoy AI Gateway**: Full enterprise gateway (OAuth, rate limiting, circuit breaking)
- **IBM ContextForge**: Registry, federation, multi-transport, admin UI
- **LobeHub mcp-proxy**: Config-driven aggregation, no auth

### Decision
- **sparfenyuk/mcp-proxy** is the sweet spot for personal use: named servers, Streamable HTTP support, lightweight
- Auth handled at Caddy layer, not in proxy
- Enterprise gateways (Envoy, ContextForge, Microsoft) are overkill for single-user
- No need for custom gateway in Go — mcp-proxy already solves aggregation

---

## Research 3 — VPS Hosting Patterns

### Key Findings
- **Docker Compose** is the standard pattern for multi-MCP deployment
- **Caddy** is ideal for auto-TLS + reverse proxy + Bearer token auth
- Bearer token validation via Caddy `header` matcher is simple and sufficient
- SSE requires `flush_interval -1` and no timeout in Caddy config
- Security hardening: non-root containers, UFW (80/443/SSH only), fail2ban, SSH keys

### Key Configuration
- Caddy auto-provisions Let's Encrypt TLS on first request
- Bearer token auth in Caddyfile via `@authorized` named matcher
- Docker internal network isolates MCP services from public access
- Only Caddy exposes ports to host

---

## Research 4 — Memory MCP Implementations

### Key Findings
- **memory-go**: Knowledge graph model (entities, relations, observations), JSONL persistence, basic
- **Memory SQLite**: ACID/WAL, key-value, good concurrency, lacks graph model
- **basic-memory**: Markdown files, project namespaces, Obsidian integration
- **mem0**: API-backed, relevance scoring, external dependency
- **RAG-memory-mcp**: Hybrid vectors+graph, complex

### Decision
- No existing solution fully covers the requirements (graph model + SQLite + project isolation + CRUD)
- Custom Memory MCP combines:
  - Entity-relation graph model from memory-go
  - SQLite backend with WAL/FTS5 from Memory SQLite approach
  - Project isolation via database-per-project pattern
  - Full CRUD with soft delete and archiving

---

## Research 5 — Go + SQLite Technical Stack

### Key Findings
- **ncruces/go-sqlite3**: Pure Go (WASM/wazero), no CGO, native WAL shared memory, FTS5 + JSON1 built-in, performance close to mattn
- **modernc.org/sqlite**: Pure Go (transpiled C), solid but slightly less WAL support
- **mattn/go-sqlite3**: CGO-based, mature but requires C compiler
- **MCP Go SDK** (`modelcontextprotocol/go-sdk`): Official, supports stdio + Streamable HTTP, auto-schema from Go structs via `mcp.AddTool`
- **FTS5 performance**: ~140ms for 1M records, 30% faster than FTS3
- **Gateway patterns**: SDK supports JSON-RPC routing, middleware for load balancing

### Decision
- **ncruces/go-sqlite3** for zero CGO dependency, clean cross-compilation, native FTS5
- **Official MCP Go SDK** for server implementation
- WAL mode with `busy_timeout = 5000` for concurrent access
- FTS5 virtual tables with triggers for automatic index sync

---

## Research 6 — Multi-Tenant SQLite Patterns

### Key Findings
- **Database-per-tenant**: Best fit for SQLite, maximum isolation, simple backup/archive
- **Schema-per-tenant**: Not practical in SQLite (single schema per connection)
- **Row-level**: Low isolation, leak risk, not recommended for distinct projects
- **Soft delete**: `deleted_at TIMESTAMP NULL` + partial indexes on active records
- **Context switching**: Central metadata DB maps tenants to DB files, dynamic connection switching
- **FTS5 + tenant isolation**: Hybrid indexes support fast, isolated retrieval

### Decision
- **Database-per-project**: Each project gets its own `.db` file
- Central `_meta.db` stores project registry
- Archiving = move `.db` to `archive/` directory + update status
- Soft delete on all knowledge graph records (entities, observations, relations)
- Partial indexes optimize queries over active-only records
