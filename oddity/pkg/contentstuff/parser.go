package contentstuff

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// FrontmatterType represents the type of frontmatter delimiter used
type FrontmatterType int

const (
	FrontmatterNone FrontmatterType = iota
	FrontmatterYAML                 // --- delimited YAML
	FrontmatterTOML                 // +++ delimited TOML
)

// FrontmatterData holds parsed frontmatter content and metadata
type FrontmatterData struct {
	Type     FrontmatterType
	Raw      string
	Data     map[string]interface{}
	StartPos int
	EndPos   int
}

func (fm *FrontmatterData) SetRaw(data []byte) error {
	fm.Raw = string(data)
	switch fm.Type {
	case FrontmatterYAML:
		return yaml.Unmarshal(data, &fm.Data)
	case FrontmatterTOML:
		return toml.Unmarshal(data, &fm.Data)
	default:
		return nil
	}
}

// ParserConfig holds configuration for the markdown parser
type ParserConfig struct {
	EnableWikiLinks      bool
	EnableHashtags       bool
	EnableWebfinger      bool
	EnableFrontmatter    bool
	LazyLoadImages       bool
	SmartypantsFractions bool

	WikiLinkRenderer  func(string) string
	ShortcodeRenderer func(string) string
}

// DefaultParserConfig returns a default parser configuration
func DefaultParserConfig() *ParserConfig {
	return &ParserConfig{
		EnableWikiLinks:      true,
		EnableHashtags:       true,
		EnableWebfinger:      false,
		EnableFrontmatter:    true,
		LazyLoadImages:       true,
		SmartypantsFractions: false,

		WikiLinkRenderer: func(linkText string) string {
			// allow setting title with pipe syntax [[link-slug|Display Text]]
			parts := strings.SplitN(linkText, "|", 2)
			if len(parts) == 2 {
				return fmt.Sprintf(`<a href="%s">%s</a>`, parts[0], parts[1])
			}

			return fmt.Sprintf(`<a href="%s">%s</a>`, linkText, linkText)
		},
		ShortcodeRenderer: func(name string) string {
			switch name {
			case "toc":
				return `<div class="toc"><!-- Table of Contents --></div>`
			case "filelist":
				return `<div class="filelist"><!-- File List --></div>`
			case "backlinks":
				return `<div class="backlinks"><!-- Backlinks --></div>`
			default:
				return fmt.Sprintf(`<!-- Unknown shortcode: %s -->`, name)
			}
		},
	}
}

// ParsedContent holds the complete parsed result
type ParsedContent struct {
	Frontmatter *FrontmatterData
	Body        []byte
	Hashtags    []string
	PlainText   string
	Images      []ImageData
	WikiLinks   []string
	Shortcodes  []ShortcodeData
	Headings    []HeadingData
	Title       string
	HTML        []byte
}

// ToMarkdown reconstructs the markdown content from parsed parts
// it includes frontmatter and title if available along with body
// it should produce markdown very close to the original input
func (pc *ParsedContent) ToMarkdown() (string, error) {
	// marshal frontmatter first
	parts := make([]string, 0, 3)
	fmStr, err := pc.Frontmatter.Marshal()
	if err != nil {
		return "", err
	}
	if fmStr != "" {
		parts = append(parts, fmStr)
	}
	// add headings if any
	if pc.Title != "" {
		parts = append(parts, fmt.Sprintf("# %s", pc.Title))
	} else if len(pc.Headings) > 0 {
		// get first h1
		for _, h := range pc.Headings {
			if h.Level == 1 {
				parts = append(parts, fmt.Sprintf("# %s", h.Text))
				break
			}
		}
	}
	// add body
	parts = append(parts, string(pc.Body))
	return strings.Join(parts, "\n"), nil
}

// ShortcodeData represents a parsed shortcode
type ShortcodeData struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// MarkdownParser provides a clean interface for parsing markdown with frontmatter
type MarkdownParser struct {
	config   *ParserConfig
	parser   *parser.Parser
	renderer *html.Renderer

	hashtags   *[]string
	shortcodes *[]ShortcodeData
	wikilinks  *[]string
}

// NewMarkdownParser creates a new parser with the given configuration
func NewMarkdownParser(config *ParserConfig) *MarkdownParser {
	if config == nil {
		config = DefaultParserConfig()
	}

	mp := &MarkdownParser{
		config: config,
	}

	mp.initializeParser()
	mp.initializeRenderer()
	return mp
}

