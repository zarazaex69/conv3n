package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBlockRegistry_Register(t *testing.T) {
	registry := NewBlockRegistry("/tmp")

	manifest := BlockManifest{
		Name:        "test/block",
		Type:        "test/block",
		Description: "Test block",
		ScriptPath:  "/tmp/test.ts",
		Category:    "test",
	}

	tmpFile := filepath.Join(os.TempDir(), "test.ts")
	if err := os.WriteFile(tmpFile, []byte("console.log('test')"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	manifest.ScriptPath = tmpFile

	if err := registry.Register(manifest); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	retrieved, ok := registry.Get("test/block")
	if !ok {
		t.Fatal("Block not found after registration")
	}

	if retrieved.Name != manifest.Name {
		t.Errorf("Expected name %s, got %s", manifest.Name, retrieved.Name)
	}
}

func TestBlockRegistry_LoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "example.ts")
	if err := os.WriteFile(scriptPath, []byte("export class Example {}"), 0644); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(tmpDir, "example.block.json")
	manifestContent := `{
		"name": "test/example",
		"type": "test/example",
		"description": "Example block",
		"scriptPath": "example.ts",
		"category": "test"
	}`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewBlockRegistry(tmpDir)
	if err := registry.LoadFromDirectory(tmpDir); err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	manifest, ok := registry.Get("test/example")
	if !ok {
		t.Fatal("Block not loaded from directory")
	}

	if manifest.Name != "test/example" {
		t.Errorf("Expected name test/example, got %s", manifest.Name)
	}

	if !filepath.IsAbs(manifest.ScriptPath) {
		t.Error("ScriptPath should be absolute after loading")
	}
}

func TestBlockRegistry_List(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewBlockRegistry(tmpDir)

	for i := 0; i < 3; i++ {
		scriptPath := filepath.Join(tmpDir, "test.ts")
		if err := os.WriteFile(scriptPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		manifest := BlockManifest{
			Name:       "test/block",
			Type:       NodeType("test/block" + string(rune(i))),
			ScriptPath: scriptPath,
		}
		if err := registry.Register(manifest); err != nil {
			t.Fatal(err)
		}
	}

	manifests := registry.List()
	if len(manifests) != 3 {
		t.Errorf("Expected 3 manifests, got %d", len(manifests))
	}
}

func TestBlockRegistry_ValidationErrors(t *testing.T) {
	registry := NewBlockRegistry("/tmp")

	tests := []struct {
		name     string
		manifest BlockManifest
		wantErr  bool
	}{
		{
			name: "empty type",
			manifest: BlockManifest{
				ScriptPath: "/tmp/test.ts",
			},
			wantErr: true,
		},
		{
			name: "empty script path",
			manifest: BlockManifest{
				Type: "test/block",
			},
			wantErr: true,
		},
		{
			name: "non-existent script",
			manifest: BlockManifest{
				Type:       "test/block",
				ScriptPath: "/nonexistent/path.ts",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(tt.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
