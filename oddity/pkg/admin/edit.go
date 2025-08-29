package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sergi/go-diff/diffmatchpatch"
	log "github.com/sirupsen/logrus"

	"oddity/pkg/contentstuff"
)

type editPageData struct {
	FullSlug    string                  `json:"fullSlug"`
	Frontmatter string                  `json:"frontmatter"`
	Content     string                  `json:"content"`
	CurrentFile string                  `json:"currentFile"`
	BreadCrumbs []contentstuff.LinkData `json:"breadCrumbs"`
}

type AdminApp struct {
	WireController *contentstuff.Wire
	SiteContent    *contentstuff.ContentStuff
}

func (s *AdminApp) HandleEditPageData(c *gin.Context) {
	if c.Request.Method == "POST" {
		var reqData editPageData
		if err := c.BindJSON(&reqData); err != nil {
			c.JSON(400, gin.H{"error": "invalid JSON body"})
			return
		}
		fmt.Println("Received edit data :", reqData)

		if reqData.CurrentFile == "untitled.md" {
			reqData.CurrentFile = ""
		}

		if reqData.CurrentFile == "" && reqData.FullSlug == "" {
			c.JSON(400, gin.H{"error": "either currentFile or fullSlug is required"})
			return
		}

		reqData.FullSlug = strings.Trim(reqData.FullSlug, "/")

		targetFile := reqData.CurrentFile
		file := s.SiteContent.FileName[targetFile]

		parser := contentstuff.NewMarkdownParser(contentstuff.DefaultParserConfig())
		editedFile, err := parser.Parse([]byte(reqData.Content))
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error parsing content: %v", err)})
			return
		}

		var fm *contentstuff.FrontmatterData
		if file.ParsedContent == nil || file.ParsedContent.Frontmatter == nil {
			fm, _, err = parser.ExtractFrontmatter([]byte("---\n" + reqData.Frontmatter + "\n---\n"))
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("error parsing frontmatter: %v", err)})
				return
			}
		} else {
			fm = file.ParsedContent.Frontmatter
			if err = fm.SetRaw([]byte(reqData.Frontmatter)); err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("error updating frontmatter: %v", err)})
				return
			}
		}

		editedFile.Frontmatter = fm
		file.ParsedContent = editedFile

		if editedFile.Title == "" {
			c.JSON(400, gin.H{"error": "content must have a title"})
			return
		}

		// if new file, generate filename from slug
		if file.FileName == "" {
			slugParts := SplitPath(reqData.FullSlug)
			for i, part := range slugParts {
				slugParts[i] = slugify(part)
			}
			reqData.FullSlug = strings.Join(slugParts, "/")

			slug := reqData.FullSlug
			// ensure unique filename
			originalSlug := slug
			i := 1
			for {
				if _, exists := s.SiteContent.SlugFileMap[slug]; !exists {
					break
				}
				slug = fmt.Sprintf("%s-%d", originalSlug, i)
				i++
			}
			file.FileName = slug + ".md"
		}

		err = contentstuff.SaveFileDetail(s.SiteContent, s.WireController, &file)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error saving file: %v", err)})
			return
		}

		//err = siteContent.RefreshContent(file.FileName)
		//if err != nil {
		//	logrus.Errorf("error refreshing content for %s: %v", file.FileName, err)
		//}

		// if this is index.md in a directory, refresh queries and perform edits
		//if strings.HasSuffix(file.FileName, "index.md") {
		//	err = wireController.ScanContentFileForQueries(file.FileName)
		//	if err != nil {
		//		logrus.Errorf("error scanning content file for queries: %v", err)
		//	}
		//
		//	err = wireController.NotifyFileChanged(file.FileName)
		//	if err != nil {
		//		logrus.Errorf("error notifying file changed: %v", err)
		//	}
		//	err = siteContent.RefreshContent(file.FileName)
		//	if err != nil {
		//		logrus.Errorf("error refreshing content for %s: %v", file.FileName, err)
		//	}
		//}

		data, err := s.buildEditPageDataResponse(file.FileName)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error building response: %v", err)})
			return
		}
		c.JSON(200, data)
		return
	}

	if c.Request.Method == "GET" {
		path := c.Query("path")
		action := c.Query("action")
		if path != "" && action == "history" {
			histFiles := s.SiteContent.GetHistory(path)

			type histReponse struct {
				Title        string `json:"title"`
				Body         string `json:"body"`
				CreatedAt    string `json:"createdAt"`
				DiffText     string `json:"diffText,omitempty"`
				DiffHTML     string `json:"diffHTML,omitempty"`
				DeltaSummary string `json:"deltaSummary,omitempty"` // e.g. +10/-2
			}
			var fullHistory []histReponse

			for i, hf := range histFiles {
				if i >= 20 {
					break
				}
				mdParser := contentstuff.NewMarkdownParser(contentstuff.DefaultParserConfig())
				_, body, err := mdParser.ExtractFrontmatter([]byte(hf.Content))
				if err != nil {
					continue
				}

				fullHistory = append(fullHistory, histReponse{
					Title: hf.Title,
					Body:  string(body),

					CreatedAt: hf.Created.Format("2006-01-02 15:04:05"),
				})
			}

			var historyResponse []histReponse
			// build diffs now
			for i := 0; i < len(fullHistory)-1; i++ {
				curr := fullHistory[i]
				prev := fullHistory[i+1]
				diffHTML, inserts, deletes := buildDiffToDeltaHTML(prev.Body, curr.Body)

				if inserts > 0 || deletes > 0 {
					histItem := fullHistory[i]
					histItem.DiffHTML = diffHTML

					var insertClass = "summary-inserts"
					var deleteClass = "summary-deletes"
					if inserts == 0 {
						insertClass = "summary-grey"
					}
					if deletes == 0 {
						deleteClass = "summary-grey"
					}
					histItem.DeltaSummary = fmt.Sprintf(`<span class="%s">+%d</span> / <span class="%s">-%d</span>`, insertClass, inserts, deleteClass, deletes)
					historyResponse = append(historyResponse, histItem)
				}
			}

			c.JSON(200, gin.H{"history": historyResponse})
			return
		}
	}

	if c.Request.Method == "GET" {
		path := c.Query("path")
		if path == "" {
			c.JSON(400, gin.H{"error": "path query param is required"})
			return
		}

		// if path exists return its content
		data, err := s.buildEditPageDataResponse(path)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, data)
		return
	}
}

