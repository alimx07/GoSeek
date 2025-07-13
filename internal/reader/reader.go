package reader

import (
	"GoSeek/internal/models"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode"
)

// TODO
// Handle --> IF the file Do not end by space (the normal)
// It creates a new chunk just to last word
// Two Syscalls per chunk (Read , Seek)
// ---> Investigate another approach (May be use []byte to store left bytes from prev read)
func ReadChunk(file *os.File, buffer []byte) ([]byte, error) {
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n == 0 {
		return nil, io.EOF
	}
	if err == io.EOF {
		return buffer[:n], nil
	}
	splitPoint := findSplitPoint(buffer[:n])

	// if splitPoint == -1 May be it is just huge text word
	// Not Likely but if it happens just discard it for now
	if splitPoint != -1 {
		file.Seek(int64(splitPoint-n), io.SeekCurrent)
		return buffer[:splitPoint], nil
	}
	return buffer[:n], nil
}

// TODO
// Use Sync.Pool buffer (Better memory utilization)
func StreamToChannel(chunkSize int, fileChan <-chan string, docChan chan<- *models.Document) error {
	var wg sync.WaitGroup
	numWorkers := 4
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int, buffer []byte) {
			defer wg.Done()
			for filePath := range fileChan {
				file, err := os.Open(filePath)
				if err != nil {
					fmt.Printf("Error in Opening file %v , Worker %v", err, workerID)
					continue
				}
				info, err := file.Stat()
				if err != nil {
					fmt.Println(filePath)
					continue
				}
				ext := filepath.Ext(filePath)
				modtime := info.ModTime().Format(time.RFC1123)
				size := uint64(info.Size())
				for {
					content, err := ReadChunk(file, buffer)
					if err != nil {
						if err != io.EOF {
							fmt.Println(filePath)
						}
						break
					}
					docChan <- models.NewDocument(
						filePath,
						size,
						modtime,
						ext,
						string(content))
				}
				file.Close()
			}
		}(i, make([]byte, chunkSize))
	}
	wg.Wait()
	return nil
}

func findSplitPoint(buffer []byte) int {
	maxPos := len(buffer) - 1
	for i := maxPos; i > 0; i-- {
		if unicode.IsSpace(rune(buffer[i])) || unicode.IsPunct(rune(buffer[i])) {
			return i + 1
		}
	}
	return -1
}
