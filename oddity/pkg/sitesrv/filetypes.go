package sitesrv

import (
	"path/filepath"
)

func IsStaticFile(path string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}
	// mime types
	staticExtMap := map[string]bool{
		".css":   true,
		".js":    true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".ico":   true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
		".otf":   true,
		".mp4":   true,
		".webm":  true,
		".ogg":   true,
		".mp3":   true,
		".wav":   true,
		".flac":  true,
		".pdf":   true,
		".txt":   true,
		".zip":   true,
	}
	return staticExtMap[ext]
}
