package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type editPageData struct {
	FullSlug    string `json:"fullSlug"`
	Frontmatter string `json:"frontmatter"`
	Content     string `json:"content"`
	CurrentFile string `json:"currentFile"`
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
		logrus.Errorf("error building edit page data response: %v", err)
	}

	c.HTML(200, "edit.html", gin.H{
		"Data": data.JSONString(),
	})
}
