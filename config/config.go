package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	ChunkSize             int
	NumWorkers            int
	IndexBatchMemoryLimit int
	ChannelBufferSize     int
}

type IndexConfig struct {
	Name               string          `yaml:"name"`
	Folders            []string        `yaml:"folders"`
	IndexPath          string          `yaml:"index_path"`
	PendingChangesPath string          `yaml:"pending_changes_path"`
	Extensions         map[string]bool `yaml:"extensions"`
}

type Config struct {
	Indexes []*IndexConfig `yaml:"indexes"`
	// TODO:
	// Problem : Think of user write to file with auto save (multiple write events for every change)
	// Add mechanism to wait some time (Delay)
	// handle only last event
	// DebounceDelayMs int           `yaml:"debounce_delay_ms"`
	LogDir string `yaml:"log_dir"`
}

// Global configs of the app
func LoadGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		ChunkSize:             1 * 1024 * 1024,
		NumWorkers:            4,
		IndexBatchMemoryLimit: 32 * 1024 * 1024,
		ChannelBufferSize:     16,
	}
}

// Configs that contains the metadata about index
func NewIndexConfig(path string) (*IndexConfig, error) {
	Name := filepath.Base(path)
	return &IndexConfig{
		Name:      path,
		IndexPath: "../index/" + Name,
		Extensions: map[string]bool{
			".txt":  true,
			".md":   true,
			".json": true,
			".log":  true,
			".go":   true,
			".py":   true,
			".java": true,
		},
		PendingChangesPath: "../pending_changes/" + Name + ".txt",
	}, nil
}

func SaveConfigToYAML(config *Config, filePath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func ReadYAMLToConfig(config *Config, filePath string) error {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return err
	}
	return nil
}
