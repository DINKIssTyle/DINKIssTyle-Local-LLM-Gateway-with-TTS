//go:build !darwin

package core

// InstallDockReopenHandler is only needed on macOS.
func InstallDockReopenHandler() {}
