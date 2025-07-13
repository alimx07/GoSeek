package main

import (
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"GoSeek/internal/walker"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

// JUST SOME DUMMY APP ENTRY POINT FOR NOW

func main() {

	// For profiling and debug
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	// debug.SetMemoryLimit(1024 * 1024 * 1024)
	// debug.SetGCPercent(50)

	searchWord := flag.String("word", "", "Word to search for (required)")
	root := flag.String("path", ".", "Root directory to search")
	flag.Parse()

	// The responsble chan for traversing docs between Walker and Indexer instances
	// TODO : Try to tune the capacity of channel
	docChan := make(chan *models.Document, 4)

	walker := walker.NewWalker(*root, []string{".txt", ".json", ".log"})
	indexer, err := indexer.NewBleveIndexer("index")
	if err != nil {
		log.Fatalf("Error creating indexer: %v", err)
	}

	var wg sync.WaitGroup
	start := time.Now()
	go walker.Walk(docChan)
	wg.Add(1)
	go func() {
		indexer.Index(docChan)
		println(indexer.Stats().String())
		wg.Done()
	}()
	wg.Wait()
	end := time.Now()
	println("Time Taken For Traversal and Indexing: ", end.Sub(start).String())
	indexer.Search(*searchWord)
}
