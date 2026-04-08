/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

package core

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Downloader handles asset downloading
type Downloader struct {
	BaseURL string
}

type DownloadFileSpec struct {
	URL      string
	DestPath string
	Label    string
}

type DownloadProgress struct {
	CurrentFile     string
	FilesCompleted  int
	FilesTotal      int
	BytesDownloaded int64
	BytesTotal      int64
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

func (d *Downloader) probeContentLength(url string) int64 {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	if resp.ContentLength < 0 {
		return 0
	}
	return resp.ContentLength
}

type downloadProgressWriter struct {
	io.Writer
	onWrite func(int64)
}

func (w *downloadProgressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if n > 0 && w.onWrite != nil {
		w.onWrite(int64(n))
	}
	return n, err
}

func (d *Downloader) DownloadFiles(specs []DownloadFileSpec, onProgress func(DownloadProgress)) error {
	if len(specs) == 0 {
		return nil
	}

	var totalBytes int64
	for _, spec := range specs {
		totalBytes += d.probeContentLength(spec.URL)
	}

	var downloadedBytes int64
	for i, spec := range specs {
		if err := os.MkdirAll(filepath.Dir(spec.DestPath), 0755); err != nil {
			return err
		}

		resp, err := http.Get(spec.URL)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("failed to download file: %s, status code: %d", spec.URL, resp.StatusCode)
		}

		if resp.ContentLength > 0 && totalBytes == 0 {
			totalBytes += resp.ContentLength
		}

		out, err := os.Create(spec.DestPath)
		if err != nil {
			resp.Body.Close()
			return err
		}

		var fileBytes int64
		writer := &downloadProgressWriter{
			Writer: out,
			onWrite: func(delta int64) {
				fileBytes += delta
				if onProgress != nil {
					onProgress(DownloadProgress{
						CurrentFile:     spec.Label,
						FilesCompleted:  i,
						FilesTotal:      len(specs),
						BytesDownloaded: downloadedBytes + fileBytes,
						BytesTotal:      totalBytes,
					})
				}
			},
		}

		_, copyErr := io.Copy(writer, resp.Body)
		closeErr := out.Close()
		resp.Body.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}

		downloadedBytes += fileBytes
		if onProgress != nil {
			onProgress(DownloadProgress{
				CurrentFile:     spec.Label,
				FilesCompleted:  i + 1,
				FilesTotal:      len(specs),
				BytesDownloaded: downloadedBytes,
				BytesTotal:      totalBytes,
			})
		}
	}

	return nil
}

// DownloadAssets downloads all required ONNX assets
func (d *Downloader) DownloadAssets(assetsDir string) error {
	files := []DownloadFileSpec{
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/duration_predictor.onnx"), DestPath: filepath.Join(assetsDir, "onnx", "duration_predictor.onnx"), Label: "duration_predictor.onnx"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/text_encoder.onnx"), DestPath: filepath.Join(assetsDir, "onnx", "text_encoder.onnx"), Label: "text_encoder.onnx"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/vector_estimator.onnx"), DestPath: filepath.Join(assetsDir, "onnx", "vector_estimator.onnx"), Label: "vector_estimator.onnx"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/vocoder.onnx"), DestPath: filepath.Join(assetsDir, "onnx", "vocoder.onnx"), Label: "vocoder.onnx"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/tts.json"), DestPath: filepath.Join(assetsDir, "onnx", "tts.json"), Label: "tts.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "onnx/unicode_indexer.json"), DestPath: filepath.Join(assetsDir, "onnx", "unicode_indexer.json"), Label: "unicode_indexer.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "LICENSE"), DestPath: filepath.Join(assetsDir, "LICENSE"), Label: "LICENSE"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/M1.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "M1.json"), Label: "M1.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/M2.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "M2.json"), Label: "M2.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/M3.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "M3.json"), Label: "M3.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/M4.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "M4.json"), Label: "M4.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/M5.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "M5.json"), Label: "M5.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/F1.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "F1.json"), Label: "F1.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/F2.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "F2.json"), Label: "F2.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/F3.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "F3.json"), Label: "F3.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/F4.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "F4.json"), Label: "F4.json"},
		{URL: fmt.Sprintf("%s/%s", d.BaseURL, "voice_styles/F5.json"), DestPath: filepath.Join(assetsDir, "voice_styles", "F5.json"), Label: "F5.json"},
	}
	return d.DownloadFiles(files, nil)
}
