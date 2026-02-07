package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/wagnerlima/memory-cloud/memory-mcp/internal/models"
)

// MetaStore manages the central _meta.db database that tracks all projects.
type MetaStore struct {
	db      *sql.DB
	dataDir string
}

// OpenMeta opens (or creates) the _meta.db database and runs migrations.
func OpenMeta(dataDir string) (*MetaStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "projects"), 0o755); err != nil {
		return nil, fmt.Errorf("create projects dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "archive"), 0o755); err != nil {
		return nil, fmt.Errorf("create archive dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "_meta.db")
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open meta db: %w", err)
	}

	if _, err := db.Exec(MetaSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate meta db: %w", err)
	}

	return &MetaStore{db: db, dataDir: dataDir}, nil
}

// Close closes the database connection.
func (m *MetaStore) Close() error {
	return m.db.Close()
}

// DataDir returns the base data directory.
func (m *MetaStore) DataDir() string {
	return m.dataDir
}

// CreateProject creates a new project entry and its isolated database file.
func (m *MetaStore) CreateProject(name, description string) (*models.Project, error) {
	id := uuid.New().String()
	dbPath := filepath.Join("projects", id+".db")

	_, err := m.db.Exec(
		`INSERT INTO projects (id, name, description, db_path, status) VALUES (?, ?, ?, ?, 'active')`,
		id, name, description, dbPath,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	// Create the project database file with schema
	absDBPath := filepath.Join(m.dataDir, dbPath)
	if err := initProjectDB(absDBPath); err != nil {
		// Rollback: remove meta entry if project DB creation fails
		m.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
		return nil, fmt.Errorf("init project db: %w", err)
	}

	return m.GetProjectByName(name)
}

// GetProjectByName looks up a project by its unique name.
func (m *MetaStore) GetProjectByName(name string) (*models.Project, error) {
	row := m.db.QueryRow(
		`SELECT id, name, description, db_path, status, created_at, updated_at FROM projects WHERE name = ?`,
		name,
	)
	return scanProject(row)
}

// GetProjectByID looks up a project by its UUID.
func (m *MetaStore) GetProjectByID(id string) (*models.Project, error) {
	row := m.db.QueryRow(
		`SELECT id, name, description, db_path, status, created_at, updated_at FROM projects WHERE id = ?`,
		id,
	)
	return scanProject(row)
}

// ListProjects returns projects filtered by status. Use "all" for no filter.
func (m *MetaStore) ListProjects(status string) ([]models.Project, error) {
	var rows *sql.Rows
	var err error

	if status == "all" {
		rows, err = m.db.Query(
			`SELECT id, name, description, db_path, status, created_at, updated_at FROM projects ORDER BY name`,
		)
	} else {
		rows, err = m.db.Query(
			`SELECT id, name, description, db_path, status, created_at, updated_at FROM projects WHERE status = ? ORDER BY name`,
			status,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.DBPath, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProjectStatus changes a project's status (active/archived) and updates updated_at.
func (m *MetaStore) UpdateProjectStatus(name, status string) (*models.Project, error) {
	result, err := m.db.Exec(
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE name = ?`,
		status, name,
	)
	if err != nil {
		return nil, fmt.Errorf("update project status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("project %q not found", name)
	}
	return m.GetProjectByName(name)
}

// ArchiveProject archives a project: sets status to 'archived' and moves
// the DB file from projects/ to archive/.
func (m *MetaStore) ArchiveProject(name string) (*models.Project, error) {
	proj, err := m.GetProjectByName(name)
	if err != nil {
		return nil, err
	}
	if proj.Status == "archived" {
		return nil, fmt.Errorf("project %q is already archived", name)
	}

	oldPath := filepath.Join(m.dataDir, proj.DBPath)
	newRelPath := filepath.Join("archive", filepath.Base(proj.DBPath))
	newPath := filepath.Join(m.dataDir, newRelPath)

	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("move project db to archive: %w", err)
	}

	_, err = m.db.Exec(
		`UPDATE projects SET status = 'archived', db_path = ?, updated_at = datetime('now') WHERE name = ?`,
		newRelPath, name,
	)
	if err != nil {
		// Try to undo the file move
		os.Rename(newPath, oldPath)
		return nil, fmt.Errorf("update project status: %w", err)
	}

	return m.GetProjectByName(name)
}

// RestoreProject restores an archived project back to active status.
func (m *MetaStore) RestoreProject(name string) (*models.Project, error) {
	proj, err := m.GetProjectByName(name)
	if err != nil {
		return nil, err
	}
	if proj.Status != "archived" {
		return nil, fmt.Errorf("project %q is not archived", name)
	}

	oldPath := filepath.Join(m.dataDir, proj.DBPath)
	newRelPath := filepath.Join("projects", filepath.Base(proj.DBPath))
	newPath := filepath.Join(m.dataDir, newRelPath)

	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("move project db from archive: %w", err)
	}

	_, err = m.db.Exec(
		`UPDATE projects SET status = 'active', db_path = ?, updated_at = datetime('now') WHERE name = ?`,
		newRelPath, name,
	)
	if err != nil {
		os.Rename(newPath, oldPath)
		return nil, fmt.Errorf("update project status: %w", err)
	}

	return m.GetProjectByName(name)
}

// DeleteProject permanently removes a project record and its database file.
func (m *MetaStore) DeleteProject(name string) error {
	proj, err := m.GetProjectByName(name)
	if err != nil {
		return err
	}

	absDBPath := filepath.Join(m.dataDir, proj.DBPath)

	// Remove DB file (ignore error if already gone)
	os.Remove(absDBPath)
	// Also remove WAL/SHM files if present
	os.Remove(absDBPath + "-wal")
	os.Remove(absDBPath + "-shm")

	_, err = m.db.Exec(`DELETE FROM projects WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete project record: %w", err)
	}
	return nil
}

// ProjectDBPath returns the absolute path to a project's database file.
func (m *MetaStore) ProjectDBPath(proj *models.Project) string {
	return filepath.Join(m.dataDir, proj.DBPath)
}

// scanProject scans a single project row.
func scanProject(row *sql.Row) (*models.Project, error) {
	var p models.Project
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.DBPath, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

// initProjectDB creates a new project database with the full schema.
func initProjectDB(dbPath string) error {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(ProjectSchema); err != nil {
		return fmt.Errorf("create project schema: %w", err)
	}
	if _, err := db.Exec(ProjectTriggers); err != nil {
		return fmt.Errorf("create project triggers: %w", err)
	}
	return nil
}
