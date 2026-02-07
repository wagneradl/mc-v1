package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/session"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
)

// KnowledgeTools holds references needed by knowledge graph tool handlers.
type KnowledgeTools struct {
	Meta    *storage.MetaStore
	Session *session.Session
}

// --- Input types ---

type CreateEntitiesInput struct {
	Entities []EntityInput `json:"entities" jsonschema:"Array of entities to create"`
}

type EntityInput struct {
	Name         string   `json:"name" jsonschema:"Entity name"`
	EntityType   string   `json:"entity_type" jsonschema:"Entity type (e.g., person, technology, concept)"`
	Observations []string `json:"observations,omitempty" jsonschema:"Initial observations about the entity"`
}

type AddObservationsInput struct {
	Observations []ObservationInput `json:"observations" jsonschema:"Array of observations to add"`
}

type ObservationInput struct {
	EntityName string   `json:"entity_name" jsonschema:"Name of the entity"`
	Contents   []string `json:"contents" jsonschema:"Observation texts to add"`
}

type CreateRelationsInput struct {
	Relations []RelationInput `json:"relations" jsonschema:"Array of relations to create"`
}

type RelationInput struct {
	From         string `json:"from" jsonschema:"Source entity name"`
	To           string `json:"to" jsonschema:"Target entity name"`
	RelationType string `json:"relation_type" jsonschema:"Relation type in active voice (e.g., uses, depends_on, manages)"`
}

type SearchNodesInput struct {
	Query string `json:"query" jsonschema:"Search query (supports FTS5 syntax: AND, OR, NOT, prefix*)"`
}

type OpenNodesInput struct {
	Names []string `json:"names" jsonschema:"Exact entity names to retrieve"`
}

type DeleteEntitiesInput struct {
	Names []string `json:"names" jsonschema:"Entity names to delete"`
}

type DeleteObservationsInput struct {
	Deletions []DeleteObservationItem `json:"deletions" jsonschema:"Array of observations to delete"`
}

type DeleteObservationItem struct {
	EntityName   string   `json:"entity_name" jsonschema:"Name of the entity"`
	Observations []string `json:"observations" jsonschema:"Observation content strings to match and delete"`
}

type DeleteRelationsInput struct {
	Relations []RelationInput `json:"relations" jsonschema:"Array of relations to delete"`
}

// --- Handlers ---

func (t *KnowledgeTools) requireProject() (*storage.ProjectStore, *mcp.CallToolResult) {
	ps := t.Session.ProjectStore()
	if ps == nil {
		return nil, toolError("No active project. Use switch_project to select one.")
	}
	return ps, nil
}

func (t *KnowledgeTools) CreateEntities(_ context.Context, _ *mcp.CallToolRequest, input CreateEntitiesInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	entities := make([]struct {
		Name         string
		EntityType   string
		Observations []string
	}, len(input.Entities))
	for i, e := range input.Entities {
		entities[i].Name = e.Name
		entities[i].EntityType = e.EntityType
		entities[i].Observations = e.Observations
	}

	created, err := ps.CreateEntities(entities)
	if err != nil {
		return toolError("Failed to create entities: %v", err), nil, nil
	}

	return toolJSON(created)
}

func (t *KnowledgeTools) AddObservations(_ context.Context, _ *mcp.CallToolRequest, input AddObservationsInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	var allCreated []any
	for _, obs := range input.Observations {
		created, err := ps.AddObservations(obs.EntityName, obs.Contents)
		if err != nil {
			return toolError("Failed to add observations for %q: %v", obs.EntityName, err), nil, nil
		}
		allCreated = append(allCreated, created)
	}

	return toolJSON(allCreated)
}

func (t *KnowledgeTools) CreateRelations(_ context.Context, _ *mcp.CallToolRequest, input CreateRelationsInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	relations := make([]struct {
		From         string
		To           string
		RelationType string
	}, len(input.Relations))
	for i, r := range input.Relations {
		relations[i].From = r.From
		relations[i].To = r.To
		relations[i].RelationType = r.RelationType
	}

	created, err := ps.CreateRelations(relations)
	if err != nil {
		return toolError("Failed to create relations: %v", err), nil, nil
	}

	return toolJSON(created)
}

func (t *KnowledgeTools) SearchNodes(_ context.Context, _ *mcp.CallToolRequest, input SearchNodesInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	entities, err := ps.Search(input.Query)
	if err != nil {
		return toolError("Search failed: %v", err), nil, nil
	}

	return toolJSON(entities)
}

func (t *KnowledgeTools) OpenNodes(_ context.Context, _ *mcp.CallToolRequest, input OpenNodesInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	entities, err := ps.GetEntities(input.Names)
	if err != nil {
		return toolError("Failed to open nodes: %v", err), nil, nil
	}

	return toolJSON(entities)
}

func (t *KnowledgeTools) ReadGraph(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	graph, err := ps.ReadGraph()
	if err != nil {
		return toolError("Failed to read graph: %v", err), nil, nil
	}

	return toolJSON(graph)
}

func (t *KnowledgeTools) DeleteEntities(_ context.Context, _ *mcp.CallToolRequest, input DeleteEntitiesInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	count, err := ps.DeleteEntities(input.Names)
	if err != nil {
		return toolError("Failed to delete entities: %v", err), nil, nil
	}

	return toolText(fmt.Sprintf("Deleted %d entities.", count)), nil, nil
}

func (t *KnowledgeTools) DeleteObservations(_ context.Context, _ *mcp.CallToolRequest, input DeleteObservationsInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	var total int64
	for _, d := range input.Deletions {
		count, err := ps.DeleteObservations(d.EntityName, d.Observations)
		if err != nil {
			return toolError("Failed to delete observations for %q: %v", d.EntityName, err), nil, nil
		}
		total += count
	}

	return toolText(fmt.Sprintf("Deleted %d observations.", total)), nil, nil
}

func (t *KnowledgeTools) DeleteRelations(_ context.Context, _ *mcp.CallToolRequest, input DeleteRelationsInput) (*mcp.CallToolResult, any, error) {
	ps, errResult := t.requireProject()
	if errResult != nil {
		return errResult, nil, nil
	}

	relations := make([]struct {
		From         string
		To           string
		RelationType string
	}, len(input.Relations))
	for i, r := range input.Relations {
		relations[i].From = r.From
		relations[i].To = r.To
		relations[i].RelationType = r.RelationType
	}

	count, err := ps.DeleteRelations(relations)
	if err != nil {
		return toolError("Failed to delete relations: %v", err), nil, nil
	}

	return toolText(fmt.Sprintf("Deleted %d relations.", count)), nil, nil
}
