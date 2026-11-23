package core

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dinkisstyle-chat/internal/mediapipeworker"
)

const (
	mediaPipeWheelURL     = "https://files.pythonhosted.org/packages/e0/7c/e5e1b0fd0a43a8f71db9062c94731a3de196186d392ce0c4417d7923a1a0/mediapipe-1.1.0-py3-none-macosx_11_0_arm64.whl"
	mediaPipeWheelSHA     = "8d262c745a4432c69c47fba664e2f9210acaca0af4eca2ad9fed42db494f3e12"
	mediaPipeWheelLibrary = "mediapipe/tasks/c/libmediapipe.dylib"
)

func mediaPipeLibraryPath() string {
	return filepath.Join(jevRuntimeDir(), "native-1.1.0", "libmediapipe.dylib")
}

func availableMediaPipeLibrary() string {
	if !mediapipeworker.Supported {
		return ""
	}
	if path := mediaPipeLibraryPath(); fileExists(path) {
		return path
	}
	// Reuse the same pinned SDK on existing installations without invoking Python.
	packages := filepath.Join(jevRuntimeDir(), "venv", "lib", "python3.12", "site-packages")
	path := filepath.Join(packages, filepath.FromSlash(mediaPipeWheelLibrary))
	if fileExists(filepath.Join(packages, "mediapipe-1.1.0.dist-info", "METADATA")) && fileExists(path) {
		return path
	}
	return ""
}

func installMediaPipeLibrary(wheel, destination string) error {
	file, err := os.Open(wheel)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, 50<<20)); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != mediaPipeWheelSHA {
		return fmt.Errorf("MediaPipe package checksum mismatch")
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return err
	}
	for _, entry := range archive.File {
		if entry.Name != mediaPipeWheelLibrary {
			continue
		}
		if entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > 160<<20 {
			return fmt.Errorf("invalid MediaPipe library size")
		}
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		output, err := os.CreateTemp(filepath.Dir(destination), ".mediapipe-*.dylib")
		if err != nil {
			return err
		}
		defer os.Remove(output.Name())
		_, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return os.Rename(output.Name(), destination)
	}
	return fmt.Errorf("MediaPipe native library is missing from package")
}
