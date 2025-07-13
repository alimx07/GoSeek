package indexer

import (
	"GoSeek/internal/models"
	"fmt"
	"os"

	"sync"

	"github.com/blevesearch/bleve/v2"
)

type BleveIndexer struct {
	index     bleve.Index
	stats     IndexStats
	statsLock sync.Mutex
}

// NewBleveIndexer creates a new BleveIndexer in specific path
// with default configurations as all fields are not indexed
// except the content one + It uses scorch engine as backend

// TODO:

// Make configurations more customized according to user choices
// Hanle indexPath operations and Cases (already found index in this path , Rename by user op , etc..)

func NewBleveIndexer(indexfile string) (*BleveIndexer, error) {

	// IF IT FOUND , JUST DELETE FOR NOW
	indexpath := indexfile + ".bleve"
	if _, err := os.Stat(indexpath); err == nil {
		os.RemoveAll(indexpath)
	}

	indexMapping := bleve.NewIndexMapping()

	// Fields
	contentField := bleve.NewTextFieldMapping()
	contentField.Index = true
	contentField.Store = false
	contentField.IncludeTermVectors = false // Enable it for highlighting words feature

	pathField := bleve.NewTextFieldMapping()
	pathField.Index = false
	pathField.Store = true
	pathField.IncludeTermVectors = false
	pathField.IncludeInAll = false

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
	documentMapping.AddFieldMappingsAt("path", pathField)
	documentMapping.AddFieldMappingsAt("size", sizeField)
	documentMapping.AddFieldMappingsAt("mod_time", modTimeField)
	documentMapping.AddFieldMappingsAt("extension", extensionField)

	indexMapping.DefaultMapping = documentMapping

	// scorchConfig := map[string]interface{}{
	// 	"unsafe_batches": true, // Enable unsafe mode
	// 	// "numSnapshotsToKeep": 1, // Keep minimal snapshots
	// 	// "mem_quota": 67108864, // 64MB memory quota
	// 	// "forceMergeDeletesPctThreshold": 10.0,
	// }

	index, err := bleve.NewUsing(indexpath, indexMapping, bleve.Config.DefaultIndexType, "scorch", nil)
	if err != nil {
		return nil, err
	}
	return &BleveIndexer{
		index: index,
		stats: IndexStats{}}, nil
}

// Index runs multiple go routines to index docs recieved from the Walker instance

// TODO:
// Memory Optimizations:
// --> Current Readings on My TestData 15GB:
// ----> Max Ram Used : Almost 3500 MB (With TermVectors OFF)
// ----> Max Ram Used : Dominate the Ram (With TermVectors ON)
// Seperate the errChan handling
func (bi *BleveIndexer) Index(docChan <-chan *models.Document) error {
	errChan := make(chan error, 10)
	numWorkers := 4
	batchSize := 100 // Try Different sizes and watch memory performance changes
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(errChan)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) error {
			defer wg.Done()
			batch := bi.index.NewBatch()
			var currSize uint64
			var currCount int
			for doc := range docChan {
				if err := batch.Index(doc.ID, doc); err != nil {
					errChan <- fmt.Errorf("worker %d: failed to index document %s: %w", workerID, doc.Path, err)
					continue
				}
				currSize += uint64(len(doc.Content))
				currCount++
				if batch.Size() >= batchSize/numWorkers {
					if err := bi.index.Batch(batch); err != nil {
						errChan <- fmt.Errorf("worker %d: failed to commit batch: %w", workerID, err)
						continue
					}
					bi.UpdateStats(currSize, currCount)
					batch = bi.index.NewBatch()
					currCount = 0
					currSize = 0
					// runtime.GC()
				}
			}
			if batch.Size() > 0 {
				if err := bi.index.Batch(batch); err != nil {
					errChan <- fmt.Errorf("worker %d: failed to commit final batch: %w", workerID, err)
				}
				bi.UpdateStats(currSize, currCount)
				// else {
				// 	bi.UpdateStats(currSize, batch.Size())
				// }
			}
			return nil
		}(i)
	}
	go func() {
		for err := range errChan {
			fmt.Println("Error:", err)
		}
	}()
	return nil
}

// Search return the results found in index according to the query

// TODO:

// Tons of Features missing right Now (highlighting , merge different Docs with same path , etc...)
// Work on them later
func (bi *BleveIndexer) Search(query string) ([]*models.Document, error) {
	fmt.Println(query)
	Query := bleve.NewQueryStringQuery(query)
	SearchRequest := bleve.NewSearchRequest(Query)
	SearchRequest.Fields = []string{"path", "size", "mod_time", "extension"}
	SearchResult, err := bi.index.Search(SearchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search index: %w", err)
	}
	var results []*models.Document
	for _, hit := range SearchResult.Hits {
		fmt.Println(hit.ID)
		doc := &models.Document{
			ID:        hit.ID,
			Path:      hit.Fields["path"].(string),
			Size:      uint64(hit.Fields["size"].(float64)),
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
	if err := bi.index.Close(); err != nil {
		return fmt.Errorf("failed to close index: %w", err)
	}
	return nil
}

// Update the statistics of the indexer
func (bi *BleveIndexer) UpdateStats(currSize uint64, currCount int) {
	bi.statsLock.Lock()
	bi.stats.DocumentsIndexed += currCount
	bi.stats.TotalSize += currSize
	bi.statsLock.Unlock()
}

func (bi *BleveIndexer) Stats() IndexStats {
	return bi.stats
}
