# Memory MCP Server — Detailed Specification

## 1. Overview

The Memory MCP is a custom MCP server built in Go that provides persistent, project-isolated knowledge graph storage for AI conversations. It implements the entity-relation-observation model with full CRUD operations, FTS5 search, soft delete, and project archiving.

## 2. Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.22+ | Single binary, performance, cross-compilation |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk/mcp` | Official SDK, stdio + Streamable HTTP, auto-schema |
| SQLite Driver | `github.com/ncruces/go-sqlite3` | Pure Go, no CGO, native WAL, FTS5 + JSON1 |
| UUID Generation | `github.com/google/uuid` | Standard UUID v4 for entity IDs |

## 3. Data Model

### 3.1 Metadata Database (`_meta.db`)

Central registry of all projects. Always open.

```sql
CREATE TABLE projects (
    id          TEXT PRIMARY KEY,                          -- UUID v4
    name        TEXT NOT NULL UNIQUE,                      -- human-readable slug
    description TEXT DEFAULT '',
    db_path     TEXT NOT NULL,                             -- relative path: "projects/{id}.db"
    status      TEXT NOT NULL DEFAULT 'active'             -- active | archived
                CHECK(status IN ('active', 'archived')),
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),   -- ISO 8601
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_name ON projects(name);
```

### 3.2 Project Database (`projects/{id}.db`)

Each project has its own SQLite file with identical schema.

```sql
-- Entities: nodes in the knowledge graph
CREATE TABLE entities (
    id          TEXT PRIMARY KEY,                          -- UUID v4
    name        TEXT NOT NULL,
    entity_type TEXT NOT NULL,                             -- e.g., "person", "technology", "concept"
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at  TEXT NULL                                  -- NULL = active, timestamp = soft-deleted
);

-- Observations: facts/notes attached to entities
CREATE TABLE observations (
    id          TEXT PRIMARY KEY,                          -- UUID v4
    entity_id   TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at  TEXT NULL
);

-- Relations: directed edges between entities (active voice)
CREATE TABLE relations (
    id              TEXT PRIMARY KEY,                      -- UUID v4
    from_entity     TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    to_entity       TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relation_type   TEXT NOT NULL,                         -- active voice: "uses", "depends_on", "manages"
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at      TEXT NULL
);

-- FTS5 virtual tables for full-text search
CREATE VIRTUAL TABLE entities_fts USING fts5(
    name,
    entity_type,
    content='entities',
    content_rowid='rowid'
);

CREATE VIRTUAL TABLE observations_fts USING fts5(
    content,
    content='observations',
    content_rowid='rowid'
);

-- Triggers to keep FTS5 in sync
CREATE TRIGGER entities_ai AFTER INSERT ON entities BEGIN
    INSERT INTO entities_fts(rowid, name, entity_type) VALUES (new.rowid, new.name, new.entity_type);
END;
CREATE TRIGGER entities_ad AFTER DELETE ON entities BEGIN
    INSERT INTO entities_fts(entities_fts, rowid, name, entity_type) VALUES('delete', old.rowid, old.name, old.entity_type);
END;
CREATE TRIGGER entities_au AFTER UPDATE ON entities BEGIN
    INSERT INTO entities_fts(entities_fts, rowid, name, entity_type) VALUES('delete', old.rowid, old.name, old.entity_type);
    INSERT INTO entities_fts(rowid, name, entity_type) VALUES (new.rowid, new.name, new.entity_type);
END;

CREATE TRIGGER observations_ai AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, content) VALUES (new.rowid, new.content);
END;
CREATE TRIGGER observations_ad AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;
CREATE TRIGGER observations_au AFTER UPDATE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, content) VALUES('delete', old.rowid, old.content);
    INSERT INTO observations_fts(rowid, content) VALUES (new.rowid, new.content);
END;

-- Partial indexes for efficient queries on active (non-deleted) records
CREATE INDEX idx_entities_active ON entities(name, entity_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_entities_type ON entities(entity_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_observations_entity ON observations(entity_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_relations_from ON relations(from_entity) WHERE deleted_at IS NULL;
CREATE INDEX idx_relations_to ON relations(to_entity) WHERE deleted_at IS NULL;
CREATE INDEX idx_relations_type ON relations(relation_type) WHERE deleted_at IS NULL;
```

