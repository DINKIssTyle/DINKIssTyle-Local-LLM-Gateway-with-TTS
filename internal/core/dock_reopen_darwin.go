//go:build darwin

package core

/*
#cgo LDFLAGS: -framework Cocoa
void DKSTInstallDockReopenHandler(void);
*/
import "C"

// InstallDockReopenHandler restores Wails' hidden main window when the user
// clicks the app's Dock icon. Wails v2 keeps the process alive after the last
// window closes, but its default AppDelegate does not implement reopen.
func InstallDockReopenHandler() {
	C.DKSTInstallDockReopenHandler()
}
