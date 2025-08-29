package contentstuff

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Wire is the notification and modification engine
type Wire struct {
	content *ContentStuff
	queries map[string][]QueryLocation // filepath -> queries in that file
	//watchers []QueryWatcher             // what to update when things change
}

// QueryLocation tracks where queries appear in files
type QueryLocation struct {
	Query     *QueryAST `json:"query"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	FilePath  string    `json:"file_path"`
	Content   []string  `json:"content"` // the generated content lines
	rawQuery  string    // temporary storage for raw query string during extraction
}

// QueryWatcher defines what should be updated when content changes
type QueryWatcher struct {
	Trigger   WatchTrigger    `json:"trigger"`   // what change triggers this
	Locations []QueryLocation `json:"locations"` // which queries to update
}

// WatchTrigger defines what changes should trigger updates
type WatchTrigger struct {
	Type      TriggerType `json:"type"`       // file_changed, tag_changed, etc
	Pattern   string      `json:"pattern"`    // file pattern or tag name
	FileTypes []FileType  `json:"file_types"` // which file types to watch
}

type TriggerType int

const (
	TriggerFileChanged TriggerType = iota
	TriggerTagChanged
	TriggerLinkChanged
	TriggerAnyContent
)

// NewWire creates a new Wire engine
func NewWire(content *ContentStuff) *Wire {
	return &Wire{
		content: content,
		queries: make(map[string][]QueryLocation),
		//watchers: make([]QueryWatcher, 0),
	}
}

func (w *Wire) QueryCount() int {
	cnt := 0
	for _, qList := range w.queries {
		cnt += len(qList)
	}
	return cnt
}

// ScanForQueries scans all content files for query comments
func (w *Wire) ScanForQueries() error {
	allFiles := w.content.AllFiles()
	for _, fileDetail := range allFiles {
		filePath := fileDetail.FileName
		if fileDetail.FileType == FileTypeMarkdown || fileDetail.FileType == FileTypeHTML {
			queries, err := w.extractQueriesFromFile(filePath)
			if err != nil {
				return fmt.Errorf("error scanning %s: %v", filePath, err)
			}
			if len(queries) > 0 {
				w.queries[filePath] = queries
				//w.registerWatchersForQueries(queries)
			}
		}
	}
	return nil
}

func (w *Wire) ScanContentFileForQueries(filePath string) error {
	fileDetail, exists := w.content.DoPath(filePath)
	if !exists {
		return fmt.Errorf("file not found in content store: %s", filePath)
	}

	if fileDetail.FileType != FileTypeMarkdown && fileDetail.FileType != FileTypeHTML {
		// Not a content file we care about
		return nil
	}

	queries, err := w.extractQueriesFromFile(filePath)
	if err != nil {
		return fmt.Errorf("error scanning %s: %v", filePath, err)
	}

	if len(queries) > 0 {
		w.queries[filePath] = queries
	} else {
		if _, exists := w.queries[filePath]; exists {
			delete(w.queries, filePath)
		}
	}
	return nil
}

// extractQueriesFromFile finds all query comments in a file
func (w *Wire) extractQueriesFromFile(filePath string) ([]QueryLocation, error) {
	fullPath := filepath.Join(w.content.Config.ContentDir, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return w.extractQueriesFromContent(filePath, string(content))
}

// extractQueriesFromContent extracts queries from content string (for testing)
func (w *Wire) extractQueriesFromContent(filePath string, content string) ([]QueryLocation, error) {
	lines := strings.Split(content, "\n")
	queries := make([]QueryLocation, 0)

	// Updated regex for XML-like syntax
	queryStartRegex := regexp.MustCompile(`<!--\s*<query\s+([^>]+)>\s*-->`)
	queryEndRegex := regexp.MustCompile(`<!--\s*</query>\s*-->`)

	var currentQuery *QueryLocation

	for i, line := range lines {
		// Look for start of query
		if matches := queryStartRegex.FindStringSubmatch(line); len(matches) > 1 {
			// Store raw query string, don't parse yet
			currentQuery = &QueryLocation{
				Query:     nil, // Will be set when we find the end tag
				StartLine: i,
				FilePath:  filePath,
				Content:   make([]string, 0),
				rawQuery:  matches[1], // Store raw query attributes
			}
		} else if queryEndRegex.MatchString(line) && currentQuery != nil {
			// End of query found - now parse the complete query
			xmlString := fmt.Sprintf("<query %s>", currentQuery.rawQuery)
			ast, err := ParseQuery(xmlString)
			if err != nil {
				// Skip invalid queries
				currentQuery = nil
				continue
			}

			currentQuery.Query = ast
			currentQuery.EndLine = i
			queries = append(queries, *currentQuery)
			currentQuery = nil
		} else if currentQuery != nil {
			// Inside a query block - this is generated content
			currentQuery.Content = append(currentQuery.Content, line)
		}
	}

	return queries, nil
}

// determineTriggerForQuery figures out what should trigger this query to update
func (w *Wire) determineTriggerForQuery(query *QueryAST) *WatchTrigger {
	switch query.Type {
	case QueryPosts, QueryPages:
		return &WatchTrigger{
			Type:      TriggerAnyContent,
			FileTypes: []FileType{FileTypeMarkdown, FileTypeHTML},
		}
	case QueryBacklinks:
		return &WatchTrigger{
			Type:      TriggerLinkChanged,
			FileTypes: []FileType{FileTypeMarkdown, FileTypeHTML},
		}
	}
	return nil
}

func (w *Wire) watcherMatchesTrigger(watcher *QueryWatcher, trigger *WatchTrigger) bool {
	return watcher.Trigger.Type == trigger.Type
}

// NotifyFileChanged is called when a file changes
func (w *Wire) NotifyFileChanged(filePath string) error {
	// Look up file in content store
	fileCtx, exists := w.content.DoPath(filePath)
	if !exists {
		return fmt.Errorf("file not found in content store: %s", filePath)
	}

	fileQueries, ok := w.queries[filePath]
	if ok {
		// the target file has queries - execute them
		for _, query := range fileQueries {
			if err := w.updateQuery(&fileCtx, query); err != nil {
				return fmt.Errorf("error updating query in %s: %v", query.FilePath, err)
			}
		}
	}

	return nil
}

//
//// NotifyAll refreshes all queries that might be affected by the specified file change
//func (w *Wire) NotifyAll(modifiedFile string) error {
//	// Look up file in content store
//	fileDetail, exists := w.content.DoPath(modifiedFile)
//	if !exists {
//		return fmt.Errorf("file not found in content store: %s", modifiedFile)
//	}
//
//	// Find all queries that might be affected by this file change
//	affectedQueries := make([]QueryLocation, 0)
//
//	// Check all queries across all files
//	for _, queryList := range w.queries {
//		for _, query := range queryList {
//			if w.shouldRefreshQuery(query, modifiedFile, fileDetail) {
//				affectedQueries = append(affectedQueries, query)
//			}
//		}
//	}
//
//	// Update all affected queries
//	for _, query := range affectedQueries {
//		if err := w.updateQuery(nil, query); err != nil {
//			return fmt.Errorf("error updating query in %s: %v", query.FilePath, err)
//		}
//	}
//
//	return nil
//}

// shouldRefreshQuery determines if a query should be refreshed based on the modified file
func (w *Wire) shouldRefreshQuery(query QueryLocation, modifiedFile string, fileDetail FileDetail) bool {
	// Skip if the query file itself was modified (avoid infinite loops)
	if query.FilePath == modifiedFile {
		return false
	}

	switch query.Query.Type {
	case QueryPosts, QueryPages:
		// Posts/pages queries should refresh when any content file changes
		if fileDetail.FileType == FileTypeMarkdown || fileDetail.FileType == FileTypeHTML {
			// If query has path filtering, check if modified file matches
			if query.Query.Path != "" {
				return w.matchesPathPattern(modifiedFile, query.Query.Path)
			}
			// No path filter means all content files are relevant
			return !strings.HasSuffix(modifiedFile, "index.md")
		}
	case QueryBacklinks:
		// Backlinks queries should refresh when files with links change
		return fileDetail.ParsedContent != nil && len(fileDetail.ParsedContent.WikiLinks) > 0
	}

	return false
}

// updateQuery re-executes a query and updates the file
func (w *Wire) updateQuery(fileCtx *FileDetail, location QueryLocation) error {
	// Execute the query to get new results
	results, err := w.executeQuery(fileCtx, location.Query)
	if err != nil {
		return err
	}

	// Read current file content
	fullPath := filepath.Join(w.content.Config.ContentDir, location.FilePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")

	// Replace content between start and end lines
	newLines := make([]string, 0, len(lines))
	newLines = append(newLines, lines[:location.StartLine+1]...) // up to and including start comment

	// Add new query results
	for _, result := range results {
		newLines = append(newLines, result)
	}

	// Add from end comment onwards
	if location.EndLine < len(lines) {
		newLines = append(newLines, lines[location.EndLine:]...)
	}

	// Write back to file
	newContent := strings.Join(newLines, "\n")
	return w.content.WriteContentFile(location.FilePath, newContent)
}

// executeQuery runs a query against current content
func (w *Wire) executeQuery(ctx *FileDetail, query *QueryAST) ([]string, error) {
	switch query.Type {
	case QueryPosts:
		return w.executePostsQuery(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported query type: %v", query.Type)
	}
}

// executePostsQuery handles "posts" queries
func (w *Wire) executePostsQuery(ctx *FileDetail, query *QueryAST) ([]string, error) {
	// Get all posts (non-index markdown files)
	var posts []FileDetail
	var allFiles = w.content.AllFiles()
	for _, file := range allFiles {
		if (file.FileType == FileTypeMarkdown || file.FileType == FileTypeHTML) &&
			!strings.HasSuffix(file.FileName, "index.md") {

			// Apply path filtering if specified
			if query.Path != "" {
				if !w.matchesPathPattern(file.FileName, query.Path) {
					continue
				}
			}

			posts = append(posts, file)
		}
	}

	allowed := w.applyAccessControl(ctx, posts, query)

	// Apply filters
	filtered := w.applyFiltersToFiles(allowed, query.Filters)

	// Apply sorting
	sorted := w.applySortToFiles(filtered, query)

	// Apply limit
	limited := w.applyLimitToFiles(sorted, query)

	// Convert to markdown format based on specified format
	return w.formatResults(limited, query.MDFormat)
}

func (w *Wire) applyAccessControl(ctx *FileDetail, posts []FileDetail, query *QueryAST) []FileDetail {
	// Rules:
	// - If ctx is nil (no context), only public posts
	// - If ctx is private, include private posts
	// - If ctx is public, only public posts unless query.IncludePrivate is true
	if ctx == nil {
		panic("ctx is nil")
	}

	ctxPage := NewPageFromFileDetail(ctx)
	isCtxPrivate := ctxPage.IsPrivate()

	var filtered []FileDetail
	for _, post := range posts {
		postPage := NewPageFromFileDetail(&post)
		isPostPrivate := postPage.IsPrivate()

		if isCtxPrivate {
			// Context is private, include all posts
			filtered = append(filtered, post)
		} else {
			// Context is public
			if !isPostPrivate {
				// Post is public, include it
				filtered = append(filtered, post)
			} else if query.IncludePrivate {
				// Post is private but query allows private, include it
				filtered = append(filtered, post)
			}
			// Otherwise skip private posts
		}
	}

	return filtered
}

// Helper functions for query execution
func (w *Wire) applyFiltersToFiles(files []FileDetail, filters []QueryFilter) []FileDetail {
	if len(filters) == 0 {
		return files
	}

	filtered := make([]FileDetail, 0, len(files))
	for _, file := range files {
		matches := true
		for _, filter := range filters {
			if !w.fileMatchesFilter(file, filter) {
				matches = false
				break
			}
		}
		if matches {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func (w *Wire) fileMatchesFilter(file FileDetail, filter QueryFilter) bool {
	switch filter.Field {
	case "tag":
		if file.ParsedContent != nil {
			for _, tag := range file.ParsedContent.Hashtags {
				switch filter.Operator {
				case "contains", "equals":
					if tag == filter.Value {
						return true
					}
				}
			}
		}
	}
	return false
}

func (w *Wire) applySortToFiles(files []FileDetail, query *QueryAST) []FileDetail {
	if query.Sort == "" {
		return files
	}

	sort.Slice(files, func(i, j int) bool {
		switch query.Sort {
		case SortDate, SortModified, SortRecent:
			pgi := NewPageFromFileDetail(&files[i])
			pgj := NewPageFromFileDetail(&files[j])

			datei := pgi.DateCreated()
			datej := pgj.DateCreated()

			if datei == nil && datej == nil {
				return false
			}
			if datei == nil {
				return false
			}
			if datej == nil {
				return true
			}

			if query.Order == SortDesc {
				return datei.After(*datej)
			}
			return datei.Before(*datej)
		case SortTitle:
			titleI := w.getTitleFromFile(files[i])
			titleJ := w.getTitleFromFile(files[j])
			if query.Order == SortDesc {
				return titleI > titleJ
			}
			return titleI < titleJ
		}
		return false
	})
	return files
}

func (w *Wire) applyLimitToFiles(files []FileDetail, query *QueryAST) []FileDetail {
	if query.Limit > 0 && len(files) > query.Limit {
		return files[:query.Limit]
	}
	return files
}

func (w *Wire) formatResults(files []FileDetail, format FormatType) ([]string, error) {
	results := make([]string, 0, len(files))

	for _, file := range files {
		page := NewPageFromFileDetail(&file)
		title := page.Title()
		slug := page.Slug()

		slug = "/" + strings.TrimPrefix(slug, "/")
		date := page.DateCreated().Format("2006-01-02")

		switch format {
		case FormatList:
			results = append(results, fmt.Sprintf("- [%s](%s)", title, slug))
		case FormatListWithDate:
			results = append(results, fmt.Sprintf("- %s - [%s](%s)", date, title, slug))
		case FormatDetailed:
			results = append(results, fmt.Sprintf("- [%s](%s)", title, slug))
			results = append(results, fmt.Sprintf("  Date: %s", date))
			if len(page.Hashtags()) > 0 {
				tags := strings.Join(page.Hashtags(), ", ")
				results = append(results, fmt.Sprintf("  Tags: %s", tags))
			}
		case FormatTable:
			// For table format, we'd need to collect all rows and format as a markdown table
			// This is more complex, so for now use compact format
			results = append(results, fmt.Sprintf("| %s | %s |", title, date))
		default:
			// Default to list format
			results = append(results, fmt.Sprintf("- [%s](%s)", title, slug))
		}
	}

	return results, nil
}

func (w *Wire) getTitleFromFile(file FileDetail) string {
	page := NewPageFromFileDetail(&file)
	return page.Title()
}

func (w *Wire) getSlugFromFile(file FileDetail) string {
	page := NewPageFromFileDetail(&file)
	return page.Slug()
}

// matchesPathPattern checks if a file path matches the given pattern
func (w *Wire) matchesPathPattern(filePath, pattern string) bool {
	// Normalize paths by converting backslashes to forward slashes
	filePath = filepath.ToSlash(filePath)
	pattern = filepath.ToSlash(pattern)

	// Handle leading "./" in pattern
	if strings.HasPrefix(pattern, "./") {
		pattern = pattern[2:]
	}

	// Try direct match first using filepath.Match
	if matched, _ := filepath.Match(pattern, filePath); matched {
		return true
	}

	// Handle directory patterns (ending with *)
	if strings.HasSuffix(pattern, "*") {
		dirPattern := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(filePath, dirPattern) {
			return true
		}
	}

	// Handle specific directory patterns like "blog/*"
	if strings.Contains(pattern, "/") {
		// For patterns like "blog/*", match files in that directory
		if strings.HasSuffix(pattern, "/*") {
			dir := strings.TrimSuffix(pattern, "/*")
			fileDir := filepath.Dir(filePath)
			return fileDir == dir || strings.HasPrefix(fileDir, dir+"/")
		}

		// For more complex patterns, use filepath.Match on the full path
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}

		// Also try matching against directory components
		pathParts := strings.Split(filePath, "/")
		patternParts := strings.Split(pattern, "/")

		return w.matchPathComponents(pathParts, patternParts)
	}

	// For simple patterns without slashes, match against filename only
	fileName := filepath.Base(filePath)
	matched, _ := filepath.Match(pattern, fileName)
	return matched
}

// matchPathComponents recursively matches path components with glob patterns
func (w *Wire) matchPathComponents(pathParts, patternParts []string) bool {
	// If pattern is exhausted but path isn't, no match (unless last pattern was **)
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	// If path is exhausted but pattern isn't, only match if remaining patterns are optional
	if len(pathParts) == 0 {
		for _, part := range patternParts {
			if part != "*" && part != "**" {
				return false
			}
		}
		return true
	}

	currentPattern := patternParts[0]

	// Handle ** (match any number of directories)
	if currentPattern == "**" {
		// Try matching rest of pattern at any remaining position in path
		for i := 0; i <= len(pathParts); i++ {
			if w.matchPathComponents(pathParts[i:], patternParts[1:]) {
				return true
			}
		}
		return false
	}

	// Handle * (match single directory or file)
	if currentPattern == "*" {
		return w.matchPathComponents(pathParts[1:], patternParts[1:])
	}

	// Handle exact match or glob pattern
	if matched, _ := filepath.Match(currentPattern, pathParts[0]); matched {
		return w.matchPathComponents(pathParts[1:], patternParts[1:])
	}

	return false
}
