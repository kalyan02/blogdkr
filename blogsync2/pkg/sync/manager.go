package sync

import (
	"archive/zip"
	"blogsync2/pkg/db"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"blogsync2/pkg/config"
	"blogsync2/pkg/dropbox"

	"gorm.io/gorm"
)

type Manager struct {
	config *config.Config
	client *dropbox.Client
	db     *gorm.DB
}

type EventType int

type Event struct {
	Type EventType
	Data any
}

const (
	FilesChanged EventType = iota
	FilesChangedWithCursor
	ForceSync
)

func NewManager(cfg *config.Config, client *dropbox.Client, database *gorm.DB) *Manager {
	manager := &Manager{
		config: cfg,
		client: client,
		db:     database,
	}

	// Try to load cursor from file
	//if cursor, err := manager.loadCursor(); err == nil {
	//	manager.lastCursor = cursor
	//}

	return manager
}

func (m *Manager) Start(eventChan <-chan Event) {
	log.Println("Starting sync manager loop")

	for event := range eventChan {
		switch event.Type {
		case FilesChanged:
			log.Println("File changed event received, starting incremental sync")
			if err := m.incrementalSync(event.Data); err != nil {
				log.Printf("Incremental sync failed: %v", err)
			}
		case ForceSync:
			log.Println("Force sync event received, starting full sync")
			if err := m.syncFiles(); err != nil {
				log.Printf("Force sync failed: %v", err)
			}
		default:
			panic("unhandled default case")
		}
	}
}

func (m *Manager) syncFiles() error {
	log.Println("Starting file synchronization")

	basePath := m.config.Sync.LocalBasePath
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create local base directory: %w", err)
	}

	files, newCursor, err := m.client.ListFolder(m.config.Sync.DropboxFolder, true)
	if err != nil {
		return fmt.Errorf("failed to list Dropbox folder: %w", err)
	}

	log.Printf("Found %d files in Dropbox", len(files))

	// Build a set of all files that should exist locally
	expectedFiles := make(map[string]bool)

	// Download/update files from Dropbox
	for _, file := range files {
		localPathRef := strings.TrimPrefix(file.Path, "/")

		needsDownload := true

		// check if it exists in db
		f := db.File{
			UserID:    1,
			LocalPath: localPathRef,
		}

		tx := m.db.Where("user_id = ? AND local_path = ?", f.UserID, f.LocalPath).First(&f)
		if err := tx.Error; err != nil && err != gorm.ErrRecordNotFound {
			log.Printf("Failed to query file %s from database: %v", localPathRef, err)
		} else {
			if tx.RowsAffected > 0 {
				dbHash := f.ContentHash
				// if db hash is not up to date, compute it
				if dbHash == "" {
					localPath := filepath.Join(m.config.Sync.LocalBasePath, localPathRef)
					// check if file exists
					if _, err := os.Stat(localPath); err == nil {
						computedHash, err := HashFile(localPath)
						if err != nil {
							log.Printf("Failed to compute hash for %s: %v", localPathRef, err)
						} else {
							dbHash = computedHash
							f.ContentHash = dbHash
							if err := m.db.Save(&f).Error; err != nil {
								log.Printf("Failed to update hash for %s in database: %v", localPathRef, err)
							} else {
								log.Printf("Updated hash for %s in database", localPathRef)
							}
						}
					}
				}

				if dbHash != "" && file.ContentHash != "" && dbHash == file.ContentHash {
					needsDownload = false
					log.Printf("File %s already up to date (hash match from DB)", file.Path)
				}
			}
		}

		if !needsDownload {
			continue
		}

		relativePath := strings.TrimPrefix(file.Path, m.config.Sync.DropboxFolder)
		relativePath = strings.TrimPrefix(relativePath, "/")

		localPath := filepath.Join(basePath, relativePath)
		expectedFiles[localPath] = true

		log.Printf("Syncing file: %s -> %s", file.Path, localPath)

		if err := m.syncSingleFile(&file, localPath); err != nil {
			log.Printf("Failed to sync file %s: %v", file.Path, err)
			continue
		}
	}

	if err := m.saveCursor(newCursor); err != nil {
		log.Printf("Failed to save cursor: %v", err)
	} else {
		//m.lastCursor = newCursor
		err := m.saveCursor(newCursor)
		if err != nil {
			log.Printf("Failed to save cursor: %v", err)
		}
		log.Printf("Saved cursor: %s", newCursor)
	}

	// Remove local files that no longer exist in Dropbox
	if err := m.removeDeletedFiles(basePath, expectedFiles); err != nil {
		log.Printf("Failed to remove deleted files: %v", err)
	}

	log.Println("File synchronization completed")

	if err := m.runBuildCommand(); err != nil {
		log.Printf("Build command failed: %v", err)
		return err
	}

	if err := m.applyCopyRules(); err != nil {
		log.Printf("Copy rules failed: %v", err)
		return err
	}

	log.Println("Full sync process completed successfully")
	return nil
}

