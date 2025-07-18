package watcher

import (
	"GoSeek/internal/indexer"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	Watcher    *fsnotify.Watcher
	Indexer    *indexer.BleveIndexer
	deleteChan chan string
	writeChan  chan string
	createChan chan string
	onDelete   func(string)
	onWrite    func(string)
	onCreate   func(string)
}

func NewFileWatcher(onDelete func(string), OnWrite func(string), onCreate func(string)) *FileWatcher {
	return &FileWatcher{
		deleteChan: make(chan string, 4),
		writeChan:  make(chan string, 4),
		createChan: make(chan string, 4),
		onDelete:   onDelete,
		onWrite:    OnWrite,
		onCreate:   onCreate,
	}
}

func (fw *FileWatcher) StartWatching() error {
	var err error
	fw.Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	go func() {
		idleTime := time.NewTimer(10 * time.Second)
		eventsQueue := make([]fsnotify.Event, 0)
		for {
			select {
			case event := <-fw.Watcher.Events:
				eventsQueue = append(eventsQueue, event)
				idleTime.Reset(10 * time.Second)
			case err := <-fw.Watcher.Errors:
				fmt.Println(err)
			case <-idleTime.C:
				for _, event := range eventsQueue {
					switch event.Op {
					case fsnotify.Remove:
						fw.onDelete(event.Name)
					case fsnotify.Create:
						fw.onCreate(event.Name)
					case fsnotify.Write:
						fw.onWrite(event.Name)
						// case fsnotify.Rename:
						// 	fw.renameChan <- event.Name
					}
				}
				eventsQueue = eventsQueue[:0]
				idleTime.Reset(10 * time.Second)
			}
		}
	}()
	return nil
}
