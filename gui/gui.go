package gui

import (
	"GoSeek/internal/models"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// UI constants
const (
	DefaultWindowWidth  = 600
	DefaultWindowHeight = 400
	TableColumnWidth0   = 300  // File Name
	TableColumnWidth1   = 100  // Score
	TableColumnWidth2   = 100  // Size
	TableColumnWidth3   = 100  // ext
	TableColumnWidth4   = 500  // File Path
	TableColumnWidth5   = 200  // ModTime
	LeftPanelOffset     = 0.25 // 25% for left panel
	ResultsPreviewSplit = 0.5  // 50% for results, 50% for preview
)

// Folder

// GUI Application
type GUI struct {
	app             fyne.App
	window          fyne.Window
	searchEntry     *widget.Entry
	sizeFilter      *widget.Select
	resultsTable    *widget.Table
	previewText     *widget.RichText
	folderTree      *widget.Tree
	tree            *treeContext
	searchResults   []models.Document
	searchTerms     []string
	excludedFolders map[string]bool
	isDarkTheme     bool
}

type treeContext struct {
	root      *Folder
	treeCache map[string]*Folder // cache UID --> Folder
}

func NewApp() *GUI {
	app := app.NewWithID("GoSeek")
	app.SetIcon(theme.FolderIcon())

	window := app.NewWindow("GoSeek - Document Search")
	window.Resize(fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight))

	gui := &GUI{
		app:         app,
		window:      window,
		isDarkTheme: false,
	}

	// Create main menu with proper action connections
	window.SetMainMenu(gui.createMainMenu())

	gui.setupUI()
	return gui
}

// Testing Menu
// TODO : Add More func if possible

func (g *GUI) createMainMenu() *fyne.MainMenu {
	// File menu
	newItem := fyne.NewMenuItem("New Index", func() {
		g.createNewIndex()
	})
	quitItem := fyne.NewMenuItem("Quit", func() {
		g.app.Quit()
	})
	fileMenu := fyne.NewMenu("File", newItem, fyne.NewMenuItemSeparator(), quitItem)

	// View menu
	themeItem := fyne.NewMenuItem("Toggle Theme", func() {
		g.toggleTheme()
	})
	viewMenu := fyne.NewMenu("View", themeItem)

	// Help menu
	aboutItem := fyne.NewMenuItem("About", func() {
		// TODO: Show about dialog
	})
	helpMenu := fyne.NewMenu("Help", aboutItem)

	return fyne.NewMainMenu(fileMenu, viewMenu, helpMenu)
}

func (g *GUI) setupUI() {

	g.createFolderTree()
	g.createResultsTable()
	previewContainer := g.createPreviewPanel()
	searchContainer := g.createSearchPanel()

	headerRow := widget.NewTable(
		func() (int, int) { return 1, 6 },
		func() fyne.CanvasObject {
			label := widget.NewLabel("Header")
			label.TextStyle.Bold = true
			return label
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			headers := []string{"File Name", "Score", "Size", "Ext", "File Path", "ModTime"}
			if id.Col < len(headers) {
				label.SetText(headers[id.Col])
				label.Refresh()
			}
		},
	)
	g.setTableColumnWidths(headerRow)

	resultsContainer := container.NewBorder(
		widget.NewLabel("Search Results"),
		nil,
		nil,
		nil,
		g.resultsTable,
	)

	mainContent := container.NewVSplit(
		resultsContainer,
		previewContainer,
	)
	mainContent.SetOffset(ResultsPreviewSplit)

	centerPanel := container.NewBorder(
		searchContainer,
		nil,
		nil,
		nil,
		mainContent,
	)

	indexLabel := widget.NewLabel("Indexed Folders")
	indexLabel.TextStyle.Bold = true

	scrollTree := container.NewVScroll(g.folderTree)
	leftPanel := container.NewBorder(
		indexLabel,
		widget.NewSeparator(),
		nil,
		nil,
		scrollTree,
	)

	mainSplit := container.NewHSplit(
		leftPanel,
		centerPanel,
	)
	mainSplit.SetOffset(LeftPanelOffset)

	g.window.SetContent(mainSplit)

	g.initializeFolderTree()
}

func (g *GUI) updateSearchResults(results []models.Document) {
	g.searchResults = results
	g.resultsTable.Refresh()

	if len(results) == 0 {
		g.previewText.ParseMarkdown("No results found.")
	} else {
		g.previewText.ParseMarkdown(fmt.Sprintf("Found %d results. Select a result to view preview.", len(results)))
	}
}

func (g *GUI) showFolderSelectionDialog(onSelected func(path string)) {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		if uri == nil {
			return
		}
		onSelected(uri.Path())
	}, g.window)

}