func (m *Manager) incrementalSync(data any) error {
	//dropbox.WebhookNotification{}
	notificationData, ok := data.(*dropbox.WebhookNotification)
	if !ok {
		return fmt.Errorf("invalid data for incremental sync")
	}
	_ = notificationData

	//if m.lastCursor == "" {
	//	log.Println("No cursor available, falling back to full sync")
	//	return m.syncFiles()
	//}

	cursor, err := m.loadCursor()
	if err != nil {
		log.Println("No cursor available, falling back to full sync")
		return m.syncFiles()
	}

	log.Println("Starting incremental sync from cursor")

	changedFiles, latestCursor, err := m.client.GetChangesFromCursor(cursor)
	if err != nil {
		return fmt.Errorf("failed to get changes from cursor: %w", err)
	}

	m.saveCursor(latestCursor)

	if len(changedFiles) == 0 {
		log.Println("No files changed since last sync")
		return nil
	}

	log.Printf("Found %d changed files", len(changedFiles))

	basePath := m.config.Sync.LocalBasePath

	for _, file := range changedFiles {
		relativePath := strings.TrimPrefix(file.Path, m.config.Sync.DropboxFolder)
		relativePath = strings.TrimPrefix(relativePath, "/")

		localPath := filepath.Join(basePath, relativePath)

		log.Printf("Syncing changed file: %s -> %s", file.Path, localPath)

		if err := m.syncSingleFile(&file, localPath); err != nil {
			log.Printf("Failed to sync changed file %s: %v", file.Path, err)
			continue
		}
	}

	if err := m.runBuildCommand(); err != nil {
		log.Printf("Build command failed: %v", err)
		return err
	}

	if err := m.applyCopyRules(); err != nil {
		log.Printf("Copy rules failed: %v", err)
		return err
	}

	log.Println("Incremental sync completed successfully")
	return nil
}

func (m *Manager) syncSingleFile(fileInfo *dropbox.FileInfo, localPath string) error {
	// Check if file already exists and is up to date
	if _, err := os.Stat(localPath); err == nil {
		if fileInfo.ContentHash != "" {
			match, err := FilesMatch(localPath, fileInfo.ContentHash)
			if err == nil && match {
				log.Printf("File %s already up to date (hash match)", fileInfo.Path)
				return nil
			}
			if err != nil {
				log.Printf("Failed to check content hash for %s: %v, falling back to size check", fileInfo.Path, err)
				if stat, statErr := os.Stat(localPath); statErr == nil {
					if uint64(stat.Size()) == fileInfo.Size {
						log.Printf("File %s size matches, assuming up to date", fileInfo.Path)
						return nil
					}
				}
			} else {
				log.Printf("File %s has different content hash, updating", fileInfo.Path)
			}
		} else {
			if stat, statErr := os.Stat(localPath); statErr == nil {
				if uint64(stat.Size()) == fileInfo.Size {
					log.Printf("File %s size matches and no hash available, assuming up to date", fileInfo.Path)
					return nil
				}
			}
		}
	}

	log.Printf("Downloading file: %s", fileInfo.Path)
	if err := m.client.DownloadFile(fileInfo.Path, localPath); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	//f := db.File{
	//	UserID:      0,
	//	RemotePath:  fileInfo.Path,
	//	LocalPath:   localPath,
	//	Size:        fileInfo.Size,
	//	ContentHash: fileInfo.ContentHash,
	//}

	log.Printf("Downloaded: %s", fileInfo.Path)
	return nil
}

func (m *Manager) runBuildCommand() error {
	if m.config.Build.Command == "" {
		return nil
	}

	log.Printf("Running build command: %s", m.config.Build.Command)

	workingDir := m.config.Build.WorkingDirectory
	if workingDir == "" {
		workingDir = "."
	}

	// Create absolute path
	if !filepath.IsAbs(workingDir) {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		workingDir = filepath.Join(wd, workingDir)
	}

	// Check if directory exists
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		log.Printf("Build working directory does not exist: %s", workingDir)
		return nil
	}

	log.Printf("Build working directory: %s", workingDir)

	parts := strings.Fields(m.config.Build.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty build command")
	}

	cmd := exec.Command("sh", "-c", m.config.Build.Command)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build command failed: %w, output: %s", err, string(output))
	}

	log.Printf("Build command completed successfully")
	log.Printf("Build output: %s", string(output))

	return nil
}

func (m *Manager) applyCopyRules() error {
	if len(m.config.CopyRules) == 0 {
		return nil
	}

	log.Println("Applying copy rules")

	for _, rule := range m.config.CopyRules {
		if err := m.applyCopyRule(&rule); err != nil {
			log.Printf("Failed to apply copy rule %+v: %v", rule, err)
		}
	}

	log.Println("Copy rules applied")
	return nil
}

