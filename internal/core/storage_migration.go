package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"dinkisstyle-chat/internal/mcp"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type storageMigrationReport struct {
	moved    []string
	replaced []string
}

func (r *storageMigrationReport) addMoved(format string, args ...interface{}) {
	r.moved = append(r.moved, fmt.Sprintf(format, args...))
}

func (r *storageMigrationReport) addReplaced(format string, args ...interface{}) {
	r.replaced = append(r.replaced, fmt.Sprintf(format, args...))
}

func (r storageMigrationReport) summary() string {
	if len(r.moved) == 0 && len(r.replaced) == 0 {
		return "No legacy files were found. Storage is already up to date."
	}

	lines := []string{
		fmt.Sprintf("Moved %d item(s).", len(r.moved)),
	}
	if len(r.replaced) > 0 {
		lines = append(lines, fmt.Sprintf("Replaced %d existing item(s) at the new location.", len(r.replaced)))
	}
	if len(r.moved) > 0 {
		lines = append(lines, "", "Moved:")
		lines = append(lines, r.moved...)
	}
	if len(r.replaced) > 0 {
		lines = append(lines, "", "Replaced:")
		lines = append(lines, r.replaced...)
	}
	return strings.Join(lines, "\n")
}

func movePathIfNeeded(src, dst string, report *storageMigrationReport) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if src == dst {
		return nil
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := movePathIfNeeded(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), report); err != nil {
				return err
			}
		}
		_ = os.Remove(src)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		report.addReplaced("%s -> %s", src, dst)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(src, dst); err != nil {
		if err := copyRecursive(src, dst); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil {
			return err
		}
	}

	report.addMoved("%s -> %s", src, dst)
	return nil
}

func moveLegacyCertificates(root string, report *storageMigrationReport) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	destDir := filepath.Join(root, certDirName)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !(strings.HasSuffix(lower, ".crt") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".pem")) {
			continue
		}
		if err := movePathIfNeeded(filepath.Join(root, name), filepath.Join(destDir, name), report); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyDatabase(root string, report *storageMigrationReport) error {
	legacyDir := filepath.Join(root, "memory", legacyMemoryUserDirName)
	for _, name := range []string{defaultMemoryDatabaseName, defaultMemoryDatabaseName + "-wal", defaultMemoryDatabaseName + "-shm"} {
		if err := movePathIfNeeded(filepath.Join(legacyDir, name), filepath.Join(root, "memory", name), report); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyDictionaryFiles(root string, report *storageMigrationReport) error {
	destDir := filepath.Join(root, dictionaryDirName)
	if err := movePathIfNeeded(filepath.Join(root, "Dictionary_editor.py"), filepath.Join(destDir, "Dictionary_editor.py"), report); err != nil {
		return err
	}

	pattern := filepath.Join(root, "dictionary_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := movePathIfNeeded(match, filepath.Join(destDir, filepath.Base(match)), report); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyTTSAssets(root string, report *storageMigrationReport) error {
	legacyAssetsRoot := filepath.Join(root, assetsDirName)
	destRoot := filepath.Join(root, assetsDirName, ttsDirName, supertonic2DirName)

	if err := movePathIfNeeded(filepath.Join(legacyAssetsRoot, legacyTTSOnnxDirName), filepath.Join(destRoot, legacyTTSOnnxDirName), report); err != nil {
		return err
	}
	if err := movePathIfNeeded(filepath.Join(legacyAssetsRoot, legacyTTSVoiceStylesDir), filepath.Join(destRoot, legacyTTSVoiceStylesDir), report); err != nil {
		return err
	}
	if err := movePathIfNeeded(filepath.Join(legacyAssetsRoot, legacyTTSLicenseFileName), filepath.Join(destRoot, legacyTTSLicenseFileName), report); err != nil {
		return err
	}
	return nil
}

func migrateLegacyEmbeddingAssets(root string, report *storageMigrationReport) error {
	return movePathIfNeeded(
		filepath.Join(root, legacyModelsDirName, embeddingsDirName),
		filepath.Join(root, assetsDirName, embeddingsDirName),
		report,
	)
}

func migrateLegacyONNXRuntime(root string, report *storageMigrationReport) error {
	return movePathIfNeeded(
		filepath.Join(root, onnxRuntimeDirName),
		filepath.Join(root, assetsDirName, runtimeDirName, onnxRuntimeDirName),
		report,
	)
}

func migrateStorageRoot(root string, report *storageMigrationReport) error {
	if root == "" || !pathExists(root) {
		return nil
	}
	steps := []func(string, *storageMigrationReport) error{
		moveLegacyCertificates,
		migrateLegacyDatabase,
		migrateLegacyDictionaryFiles,
		migrateLegacyTTSAssets,
		migrateLegacyEmbeddingAssets,
		migrateLegacyONNXRuntime,
	}
	for _, step := range steps {
		if err := step(root, report); err != nil {
			return err
		}
	}
	return nil
}

func detectLegacyStorageNeedsMigration(root string) (bool, string) {
	if runtime.GOOS == "darwin" {
		legacyRoot := getLegacyMacAppDataDir()
		if legacyRoot != "" && legacyRoot != root && pathExists(legacyRoot) {
			return true, "A previous macOS app data folder was found and should be moved into the current storage location."
		}
	}

	checks := []struct {
		path    string
		message string
	}{
		{filepath.Join(root, "memory", legacyMemoryUserDirName, defaultMemoryDatabaseName), "A legacy SQLite database is still stored under memory/default."},
		{filepath.Join(root, legacyModelsDirName, embeddingsDirName), "Legacy embedding files were found in the old models folder."},
		{filepath.Join(root, onnxRuntimeDirName), "Legacy ONNX Runtime files were found in the old root onnxruntime folder."},
		{filepath.Join(root, "Dictionary_editor.py"), "Legacy dictionary files are still stored in the root folder."},
		{filepath.Join(root, "dictionary_ko.txt"), "Legacy dictionary files are still stored in the root folder."},
		{filepath.Join(root, assetsDirName, legacyTTSOnnxDirName), "Legacy TTS assets are still stored in the old assets folder layout."},
		{filepath.Join(root, assetsDirName, legacyTTSVoiceStylesDir), "Legacy TTS voice styles are still stored in the old assets folder layout."},
	}
	for _, check := range checks {
		if pathExists(check.path) {
			return true, check.message
		}
	}
	return false, ""
}

func removeDirIfEmpty(path string) {
	if path == "" {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(path)
}

func removeLegacyPathIfEffectivelyEmpty(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			removeLegacyPathIfEffectivelyEmpty(childPath)
			continue
		}
		if entry.Name() == ".DS_Store" {
			_ = os.Remove(childPath)
		}
	}

	removeDirIfEmpty(path)
}

func cleanupLegacyFolders(root string) {
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, "memory", legacyMemoryUserDirName))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, legacyModelsDirName, embeddingsDirName))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, legacyModelsDirName))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, onnxRuntimeDirName))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, assetsDirName, legacyTTSOnnxDirName))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, assetsDirName, legacyTTSVoiceStylesDir))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, "memory"))
	removeLegacyPathIfEffectivelyEmpty(filepath.Join(root, assetsDirName))
}

