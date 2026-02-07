package session

import (
	"fmt"
	"sync"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/storage"
)

// Session holds the current project context for an MCP session.
type Session struct {
	mu                 sync.Mutex
	currentProjectID   string
	currentProjectName string
	projectDB          *storage.ProjectStore
}

// New creates a new empty session with no active project.
func New() *Session {
	return &Session{}
}

// SwitchProject closes the current project (if any) and opens the given one.
func (s *Session) SwitchProject(meta *storage.MetaStore, name string) (*models.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proj, err := meta.GetProjectByName(name)
	if err != nil {
		return nil, err
	}
	if proj.Status == "archived" {
		return nil, fmt.Errorf("project %q is archived — restore it first", name)
	}

	// Close current project DB if open
	if s.projectDB != nil {
		s.projectDB.Close()
		s.projectDB = nil
	}

	dbPath := meta.ProjectDBPath(proj)
	pdb, err := storage.OpenProject(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open project db: %w", err)
	}

	s.currentProjectID = proj.ID
	s.currentProjectName = proj.Name
	s.projectDB = pdb

	return proj, nil
}

// GetCurrent returns info about the current project, or nil if none is active.
func (s *Session) GetCurrent() (id, name string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projectDB == nil {
		return "", "", false
	}
	return s.currentProjectID, s.currentProjectName, true
}

// ProjectStore returns the current project's storage, or nil if no project is active.
func (s *Session) ProjectStore() *storage.ProjectStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectDB
}

// Clear closes the current project and resets session state.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projectDB != nil {
		s.projectDB.Close()
		s.projectDB = nil
	}
	s.currentProjectID = ""
	s.currentProjectName = ""
}

// Close is an alias for Clear, used during server shutdown.
func (s *Session) Close() {
	s.Clear()
}
