package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/session"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/tools"
)

// New creates a fully configured MCP server with all tools registered.
func New(meta *storage.MetaStore) *mcp.Server {
	sess := session.New()

	pt := &tools.ProjectTools{Meta: meta, Session: sess}
	kt := &tools.KnowledgeTools{Meta: meta, Session: sess}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "memory-mcp",
		Version: "0.1.0",
	}, nil)

	// Project management tools
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "List all projects with optional status filter (active, archived, all)",
	}, pt.ListProjects)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_project",
		Description: "Create a new project with its own isolated database",
	}, pt.CreateProject)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "switch_project",
		Description: "Switch the active project context for the current session",
	}, pt.SwitchProject)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_current_project",
		Description: "Get information about the currently active project",
	}, pt.GetCurrentProject)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "archive_project",
		Description: "Archive a project (preserves data, makes it inactive)",
	}, pt.ArchiveProject)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_project",
		Description: "Permanently delete a project and all its data (irreversible)",
	}, pt.DeleteProject)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restore_project",
		Description: "Restore an archived project back to active status",
	}, pt.RestoreProject)

	// Knowledge graph tools
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_entities",
		Description: "Create one or more entities in the knowledge graph (requires active project)",
	}, kt.CreateEntities)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_observations",
		Description: "Add observations to existing entities (requires active project)",
	}, kt.AddObservations)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_relations",
		Description: "Create directed relations between entities (requires active project)",
	}, kt.CreateRelations)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_nodes",
		Description: "Search entities and observations using FTS5 full-text search (requires active project)",
	}, kt.SearchNodes)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "open_nodes",
		Description: "Retrieve specific entities by exact name match (requires active project)",
	}, kt.OpenNodes)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_graph",
		Description: "Read the entire knowledge graph of the current project (requires active project)",
	}, kt.ReadGraph)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_entities",
		Description: "Soft-delete entities and cascade to their observations and relations (requires active project)",
	}, kt.DeleteEntities)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_observations",
		Description: "Soft-delete specific observations from entities (requires active project)",
	}, kt.DeleteObservations)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_relations",
		Description: "Soft-delete specific relations (requires active project)",
	}, kt.DeleteRelations)

	return srv
}
