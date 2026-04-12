package util

import (
	"github.com/mikumifa/BiliShareMall/internal/domain"
	"os"
	"path/filepath"
	"strings"
)

func GetPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	path = filepath.Clean(path)
	dataPrefix := "data" + string(os.PathSeparator)
	dictPrefix := "dict" + string(os.PathSeparator)

	if path == "data" {
		return filepath.Clean(domain.Env.DataPath)
	}
	if strings.HasPrefix(path, dataPrefix) {
		return filepath.Join(domain.Env.DataPath, strings.TrimPrefix(path, dataPrefix))
	}
	if path == "dict" {
		return filepath.Join(domain.Env.DataPath, "dict")
	}
	if strings.HasPrefix(path, dictPrefix) {
		return filepath.Join(domain.Env.DataPath, "dict", strings.TrimPrefix(path, dictPrefix))
	}
	return filepath.Clean(filepath.Join(domain.Env.BasePath, path))
}
