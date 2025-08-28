package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

var siteContent *ContentStuff

func main() {

	startT := time.Now()
	siteContent = NewContentStuff(DefaultConfig)
	err := siteContent.LoadContent()
	if err != nil {
		log.Fatalf("error loading content: %v", err)
	}
	log.Infof("Loaded %d content files in %v", len(siteContent.FileName), time.Since(startT))

	//closeCh, err := siteContent.WatchContentChanges()
	//if err != nil {
	//	log.Fatalf("error watching content changes: %v", err)
	//}
	//defer func() {
	//	close(closeCh)
	//}()

	startT = time.Now()
	wire := NewWire(siteContent)
	err = wire.ScanForQueries()
	if err != nil {
		log.Fatalf("error scanning for queries: %v", err)
	}
	log.Infof("Scanned %d query files in %v", len(wire.queries), time.Since(startT))

	// notify all index files
	for fname := range siteContent.FileName {
		if strings.HasSuffix(fname, "index.md") || strings.HasSuffix(fname, "index.html") || strings.HasSuffix(fname, "_index.md") {
			err = wire.NotifyFileChanged(fname)
			if err != nil {
				log.Errorf("error notifying file changed: %v", err)
			}
			fmt.Println("Index file:", fname)
		}
	}

	r := gin.Default()
	r.LoadHTMLGlob("tmpl/*")
	// auth middleware
	r.Use(AuthMiddleware())

	//r.Handle("GET", "/*path", handleAllContentPages)
	adminGroup := r.Group("/admin")
	adminGroup.GET("/edit", handleAdminEditor)
	adminGroup.Any("/edit-data", handleEditPageData)
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
		EditURL:  fmt.Sprintf("/admin/edit?path=%s", page.Slug()),
	}

	c.HTML(200, "post.html", indexPage)
	fmt.Println(c.Errors)
}

func renderPage(c *gin.Context, file FileDetail) {
	// load the file content and render it
	p := NewPageFromFileDetail(&file)
	postPage := PostPage{
		Site:    SC,
		EditURL: fmt.Sprintf("/admin/edit?path=%s", p.Slug()),
	}
	postPage.Meta = PageMeta{
		Title: p.Title(),
	}
	postPage.PageHTML = p.SafeHTML()
	postPage.CreatedDate = p.DateCreated()
	//postPage.ModifiedDate = p.DateModified()

	c.HTML(200, "post.html", postPage)
}

type editPageData struct {
	FullSlug    string `json:"fullSlug"`
	Frontmatter string `json:"frontmatter"`
	Content     string `json:"content"`
	CurrentFile string `json:"currentFile"`
}

func (e editPageData) JSONString() string {
	jsonBytes, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		log.Errorf("error marshalling editPageData to JSON: %v", err)
		return "{}"
	}
	return string(jsonBytes)
}

func handleEditPageData(c *gin.Context) {
	if c.Request.Method == "POST" {
		var reqData editPageData
		if err := c.BindJSON(&reqData); err != nil {
			c.JSON(400, gin.H{"error": "invalid JSON body"})
			return
		}
		fmt.Println("Received edit data :", reqData)

		if reqData.CurrentFile == "" && reqData.FullSlug == "" {
			c.JSON(400, gin.H{"error": "either currentFile or fullSlug is required"})
			return
		}

		targetFile := reqData.CurrentFile
		file := siteContent.FileName[targetFile]
		fm := file.ParsedContent.Frontmatter

		parser := NewMarkdownParser(DefaultParserConfig())
		editedFile, err := parser.Parse([]byte(reqData.Content))
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error parsing content: %v", err)})
			return
		}

		editedFile.Frontmatter = fm
		file.ParsedContent = editedFile

		err = SaveFileDetail(&siteContent.Config, &file)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error saving file: %v", err)})
			return
		}
		err = siteContent.RefreshContent(file.FileName)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error refreshing content: %v", err)})
			return
		}

		data, err := buildEditPageDataResponse(file.FileName)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error building response: %v", err)})
			return
		}
		c.JSON(200, data)
		return
	}

	if c.Request.Method == "GET" {
		path := c.Query("path")
		if path == "" {
			c.JSON(400, gin.H{"error": "path query param is required"})
			return
		}

		// if path exists return its content
		data, err := buildEditPageDataResponse(path)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, data)
		return
	}
}

func buildEditPageDataResponse(path string) (editPageData, error) {
	file, ok := siteContent.DoPath(path)
	if !ok {
		return editPageData{}, fmt.Errorf("file not found")
	}
	if file.FileType != FileTypeMarkdown && file.FileType != FileTypeHTML {
		return editPageData{}, fmt.Errorf("not editable file type")
	}
	pg := NewPageFromFileDetail(&file)
	data := editPageData{
		FullSlug:    pg.Slug(),
		Frontmatter: file.ParsedContent.Frontmatter.Raw,
		Content:     string(pg.BodyWithTitle()),
		CurrentFile: file.FileName,
	}
	return data, nil
}

func handleAdminEditor(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.String(400, "path query param is required")
		return
	}

	data, err := buildEditPageDataResponse(path)
	if err != nil {
		log.Errorf("error building edit page data response: %v", err)
	}

	c.HTML(200, "edit.html", gin.H{
		"Data": data.JSONString(),
	})
}
