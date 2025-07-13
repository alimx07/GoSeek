package indexer

import (
	"GoSeek/internal/models"
	"fmt"
)

// General Indexer Interface

// May Be --> Add new indexers
type Indexer interface {

	// It is a consumer function to Walk func of Walker Instance
	Index(docChan <-chan *models.Document)

	// TODO:
	// Try to seperate the search operation from Indexer instance
	// More modular + for handing more complex features in search
	Search(query string) ([]*models.Document, error)
	Close() error
	Stats()
}

type IndexStats struct {
	TotalSize        uint64
	DocumentsIndexed int
}

func (stats IndexStats) String() string {
	return fmt.Sprintf("%v Documents Indexed of Size %v MB", stats.DocumentsIndexed, stats.TotalSize/(1024*1024))
}
