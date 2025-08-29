package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type editPageData struct {
	FullSlug    string     `json:"fullSlug"`
	Frontmatter string     `json:"frontmatter"`
	Content     string     `json:"content"`
	CurrentFile string     `json:"currentFile"`
	BreadCrumbs []LinkData `json:"breadCrumbs"`
}

func handleEditPageData(c *gin.Context) {
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
		file := siteContent.FileName[targetFile]

		parser := NewMarkdownParser(DefaultParserConfig())
		editedFile, err := parser.Parse([]byte(reqData.Content))
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error parsing content: %v", err)})
			return
		}

		var fm *FrontmatterData
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
				if _, exists := siteContent.SlugFileMap[slug]; !exists {
					break
				}
				slug = fmt.Sprintf("%s-%d", originalSlug, i)
				i++
			}
			file.FileName = slug + ".md"
		}

		err = SaveFileDetail(&siteContent.Config, &file)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("error saving file: %v", err)})
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

func buildBreadCrumbLinks(path string) []LinkData {
	path = strings.Trim(path, "/")

	parts := SplitPath(path)
	var links []LinkData
	links = append(links, LinkData{
		Text: "Home",
		URL:  "/",
	})

	for i := range parts {
		if parts[i] == "" || parts[i] == "index" || parts[i] == "index.md" {
			continue
		}

		linkPath := strings.Join(parts[:i+1], "/")
		links = append(links, LinkData{
			Text: parts[i],
			URL:  "/" + linkPath,
		})
	}
	return links
}

func buildEditPageDataResponse(path string) (editPageData, error) {
	file, ok := siteContent.DoPath(path)
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
	if file.FileType != FileTypeMarkdown && file.FileType != FileTypeHTML {
		return editPageData{}, fmt.Errorf("not editable file type")
	}
	pg := NewPageFromFileDetail(&file)
	data := editPageData{
		FullSlug:    pg.Slug(),
		Frontmatter: file.ParsedContent.Frontmatter.Raw,
		Content:     string(pg.BodyWithTitle()),
		CurrentFile: file.FileName,
		BreadCrumbs: buildBreadCrumbLinks(pg.Slug()),
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
		logrus.Errorf("error building edit page data response: %v", err)
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
