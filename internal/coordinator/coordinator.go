package coordinator

import (
	"GoSeek/config"
	"GoSeek/internal/fileprocessor"
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"GoSeek/internal/watcher"
	"context"
	"fmt"
	"os"
	"sync"
)

type WorkItem struct {
	Type     string // "create", "delete", "update"
	FilePath string
	Data     interface{}
}

type Coordinator struct {
	fileprocessor *fileprocessor.FileProcessor
	watcher       *watcher.FileWatcher
	Indexer       *indexer.BleveIndexer
	cfg           *config.IndexConfig

	// Persistent channels
	workChan chan WorkItem
	fileChan chan string
	docChan  chan *models.Document

	// Worker control
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	updateChan chan string
	saveUpdate chan struct{}
}

var gCfg *config.GlobalConfig = config.LoadGlobalConfig()

func NewCoordinator(cfg *config.IndexConfig, saveUpdate chan struct{}) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())

	coord := &Coordinator{
		fileprocessor: fileprocessor.NewFileProcessor(cfg.Extensions, gCfg.ChunkSize, gCfg.NumWorkers),
		Indexer:       indexer.NewBleveIndexer(cfg.IndexPath, gCfg.IndexBatchMemoryLimit, gCfg.NumWorkers),
		cfg:           cfg,

		// channels
		workChan: make(chan WorkItem, gCfg.ChannelBufferSize*2),
		fileChan: make(chan string, gCfg.ChannelBufferSize*4),
		docChan:  make(chan *models.Document, gCfg.ChannelBufferSize),

		ctx:        ctx,
		cancel:     cancel,
		updateChan: make(chan string, 2),
		saveUpdate: saveUpdate,
	}

	coord.watcher = watcher.NewFileWatcher(
		func(path string) { coord.workChan <- WorkItem{Type: "delete", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "update", FilePath: path} },
		func(path string) { coord.workChan <- WorkItem{Type: "create", FilePath: path} },
	)

	// Start persistent workers
	coord.startWorkers()

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
				go func() {
					c.Indexer.Index_m.Delete(work.FilePath)
				}()
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
			// It is a folder --> Walk and give me the files
			if info.IsDir() {
				c.fileprocessor.Walk(filePath, c.fileChan, c.updateChan)
				continue
			}
			// It is file then read its content
			// send on docChan to start indexing
			c.fileprocessor.Read(filePath, info, c.docChan)
		}
	}
}

func (c *Coordinator) documentIndexer() {
	defer c.wg.Done()
	batch := c.Indexer.NewBatch()
	var batchSize int
	var batchCount int
	// idleTime := time.NewTimer(10 * time.Second)
	flushBatch := c.Indexer.FlushBatch
	for {
		select {
		case <-c.ctx.Done():
			// Process remaining batch
			flushBatch(batch, &batchSize, &batchCount)
			return
		case doc := <-c.docChan:
			if err := c.Indexer.IndexDocument(batch, doc); err != nil {
				fmt.Printf("Error adding document to batch: %v\n", err)
				continue
			}

			batchSize += len(doc.Content)
			batchCount++

			// Check if batch should be flushed
			if batchSize >= gCfg.IndexBatchMemoryLimit {
				flushBatch(batch, &batchSize, &batchCount)
				batch = c.Indexer.NewBatch()
			}
		}
	}
}

// An IntialScan is just send a create event
// with root path
func (c *Coordinator) IntialScan(filePath string) {
	c.workChan <- WorkItem{Type: "create", FilePath: filePath}
}

// Start the watcher to watch filesystem
func (c *Coordinator) StartWatching() error {
	return c.watcher.StartWatching()
}

func (c *Coordinator) AddDir() {
	for folder := range c.updateChan {
		c.watcher.Watcher.Add(folder)
		c.cfg.Folders = append(c.cfg.Folders, folder)
		c.saveUpdate <- struct{}{}
	}
}

// Close the coordinator and
// realese resources
func (c *Coordinator) Shutdown() {
	c.cancel()
	close(c.workChan)
	close(c.fileChan)
	close(c.docChan)
	close(c.updateChan)
	c.wg.Wait()
}
