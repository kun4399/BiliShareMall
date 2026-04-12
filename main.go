package main

import (
	"embed"
	app "github.com/mikumifa/BiliShareMall/internal/app"
	. "github.com/mikumifa/BiliShareMall/internal/domain"
	. "github.com/mikumifa/BiliShareMall/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed all:dict
var runtimeAssets embed.FS

func InitEnv() {

	exePath, err := os.Executable()
	if err != nil {
		log.Error().Err(err).Msg("Init")
		return
	}

	for _, v := range os.Args {
		if v == "tasksch" {
			Env.FromTaskSch = true
			break
		}
	}

	Env.BasePath = filepath.Dir(exePath)
	Env.AppName = strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))

	if Env.OS == "darwin" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			log.Panic().Err(homeErr).Msg("Resolve user home failed")
		}
		Env.DataPath = filepath.Join(home, "Library", "Application Support", Env.AppName)
	} else if Env.OS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				log.Panic().Err(homeErr).Msg("Resolve user home failed")
			}
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		Env.DataPath = filepath.Join(localAppData, Env.AppName)
	} else {
		log.Panic().Str("os", Env.OS).Msg("System not support")
	}

	if err = os.MkdirAll(Env.DataPath, 0o755); err != nil {
		log.Panic().Err(err).Str("dataPath", Env.DataPath).Msg("Create data dir failed")
	}
	if err = SyncEmbeddedDir(runtimeAssets, "dict", Env.DataPath); err != nil {
		log.Panic().Err(err).Msg("Sync runtime dict assets failed")
	}
}

func main() {
	// Create an instance of the newApp structure
	InitEnv()
	newApp := app.NewApp()
	log.Info().Msg("Creating newApp")
	err := FileLogger()
	if err != nil {
		log.Panic().Err(err).Msg("Init file logger failed")
	}
	err = wails.Run(&options.App{
		Title:  "BiliShareMall",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: frontendAssets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        newApp.Startup,
		Bind: []interface{}{
			newApp,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
