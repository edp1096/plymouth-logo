package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptThemeInstalledStateAndRuntime(t *testing.T) {
	entry := spinnerEntry{ID: "test", Name: "Test", FPS: 10, Frames: 12}
	conf, script := scriptTheme(entry, "#234567", position{20, 80}, position{70, 30})
	dir := t.TempDir()
	os.WriteFile(dir+"/opi-custom-logo.plymouth", []byte(conf), 0644)
	os.WriteFile(dir+"/theme.script", []byte(script), 0644)
	os.WriteFile(dir+"/watermark.png", sample(), 0644)
	for i := 1; i <= entry.Frames; i++ {
		os.WriteFile(fmt.Sprintf("%s/throbber-%04d.png", dir, i), sample(), 0644)
	}
	old := themeDir
	themeDir = dir
	defer func() { themeDir = old }()
	if installedPosition(true) != (position{20, 80}) || installedLogoPosition(true) != (position{70, 30}) || installedBackground(true) != "#234567" {
		t.Fatal("script theme state was not restored")
	}
	if !strings.Contains(conf, "ModuleName=script") || !strings.Contains(conf, "AnimationFrames=12") {
		t.Fatal(conf)
	}
	previews, err := installedSpinnerPreview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = png.Decode(bytes.NewReader(previews)); err != nil {
		t.Fatal(err)
	}
	plugins, _ := filepath.Glob("/usr/lib/*/plymouth/script.so")
	python, err := exec.LookPath("python3")
	if len(plugins) == 0 || err != nil {
		t.Log("optional installed-Plymouth runtime check unavailable")
		return
	}
	out, err := exec.Command(python, "tests/script-runtime.py", plugins[0], dir+"/theme.script", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Log(string(out))
}

func TestAnimationPreviewTiming(t *testing.T) {
	data, err := animationPNG(spinnerEntry{FPS: 1000.0 / 60, Frames: 12}, func(i int) ([]byte, error) { return sample(), nil })
	if err != nil {
		t.Fatal(err)
	}
	frames := 0
	for pos := 8; pos+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		kind := string(data[pos+4 : pos+8])
		body := data[pos+8 : pos+8+n]
		if kind == "acTL" && binary.BigEndian.Uint32(body[:4]) != 12 {
			t.Fatal("wrong frame count")
		}
		if kind == "fcTL" {
			frames++
			if binary.BigEndian.Uint16(body[20:22]) != 60 || binary.BigEndian.Uint16(body[22:24]) != 1000 {
				t.Fatal("lost original 60 ms frame timing")
			}
		}
		pos += n + 12
	}
	if frames != 12 {
		t.Fatal(frames)
	}
	if err = spinnerMemoryBudget(spinnerEntry{Frames: 600}, 320); err == nil {
		t.Fatal("excessive early boot memory accepted")
	}
}

func TestVariableSpinnerFramesAndValidation(t *testing.T) {
	root := t.TempDir()
	dir := root + "/assets/spinners/custom"
	os.MkdirAll(dir, 0755)
	os.WriteFile(dir+"/item.json", []byte(`{"id":"custom","name":"Custom","fps":12}`), 0644)
	for i := 1; i <= 3; i++ {
		os.WriteFile(fmt.Sprintf("%s/throbber-%04d.png", dir, i), sample(), 0644)
	}
	oldAssets, oldDirs, oldCatalog, oldAliases := assets, spinnerDirectories, spinnerCatalog, spinnerAliases
	defer func() {
		assets, spinnerDirectories, spinnerCatalog, spinnerAliases = oldAssets, oldDirs, oldCatalog, oldAliases
	}()
	assets = diskResources{root}
	spinnerDirectories, spinnerCatalog, spinnerAliases = loadSpinnerCatalog()
	entry, err := spinnerMetadata("custom")
	if err != nil || entry.Frames != 3 || entry.FPS != 12 {
		t.Fatal(entry, err)
	}
	dest := t.TempDir()
	if err = installSpinnerItem(dest, "custom", 64); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(dest + "/throbber-*.png")
	if len(files) != 3 {
		t.Fatal("fixed 60-frame assumption retained")
	}
	os.Remove(dir + "/throbber-0002.png")
	if _, err = spinnerFrameFiles("assets/spinners/custom"); err == nil {
		t.Fatal("gap accepted")
	}
}
