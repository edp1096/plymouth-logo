package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledPreview(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	logo := filepath.Join(dir, "logo.png")
	os.WriteFile(config, []byte("[Daemon]\nTheme = opi-custom-logo\n"), 0644)
	original := sample()
	os.WriteFile(logo, original, 0644)
	got, size, applied := installedPreview(config, logo)
	if !applied || size != 80 || !bytes.Equal(got, original) {
		t.Fatal("installed logo not restored")
	}
	os.WriteFile(config, []byte("[Daemon]\nTheme=bgrt\n"), 0644)
	_, size, applied = installedPreview(config, logo)
	if applied || size != 320 {
		t.Fatal("old custom file mistaken for selected theme")
	}
	os.WriteFile(config, []byte("[Daemon]\nTheme=opi-custom-logo\n"), 0644)
	os.WriteFile(logo, []byte("invalid"), 0644)
	_, _, applied = installedPreview(config, logo)
	if applied {
		t.Fatal("corrupt image accepted")
	}
}

func TestOriginalBackupSelection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"backup-20260922-000000", "backup-20260923-000000"} {
		dir := filepath.Join(root, name)
		os.Mkdir(dir, 0700)
		os.WriteFile(dir+"/initrd.img", []byte("image"), 0600)
		os.WriteFile(dir+"/sha256", []byte("checksum"), 0600)
	}
	os.WriteFile(filepath.Join(root, "backup-20260923-000000/config"), []byte("[Daemon]\nTheme=opi-custom-logo"), 0600)
	got, err := originalBackup(root)
	if err != nil || filepath.Base(got) != "backup-20260922-000000" {
		t.Fatal(got, err)
	}
	os.Mkdir(filepath.Join(root, "backup-20260922-000000/theme"), 0700)
	if _, err = originalBackup(root); err == nil {
		t.Fatal("custom backup treated as original")
	}
}
