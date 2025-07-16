package watcher

import (
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"GoSeek/internal/walker"
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	Watcher    *fsnotify.Watcher
	Indexer    *indexer.BleveIndexer
	FolderPath string
	renameMap  map[string]time.Time
	deleteChan chan string
	writeChan  chan string
	createChan chan string
	renameChan chan string
}

func NewFileWatcher(indexer *indexer.BleveIndexer, folderpath string) *FileWatcher {
	return &FileWatcher{
		Indexer:    indexer,
		FolderPath: folderpath,
		renameMap:  make(map[string]time.Time),
		deleteChan: make(chan string, 1),
		createChan: make(chan string, 1),
		writeChan:  make(chan string, 1),
		renameChan: make(chan string, 1),
	}
}

func (fw *FileWatcher) StartWatching() {
	var err error
	fw.Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating watcher for", fw.FolderPath)
		return
	}
	go fw.createProcess()
	go fw.deleteProcess()
	go fw.writeProcess()
	go fw.renameProcess()
	go func() {
		idleTime := time.NewTimer(10 * time.Second)
		eventsQueue := make([]fsnotify.Event, 0)
		for {
			select {
			case event := <-fw.Watcher.Events:
				eventsQueue = append(eventsQueue, event)
				fmt.Println(event.Name, "    ", event.Op)
				idleTime.Reset(10 * time.Second)
			case err := <-fw.Watcher.Errors:
				fmt.Println(err)
			case <-idleTime.C:
				for _, event := range eventsQueue {
					// what if when the indexing runnig
					// some event happens ??
					switch event.Op {
					case fsnotify.Remove:
						fw.deleteChan <- event.Name
					// ON Rename --> Rename evetn with old path Then Create event with new path
					// Handle --> Old path still in the db
					// Multiple Writes / Saves triger multiple events in same time
					case fsnotify.Create:
						if fw.isRename(event.Name) {
							fw.renameChan <- event.Name
						} else {
							fw.createChan <- event.Name
						}
					case fsnotify.Rename:
						fw.renameMap[event.Name] = time.Now()
					case fsnotify.Write:
						fw.writeChan <- event.Name
					}
				}
				eventsQueue = make([]fsnotify.Event, 0)
				idleTime.Reset(10 * time.Second)
			}
		}
	}()
}

// Just Delete from the index

// TODO :
// Batching Delete for multiple delete events at once (Rare event)
func (fw *FileWatcher) deleteProcess() {
	for filePath := range fw.deleteChan {
		fw.Indexer.Index_m.Delete(filePath)
	}
}

// Handling create event
// It triggered when new item moved or created in Watched file
// the Item can be folder / file
// So Think of it as a new Folder indexing start process
// As the Normal Flow --> Walker-->Reader-->Indexing(In already found bleve.index)
func (fw *FileWatcher) createProcess() {
	for filePath := range fw.createChan {
		// Allowed Extensions will be global config later
		walker := walker.NewWalker(filePath, []string{".txt", ".json", ".log"})
		tempChan := make(chan *models.Document, 8)
		go func() {
			dirs := walker.Walk(tempChan)
			fw.AddDirs(dirs)
		}()
		go fw.Indexer.Index(tempChan)
	}
}

// Handling write event
// The Simples way
// GO with normal flow ---> Walker --> Reader --> indexing
// we should begin from the Reader section but
// starting from Walker just keep the flow defined and no time consuming at all
func (fw *FileWatcher) writeProcess() {
	// All Errors will be handled later
	for filePath := range fw.writeChan {
		file, err := os.Open(filePath)
		if err != nil {
			return
		}
		info, err := file.Stat()
		if err != nil {
			return
		}
		if info.IsDir() {
			return
		}
		walker := walker.NewWalker(filePath, []string{".txt", ".json", ".log"})
		tempChan := make(chan *models.Document, 8)
		go walker.Walk(tempChan)
		go fw.Indexer.Index(tempChan)
	}
}

// Add all dirs for being watching
// The fsnotify not watch subdirectories by default
// So Add all Folders paths to Watcher (parent + sub dirs)
func (fw *FileWatcher) AddDirs(subDirs []string) {
	// fmt.Println(subDirs)
	for _, dir := range subDirs {
		fw.Watcher.Add(dir)
	}
}

// Handle Rename Event
// Need logic
func (fw *FileWatcher) renameProcess() {
	for filePath := range fw.renameChan {
		// logic
		print(filePath)
	}
}

// Think of way to handle that

// When file/Folder renamed the system send two events
// Rename with oldPath
// Create with newPath
// Find a way to track of the create event has prev rename --> renameProcess
// Just treat all as create process will be inefficient
// as imagine of the parent Dir is renamed (Now we will index everything again)
func (fw *FileWatcher) isRename(filePath string) bool {
	// prevTime, ok := fw.renameMap[filePath]
	return false
}