func (m *Manager) applyCopyRule(rule *config.CopyRule) error {
	log.Printf("Applying copy rule: %+v", rule)

	destPath := rule.Destination
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	recursive := rule.Recursive != nil && *rule.Recursive

	matches, err := filepath.Glob(rule.SourcePattern)
	if err != nil {
		return fmt.Errorf("failed to parse glob pattern: %w", err)
	}

	for _, sourcePath := range matches {
		stat, err := os.Stat(sourcePath)
		if err != nil {
			continue
		}

		if stat.IsDir() && recursive {
			err = m.copyDirectoryRecursive(sourcePath, destPath)
		} else if !stat.IsDir() {
			fileName := filepath.Base(sourcePath)
			destFile := filepath.Join(destPath, fileName)
			err = copyFile(sourcePath, destFile)
		}

		if err != nil {
			log.Printf("Failed to copy %s: %v", sourcePath, err)
		} else {
			log.Printf("Copied: %s -> %s", sourcePath, destPath)
		}
	}

	return nil
}

func (m *Manager) copyDirectoryRecursive(source, dest string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (m *Manager) removeDeletedFiles(basePath string, expectedFiles map[string]bool) error {
	log.Println("Checking for deleted files to remove")

	var filesToRemove []string

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			filesToRemove = append(filesToRemove, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	removedCount := 0
	for _, localFile := range filesToRemove {
		if !expectedFiles[localFile] {
			log.Printf("Removing deleted file: %s", localFile)
			if err := os.Remove(localFile); err != nil {
				log.Printf("Failed to remove file %s: %v", localFile, err)
			} else {
				removedCount++
			}
		}
	}

	// Remove empty directories
	m.removeEmptyDirectories(basePath)

	if removedCount > 0 {
		log.Printf("Removed %d deleted files", removedCount)
	}

	return nil
}

func (m *Manager) removeEmptyDirectories(basePath string) {
	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == basePath {
			return err
		}

		// Check if directory is empty
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove empty directory %s: %v", path, err)
			} else {
				log.Printf("Removed empty directory: %s", path)
			}
		}

		return nil
	})
}

func (m *Manager) cursorFilePath() string {
	return filepath.Join(m.config.Sync.LocalBasePath, ".blogsync_cursor")
}

func (m *Manager) loadCursor() (string, error) {
	//data, err := os.ReadFile(m.cursorFilePath())
	//if err != nil {
	//	return "", err
	//}
	//return string(data), nil
	var cursor db.SyncCursor
	if err := m.db.First(&cursor, "user_id = ?", 1).Error; err != nil {
		return "", err
	}
	return cursor.Cursor, nil
}

func (m *Manager) saveCursor(cursor string) error {
	//return os.WriteFile(m.cursorFilePath(), []byte(cursor), 0644)
	var syncCursor db.SyncCursor
	if err := m.db.First(&syncCursor, "user_id = ?", 1).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			syncCursor = db.SyncCursor{
				UserID: 1,
				Cursor: cursor,
			}
			if err := m.db.Create(&syncCursor).Error; err != nil {
				return fmt.Errorf("failed to create cursor in database: %w", err)
			}
			return nil
		}
	}
	syncCursor.Cursor = cursor
	if err := m.db.Save(&syncCursor).Error; err != nil {
		return fmt.Errorf("failed to update cursor in database: %w", err)
	}
	return nil
}

func (m *Manager) ExtractZip(zipPath, extractTo string) (int, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	if err := os.MkdirAll(extractTo, 0755); err != nil {
		return 0, err
	}

	extractedCount := 0

	for _, file := range reader.File {
		path := filepath.Join(extractTo, file.Name)

		// Ensure the path is within the extract directory
		if !strings.HasPrefix(path, filepath.Clean(extractTo)+string(os.PathSeparator)) {
			return extractedCount, fmt.Errorf("invalid file path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.FileInfo().Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return extractedCount, err
		}

		fileReader, err := file.Open()
		if err != nil {
			return extractedCount, err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
		if err != nil {
			return extractedCount, err
		}
		defer targetFile.Close()

		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return extractedCount, err
		}

		// persist to db
		f := db.File{
			UserID:    1,
			LocalPath: file.Name,
			Size:      uint64(file.FileInfo().Size()),
		}

		extractedCount++

		// get record with user id and localpath
		existingFile := &db.File{}
		tx := m.db.Where("user_id = ? AND local_path = ?", f.UserID, f.LocalPath).First(existingFile)
		if err := tx.Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				log.Printf("Failed to query file %s from database: %v", path, err)
			} else {
				// create
				log.Printf("File %s not found in database, will create new record", path)
				if err := m.db.Create(&f).Error; err != nil {
					log.Printf("Failed to create file %s in database: %v", path, err)
				} else {
					log.Printf("Created file %s in database", path)
				}
				continue
			}
		}
		if tx.RowsAffected > 0 {
			log.Printf("File %s already exists in database, will update size if changed", path)
			existingFile.Size = f.Size
			if err := m.db.Save(existingFile).Error; err != nil {
				log.Printf("Failed to update file %s in database: %v", path, err)
			} else {
				log.Printf("Updated file %s in database", path)
			}
			continue
		}

	}

	return extractedCount, nil
}
