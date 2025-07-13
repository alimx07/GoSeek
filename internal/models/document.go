package models

import (
	"fmt"
	"sync/atomic"
)

var globalIDCounter uint64

type Document struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Size      uint64 `json:"size"`
	ModTime   string `json:"mod_time"`
	Extension string `json:"extension"`
	Content   string `json:"content"`
}

// Returns New Document object with unique ID
func NewDocument(path string, size uint64, modTime string, extension string, content string) *Document {
	atomic.AddUint64(&globalIDCounter, 1) // Atomic Add
	return &Document{
		ID:        fmt.Sprintf("doc_%d", globalIDCounter), 
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		Extension: extension,
		Content:   content,
	}
}
