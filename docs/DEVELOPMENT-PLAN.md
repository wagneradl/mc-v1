# Development Plan

## Overview

Three phases, each with clear deliverables and acceptance criteria. Phase 1 is the core development effort (custom code). Phases 2 and 3 are primarily configuration and deployment.

## Phase 1 — Memory MCP Server (Go)

**Goal:** Fully functional Memory MCP binary that passes all tool tests via stdio.

### Task 1.1 — Project Scaffold
- [x] Initialize Go module (`go mod init github.com/wagnerlima/memory-cloud/memory-mcp`)
- [x] Add dependencies: `ncruces/go-sqlite3`, `modelcontextprotocol/go-sdk`, `google/uuid`
- [x] Create directory structure per MEMORY-MCP-SPEC.md §6
- [x] Create Dockerfile (multi-stage build)
- [x] Verify clean build: `go build -o memory-mcp .`

**Acceptance:** Binary compiles, runs, and exits cleanly.

### Task 1.2 — Storage Layer: Schema & Meta DB
- [x] Implement `internal/storage/schema.go` — SQL schema constants, migration logic
- [x] Implement `internal/storage/meta.go` — MetaStore struct wrapping `_meta.db`
  - `Open(dataDir)` → opens/creates `_meta.db`, runs migrations
  - `CreateProject(name, description)` → inserts project, creates DB file
  - `GetProject(name)` → lookup by name
  - `ListProjects(status)` → filtered list
  - `UpdateProjectStatus(name, status)` → archive/restore
  - `DeleteProject(name)` → removes record and DB file
  - `Close()` → cleanup

**Acceptance:** Unit tests pass for all MetaStore operations. `_meta.db` file created correctly.

### Task 1.3 — Storage Layer: Project DB Operations
- [x] Implement `internal/storage/project.go` — ProjectStore struct wrapping a project DB
  - `Open(dbPath)` → opens project DB, runs migrations, configures WAL/PRAGMAs
  - `CreateEntities(entities)` → batch insert with observations
  - `AddObservations(entityName, contents)` → add to existing entity
  - `CreateRelations(relations)` → insert relations
  - `DeleteEntities(names)` → soft delete with cascade
  - `DeleteObservations(entityName, contents)` → soft delete matching observations
  - `DeleteRelations(relations)` → soft delete matching relations
  - `GetEntities(names)` → exact name lookup with observations + relations
  - `ReadGraph()` → full graph dump
  - `Close()` → cleanup

**Acceptance:** Unit tests for all CRUD operations. Soft delete verified. Cascade verified.

### Task 1.4 — Storage Layer: FTS5 Search
- [x] Implement `internal/storage/search.go`
  - `Search(query)` → FTS5 search across entities and observations
  - Merge results: entity matches + entities owning matched observations
  - Load full entity data (observations + relations) for results
  - Handle FTS5 query syntax (AND, OR, NOT, prefix*)

**Acceptance:** Search returns correct results for various query patterns. Performance < 100ms for typical workloads.

### Task 1.5 — Session Management
- [x] Implement `internal/session/session.go`
  - `Session` struct with `CurrentProjectID`, `CurrentProjectName`, `ProjectDB`
  - `SwitchProject(metaStore, name)` → close current, open new
  - `GetCurrent()` → return current project info
  - `Clear()` → close and reset
  - Thread-safe access (sync.Mutex)

**Acceptance:** Session correctly switches between projects. Old connection properly closed.

### Task 1.6 — MCP Tool Handlers
- [x] Implement `internal/tools/projects.go` — handlers for project management tools
  - `handleListProjects`, `handleCreateProject`, `handleSwitchProject`
  - `handleGetCurrentProject`, `handleArchiveProject`, `handleDeleteProject`
  - `handleRestoreProject`
- [x] Implement `internal/tools/knowledge.go` — handlers for knowledge graph tools
  - `handleCreateEntities`, `handleAddObservations`, `handleCreateRelations`
  - `handleSearchNodes`, `handleOpenNodes`, `handleReadGraph`
  - `handleDeleteEntities`, `handleDeleteObservations`, `handleDeleteRelations`
  - All check for active project, return helpful error if none

**Acceptance:** All handlers correctly parse input, call storage layer, format output.

### Task 1.7 — MCP Server Wiring
- [x] Implement `internal/server/server.go` — server setup and tool registration
- [x] Implement `main.go` — entry point with CLI flags
  - `--transport stdio|http`
  - `--port 8081` (for HTTP mode)
  - `--data-dir ./data`
