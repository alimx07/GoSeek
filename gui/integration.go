package gui

import (
	"GoSeek/config"
	"GoSeek/internal/coordinator"
	"GoSeek/internal/indexer"
	"GoSeek/internal/models"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

type Folder struct {
	Name     string
	Children map[string]*Folder
}

var root = &Folder{
	Name:     "",
	Children: make(map[string]*Folder),
}

var FolderIndex = make(map[string]*coordinator.Coordinator)

// CreateTree creates prefix tree (trie) from all paths in index
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
		if _, ok := vis[dir]; !ok { // prevent duplicates
			c.UpdateChan <- prePath + string(filepath.Separator) + dir
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

// Get All prevIndexes on the fly when app reopen
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

func insertToTree(root *Folder, path string) {
	if path == "" {
		return
	}
	// println(path)
	parts := strings.Split(path, string(filepath.Separator))
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

// Search in all indexes found with specific query
func SearchDocuments(queryString *query.QueryStringQuery, folders []string) ([]models.Document, error) {
	var results []models.Document
	indexGroups := groupFolders(folders)
	for coord, dirs := range indexGroups {
		dirQuery := CreateDirQuery(dirs)
		query := bleve.NewConjunctionQuery(queryString, dirQuery)
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Size = 1000 // Limit results count for now (rare to be more than that) handle later
		searchRequest.Fields = []string{"path", "score", "size", "mod_time", "extension"}
		res, err := coord.Indexer.Search(searchRequest)
		if err != nil {
			return nil, err
		}
		results = append(results, res...)
	}
	// for _, res := range results {
	// 	println(res.Dir)
	// }
	return results, nil
}

// Group Folders according to coord (Index)
func groupFolders(folders []string) map[*coordinator.Coordinator][]string {
	groups := make(map[*coordinator.Coordinator][]string)
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

func CreateDirQuery(dirs []string) *query.DisjunctionQuery {
	queries := make([]query.Query, 0, len(dirs))
	for _, dir := range dirs {
		q := bleve.NewTermQuery(dir)
		q.SetField("dir")
		queries = append(queries, q)
	}
	dirQuery := bleve.NewDisjunctionQuery(queries...)
	return dirQuery
}

func CreateStringQuery(queryString string) (*query.QueryStringQuery, error) {
	StringQuery := bleve.NewQueryStringQuery(queryString)
	// println(keywordQuery.Query)
	err := StringQuery.Validate()
	if err != nil {
		return nil, err
	}
	return StringQuery, nil
}

func GetSearchTerms(queryString *query.QueryStringQuery) []string {
	parseQuery, _ := queryString.Parse()
	searchTerms := walkQuery(parseQuery)
	return searchTerms
}
func walkQuery(q query.Query) []string {
	switch t := q.(type) {
	case *query.TermQuery:
		// println("T-->", t.Term)
		return []string{t.Term}
	case *query.MatchQuery:
		// println("M-->", t.Match)
		return []string{t.Match}
	case *query.BooleanQuery:
		var out []string
		if t.Must != nil {
			if mustSlice, ok := (t.Must).(*query.ConjunctionQuery); ok {
				for _, must := range mustSlice.Conjuncts {
					out = append(out, walkQuery(must)...)
				}
			}
		}
		if t.Should != nil {
			if shouldSlice, ok := (t.Should).(*query.DisjunctionQuery); ok {
				for _, should := range shouldSlice.Disjuncts {
					// println("S")
					out = append(out, walkQuery(should)...)
				}
			}
			if t.MustNot != nil {
				if mustNotSlice, ok := (t.MustNot).(*query.DisjunctionQuery); ok {
					for _, mustNot := range mustNotSlice.Disjuncts {
						out = append(out, walkQuery(mustNot)...)
					}
				}
			}
		}
		return out
	case *query.ConjunctionQuery:
		var out []string
		for _, sub := range t.Conjuncts {
			out = append(out, walkQuery(sub)...)
		}
		return out
	case *query.DisjunctionQuery:
		var out []string
		for _, sub := range t.Disjuncts {
			out = append(out, walkQuery(sub)...)
		}
		return out
	case *query.RegexpQuery:
		// println("R--->", t.Regexp)
		return []string{"/" + t.Regexp + "/"}
	}
	return nil
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

func BuildRegexPattern(terms []string) (*regexp.Regexp, error) {
	patterns := []string{}
	for _, term := range terms {
		if isRegexInput(term) {
			patterns = append(patterns, `(?i)`+term[1:len(term)-1]+`\b`)
		} else {
			escaped := regexp.QuoteMeta(term)
			patterns = append(patterns, `(?i)\b`+escaped+`\b`)
		}
	}

	fullPattern := strings.Join(patterns, "|")
	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return nil, err
	}
	return re, nil
}

// Open File in Content Preview section with
func GetDocumentPreview(path string, re *regexp.Regexp, updatafn func([]widget.RichTextSegment)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	segments := []widget.RichTextSegment{}
	linecount := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lastIndex := 0
			locs := re.FindAllStringIndex(line, -1)
			for _, loc := range locs {
				start, end := loc[0], loc[1]
				// println(start, "  ", end)
				if start > lastIndex {
					seg := &widget.TextSegment{
						Text: line[lastIndex:start],
					}
					// seg.Style.Alignment = true
					seg.Style.Inline = true
					segments = append(segments, seg)
				}

				match := &widget.TextSegment{
					Text:  line[start:end] + " ",
					Style: widget.RichTextStyleBlockquote,
				}

				match.Style.Inline = true
				match.Style.ColorName = theme.ColorNameWarning
				segments = append(segments, match)
				lastIndex = end
			}

			if lastIndex < len(line) {
				segments = append(segments, &widget.TextSegment{
					Text: line[lastIndex:],
				})
			}

		}
		linecount++
		if linecount%200 == 0 {
			updatafn(segments)
			segments = []widget.RichTextSegment{}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil
		}
	}
	updatafn(segments)
	return nil
}

func isRegexInput(input string) bool {
	return len(input) >= 2 && input[0] == '/' && input[len(input)-1] == '/'
}
