//go:build !darwin || !cgo

package computeruse

import (
	"context"
	"encoding/json"
	"fmt"
)

type unsupportedBackend struct{}

func newNativeBackend() Backend            { return unsupportedBackend{} }
func (unsupportedBackend) Available() bool { return false }
func (unsupportedBackend) Observe(context.Context) (Screen, []byte, error) {
	return Screen{}, nil, fmt.Errorf("computer use requires macOS with cgo")
}
func (unsupportedBackend) Execute(context.Context, string, Request) (json.RawMessage, error) {
	return nil, fmt.Errorf("computer use requires macOS with cgo")
}
