package core

import "log"

type trayMenuLabels struct {
	status string
	toggle string
	show   string
	quit   string
}

func getTrayMenuLabels(language string, running bool) trayMenuLabels {
	if language == "en" {
		if running {
			return trayMenuLabels{
				status: "Server status: Running",
				toggle: "Stop Server",
				show:   "Open Main Window",
				quit:   "Quit",
			}
		}
		return trayMenuLabels{
			status: "Server status: Stopped",
			toggle: "Start Server",
			show:   "Open Main Window",
			quit:   "Quit",
		}
	}

	if running {
		return trayMenuLabels{
			status: "서버 상태: 작동 중",
			toggle: "서버 중지",
			show:   "메인 창 열기",
			quit:   "종료",
		}
	}
	return trayMenuLabels{
		status: "서버 상태: 중지됨",
		toggle: "서버 시작",
		show:   "메인 창 열기",
		quit:   "종료",
	}
}

// IsServerRunning returns a synchronized snapshot of the server state.
func (a *App) IsServerRunning() bool {
	if a == nil {
		return false
	}
	a.serverMux.Lock()
	defer a.serverMux.Unlock()
	return a.isRunning
}

func (a *App) toggleServerFromTray() {
	if a == nil {
		return
	}

	var err error
	if a.IsServerRunning() {
		err = a.StopServer()
	} else {
		err = a.StartServerWithCurrentConfig()
	}
	if err != nil {
		log.Printf("[TRAY] Failed to toggle server: %v", err)
	}
}
