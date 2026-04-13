package main

import (
	"embed"
	app "github.com/kun4399/BiliShareMall/internal/app"
	"github.com/kun4399/BiliShareMall/internal/bootstrap"
	. "github.com/kun4399/BiliShareMall/internal/util"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed all:dict
var runtimeAssets embed.FS

func main() {
	// Create an instance of the newApp structure
	bootstrap.InitEnv(bootstrap.InitOptions{RuntimeAssets: runtimeAssets})
	newApp := app.NewApp()
	err := FileLogger()
	if err != nil {
		panic(err)
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
