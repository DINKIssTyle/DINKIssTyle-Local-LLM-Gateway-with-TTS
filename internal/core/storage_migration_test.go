package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateStorageRootMovesLegacyLayout(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir failed for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write failed for %s: %v", rel, err)
		}
	}

	mustWrite("localhost.crt", "crt")
	mustWrite("localhost.key", "key")
	mustWrite(filepath.Join("memory", "default", "memory.db"), "db")
	mustWrite("dictionary_ko.txt", "hello, 안녕")
	mustWrite("Dictionary_editor.py", "print('editor')")
	mustWrite(filepath.Join("assets", "onnx", "vocoder.onnx"), "onnx")
	mustWrite(filepath.Join("assets", "voice_styles", "M1.json"), "{}")
	mustWrite(filepath.Join("assets", "LICENSE"), "license")
	mustWrite(filepath.Join("models", "embeddings", "multilingual-e5-small", "model.onnx"), "embedding")
	mustWrite(filepath.Join("onnxruntime", "libonnxruntime.so"), "runtime")

	report := &storageMigrationReport{}
	if err := migrateStorageRoot(root, report); err != nil {
		t.Fatalf("migrateStorageRoot failed: %v", err)
	}
	cleanupLegacyFolders(root)

	assertExists := func(rel string) {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}

	assertMissing := func(rel string) {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be missing, stat err=%v", rel, err)
		}
	}

	assertExists(filepath.Join("cert", "localhost.crt"))
	assertExists(filepath.Join("cert", "localhost.key"))
	assertExists(filepath.Join("memory", "memory.db"))
	assertExists(filepath.Join("dictionary", "dictionary_ko.txt"))
	assertExists(filepath.Join("dictionary", "Dictionary_editor.py"))
	assertExists(filepath.Join("assets", "tts", "supertonic2", "onnx", "vocoder.onnx"))
	assertExists(filepath.Join("assets", "tts", "supertonic2", "voice_styles", "M1.json"))
	assertExists(filepath.Join("assets", "tts", "supertonic2", "LICENSE"))
	assertExists(filepath.Join("assets", "embeddings", "multilingual-e5-small", "model.onnx"))
	assertExists(filepath.Join("assets", "runtime", "onnxruntime", "libonnxruntime.so"))

	assertMissing("localhost.crt")
	assertMissing(filepath.Join("memory", "default", "memory.db"))
	assertMissing(filepath.Join("memory", "default"))
	assertMissing("dictionary_ko.txt")
	assertMissing(filepath.Join("models", "embeddings", "multilingual-e5-small", "model.onnx"))
	assertMissing(filepath.Join("models", "embeddings"))
	assertMissing(filepath.Join("models"))
	assertMissing(filepath.Join("onnxruntime", "libonnxruntime.so"))
	assertMissing(filepath.Join("onnxruntime"))

	if len(report.moved) == 0 {
		t.Fatalf("expected migration report to record moved items")
	}
}

func TestMigrateStorageRootReplacesExistingDestination(t *testing.T) {
	root := t.TempDir()

	legacy := filepath.Join(root, "dictionary_ko.txt")
	dest := filepath.Join(root, "dictionary", "dictionary_ko.txt")
	if err := os.WriteFile(legacy, []byte("legacy"), 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := os.WriteFile(dest, []byte("new"), 0644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	report := &storageMigrationReport{}
	if err := migrateLegacyDictionaryFiles(root, report); err != nil {
		t.Fatalf("migrateLegacyDictionaryFiles failed: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "legacy" {
		t.Fatalf("expected destination to be replaced by legacy content, got %q", string(data))
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("expected legacy source to be removed, stat err=%v", err)
	}
	if len(report.replaced) != 1 {
		t.Fatalf("expected one replaced item, got %d", len(report.replaced))
	}
}

func TestCleanupLegacyFoldersRemovesEffectivelyEmptyLegacyDirectories(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir failed for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write failed for %s: %v", rel, err)
		}
	}

	mustWrite(filepath.Join("memory", "default", ".DS_Store"), "junk")
	mustWrite(filepath.Join("models", "embeddings", ".DS_Store"), "junk")
	mustWrite(filepath.Join("assets", "onnx", ".DS_Store"), "junk")

	cleanupLegacyFolders(root)

	assertMissing := func(rel string) {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be missing, stat err=%v", rel, err)
		}
	}

	assertMissing(filepath.Join("memory", "default"))
	assertMissing(filepath.Join("models"))
	assertMissing(filepath.Join("assets", "onnx"))
}
