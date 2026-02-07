package models

// Project represents a project entry in the meta database.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DBPath      string `json:"db_path"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Entity represents a node in the knowledge graph.
type Entity struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	EntityType   string        `json:"entity_type"`
	Observations []Observation `json:"observations,omitempty"`
	Relations    []Relation    `json:"relations,omitempty"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

// Observation represents a fact attached to an entity.
type Observation struct {
	ID        string `json:"id"`
	EntityID  string `json:"entity_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// Relation represents a directed edge between two entities.
type Relation struct {
	ID           string `json:"id"`
	FromEntity   string `json:"from_entity"`
	ToEntity     string `json:"to_entity"`
	RelationType string `json:"relation_type"`
	CreatedAt    string `json:"created_at"`
}

// KnowledgeGraph represents the full graph for a project.
type KnowledgeGraph struct {
	Entities  []Entity  `json:"entities"`
	Relations []Relation `json:"relations"`
}
