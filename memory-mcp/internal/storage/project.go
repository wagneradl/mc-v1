package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
)

// ProjectStore manages a single project's knowledge graph database.
type ProjectStore struct {
	db *sql.DB
}

// OpenProject opens an existing project database and configures it.
func OpenProject(dbPath string) (*ProjectStore, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-64000)")
	if err != nil {
		return nil, fmt.Errorf("open project db: %w", err)
	}
	// Verify the connection works
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping project db: %w", err)
	}
	return &ProjectStore{db: db}, nil
}

// Close closes the project database connection.
func (p *ProjectStore) Close() error {
	return p.db.Close()
}

// CreateEntities inserts entities with their optional initial observations.
// Returns the created entities with their generated IDs.
func (p *ProjectStore) CreateEntities(entities []struct {
	Name         string
	EntityType   string
	Observations []string
}) ([]models.Entity, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var created []models.Entity

	for _, e := range entities {
		entityID := uuid.New().String()
		_, err := tx.Exec(
			`INSERT INTO entities (id, name, entity_type) VALUES (?, ?, ?)`,
			entityID, e.Name, e.EntityType,
		)
		if err != nil {
			return nil, fmt.Errorf("insert entity %q: %w", e.Name, err)
		}

		entity := models.Entity{
			ID:         entityID,
			Name:       e.Name,
			EntityType: e.EntityType,
		}

		for _, obsContent := range e.Observations {
			obsID := uuid.New().String()
			_, err := tx.Exec(
				`INSERT INTO observations (id, entity_id, content) VALUES (?, ?, ?)`,
				obsID, entityID, obsContent,
			)
			if err != nil {
				return nil, fmt.Errorf("insert observation for %q: %w", e.Name, err)
			}
			entity.Observations = append(entity.Observations, models.Observation{
				ID:       obsID,
				EntityID: entityID,
				Content:  obsContent,
			})
		}

		// Re-read to get timestamps
		row := tx.QueryRow(
			`SELECT created_at, updated_at FROM entities WHERE id = ?`, entityID,
		)
		row.Scan(&entity.CreatedAt, &entity.UpdatedAt)

		created = append(created, entity)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return created, nil
}

// AddObservations adds observations to existing entities identified by name.
func (p *ProjectStore) AddObservations(entityName string, contents []string) ([]models.Observation, error) {
	// Find the entity
	var entityID string
	err := p.db.QueryRow(
		`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, entityName,
	).Scan(&entityID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entity %q not found", entityName)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup entity %q: %w", entityName, err)
	}

	tx, err := p.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var created []models.Observation
	for _, content := range contents {
		obsID := uuid.New().String()
		_, err := tx.Exec(
			`INSERT INTO observations (id, entity_id, content) VALUES (?, ?, ?)`,
			obsID, entityID, content,
		)
		if err != nil {
			return nil, fmt.Errorf("insert observation: %w", err)
		}
		created = append(created, models.Observation{
			ID:       obsID,
			EntityID: entityID,
			Content:  content,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Re-read to get timestamps
	for i, obs := range created {
		p.db.QueryRow(`SELECT created_at FROM observations WHERE id = ?`, obs.ID).Scan(&created[i].CreatedAt)
	}

	return created, nil
}

// CreateRelations inserts directed relations between entities.
func (p *ProjectStore) CreateRelations(relations []struct {
	From         string
	To           string
	RelationType string
}) ([]models.Relation, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var created []models.Relation

	for _, r := range relations {
		// Resolve entity names to IDs
		var fromID, toID string
		err := tx.QueryRow(`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, r.From).Scan(&fromID)
		if err != nil {
			return nil, fmt.Errorf("from entity %q not found: %w", r.From, err)
		}
		err = tx.QueryRow(`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, r.To).Scan(&toID)
		if err != nil {
			return nil, fmt.Errorf("to entity %q not found: %w", r.To, err)
		}

		relID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO relations (id, from_entity, to_entity, relation_type) VALUES (?, ?, ?, ?)`,
			relID, fromID, toID, r.RelationType,
		)
		if err != nil {
			return nil, fmt.Errorf("insert relation: %w", err)
		}
		created = append(created, models.Relation{
			ID:           relID,
			FromEntity:   fromID,
			ToEntity:     toID,
			RelationType: r.RelationType,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	for i, rel := range created {
		p.db.QueryRow(`SELECT created_at FROM relations WHERE id = ?`, rel.ID).Scan(&created[i].CreatedAt)
	}

	return created, nil
}

// DeleteEntities soft-deletes entities and cascades to their observations and relations.
func (p *ProjectStore) DeleteEntities(names []string) (int64, error) {
	if len(names) == 0 {
		return 0, nil
	}

	tx, err := p.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, name := range names {
		placeholders[i] = "?"
		args[i] = name
	}
	inClause := strings.Join(placeholders, ",")

	// Get entity IDs for cascading
	rows, err := tx.Query(
		fmt.Sprintf(`SELECT id FROM entities WHERE name IN (%s) AND deleted_at IS NULL`, inClause),
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("query entities: %w", err)
	}
	var entityIDs []any
	var idPlaceholders []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		entityIDs = append(entityIDs, id)
		idPlaceholders = append(idPlaceholders, "?")
	}
	rows.Close()

	if len(entityIDs) == 0 {
		return 0, nil
	}

	idInClause := strings.Join(idPlaceholders, ",")

	// Soft-delete observations
	_, err = tx.Exec(
		fmt.Sprintf(`UPDATE observations SET deleted_at = datetime('now') WHERE entity_id IN (%s) AND deleted_at IS NULL`, idInClause),
		entityIDs...,
	)
	if err != nil {
		return 0, fmt.Errorf("soft-delete observations: %w", err)
	}

	// Soft-delete relations (both from and to)
	for _, eid := range entityIDs {
		_, err = tx.Exec(
			`UPDATE relations SET deleted_at = datetime('now') WHERE (from_entity = ? OR to_entity = ?) AND deleted_at IS NULL`,
			eid, eid,
		)
		if err != nil {
			return 0, fmt.Errorf("soft-delete relations: %w", err)
		}
	}

	// Soft-delete the entities
	result, err := tx.Exec(
		fmt.Sprintf(`UPDATE entities SET deleted_at = datetime('now'), updated_at = datetime('now') WHERE name IN (%s) AND deleted_at IS NULL`, inClause),
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("soft-delete entities: %w", err)
	}

	count, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return count, nil
}

// DeleteObservations soft-deletes observations matching entity name and content strings.
func (p *ProjectStore) DeleteObservations(entityName string, contents []string) (int64, error) {
	var entityID string
	err := p.db.QueryRow(
		`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, entityName,
	).Scan(&entityID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("entity %q not found", entityName)
	}
	if err != nil {
		return 0, fmt.Errorf("lookup entity: %w", err)
	}

	tx, err := p.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var total int64
	for _, content := range contents {
		result, err := tx.Exec(
			`UPDATE observations SET deleted_at = datetime('now') WHERE entity_id = ? AND content = ? AND deleted_at IS NULL`,
			entityID, content,
		)
		if err != nil {
			return 0, fmt.Errorf("soft-delete observation: %w", err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return total, nil
}

// DeleteRelations soft-deletes relations matching from/to entity names and type.
func (p *ProjectStore) DeleteRelations(relations []struct {
	From         string
	To           string
	RelationType string
}) (int64, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var total int64
	for _, r := range relations {
		// Resolve names to IDs
		var fromID, toID string
		err := tx.QueryRow(`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, r.From).Scan(&fromID)
		if err != nil {
			continue // skip if entity not found
		}
		err = tx.QueryRow(`SELECT id FROM entities WHERE name = ? AND deleted_at IS NULL`, r.To).Scan(&toID)
		if err != nil {
			continue
		}

		result, err := tx.Exec(
			`UPDATE relations SET deleted_at = datetime('now') WHERE from_entity = ? AND to_entity = ? AND relation_type = ? AND deleted_at IS NULL`,
			fromID, toID, r.RelationType,
		)
		if err != nil {
			return 0, fmt.Errorf("soft-delete relation: %w", err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return total, nil
}

// GetEntities retrieves entities by exact name match with their observations and relations.
func (p *ProjectStore) GetEntities(names []string) ([]models.Entity, error) {
	if len(names) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, name := range names {
		placeholders[i] = "?"
		args[i] = name
	}
	inClause := strings.Join(placeholders, ",")

	rows, err := p.db.Query(
		fmt.Sprintf(`SELECT id, name, entity_type, created_at, updated_at FROM entities WHERE name IN (%s) AND deleted_at IS NULL`, inClause),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}
	defer rows.Close()

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.EntityType, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load observations and relations for each entity
	for i := range entities {
		obs, err := p.getObservations(entities[i].ID)
		if err != nil {
			return nil, err
		}
		entities[i].Observations = obs

		rels, err := p.getRelations(entities[i].ID)
		if err != nil {
			return nil, err
		}
		entities[i].Relations = rels
	}

	return entities, nil
}

// ReadGraph returns the complete active knowledge graph.
func (p *ProjectStore) ReadGraph() (*models.KnowledgeGraph, error) {
	// Load all active entities
	rows, err := p.db.Query(
		`SELECT id, name, entity_type, created_at, updated_at FROM entities WHERE deleted_at IS NULL ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}
	defer rows.Close()

	var entities []models.Entity
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.EntityType, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load observations for each entity
	for i := range entities {
		obs, err := p.getObservations(entities[i].ID)
		if err != nil {
			return nil, err
		}
		entities[i].Observations = obs
	}

	// Load all active relations
	relRows, err := p.db.Query(
		`SELECT r.id, r.from_entity, r.to_entity, r.relation_type, r.created_at
		 FROM relations r
		 WHERE r.deleted_at IS NULL
		 ORDER BY r.created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer relRows.Close()

	var relations []models.Relation
	for relRows.Next() {
		var r models.Relation
		if err := relRows.Scan(&r.ID, &r.FromEntity, &r.ToEntity, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, r)
	}
	if err := relRows.Err(); err != nil {
		return nil, err
	}

	return &models.KnowledgeGraph{
		Entities:  entities,
		Relations: relations,
	}, nil
}

// getObservations loads all active observations for an entity.
func (p *ProjectStore) getObservations(entityID string) ([]models.Observation, error) {
	rows, err := p.db.Query(
		`SELECT id, entity_id, content, created_at FROM observations WHERE entity_id = ? AND deleted_at IS NULL ORDER BY created_at`,
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query observations: %w", err)
	}
	defer rows.Close()

	var obs []models.Observation
	for rows.Next() {
		var o models.Observation
		if err := rows.Scan(&o.ID, &o.EntityID, &o.Content, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan observation: %w", err)
		}
		obs = append(obs, o)
	}
	return obs, rows.Err()
}

// getRelations loads all active relations where the entity is either source or target.
func (p *ProjectStore) getRelations(entityID string) ([]models.Relation, error) {
	rows, err := p.db.Query(
		`SELECT id, from_entity, to_entity, relation_type, created_at
		 FROM relations
		 WHERE (from_entity = ? OR to_entity = ?) AND deleted_at IS NULL
		 ORDER BY created_at`,
		entityID, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	var rels []models.Relation
	for rows.Next() {
		var r models.Relation
		if err := rows.Scan(&r.ID, &r.FromEntity, &r.ToEntity, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		rels = append(rels, r)
	}
	return rels, rows.Err()
}
