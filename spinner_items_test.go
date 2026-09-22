package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledSpinners(t *testing.T) {
	for id, dir := range spinnerDirectories {
		entry, err := spinnerMetadata(id)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= entry.Frames; i++ {
			data, err := assets.ReadFile(fmt.Sprintf("assets/spinners/%s/throbber-%04d.png", dir, i))
			if err != nil {
				t.Fatal(err)
			}
			im, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if im.Bounds().Dx() != 160 || im.Bounds().Dy() != 160 {
				t.Fatal(id, "invalid dimensions")
			}
			transparent := false
			for y := 0; y < 160 && !transparent; y++ {
				for x := 0; x < 160; x++ {
					_, _, _, a := im.At(x, y).RGBA()
					if a == 0 {
						transparent = true
						break
					}
				}
			}
			if !transparent {
				t.Fatal(id, i, "background is not transparent")
			}
		}
	}
	if _, err := spinnerItem("../../etc"); err == nil {
		t.Fatal("invalid item accepted")
	}
}
func TestSpinnerReplacementAndRestoreMetadata(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "throbber-9999.png"), []byte("stale"), 0644)
	if err := installSpinnerItem(dir, "pepe"); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "throbber-*.png"))
	if len(files) != 60 {
		t.Fatal(len(files))
	}
	if exists(filepath.Join(dir, "throbber-9999.png")) {
		t.Fatal("stale frame retained")
	}
	old := themeDir
	themeDir = dir
	defer func() { themeDir = old }()
	if installedSpinnerItem(true) != "pepe" || installedSpinnerItem(false) != "default" {
		t.Fatal("installed item not read")
	}
	if err := installSpinnerItem(dir, "invalid"); err == nil {
		t.Fatal("invalid item accepted")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "spinner-item"))
	if strings.TrimSpace(string(data)) != "pepe" {
		t.Fatal("failed validation changed state")
	}
}

func TestSpinnerSize(t *testing.T) {
	for _, n := range []int{64, 240} {
		data, err := resizePNG(sample(), n, true)
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Dx() != n || im.Bounds().Dy() != n/2 {
			t.Fatal("size or aspect ratio lost", im.Bounds())
		}
	}
	if _, err := spinnerSize("pepe", 321); err == nil {
		t.Fatal("oversize accepted")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "throbber-0001.png"), sample(), 0644)
	if err := installSpinnerItem(dir, "default", 96); err != nil {
		t.Fatal(err)
	}
	old := themeDir
	themeDir = dir
	defer func() { themeDir = old }()
	if installedSpinnerSize(true) != 96 {
		t.Fatal("installed size not read")
	}
}

func TestSpinnerCatalog(t *testing.T) {
	if len(spinnerCatalog) != len(spinnerDirectories) {
		t.Fatal("catalog mismatch")
	}
	id, err := spinnerItem("redgirl")
	if err != nil || id != "beubmi" {
		t.Fatal("legacy installed ID lost", id, err)
	}
	if spinnerDirectories[id] != "beubmi" {
		t.Fatal("renamed folder not discovered")
	}
	for _, item := range spinnerCatalog {
		if item.Name == "" {
			t.Fatal("missing display name")
		}
	}
}
