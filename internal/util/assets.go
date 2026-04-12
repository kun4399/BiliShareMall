package util

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SyncEmbeddedDir syncs embedded files from srcDir to dstRoot/srcDir.
func SyncEmbeddedDir(assets fs.FS, srcDir string, dstRoot string) error {
	return fs.WalkDir(assets, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(dstRoot, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}

		mode := fs.FileMode(0o644)
		if strings.HasSuffix(path, ".dylib") || strings.HasSuffix(path, ".dll") {
			mode = 0o755
		}

		old, err := os.ReadFile(target)
		if err == nil && bytes.Equal(old, content) {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, mode)
	})
}