func (a *App) RunStorageMigration() (string, error) {
	report := &storageMigrationReport{}
	currentRoot := GetAppDataDir()

	if runtime.GOOS == "darwin" {
		legacyRoot := getLegacyMacAppDataDir()
		if legacyRoot != "" && legacyRoot != currentRoot && pathExists(legacyRoot) {
			if err := movePathIfNeeded(legacyRoot, currentRoot, report); err != nil {
				return "", err
			}
		}
	}

	if err := migrateStorageRoot(currentRoot, report); err != nil {
		return "", err
	}
	cleanupLegacyFolders(currentRoot)
	a.CheckAndSetupPaths()
	if a.authMgr != nil {
		if err := a.reloadUsersFromCurrentStorage(); err != nil {
			return report.summary(), err
		}
	}
	mcp.CloseDB()
	if err := mcp.InitDB(GetMemoryDatabasePath()); err != nil {
		return report.summary(), err
	}

	a.loadConfig()
	applyEmbeddingRuntimeConfig()
	if a.enableTTS && a.CheckAssets() {
		if err := InitTTS(getTTSAssetsDir(), ttsConfig.Threads); err != nil {
			return report.summary(), err
		}
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "assets-ready")
	}

	return report.summary(), nil
}

func (a *App) ConfirmAndRunStorageMigration() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application context is not ready")
	}

	selection, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.QuestionDialog,
		Title:         "Run Storage Migration",
		Message:       "Move legacy files into the new cert, memory, dictionary, and assets folders now?",
		Buttons:       []string{"Run Migration", "Cancel"},
		DefaultButton: "Run Migration",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return "", err
	}
	if selection != "Run Migration" {
		return "", nil
	}

	return a.RunStorageMigration()
}
