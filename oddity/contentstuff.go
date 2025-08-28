package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

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
	// traverse the directory c.Config.ContentDir
	err := filepath.Walk(c.Config.ContentDir, c.scanContentPath)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContentStuff) WatchContentChanges() (chan bool, error) {
	// setup inotify on c.Config.ContentDir
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating watcher: %v", err)
	}

	absContentDir, err := filepath.Abs(c.Config.ContentDir)
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

	err = watcher.Add(c.Config.ContentDir)
	if err != nil {
		return nil, fmt.Errorf("error adding watcher: %v", err)
	}

	return done, nil
}

func (c *ContentStuff) RefreshContent(path string) error {
	path = filepath.Join(c.Config.ContentDir, path)
	return c.scanContentPath(path, nil, nil)
}

func (c *ContentStuff) scanContentPath(path string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}
	relPath, err := filepath.Rel(c.Config.ContentDir, path)
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

func SaveFileDetail(cfg *Config, fd *FileDetail) error {
	if fd.FileType == FileTypeMarkdown {
		content, err := fd.ParsedContent.ToMarkdown()
		if err != nil {
			return fmt.Errorf("error converting to markdown: %v", err)
		}

		err = os.WriteFile(filepath.Join(cfg.ContentDir, fd.FileName), []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}
	}

	if fd.FileType == FileTypeHTML {
		// just write body
		err := os.WriteFile(filepath.Join(cfg.ContentDir, fd.FileName), []byte(fd.ParsedContent.HTML), 0644)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}
	}

	return nil
}