func (g *GUI) handleFolderOperation(operation func() (*treeContext, error), successMessage string) {
	go func() {
		tc, err := operation() // REMOVE OR UPDATE
		if err != nil {
			dialog.ShowError(err, g.window)
		} else {
			g.tree = tc
			g.folderTree.Refresh()
			if successMessage != "" {
				dialog.ShowInformation("Success", successMessage, g.window)
			}
		}
	}()
}

func (g *GUI) setTableColumnWidths(table *widget.Table) {
	table.SetColumnWidth(0, TableColumnWidth0) // File Name
	table.SetColumnWidth(1, TableColumnWidth1) // Score
	table.SetColumnWidth(2, TableColumnWidth2) // Size
	table.SetColumnWidth(3, TableColumnWidth3) // Ext
	table.SetColumnWidth(4, TableColumnWidth4) // FilePath
	table.SetColumnWidth(5, TableColumnWidth5) // ModTime
}

func (g *GUI) clearSearch() {
	g.searchEntry.SetText("")
	g.searchResults = []models.Document{}
	g.resultsTable.Refresh()
	g.searchTerms = []string{}
	g.previewText.ParseMarkdown("Select a search result to view preview...")
}

func (g *GUI) createSearchPanel() *fyne.Container {

	g.searchEntry = widget.NewEntry()
	g.searchEntry.SetPlaceHolder("Enter search terms...")
	g.searchEntry.SetText("GoSeek")
	g.searchEntry.OnSubmitted = func(query string) {
		g.performSearch()
	}

	// TODO : Entry Range will be better (Min - Max) unit
	g.sizeFilter = widget.NewSelect(
		[]string{"Any Size", "< 1MB", "< 10MB", "< 100MB", "> 100MB"},
		func(value string) {
			// Handle size filter change
			fmt.Printf("Size filter changed to: %s\n", value)
		},
	)
	g.sizeFilter.SetSelected("Any Size")

	searchButton := widget.NewButtonWithIcon("Search", theme.SearchIcon(), func() {
		g.performSearch()
	})
	searchButton.Importance = widget.HighImportance

	clearButton := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		g.clearSearch()
	})

	searchRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(
			widget.NewLabel("Size:"),
			g.sizeFilter,
			searchButton,
			clearButton,
		),
		g.searchEntry,
	)

	return container.NewVBox(searchRow)
}
func truncateText(text string, maxLen int) string {
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}
func (g *GUI) createResultsTable() {
	g.resultsTable = widget.NewTable(
		func() (int, int) {
			// Add one row for the header
			return len(g.searchResults) + 1, 6 // rows, columns
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			headers := []string{"File Name", "Score", "Size", "Ext", "File Path", "ModTime"}
			if id.Row == 0 {
				label.TextStyle.Bold = true
				label.SetText(headers[id.Col])
			} else {
				label.TextStyle.Bold = false
				result := g.searchResults[id.Row-1]
				switch id.Col {
				case 0:
					label.SetText(truncateText(result.Path, 30))
				case 1:
					label.SetText(fmt.Sprintf("%.2f", result.Score))
				case 2:
					label.SetText(fmt.Sprintf("%v", result.Size))
				case 3:
					label.SetText(result.Extension)
				case 4:
					label.SetText(truncateText(result.Path, 80))
				case 5:
					label.SetText(result.ModTime)
				}
			}
		},
	)

	g.setTableColumnWidths(g.resultsTable)

	g.resultsTable.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 && id.Row-1 < len(g.searchResults) {
			result := g.searchResults[id.Row-1]
			g.loadPreview(result.Path)
		}
	}
}

func (g *GUI) createPreviewPanel() *fyne.Container {
	g.previewText = widget.NewRichText()
	g.previewText.ParseMarkdown("Select a search result to view preview...")
	g.previewText.Wrapping = fyne.TextWrapWord

	scrollPerview := container.NewVScroll(g.previewText)
	//TODO:
	// Add Buttons to jump between found terms in document
	// controls := container.NewHBox(itemNum, totalItems, prevBtn, nextBtn)

	previewPanel := container.NewBorder(
		nil,
		nil,
		nil,
		nil,
		scrollPerview,
	)
	return previewPanel
}
func (g *GUI) initTreeContext() {
	root := GetIndexes()
	g.tree = &treeContext{
		root:      root,
		treeCache: make(map[string]*Folder),
	}
}

