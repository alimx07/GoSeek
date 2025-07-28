package gui

import (
	"GoSeek/config"
	"GoSeek/internal/coordinator"
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

type Folder struct {
	Name     string
	Children map[string]*Folder
}

// var gCfg = config.LoadGlobalConfig()
// var Cfg = config.Config{}
var root = &Folder{
	Name:     "",
	Children: make(map[string]*Folder),
}

var FolderIndex = make(map[string]*coordinator.Coordinator)

// CreateTree create prefix tree (trie) from all paths in index
func CreateTreeFromIndex(root *Folder, prePath string, res *bleve.SearchResult, c *coordinator.Coordinator) *Folder {
	if root == nil || res == nil {
		return root
	}

	vis := make(map[string]bool)
	for _, hit := range res.Hits {
		pathField := hit.ID

		dir := filepath.Dir(pathField)
		if dir == "." {
			continue
		}
		// println("OUT: ", dir)
		if _, ok := vis[dir]; !ok {
			c.UpdateChan <- prePath + "/" + dir
			vis[dir] = true
			insertToTree(root, dir)
		}
	}
	return root
}

// Get All Paths in specific index
func GetPaths(index *indexer.BleveIndexer) *bleve.SearchResult {
	if index == nil || index.Index == nil {
		return nil
	}

	query := bleve.NewMatchAllQuery()
	req := bleve.NewSearchRequest(query)
	req.Fields = []string{"path"}
	req.Size = 10000

	searchResults, err := index.Index.Search(req)
	if err != nil {
		fmt.Printf("Error searching index: %v\n", err)
		return nil
	}
	return searchResults
}

// Get All prevIndexes at the fly when app reopen
func GetIndexes() *Folder {
	data, err := config.ReadFromFile()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
		return root
	}
	indexes := strings.Split(data, "\n")
	for _, path := range indexes {
		trimmedPath := strings.TrimSpace(path)
		if trimmedPath == "" {
			continue
		}

		c := coordinator.NewCoordinatorPrevIndex(trimmedPath)
		FolderIndex[filepath.Base(path)] = c
		if c == nil || c.Indexer == nil {
			continue // Skip if coordinator creation failed
		}
		paths := GetPaths(c.Indexer)
		if paths != nil && len(paths.Hits) > 0 {
			// println(len(paths.Hits))
			CreateTreeFromIndex(root, filepath.Dir(trimmedPath), paths, c)
		}
	}
	return root
}

// TODO:
// more dynamic way for handling paths
// Fix: Add paths inside index according to OS seperator
func insertToTree(root *Folder, path string) {
	if path == "" {
		return
	}
	// println(path)
	// Use proper path separator for Windows
	parts := strings.Split(path, "\\")
	node := root

	for _, part := range parts {
		if part == "" {
			continue
		}
		if node.Children == nil {
			node.Children = make(map[string]*Folder)
		}
		if _, exists := node.Children[part]; !exists {
			node.Children[part] = &Folder{
				Name:     part,
				Children: make(map[string]*Folder),
			}
		}
		node = node.Children[part]
	}
}

// Search in indexes
func SearchDocuments(queryString string, folders []string) []models.Document {
	var results []models.Document
	indexGroups := groupFolders(folders)
	// println(queryString, " ", len(indexGroups))
	for coord, dirs := range indexGroups {
		// println(dirs, coord)
		query := CreateQuery(queryString, dirs)
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Size = 1000 // Limit results count for now (rare to be more than that) handle later
		searchRequest.Fields = []string{"path", "score", "size", "mod_time", "extension"}
		res, err := coord.Indexer.Search(searchRequest)
		// println(len(res))
		if err != nil {
			println(err)
		}
		results = append(results, res...)
	}
	return results
}

// Group Folders according to coord (Index)
func groupFolders(folders []string) map[*coordinator.Coordinator][]string {
	groups := make(map[*coordinator.Coordinator][]string)
	// println(folders)
	// for coord, folder := range FolderIndex {
	// 	println(coord, " ", folder)
	// }
	for _, folder := range folders {
		for parentFolder, coord := range FolderIndex {
			// println(parentFolder, " ", folder, " ", coord)
			if strings.HasPrefix(folder, parentFolder) {
				groups[coord] = append(groups[coord], folder)
			}
		}
	}
	return groups
}

// Create Query --> search in these dirs and get files with queryString word
func CreateQuery(queryString string, dirs []string) *query.ConjunctionQuery {
	queries := make([]query.Query, 0, len(dirs))
	for _, dir := range dirs {
		q := bleve.NewTermQuery(dir)
		q.SetField("dir")
		queries = append(queries, q)
	}
	dirQuery := bleve.NewDisjunctionQuery(queries...)
	keywordQuery := bleve.NewMatchQuery(queryString)
	keywordQuery.SetField("content")
	finalQuery := bleve.NewConjunctionQuery(dirQuery, keywordQuery)

	return finalQuery
}

// Create New index
func IndexFolder(path string) (*treeContext, error) {
	config.SaveToFile(path)
	// use some defined extensions for now
	extensions := map[string]bool{
		".txt": true,
		".log": true,
		".md":  true,
		".go":  true,
		".py":  true,
	}
	coord := coordinator.NewCoordinator(path, extensions)
	FolderIndex[filepath.Base(path)] = coord
	done := make(chan struct{})

	coord.SetOnComplete(func() {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	// Start the initial scan
	coord.IntialScan(path)

	<-done // what until Done
	// TODO :
	// fix : The function may end before last batch indexes
	paths := GetPaths(coord.Indexer)
	CreateTreeFromIndex(root, path, paths, coord)
	return &treeContext{
		root:      root,
		treeCache: make(map[string]*Folder),
	}, nil
}

func RemoveFolder(path string) (*treeContext, error) {
	// Mock removal function
	fmt.Printf("Removing folder from index: %s\n", path)
	return nil, nil
}

// Open File in Content Preview section
func GetDocumentPreview(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	// TODO:
	// Add Streaming the content according to page num in preview
	buf := make([]byte, 256*1024) // Just read files for now
	n, err := file.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}
