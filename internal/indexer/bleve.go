package indexer

import (
	// "GoSeek/config"

	"GoSeek/internal/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sync"

	"github.com/blevesearch/bleve/v2"
)

type BleveIndexer struct {
	Index     bleve.Index
	stats     IndexStats
	statsLock sync.Mutex
}

// NewBleveIndexer creates a new BleveIndexer in specific path
// with default configurations as all fields are not indexed
// except the content one + It uses scorch engine as backend

// TODO:

// Make configurations more customized according to user choices
// Handle indexPath operations and Cases (already found index in this path , Rename by user op , etc..)

func NewBleveIndexer(folderPath string, extensions map[string]bool) (*BleveIndexer, error) {

	// IF IT IS FOUND RETURN IT
	indexpath := "index/" + filepath.Base(folderPath)
	currIndex := OpenBleve(indexpath)
	if currIndex != nil {
		currIndex.Index.Close()
		return nil, fmt.Errorf("there is already index with that path")
	}

	indexMapping := bleve.NewIndexMapping()

	// Fields
	contentField := bleve.NewTextFieldMapping()
	contentField.Index = true
	contentField.Store = false
	contentField.IncludeTermVectors = false

	dirFiled := bleve.NewTextFieldMapping()
	dirFiled.Index = true
	dirFiled.Store = false
	dirFiled.IncludeTermVectors = false
	dirFiled.IncludeInAll = false
	dirFiled.Analyzer = "keyword"

	sizeField := bleve.NewNumericFieldMapping()
	sizeField.Index = true
	sizeField.Store = true
	sizeField.IncludeTermVectors = false
	sizeField.IncludeInAll = false

	modTimeField := bleve.NewDateTimeFieldMapping()
	modTimeField.Index = true
	modTimeField.Store = true
	modTimeField.IncludeTermVectors = false
	modTimeField.IncludeInAll = false

	extensionField := bleve.NewTextFieldMapping()
	extensionField.Index = true
	extensionField.Store = true
	extensionField.IncludeTermVectors = false
	extensionField.IncludeInAll = false

	documentMapping := bleve.NewDocumentMapping()
	documentMapping.AddFieldMappingsAt("dir", dirFiled)
	documentMapping.AddFieldMappingsAt("content", contentField)
	documentMapping.AddFieldMappingsAt("size", sizeField)
	documentMapping.AddFieldMappingsAt("mod_time", modTimeField)
	documentMapping.AddFieldMappingsAt("extension", extensionField)

	indexMapping.DefaultMapping = documentMapping

	index, err := bleve.NewUsing(indexpath, indexMapping, bleve.Config.DefaultIndexType, "scorch", nil)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(extensions)

	err = index.SetInternal([]byte("__extensions__"), data)
	if err != nil {
		return nil, err
	}
	err = index.SetInternal([]byte("__base_path__"), []byte(filepath.Dir(folderPath)))
	if err != nil {
		return nil, err
	}
	return &BleveIndexer{
		Index: index,
		stats: IndexStats{},
	}, nil
}

func OpenBleve(indexpath string) *BleveIndexer {
	_, err := os.Stat(indexpath)
	if err != nil {
		println(err)
		return nil
	}
	var index bleve.Index
	index, err = bleve.Open(indexpath)
	if err != nil {
		return nil
	}
	return &BleveIndexer{
		Index: index,
		stats: IndexStats{},
	}
}

func (bi *BleveIndexer) BatchIndex(batch *bleve.Batch) error {
	return bi.Index.Batch(batch)
}

// NewBatch - Create new batch
func (bi *BleveIndexer) NewBatch() *bleve.Batch {
	return bi.Index.NewBatch()
}

// IndexDocument - Index single document to batch
func (bi *BleveIndexer) IndexDocument(batch *bleve.Batch, doc *models.Document) error {
	return batch.Index(doc.Path, doc)
}

// DeleteDocument - Delete document from index
func (bi *BleveIndexer) DeleteDocumentBatch(batch *bleve.Batch, filePath string) {
	batch.Delete(filePath)
}

func (bi *BleveIndexer) DeleteSingleDocument(filePath string) {
	bi.Index.Delete(filePath)
}

func (bi *BleveIndexer) FlushBatch(batch *bleve.Batch, batchSize, batchCount *int32) {
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

func (bi *BleveIndexer) Search(req *bleve.SearchRequest) ([]models.Document, error) {
	basepath, err := bi.Index.GetInternal([]byte("__base_path__"))
	if err != nil {
		return nil, err
	}
	basePath := string(basepath)
	SearchResult, err := bi.Index.Search(req)
	if SearchResult == nil {
		println(err.Error())
		return nil, nil
	}
	var results []models.Document
	for _, hit := range SearchResult.Hits {
		if hit.Fields == nil {
			return nil, nil
		}
		// fmt.Println(hit.ID)
		doc := models.Document{
			Path:      basePath + string(filepath.Separator) + hit.ID,
			Score:     hit.Score,
			Size:      int64(hit.Fields["size"].(float64)),
			ModTime:   hit.Fields["mod_time"].(string),
			Extension: hit.Fields["extension"].(string),
			// Dir:       hit.Fields["dir"].(string),
			// Content: hit.Fields["Content"].(string),
		}
		// if snippets, ok := hit.Fragments["content"]; ok {
		// 	for _, snippet := range snippets {
		// 		fmt.Println("Snippet:", snippet)
		// 	}
		// }
		// println(doc.Path, doc.Size, doc.Extension, doc.ModTime)
		results = append(results, doc)
	}
	return results, nil
}

// Close the index and release the resources
func (bi *BleveIndexer) Close() error {
	if err := bi.Index.Close(); err != nil {
		return fmt.Errorf("failed to close index: %w", err)
	}
	return nil
}

// Update the statistics of the indexer
func (bi *BleveIndexer) UpdateStats(currSize, currCount int32) {
	bi.statsLock.Lock()
	bi.stats.DocumentsIndexed += currCount
	bi.stats.TotalSize += currSize
	bi.statsLock.Unlock()
}

func (bi *BleveIndexer) Stats() IndexStats {
	return bi.stats
}
