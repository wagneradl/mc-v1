package storage

import (
	"fmt"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
)

// Search performs FTS5 full-text search across entities and observations.
// It returns fully-loaded entities (with observations and relations) that match.
func (p *ProjectStore) Search(query string) ([]models.Entity, error) {
	entityIDs := make(map[string]bool)

	// Search entities_fts for matching entity names/types
	rows, err := p.db.Query(
		`SELECT e.id FROM entities e
		 JOIN entities_fts ON entities_fts.rowid = e.rowid
		 WHERE entities_fts MATCH ? AND e.deleted_at IS NULL`,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("search entities fts: %w", err)
	}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		entityIDs[id] = true
	}
	rows.Close()

	// Search observations_fts for matching observation content
	obsRows, err := p.db.Query(
		`SELECT DISTINCT o.entity_id FROM observations o
		 JOIN observations_fts ON observations_fts.rowid = o.rowid
		 WHERE observations_fts MATCH ? AND o.deleted_at IS NULL`,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("search observations fts: %w", err)
	}
	for obsRows.Next() {
		var entityID string
		obsRows.Scan(&entityID)
		entityIDs[entityID] = true
	}
	obsRows.Close()

	if len(entityIDs) == 0 {
		return nil, nil
	}

	// Load full entity data for all matched IDs
	var entities []models.Entity
	for id := range entityIDs {
		var e models.Entity
		err := p.db.QueryRow(
			`SELECT id, name, entity_type, created_at, updated_at FROM entities WHERE id = ? AND deleted_at IS NULL`,
			id,
		).Scan(&e.ID, &e.Name, &e.EntityType, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			continue // entity may have been deleted between search and load
		}

		obs, err := p.getObservations(e.ID)
		if err != nil {
			return nil, err
		}
		e.Observations = obs

		rels, err := p.getRelations(e.ID)
		if err != nil {
			return nil, err
		}
		e.Relations = rels

		entities = append(entities, e)
	}

	return entities, nil
}
