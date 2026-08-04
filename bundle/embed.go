// Package bundledata exposes small runtime resources that must remain
// available even when a platform packager omits external bundle files.
package bundledata

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed skills/builtin
var builtinSkills embed.FS

// MaterializeBuiltinSkills writes the embedded, app-managed built-in skills to
// target. User skills live in a separate directory and are never touched.
func MaterializeBuiltinSkills(target string) error {
	root, err := fs.Sub(builtinSkills, "skills/builtin")
	if err != nil {
		return err
	}
	return fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		destination := filepath.Join(target, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
}
