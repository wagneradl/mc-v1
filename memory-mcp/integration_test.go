package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/server"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
)

// setupIntegration creates a real MCP server with in-memory transport and returns a connected client session.
func setupIntegration(t *testing.T) (*mcp.ClientSession, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "memory-mcp-integration-*")
	if err != nil {
		t.Fatal(err)
	}

	meta, err := storage.OpenMeta(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}

	srv := server.New(meta)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	_, err = srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		meta.Close()
		os.RemoveAll(dir)
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		meta.Close()
		os.RemoveAll(dir)
		t.Fatalf("client connect: %v", err)
	}

	cleanup := func() {
		session.Close()
		meta.Close()
		os.RemoveAll(dir)
	}
	return session, cleanup
}

// callTool is a helper that calls a tool and returns the text content.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if len(result.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", name)
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): expected TextContent, got %T", name, result.Content[0])
	}
	if result.IsError {
		t.Fatalf("CallTool(%s) returned error: %s", name, tc.Text)
	}
	return tc.Text
}

// callToolExpectError calls a tool and expects an error response (IsError=true).
func callToolExpectError(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): protocol error: %v", name, err)
	}
	if !result.IsError {
		tc := result.Content[0].(*mcp.TextContent)
		t.Fatalf("CallTool(%s): expected error but got success: %s", name, tc.Text)
	}
	tc := result.Content[0].(*mcp.TextContent)
	return tc.Text
}

