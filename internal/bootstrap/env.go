package bootstrap

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/kun4399/BiliShareMall/internal/domain"
	. "github.com/kun4399/BiliShareMall/internal/util"
	"github.com/rs/zerolog/log"
)

const (
	DefaultAppName  = "BiliShareMall"
	DefaultHTTPAddr = ":3761"
)

type InitOptions struct {
	RuntimeAssets fs.FS
}

func InitEnv(opts InitOptions) {
	exePath, err := os.Executable()
	if err != nil {
		log.Error().Err(err).Msg("Init")
		return
	}

	exeDir := filepath.Dir(exePath)
	cwd, _ := os.Getwd()

	Env.AppName = strings.TrimSpace(os.Getenv("BSM_APP_NAME"))
	if Env.AppName == "" {
		Env.AppName = DefaultAppName
	}
	Env.BasePath = resolveBasePath(exeDir, cwd)
	Env.DataPath = resolveDataPath(Env.AppName)
	Env.FromTaskSch = false

	for _, v := range os.Args {
		if v == "tasksch" {
			Env.FromTaskSch = true
			break
		}
	}

	if err = os.MkdirAll(Env.DataPath, 0o755); err != nil {
		log.Panic().Err(err).Str("dataPath", Env.DataPath).Msg("Create data dir failed")
	}

	if opts.RuntimeAssets != nil {
		if err = SyncEmbeddedDir(opts.RuntimeAssets, "dict", Env.DataPath); err != nil {
			log.Panic().Err(err).Msg("Sync runtime dict assets failed")
		}
	}
}

func HTTPAddr() string {
	if value := strings.TrimSpace(os.Getenv("BSM_HTTP_ADDR")); value != "" {
		return value
	}
	return DefaultHTTPAddr
}

func resolveBasePath(exeDir, cwd string) string {
	if value := strings.TrimSpace(os.Getenv("BSM_BASE_PATH")); value != "" {
		return filepath.Clean(value)
	}

	if looksLikeProjectRoot(cwd) {
		return filepath.Clean(cwd)
	}

	return filepath.Clean(exeDir)
}

func resolveDataPath(appName string) string {
	if value := strings.TrimSpace(os.Getenv("BSM_DATA_DIR")); value != "" {
		return filepath.Clean(value)
	}

	if Env.OS == "darwin" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			log.Panic().Err(homeErr).Msg("Resolve user home failed")
		}
		return filepath.Join(home, "Library", "Application Support", appName)
	}

	if Env.OS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				log.Panic().Err(homeErr).Msg("Resolve user home failed")
			}
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, appName)
	}

	if Env.OS == "linux" {
		if xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, appName)
		}
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			log.Panic().Err(homeErr).Msg("Resolve user home failed")
		}
		return filepath.Join(home, ".local", "share", appName)
	}

	log.Panic().Str("os", Env.OS).Msg("System not support")
	return ""
}

func looksLikeProjectRoot(path string) bool {
	if path == "" {
		return false
	}

	required := []string{
		filepath.Join(path, "dict", "init.sql"),
		filepath.Join(path, "frontend"),
	}
	for _, candidate := range required {
		if _, err := os.Stat(candidate); err != nil {
			return false
		}
	}
	return true
}
