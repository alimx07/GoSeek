package config

import (
	"errors"
)

type Config struct {
	RootPath              string
	IndexPath             string
	AllowedExtensions     map[string]bool
	ChunkSize             int
	NumWorkers            int
	WatcherIdleTime       int
	IndexBatchMemoryLimit int
	ChannelBufferSize     int
}

var (
	ErrInvalidRootPath    = errors.New("invalid folder path")
	ErrInvalidIndexPath   = errors.New("invalid index path")
	ErrInvalidChunkSize   = errors.New("chunk size must be greater than 0")
	ErrInvalidNumWorkers  = errors.New("number of workers must be greater than 0")
	ErrInvalidBufferSize  = errors.New("channel buffer size must be greater than 0")
	ErrInvalidMemoryLimit = errors.New("index batch memory limit must be greater than 0")
	// ErrNoAllowedExtensions = errors.New("no allowed extensions specified")
)

// The Config returns from user
// right now just use some default values
func LoadConfig() (*Config, error) {
	return &Config{
		RootPath:  ".",
		IndexPath: "index.bleve",
		AllowedExtensions: map[string]bool{
			".txt":  true,
			".md":   true,
			".json": true,
			".log":  true,
			".go":   true,
			".py":   true,
			".java": true,
		},
		ChunkSize:             1 * 1024 * 1024,
		NumWorkers:            4,
		IndexBatchMemoryLimit: 32 * 1024 * 1024,
		ChannelBufferSize:     16,
	}, nil
}