// initializeParser sets up the markdown parser with extensions and inline parsers
func (mp *MarkdownParser) initializeParser() {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.Attributes
	if !mp.config.SmartypantsFractions {
		extensions = extensions &^ parser.MathJax
	}

	mp.parser = parser.NewWithExtensions(extensions)

	// Register inline parsers based on configuration
	if mp.config.EnableWikiLinks {
		prev := mp.parser.RegisterInline('[', nil)
		wikiFunc, wikilinks := mp.wikiLinkParser(prev)
		mp.wikilinks = wikilinks
		mp.parser.RegisterInline('[', wikiFunc)
	}

	if mp.config.EnableHashtags {
		hashtagFn, hashtags := mp.hashtagParser()
		mp.hashtags = hashtags
		mp.parser.RegisterInline('#', hashtagFn)
	}

	// Initialize shortcode tracking
	shortcodes := make([]ShortcodeData, 0)
	mp.shortcodes = &shortcodes

	// Register shortcode parser
	mp.parser.RegisterInline('{', mp.shortcodeParser())
}

// initializeRenderer sets up the HTML renderer with appropriate flags
func (mp *MarkdownParser) initializeRenderer() {
	htmlFlags := html.CommonFlags

	if !mp.config.SmartypantsFractions {
		htmlFlags = htmlFlags &^ html.SmartypantsFractions
	}

	if mp.config.LazyLoadImages {
		htmlFlags = htmlFlags | html.LazyLoadImages
	}

	opts := html.RendererOptions{Flags: htmlFlags}
	mp.renderer = html.NewRenderer(opts)
}

// Parse parses the complete markdown content including frontmatter
func (mp *MarkdownParser) Parse(content []byte) (*ParsedContent, error) {
	result := &ParsedContent{}

	// Extract frontmatter if enabled
	bodyContent := content
	if mp.config.EnableFrontmatter {
		var err error
		result.Frontmatter, bodyContent, err = mp.ExtractFrontmatter(content)
		if err != nil {
			return nil, fmt.Errorf("frontmatter parsing error: %w", err)
		}
	}

	// Strip first H1 from content
	result.Headings = mp.ExtractHeadings(bodyContent)
	bodyContent, h1Removed := RemoveFirstH1(bodyContent, 5)

	if h1Removed {
		for _, h := range result.Headings {
			if h.Level == 1 {
				result.Title = h.Text
				break
			}
		}
	}
	// Set title from frontmatter or first H1
	if result.Title == "" && result.Frontmatter != nil {
		if title, ok := result.Frontmatter.Data["title"].(string); ok && title != "" {
			result.Title = title
		}
	}

	result.Body = bodyContent
	result.HTML = markdown.ToHTML(bodyContent, mp.parser, mp.renderer)

	// Extract hashtags if enabled
	if mp.config.EnableHashtags && mp.hashtags != nil {
		result.Hashtags = *mp.hashtags
		// Reset hashtags for next parse
		*mp.hashtags = (*mp.hashtags)[:0]
	}

	// Generate plain text
	result.PlainText = mp.ExtractPlainText(bodyContent)

	// Extract images
	result.Images = mp.ExtractImages(bodyContent, "")

	// Extract shortcodes - get from parser state after HTML parsing
	if mp.shortcodes != nil {
		result.Shortcodes = *mp.shortcodes
		// Reset shortcodes for next parse
		*mp.shortcodes = (*mp.shortcodes)[:0]
	}

	// Extract wiki links - get from parser state after HTML parsing
	if mp.wikilinks != nil {
		result.WikiLinks = *mp.wikilinks
		// Reset wiki links for next parse
		*mp.wikilinks = (*mp.wikilinks)[:0]
	}

	return result, nil
}

