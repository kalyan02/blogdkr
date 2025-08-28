package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

var SC = SiteConfig{
	Title: "Kalyan",
	Navigation: []NavigationLink{
		{
			Name: "Home",
			URL:  "/",
		},
		{
			Name: "About",
			URL:  "/about",
		},
	},
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
	CreatedAt     time.Time
	ModifiedAt    time.Time
	ParsedContent *ParsedContent
}

type ContentStuff struct {
	FileName    map[string]FileDetail
	SlugFileMap map[string]FileDetail
	Config      Config
}

func (c *ContentStuff) DoPath(p string) (FileDetail, bool) {
	if fd, ok := c.FileName[p]; ok {
		return fd, true
	}
	if fd, ok := c.SlugFileMap[p]; ok {
		return fd, true
	}
	return FileDetail{}, false
}

func NewContentStuff(config Config) *ContentStuff {
	return &ContentStuff{
		Config:      config,
		FileName:    make(map[string]FileDetail),
		SlugFileMap: make(map[string]FileDetail),
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

		var ctime time.Time
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			// convert to time.Time
			ctime = time.Unix(int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec))
		}

		if info.IsDir() {
			c.FileName[relPath] = FileDetail{
				FileName:   relPath,
				FileType:   FileTypeDirectory,
				LoadedAt:   time.Now(),
				ModifiedAt: info.ModTime(),
				CreatedAt:  ctime,
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

			fd := FileDetail{
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
				CreatedAt: ctime,
			}
			c.FileName[relPath] = fd

			// crreate at <dir>/<slug>
			pg := NewPageFromFileDetail(&fd)
			slugPath := pg.Slug()
			if slugPath != "" {
				c.SlugFileMap[slugPath] = fd
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

	startT = time.Now()
	wire := NewWire(siteContent)
	err = wire.ScanForQueries()
	if err != nil {
		log.Fatalf("error scanning for queries: %v", err)
	}
	log.Infof("Scanned %d query files in %v", len(wire.queries), time.Since(startT))

	err = wire.NotifyFileChanged("_index.md")
	if err != nil {
		log.Errorf("error notifying file changed: %v", err)
	}

	r := gin.Default()
	r.LoadHTMLGlob("tmpl/*")
	// auth middleware
	r.Use(AuthMiddleware())

	//r.Handle("GET", "/*path", handleAllContentPages)
	r.NoRoute(handleAllContentPages)

	r.Run(":8081")
}

func IsStaticFile(path string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}
	// mime types
	staticExtMap := map[string]bool{
		".css":   true,
		".js":    true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".ico":   true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
		".otf":   true,
		".mp4":   true,
		".webm":  true,
		".ogg":   true,
		".mp3":   true,
		".wav":   true,
		".flac":  true,
		".pdf":   true,
		".txt":   true,
		".zip":   true,
	}
	return staticExtMap[ext]
}

func handleAllContentPages(c *gin.Context) {
	requestPath := c.Request.URL.Path

	// check if it is a static file is that's requested
	if IsStaticFile(requestPath) {
		for _, staticDir := range siteContent.Config.StaticDirs {
			staticFilePath := filepath.Join(staticDir, requestPath)
			if _, err := os.Stat(staticFilePath); err == nil {
				c.File(staticFilePath)
				return
			}
		}
	}

	// trim suffix
	requestPath = strings.TrimPrefix(requestPath, "/")
	requestPath = strings.TrimSuffix(requestPath, "/")

	if requestPath == "" {
		requestPath = "."
	}

	if file, ok := siteContent.DoPath(requestPath); ok {
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

	page := NewPageFromFileDetail(&file)

	indexPage := PostPage{
		Site: SC,
		Meta: PageMeta{
			Title: page.Title(),
		},
		PageHTML: page.SafeHTML(),
	}

	//if len(posts) > 0 {
	//	for _, p := range posts {
	//		ps := PostSummary{}
	//		postPage := NewPageFromFileDetail(&p)
	//		if p.ParsedContent != nil && p.ParsedContent.Frontmatter != nil {
	//			ps.Title = postPage.Title()
	//			ps.Date = postPage.DateCreated()
	//			ps.Tags = postPage.Hashtags()
	//			ps.Slug = postPage.Slug()
	//		} else {
	//			base := filepath.Base(p.FileName)
	//			ps.Title = strings.TrimSuffix(base, filepath.Ext(base))
	//		}
	//
	//		indexPage.Posts = append(indexPage.Posts, ps)
	//	}
	//	// sort posts by date desc
	//	sort.Slice(indexPage.Posts, func(i, j int) bool {
	//		return indexPage.Posts[i].Date.After(indexPage.Posts[j].Date)
	//	})
	//}

	c.HTML(200, "post.html", indexPage)
	fmt.Println(c.Errors)
}

func renderPage(c *gin.Context, file FileDetail) {
	// load the file content and render it
	p := NewPageFromFileDetail(&file)
	postPage := PostPage{
		Site: SC,
	}
	postPage.Meta = PageMeta{
		Title: p.Title(),
	}
	postPage.PageHTML = p.SafeHTML()
	postPage.CreatedDate = p.DateCreated()
	//postPage.ModifiedDate = p.DateModified()

	c.HTML(200, "post.html", postPage)

	//c.String(200, "Rendering page: %s", string(file.ParsedContent.HTML))
}
