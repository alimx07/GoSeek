package reader

import (
	"GoSeek/internal/models"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

var Mp = make(map[string]map[int64]string)

// I still need to save that in presistence storage
var FileHash = make(map[string]uint64) // Save hashs of found files in index
var mu sync.Mutex

func StreamToChannel(chunkSize int, fileChan <-chan string, docChan chan<- *models.Document) error {
	var wg sync.WaitGroup
	numWorkers := 4
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int, buffer []byte, content *strings.Builder) {
			defer wg.Done()
			for filePath := range fileChan {
				file, err := os.Open(filePath)
				if err != nil {
					fmt.Printf("Error in Opening file %v , Worker %v", err, workerID)
					continue
				}
				fmt.Println("Reader: ", file.Name())
				info, err := file.Stat()
				if err != nil {
					fmt.Println(filePath)
					continue
				}
				fileHash := xxhash.New()
				for {
					n, err := file.Read(buffer)
					if err != nil && err != io.EOF {
						print(err)
					}
					if err == io.EOF {
						break
					}
					fileHash.Write(buffer[:n])
					content.Write(buffer[:n])
				}
				currHash := fileHash.Sum64()
				mu.Lock()
				// If the Hash is the same
				// Negative Write (Just AutoSave and the content is the same)
				if hash, ok := FileHash[filePath]; ok && hash == currHash {
					mu.Unlock()
					continue
				}
				FileHash[filePath] = currHash
				mu.Unlock()
				ext := filepath.Ext(filePath)
				modtime := info.ModTime().Format(time.RFC1123)
				size := uint64(info.Size())
				docChan <- models.NewDocument(filePath, size, modtime, ext, content.String())
				content.Reset()
				file.Close()
			}
		}(i, make([]byte, chunkSize), &strings.Builder{})
	}
	wg.Wait()
	return nil
}
