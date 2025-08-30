package admin

import (
	"fmt"
	"io/fs"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (s *AdminApp) HandleFileUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid multipart form"})
		return
	}

	fullSlug := c.PostForm("fullSlug")
	if fullSlug == "" {
		c.JSON(400, gin.H{"error": "fullSlug parameter is required"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "no files uploaded"})
		return
	}

	// Create post-specific upload directory
	uploadDir := filepath.Join(s.SiteContent.ContentConfig.UploadDir, fullSlug)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create upload directory: %v", err)})
		return
	}

	// Get resize parameters (for all files, but only applied to images)
	widthStr := c.PostForm("width")
	heightStr := c.PostForm("height")
	var resizeWidth, resizeHeight int
	if widthStr != "" {
		if w, err := strconv.Atoi(widthStr); err == nil && w > 0 {
			resizeWidth = w
		}
	}
	if heightStr != "" {
		if h, err := strconv.Atoi(heightStr); err == nil && h > 0 {
			resizeHeight = h
		}
	}

	var uploadedFiles []string
	for _, fileHeader := range files {
		filename := filepath.Base(fileHeader.Filename)
		targetPath := filepath.Join(uploadDir, filename)

		// Check if this is an image and should be resized
		isImage := isImageFile(filename)
		shouldResize := isImage && (resizeWidth > 0 || resizeHeight > 0)

		if shouldResize {
			// Process and resize image
			if err := s.processAndSaveImage(fileHeader, targetPath, resizeWidth, resizeHeight); err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("failed to process image %s: %v", filename, err)})
				return
			}
		} else {
			// Save file normally
			if err := c.SaveUploadedFile(fileHeader, targetPath); err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("failed to save file: %v", err)})
				return
			}
		}

		uploadedFiles = append(uploadedFiles, fmt.Sprintf("/uploads/%s/%s", fullSlug, filename))
		logrus.Infof("Uploaded file: %s (resized: %v)", targetPath, shouldResize)
	}

	c.JSON(200, gin.H{"uploaded": uploadedFiles})
}

func (s *AdminApp) HandleUploadsList(c *gin.Context) {
	fullSlug := c.Query("fullSlug")
	if fullSlug == "" {
		c.JSON(400, gin.H{"error": "fullSlug parameter is required"})
		return
	}

	uploadDir := filepath.Join(s.SiteContent.ContentConfig.UploadDir, fullSlug)

	// Check if directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		c.JSON(200, gin.H{"files": []FileInfo{}})
		return
	}

	var files []FileInfo
	err := filepath.WalkDir(uploadDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		fileType := "file"
		if strings.HasPrefix(getContentType(path), "image/") {
			fileType = "image"
		}

		files = append(files, FileInfo{
			Name: d.Name(),
			Type: fileType,
			Size: info.Size(),
		})
		return nil
	})

	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to list files: %v", err)})
		return
	}

	c.JSON(200, gin.H{"files": files})
}

func (s *AdminApp) HandleFileDelete(c *gin.Context) {
	type reqStruct struct {
		FullSlug string `json:"fullSlug"`
		Filename string `json:"filename"`
	}

	var req reqStruct

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON body"})
		return
	}

	if req.FullSlug == "" || req.Filename == "" {
		c.JSON(400, gin.H{"error": "fullSlug and filename are required"})
		return
	}

	// Security: ensure filename doesn't contain path traversal
	if strings.Contains(req.Filename, "..") || strings.Contains(req.Filename, "/") || strings.Contains(req.Filename, "\\") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}

	filePath := filepath.Join(s.SiteContent.ContentConfig.UploadDir, req.FullSlug, req.Filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "file not found"})
		} else {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to delete file: %v", err)})
		}
		return
	}

	logrus.Infof("Deleted file: %s", filePath)
	c.JSON(200, gin.H{"success": true})
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func isImageFile(filename string) bool {
	return strings.HasPrefix(getContentType(filename), "image/")
}

func (s *AdminApp) processAndSaveImage(fileHeader *multipart.FileHeader, targetPath string, width, height int) error {
	// Open uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	// Decode image
	img, err := imaging.Decode(src)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Resize image if dimensions are specified
	if width > 0 || height > 0 {
		if width > 0 && height > 0 {
			// Both dimensions specified - use as bounding box (maintain aspect ratio)
			img = imaging.Fit(img, width, height, imaging.Lanczos)
		} else if width > 0 {
			// Only width specified - maintain aspect ratio
			img = imaging.Resize(img, width, 0, imaging.Lanczos)
		} else {
			// Only height specified - maintain aspect ratio
			img = imaging.Resize(img, 0, height, imaging.Lanczos)
		}
	}

	// Save the processed image (imaging.Save automatically detects format from extension)
	return imaging.Save(img, targetPath)
}