func TestIntegration_ListTools(t *testing.T) {
	session, cleanup := setupIntegration(t)
	defer cleanup()

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedTools := []string{
		"list_projects", "create_project", "switch_project", "get_current_project",
		"archive_project", "delete_project", "restore_project",
		"create_entities", "add_observations", "create_relations",
		"search_nodes", "open_nodes", "read_graph",
		"delete_entities", "delete_observations", "delete_relations",
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("Missing tool: %s", name)
		}
	}

	if len(result.Tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	session, cleanup := setupIntegration(t)
	defer cleanup()

	// Step 1: create_project("test-project")
	text := callTool(t, session, "create_project", map[string]any{
		"name":        "test-project",
		"description": "Integration test project",
	})
	var proj models.Project
	if err := json.Unmarshal([]byte(text), &proj); err != nil {
		t.Fatalf("parse create_project: %v", err)
	}
	if proj.Name != "test-project" {
		t.Errorf("project name = %q, want %q", proj.Name, "test-project")
	}
	if proj.Status != "active" {
		t.Errorf("project status = %q, want %q", proj.Status, "active")
	}

	// Step 2: get_current_project — should be auto-switched
	text = callTool(t, session, "get_current_project", nil)
	if err := json.Unmarshal([]byte(text), &proj); err != nil {
		t.Fatalf("parse get_current_project: %v", err)
	}
	if proj.Name != "test-project" {
		t.Errorf("current project = %q, want %q", proj.Name, "test-project")
	}

	// Step 3: create_entities
	text = callTool(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{
				"name":         "Go",
				"entity_type":  "technology",
				"observations": []any{"Fast compiled language"},
			},
			map[string]any{
				"name":        "Memory Cloud",
				"entity_type": "project",
			},
		},
	})
	var entities []models.Entity
	if err := json.Unmarshal([]byte(text), &entities); err != nil {
		t.Fatalf("parse create_entities: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if entities[0].Name != "Go" {
		t.Errorf("entity[0].Name = %q, want %q", entities[0].Name, "Go")
	}
	if len(entities[0].Observations) != 1 {
		t.Errorf("expected 1 observation on Go, got %d", len(entities[0].Observations))
	}

	// Step 4: add_observations
	text = callTool(t, session, "add_observations", map[string]any{
		"observations": []any{
			map[string]any{
				"entity_name": "Go",
				"contents":    []any{"Great for CLI tools"},
			},
		},
	})
	if !strings.Contains(text, "Great for CLI tools") {
		t.Error("add_observations should return the new observation")
	}

	// Step 5: create_relations
	text = callTool(t, session, "create_relations", map[string]any{
		"relations": []any{
			map[string]any{
				"from":          "Go",
				"to":            "Memory Cloud",
				"relation_type": "powers",
			},
		},
	})
	var rels []models.Relation
	if err := json.Unmarshal([]byte(text), &rels); err != nil {
		t.Fatalf("parse create_relations: %v", err)
	}
	if len(rels) != 1 || rels[0].RelationType != "powers" {
		t.Error("expected 1 relation with type 'powers'")
	}

	// Step 6: search_nodes("Go")
	text = callTool(t, session, "search_nodes", map[string]any{
		"query": "Go",
	})
	var searchResults []models.Entity
	if err := json.Unmarshal([]byte(text), &searchResults); err != nil {
		t.Fatalf("parse search_nodes: %v", err)
	}
	if len(searchResults) == 0 {
		t.Fatal("search_nodes('Go') returned no results")
	}
	// Should find Go entity
	found := false
	for _, e := range searchResults {
		if e.Name == "Go" {
			found = true
			if len(e.Observations) != 2 {
				t.Errorf("Go should have 2 observations, got %d", len(e.Observations))
			}
			if len(e.Relations) != 1 {
				t.Errorf("Go should have 1 relation, got %d", len(e.Relations))
			}
		}
	}
	if !found {
		t.Error("search did not return Go entity")
	}

	// Step 7: read_graph
	text = callTool(t, session, "read_graph", nil)
	var graph models.KnowledgeGraph
	if err := json.Unmarshal([]byte(text), &graph); err != nil {
		t.Fatalf("parse read_graph: %v", err)
	}
	if len(graph.Entities) != 2 {
		t.Errorf("graph should have 2 entities, got %d", len(graph.Entities))
	}
	if len(graph.Relations) != 1 {
		t.Errorf("graph should have 1 relation, got %d", len(graph.Relations))
	}

	// Step 8: open_nodes
	text = callTool(t, session, "open_nodes", map[string]any{
		"names": []any{"Go", "Memory Cloud"},
	})
	var openedNodes []models.Entity
	if err := json.Unmarshal([]byte(text), &openedNodes); err != nil {
		t.Fatalf("parse open_nodes: %v", err)
	}
	if len(openedNodes) != 2 {
		t.Errorf("open_nodes should return 2 entities, got %d", len(openedNodes))
	}

	// Step 9: delete_observations
	text = callTool(t, session, "delete_observations", map[string]any{
		"deletions": []any{
			map[string]any{
				"entity_name":  "Go",
				"observations": []any{"Fast compiled language"},
			},
		},
	})
	if !strings.Contains(text, "Deleted 1") {
		t.Errorf("expected 'Deleted 1', got %q", text)
	}

	// Verify Go now has 1 observation
	text = callTool(t, session, "open_nodes", map[string]any{
		"names": []any{"Go"},
	})
	json.Unmarshal([]byte(text), &openedNodes)
	if len(openedNodes) == 1 && len(openedNodes[0].Observations) != 1 {
		t.Errorf("Go should have 1 observation after delete, got %d", len(openedNodes[0].Observations))
	}

	// Step 10: delete_entities
	text = callTool(t, session, "delete_entities", map[string]any{
		"names": []any{"Go"},
	})
	if !strings.Contains(text, "Deleted 1") {
		t.Errorf("expected 'Deleted 1', got %q", text)
	}

	// Verify Go is gone, relation is gone, Memory Cloud still exists
	text = callTool(t, session, "read_graph", nil)
	json.Unmarshal([]byte(text), &graph)
	if len(graph.Entities) != 1 {
		t.Errorf("graph should have 1 entity after deleting Go, got %d", len(graph.Entities))
	}
	if graph.Entities[0].Name != "Memory Cloud" {
		t.Errorf("remaining entity should be Memory Cloud, got %q", graph.Entities[0].Name)
	}
	if len(graph.Relations) != 0 {
		t.Errorf("graph should have 0 relations after deleting Go, got %d", len(graph.Relations))
	}

	// Step 11: delete_relations (create a new one first to test)
	callTool(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{"name": "SQLite", "entity_type": "technology"},
		},
	})
	callTool(t, session, "create_relations", map[string]any{
		"relations": []any{
			map[string]any{"from": "Memory Cloud", "to": "SQLite", "relation_type": "uses"},
		},
	})
	text = callTool(t, session, "delete_relations", map[string]any{
		"relations": []any{
			map[string]any{"from": "Memory Cloud", "to": "SQLite", "relation_type": "uses"},
		},
	})
	if !strings.Contains(text, "Deleted 1") {
		t.Errorf("expected 'Deleted 1', got %q", text)
	}

	// Step 12: archive_project
	text = callTool(t, session, "archive_project", map[string]any{
		"name": "test-project",
	})
	var archivedProj models.Project
	if err := json.Unmarshal([]byte(text), &archivedProj); err != nil {
		t.Fatalf("parse archive_project: %v", err)
	}
	if archivedProj.Status != "archived" {
		t.Errorf("project status = %q, want %q", archivedProj.Status, "archived")
	}

	// Verify current project is cleared
	text = callTool(t, session, "get_current_project", nil)
	if !strings.Contains(text, "No project") {
		t.Error("get_current_project should say no project after archive")
	}

	// list_projects should show 0 active
	text = callTool(t, session, "list_projects", map[string]any{"status": "active"})
	var projects []models.Project
	json.Unmarshal([]byte(text), &projects)
	if len(projects) != 0 {
		t.Errorf("expected 0 active projects, got %d", len(projects))
	}

	// Step 13: restore_project
	text = callTool(t, session, "restore_project", map[string]any{
		"name": "test-project",
	})
	var restoredProj models.Project
	if err := json.Unmarshal([]byte(text), &restoredProj); err != nil {
		t.Fatalf("parse restore_project: %v", err)
	}
	if restoredProj.Status != "active" {
		t.Errorf("project status = %q, want %q", restoredProj.Status, "active")
	}

	// Switch back to it and verify data is intact
	callTool(t, session, "switch_project", map[string]any{"name": "test-project"})
	text = callTool(t, session, "read_graph", nil)
	json.Unmarshal([]byte(text), &graph)
	if len(graph.Entities) != 2 {
		t.Errorf("graph should still have 2 entities after restore, got %d", len(graph.Entities))
	}

	// Step 14: delete_project
	text = callTool(t, session, "delete_project", map[string]any{
		"name": "test-project",
	})
	if !strings.Contains(text, "permanently deleted") {
		t.Errorf("expected confirmation, got %q", text)
	}

	// Verify gone
	text = callTool(t, session, "list_projects", map[string]any{"status": "all"})
	json.Unmarshal([]byte(text), &projects)
	if len(projects) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(projects))
	}
}

