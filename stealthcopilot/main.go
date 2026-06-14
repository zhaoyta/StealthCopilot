package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "StealthCopilot",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Mac: &mac.Options{
			ContentProtection: contentProtectionEnabled(),
		},
		Windows: &windows.Options{
			ContentProtection: contentProtectionEnabled(),
		},
		// 通过 app 字段将各服务暴露给前端：
		// app.ConfigSvc → window.go.config.Service.*
		// app.ResumeSvc → window.go.resume.Service.*
		// app.SystemSvc → window.go.system.Service.*
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func contentProtectionEnabled() bool {
	return os.Getenv("SC_DISABLE_CONTENT_PROTECTION") != "1"
}
