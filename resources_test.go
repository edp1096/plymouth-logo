package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResourceExportPreservesEditsAndDeletions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := exportResources(bundledAssets, root); err != nil {
		t.Fatal(err)
	}
	css := filepath.Join(root, "assets/custom.txt")
	if err := os.WriteFile(css, []byte("custom style"), 0644); err != nil {
		t.Fatal(err)
	}
	removed := filepath.Join(root, "assets/spinners/pepe")
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}
	if err := exportResources(bundledAssets, root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(css)
	if string(data) != "custom style" {
		t.Fatal("custom resource overwritten")
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatal("deleted item silently restored")
	}
	if exists(filepath.Join(root, "web")) {
		t.Fatal("web UI should not be exported")
	}
	oldAssets, oldDirs, oldCatalog, oldAliases := assets, spinnerDirectories, spinnerCatalog, spinnerAliases
	defer func() {
		assets, spinnerDirectories, spinnerCatalog, spinnerAliases = oldAssets, oldDirs, oldCatalog, oldAliases
	}()
	if err := useResources(root); err != nil {
		t.Fatal(err)
	}
	if _, err := spinnerItem("pepe"); err == nil {
		t.Fatal("removed item still selectable")
	}
	os.MkdirAll(filepath.Join(root, "web"), 0755)
	os.WriteFile(filepath.Join(root, "web/app.js"), []byte("outdated external UI"), 0644)
	web, err := assets.ReadFile("web/app.js")
	embedded, _ := bundledAssets.ReadFile("web/app.js")
	if err != nil || string(web) != string(embedded) {
		t.Fatal("embedded UI was overridden")
	}
	if _, err := assets.ReadFile("../private"); err == nil {
		t.Fatal("path traversal accepted")
	}
}
func TestExternalSpinnerFolderRename(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if err := exportResources(bundledAssets, root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "assets/spinners/beubmi"), filepath.Join(root, "assets/spinners/my-character")); err != nil {
		t.Fatal(err)
	}
	oldAssets, oldDirs, oldCatalog, oldAliases := assets, spinnerDirectories, spinnerCatalog, spinnerAliases
	defer func() {
		assets, spinnerDirectories, spinnerCatalog, spinnerAliases = oldAssets, oldDirs, oldCatalog, oldAliases
	}()
	if err := useResources(root); err != nil {
		t.Fatal(err)
	}
	if spinnerDirectories["beubmi"] != "my-character" {
		t.Fatal("external folder not discovered")
	}
	data, err := assets.ReadFile("assets/spinners/my-character/throbber-0001.png")
	if err != nil || len(data) == 0 {
		t.Fatal("external frame unavailable")
	}
}
