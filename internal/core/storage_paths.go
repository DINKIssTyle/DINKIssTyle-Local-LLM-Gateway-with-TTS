package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	certDirName               = "cert"
	dictionaryDirName         = "dictionary"
	assetsDirName             = "assets"
	ttsDirName                = "tts"
	supertonic2DirName        = "supertonic2"
	embeddingsDirName         = "embeddings"
	runtimeDirName            = "runtime"
	onnxRuntimeDirName        = "onnxruntime"
	legacyModelsDirName       = "models"
	legacyTTSOnnxDirName      = "onnx"
	legacyTTSVoiceStylesDir   = "voice_styles"
	legacyTTSLicenseFileName  = "LICENSE"
	legacyMemoryUserDirName   = "default"
	defaultMemoryDatabaseName = "memory.db"
)

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func joinAppDataPath(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, GetAppDataDir())
	all = append(all, parts...)
	return filepath.Join(all...)
}

func getLegacyMacAppDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "Documents", legacyMacAppDataDirName)
}

func getWritableCertDir() string {
	return joinAppDataPath(certDirName)
}

func getWritableDictionaryDir() string {
	return joinAppDataPath(dictionaryDirName)
}

func getManagedAssetsRootDir() string {
	return joinAppDataPath(assetsDirName)
}

func getWritableTTSAssetsDir() string {
	return joinAppDataPath(assetsDirName, ttsDirName, supertonic2DirName)
}

func getWritableEmbeddingRootDir() string {
	return joinAppDataPath(assetsDirName, embeddingsDirName)
}

func getWritableONNXRuntimeDir() string {
	return joinAppDataPath(assetsDirName, runtimeDirName, onnxRuntimeDirName)
}

func GetMemoryDatabasePath() string {
	return joinAppDataPath("memory", defaultMemoryDatabaseName)
}

func GetModelsRootDir() string {
	return getManagedAssetsRootDir()
}

func getEmbeddingModelInstallDir(modelID string) string {
	modelID = filepath.Clean(modelID)
	if modelID == "." || modelID == "" {
		modelID = "multilingual-e5-small"
	}
	return filepath.Join(getWritableEmbeddingRootDir(), modelID)
}

func getDictionaryFilename(lang string) string {
	if lang == "" {
		lang = "ko"
	}
	return fmt.Sprintf("dictionary_%s.txt", lang)
}

func getWritableDictionaryFilePath(lang string) string {
	return filepath.Join(getWritableDictionaryDir(), getDictionaryFilename(lang))
}

func getDictionarySourcePath(lang string) string {
	filename := getDictionaryFilename(lang)
	candidates := []string{
		getWritableDictionaryFilePath(lang),
		GetResourcePath(filepath.Join(dictionaryDirName, filename)),
		GetResourcePath(filename),
		filepath.Join("bundle", dictionaryDirName, filename),
		filepath.Join("bundle", filename),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

func getDictionaryEditorSourcePath() string {
	candidates := []string{
		filepath.Join(getWritableDictionaryDir(), "Dictionary_editor.py"),
		GetResourcePath(filepath.Join(dictionaryDirName, "Dictionary_editor.py")),
		GetResourcePath("Dictionary_editor.py"),
		filepath.Join("bundle", dictionaryDirName, "Dictionary_editor.py"),
		filepath.Join("bundle", "Dictionary_editor.py"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

func getTTSAssetsDir() string {
	writable := getWritableTTSAssetsDir()
	if fileExists(filepath.Join(writable, legacyTTSOnnxDirName, "vocoder.onnx")) {
		return writable
	}

	primary := GetResourcePath(filepath.Join(assetsDirName, ttsDirName, supertonic2DirName))
	if fileExists(filepath.Join(primary, legacyTTSOnnxDirName, "vocoder.onnx")) {
		return primary
	}

	legacy := GetResourcePath(assetsDirName)
	if fileExists(filepath.Join(legacy, legacyTTSOnnxDirName, "vocoder.onnx")) {
		return legacy
	}

	return primary
}

func getONNXRuntimeLibraryFileName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "linux":
		return "libonnxruntime.so"
	default:
		return "onnxruntime.dll"
	}
}

func getONNXRuntimeDir() string {
	writable := getWritableONNXRuntimeDir()
	if fileExists(filepath.Join(writable, getONNXRuntimeLibraryFileName())) {
		return writable
	}

	primary := GetResourcePath(filepath.Join(assetsDirName, runtimeDirName, onnxRuntimeDirName))
	if fileExists(filepath.Join(primary, getONNXRuntimeLibraryFileName())) {
		return primary
	}

	legacy := GetResourcePath(onnxRuntimeDirName)
	if fileExists(filepath.Join(legacy, getONNXRuntimeLibraryFileName())) {
		return legacy
	}

	return primary
}

func getONNXRuntimeLibraryPath() string {
	libName := getONNXRuntimeLibraryFileName()
	candidates := []string{
		filepath.Join(getONNXRuntimeDir(), libName),
		GetResourcePath(filepath.Join(assetsDirName, runtimeDirName, onnxRuntimeDirName, libName)),
		GetResourcePath(filepath.Join(onnxRuntimeDirName, libName)),
		GetResourcePath(libName),
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, libName),
			filepath.Join(exeDir, onnxRuntimeDirName, libName),
			filepath.Join(exeDir, assetsDirName, runtimeDirName, onnxRuntimeDirName, libName),
		)
	}

	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}

	return candidates[0]
}

