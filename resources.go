package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type resourceStore interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]fs.DirEntry, error)
}
type diskResources struct{ root string }

func (d diskResources) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid resource path")
	}
	if strings.HasPrefix(name, "web/") {
		return bundledAssets.ReadFile(name)
	}
	return readLimited(filepath.Join(d.root, filepath.FromSlash(name)), 32*1024*1024)
}
func (d diskResources) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid resource path")
	}
	if name == "web" || strings.HasPrefix(name, "web/") {
		return bundledAssets.ReadDir(name)
	}
	return os.ReadDir(filepath.Join(d.root, filepath.FromSlash(name)))
}

func userDataBase() string {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(home, ".local", "share")
}
func defaultDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "data")
}

// Export once. Later starts never restore deleted files or overwrite custom files.
func exportResources(source embed.FS, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".plymouth-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	err = fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if name == "web" {
			return fs.SkipDir
		}
		target := filepath.Join(stage, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := source.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "README.txt"), []byte("Editable Plymouth Logo resources. Changes take effect after restarting the app.\nassets/: logo, spinner items, and base theme. The web UI is embedded in the executable.\nThese files are never overwritten automatically. Back up before editing.\n"), 0644); err != nil {
		return err
	}
	return os.Rename(stage, dest)
}
func useResources(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("resource path is not a directory")
	}
	assets = diskResources{dir}
	var catalogErr error
	func() {
		defer func() {
			if failure := recover(); failure != nil {
				catalogErr = fmt.Errorf("invalid spinner resources: %v", failure)
			}
		}()
		spinnerDirectories, spinnerCatalog, spinnerAliases = loadSpinnerCatalog()
	}()
	if catalogErr != nil {
		return catalogErr
	}
	for _, name := range []string{"web/index.html", "web/app.js", "web/style.css", "assets/default.png", "assets/theme/bgrt.plymouth"} {
		if _, err := assets.ReadFile(name); err != nil {
			return fmt.Errorf("missing resource %s: %w", name, err)
		}
	}
	return nil
}
func quoteDesktop(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, `$`, `\\$`, "`", "\\\\`", "%", "%%").Replace(value) + `"`
}
