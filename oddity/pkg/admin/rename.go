package admin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type RenameRequest struct {
	OldSlug string `json:"oldSlug" binding:"required"`
	NewSlug string `json:"newSlug" binding:"required"`
}

func (s *AdminApp) HandleRename(c *gin.Context) {
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// Trim whitespace and validate
	req.OldSlug = strings.TrimSpace(req.OldSlug)
	req.NewSlug = strings.TrimSpace(req.NewSlug)

	if req.OldSlug == "" || req.NewSlug == "" {
		c.JSON(400, gin.H{"error": "both old and new slug are required"})
		return
	}

	if req.OldSlug == req.NewSlug {
		c.JSON(400, gin.H{"error": "new slug must be different from current slug"})
		return
	}

	// Validate new slug format
	if err := validateSlug(req.NewSlug); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("invalid new slug: %v", err)})
		return
	}

	// Check if old file exists
	oldFilePath := filepath.Join(s.SiteContent.Config().Content.ContentDir, req.OldSlug+".md")
	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "original file not found"})
		return
	}

	// Check if target already exists
	newFilePath := filepath.Join(s.SiteContent.Config().Content.ContentDir, req.NewSlug+".md")
	if _, err := os.Stat(newFilePath); !os.IsNotExist(err) {
		c.JSON(409, gin.H{"error": "target file already exists"})
		return
	}

	// Check if target uploads directory already exists
	newUploadsDir := filepath.Join(s.SiteContent.Config().Content.UploadDir, req.NewSlug)
	if _, err := os.Stat(newUploadsDir); !os.IsNotExist(err) {
		c.JSON(409, gin.H{"error": "target uploads directory already exists"})
		return
	}

	// Perform the rename operations
	if err := s.performRename(req.OldSlug, req.NewSlug); err != nil {
		log.Errorf("Failed to rename %s to %s: %v", req.OldSlug, req.NewSlug, err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("rename failed: %v", err)})
		return
	}

	err := s.SiteContent.ReloadContent()
	if err != nil {
		log.Errorf("Failed to reload content after rename: %v", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to reload content: %v", err)})
		return
	}

	log.Infof("Successfully renamed %s to %s", req.OldSlug, req.NewSlug)
	c.JSON(200, gin.H{
		"message": "Post renamed successfully",
		"oldSlug": req.OldSlug,
		"newSlug": req.NewSlug,
	})
}

func (s *AdminApp) performRename(oldSlug, newSlug string) error {
	oldFilePath := filepath.Join(s.SiteContent.Config().Content.ContentDir, oldSlug+".md")
	newFilePath := filepath.Join(s.SiteContent.Config().Content.ContentDir, newSlug+".md")

	// Create target directory if it doesn't exist
	newDir := filepath.Dir(newFilePath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %v", err)
	}

	// Move the markdown file
	if err := os.Rename(oldFilePath, newFilePath); err != nil {
		return fmt.Errorf("failed to move markdown file: %v", err)
	}

	// Move uploads directory if it exists
	oldUploadsDir := filepath.Join(s.SiteContent.Config().Content.UploadDir, oldSlug)
	newUploadsDir := filepath.Join(s.SiteContent.Config().Content.UploadDir, newSlug)

	if _, err := os.Stat(oldUploadsDir); err == nil {
		// Create target uploads parent directory if needed
		newUploadsParent := filepath.Dir(newUploadsDir)
		if err := os.MkdirAll(newUploadsParent, 0755); err != nil {
			// Try to rollback the file move
			os.Rename(newFilePath, oldFilePath)
			return fmt.Errorf("failed to create target uploads directory: %v", err)
		}

		// First try simple rename (fastest if on same filesystem)
		if err := os.Rename(oldUploadsDir, newUploadsDir); err != nil {
			// If rename failed, try copy and remove (cross-filesystem move)
			log.Infof("Simple rename failed, attempting copy+remove: %v", err)
			if err := s.copyDirectory(oldUploadsDir, newUploadsDir); err != nil {
				// Try to rollback the file move
				os.Rename(newFilePath, oldFilePath)
				return fmt.Errorf("failed to copy uploads directory: %v", err)
			}

			// Copy succeeded, now remove the original directory
			if err := os.RemoveAll(oldUploadsDir); err != nil {
				log.Errorf("Warning: failed to remove old uploads directory %s: %v", oldUploadsDir, err)
				// Don't fail the operation - the copy succeeded
			} else {
				log.Infof("Successfully removed old uploads directory: %s", oldUploadsDir)
			}
		} else {
			log.Infof("Successfully renamed uploads directory: %s -> %s", oldUploadsDir, newUploadsDir)
		}
	}

	// Update image paths in the markdown content
	if err := s.updateContentPaths(newFilePath, oldSlug, newSlug); err != nil {
		log.Errorf("Warning: failed to update content paths: %v", err)
		// Don't fail the operation - the rename was successful
	}

	// Update database history records
	if err := s.updateHistoryRecords(oldSlug, newSlug); err != nil {
		// Try to rollback file and directory moves
		os.Rename(newFilePath, oldFilePath)
		if _, err := os.Stat(newUploadsDir); err == nil {
			os.Rename(newUploadsDir, oldUploadsDir)
		}
		return fmt.Errorf("failed to update history records: %v", err)
	}

	// Clean up empty directories in the old path
	s.cleanupEmptyDirectories(filepath.Dir(oldFilePath))
	s.cleanupEmptyDirectories(filepath.Dir(oldUploadsDir))

	return nil
}

