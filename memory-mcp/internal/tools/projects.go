package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/session"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
)

// ProjectTools holds references needed by project management tool handlers.
type ProjectTools struct {
	Meta    *storage.MetaStore
	Session *session.Session
}

// --- Input types ---

type ListProjectsInput struct {
	Status string `json:"status" jsonschema:"Filter projects by status: active, archived, or all"`
}

type CreateProjectInput struct {
	Name        string `json:"name" jsonschema:"Unique project name (slug-friendly)"`
	Description string `json:"description,omitempty" jsonschema:"Optional project description"`
}

type SwitchProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the project to switch to"`
}

type ArchiveProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the project to archive"`
}

type DeleteProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the project to permanently delete"`
}

type RestoreProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the archived project to restore"`
}

// --- Handlers ---

func (t *ProjectTools) ListProjects(_ context.Context, _ *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, any, error) {
	status := input.Status
	if status == "" {
		status = "active"
	}

	projects, err := t.Meta.ListProjects(status)
	if err != nil {
		return toolError("Failed to list projects: %v", err), nil, nil
	}
	if projects == nil {
		projects = []models.Project{}
	}

	return toolJSON(projects)
}

func (t *ProjectTools) CreateProject(_ context.Context, _ *mcp.CallToolRequest, input CreateProjectInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return toolError("Project name is required"), nil, nil
	}

	proj, err := t.Meta.CreateProject(input.Name, input.Description)
	if err != nil {
		return toolError("Failed to create project: %v", err), nil, nil
	}

	// Auto-switch to the new project
	_, err = t.Session.SwitchProject(t.Meta, proj.Name)
	if err != nil {
		return toolError("Project created but failed to switch: %v", err), nil, nil
	}

	return toolJSON(proj)
}

func (t *ProjectTools) SwitchProject(_ context.Context, _ *mcp.CallToolRequest, input SwitchProjectInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return toolError("Project name is required"), nil, nil
	}

	proj, err := t.Session.SwitchProject(t.Meta, input.Name)
	if err != nil {
		return toolError("Failed to switch project: %v", err), nil, nil
	}

	return toolJSON(proj)
}

func (t *ProjectTools) GetCurrentProject(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	id, name, ok := t.Session.GetCurrent()
	if !ok {
		return toolText("No project is currently active. Use switch_project to select one."), nil, nil
	}

	proj, err := t.Meta.GetProjectByID(id)
	if err != nil {
		return toolText(fmt.Sprintf("Active project: %s (details unavailable)", name)), nil, nil
	}

	return toolJSON(proj)
}

func (t *ProjectTools) ArchiveProject(_ context.Context, _ *mcp.CallToolRequest, input ArchiveProjectInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return toolError("Project name is required"), nil, nil
	}

	// If archiving the current project, clear the session
	_, currentName, ok := t.Session.GetCurrent()
	if ok && currentName == input.Name {
		t.Session.Clear()
	}

	proj, err := t.Meta.ArchiveProject(input.Name)
	if err != nil {
		return toolError("Failed to archive project: %v", err), nil, nil
	}

	return toolJSON(proj)
}

func (t *ProjectTools) DeleteProject(_ context.Context, _ *mcp.CallToolRequest, input DeleteProjectInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return toolError("Project name is required"), nil, nil
	}

	// If deleting the current project, clear the session
	_, currentName, ok := t.Session.GetCurrent()
	if ok && currentName == input.Name {
		t.Session.Clear()
	}

	err := t.Meta.DeleteProject(input.Name)
	if err != nil {
		return toolError("Failed to delete project: %v", err), nil, nil
	}

	return toolText(fmt.Sprintf("Project %q permanently deleted.", input.Name)), nil, nil
}

func (t *ProjectTools) RestoreProject(_ context.Context, _ *mcp.CallToolRequest, input RestoreProjectInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return toolError("Project name is required"), nil, nil
	}

	proj, err := t.Meta.RestoreProject(input.Name)
	if err != nil {
		return toolError("Failed to restore project: %v", err), nil, nil
	}

	return toolJSON(proj)
}

// --- Helpers ---

func toolText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func toolError(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}

func toolJSON(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError("Failed to marshal result: %v", err), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}
