package main

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ContentDir string
	StaticDirs []string
	ThemeDir   string
	Port       int
}

var DefaultConfig = Config{
	ContentDir: "content/content",
	StaticDirs: []string{"content/static"},
	Port:       8081,
}

type FileType int

const (
	FileTypeMarkdown FileType = iota
	FileTypeHTML
	FileTypeStatic
	FileTypeDirectory
)

type FileDetail struct {
	FileName      string
	FileType      FileType
	LoadedAt      time.Time
	ModifiedAt    time.Time
	ParsedContent *ParsedContent
}

type ContentStuff struct {
	FileName    map[string]FileDetail
	SlugFileMap map[string]string
	Config      Config
}

func NewContentStuff(config Config) *ContentStuff {
	return &ContentStuff{
		Config:      config,
		FileName:    make(map[string]FileDetail),
		SlugFileMap: make(map[string]string),
	}
}

func (c *ContentStuff) LoadContent() error {
	// recursively load all markdown files from ContentDir
	// and populate FileName and SlugFileMap

	// traverse the directory c.Config.ContentDir
	err := filepath.Walk(c.Config.ContentDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(c.Config.ContentDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			c.FileName[relPath] = FileDetail{
				FileName:   relPath,
				FileType:   FileTypeDirectory,
				LoadedAt:   time.Now(),
				ModifiedAt: info.ModTime(),
			}
		}

		if !info.IsDir() && (filepath.Ext(path) == ".md" || filepath.Ext(path) == ".html") {

			fileContent, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			mdParser := NewMarkdownParser(DefaultParserConfig())
			pc, err := mdParser.Parse(fileContent)
			if err != nil {
				return err
			}

			c.FileName[relPath] = FileDetail{
				FileName:      relPath,
				ParsedContent: pc,
				LoadedAt:      time.Now(),
				ModifiedAt:    info.ModTime(),
				FileType: func() FileType {
					if filepath.Ext(path) == ".md" {
						return FileTypeMarkdown
					} else {
						return FileTypeHTML
					}
				}(),
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

var siteContent *ContentStuff

func main() {

	startT := time.Now()
	siteContent = NewContentStuff(DefaultConfig)
	err := siteContent.LoadContent()
	if err != nil {
		log.Fatalf("error loading content: %v", err)
	}
	log.Infof("Loaded %d content files in %v", len(siteContent.FileName), time.Since(startT))

	r := gin.Default()
	r.LoadHTMLGlob("tmpl/*")
	r.Static("/static", "./content/static")

	// auth middleware
	r.Use(AuthMiddleware())

	//r.Handle("GET", "/*path", handleAllContentPages)
	r.NoRoute(handleAllContentPages)

	r.Run(":8081")
}

func handleAllContentPages(c *gin.Context) {
	requestPath := c.Request.URL.Path

	// trim suffix
	requestPath = strings.TrimPrefix(requestPath, "/")
	requestPath = strings.TrimSuffix(requestPath, "/")

	if requestPath == "" {
		requestPath = "index"
	}

	if file, ok := siteContent.FileName[requestPath]; ok {
		if file.FileType == FileTypeDirectory {
			// look for index.md or index.html in this directory
			renderIndexAtPath(c, requestPath)
			return
		}
		// if ends with index.html or index.md then render index of parent directory
		if strings.HasSuffix(requestPath, "index.html") || strings.HasSuffix(requestPath, "index.md") || strings.HasSuffix(requestPath, "index") {
			renderIndexFileAtPath(c, requestPath)
			return
		}

		renderPage(c, file)
		return
	}
	c.String(404, "Not Found")
}

func renderIndexAtPath(c *gin.Context, path string) {
	potentialIdxFiles := []string{
		filepath.Join(path, "_index.md"),
		filepath.Join(path, "index.md"),
		filepath.Join(path, "index.html"),
	}
	for _, idxFile := range potentialIdxFiles {
		if _, ok := siteContent.FileName[idxFile]; ok {
			renderIndexFileAtPath(c, idxFile)
			return
		}
	}

	defaultIndexPath := filepath.Join(path, "index.md")
	renderIndexAtPath(c, defaultIndexPath)
}

func renderIndexFileAtPath(c *gin.Context, path string) {
	file, ok := siteContent.FileName[path]
	if !ok {
		c.String(404, "Index Not Found")
		return
	}

	mdParser := NewMarkdownParser(DefaultParserConfig())
	headings := mdParser.ExtractHeadings(file.ParsedContent.Body)
	firstL1 := ""
	if len(headings) > 0 {
		for _, h := range headings {
			if h.Level == 1 {
				firstL1 = h.Text
				break
			}
		}
	}

	// check front matter title
	if file.ParsedContent != nil && file.ParsedContent.Frontmatter != nil {
		if title, ok := file.ParsedContent.Frontmatter.Data["title"].(string); ok && title != "" {
			firstL1 = title
		}
	}

	// use filename (wihtout dir and extension) as title if no h1 or frontmatter title
	if firstL1 == "" {
		base := filepath.Base(file.FileName)
		firstL1 = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// get posts in this directory (its relative to content dir)
	var posts []FileDetail
	for _, f := range siteContent.FileName {
		if f.FileType == FileTypeMarkdown || f.FileType == FileTypeHTML {
			if filepath.Dir(f.FileName) == filepath.Dir(file.FileName) &&
				!strings.HasSuffix(f.FileName, "index.md") &&
				!strings.HasSuffix(f.FileName, "index.html") &&
				!strings.HasSuffix(f.FileName, "_index.md") {
				posts = append(posts, f)
			}
		}
	}

	indexPage := IndexPage{
		Meta: PageMeta{
			Title: firstL1,
		},
		AboutHTML: template.HTML(file.ParsedContent.HTML),
	}

	if len(posts) > 0 {
		for _, p := range posts {
			ps := PostSummary{}
			if p.ParsedContent != nil && p.ParsedContent.Frontmatter != nil {
				if title, ok := p.ParsedContent.Frontmatter.Data["title"].(string); ok && title != "" {
					ps.Title = title
				} else {
					base := filepath.Base(p.FileName)
					ps.Title = strings.TrimSuffix(base, filepath.Ext(base))
				}
				if summary, ok := p.ParsedContent.Frontmatter.Data["summary"].(string); ok && summary != "" {
					ps.Summary = summary
				}
				if date, ok := p.ParsedContent.Frontmatter.Data["date"].(string); ok && date != "" {
					t, err := time.Parse("2006-01-02", date)
					if err == nil {
						ps.Date = t
					}
				} else {
					ps.Date = p.ModifiedAt
				}
				if tags, ok := p.ParsedContent.Frontmatter.Data["tags"].([]any); ok && len(tags) > 0 {
					for _, t := range tags {
						if ts, ok := t.(string); ok {
							ps.Tags = append(ps.Tags, ts)
						}
					}
				}

				// slug
				if slug, ok := p.ParsedContent.Frontmatter.Data["slug"].(string); ok && slug != "" {
					ps.Slug = filepath.Join(filepath.Dir(path), slug)
				} else {
					ps.Slug = strings.TrimSuffix(p.FileName, filepath.Ext(p.FileName))
				}
			} else {
				base := filepath.Base(p.FileName)
				ps.Title = strings.TrimSuffix(base, filepath.Ext(base))
			}

			indexPage.Posts = append(indexPage.Posts, ps)
		}
		// sort posts by date desc
		sort.Slice(indexPage.Posts, func(i, j int) bool {
			return indexPage.Posts[i].Date.After(indexPage.Posts[j].Date)
		})
	}

	c.HTML(200, "index.html", indexPage)
}

func renderPage(c *gin.Context, file FileDetail) {
	// load the file content and render it
	c.String(200, "Rendering page: %s", string(file.ParsedContent.HTML))
}
