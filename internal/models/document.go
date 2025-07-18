package models

type Document struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	ModTime   string `json:"mod_time"`
	Extension string `json:"extension"`
	Content   string `json:"content"`
}

// Returns New Document object
func NewDocument(path string, size int64, modTime string, extension string, content string) *Document {

	return &Document{
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		Extension: extension,
		Content:   content,
	}
}
