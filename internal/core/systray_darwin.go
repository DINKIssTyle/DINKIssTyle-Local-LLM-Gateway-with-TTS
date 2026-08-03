//go:build darwin

package core

/*
#include <stdlib.h>
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
void DKSTInitStatusItem(const unsigned char *iconBytes, int iconLength);
void DKSTUpdateStatusItem(const char *statusTitle, const char *toggleTitle,
                          const char *showTitle, const char *quitTitle);
void DKSTRemoveStatusItem(void);
*/
import "C"

import "unsafe"

var darwinTrayApp *App

// InitSystemTray adds a native NSStatusItem without replacing Wails'
// NSApplicationDelegate.
func InitSystemTray(app *App, iconData []byte) {
	darwinTrayApp = app
	if len(iconData) == 0 {
		C.DKSTInitStatusItem(nil, 0)
	} else {
		C.DKSTInitStatusItem(
			(*C.uchar)(unsafe.Pointer(&iconData[0])),
			C.int(len(iconData)),
		)
	}
	UpdateTrayServerState(app != nil && app.IsServerRunning())
}

// UpdateTrayServerState refreshes the native macOS status and toggle items.
func UpdateTrayServerState(running bool) {
	language := "ko"
	if darwinTrayApp != nil {
		language = darwinTrayApp.GetServerUILanguage()
	}
	labels := getTrayMenuLabels(language, running)
	status := C.CString(labels.status)
	toggle := C.CString(labels.toggle)
	show := C.CString(labels.show)
	quit := C.CString(labels.quit)
	defer C.free(unsafe.Pointer(status))
	defer C.free(unsafe.Pointer(toggle))
	defer C.free(unsafe.Pointer(show))
	defer C.free(unsafe.Pointer(quit))
	C.DKSTUpdateStatusItem(status, toggle, show, quit)
}

// QuitSystemTray removes the native menu-bar item.
func QuitSystemTray() {
	C.DKSTRemoveStatusItem()
}

//export DKSTTrayShowMainWindow
func DKSTTrayShowMainWindow() {
	if darwinTrayApp != nil {
		darwinTrayApp.ShowMainWindow()
	}
}

//export DKSTTrayToggleServer
func DKSTTrayToggleServer() {
	if darwinTrayApp != nil {
		go darwinTrayApp.toggleServerFromTray()
	}
}

//export DKSTTrayQuit
func DKSTTrayQuit() {
	if darwinTrayApp != nil {
		darwinTrayApp.Quit()
	}
}
