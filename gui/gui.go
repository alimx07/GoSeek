package main

import (
	"fmt"
	"strconv"
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
	DefaultWindowWidth  = 800
	DefaultWindowHeight = 600
	TableColumnWidth0   = 150  // File Name
	TableColumnWidth1   = 80   // Score
	TableColumnWidth2   = 100  // Size
	TableColumnWidth3   = 80   // ext
	TableColumnWidth4   = 300  // File Path
	TableColumnWidth5   = 100  // ModTime
	LeftPanelOffset     = 0.25 // 25% for left panel
	ResultsPreviewSplit = 0.5  // 50% for results, 50% for preview
)

// Folder
type Folder struct {
	Name     string
	Path     string
	Children []*Folder
}

type SearchResult struct {
	FileName  string
	Score     float32
	Size      int
	Extension string
	FilePath  string
	ModTime   string
}

// Search in indexes
func SearchDocuments(query string, exact, caseSensitive bool) []SearchResult {
	return nil
}

// Read Data from Config.yaml
func GetIndexedFolders() []*Folder {
	return nil
}

func IndexFolder(path string) error {
	// Mock indexing function
	fmt.Printf("Indexing folder: %s\n", path)
	time.Sleep(100 * time.Millisecond) // Simulate work
	return nil
}

func RemoveFolder(path string) error {
	// Mock removal function
	fmt.Printf("Removing folder from index: %s\n", path)
	return nil
}

// Open File in Content Preview section
func GetDocumentPreview(path string) string {
	return ""
}

// GUI Application
type GUI struct {
	app             fyne.App
	window          fyne.Window
	searchEntry     *widget.Entry
	sizeFilter      *widget.Select
	resultsTable    *widget.Table
	previewText     *widget.RichText
	folderTree      *widget.Tree
	searchResults   []SearchResult
	excludedFolders map[string]bool
	isDarkTheme     bool
}

