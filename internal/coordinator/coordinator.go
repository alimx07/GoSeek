package coordinator

import (
	"GoSeek/config"
	"GoSeek/internal/fileprocessor"
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"GoSeek/internal/watcher"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type WorkItem struct {
	Type     string // "create", "delete", "update"
	FilePath string
	// Data     interface{}
}

type Coordinator struct {
	fileprocessor *fileprocessor.FileProcessor
	watcher       *watcher.FileWatcher
	Indexer       *indexer.BleveIndexer
	// Cfg           *config.IndexConfig

	// Persistent channels
	workChan chan WorkItem
	fileChan chan string
	docChan  chan *models.Document

	// Worker control
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	UpdateChan  chan string
	onComplete  func()
	pendingWork int32
	mu          sync.RWMutex
}

var gCfg *config.GlobalConfig = config.LoadGlobalConfig()

// TODO:
// Use another dynamic way to intiallize coordinators
// of prevIndexes or new ones
func NewCoordinator(folderPath string, extensions map[string]bool) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	// indexPath := "index/" + filepath.Base(folderPath)
	indexer, err := indexer.NewBleveIndexer(folderPath, extensions)
	if err != nil {
		cancel()
		println(err)
		return nil
	}
	coord := &Coordinator{
		fileprocessor: fileprocessor.NewFileProcessor(filepath.Dir(folderPath), extensions, gCfg.ChunkSize, gCfg.NumWorkers),
		Indexer:       indexer,

		// channels
		workChan: make(chan WorkItem, gCfg.ChannelBufferSize*2),
		fileChan: make(chan string, gCfg.ChannelBufferSize*4),
		docChan:  make(chan *models.Document, gCfg.ChannelBufferSize),

		ctx:        ctx,
		cancel:     cancel,
		UpdateChan: make(chan string, 4),
	}

	coord.watcher = watcher.NewFileWatcher(
		func(path string) { coord.workChan <- WorkItem{Type: "delete", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "update", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "create", FilePath: path} },
	)

	// Start persistent workers
	coord.startWorkers()
	coord.watcher.StartWatching()

	return coord
}
func NewCoordinatorPrevIndex(path string) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	indexPath := "index/" + filepath.Base(path)
	indexer := indexer.OpenBleve(indexPath)
	data, err := indexer.Index.GetInternal([]byte("__extensions__"))
	if err != nil {
		cancel()
		return nil // For Now
	}
	var extensions map[string]bool
	json.Unmarshal(data, &extensions)
	coord := &Coordinator{
		fileprocessor: fileprocessor.NewFileProcessor(filepath.Dir(path), extensions, gCfg.ChunkSize, gCfg.NumWorkers),
		Indexer:       indexer,

		// channels
		workChan: make(chan WorkItem, gCfg.ChannelBufferSize*2),
		fileChan: make(chan string, gCfg.ChannelBufferSize*4),
		docChan:  make(chan *models.Document, gCfg.ChannelBufferSize),

		ctx:        ctx,
		cancel:     cancel,
		UpdateChan: make(chan string, 2),
	}

	coord.watcher = watcher.NewFileWatcher(
		func(path string) { coord.workChan <- WorkItem{Type: "delete", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "update", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "create", FilePath: path} },
	)

	// Start persistent workers
	coord.startWorkers()
	coord.watcher.StartWatching()

	return coord
}

func (c *Coordinator) startWorkers() {
	// Single work dispatcher
	c.wg.Add(1)
	go c.workDispatcher()

	for i := 0; i < gCfg.NumWorkers/2; i++ {
		c.wg.Add(2)
		// file processor pool
		go c.fileProcess()
		// Document indexer
		go c.documentIndexer()
	}
	c.wg.Add(1)
	go c.AddDir()
}

