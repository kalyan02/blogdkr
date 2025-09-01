package contentstuff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"oddity/pkg/config"
)

type ContentStuff struct {
	FileName    map[string]FileDetail
	SlugFileMap map[string]FileDetail
	Config      *config.Config
	DBHandle    *gorm.DB
}

func (c *ContentStuff) AllFiles() []FileDetail {
	var fds []FileDetail
	for _, fd := range c.FileName {
		fds = append(fds, fd)
	}
	return fds
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

func NewContentStuff(config *config.Config) *ContentStuff {
	return &ContentStuff{
		Config:      config,
		FileName:    make(map[string]FileDetail),
		SlugFileMap: make(map[string]FileDetail),
	}
}

func (c *ContentStuff) LoadContent() error {
	// DB Connect
	db, err := sqliteConnect(c.Config.Content.SidecarDB)
	if err != nil {
		return fmt.Errorf("error connecting to sqlite db: %v", err)
	}
	c.DBHandle = db

	err = c.DBHandle.AutoMigrate(&PostHistory{})
	if err != nil {
		return fmt.Errorf("error migrating sqlite db: %v", err)
	}

	// traverse the directory c.Config.ContentDir
	err = filepath.Walk(c.Config.Content.ContentDir, c.scanContentPath)
	if err != nil {
		return fmt.Errorf("error walking content dir: %v", err)
	}

	err2 := c.initializeDBHistory()
	if err2 != nil {
		return err2
	}

	return nil
}

func (c *ContentStuff) initializeDBHistory() error {
	// now load from db and create records there if not exists
	var fds []PostHistory
	result := c.DBHandle.Find(&fds)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("error loading file details from db: %v", result.Error)
	}

	var needsCreate []FileDetail
	var existingSlugs = map[string]bool{}
	for _, ph := range fds {
		existingSlugs[ph.FileName] = true
		existingSlugs[ph.FullSlug] = true
	}
	for _, fd := range c.FileName {
		if fd.FileType == FileTypeDirectory {
			continue
		}
		need := true
		if _, ok := existingSlugs[fd.FileName]; ok {
			need = false
		} else if _, ok := existingSlugs[NewPageFromFileDetail(&fd).Slug()]; ok {
			need = false
		}

		if need {
			needsCreate = append(needsCreate, fd)
		}
	}

	// create records for needsCreate
	logrus.Infof("Creating %d post history records", len(needsCreate))
	for _, fd := range needsCreate {
		if fd.FileType == FileTypeDirectory {
			continue
		}
		pg := NewPageFromFileDetail(&fd)
		if fd.ParsedContent == nil {
			panic(fmt.Sprintf("parsed content is nil for %s", fd.FileName))
		}
		rawContent, err := fd.ParsedContent.ToMarkdown()
		if err != nil {
			logrus.Errorf("error converting to markdown for %s: %v", fd.FileName, err)
			continue
		}
		ph := PostHistory{
			FileName: fd.FileName,
			FullSlug: pg.Slug(),
			Title:    pg.Title(),
			HTML:     string(fd.ParsedContent.HTML),
			Content:  rawContent,
		}
		if err := c.DBHandle.Create(&ph).Error; err != nil {
			logrus.Errorf("error creating post history for %s: %v", ph.FullSlug, err)
		} else {
			logrus.Infof("created post history for %s", ph.FullSlug)
		}
	}
	return nil
}

func (c *ContentStuff) WatchContentChanges() (chan bool, error) {
	// setup inotify on c.Config.ContentDir
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating watcher: %v", err)
	}

	absContentDir, err := filepath.Abs(c.Config.Content.ContentDir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %v", err)
	}
	_ = absContentDir

	done := make(chan bool)
	go func() {
		defer watcher.Close()

		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				logrus.Infof("event: %v", event)
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					logrus.Infof("modified file: %s", event.Name)
					// reload this file
					err := c.scanContentPath(event.Name, nil, nil)
					if err != nil {
						logrus.Errorf("error reloading file %s: %v", event.Name, err)
					} else {
						logrus.Infof("reloaded file: %s", event.Name)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logrus.Errorf("error: %v", err)
			}
		}
	}()

	err = watcher.Add(c.Config.Content.ContentDir)
	if err != nil {
		return nil, fmt.Errorf("error adding watcher: %v", err)
	}

	return done, nil
}

func (c *ContentStuff) RefreshContent(path string) error {
	path = filepath.Join(c.Config.Content.ContentDir, path)
	return c.scanContentPath(path, nil, nil)
}

