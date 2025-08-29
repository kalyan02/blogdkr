package contentstuff

import (
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// QueryRenderer provides separate HTML rendering for query sections
type QueryRenderer struct {
	content   *ContentStuff
	templates map[string]*template.Template
}

// QuerySection represents a detected query section in content
type QuerySection struct {
	Query      *QueryAST
	StartLine  int
	EndLine    int
	Content    string
	Results    []FileDetail
	HTMLOutput template.HTML
}

// NewQueryRenderer creates a new query-aware HTML renderer
func NewQueryRenderer(content *ContentStuff) *QueryRenderer {
	return &QueryRenderer{
		content:   content,
		templates: make(map[string]*template.Template),
	}
}

// LoadTemplate loads a template for a specific query type
func (qr *QueryRenderer) LoadTemplate(name string, templatePath string) error {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load template %s: %v", name, err)
	}
	qr.templates[name] = tmpl
	return nil
}

// RenderWithQueries processes content and renders query sections with custom templates
func (qr *QueryRenderer) RenderWithQueries(content string, defaultRenderer func(string) template.HTML) (template.HTML, error) {
	// Detect query sections in the content
	sections, err := qr.extractQuerySections(content)
	if err != nil {
		return "", err
	}

	if len(sections) == 0 {
		// No queries, use default renderer
		return defaultRenderer(content), nil
	}

	// Process content with query sections
	var result strings.Builder
	lastEnd := 0

	for _, section := range sections {
		// Add content before query section using default renderer
		if section.StartLine > lastEnd {
			beforeContent := qr.getContentLines(content, lastEnd, section.StartLine)
			if beforeContent != "" {
				result.WriteString(string(defaultRenderer(beforeContent)))
			}
		}

		// Render query section with custom template
		queryHTML, err := qr.renderQuerySection(&section)
		if err != nil {
			// Fall back to original markdown content if template rendering fails
			result.WriteString(section.Content)
		} else {
			result.WriteString(string(queryHTML))
		}

		lastEnd = section.EndLine + 1
	}

	// Add any remaining content
	if lastEnd < len(strings.Split(content, "\n")) {
		afterContent := qr.getContentAfterLine(content, lastEnd)
		if afterContent != "" {
			result.WriteString(string(defaultRenderer(afterContent)))
		}
	}

	return template.HTML(result.String()), nil
}

// extractQuerySections finds all query sections in markdown content
func (qr *QueryRenderer) extractQuerySections(content string) ([]QuerySection, error) {
	lines := strings.Split(content, "\n")
	sections := make([]QuerySection, 0)

	queryStartRegex := regexp.MustCompile(`<!--\s*<query\s+([^>]+)>\s*-->`)
	queryEndRegex := regexp.MustCompile(`<!--\s*</query>\s*-->`)

	var currentSection *QuerySection

	for i, line := range lines {
		if matches := queryStartRegex.FindStringSubmatch(line); len(matches) > 1 {
			// Parse query attributes
			xmlString := fmt.Sprintf("<query %s>", matches[1])
			ast, err := ParseQuery(xmlString)
			if err != nil {
				continue // skip invalid queries
			}

			currentSection = &QuerySection{
				Query:     ast,
				StartLine: i,
			}
		} else if queryEndRegex.MatchString(line) && currentSection != nil {
			// End of query found
			currentSection.EndLine = i
			currentSection.Content = qr.getContentLines(content, currentSection.StartLine, currentSection.EndLine+1)

			// Execute query to get results
			if err := qr.executeQueryForSection(currentSection); err != nil {
				// If query execution fails, skip this section
				currentSection = nil
				continue
			}

			sections = append(sections, *currentSection)
			currentSection = nil
		}
	}

	return sections, nil
}

// executeQueryForSection executes the query and stores results in the section
func (qr *QueryRenderer) executeQueryForSection(section *QuerySection) error {
	switch section.Query.Type {
	case QueryPosts:
		return qr.executePostsQueryForSection(section)
	case QueryBacklinks:
		return qr.executeBacklinksQueryForSection(section)
	default:
		return fmt.Errorf("unsupported query type: %v", section.Query.Type)
	}
}

// executePostsQueryForSection executes a posts query and stores results
func (qr *QueryRenderer) executePostsQueryForSection(section *QuerySection) error {
	// Get all posts (non-index markdown files)
	var posts []FileDetail
	var allFiles = qr.content.AllFiles()
	for _, file := range allFiles {
		if (file.FileType == FileTypeMarkdown || file.FileType == FileTypeHTML) &&
			!strings.HasSuffix(file.FileName, "index.md") {
			posts = append(posts, file)
		}
	}

	// Apply filters, sorting, and limits (reuse Wire engine logic)
	wire := NewWire(qr.content)
	filtered := wire.applyFiltersToFiles(posts, section.Query.Filters)
	sorted := wire.applySortToFiles(filtered, section.Query)
	limited := wire.applyLimitToFiles(sorted, section.Query)

	section.Results = limited
	return nil
}

// executeBacklinksQueryForSection executes a backlinks query
func (qr *QueryRenderer) executeBacklinksQueryForSection(section *QuerySection) error {
	// TODO: Implement backlinks query execution
	section.Results = []FileDetail{}
	return nil
}