- [x] Register all tools with mcp-go SDK using Go struct schemas
- [x] Wire handlers to session + storage layer

**Acceptance:** Server starts in stdio mode, responds to `initialize`, lists all tools via `tools/list`.

### Task 1.8 — Integration Testing
- [x] Test full workflow via stdio:
  1. `create_project("test-project")`
  2. `switch_project("test-project")` (auto if just created)
  3. `create_entities([{name: "Go", type: "technology", observations: ["Fast compiled language"]}])`
  4. `add_observations({entity_name: "Go", contents: ["Great for CLI tools"]})`
  5. `create_relations([{from: "Go", to: "Memory Cloud", relation_type: "powers"}])`
  6. `search_nodes("Go")`
  7. `read_graph()`
  8. `delete_observations({entity_name: "Go", observations: ["Fast compiled language"]})`
  9. `delete_entities(["Go"])`
  10. `archive_project("test-project")`
  11. `restore_project("test-project")`
  12. `delete_project("test-project")`
- [x] Test error cases: no active project, duplicate names, missing entities
- [ ] Test with MCP Inspector: `npx @modelcontextprotocol/inspector`

**Acceptance:** Full workflow passes. Error messages are clear and actionable.

---

## Phase 2 — Infrastructure & Proxy Setup

**Goal:** All MCPs running behind mcp-proxy in Docker Compose, accessible via HTTPS with auth.

### Task 2.1 — Docker Compose
- [x] Write `docker-compose.yml` with Caddy, mcp-proxy, and Memory MCP
- [x] Configure named servers in mcp-proxy for all 8 MCPs
- [x] Set up volume mounts for memory data and Caddy certs
- [x] Test locally: `docker compose up -d`
- [x] Verify all services start without errors

### Task 2.2 — Caddy Configuration
- [x] Write Caddyfile with Bearer token auth and reverse proxy
- [x] Configure SSE-friendly settings (flush_interval, timeouts)
- [x] Test TLS auto-provisioning
- [x] Test auth: 401 without token, 200 with token

### Task 2.3 — Integration Testing
- [x] Test each MCP server through the proxy (all 8 initialize, Brave + GitHub verified end-to-end)
- [x] Test Memory MCP tools through the full stack (Caddy → proxy → Memory)
- [x] Test SSE endpoint for streaming
- [ ] Test with MCP Inspector through HTTPS (deferred — not blocking)

### Task 2.4 — Environment & Secrets
- [x] Create `.env.example` with all required variables (no real values)
- [x] Document which API keys are needed and where to get them
- [x] Add `.env` to `.gitignore`

---

## Phase 3 — VPS Deployment

**Goal:** Production deployment on VPS, all clients configured and working.

### Task 3.1 — VPS Setup
- [x] Provision VPS (Ubuntu 22.04, 2 vCPU, 8GB RAM, 75GB SSD — Hetzner)
- [x] Install Docker 29.2.1 + Docker Compose v5.0.2
- [x] Configure firewall (UFW: 80, 443, SSH)
- [x] Set up SSH key auth, disable root login (`deploy` user, root blocked)
- [x] Install and configure fail2ban
- [x] Create `/opt/mcp-hub` directory structure

### Task 3.2 — DNS & TLS
- [x] Configure DNS A record for MCP subdomain (Cloudflare proxied)
- [x] Cloudflare Origin Certificate (15-year) configured in Caddy
- [x] SSL mode: Full (strict) on Cloudflare
- [x] Deploy with `docker compose up -d`
- [x] Verify TLS certificate provisioning (Caddy logs confirmed)

### Task 3.3 — Deploy & Verify
- [x] Upload code and configs to VPS via rsync
- [x] Create `.env` with production secrets (DOMAIN=api.wagnerlima.cc)
- [x] `docker compose up -d` — all 8 MCPs initialized
- [x] Smoke test: 401 without token, Memory MCP initialize with token
- [x] Test Brave Search + GitHub MCP end-to-end through production domain
- [x] Test Memory MCP full CRUD workflow (create project, entities, search, delete)

### Task 3.4 — Client Configuration
- [x] Configure Claude Desktop (8 remote via `uvx mcp-proxy` + 3 local: filesystem, desktop-commander, azure-cli)
- [x] Configure Claude Code (8 remote via `claude mcp add --transport http`)
- [x] Test Memory MCP workflow end-to-end from each client
- [x] Document client-specific quirks: Claude Desktop needs `uvx mcp-proxy` bridge for stdio→streamablehttp