func (tc *treeContext) findFolder(UID widget.TreeNodeID) *Folder {
	uidStr := strings.TrimSpace(string(UID))

	if uidStr == "" {
		return tc.root
	}
	if folder, ok := tc.treeCache[uidStr]; ok {
		return folder
	}

	folder := tc.findFolderinTree(uidStr)
	tc.treeCache[uidStr] = folder
	return folder
}

func (tc *treeContext) findFolderinTree(uid string) *Folder {

	parts := strings.Split(uid, string(filepath.Separator))
	currentFolder := tc.root

	for _, part := range parts {
		if part == "" {
			continue
		}

		if currentFolder.Children == nil {
			return nil
		}

		if child, exists := currentFolder.Children[part]; exists {
			currentFolder = child
		} else {
			return nil
		}
	}
	return currentFolder
}

func (g *GUI) createFolderTree() {
	if g.excludedFolders == nil {
		g.excludedFolders = make(map[string]bool)
	}

	g.initTreeContext()

	g.folderTree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {

			// Get the root folder trie
			rootFolder := g.tree.root
			if rootFolder == nil {
				println("No root folder found")
				return []widget.TreeNodeID{}
			}

			// Handle empty/whitespace UIDs (root level)
			if strings.TrimSpace(string(uid)) == "" {
				if len(rootFolder.Children) > 0 {
					var ids []widget.TreeNodeID
					for name := range rootFolder.Children {
						id := widget.TreeNodeID(name)
						ids = append(ids, id)
					}
					return ids
				}
				return []widget.TreeNodeID{}
			}

			// Find the folder for this UID
			folder := g.tree.findFolder(uid)
			if folder != nil && len(folder.Children) > 0 {
				var ids []widget.TreeNodeID
				for name := range folder.Children {
					childID := widget.TreeNodeID(string(uid) + string(filepath.Separator) + name)
					ids = append(ids, childID)
				}
				return ids
			}

			return []widget.TreeNodeID{}
		},
		func(uid widget.TreeNodeID) bool {

			rootFolder := g.tree.root
			if rootFolder == nil {
				return false
			}

			if strings.TrimSpace(string(uid)) == "" {
				hasFolders := len(rootFolder.Children) > 0
				return hasFolders
			}

			folder := g.tree.findFolder(uid)
			hasBranch := folder != nil && len(folder.Children) > 0
			return hasBranch
		},
		func(branch bool) fyne.CanvasObject {
			icon := theme.DocumentIcon()
			if branch {
				icon = theme.FolderIcon()
			}
			check := widget.NewCheck("", nil)
			label := widget.NewLabel("Folder")
			return container.NewHBox(
				widget.NewIcon(icon),
				check,
				label,
			)
		},
		func(uid widget.TreeNodeID, branch bool, node fyne.CanvasObject) {

			if strings.TrimSpace(string(uid)) == "" {
				return
			}

			rootFolder := g.tree.root
			if rootFolder == nil {
				return
			}

			folder := g.tree.findFolder(uid)
			if folder != nil {
				box := node.(*fyne.Container)
				check := box.Objects[1].(*widget.Check)
				label := box.Objects[2].(*widget.Label)

				label.SetText(folder.Name)

				// Create a path for the folder for exclusion tracking
				// TODO : Modify if UID will be changed Or Remove if not
				folderPath := g.getFolderPath(uid)

				check.SetChecked(!g.excludedFolders[folderPath])

				check.OnChanged = func(checked bool) {
					if checked {
						delete(g.excludedFolders, folderPath)
					} else {
						g.excludedFolders[folderPath] = true
					}
				}
			}
		},
	)

	g.folderTree.OnSelected = func(uid widget.TreeNodeID) {
		println("Tree node selected:", string(uid))

		rootFolder := g.tree.root
		if rootFolder == nil {
			return
		}

		folder := g.tree.findFolder(uid)
		if folder != nil {
			g.showFolderContextMenu(folder)
		}
	}
}

// Helper function to get the full path of a folder from UID
func (g *GUI) getFolderPath(uid widget.TreeNodeID) string {
	return string(uid) // For now, The UID as the path
}

func (g *GUI) initializeFolderTree() {
	rootFolder := g.tree.root
	if rootFolder == nil {
		return
	}

	time.Sleep(250 * time.Millisecond)
	g.folderTree.Refresh()

	// Open all first-level branches
	for name := range rootFolder.Children {
		node := widget.TreeNodeID(name)

		fyne.Do(func() {
			g.folderTree.OpenBranch(node)
		})
	}

	g.folderTree.Refresh()
}