// renderQuerySection renders a query section using custom template or fallback
func (qr *QueryRenderer) renderQuerySection(section *QuerySection) (template.HTML, error) {
	// Check if custom HTML template is specified
	if section.Query.HTMLTemplate != "" {
		return qr.renderWithCustomTemplate(section)
	}

	// Use default rendering based on query type
	return qr.renderWithDefaultTemplate(section)
}

// renderWithCustomTemplate renders using a specified template file
func (qr *QueryRenderer) renderWithCustomTemplate(section *QuerySection) (template.HTML, error) {
	templateName := section.Query.HTMLTemplate

	// Load template if not already cached
	if _, exists := qr.templates[templateName]; !exists {
		// Try to load template from templates directory
		templatePath := filepath.Join("templates", "queries", templateName)
		if err := qr.LoadTemplate(templateName, templatePath); err != nil {
			return "", err
		}
	}

	tmpl := qr.templates[templateName]

	// Prepare template data
	data := qr.prepareTemplateData(section)

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("template execution failed: %v", err)
	}

	return template.HTML(result.String()), nil
}

// renderWithDefaultTemplate renders using built-in templates based on query type
func (qr *QueryRenderer) renderWithDefaultTemplate(section *QuerySection) (template.HTML, error) {
	switch section.Query.Type {
	case QueryPosts:
		return qr.renderPostsDefault(section), nil
	case QueryBacklinks:
		return qr.renderBacklinksDefault(section), nil
	default:
		return template.HTML(section.Content), nil
	}
}

// renderPostsDefault renders posts query with default styling
func (qr *QueryRenderer) renderPostsDefault(section *QuerySection) template.HTML {
	var result strings.Builder

	result.WriteString(`<div class="query-results posts-query">`)

	for _, file := range section.Results {
		page := NewPageFromFileDetail(&file)
		title := page.Title()
		slug := page.Slug()
		date := file.ModifiedAt.Format("2006-01-02")

		result.WriteString(`<div class="query-item">`)
		result.WriteString(fmt.Sprintf(`<h3><a href="/%s">%s</a></h3>`, slug, title))
		result.WriteString(fmt.Sprintf(`<time class="text-sm text-gray-500">%s</time>`, date))

		// Add tags if available
		if len(page.Hashtags()) > 0 {
			result.WriteString(`<div class="tags">`)
			for _, tag := range page.Hashtags() {
				result.WriteString(fmt.Sprintf(`<span class="tag">#%s</span>`, tag))
			}
			result.WriteString(`</div>`)
		}

		result.WriteString(`</div>`)
	}

	result.WriteString(`</div>`)
	return template.HTML(result.String())
}

// renderBacklinksDefault renders backlinks query with default styling
func (qr *QueryRenderer) renderBacklinksDefault(section *QuerySection) template.HTML {
	var result strings.Builder

	result.WriteString(`<div class="query-results backlinks-query">`)
	result.WriteString(`<h3>Backlinks</h3>`)

	if len(section.Results) == 0 {
		result.WriteString(`<p class="text-gray-500">No backlinks found.</p>`)
	} else {
		result.WriteString(`<ul>`)
		for _, file := range section.Results {
			page := NewPageFromFileDetail(&file)
			title := page.Title()
			slug := page.Slug()
			result.WriteString(fmt.Sprintf(`<li><a href="/%s">%s</a></li>`, slug, title))
		}
		result.WriteString(`</ul>`)
	}

	result.WriteString(`</div>`)
	return template.HTML(result.String())
}

// prepareTemplateData prepares data for template rendering
func (qr *QueryRenderer) prepareTemplateData(section *QuerySection) map[string]interface{} {
	// Convert FileDetail to more template-friendly format
	var posts []map[string]interface{}
	for _, file := range section.Results {
		page := NewPageFromFileDetail(&file)
		post := map[string]interface{}{
			"Title":       page.Title(),
			"Slug":        page.Slug(),
			"Date":        file.ModifiedAt.Format("2006-01-02"),
			"CreatedAt":   page.DateCreated(),
			"ModifiedAt":  file.ModifiedAt,
			"Tags":        page.Hashtags(),
			"WordCount":   ExtractWordCount(file.ParsedContent.Body),
			"ReadingTime": ExtractReadingTime(file.ParsedContent.Body),
		}
		posts = append(posts, post)
	}

	return map[string]interface{}{
		"Query":      section.Query,
		"Posts":      posts,
		"Count":      len(posts),
		"UpdatedAt":  time.Now().Format("2006-01-02 15:04:05"),
		"QueryType":  section.Query.Type.String(),
		"HasResults": len(posts) > 0,
	}
}

// Helper methods

// getContentLines extracts lines from content between start and end
func (qr *QueryRenderer) getContentLines(content string, start, end int) string {
	lines := strings.Split(content, "\n")
	if start >= len(lines) || start < 0 {
		return ""
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

// getContentAfterLine gets all content after a specific line
func (qr *QueryRenderer) getContentAfterLine(content string, line int) string {
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return ""
	}
	return strings.Join(lines[line:], "\n")
}
