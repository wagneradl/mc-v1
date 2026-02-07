# Architecture Decision Records (ADRs)

## ADR-001: SQLite Driver — ncruces/go-sqlite3

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Three viable Go SQLite drivers exist: mattn/go-sqlite3 (CGO), modernc.org/sqlite (transpiled), ncruces/go-sqlite3 (WASM/wazero).

**Decision:** Use `ncruces/go-sqlite3`.

**Rationale:**
- Pure Go — no CGO dependency, clean cross-compilation, simpler Docker builds
- Native WAL shared memory support on Unix (mmap) — critical for concurrent read performance
- FTS5 and JSON1 built-in without extra build flags
- Performance within 5-10% of CGO-based mattn in benchmarks
- Active maintenance and growing adoption

**Consequences:**
- Slightly larger binary due to WASM runtime
- Less ecosystem examples than mattn (but API is compatible)

---

## ADR-002: Database-per-Project Isolation

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Need to isolate memory data between projects. Options: database-per-project, row-level isolation with tenant_id, schema-per-project.

**Decision:** Use database-per-project (one `.db` file per project).

**Rationale:**
- SQLite doesn't support schema-per-tenant natively
- Row-level isolation has leak risk and requires discipline in every query
- Database-per-project gives complete isolation with zero risk of cross-contamination
- Trivial backup/archive: copy or move the file
- Trivial deletion: delete the file
- No performance impact from unrelated project data
- Central `_meta.db` provides project registry

**Consequences:**
- File proliferation (mitigated: personal use = limited projects)
- Must manage connection switching (mitigated: simple session state)
- Cannot query across projects (acceptable: cross-project queries not needed)

---

## ADR-003: sparfenyuk/mcp-proxy as Aggregator

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Need to aggregate multiple MCP servers behind a single endpoint. Options: custom Go gateway, Supergateway, mcp-proxy, enterprise gateways (Envoy, ContextForge).

**Decision:** Use `sparfenyuk/mcp-proxy` for MCP aggregation.

**Rationale:**
- Named server routing maps directly to our use case (one name per MCP)
- Supports Streamable HTTP + SSE compatibility
- stdio↔HTTP bridge built in (needed for stdio-only MCP servers)
- Lightweight, single container
- Auth is not needed at this layer (Caddy handles it)
- Enterprise gateways are overkill for single-user personal hub

**Consequences:**
- Dependency on third-party project for core routing
- If mcp-proxy proves limiting, can replace with custom Go gateway using the same MCP SDK
- Must verify exact command syntax for each MCP server during Phase 2

**Fallback:** If mcp-proxy doesn't meet needs, the MCP Go SDK supports building a custom aggregator with JSON-RPC routing.

---

## ADR-004: Caddy for TLS + Auth

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Need TLS termination and authentication for the public endpoint. Options: Nginx + certbot, Caddy, Traefik, cloud load balancer.

**Decision:** Use Caddy.

**Rationale:**
- Auto-TLS via Let's Encrypt with zero configuration beyond domain name
- Native Bearer token validation via header matcher (no plugins needed)
- SSE/streaming-friendly with `flush_interval -1`
- Single Caddyfile config, no separate cert renewal cron
- Low resource footprint
- Widely used in self-hosting community

**Consequences:**
- Less flexible than Nginx for complex routing (not needed here)
- Caddy v2 only (v1 deprecated) — use `caddy:2-alpine` image

---

## ADR-005: Entity-Relation-Observation Knowledge Graph Model

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Need a data model for AI memory storage. Options: key-value, document-based, knowledge graph (entity-relation), hybrid.

**Decision:** Use entity-relation-observation model (knowledge graph).

**Rationale:**
- Proven in existing MCP memory servers (memory-go, Anthropic's reference implementation)
- Entities represent distinct concepts/things with typed classification
- Relations capture directed connections between entities (active voice)
- Observations attach factual statements to entities without rigid schema
- FTS5 search works naturally over entity names and observation content
- Compatible with how LLMs naturally structure knowledge about topics

**Consequences:**
- Slightly more complex than key-value (justified by richer retrieval)
- Must maintain FTS5 triggers for index sync (handled by SQLite triggers)
- Soft delete adds complexity but enables recovery

---

## ADR-006: Soft Delete with Partial Indexes

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Need ability to delete knowledge graph records with recovery option. Options: hard delete, soft delete (deleted_at), event sourcing.

**Decision:** Use soft delete with `deleted_at` column and partial indexes.

**Rationale:**
- Soft delete allows recovery of accidentally deleted data
- `deleted_at IS NULL` filter on all queries ensures deleted data is invisible
- Partial indexes (`WHERE deleted_at IS NULL`) keep query performance optimal for active data
- Event sourcing is overkill for personal memory system
- Hard delete is available for projects (`delete_project`) when permanent removal is desired

**Consequences:**
- All queries must include `WHERE deleted_at IS NULL` (enforced in storage layer)
- DB size grows over time with soft-deleted records (mitigated: personal use = low volume)
- No automatic purge of soft-deleted records (can add later if needed)

---

## ADR-007: Go as Implementation Language

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Memory MCP server needs to be reliable, fast, and deployable as a single binary. Options: Go, Rust, Python, TypeScript/Node.

**Decision:** Use Go.

**Rationale:**
- Official MCP Go SDK available (`modelcontextprotocol/go-sdk`)
- Single binary deployment — no runtime dependencies
- Excellent SQLite driver ecosystem (ncruces — pure Go)
- Fast compilation, strong concurrency primitives
- Small Docker images (alpine/scratch base)
- Wagner's existing experience with Go from previous Memory Cloud work

**Consequences:**
- Less ecosystem than TypeScript for MCP (most community servers are TS/Python)
- Go's error handling verbosity (acceptable trade-off for reliability)

---

## ADR-008: Transport Strategy — Dual SSE + Streamable HTTP

**Status:** Accepted  
**Date:** 2025-02-07

**Context:** Multiple clients need to connect, each with different transport support. Must serve Claude Desktop (stdio), Claude Code (HTTP/SSE), ChatGPT (SSE), and IDEs.

**Decision:** Expose both Streamable HTTP and SSE endpoints via mcp-proxy, with local stdio bridge for Claude Desktop.

**Rationale:**
- Streamable HTTP is the spec's forward direction
- SSE is still widely used and needed for backward compatibility
- mcp-proxy exposes both `/mcp` and `/sse` endpoints natively
- Claude Desktop's stdio limitation is solved by local mcp-proxy bridge
- This covers all known client types without custom code

**Consequences:**
- Claude Desktop users need local mcp-proxy installed
- Must document per-client configuration (see CLIENT-CONFIG.md)
- If a client only supports one transport, it still works
