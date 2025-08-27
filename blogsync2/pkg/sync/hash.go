package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

func FilesMatch(localPath, dropboxHash string) (bool, error) {
	if dropboxHash == "" {
		return false, fmt.Errorf("no dropbox hash provided")
	}

	localHash, err := CalculateContentHash(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate local hash: %w", err)
	}

	return localHash == dropboxHash, nil
}

func CalculateContentHash(filePath string) (string, error) {
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

const BlockSize = 4 * 1024 * 1024 // 4MB

// DropboxContentHasher implements the same algorithm that the Dropbox API uses
// for the "content_hash" metadata field.
type DropboxContentHasher struct {
	overallHasher hash.Hash
	blockHasher   hash.Hash
	blockPos      int
}

// NewDropboxContentHasher creates a new instance of the hasher
func NewDropboxContentHasher() *DropboxContentHasher {
	return &DropboxContentHasher{
		overallHasher: sha256.New(),
		blockHasher:   sha256.New(),
		blockPos:      0,
	}
}

// Write implements the io.Writer interface
func (h *DropboxContentHasher) Write(p []byte) (n int, err error) {
	input := p
	totalWritten := 0

	for len(input) > 0 {
		if h.blockPos == BlockSize {
			// Finalize current block and add its hash to overall hasher
			blockHash := h.blockHasher.Sum(nil)
			h.overallHasher.Write(blockHash)
			h.blockHasher = sha256.New()
			h.blockPos = 0
		}

		spaceInBlock := BlockSize - h.blockPos
		toWrite := len(input)
		if toWrite > spaceInBlock {
			toWrite = spaceInBlock
		}

		chunk := input[:toWrite]
		h.blockHasher.Write(chunk)
		h.blockPos += len(chunk)
		totalWritten += len(chunk)
		input = input[toWrite:]
	}

	return totalWritten, nil
}

// Sum returns the final hash. This method finalizes the hash calculation.
func (h *DropboxContentHasher) Sum() []byte {
	// If there's any data in the current block, finalize it
	if h.blockPos > 0 {
		blockHash := h.blockHasher.Sum(nil)
		h.overallHasher.Write(blockHash)
	}
	return h.overallHasher.Sum(nil)
}

// SumHex returns the final hash as a hexadecimal string
func (h *DropboxContentHasher) SumHex() string {
	return hex.EncodeToString(h.Sum())
}

// HashFile computes the Dropbox content hash for a file
func HashFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := NewDropboxContentHasher()
	buf := make([]byte, 4096)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hasher.SumHex(), nil
}

// HashBytes computes the Dropbox content hash for a byte slice
func HashBytes(data []byte) string {
	hasher := NewDropboxContentHasher()
	hasher.Write(data)
	return hasher.SumHex()
}

// HashReader computes the Dropbox content hash for any io.Reader
func HashReader(reader io.Reader) (string, error) {
	hasher := NewDropboxContentHasher()
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hasher.SumHex(), nil
}