func (g *GUI) showFolderContextMenu(folder *Folder) {
	menu := fyne.NewMenu("Folder Actions",
		fyne.NewMenuItem("Reindex", func() {
			g.reindexFolder(folder.Name)
		}),
		fyne.NewMenuItem("Remove from Index", func() {
			g.removeFolderFromIndex(folder.Name)
		}),
	)

	widget.ShowPopUpMenuAtPosition(menu, g.window.Canvas(), fyne.CurrentApp().Driver().AbsolutePositionForObject(g.folderTree))
}

func (g *GUI) getCheckedFolders() []string {
	var checkedFolders []string

	g.walkTree("", g.tree.root, func(path string) {
		if !g.excludedFolders[path] {
			checkedFolders = append(checkedFolders, path)
		}
	})
	return checkedFolders
}

func (g *GUI) walkTree(currPath string, folder *Folder, callback func(string)) {

	if folder == nil {
		return
	}
	if currPath != "" {
		callback(currPath)
	}
	for name, child := range folder.Children {
		childPath := currPath
		if childPath == "" {
			childPath = name
		} else {
			childPath = currPath + string(filepath.Separator) + name
		}
		g.walkTree(childPath, child, callback)
	}
}
func (g *GUI) performSearch() {
	query := g.searchEntry.Text
	if query == "" {
		return
	}

	// sizeFilter := g.sizeFilter.Selected

	g.previewText.ParseMarkdown("Searching...")
	fyne.Do(func() {
		folders := g.getCheckedFolders()
		stringQuery, err := CreateStringQuery(query)
		if err != nil {
			print(err)
			return
		}
		g.searchTerms = GetSearchTerms(stringQuery)
		results, err := SearchDocuments(stringQuery, folders)
		if err != nil {
			print(err)
			return
		}
		// if sizeFilter != "Any Size" {
		// 	fmt.Printf("Applying size filter: %s\n", sizeFilter)

		// }

		g.updateSearchResults(results)
	})
}

func (g *GUI) loadPreview(filePath string) {

	// g.previewText.ParseMarkdown("Loading preview...")
	g.previewText.Segments = []widget.RichTextSegment{}
	updatafn := func(seg []widget.RichTextSegment) {
		fyne.Do(func() {
			g.previewText.Segments = append(g.previewText.Segments, seg...)
			g.previewText.Refresh()
		})
	}
	fyne.Do(func() {
		re, err := BuildRegexPattern(g.searchTerms)
		if err != nil {
			print(err.Error())
			return
		}
		GetDocumentPreview(filePath, re, updatafn)
	})
}

func (g *GUI) reindexFolder(path string) {
	dialog.ShowInformation("Reindexing", fmt.Sprintf("Reindexing folder: %s", path), g.window)

	g.handleFolderOperation(func() (*treeContext, error) {
		return IndexFolder(path)
	}, "")
}

func (g *GUI) removeFolderFromIndex(path string) {
	dialog.ShowConfirm("Remove Folder",
		fmt.Sprintf("Are you sure you want to remove '%s' from the index?", path),
		func(confirmed bool) {
			if confirmed {
				g.handleFolderOperation(func() (*treeContext, error) {
					return RemoveFolder(path)
				}, "")
			}
		}, g.window)
}

// TODO : Change that
func (g *GUI) toggleTheme() {
	g.isDarkTheme = !g.isDarkTheme
	if g.isDarkTheme {
		g.app.Settings().SetTheme(theme.DarkTheme())
	} else {
		g.app.Settings().SetTheme(theme.LightTheme())
	}
}

func (g *GUI) createNewIndex() {
	g.showFolderSelectionDialog(func(folderPath string) {

		confirmMsg := fmt.Sprintf("Create new index for folder:\n\n%s\n\nThis will index all files in the selected folder and its subfolders. Continue?", folderPath)

		dialog.ShowConfirm("Create New Index", confirmMsg, func(confirmed bool) {
			if confirmed {
				g.startIndexing(folderPath)
			}
		}, g.window)
	})
}

func (g *GUI) startIndexing(folderPath string) {

	progressDialog := dialog.NewInformation("Indexing", "Indexing folder: "+folderPath+"\n\nPlease wait...", g.window)
	progressDialog.Show()

	// Simulate indexing process
	tc, err := IndexFolder(folderPath)

	progressDialog.Hide()

	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to index folder: %v", err), g.window)
	} else {
		// g.initTreeContext()
		dialog.ShowInformation("Success",
			fmt.Sprintf("Successfully created index for:\n%s\n\nThe folder has been added to your indexed folders.", folderPath),
			g.window)

		g.tree = tc
		g.folderTree.Refresh()
		g.clearSearch()
		g.previewText.ParseMarkdown("Index created successfully. Enter search terms to begin searching.")
	}
}

func (g *GUI) Run() {
	g.window.ShowAndRun()
}
