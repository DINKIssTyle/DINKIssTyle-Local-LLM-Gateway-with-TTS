/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Downloader handles asset downloading
type Downloader struct {
	BaseURL string
}

func NewDownloader() *Downloader {
	return &Downloader{
		BaseURL: "https://huggingface.co/Supertone/supertonic-2/resolve/main",
	}
}

// DownloadFile downloads a file from url to filepath
func (d *Downloader) DownloadFile(url, destPath string) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s, status code: %d", url, resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// DownloadAssets downloads all required ONNX assets
func (d *Downloader) DownloadAssets(assetsDir string) error {
	files := map[string]string{
		"onnx/duration_predictor.onnx": "onnx/duration_predictor.onnx",
		"onnx/text_encoder.onnx":       "onnx/text_encoder.onnx",
		"onnx/vector_estimator.onnx":   "onnx/vector_estimator.onnx",
		"onnx/vocoder.onnx":            "onnx/vocoder.onnx",
		"onnx/tts.json":                "onnx/tts.json",
		"onnx/unicode_indexer.json":    "onnx/unicode_indexer.json",
		"LICENSE":                      "LICENSE",
		// Voice styles
		"voice_styles/M1.json": "voice_styles/M1.json",
		"voice_styles/M2.json": "voice_styles/M2.json",
		"voice_styles/M3.json": "voice_styles/M3.json",
		"voice_styles/M4.json": "voice_styles/M4.json",
		"voice_styles/M5.json": "voice_styles/M5.json",
		"voice_styles/F1.json": "voice_styles/F1.json",
		"voice_styles/F2.json": "voice_styles/F2.json",
		"voice_styles/F3.json": "voice_styles/F3.json",
		"voice_styles/F4.json": "voice_styles/F4.json",
		"voice_styles/F5.json": "voice_styles/F5.json",
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(files))

	for remotePath, localRelPath := range files {
		wg.Add(1)
		go func(remote, local string) {
			defer wg.Done()
			url := fmt.Sprintf("%s/%s", d.BaseURL, remote)
			dest := filepath.Join(assetsDir, local)
			fmt.Printf("Downloading %s to %s...\n", url, dest)
			if err := d.DownloadFile(url, dest); err != nil {
				errChan <- fmt.Errorf("failed to download %s: %w", remote, err)
			}
		}(remotePath, localRelPath)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan // Return first error
	}

	return nil
}
