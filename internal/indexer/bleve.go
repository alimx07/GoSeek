package indexer

import (
	// "GoSeek/config"
	"GoSeek/internal/db"
	"GoSeek/internal/models"
	"fmt"
	"os"
	"time"

	"sync"

	"github.com/blevesearch/bleve/v2"
)

type BleveIndexer struct {
	Index_m           bleve.Index
	MaxLimitBatchSize int
	stats             IndexStats
	statsLock         sync.Mutex
	DB                *db.DB
	numWorkers        int
}

// NewBleveIndexer creates a new BleveIndexer in specific path
// with default configurations as all fields are not indexed
// except the content one + It uses scorch engine as backend

// TODO:

// Make configurations more customized according to user choices
// Handle indexPath operations and Cases (already found index in this path , Rename by user op , etc..)

func NewBleveIndexer(indexpath string, batchLimit int, numWorkers int) *BleveIndexer {

	// IF IT IS FOUND , JUST DELETE FOR NOW
	if _, err := os.Stat(indexpath); err == nil {
		os.RemoveAll(indexpath)
	}

	indexMapping := bleve.NewIndexMapping()

	// Fields
	contentField := bleve.NewTextFieldMapping()
	contentField.Index = true
	contentField.Store = false
	contentField.IncludeTermVectors = false // Enable it for highlighting words feature

	sizeField := bleve.NewNumericFieldMapping()
	sizeField.Index = false
	sizeField.Store = true
	sizeField.IncludeTermVectors = false
	sizeField.IncludeInAll = false

	modTimeField := bleve.NewTextFieldMapping()
	modTimeField.Index = false
	modTimeField.Store = true
	modTimeField.IncludeTermVectors = false
	modTimeField.IncludeInAll = false

	extensionField := bleve.NewTextFieldMapping()
	extensionField.Index = false
	extensionField.Store = true
	extensionField.IncludeTermVectors = false
	extensionField.IncludeInAll = false

	documentMapping := bleve.NewDocumentMapping()
	documentMapping.AddFieldMappingsAt("content", contentField)
	documentMapping.AddFieldMappingsAt("size", sizeField)
	documentMapping.AddFieldMappingsAt("mod_time", modTimeField)
	documentMapping.AddFieldMappingsAt("extension", extensionField)

	indexMapping.DefaultMapping = documentMapping

	// scorchConfig := map[string]interface{}{
	// 	"unsafe_batches":     true, // Enable unsafe mode
	// 	"numSnapshotsToKeep": 1,    // Keep minimal snapshots
	// 	// "mem_quota": 67108864, // 64MB memory quota
	// 	// "forceMergeDeletesPctThreshold": 10.0,
	// }

	index, err := bleve.NewUsing(indexpath, indexMapping, bleve.Config.DefaultIndexType, "scorch", nil)
	if err != nil {
		return nil
	}
	return &BleveIndexer{
		Index_m:           index,
		stats:             IndexStats{},
		MaxLimitBatchSize: batchLimit,
		numWorkers:        numWorkers,
	}
}

func (bi *BleveIndexer) BatchIndex(batch *bleve.Batch) error {
	return bi.Index_m.Batch(batch)
}

// NewBatch - Create new batch
func (bi *BleveIndexer) NewBatch() *bleve.Batch {
	return bi.Index_m.NewBatch()
}

// IndexDocument - Index single document to batch
func (bi *BleveIndexer) IndexDocument(batch *bleve.Batch, doc *models.Document) error {
	return batch.Index(doc.Path, doc)
}

// DeleteDocument - Delete document from index
func (bi *BleveIndexer) DeleteDocument(batch *bleve.Batch, filePath string) {
	batch.Delete(filePath)
}

func (bi *BleveIndexer) FlushBatch(batch *bleve.Batch, batchSize, batchCount *int) {
	start := time.Now()
	// println("-----> batchSize:", *batchSize/(1024*1024))
	if batch.Size() > 0 {
		if err := bi.BatchIndex(batch); err != nil {
			fmt.Printf("Error indexing batch: %v\n", err)
		} else {
			bi.UpdateStats(*batchSize, *batchCount)
			// fmt.Printf("Indexed batch: %d docs, %d MB\n", batchCount, batchSize/(1024*1024))
		}
		*batchSize = 0
		*batchCount = 0
	}
	end := time.Now()
	println("---------->", end.Sub(start).String()) // Time for every batch
}

// Search return the results found in index according to the query

// TODO:

// Tons of feature missing right now (highlighting , wildcard search , etc....)
// Work on them later
func (bi *BleveIndexer) Search(query string) ([]*models.Document, error) {
	fmt.Println(query)
	Query := bleve.NewQueryStringQuery(query)
	SearchRequest := bleve.NewSearchRequest(Query)
	SearchRequest.Fields = []string{"path", "size", "mod_time", "extension"}
	SearchResult, err := bi.Index_m.Search(SearchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search index: %w", err)
	}
	var results []*models.Document
	for _, hit := range SearchResult.Hits {
		fmt.Println(hit.ID)
		doc := &models.Document{
			Path:      hit.ID,
			Size:      int64(hit.Fields["size"].(float64)),
			ModTime:   hit.Fields["mod_time"].(string),
			Extension: hit.Fields["extension"].(string),
			// Content: hit.Fields["Content"].(string),
		}
		println(doc.Path, doc.Size, doc.Extension, doc.ModTime)
		results = append(results, doc)
	}
	return results, nil
}

// Close the index and release the resources
func (bi *BleveIndexer) Close() error {
	if err := bi.Index_m.Close(); err != nil {
		return fmt.Errorf("failed to close index: %w", err)
	}
	return nil
}

// Update the statistics of the indexer
func (bi *BleveIndexer) UpdateStats(currSize, currCount int) {
	bi.statsLock.Lock()
	bi.stats.DocumentsIndexed += currCount
	bi.stats.TotalSize += currSize
	bi.statsLock.Unlock()
}

func (bi *BleveIndexer) Stats() IndexStats {
	return bi.stats
}
