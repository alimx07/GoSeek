package main

import (
	"GoSeek/config"
	"GoSeek/internal/watcher"
	"os"
)

type WorkItem struct {
	Type     string // "create", "delete", "update"
	FilePath string
}

type IndexWatcher struct {
	config   *config.IndexConfig
	watcher  *watcher.FileWatcher
	File     *os.File
	workChan chan WorkItem
}

type SnakeConfig struct {
	config   config.Config
	Watchers []*IndexWatcher
}

func (s *SnakeConfig) LoadConfig(filePath string) error {
	// open file to get data
	err := config.ReadYAMLToConfig(&s.config, filePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *SnakeConfig) StartWatchers() {
	for _, index := range s.config.Indexes {
		i, err := NewIndexWatcher(index)
		if err != nil {
			continue
		}
		s.Watchers = append(s.Watchers, i) // save pointers to watchers and terminate them when app is ON or signal
	}
}

func (s *SnakeConfig) ShutDown() {
	for _, watcher := range s.Watchers {
		watcher.ShutDown()
	}
}

func NewIndexWatcher(c *config.IndexConfig) (*IndexWatcher, error) {
	i := IndexWatcher{
		config:   c,
		workChan: make(chan WorkItem),
	}
	i.watcher = watcher.NewFileWatcher(
		func(path string) { i.workChan <- WorkItem{Type: "delete", FilePath: path} },
		func(path string) { i.workChan <- WorkItem{Type: "update", FilePath: path} },
		func(path string) { i.workChan <- WorkItem{Type: "create", FilePath: path} },
	)
	var err error
	i.File, err = FileStart(i.config.PendingChangesPath)
	if err != nil {
		return nil, err
	}
	i.StartWatcher()
	for _, folder := range c.Folders {
		i.watcher.Watcher.Add(folder) // watch the files
	}
	return &i, nil
}

func (i *IndexWatcher) StartWatcher() {
	i.watcher.StartWatching()
	go i.WriteData(i.File)
}

func FileStart(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (i *IndexWatcher) WriteData(file *os.File) {
	for work := range i.workChan {
		// syscall every write
		// acceptable no much writes
		// buffer using add more complexity of handle crash without flushing
		file.WriteString(work.Type + ":" + work.FilePath + "\n")
	}
}

func (i *IndexWatcher) ShutDown() {
	i.File.Close()
	close(i.workChan)
}

// yaml file structure

// indexes:
//   - name: documents
//     folders:
//        - /home/you/Documents
//     index_path: /home/you/.docfetcher-go/indexes/documents
//     pending_changes_path: /home/you/.docfetcher-go/pending-changes/documents.json
//    extensions:
//   	.pdf: true
//   	.txt: true
//   	.docx: false

// log_dir: goseek/logs/
