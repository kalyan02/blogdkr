package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveRelative(findActualPathFor string, inRelationTo string) (string, error) {
	confDir := inRelationTo

	if filepath.IsAbs(findActualPathFor) {
		return findActualPathFor, nil
	}

	if fi, err := os.Stat(inRelationTo); err == nil && fi.IsDir() {
		// configPath points to a directory already
	} else {
		// configPath points to a file or does not exist yet
		confDir = filepath.Dir(inRelationTo)
	}

	// resolve symlinks in config path
	if resolved, err := filepath.EvalSymlinks(confDir); err == nil {
		confDir = resolved
	}

	joined := filepath.Join(confDir, findActualPathFor)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return filepath.Clean(abs), nil
}
