package fileprocessor

import (
	"GoSeek/internal/models"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileProcessor struct {
	allowedExtensions map[string]bool
	bufferPool        sync.Pool
	builderPool       sync.Pool
}

// NewWalker returns a pointer to Walker Instance standing
// on current root path and intersted in specific files with extensions in allowedExtensions slice

func NewFileProcessor(allowedExtensions map[string]bool, chunksize, numWorkers int) *FileProcessor {
	fp := &FileProcessor{
		allowedExtensions: allowedExtensions,
	}
	fp.builderPool = sync.Pool{
		New: func() interface{} {
			return new(strings.Builder)
		},
	}
	fp.bufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, chunksize)
			return &buf
		},
	}
	return fp
}

func (fp *FileProcessor) getBuffer() *[]byte {
	return fp.bufferPool.Get().(*[]byte)
}

func (fp *FileProcessor) putBuffer(buf *[]byte) {
	fp.bufferPool.Put(buf)
}

func (fp *FileProcessor) getBuilder() *strings.Builder {
	return fp.builderPool.Get().(*strings.Builder)
}

func (fp *FileProcessor) putBuilder(b *strings.Builder) {
	b.Reset()
	fp.builderPool.Put(b)
}

// Walk is the main method of the walker instance
// It starts to traverse the system using filepath.WalkDir func
// It is also the producer func to Index consumer

// TODO:
// Try using fastwalk module (It is stated as being much faster than filepath.WalkDir)

func (fp *FileProcessor) Walk(filePath string, fileChan chan<- string, updateChan chan string) {
	// go StreamToIndex(w.chunkSize, w.numWorkers, fileChan, docChan)
	err := filepath.WalkDir(filePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// fmt.Println("Error opening file/folder at: ", err)
			return nil
		}
		// println("Walker", "     ", path)
		if d.IsDir() {
			updateChan <- path
			return nil
		}
		ext := filepath.Ext(path)
		if _, ok := fp.allowedExtensions[ext]; !ok {
			return nil
		}
		fileChan <- path
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
	}
}

func (fp *FileProcessor) Read(filePath string, info os.FileInfo, docChan chan<- *models.Document) error {

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error in Opening file %v", err)
		return nil
	}
	// fmt.Println("Reader: ", file.Name())
	content := fp.getBuilder()

	// fileHash := xxhash.New()
	buffer := *fp.getBuffer()
	// println("Reader    ", filePath)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			print(err)
		}
		if err == io.EOF {
			break
		}
		// fileHash.Write(buffer[:n])
		content.Write(buffer[:n])
	}
	// currHash := fileHash.Sum64()

	ext := filepath.Ext(filePath)
	modtime := info.ModTime().Format(time.RFC1123)
	size := info.Size()
	docChan <- models.NewDocument(filePath, size, modtime, ext, content.String())
	fp.putBuffer(&buffer)
	fp.putBuilder(content)
	file.Close() // DONE
	return nil
}