func (c *ContentStuff) scanContentPath(path string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(c.Config.Content.ContentDir, path)
	if err != nil {
		return err
	}

	// if info is nil then os.Stat the path
	if info == nil {
		info, err = os.Stat(path)
		if err != nil {
			return err
		}
	}

	//var ctime time.Time
	//if stat, ok := info.Sys().(*syscall.Stat_t); ok {
	//	// convert to time.Time
	//	ctime = time.Unix(int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec))
	//}

	if info.IsDir() {
		c.FileName[relPath] = FileDetail{
			FileName:   relPath,
			FileType:   FileTypeDirectory,
			LoadedAt:   time.Now(),
			ModifiedAt: info.ModTime(),
			CreatedAt:  info.ModTime(),
		}
	}

	if !info.IsDir() && (filepath.Ext(path) == ".md" || filepath.Ext(path) == ".html") {

		// if it already exists and modtime is same then skip
		if existing, ok := c.FileName[relPath]; ok {
			if existing.ModifiedAt.Equal(info.ModTime()) {
				return nil
			}
		}

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
			CreatedAt: info.ModTime(),
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
}

func (c *ContentStuff) GetHistory(path string) []PostHistory {
	var histories []PostHistory
	result := c.DBHandle.Where("file_name = ? or full_slug = ?", path, path).Order("created DESC").Find(&histories)
	if result.Error != nil {
		logrus.Errorf("error getting history for %s: %v", path, result.Error)
		return nil
	}
	return histories
}

func (c *ContentStuff) PersistHistory() error { return nil }

// WriteFile will simply create folders and write the file and is not content dir aware
func (c *ContentStuff) WriteFile(targetFile string, content string) error {
	targetFileDir := filepath.Dir(targetFile)
	if _, err := os.Stat(targetFileDir); os.IsNotExist(err) {
		err = os.MkdirAll(targetFileDir, 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
	}

	err := os.WriteFile(targetFile, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	return nil
}

func (c *ContentStuff) ReadContentFile(fileName string) (string, error) {
	targetFile := filepath.Join(c.Config.Content.ContentDir, fileName)
	content, err := os.ReadFile(targetFile)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}
	return string(content), nil
}

// WriteContentFile will resolve the content path to the directory before writing
func (c *ContentStuff) WriteContentFile(fileName string, content string) error {
	targetFile := filepath.Join(c.Config.Content.ContentDir, fileName)

	c.WriteContentFileHistory(fileName, content)

	return c.WriteFile(targetFile, content)
}

func (c *ContentStuff) WriteContentFileHistory(fileName string, content string) {
	if strings.HasSuffix(fileName, ".md") {
		if fd, ok := c.DoPath(fileName); ok {
			pg := NewPageFromFileDetail(&fd)
			if pg != nil && pg.File.ParsedContent != nil {
				ph := PostHistory{
					FileName: fileName,
					FullSlug: pg.Slug(),
					Title:    pg.Title(),
					HTML:     string(pg.File.ParsedContent.HTML),
					Content:  content,
				}
				if err := c.DBHandle.Create(&ph).Error; err != nil {
					logrus.Errorf("error creating post history for %s: %v", ph.FullSlug, err)
				} else {
					logrus.Infof("created post history for %s", ph.FullSlug)
				}
			}
		}
	}
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

func SaveFileDetail(sc *ContentStuff, wc *Wire, fd *FileDetail) error {
	if fd.FileName == "" {
		return fmt.Errorf("file name is empty")
	}
	if fd.FileType == FileTypeMarkdown {
		content, err := fd.ParsedContent.ToMarkdown()
		if err != nil {
			return fmt.Errorf("error converting to markdown: %v", err)
		}

		//targetFile := filepath.Join(sc.Config.Content.ContentDir, fd.FileName)

		err = sc.WriteContentFile(fd.FileName, content)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}

		// refresh the file
		err = sc.RefreshContent(fd.FileName)
		if err != nil {
			return fmt.Errorf("error refreshing content: %v", err)
		}

		err = wc.ScanContentFileForQueries(fd.FileName)
		if err != nil {
			return fmt.Errorf("error scanning content file for queries: %v", err)
		}

		err = wc.NotifyFileChanged(fd.FileName)
		if err != nil {
			return fmt.Errorf("error notifying file %s changed: %v", fd.FileName, err)
		}

		// refresh the dir
		err = sc.RefreshContent(filepath.Dir(fd.FileName))
		if err != nil {
			return fmt.Errorf("error refreshing content: %v", err)
		}

		// refresh the file
		err = sc.RefreshContent(fd.FileName)
		if err != nil {
			return fmt.Errorf("error refreshing content: %v", err)
		}

		//targetFileDir := filepath.Dir(targetFile)
		//indexPaths := []string{
		//	filepath.Join(targetFileDir, "index.md"),
		//	filepath.Join(targetFileDir, "index.html"),
		//}

		err = wc.TriggerDependencyUpdates(fd.FileName)
		if err != nil {
			logrus.Errorf("error notifying file change for %s: %v", fd.FileName, err)
		}

		for _, ip := range wc.FindDependencies(fd.FileName) {
			targetFile := filepath.Join(sc.Config.Content.ContentDir, ip)
			if _, err := os.Stat(targetFile); err == nil {
				relativeIP, err := filepath.Rel(sc.Config.Content.ContentDir, targetFile)
				if err != nil {
					logrus.Errorf("error getting relative path for %s: %v", ip, err)
					continue
				}
				err = sc.RefreshContent(relativeIP)
				if err != nil {
					logrus.Errorf("error refreshing content for %s: %v", ip, err)
				}
				err = wc.ScanContentFileForQueries(relativeIP)
				if err != nil {
					logrus.Errorf("error scanning content file for queries %s: %v", ip, err)
				}
			}
		}

		return nil
	}

	if fd.FileType == FileTypeHTML {
		// just write body
		err := sc.WriteContentFile(fd.FileName, string(fd.ParsedContent.HTML))
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}
	}

	return nil
}

type PostHistory struct {
	ID       int64     `gorm:"primaryKey;autoIncrement:true"`
	FileName string    `gorm:"index;not null"`
	FullSlug string    `gorm:"index;not null"`
	Title    string    `gorm:"text"`
	Content  string    `gorm:"text"`
	HTML     string    `gorm:"text"`
	Created  time.Time `gorm:"autoCreateTime"`
	Updated  time.Time `gorm:"autoUpdateTime"`
}

func sqliteConnect(name string) (db *gorm.DB, err error) {
	db, err = gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