func (c *Coordinator) workDispatcher() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case work := <-c.workChan:
			switch work.Type {
			case "delete":
				// Delete single file
				// TODO :
				// Trade off between:
				// --> Delete in batchs in case of multiple deletes come
				// less time but timer will be created and call flush every t seconds (in case of limit of flush unreached)
				// --> Delete in single files as delete event is not frequent in our main program purpose
				c.Indexer.DeleteSingleDocument(work.FilePath)
			case "create", "update":
				c.fileChan <- work.FilePath
			}
		}
	}
}

func (c *Coordinator) fileProcess() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case filePath := <-c.fileChan:

			file, err := os.Open(filePath)
			if err != nil {
				print(err)
				continue
			}
			info, _ := file.Stat()
			file.Close()
			// It is a folder --> Walk and give me the files
			if info.IsDir() {
				c.fileprocessor.Walk(filePath, c.fileChan, c.UpdateChan)
				continue
			}
			// It is file then read its content
			// send on docChan to start indexing
			// println("BEFORE READING", info.Name())
			c.fileprocessor.Read(filePath, info, c.docChan, &c.pendingWork)
		}
	}
}

func (c *Coordinator) SetOnComplete(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onComplete = callback
}

// Update the completion trigger
func (c *Coordinator) triggerComplete() {
	c.mu.RLock()
	callback := c.onComplete
	c.mu.RUnlock()

	if callback != nil {
		callback()
	}
}

func (c *Coordinator) documentIndexer() {
	defer c.wg.Done()
	batch := c.Indexer.NewBatch()
	var batchSize int32
	var batchCount int32
	flushBatch := c.Indexer.FlushBatch

	// Add a ticker to periodically check for completion
	// Think of better way to avoid polling
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			// Process remaining batch
			if batchCount > 0 {
				flushBatch(batch, &batchSize, &batchCount)
			}
			atomic.StoreInt32(&c.pendingWork, 0)
			c.triggerComplete()
			return
		case doc := <-c.docChan:
			// println("INDEXER: ", doc.Path)
			if err := c.Indexer.IndexDocument(batch, doc); err != nil {
				atomic.AddInt32(&c.pendingWork, -1)
				fmt.Printf("Error adding document to batch: %v\n", err)
				continue
			}
			// println("NORMAL: ", atomic.LoadInt32(&c.pendingWork))

			atomic.AddInt32(&batchSize, int32(len(doc.Content)))
			atomic.AddInt32(&batchCount, 1)

			// Check if batch should be flushed
			if atomic.LoadInt32(&batchSize) >= atomic.LoadInt32(&gCfg.IndexBatchMemoryLimit) {
				// println("Before Batch: ", atomic.LoadInt32(&c.pendingWork))
				atomic.AddInt32(&c.pendingWork, -batchCount)
				flushBatch(batch, &batchSize, &batchCount)
				// println("After Batch: ", atomic.LoadInt32(&c.pendingWork))
				batch = c.Indexer.NewBatch()
			}
		case <-ticker.C:
			// Periodically check for completion and flush small batches
			if batchCount > 0 {
				// println("Before Batch: ", atomic.LoadInt32(&c.pendingWork))
				atomic.AddInt32(&c.pendingWork, -batchCount)
				flushBatch(batch, &batchSize, &batchCount)
				// println("After Batch: ", atomic.LoadInt32(&c.pendingWork))
				batch = c.Indexer.NewBatch()
			}
			// ticker.Reset(10 * time.Second)
			// println(atomic.LoadInt32(&c.pendingWork))
			// Check if all work is done
			if atomic.LoadInt32(&c.pendingWork) == 0 {
				c.triggerComplete()
			}
		}
	}
}

func (c *Coordinator) IntialScan(filePath string) {
	atomic.StoreInt32(&c.pendingWork, 0)
	c.workChan <- WorkItem{Type: "create", FilePath: filePath}
}
func (c *Coordinator) AddDir() {
	for folder := range c.UpdateChan {
		c.watcher.Watcher.Add(folder)
	}
}

// Close the coordinator and
// realese resources
func (c *Coordinator) Shutdown() {
	c.cancel()
	close(c.workChan)
	close(c.fileChan)
	close(c.docChan)
	close(c.UpdateChan)
	c.wg.Wait()
}
