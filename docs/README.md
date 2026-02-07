# Memory Cloud — Documentation Index

## Overview

Memory Cloud is a self-hosted MCP (Model Context Protocol) Hub running on a VPS, serving as a secure gateway for multiple MCP servers accessible via Streamable HTTP, SSE, and stdio (via proxy). The centerpiece is a custom Memory MCP server built in Go with SQLite, featuring project-isolated knowledge graphs with full CRUD, archiving, and context switching.

## Document Index

| Document | Purpose |
|----------|---------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | System architecture, component diagram, technology decisions |
| [MEMORY-MCP-SPEC.md](./MEMORY-MCP-SPEC.md) | Detailed specification for the custom Memory MCP server |
| [INFRASTRUCTURE.md](./INFRASTRUCTURE.md) | VPS setup, Docker Compose, Caddy, security hardening |
| [CLIENT-CONFIG.md](./CLIENT-CONFIG.md) | Configuration guides for Claude Desktop, Code, ChatGPT, IDEs |
| [DEVELOPMENT-PLAN.md](./DEVELOPMENT-PLAN.md) | Phased development plan with tasks and acceptance criteria |
| [RESEARCH-SUMMARY.md](./RESEARCH-SUMMARY.md) | Consolidated findings from all 6 research phases |
| [DECISIONS.md](./DECISIONS.md) | Architecture Decision Records (ADRs) |

## Key Links

- **MCP Spec**: https://modelcontextprotocol.io/specification/2025-06-18
- **Go MCP SDK**: https://github.com/modelcontextprotocol/go-sdk
- **ncruces/go-sqlite3**: https://github.com/ncruces/go-sqlite3
- **mcp-proxy (sparfenyuk)**: https://github.com/sparfenyuk/mcp-proxy
- **Caddy**: https://caddyserver.com/docs

## Project Status

- [x] Research phase complete (6/6 research queries)
- [x] Architecture defined
- [x] Documentation consolidated
- [ ] Phase 1: Memory MCP development
- [ ] Phase 2: Infrastructure + Proxy setup
- [ ] Phase 3: VPS deployment
