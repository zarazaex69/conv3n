package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type BlockManifest struct {
	Name         string                 `json:"name"`
	Type         NodeType               `json:"type"`
	Description  string                 `json:"description"`
	ScriptPath   string                 `json:"scriptPath"`
	Category     string                 `json:"category"`
	Inputs       []PortDefinition       `json:"inputs"`
	Outputs      []PortDefinition       `json:"outputs"`
	ConfigSchema map[string]interface{} `json:"configSchema,omitempty"`
}

type PortDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type BlockRegistry struct {
	mu       sync.RWMutex
	blocks   map[NodeType]BlockManifest
	basePath string
}

func NewBlockRegistry(basePath string) *BlockRegistry {
	return &BlockRegistry{
		blocks:   make(map[NodeType]BlockManifest),
		basePath: basePath,
	}
}

func (r *BlockRegistry) Register(manifest BlockManifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if manifest.Type == "" {
		return fmt.Errorf("block type cannot be empty")
	}

	if manifest.ScriptPath == "" {
		return fmt.Errorf("script path cannot be empty for block %s", manifest.Type)
	}

	if !filepath.IsAbs(manifest.ScriptPath) {
		manifest.ScriptPath = filepath.Join(r.basePath, manifest.ScriptPath)
	}

	if _, err := os.Stat(manifest.ScriptPath); err != nil {
		return fmt.Errorf("script not found: %s", manifest.ScriptPath)
	}

	r.blocks[manifest.Type] = manifest
	return nil
}

func (r *BlockRegistry) Get(nodeType NodeType) (BlockManifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	manifest, ok := r.blocks[nodeType]
	return manifest, ok
}

func (r *BlockRegistry) List() []BlockManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifests := make([]BlockManifest, 0, len(r.blocks))
	for _, m := range r.blocks {
		manifests = append(manifests, m)
	}
	return manifests
}

func (r *BlockRegistry) LoadFromDirectory(dir string) error {
	pattern := filepath.Join(dir, "**", "*.block.json")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(matches) == 0 {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".block.json") {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
	}

	for _, path := range matches {
		if err := r.loadManifest(path); err != nil {
			return fmt.Errorf("failed to load manifest %s: %w", path, err)
		}
	}

	return nil
}

func (r *BlockRegistry) loadManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var manifest BlockManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("invalid manifest JSON: %w", err)
	}

	if manifest.ScriptPath == "" {
		dir := filepath.Dir(path)
		base := strings.TrimSuffix(filepath.Base(path), ".block.json")
		manifest.ScriptPath = filepath.Join(dir, base+".ts")
	}

	return r.Register(manifest)
}