func TestIntegration_ErrorCases(t *testing.T) {
	session, cleanup := setupIntegration(t)
	defer cleanup()

	// Error: knowledge graph tool without active project
	errText := callToolExpectError(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{"name": "test", "entity_type": "thing"},
		},
	})
	if !strings.Contains(errText, "No active project") {
		t.Errorf("expected 'No active project', got %q", errText)
	}

	errText = callToolExpectError(t, session, "search_nodes", map[string]any{
		"query": "test",
	})
	if !strings.Contains(errText, "No active project") {
		t.Errorf("expected 'No active project', got %q", errText)
	}

	errText = callToolExpectError(t, session, "read_graph", nil)
	if !strings.Contains(errText, "No active project") {
		t.Errorf("expected 'No active project', got %q", errText)
	}

	// Create a project for remaining tests
	callTool(t, session, "create_project", map[string]any{
		"name": "error-test",
	})

	// Error: duplicate project name
	errText = callToolExpectError(t, session, "create_project", map[string]any{
		"name": "error-test",
	})
	if !strings.Contains(errText, "Failed to create project") {
		t.Errorf("expected 'Failed to create project' for duplicate, got %q", errText)
	}

	// Error: add observations to nonexistent entity
	errText = callToolExpectError(t, session, "add_observations", map[string]any{
		"observations": []any{
			map[string]any{
				"entity_name": "DoesNotExist",
				"contents":    []any{"test"},
			},
		},
	})
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found', got %q", errText)
	}

	// Error: create relation with nonexistent entity
	callTool(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{"name": "A", "entity_type": "thing"},
		},
	})
	errText = callToolExpectError(t, session, "create_relations", map[string]any{
		"relations": []any{
			map[string]any{"from": "A", "to": "NonExistent", "relation_type": "links"},
		},
	})
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found', got %q", errText)
	}

	// Error: switch to nonexistent project
	errText = callToolExpectError(t, session, "switch_project", map[string]any{
		"name": "nonexistent-project",
	})
	if !strings.Contains(errText, "not found") {
		t.Errorf("expected 'not found' for switch, got %q", errText)
	}

	// Error: archive already archived project
	callTool(t, session, "archive_project", map[string]any{"name": "error-test"})
	errText = callToolExpectError(t, session, "archive_project", map[string]any{
		"name": "error-test",
	})
	if !strings.Contains(errText, "already archived") {
		t.Errorf("expected 'already archived', got %q", errText)
	}

	// Error: switch to archived project
	errText = callToolExpectError(t, session, "switch_project", map[string]any{
		"name": "error-test",
	})
	if !strings.Contains(errText, "archived") {
		t.Errorf("expected mention of 'archived' for switch, got %q", errText)
	}

	// Error: restore a non-archived project
	callTool(t, session, "restore_project", map[string]any{"name": "error-test"})
	errText = callToolExpectError(t, session, "restore_project", map[string]any{
		"name": "error-test",
	})
	if !strings.Contains(errText, "not archived") {
		t.Errorf("expected 'not archived', got %q", errText)
	}

	// Cleanup
	callTool(t, session, "delete_project", map[string]any{"name": "error-test"})
}

