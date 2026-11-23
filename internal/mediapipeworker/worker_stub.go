//go:build !darwin || !arm64 || !cgo

package mediapipeworker

import "fmt"

const Supported = false

func Run(library, model string) error {
	return fmt.Errorf("native MediaPipe is unavailable on this build")
}
