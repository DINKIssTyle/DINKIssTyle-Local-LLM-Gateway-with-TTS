//go:build darwin

package core

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
void DKSTSetDockIconVisible(int visible);
*/
import "C"

func setDockIconVisible(visible bool) {
	if visible {
		C.DKSTSetDockIconVisible(1)
		return
	}
	C.DKSTSetDockIconVisible(0)
}