func (s *AdminApp) updateHistoryRecords(oldSlug, newSlug string) error {
	// Update all history records that reference the old slug
	// Both FileName and FullSlug fields need to be updated
	oldFileName := oldSlug + ".md"
	newFileName := newSlug + ".md"

	tx := s.SiteContent.DB().Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start database transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	// Update records where FileName matches
	result := tx.Exec("UPDATE post_histories SET file_name = ?, full_slug = ? WHERE file_name = ?",
		newFileName, newSlug, oldFileName)
	if result.Error != nil {
		return fmt.Errorf("failed to update history records by filename: %v", result.Error)
	}

	// Update records where FullSlug matches (in case there are duplicates or inconsistencies)
	result = tx.Exec("UPDATE post_histories SET file_name = ?, full_slug = ? WHERE full_slug = ? AND file_name != ?",
		newFileName, newSlug, oldSlug, newFileName)
	if result.Error != nil {
		return fmt.Errorf("failed to update history records by slug: %v", result.Error)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit database transaction: %v", err)
	}

	log.Infof("Updated history records: %d rows affected", result.RowsAffected)
	return nil
}

func (s *AdminApp) cleanupEmptyDirectories(dir string) {
	// Only clean up within content directory for safety
	contentDir := s.SiteContent.Config().Content.ContentDir
	if !strings.HasPrefix(dir, contentDir) {
		return
	}

	// Don't remove the root content directory
	if dir == contentDir {
		return
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}

	// Remove empty directory and recurse up
	if err := os.Remove(dir); err == nil {
		log.Infof("Removed empty directory: %s", dir)
		s.cleanupEmptyDirectories(filepath.Dir(dir))
	}
}

func validateSlug(slug string) error {
	if strings.Contains(slug, "..") {
		return fmt.Errorf("slug cannot contain '..'")
	}

	if strings.Contains(slug, "\\") {
		return fmt.Errorf("slug cannot contain backslashes")
	}

	if strings.HasPrefix(slug, "/") || strings.HasSuffix(slug, "/") {
		return fmt.Errorf("slug cannot start or end with '/'")
	}

	// Check for dangerous characters
	dangerous := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range dangerous {
		if strings.Contains(slug, char) {
			return fmt.Errorf("slug cannot contain '%s'", char)
		}
	}

	return nil
}

func (s *AdminApp) copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			// Copy file
			return s.copyFile(path, dstPath)
		}
	})
}

func (s *AdminApp) copyFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func (s *AdminApp) updateContentPaths(filePath, oldSlug, newSlug string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	oldPath := fmt.Sprintf("/uploads/%s/", oldSlug)
	newPath := fmt.Sprintf("/uploads/%s/", newSlug)

	updatedContent := strings.ReplaceAll(string(content), oldPath, newPath)

	if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated file: %v", err)
	}

	log.Infof("Updated content paths: %s -> %s in %s", oldPath, newPath, filePath)
	return nil
}
