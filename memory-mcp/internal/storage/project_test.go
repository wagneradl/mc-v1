package storage

import (
	"path/filepath"
	"testing"
)

// setupProjectStore creates a fresh project DB in a temp directory and returns a ProjectStore.
func setupProjectStore(t *testing.T) *ProjectStore {
	t.Helper()
	dir := tempDir(t)
	dbPath := filepath.Join(dir, "test.db")
	if err := initProjectDB(dbPath); err != nil {
		t.Fatalf("initProjectDB: %v", err)
	}
	ps, err := OpenProject(dbPath)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	t.Cleanup(func() { ps.Close() })
	return ps
}

func TestCreateEntities(t *testing.T) {
	ps := setupProjectStore(t)

	entities := []struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology", Observations: []string{"Fast compiled language", "Great for CLI tools"}},
		{Name: "SQLite", EntityType: "technology", Observations: []string{"Embedded database"} },
	}

	created, err := ps.CreateEntities(entities)
	if err != nil {
		t.Fatalf("CreateEntities: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("Expected 2 created entities, got %d", len(created))
	}

	// Check Go entity
	goEntity := created[0]
	if goEntity.Name != "Go" {
		t.Errorf("Name = %q, want %q", goEntity.Name, "Go")
	}
	if goEntity.EntityType != "technology" {
		t.Errorf("EntityType = %q, want %q", goEntity.EntityType, "technology")
	}
	if len(goEntity.Observations) != 2 {
		t.Errorf("Expected 2 observations for Go, got %d", len(goEntity.Observations))
	}
	if goEntity.ID == "" {
		t.Error("Entity ID should not be empty")
	}
}

func TestAddObservations(t *testing.T) {
	ps := setupProjectStore(t)

	// Create entity first
	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{{Name: "Go", EntityType: "technology"}})

	obs, err := ps.AddObservations("Go", []string{"Version 1.22", "Supports generics"})
	if err != nil {
		t.Fatalf("AddObservations: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("Expected 2 observations, got %d", len(obs))
	}
	if obs[0].Content != "Version 1.22" {
		t.Errorf("Content = %q, want %q", obs[0].Content, "Version 1.22")
	}
}

func TestAddObservationsNonExistent(t *testing.T) {
	ps := setupProjectStore(t)

	_, err := ps.AddObservations("DoesNotExist", []string{"test"})
	if err == nil {
		t.Error("Expected error for nonexistent entity")
	}
}

func TestCreateRelations(t *testing.T) {
	ps := setupProjectStore(t)

	// Create entities
	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology"},
		{Name: "Memory Cloud", EntityType: "project"},
	})

	rels, err := ps.CreateRelations([]struct {
		From         string
		To           string
		RelationType string
	}{{From: "Go", To: "Memory Cloud", RelationType: "powers"}})

	if err != nil {
		t.Fatalf("CreateRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("Expected 1 relation, got %d", len(rels))
	}
	if rels[0].RelationType != "powers" {
		t.Errorf("RelationType = %q, want %q", rels[0].RelationType, "powers")
	}
}