func TestIntegration_MultiProjectIsolation(t *testing.T) {
	session, cleanup := setupIntegration(t)
	defer cleanup()

	// Create two projects
	callTool(t, session, "create_project", map[string]any{"name": "project-a"})
	callTool(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{"name": "EntityInA", "entity_type": "thing"},
		},
	})

	callTool(t, session, "create_project", map[string]any{"name": "project-b"})
	callTool(t, session, "create_entities", map[string]any{
		"entities": []any{
			map[string]any{"name": "EntityInB", "entity_type": "thing"},
		},
	})

	// Project B should only see EntityInB
	text := callTool(t, session, "read_graph", nil)
	var graph models.KnowledgeGraph
	json.Unmarshal([]byte(text), &graph)
	if len(graph.Entities) != 1 || graph.Entities[0].Name != "EntityInB" {
		t.Errorf("Project B should only have EntityInB, got %+v", graph.Entities)
	}

	// Switch to A — should only see EntityInA
	callTool(t, session, "switch_project", map[string]any{"name": "project-a"})
	text = callTool(t, session, "read_graph", nil)
	json.Unmarshal([]byte(text), &graph)
	if len(graph.Entities) != 1 || graph.Entities[0].Name != "EntityInA" {
		t.Errorf("Project A should only have EntityInA, got %+v", graph.Entities)
	}

	// Cleanup
	callTool(t, session, "delete_project", map[string]any{"name": "project-a"})
	callTool(t, session, "delete_project", map[string]any{"name": "project-b"})
}