### Task 3.5 — Operations Setup
- [x] Set up backup script (`/opt/mcp-hub/backup.sh` — tar.gz of SQLite volume)
- [x] Schedule automated backups (cron daily at 03:00, keep 30 days)
- [x] Document update procedure (`/opt/mcp-hub/update.sh` — pull/rebuild + restart)
- [x] SSH hardened: `deploy` user with docker group, root login disabled
- [ ] Test restore from backup (manual procedure documented)

---

## Phase 4 — OAuth 2.1 Gateway (ChatGPT Integration)

**Goal:** Enable ChatGPT MCP Apps to connect via OAuth 2.1 (Authorization Code + PKCE + DCR).

### Task 4.1 — OAuth Server: Metadata Endpoints
- [x] Create `oauth-server/` Go project (stdlib + google/uuid)
- [x] Implement `GET /.well-known/oauth-protected-resource`
- [x] Implement `GET /.well-known/oauth-authorization-server` (RFC 8414)
- [x] Implement `GET /health`
- [x] Create multi-stage Dockerfile

### Task 4.2 — OAuth Server: Dynamic Client Registration
- [x] Implement `POST /oauth/register` (DCR)
- [x] In-memory client store with sync.RWMutex

### Task 4.3 — OAuth Server: Authorization Endpoint
- [x] Implement `GET /oauth/authorize` — HTML login page
- [x] Implement `POST /oauth/authorize` — password validation, auth code generation
- [x] Auth codes: single-use, 5-minute expiry, periodic cleanup

### Task 4.4 — OAuth Server: Token Endpoint
- [x] Implement `POST /oauth/token` — authorization_code grant with PKCE (S256)
- [x] Implement `POST /oauth/token` — refresh_token grant with rotation
- [x] access_token = MCP_BEARER_TOKEN (zero change to existing auth)

### Task 4.5 — Infrastructure
- [x] Add oauth-server service to docker-compose.yml with healthcheck
- [x] Update Caddyfile: `route` directive for explicit ordering
- [x] Routes: `.well-known/*` and `/oauth/*` → oauth-server (no auth)
- [x] Routes: authorized requests → mcp-proxy (Bearer auth)
- [x] Update .env.example with OAUTH_AUTHORIZE_PASSWORD

### Task 4.6 — Local Testing
- [x] Discovery endpoints return correct metadata
- [x] DCR returns client_id
- [x] Authorization flow: login page, wrong password error, correct password redirect
- [x] Token exchange: PKCE validation, access_token = MCP_BEARER_TOKEN
- [x] One-time code use enforced
- [x] Refresh token rotation
- [x] MCP access with Bearer: 200, without: 401

### Task 4.7 — VPS Deploy
- [x] rsync code to VPS, generate production OAUTH_AUTHORIZE_PASSWORD
- [x] Build and deploy (update.sh --rebuild)
- [x] Fix Caddyfile routing (handle → route directive)
- [x] All production endpoints verified

### Task 4.8 — ChatGPT MCP Apps
- [x] Configuration documented for all 8 MCPs (SSE endpoints + OAuth)

### Task 4.9 — Documentation
- [x] CLIENT-CONFIG.md: ChatGPT section with OAuth flow
- [x] ARCHITECTURE.md: oauth-server component + auth layer
- [x] INFRASTRUCTURE.md: OAUTH_AUTHORIZE_PASSWORD env var
- [x] DEVELOPMENT-PLAN.md: Phase 4 complete

---

## Estimated Effort

| Phase | Estimated Sessions | Complexity | Status |
|-------|-------------------|------------|--------|
| Phase 1 (Memory MCP) | 2-3 sessions | High (custom Go development) | ✅ Complete |
| Phase 2 (Infra + Proxy) | 1 session | Medium (config and wiring) | ✅ Complete |
| Phase 3 (Deploy) | 1 session | Low (standard ops) | ✅ Complete |
| Phase 4 (OAuth Gateway) | 1 session | Medium (Go + infra) | ✅ Complete |

**Total: ~5-6 development sessions**

## Development Session Guidelines

For each session:
1. Start by reading relevant docs from this `/docs` folder
2. Reference MEMORY-MCP-SPEC.md for tool schemas and behavior
3. Reference ARCHITECTURE.md for component relationships
4. Reference INFRASTRUCTURE.md for deployment details
5. Update docs if any decisions change during development
