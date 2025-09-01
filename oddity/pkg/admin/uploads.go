package admin

import (
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"os/exec"
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
	uploadDir := filepath.Join(s.SiteContent.Config.Content.UploadDir, fullSlug)
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

		// Check if this is an image and should be processed
		isImage := isImageFile(filename)
		ext := strings.ToLower(filepath.Ext(filename))
		isHeif := ext == ".heic" || ext == ".heif"
		shouldProcess := isImage && ((resizeWidth > 0 || resizeHeight > 0) || isHeif)

		if shouldProcess {
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

		finalFilename := filename
		if isHeif {
			finalFilename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
		}
		uploadedFiles = append(uploadedFiles, fmt.Sprintf("/uploads/%s/%s", fullSlug, finalFilename))
		logrus.Infof("Uploaded file: %s (processed: %v)", targetPath, shouldProcess)
	}

	c.JSON(200, gin.H{"uploaded": uploadedFiles})
}

func (s *AdminApp) HandleUploadsList(c *gin.Context) {
	fullSlug := c.Query("fullSlug")
	if fullSlug == "" {
		c.JSON(400, gin.H{"error": "fullSlug parameter is required"})
		return
	}

	uploadDir := filepath.Join(s.SiteContent.Config.Content.UploadDir, fullSlug)

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

	filePath := filepath.Join(s.SiteContent.Config.Content.UploadDir, req.FullSlug, req.Filename)

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
	case ".heic", ".heif":
		return "image/heif"
	default:
		return "application/octet-stream"
	}
}

func isImageFile(filename string) bool {
	return strings.HasPrefix(getContentType(filename), "image/")
}

func (s *AdminApp) processAndSaveImage(fileHeader *multipart.FileHeader, targetPath string, width, height int) error {
	// Check if this is a HEIF file that needs conversion
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == ".heic" || ext == ".heif" {
		return s.processHeifImage(fileHeader, targetPath, width, height)
	}

	// Open uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	// Decode image
	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
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

func (s *AdminApp) processHeifImage(fileHeader *multipart.FileHeader, targetPath string, width, height int) error {
	// Create temporary file for HEIF input
	tmpFile, err := os.CreateTemp("", "heif_*.heic")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy uploaded file to temp file
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		return fmt.Errorf("failed to copy to temp file: %v", err)
	}
	tmpFile.Close()

	// Convert HEIF to JPEG using ImageMagick, auto-orient and resize if needed
	args := []string{tmpFile.Name(), "-auto-orient"}

	if width > 0 || height > 0 {
		if width > 0 && height > 0 {
			args = append(args, "-resize", fmt.Sprintf("%dx%d>", width, height))
		} else if width > 0 {
			args = append(args, "-resize", fmt.Sprintf("%dx", width))
		} else {
			args = append(args, "-resize", fmt.Sprintf("x%d", height))
		}
	}

	// Change extension to jpg for output
	jpgPath := strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".jpg"
	args = append(args, jpgPath)

	cmd := exec.Command("magick", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ImageMagick conversion failed: %v, output: %s", err, string(output))
	}

	logrus.Infof("Converted HEIF to JPEG: %s -> %s", fileHeader.Filename, jpgPath)
	return nil
}
