package tmpl

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// Files embeds all template files
//
//go:embed *.html
var Files embed.FS

func Get(name string) ([]byte, error) {
	return Files.ReadFile(name)
}

// Create will write the entire embedded fs structure to the given path

func Create(path string) error {
	return fs.WalkDir(Files, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		targetPath := filepath.Join(path, filePath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, os.ModePerm)
		}

		data, err := Files.ReadFile(filePath)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, data, os.ModePerm)
	})
}
