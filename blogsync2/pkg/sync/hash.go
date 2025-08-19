package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func FilesMatch(localPath, dropboxHash string) (bool, error) {
	if dropboxHash == "" {
		return false, fmt.Errorf("no dropbox hash provided")
	}

	localHash, err := calculateContentHash(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate local hash: %w", err)
	}

	return localHash == dropboxHash, nil
}

func calculateContentHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}