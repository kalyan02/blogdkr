package contentstuff

import (
	"path/filepath"
	"strings"
)

func IsPrivate(sc *ContentStuff, fd FileDetail) bool {
	page := NewPageFromFileDetail(&fd)
	if page.IsPrivate() {
		return true
	}

	// check parent index
	if strings.Contains(page.Slug(), "/") {
		parentDir := filepath.Dir(page.Slug())
		parentIdxPath := filepath.Join(parentDir, "index.md")
		if parentIdxFile, ok := sc.DoPath(parentIdxPath); ok {
			parentIdxPage := NewPageFromFileDetail(&parentIdxFile)
			if parentIdxPage.IsPrivate() {

				return true
			}
		}
	}
	return false
}
