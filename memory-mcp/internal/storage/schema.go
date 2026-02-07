package storage

// MetaSchema is the SQL schema for the central _meta.db database.
const MetaSchema = `
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    db_path     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK(status IN ('active', 'archived')),
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
`

// ProjectSchema is the SQL schema for each per-project database.
const ProjectSchema = `
CREATE TABLE IF NOT EXISTS entities (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at  TEXT NULL
);

CREATE TABLE IF NOT EXISTS observations (
    id          TEXT PRIMARY KEY,
    entity_id   TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at  TEXT NULL
);

CREATE TABLE IF NOT EXISTS relations (
    id              TEXT PRIMARY KEY,
    from_entity     TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    to_entity       TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relation_type   TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at      TEXT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
    name,
    entity_type,
    content='entities',
    content_rowid='rowid'
);

CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
    content,
    content='observations',
    content_rowid='rowid'
);

-- Partial indexes for efficient queries on active (non-deleted) records
CREATE INDEX IF NOT EXISTS idx_entities_active ON entities(name, entity_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_observations_entity ON observations(entity_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_relations_from ON relations(from_entity) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_relations_to ON relations(to_entity) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_relations_type ON relations(relation_type) WHERE deleted_at IS NULL;
`

// ProjectTriggers must be executed separately since CREATE TRIGGER doesn't
// support IF NOT EXISTS. We check for their existence before creating.
const ProjectTriggers = `
CREATE TRIGGER IF NOT EXISTS entities_ai AFTER INSERT ON entities BEGIN
    INSERT INTO entities_fts(rowid, name, entity_type) VALUES (new.rowid, new.name, new.entity_type);
END;
CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN
    INSERT INTO entities_fts(entities_fts, rowid, name, entity_type) VALUES('delete', old.rowid, old.name, old.entity_type);
END;
CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN
    INSERT INTO entities_fts(entities_fts, rowid, name, entity_type) VALUES('delete', old.rowid, old.name, old.entity_type);
    INSERT INTO entities_fts(rowid, name, entity_type) VALUES (new.rowid, new.name, new.entity_type);
END;

CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
    INSERT INTO observations_fts(rowid, content) VALUES (new.rowid, new.content);
END;
CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;
CREATE TRIGGER IF NOT EXISTS observations_au AFTER UPDATE ON observations BEGIN
    INSERT INTO observations_fts(observations_fts, rowid, content) VALUES('delete', old.rowid, old.content);
    INSERT INTO observations_fts(rowid, content) VALUES (new.rowid, new.content);
END;
`

// Pragmas configures SQLite for optimal performance.
const Pragmas = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA cache_size = -64000;
`
