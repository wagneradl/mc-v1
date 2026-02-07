package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "memory-mcp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestOpenMeta(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatalf("OpenMeta: %v", err)
	}
	defer meta.Close()

	// Verify directories were created
	for _, sub := range []string{"projects", "archive"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("Expected %s dir to exist: %v", sub, err)
		}
	}
	// Verify _meta.db was created
	if _, err := os.Stat(filepath.Join(dir, "_meta.db")); err != nil {
		t.Errorf("Expected _meta.db to exist: %v", err)
	}
}

func TestCreateAndGetProject(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	proj, err := meta.CreateProject("test-project", "A test project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if proj.Name != "test-project" {
		t.Errorf("Name = %q, want %q", proj.Name, "test-project")
	}
	if proj.Description != "A test project" {
		t.Errorf("Description = %q, want %q", proj.Description, "A test project")
	}
	if proj.Status != "active" {
		t.Errorf("Status = %q, want %q", proj.Status, "active")
	}
	if proj.ID == "" {
		t.Error("ID should not be empty")
	}

	// Verify project DB file was created
	dbPath := filepath.Join(dir, proj.DBPath)
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("Project DB file should exist at %s: %v", dbPath, err)
	}

	// Get by name
	got, err := meta.GetProjectByName("test-project")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	if got.ID != proj.ID {
		t.Errorf("GetByName ID = %q, want %q", got.ID, proj.ID)
	}

	// Get by ID
	got2, err := meta.GetProjectByID(proj.ID)
	if err != nil {
		t.Fatalf("GetProjectByID: %v", err)
	}
	if got2.Name != "test-project" {
		t.Errorf("GetByID Name = %q, want %q", got2.Name, "test-project")
	}
}

func TestCreateDuplicateProject(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	_, err = meta.CreateProject("dup", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = meta.CreateProject("dup", "")
	if err == nil {
		t.Error("Expected error on duplicate project name")
	}
}

func TestListProjects(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	meta.CreateProject("alpha", "")
	meta.CreateProject("beta", "")

	projects, err := meta.ListProjects("active")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("ListProjects(active) = %d projects, want 2", len(projects))
	}

	projects, err = meta.ListProjects("archived")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("ListProjects(archived) = %d projects, want 0", len(projects))
	}
}

func TestArchiveAndRestoreProject(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	meta.CreateProject("archivable", "")

	archived, err := meta.ArchiveProject("archivable")
	if err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	if archived.Status != "archived" {
		t.Errorf("Status = %q, want %q", archived.Status, "archived")
	}

	// DB file should be in archive/
	archivePath := filepath.Join(dir, archived.DBPath)
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("Archived DB should exist at %s: %v", archivePath, err)
	}

	// Should appear in archived list
	projects, _ := meta.ListProjects("archived")
	if len(projects) != 1 {
		t.Errorf("Expected 1 archived project, got %d", len(projects))
	}

	// Restore
	restored, err := meta.RestoreProject("archivable")
	if err != nil {
		t.Fatalf("RestoreProject: %v", err)
	}
	if restored.Status != "active" {
		t.Errorf("Status = %q, want %q", restored.Status, "active")
	}

	// DB file should be back in projects/
	restoredPath := filepath.Join(dir, restored.DBPath)
	if _, err := os.Stat(restoredPath); err != nil {
		t.Errorf("Restored DB should exist at %s: %v", restoredPath, err)
	}
}

func TestDeleteProject(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	proj, _ := meta.CreateProject("deletable", "")
	dbPath := filepath.Join(dir, proj.DBPath)

	err = meta.DeleteProject("deletable")
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// DB file should be gone
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("DB file should have been deleted")
	}

	// Should not appear in any list
	projects, _ := meta.ListProjects("all")
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}

func TestGetNonExistentProject(t *testing.T) {
	dir := tempDir(t)
	meta, err := OpenMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer meta.Close()

	_, err = meta.GetProjectByName("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent project")
	}
}
