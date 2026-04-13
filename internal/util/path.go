package util

import (
	"github.com/kun4399/BiliShareMall/internal/domain"
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
		dataDict := filepath.Join(domain.Env.DataPath, "dict")
		if pathExists(dataDict) {
			return dataDict
		}
		return filepath.Join(domain.Env.BasePath, "dict")
	}
	if strings.HasPrefix(path, dictPrefix) {
		dataFile := filepath.Join(domain.Env.DataPath, "dict", strings.TrimPrefix(path, dictPrefix))
		if pathExists(dataFile) {
			return dataFile
		}
		return filepath.Join(domain.Env.BasePath, path)
	}
	return filepath.Clean(filepath.Join(domain.Env.BasePath, path))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
