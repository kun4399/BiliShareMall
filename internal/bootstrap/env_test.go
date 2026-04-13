package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/kun4399/BiliShareMall/internal/domain"
)

func TestResolveDataPathUsesBSMDataDirOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BSM_DATA_DIR", filepath.Join(root, "custom-data"))

	originalOS := domain.Env.OS
	domain.Env.OS = "linux"
	t.Cleanup(func() {
		domain.Env.OS = originalOS
	})

	got := resolveDataPath("BiliShareMall")
	want := filepath.Clean(filepath.Join(root, "custom-data"))

	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolveDataPathSupportsLinuxFallback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BSM_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))

	originalOS := domain.Env.OS
	domain.Env.OS = "linux"
	t.Cleanup(func() {
		domain.Env.OS = originalOS
	})

	got := resolveDataPath("BiliShareMall")
	want := filepath.Join(filepath.Clean(filepath.Join(root, "xdg-data")), "BiliShareMall")

	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