func RemoveFirstH1(markdown []byte, linesToSearch int) ([]byte, bool) {
	lines := strings.Split(string(markdown), "\n")
	result := make([]string, 0, len(lines))
	h1Removed := false

	for i, line := range lines {
		if linesToSearch > 0 && i < linesToSearch {
			// Check for ATX-style heading (# Title)
			if !h1Removed && regexp.MustCompile(`^#\s`).MatchString(line) {
				h1Removed = true
				// Skip this line and any immediately following empty lines
				for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
					i++
				}
				continue
			}
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n")), h1Removed
}

// ExtractFrontmatter extracts and parses frontmatter from content
func (mp *MarkdownParser) ExtractFrontmatter(content []byte) (*FrontmatterData, []byte, error) {
	if len(content) == 0 {
		return nil, content, nil
	}

	// Check for YAML frontmatter (---)
	// Use (?s) flag for . to match newlines
	if yamlMatch := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`).FindSubmatch(content); yamlMatch != nil {
		frontmatter := &FrontmatterData{
			Type:   FrontmatterYAML,
			EndPos: len(yamlMatch[0]),
			Data:   make(map[string]interface{}),
		}

		if err := frontmatter.SetRaw(yamlMatch[1]); err != nil {
			return nil, content, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}

		body := content[frontmatter.EndPos:]
		return frontmatter, body, nil
	}

	// Check for TOML frontmatter (+++)
	// Use (?s) flag for . to match newlines
	if tomlMatch := regexp.MustCompile(`(?s)^\+\+\+\s*\n(.*?)\n\+\+\+\s*\n`).FindSubmatch(content); tomlMatch != nil {
		frontmatter := &FrontmatterData{
			Type:   FrontmatterTOML,
			EndPos: len(tomlMatch[0]),
			Data:   make(map[string]interface{}),
		}

		if err := frontmatter.SetRaw(tomlMatch[1]); err != nil {
			return nil, content, fmt.Errorf("failed to parse TOML frontmatter: %w", err)
		}

		body := content[frontmatter.EndPos:]
		return frontmatter, body, nil
	}

	// No frontmatter found
	return nil, content, nil
}

// ExtractPlainText converts markdown to plain text, removing all formatting
func (mp *MarkdownParser) ExtractPlainText(content []byte) string {
	parser := parser.New()
	doc := markdown.Parse(content, parser)

	var buffer bytes.Buffer
	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if leaf := node.AsLeaf(); leaf != nil {
				buffer.Write(leaf.Literal)
				buffer.WriteByte(' ')
			}
		}
		return ast.GoToNext
	})

	// Clean up the text
	text := buffer.String()
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)

	// Remove multiple spaces
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return text
}

// ExtractImages finds all images in the markdown content
func (mp *MarkdownParser) ExtractImages(content []byte, dir string) []ImageData {
	var images []ImageData
	parser := parser.New()
	doc := markdown.Parse(content, parser)

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if img, ok := node.(*ast.Image); ok {
				text := mp.nodeToString(img)
				if len(text) > 0 {
					name := path.Join(dir, string(img.Destination))
					image := ImageData{
						Title: text,
						Name:  name,
					}
					images = append(images, image)
				}
				return ast.SkipChildren
			}
		}
		return ast.GoToNext
	})

	return images
}

// ExtractHeadings extracts all headings from the markdown content
func (mp *MarkdownParser) ExtractHeadings(content []byte) []HeadingData {
	return ExtractHeadings(content)
}

func (mp *MarkdownParser) wikiLinkParser(fallback func(*parser.Parser, []byte, int) (int, ast.Node)) (func(*parser.Parser, []byte, int) (int, ast.Node), *[]string) {
	wikilinks := make([]string, 0)

	parseFunc := func(p *parser.Parser, original []byte, offset int) (int, ast.Node) {
		data := original[offset:]
		n := len(data)

		// Minimum: [[X]]
		if n < 5 || data[1] != '[' {
			return fallback(p, original, offset)
		}

		// Find the closing ]]
		i := 2
		for i+1 < n && !(data[i] == ']' && data[i+1] == ']') {
			i++
		}

		if i+1 >= n {
			return fallback(p, original, offset)
		}

		linkText := string(data[2:i])

		// Record the wiki link
		if mp.wikilinks != nil && linkText != "" {
			*mp.wikilinks = append(*mp.wikilinks, linkText)
		}

		renderedLink := mp.config.WikiLinkRenderer(linkText)
		link := &ast.HTMLSpan{
			Leaf: ast.Leaf{Literal: []byte(renderedLink)},
		}

		// Return placeholder span instead of rendered link
		return i + 2, link
	}

	return parseFunc, &wikilinks
}

// hashtagParser creates a parser for #hashtags
func (mp *MarkdownParser) hashtagParser() (func(*parser.Parser, []byte, int) (int, ast.Node), *[]string) {
	hashtags := make([]string, 0)

	parseFunc := func(p *parser.Parser, data []byte, offset int) (int, ast.Node) {
		if p.InsideLink {
			return 0, nil
		}

		data = data[offset:]
		n := len(data)

		// Find the end of the hashtag
		i := 1 // Skip the #
		for i < n && !parser.IsSpace(data[i]) && data[i] != '\n' && isHashtagChar(data[i]) {
			i++
		}

		if i <= 1 {
			return 0, nil
		}

		hashtagText := string(data[1:i])
		hashtags = append(hashtags, hashtagText)

		link := &ast.Link{
			AdditionalAttributes: []string{`class="tag"`},
			Destination:          []byte(fmt.Sprintf("/search/?q=%%23%s", hashtagText)),
		}

		// Replace underscores with spaces in display text
		displayText := strings.ReplaceAll(string(data[0:i]), "_", " ")
		ast.AppendChild(link, &ast.Text{
			Leaf: ast.Leaf{Literal: []byte(displayText)},
		})

		return i, link
	}

	return parseFunc, &hashtags
}

// isHashtagChar checks if a character is valid for hashtags
func isHashtagChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '-'
}

// shortcodeParser creates a parser for {{shortcode}} syntax
func (mp *MarkdownParser) shortcodeParser() func(*parser.Parser, []byte, int) (int, ast.Node) {
	return func(p *parser.Parser, data []byte, offset int) (int, ast.Node) {
		data = data[offset:]
		n := len(data)

		// Minimum: {{x}}
		if n < 5 || data[1] != '{' {
			return 0, nil
		}

		// Find the closing }}
		i := 2
		for i+1 < n && !(data[i] == '}' && data[i+1] == '}') {
			i++
		}

		if i+1 >= n {
			return 0, nil
		}

		shortcodeName := strings.TrimSpace(string(data[2:i]))

		// Skip empty shortcodes
		if shortcodeName == "" {
			return 0, nil
		}

		// Record the shortcode
		if mp.shortcodes != nil {
			*mp.shortcodes = append(*mp.shortcodes, ShortcodeData{
				Name:     shortcodeName,
				Position: offset,
			})
		}

		rendered := &ast.HTMLSpan{
			Leaf: ast.Leaf{Literal: []byte("{{" + shortcodeName + "}}")},
		}

		if mp.config.ShortcodeRenderer != nil {
			renderedShortcode := mp.config.ShortcodeRenderer(shortcodeName)
			rendered = &ast.HTMLSpan{
				Leaf: ast.Leaf{Literal: []byte(renderedShortcode)},
			}
		}

		// Return a placeholder span for now - rendering happens separately
		return i + 2, rendered
	}
}

// nodeToString extracts text content from an AST node
func (mp *MarkdownParser) nodeToString(node ast.Node) string {
	var buffer bytes.Buffer
	ast.WalkFunc(node, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if text, ok := node.(*ast.Text); ok {
				buffer.Write(text.Literal)
			}
		}
		return ast.GoToNext
	})
	return buffer.String()
}

// Utility functions for extracting specific data

// ExtractPlainText extracts plain text from markdown content
func ExtractPlainText(content []byte) string {
	parser := NewMarkdownParser(DefaultParserConfig())
	return parser.ExtractPlainText(content)
}

// ExtractHashtags extracts all hashtags from markdown content
func ExtractHashtags(content []byte) []string {
	config := DefaultParserConfig()
	config.EnableWikiLinks = false // Focus only on hashtags
	parser := NewMarkdownParser(config)

	// Parse content to extract hashtags
	parsed, err := parser.Parse(content)
	if err != nil {
		return []string{}
	}
	return parsed.Hashtags
}

// ExtractImages extracts all images from markdown content
func ExtractImages(content []byte, baseDir string) []ImageData {
	parser := NewMarkdownParser(DefaultParserConfig())
	return parser.ExtractImages(content, baseDir)
}

// ExtractLinks extracts all regular markdown links from content
func ExtractLinks(content []byte) []LinkData {
	var links []LinkData
	parser := parser.New()
	doc := markdown.Parse(content, parser)

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if link, ok := node.(*ast.Link); ok {
				text := extractNodeText(link)
				linkData := LinkData{
					Text: text,
					URL:  string(link.Destination),
				}
				if len(link.Title) > 0 {
					linkData.Title = string(link.Title)
				}
				links = append(links, linkData)
				return ast.SkipChildren
			}
		}
		return ast.GoToNext
	})

	return links
}

// ExtractHeadings extracts all headings from markdown content
func ExtractHeadings(content []byte) []HeadingData {
	var headings []HeadingData
	parser := parser.New()
	doc := markdown.Parse(content, parser)

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if heading, ok := node.(*ast.Heading); ok {
				text := extractNodeText(heading)
				headingData := HeadingData{
					Level: heading.Level,
					Text:  text,
				}
				if len(heading.HeadingID) > 0 {
					headingData.ID = string(heading.HeadingID)
				}
				headings = append(headings, headingData)
				return ast.SkipChildren
			}
		}
		return ast.GoToNext
	})

	return headings
}

// ExtractCodeBlocks extracts all code blocks from markdown content
func ExtractCodeBlocks(content []byte) []CodeBlockData {
	var codeBlocks []CodeBlockData
	parser := parser.New()
	doc := markdown.Parse(content, parser)

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if codeBlock, ok := node.(*ast.CodeBlock); ok {
				code := CodeBlockData{
					Language: string(codeBlock.Info),
					Code:     string(codeBlock.Literal),
				}
				codeBlocks = append(codeBlocks, code)
			}
		}
		return ast.GoToNext
	})

	return codeBlocks
}

// ExtractWordCount counts words in the plain text version of markdown content
func ExtractWordCount(content []byte) int {
	plainText := ExtractPlainText(content)
	if len(plainText) == 0 {
		return 0
	}

	// Split by whitespace and count non-empty words
	words := strings.Fields(plainText)
	return len(words)
}

// ExtractReadingTime estimates reading time in minutes (assuming 200 words per minute)
func ExtractReadingTime(content []byte) int {
	wordCount := ExtractWordCount(content)
	readingTime := (wordCount + 199) / 200 // Round up
	if readingTime < 1 {
		return 1
	}
	return readingTime
}

// Helper data structures for extraction functions

// LinkData represents a markdown link
type LinkData struct {
	Text  string `json:"text"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// HeadingData represents a markdown heading
type HeadingData struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id,omitempty"`
}

// CodeBlockData represents a code block
type CodeBlockData struct {
	Language string `json:"language,omitempty"`
	Code     string `json:"code"`
}

// extractNodeText is a helper function to extract text from AST nodes
func extractNodeText(node ast.Node) string {
	var buffer bytes.Buffer
	ast.WalkFunc(node, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if text, ok := node.(*ast.Text); ok {
				buffer.Write(text.Literal)
			}
		}
		return ast.GoToNext
	})
	return buffer.String()
}

