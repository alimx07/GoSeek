package walker

import (
	"GoSeek/internal/models"
	"GoSeek/internal/reader"
	"fmt"
	"io/fs"
	"path/filepath"
)

type Walker struct {
	Root              string
	allowedExtensions map[string]bool
}

// NewWalker returns a pointer to Walker Instance standing
// on current root path and intersted in specific files with extensions in allowedExtensions slice

func NewWalker(root string, allowedExtensions []string) *Walker {
	m := make(map[string]bool)
	for _, ext := range allowedExtensions {
		m[ext] = true
	}
	return &Walker{
		Root:              root,
		allowedExtensions: m,
	}
}

// Walk is the main method of the walker instance
// It starts to traverse the system using filepath.WalkDir func
// It is also the producer func to Index consumer

// TODO:
// Try using fastwalk module (It is stated as being much faster than filepath.WalkDir)

func (w *Walker) Walk(docChan chan<- *models.Document) {

	chunkSize := 5 * 1024 * 1024
	fileChan := make(chan string, 100)
	go func() {
		reader.StreamToChannel(chunkSize, fileChan, docChan) // GO READ SOME FILES CONTENT
		close(docChan)
	}()
	
	err := filepath.WalkDir(w.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %q: %v", path, err)
		}
		if d.IsDir() {
			return nil // Skip directories for now
		}
		ext := filepath.Ext(path)
		if _, ok := w.allowedExtensions[ext]; !ok {
			return nil
		}
		fileChan <- path
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
		return
	}

	close(fileChan)
}
