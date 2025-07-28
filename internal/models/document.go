package models

import "path/filepath"

type Document struct {
	Path      string  `json:"path"`
	Dir       string  `json:"dir"`
	Size      int64   `json:"size"`
	Score     float64 `json:"score"`
	ModTime   string  `json:"mod_time"`
	Extension string  `json:"extension"`
	Content   string  `json:"content"`
}

// Returns New Document object
func NewDocument(path string, size int64, modTime string, extension string, content string) *Document {

	return &Document{
		Path:      path,
		Dir:       filepath.Dir(path),
		Size:      size,
		ModTime:   modTime,
		Extension: extension,
		Content:   content,
	}
}