func buildDiffToDeltaHTML(text1, text2 string) (string, int, int) {
	var text bytes.Buffer
	text.WriteString(`<div class="diff">`)
	insertCount := 0
	deleteCount := 0

	dmp := diffmatchpatch.New()
	t1, t2, tt := dmp.DiffLinesToChars(text1, text2)
	diffs := dmp.DiffMain(t1, t2, false)
	diffs = dmp.DiffCharsToLines(diffs, tt)

	for _, d := range diffs {

		textLines := strings.Split(d.Text, "\n")
		startTag := ""
		endTag := ""

		switch d.Type {
		case diffmatchpatch.DiffInsert:
			startTag = `<span class="diff-insert">`
			endTag = "</span>"
			insertCount += len(textLines)
		case diffmatchpatch.DiffDelete:
			startTag = `<span class="diff-delete">`
			endTag = "</span>"
			deleteCount += len(textLines)
		case diffmatchpatch.DiffEqual:
			startTag = `<span class="diff-equal">`
			endTag = "</span>"
		}

		for i, line := range textLines {
			if line == "" && i == len(textLines)-1 {
				continue
			}
			text.WriteString(startTag)
			if line == "" {
				line = " "
			}
			line = strings.ReplaceAll(line, "\t", "     ")
			escapedLine := html.EscapeString(line)
			//escapedLine := strings.ReplaceAll(line, " ", "&nbsp;")
			text.WriteString(escapedLine)
			text.WriteString(endTag)
		}

	}
	text.WriteString(`</div>`)
	textString := text.String()
	return textString, insertCount, deleteCount
}

func buildBreadCrumbLinks(path string) []contentstuff.LinkData {
	path = strings.Trim(path, "/")

	parts := SplitPath(path)
	var links []contentstuff.LinkData
	links = append(links, contentstuff.LinkData{
		Text: "Home",
		URL:  "/",
	})

	for i := range parts {
		if parts[i] == "" || parts[i] == "index" || parts[i] == "index.md" {
			continue
		}

		linkPath := strings.Join(parts[:i+1], "/")
		links = append(links, contentstuff.LinkData{
			Text: parts[i],
			URL:  "/" + linkPath,
		})
	}
	return links
}

func (s *AdminApp) buildEditPageDataResponse(path string) (editPageData, error) {
	file, ok := s.SiteContent.DoPath(path)
	if !ok {
		defaultResponse := editPageData{
			FullSlug:    path,
			BreadCrumbs: buildBreadCrumbLinks(path),
		}

		path = strings.Trim(path, "/")

		defaultResponse.Content = fmt.Sprintf("# %s\n\nwrite...", filepath.Base(path))

		if strings.HasSuffix(path, "index") || strings.HasSuffix(path, "index.md") {
			if strings.Contains(path, "/") {
				defaultResponse.Content = fmt.Sprintf(`
# %s

<!-- <query type="posts" sort="recent" path="%s/*"> -->
<!-- </query> -->
`, filepath.Dir(path), filepath.Dir(path))
			}
		}

		return defaultResponse, nil
	}
	if file.FileType != contentstuff.FileTypeMarkdown && file.FileType != contentstuff.FileTypeHTML {
		return editPageData{}, fmt.Errorf("not editable file type")
	}
	pg := contentstuff.NewPageFromFileDetail(&file)
	data := editPageData{
		FullSlug:    pg.Slug(),
		Frontmatter: file.ParsedContent.Frontmatter.Raw,
		Content:     string(pg.BodyWithTitle()),
		CurrentFile: file.FileName,
		BreadCrumbs: buildBreadCrumbLinks(pg.Slug()),
	}
	return data, nil
}

func (s *AdminApp) HandleAdminEditor(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.String(400, "path query param is required")
		return
	}

	data, err := s.buildEditPageDataResponse(path)
	if err != nil {
		log.Errorf("error building edit page data response: %v", err)
	}

	c.HTML(200, "edit.html", gin.H{
		"Data": data.JSONString(),
	})
}

func slugify(s string) string {
	// convert to lowercase
	s = strings.ToLower(s)
	// replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// remove all non-alphanumeric and non-hyphen characters
	s = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(s, "")
	// replace multiple hyphens with a single hyphen
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	// trim leading and trailing hyphens
	s = strings.Trim(s, "-")
	return s
}

// SplitPath returns path components as a slice, OS-aware.
func SplitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == "/" || dir == "." {
			if dir != "" && dir != "." {
				parts = append([]string{dir}, parts...)
			}
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

func (e editPageData) JSONString() string {
	jsonBytes, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		log.Errorf("error marshalling editPageData to JSON: %v", err)
		return "{}"
	}
	return string(jsonBytes)
}