func TestDeleteEntities(t *testing.T) {
	ps := setupProjectStore(t)

	// Create entity with observations and relations
	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology", Observations: []string{"Fast"}},
		{Name: "Rust", EntityType: "technology"},
	})
	ps.CreateRelations([]struct {
		From         string
		To           string
		RelationType string
	}{{From: "Go", To: "Rust", RelationType: "competes_with"}})

	count, err := ps.DeleteEntities([]string{"Go"})
	if err != nil {
		t.Fatalf("DeleteEntities: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 deleted, got %d", count)
	}

	// Verify Go is gone from active entities
	entities, err := ps.GetEntities([]string{"Go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 0 {
		t.Error("Deleted entity should not be returned by GetEntities")
	}

	// Verify Rust still exists but relation is gone
	entities, err = ps.GetEntities([]string{"Rust"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatal("Rust should still exist")
	}
	if len(entities[0].Relations) != 0 {
		t.Error("Relation involving deleted entity should be soft-deleted")
	}
}

func TestDeleteObservations(t *testing.T) {
	ps := setupProjectStore(t)

	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{{Name: "Go", EntityType: "technology", Observations: []string{"Fast", "Compiled", "Typed"}}})

	count, err := ps.DeleteObservations("Go", []string{"Fast", "Typed"})
	if err != nil {
		t.Fatalf("DeleteObservations: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 deleted, got %d", count)
	}

	// Verify only "Compiled" remains
	entities, _ := ps.GetEntities([]string{"Go"})
	if len(entities) != 1 {
		t.Fatal("Go should still exist")
	}
	if len(entities[0].Observations) != 1 {
		t.Fatalf("Expected 1 remaining observation, got %d", len(entities[0].Observations))
	}
	if entities[0].Observations[0].Content != "Compiled" {
		t.Errorf("Remaining observation = %q, want %q", entities[0].Observations[0].Content, "Compiled")
	}
}

func TestDeleteRelations(t *testing.T) {
	ps := setupProjectStore(t)

	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology"},
		{Name: "SQLite", EntityType: "technology"},
	})
	ps.CreateRelations([]struct {
		From         string
		To           string
		RelationType string
	}{{From: "Go", To: "SQLite", RelationType: "uses"}})

	count, err := ps.DeleteRelations([]struct {
		From         string
		To           string
		RelationType string
	}{{From: "Go", To: "SQLite", RelationType: "uses"}})
	if err != nil {
		t.Fatalf("DeleteRelations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 deleted, got %d", count)
	}

	// Verify relation is gone
	entities, _ := ps.GetEntities([]string{"Go"})
	if len(entities) != 1 {
		t.Fatal("Go should still exist")
	}
	if len(entities[0].Relations) != 0 {
		t.Error("Deleted relation should not appear")
	}
}

func TestGetEntities(t *testing.T) {
	ps := setupProjectStore(t)

	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology", Observations: []string{"Fast"}},
		{Name: "Rust", EntityType: "technology"},
		{Name: "Python", EntityType: "technology"},
	})

	entities, err := ps.GetEntities([]string{"Go", "Rust"})
	if err != nil {
		t.Fatalf("GetEntities: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}
}

func TestReadGraph(t *testing.T) {
	ps := setupProjectStore(t)

	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology", Observations: []string{"Fast"}},
		{Name: "SQLite", EntityType: "technology"},
	})
	ps.CreateRelations([]struct {
		From         string
		To           string
		RelationType string
	}{{From: "Go", To: "SQLite", RelationType: "uses"}})

	graph, err := ps.ReadGraph()
	if err != nil {
		t.Fatalf("ReadGraph: %v", err)
	}
	if len(graph.Entities) != 2 {
		t.Errorf("Expected 2 entities in graph, got %d", len(graph.Entities))
	}
	if len(graph.Relations) != 1 {
		t.Errorf("Expected 1 relation in graph, got %d", len(graph.Relations))
	}
}

func TestSearchFTS(t *testing.T) {
	ps := setupProjectStore(t)

	ps.CreateEntities([]struct {
		Name         string
		EntityType   string
		Observations []string
	}{
		{Name: "Go", EntityType: "technology", Observations: []string{"Fast compiled language"}},
		{Name: "Python", EntityType: "technology", Observations: []string{"Dynamic scripting language"}},
	})

	// Search for "compiled" should find Go via observation
	results, err := ps.Search("compiled")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'compiled', got %d", len(results))
	}
	if results[0].Name != "Go" {
		t.Errorf("Expected Go, got %q", results[0].Name)
	}

	// Search for entity name
	results, err = ps.Search("Python")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'Python', got %d", len(results))
	}

	// Search for "language" should find both
	results, err = ps.Search("language")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'language', got %d", len(results))
	}

	// Search for "technology" (entity type) should find both
	results, err = ps.Search("technology")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'technology', got %d", len(results))
	}
}