func NewApp() *GUI {
	app := app.New()
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

// TODO : Fix content preview dynamic sizing issue
func (g *GUI) setupUI() {

	g.createFolderTree()
	g.createResultsTable()
	g.createPreviewPanel()
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

	resultsContainer := container.NewVBox(
		widget.NewLabel("Search Results"),
		headerRow,
		g.resultsTable,
	)

	previewContainer := container.NewVBox(
		widget.NewLabel("Document Preview"),
		g.previewText,
	)

	mainContent := container.NewVSplit(
		resultsContainer,
		previewContainer,
	)
	mainContent.SetOffset(ResultsPreviewSplit)

	centerPanel := container.NewVBox(
		searchContainer,
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

func (g *GUI) updateSearchResults(results []SearchResult) {
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

func (g *GUI) handleFolderOperation(operation func() error, successMessage string) {
	go func() {
		err := operation() // REMOVE OR UPDATE
		if err != nil {
			dialog.ShowError(err, g.window)
		} else {
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
	g.searchResults = []SearchResult{}
	g.resultsTable.Refresh()
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

	// Clear button
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

func (g *GUI) createResultsTable() {
	g.resultsTable = widget.NewTable(
		func() (int, int) {
			return len(g.searchResults), 6 // rows, columns
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			if id.Row >= len(g.searchResults) {
				label.SetText("")
				return
			}

			result := g.searchResults[id.Row]
			switch id.Col {
			case 0:
				label.SetText(result.FileName)
			case 1:
				label.SetText(fmt.Sprintf("%.2f", result.Score))
			case 2:
				label.SetText(fmt.Sprintf("%v", result.Size))
			case 3:
				label.SetText(result.Extension)
			case 4:
				label.SetText(result.FilePath)
			case 5:
				label.SetText(result.ModTime)
			}
		},
	)

	g.setTableColumnWidths(g.resultsTable)

	g.resultsTable.OnSelected = func(id widget.TableCellID) {
		if id.Row < len(g.searchResults) {
			result := g.searchResults[id.Row]
			g.loadPreview(result.FilePath)
		}
	}
}

func (g *GUI) createPreviewPanel() {
	g.previewText = widget.NewRichText()
	g.previewText.ParseMarkdown("Select a search result to view preview...")
	g.previewText.Wrapping = fyne.TextWrapWord
}

// ALL Folder and Tree Operations
// need revisting , testing and Refactor
// Just work for now
func (g *GUI) createFolderTree() {

	println("FOLDER TREE CREATION CALLED")
	if g.excludedFolders == nil {
		g.excludedFolders = make(map[string]bool)
	}

	g.folderTree = widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			println("=== CHILDREN FUNCTION CALLED for UID: '", string(uid), "' ===")
			folders := GetIndexedFolders()
			println("Available folders:", len(folders))

			// Handle empty/whitespace UIDs (root level)
			if strings.TrimSpace(string(uid)) == "" {
				println("Root level - returning", len(folders), "root folders")
				var ids []widget.TreeNodeID
				for i := range folders {
					id := widget.TreeNodeID(strconv.Itoa(i))
					ids = append(ids, id)
					println("  Root folder ID:", string(id), "for", folders[i].Name)
				}
				return ids
			}

			folder := g.findFolder(uid, folders)
			if folder != nil && len(folder.Children) > 0 {
				println("Found folder:", folder.Name, "with", len(folder.Children), "children")
				var ids []widget.TreeNodeID
				for i := range folder.Children {
					childID := widget.TreeNodeID(string(uid) + "-" + strconv.Itoa(i))
					ids = append(ids, childID)
					println("  Child ID:", string(childID), "for", folder.Children[i].Name)
				}
				return ids
			}

			println("No children for UID:", string(uid))
			return []widget.TreeNodeID{}
		},
		func(uid widget.TreeNodeID) bool {
			println("=== ISBRANCH FUNCTION CALLED for UID: '", string(uid), "' ===")

			if strings.TrimSpace(string(uid)) == "" {
				folders := GetIndexedFolders()
				hasFolders := len(folders) > 0
				println("Root level UID - returning", hasFolders, "because we have", len(folders), "folders")
				return hasFolders
			}

			folders := GetIndexedFolders()
			folder := g.findFolder(uid, folders)
			hasBranch := folder != nil && len(folder.Children) > 0
			println("IsBranch result:", hasBranch)
			if folder != nil {
				println("  Folder:", folder.Name, "Children:", len(folder.Children))
			} else {
				println("  No folder found for UID:", string(uid))
			}
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
			println("=== UPDATE FUNCTION CALLED for UID: '", string(uid), "' ===")

			if strings.TrimSpace(string(uid)) == "" {
				println("Empty UID in update function - skipping")
				return
			}

			folders := GetIndexedFolders()
			folder := g.findFolder(uid, folders)
			if folder != nil {
				box := node.(*fyne.Container)
				check := box.Objects[1].(*widget.Check)
				label := box.Objects[2].(*widget.Label)

				label.SetText(folder.Name)
				println("Updating node label to:", folder.Name)

				// Update checkbox state
				check.SetChecked(!g.excludedFolders[folder.Path])

				// Handle user interaction
				check.OnChanged = func(checked bool) {
					if checked {
						delete(g.excludedFolders, folder.Path)
					} else {
						g.excludedFolders[folder.Path] = true
					}
				}
			} else {
				println("No folder found for UID in update function:", string(uid))
			}
		},
	)

	g.folderTree.OnSelected = func(uid widget.TreeNodeID) {
		println("Tree node selected:", string(uid))
		if strings.Contains(string(uid), "-") {
			return
		}

		folders := GetIndexedFolders()
		folder := g.findFolder(uid, folders)
		if folder != nil {
			g.showFolderContextMenu(folder)
		}
	}
}

func (g *GUI) findFolder(uid widget.TreeNodeID, folders []*Folder) *Folder {
	println("=== FIND FOLDER CALLED for UID: '", string(uid), "' ===")
	uidStr := strings.TrimSpace(string(uid))
	if uidStr == "" {
		println("Empty UID - returning nil")
		return nil
	}

	parts := strings.Split(uidStr, "-")
	if len(parts) == 0 {
		println("No parts found - returning nil")
		return nil
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil {
		println("Error parsing index:", err)
		return nil
	}

	if index >= len(folders) {
		println("Index", index, "out of range, max:", len(folders)-1)
		return nil
	}

	folder := folders[index]
	println("Found root folder:", folder.Name, "at index", index)

	for i := 1; i < len(parts); i++ {
		childIndex, err := strconv.Atoi(parts[i])
		if err != nil {
			println("Error parsing child index:", err)
			return nil
		}

		if childIndex >= len(folder.Children) {
			println("Child index", childIndex, "out of range for", folder.Name)
			return nil
		}

		folder = folder.Children[childIndex]
		println("Navigated to child:", folder.Name)
	}

	println("Final folder found:", folder.Name)
	return folder
}

func (g *GUI) initializeFolderTree() {
	println("=== INITIALIZING FOLDER TREE ===")

	folders := GetIndexedFolders()
	println("Folders available for initialization:", len(folders))

	time.Sleep(500 * time.Millisecond)
	g.folderTree.Refresh()
	for i := range folders {
		node := widget.TreeNodeID(strconv.Itoa(i))
		println("Attempting to open branch for node:", string(node))

		fyne.DoAndWait(func() {
			g.folderTree.OpenBranch(node)
		})
	}

	g.folderTree.Refresh()
	println("=== Tree initialization complete ===")
}

func (g *GUI) showFolderContextMenu(folder *Folder) {
	menu := fyne.NewMenu("Folder Actions",
		fyne.NewMenuItem("Reindex", func() {
			g.reindexFolder(folder.Path)
		}),
		fyne.NewMenuItem("Remove from Index", func() {
			g.removeFolderFromIndex(folder.Path)
		}),
	)

	widget.ShowPopUpMenuAtPosition(menu, g.window.Canvas(), fyne.CurrentApp().Driver().AbsolutePositionForObject(g.folderTree))
}

func (g *GUI) performSearch() {
	query := g.searchEntry.Text
	if query == "" {
		return
	}

	sizeFilter := g.sizeFilter.Selected

	g.previewText.ParseMarkdown("Searching...")

	go func() {
		results := SearchDocuments(query, false, false)

		if sizeFilter != "Any Size" {
			fmt.Printf("Applying size filter: %s\n", sizeFilter)

		}

		g.updateSearchResults(results)
	}()
}

func (g *GUI) loadPreview(filePath string) {

	g.previewText.ParseMarkdown("Loading preview...")

	go func() {
		preview := GetDocumentPreview(filePath)

		if preview != "" {
			g.previewText.ParseMarkdown(preview)
		} else {
			g.previewText.ParseMarkdown("No preview available for this file.")
		}
	}()
}

func (g *GUI) reindexFolder(path string) {
	dialog.ShowInformation("Reindexing", fmt.Sprintf("Reindexing folder: %s", path), g.window)

	g.handleFolderOperation(func() error {
		return IndexFolder(path)
	}, "")
}

func (g *GUI) removeFolderFromIndex(path string) {
	dialog.ShowConfirm("Remove Folder",
		fmt.Sprintf("Are you sure you want to remove '%s' from the index?", path),
		func(confirmed bool) {
			if confirmed {
				g.handleFolderOperation(func() error {
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

	go func() {
		// Simulate indexing process
		err := IndexFolder(folderPath)

		progressDialog.Hide()

		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to index folder: %v", err), g.window)
		} else {

			dialog.ShowInformation("Success",
				fmt.Sprintf("Successfully created index for:\n%s\n\nThe folder has been added to your indexed folders.", folderPath),
				g.window)

			g.folderTree.Refresh()
			g.clearSearch()
			g.previewText.ParseMarkdown("Index created successfully. Enter search terms to begin searching.")
		}
	}()
}

func (g *GUI) Run() {
	g.window.ShowAndRun()
}

func main() {
	// TODO:
	// Use Threads to ensure low latency
	// Refactor & Integrate
	gui := NewApp()
	gui.Run()
}