// Helper methods for FrontmatterData

// GetString safely gets a string value from frontmatter data
func (fm *FrontmatterData) GetString(key string) (string, bool) {
	if fm == nil || fm.Data == nil {
		return "", false
	}
	if val, ok := fm.Data[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// SetString safely sets a string value in frontmatter data
func (fm *FrontmatterData) SetString(key, value string) {
	if fm == nil {
		return
	}
	if fm.Data == nil {
		fm.Data = make(map[string]interface{})
	}
	fm.Data[key] = value
}

// SetValue safely sets a value of any type in frontmatter data
func (fm *FrontmatterData) SetValue(key string, value any) {
	if fm == nil {
		return
	}
	if fm.Data == nil {
		fm.Data = make(map[string]interface{})
	}
	fm.Data[key] = value
}

// GetValue safely gets a value of any type from frontmatter data
func (fm *FrontmatterData) GetValue(key string) (any, bool) {
	if fm == nil || fm.Data == nil {
		return nil, false
	}
	val, ok := fm.Data[key]
	return val, ok
}

// GetStringSlice safely gets a string slice from frontmatter data
func (fm *FrontmatterData) GetStringSlice(key string) []string {
	if fm == nil || fm.Data == nil {
		return nil
	}
	if val, ok := fm.Data[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// GetBool safely gets a boolean value from frontmatter data
func (fm *FrontmatterData) GetBool(key string) bool {
	if fm == nil || fm.Data == nil {
		return false
	}
	if val, ok := fm.Data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// HasKey checks if a key exists in frontmatter data
func (fm *FrontmatterData) HasKey(key string) bool {
	if fm == nil || fm.Data == nil {
		return false
	}
	_, exists := fm.Data[key]
	return exists
}

// Marshal the frontmatter data to string
func (fm *FrontmatterData) Marshal() (string, error) {
	if fm == nil {
		return "", nil
	}
	switch fm.Type {
	case FrontmatterYAML:
		out, err := yaml.Marshal(fm.Data)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("---\n%s---", string(out)), nil
	case FrontmatterTOML:
		var buf bytes.Buffer
		encoder := toml.NewEncoder(&buf)
		if err := encoder.Encode(fm.Data); err != nil {
			return "", err
		}
		return fmt.Sprintf("+++\n%s+++", buf.String()), nil
	default:
		return "", nil
	}
}