func getGoModuleCacheDir() string {
	if value := strings.TrimSpace(os.Getenv("GOMODCACHE")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("GOPATH")); value != "" {
		return filepath.Join(value, "pkg", "mod")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "go", "pkg", "mod")
}

func getONNXRuntimeModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != "github.com/yalue/onnxruntime_go" {
			continue
		}
		version := strings.TrimSpace(dep.Version)
		if version != "" {
			return version
		}
	}
	return ""
}

func getONNXRuntimeLibraryCandidates() []string {
	libName := getONNXRuntimeLibraryFileName()
	candidates := []string{
		filepath.Join(GetResourcePath(filepath.Join(assetsDirName, runtimeDirName, onnxRuntimeDirName)), libName),
		GetResourcePath(filepath.Join(onnxRuntimeDirName, libName)),
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, libName),
			filepath.Join(exeDir, onnxRuntimeDirName, libName),
			filepath.Join(exeDir, assetsDirName, runtimeDirName, onnxRuntimeDirName, libName),
			filepath.Join(exeDir, "..", "Resources", assetsDirName, runtimeDirName, onnxRuntimeDirName, libName),
		)
	}

	// Add CWD based candidates for development
	cwd, err := os.Getwd()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, libName),
			filepath.Join(cwd, onnxRuntimeDirName, libName),
			filepath.Join(cwd, assetsDirName, runtimeDirName, onnxRuntimeDirName, libName),
			filepath.Join(cwd, "bundle", assetsDirName, runtimeDirName, onnxRuntimeDirName, libName),
		)
	}

	modCache := getGoModuleCacheDir()
	if modCache != "" {
		if version := getONNXRuntimeModuleVersion(); version != "" {
			candidates = append(candidates,
				filepath.Join(modCache, "github.com", "yalue", "onnxruntime_go@"+version, "test_data", libName),
			)
		}
		if matches, err := filepath.Glob(filepath.Join(modCache, "github.com", "yalue", "onnxruntime_go@*", "test_data", libName)); err == nil {
			candidates = append(candidates, matches...)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	deduped := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		deduped = append(deduped, candidate)
	}
	return deduped
}

func ensureONNXRuntimeLibraryPresent() error {
	libName := getONNXRuntimeLibraryFileName()
	destDir := getWritableONNXRuntimeDir()
	destPath := filepath.Join(destDir, libName)
	if fileExists(destPath) {
		return nil
	}

	for _, candidate := range getONNXRuntimeLibraryCandidates() {
		if !fileExists(candidate) {
			continue
		}
		if err := copyRecursive(candidate, destPath); err != nil {
			return err
		}
		licenseSrc := filepath.Join(filepath.Dir(candidate), "LICENSE.txt")
		licenseDest := filepath.Join(destDir, "LICENSE.txt")
		if fileExists(licenseSrc) && !fileExists(licenseDest) {
			_ = copyRecursive(licenseSrc, licenseDest)
		}
		return nil
	}

	return fmt.Errorf("onnx runtime library %q was not found in app assets, bundle, executable, or Go module cache", libName)
}
