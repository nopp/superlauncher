package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "superlauncher-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	// Initial list empty
	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(list))
	}

	// Create
	app := &App{
		Name:        "Solid Launcher 3D",
		URL:         "http://192.168.1.100:8080",
		Description: "3D printer and models manager",
		Position:    10,
		NewTab:      true,
	}
	if err := store.Create(app); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if app.ID == 0 {
		t.Fatalf("expected positive ID, got %d", app.ID)
	}

	// Read
	got, err := store.Get(app.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != app.Name || got.URL != app.URL {
		t.Fatalf("mismatch: got %+v, expected %+v", got, app)
	}

	// Update
	app.Name = "Workshop 3D"
	app.Icon = "http://example.com/icon.png"
	if err := store.Update(app); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err = store.Get(app.ID)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if got.Name != "Workshop 3D" || got.Icon != "http://example.com/icon.png" {
		t.Fatalf("update not reflected: %+v", got)
	}

	// Delete
	if err := store.Delete(app.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	list, err = store.List()
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 apps after delete, got %d", len(list))
	}
}
