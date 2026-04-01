package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"parser-svc/internal/config"
)

type FileSet struct {
	JSONFiles    []string
	IgnoredFiles []string
}

func DiscoverFiles(mode string, manifestPath string) (FileSet, error) {
	normalizedMode := strings.ToLower(strings.TrimSpace(mode))
	normalizedPath := strings.TrimSpace(manifestPath)

	if normalizedPath == "" {
		return FileSet{}, fmt.Errorf("manifest path is required")
	}

	switch normalizedMode {
	case config.ManifestModeFile:
		return discoverFileMode(normalizedPath)
	case config.ManifestModeDirectory:
		return discoverDirectoryMode(normalizedPath)
	default:
		return FileSet{}, fmt.Errorf("unsupported manifest mode: %s", mode)
	}
}

func LoadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	return data, nil
}

func discoverFileMode(path string) (FileSet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileSet{}, fmt.Errorf("stat manifest file: %w", err)
	}
	if info.IsDir() {
		return FileSet{}, fmt.Errorf("manifest file mode expects file, got directory: %s", path)
	}

	if isJSONFile(path) {
		return FileSet{JSONFiles: []string{path}}, nil
	}

	return FileSet{IgnoredFiles: []string{path}}, nil
}

func discoverDirectoryMode(path string) (FileSet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileSet{}, fmt.Errorf("stat manifest directory: %w", err)
	}
	if !info.IsDir() {
		return FileSet{}, fmt.Errorf("manifest directory mode expects directory, got file: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return FileSet{}, fmt.Errorf("read manifest directory: %w", err)
	}

	result := FileSet{
		JSONFiles:    make([]string, 0),
		IgnoredFiles: make([]string, 0),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		if isJSONFile(entry.Name()) {
			result.JSONFiles = append(result.JSONFiles, fullPath)
			continue
		}

		result.IgnoredFiles = append(result.IgnoredFiles, fullPath)
	}

	sort.Strings(result.JSONFiles)
	sort.Strings(result.IgnoredFiles)

	return result, nil
}

func isJSONFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}
