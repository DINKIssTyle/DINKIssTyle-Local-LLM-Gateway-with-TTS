package harness

import (
	"context"
	"embed"
	"log"
	"runtime"

	"dinkisstyle-chat/internal/config"
	"dinkisstyle-chat/internal/core"
	"dinkisstyle-chat/internal/mcp"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DesktopAssets groups the embedded resources needed to bootstrap the desktop app.
type DesktopAssets struct {
	Frontend    embed.FS
	TrayIconPNG []byte
	TrayIconICO []byte
}

// Desktop encapsulates the Wails bootstrap flow for the desktop application.
type Desktop struct {
	app    *core.App
	assets DesktopAssets
}

// NewDesktop creates a harness for running the desktop application.
func NewDesktop(assets DesktopAssets) *Desktop {
	core.InitLoggingFilter()
	return &Desktop{
		app:    core.NewApp(assets.Frontend),
		assets: assets,
	}
}

// Run starts the desktop application.
func (d *Desktop) Run() error {
	core.InitSystemTray(d.app, d.trayIcon())
	return wails.Run(d.options())
}

func (d *Desktop) trayIcon() []byte {
	if runtime.GOOS == "windows" {
		return d.assets.TrayIconICO
	}
	return d.assets.TrayIconPNG
}

func (d *Desktop) options() *options.App {
	return &options.App{
		Title:     "DKST LLM Chat Server",
		Width:     config.NormalWindowWidth,
		Height:    config.NormalWindowHeight,
		MinWidth:  config.NormalWindowWidth,
		MinHeight: config.NormalWindowHeight,
		AssetServer: &assetserver.Options{
			Assets: d.assets.Frontend,
		},
		OnStartup: func(ctx context.Context) {
			d.initMemoryDB()
			d.app.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			mcp.CloseDB()
			d.app.Shutdown(ctx)
		},
		HideWindowOnClose: false,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if d.app.IsQuitting {
				return false
			}
			if d.app.GetMinimizeToTray() {
				wruntime.WindowHide(ctx)
				return true
			}
			return false
		},
		Menu: core.CreateAppMenu(d.app),
		Bind: []interface{}{
			d.app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windows.SystemDefault,
			CustomTheme:          nil,
		},
	}
}

func (d *Desktop) initMemoryDB() {
	dbPath := core.GetMemoryDatabasePath()
	if err := mcp.InitDB(dbPath); err != nil {
		log.Printf("Failed to init SQLite: %v", err)
	}
}