### 3.3 SQLite Configuration (per connection)

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA cache_size = -64000;  -- 64MB cache
```

## 4. MCP Tools Specification

### 4.1 Project Management Tools

#### `list_projects`
List all projects with optional status filter.

**Input Schema:**
```json
{
    "status": {
        "type": "string",
        "enum": ["active", "archived", "all"],
        "default": "active",
        "description": "Filter projects by status"
    }
}
```

**Returns:** Array of project objects `{id, name, description, status, created_at, updated_at}`

---

#### `create_project`
Create a new project with its own isolated database.

**Input Schema:**
```json
{
    "name": {
        "type": "string",
        "description": "Unique project name (slug-friendly)"
    },
    "description": {
        "type": "string",
        "description": "Optional project description",
        "default": ""
    }
}
```

**Side Effects:**
1. Creates entry in `_meta.db`
2. Creates `projects/{id}.db` with full schema
3. Automatically switches context to new project

**Returns:** Created project object

---

#### `switch_project`
Switch the active project context for the current session.

**Input Schema:**
```json
{
    "name": {
        "type": "string",
        "description": "Name of the project to switch to"
    }
}
```

**Side Effects:**
1. Closes current project DB connection (if any)
2. Opens target project DB connection
3. Updates session state

**Returns:** Confirmation with project details

**Error:** Returns error if project not found or is archived

---

#### `get_current_project`
Get information about the currently active project.

**Input Schema:** None (no parameters)

**Returns:** Current project object or message indicating no project is active

---

#### `archive_project`
Archive a project (soft operation — preserves data).

**Input Schema:**
```json
{
    "name": {
        "type": "string",
        "description": "Name of the project to archive"
    }
}
```

**Side Effects:**
1. Updates project status to 'archived' in `_meta.db`
2. Moves DB file from `projects/` to `archive/`
3. If archived project was active, clears session context

**Returns:** Confirmation

---

#### `delete_project`
Permanently delete a project and all its data.

**Input Schema:**
```json
{
    "name": {
        "type": "string",
        "description": "Name of the project to permanently delete"
    }
}
```

**Side Effects:**
1. Removes DB file (from projects/ or archive/)
2. Removes entry from `_meta.db`
3. If deleted project was active, clears session context

**Returns:** Confirmation

**Warning:** This is destructive and irreversible.

---

#### `restore_project`
Restore an archived project back to active status.

**Input Schema:**
```json
{
    "name": {
        "type": "string",
        "description": "Name of the archived project to restore"
    }
}
```

**Side Effects:**
1. Moves DB file from `archive/` back to `projects/`
2. Updates status to 'active' in `_meta.db`

**Returns:** Restored project object

---

### 4.2 Knowledge Graph Tools (require active project)

All tools below return an error if no project is currently active. The error message instructs the caller to use `switch_project` first.

#### `create_entities`
Create one or more entities in the knowledge graph.

**Input Schema:**
```json
{
    "entities": {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "name": { "type": "string", "description": "Entity name" },
                "entity_type": { "type": "string", "description": "Entity type (e.g., person, technology, concept)" },
                "observations": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Initial observations about the entity",
                    "default": []
                }
            },
            "required": ["name", "entity_type"]
        },
        "description": "Array of entities to create"
    }
}
```

**Returns:** Array of created entity objects with their IDs

---

#### `add_observations`
Add observations to existing entities.

**Input Schema:**
```json
{
    "observations": {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "entity_name": { "type": "string", "description": "Name of the entity" },
                "contents": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Observation texts to add"
                }
            },
            "required": ["entity_name", "contents"]
        }
    }
}
```

**Returns:** Array of created observation objects

---

#### `create_relations`
Create directed relations between entities.

**Input Schema:**
```json
{
    "relations": {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "from": { "type": "string", "description": "Source entity name" },
                "to": { "type": "string", "description": "Target entity name" },
                "relation_type": { "type": "string", "description": "Relation type in active voice (e.g., uses, depends_on, manages)" }
            },
            "required": ["from", "to", "relation_type"]
        }
    }
}
```

**Returns:** Array of created relation objects

---

#### `search_nodes`
Search entities and observations using FTS5 full-text search.

**Input Schema:**
```json
{
    "query": {
        "type": "string",
        "description": "Search query (supports FTS5 syntax: AND, OR, NOT, prefix*)"
    }
}
```

**Behavior:**
1. Searches `entities_fts` for matching entity names/types
2. Searches `observations_fts` for matching observation content
3. For each matched entity, loads all its active observations and relations
4. Deduplicates and merges results

**Returns:** Array of entity objects with their observations and relations

---

#### `open_nodes`
Retrieve specific entities by exact name match.

**Input Schema:**
```json
{
    "names": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Exact entity names to retrieve"
    }
}
```

**Returns:** Array of entity objects with observations and relations (only active/non-deleted)

---

#### `read_graph`
Read the entire knowledge graph of the current project.

**Input Schema:** None

**Returns:** Complete graph: `{ entities: [...], relations: [...] }` with all active entities, their observations, and all active relations.

**Warning:** Can be large. Prefer `search_nodes` for targeted retrieval.

---

#### `delete_entities`
Soft-delete entities and cascade to their observations and relations.

**Input Schema:**
```json
{
    "names": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Entity names to delete"
    }
}
```

**Side Effects:** Sets `deleted_at` on matched entities + their observations and relations (both from/to).

**Returns:** Count of deleted entities

---

#### `delete_observations`
Soft-delete specific observations from entities.

**Input Schema:**
```json
{
    "deletions": {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "entity_name": { "type": "string" },
                "observations": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Observation content strings to match and delete"
                }
            },
            "required": ["entity_name", "observations"]
        }
    }
}
```

**Returns:** Count of deleted observations

---

#### `delete_relations`
Soft-delete specific relations.

**Input Schema:**
```json
{
    "relations": {
        "type": "array",
        "items": {
            "type": "object",
            "properties": {
                "from": { "type": "string" },
                "to": { "type": "string" },
                "relation_type": { "type": "string" }
            },
            "required": ["from", "to", "relation_type"]
        }
    }
}
```

**Returns:** Count of deleted relations

## 5. Session Management

### 5.1 State

The Memory MCP maintains per-session state:

```go
type Session struct {
    CurrentProjectID   string       // active project UUID (empty if none)
    CurrentProjectName string       // active project name
    ProjectDB          *sql.DB      // open connection to project SQLite DB
}
```

### 5.2 Lifecycle

1. **Server starts** → Opens `_meta.db`, no project active
2. **`switch_project(name)`** → Looks up project in `_meta.db`, opens its DB, sets session state
3. **CRUD operations** → Use `session.ProjectDB`, fail if nil
4. **`switch_project(other)`** → Closes current DB, opens new one
5. **`archive_project(current)`** → Closes DB, moves file, clears session
6. **Server stops** → Closes all connections gracefully

### 5.3 Error Handling

All knowledge graph tools check for active project before executing:

```go
if session.ProjectDB == nil {
    return mcp.NewToolError("No active project. Use switch_project to select one.")
}
```

## 6. Go Project Structure

```
memory-mcp/
├── main.go                    # Entry point, MCP server setup
├── go.mod
├── go.sum
├── Dockerfile
├── internal/
│   ├── server/
│   │   └── server.go          # MCP server configuration, tool registration
│   ├── session/
│   │   └── session.go         # Session state management
│   ├── storage/
│   │   ├── meta.go            # _meta.db operations (project CRUD)
│   │   ├── project.go         # Project DB operations (entities, observations, relations)
│   │   ├── schema.go          # SQL schema definitions and migrations
│   │   └── search.go          # FTS5 search logic
│   ├── tools/
│   │   ├── projects.go        # Project management tool handlers
│   │   └── knowledge.go       # Knowledge graph tool handlers
│   └── models/
│       └── models.go          # Shared data structures (Entity, Observation, Relation, Project)
└── data/                      # Runtime data directory (volume-mounted)
    ├── _meta.db
    ├── projects/
    │   └── {uuid}.db
    └── archive/
        └── {uuid}.db
```

## 7. Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o memory-mcp .

FROM alpine:3.20
RUN adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /build/memory-mcp .
RUN mkdir -p data/projects data/archive && chown -R appuser:appuser data
USER appuser
ENTRYPOINT ["./memory-mcp"]
```

## 8. Transport Configuration

The Memory MCP binary supports two modes:

- **stdio** (default): For direct use with mcp-proxy or Claude Desktop
  ```
  ./memory-mcp --transport stdio --data-dir ./data
  ```

- **Streamable HTTP**: For direct HTTP access (testing/development)
  ```
  ./memory-mcp --transport http --port 8081 --data-dir ./data
  ```

In production, stdio mode is used behind mcp-proxy which handles HTTP exposure.
