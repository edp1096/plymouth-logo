package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testImageLayer() sceneSpec {
	return sceneSpec{animationSpec: animationSpec{Size: 160, Position: position{20, 80}}, Kind: "image", Name: "logo.png", Image: base64.StdEncoding.EncodeToString(sample())}
}
func testAnimationLayer() sceneSpec {
	return sceneSpec{animationSpec: animationSpec{Item: "default", Size: 32, FPS: 10, Position: position{80, 20}}, Kind: "animation"}
}
func TestSceneValidation(t *testing.T) {
	im, anim := testImageLayer(), testAnimationLayer()
	layers, err := resolveSceneLayers([]sceneSpec{im, anim, im})
	if err != nil || len(layers) != 3 || layers[0].Width != 160 || layers[0].Height != 80 || layers[0].SourceWidth != 80 {
		t.Fatal(layers, err)
	}
	for _, bad := range []sceneSpec{{Kind: "unknown"}, {Kind: "image"}, func() sceneSpec { b := im; b.Size = 1921; return b }(), func() sceneSpec { b := im; b.Position.X = 101; return b }(), func() sceneSpec { b := anim; b.Image = im.Image; return b }()} {
		if _, err = resolveSceneLayers([]sceneSpec{bad}); err == nil {
			t.Fatal("invalid layer accepted", bad.Kind)
		}
	}
	tiny := im
	tiny.Size = 1
	if _, err = resolveSceneLayers([]sceneSpec{tiny}); err != nil {
		t.Fatal("legacy small image must retain dimensions", err)
	}
	// Nine images are rejected independently of the total layer count.
	many := make([]sceneSpec, 9)
	for i := range many {
		many[i] = im
	}
	if _, err = resolveSceneLayers(many); err == nil {
		t.Fatal("image count unchecked")
	}
	large := anim
	large.Size = 320
	many = []sceneSpec{large, large, large, large, large}
	big := im
	big.Size = 1920
	// Preserve legacy animation capacity while accounting for static images separately.
	if _, err = resolveSceneLayers(many); err != nil {
		t.Fatal(err)
	}
	if _, err = resolveSceneLayers(append(many, big)); err != nil {
		t.Fatal("legacy capacity was reduced", err)
	}
	if _, err = resolveSceneLayers([]sceneSpec{big, big, big}); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatal("static image memory unchecked", err)
	}
	if _, err = resolveSceneLayers(append(many, large)); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatal("animation memory unchecked", err)
	}
	broken := im
	broken.Image = base64.StdEncoding.EncodeToString(sample()[:40])
	if _, err = resolveSceneLayers([]sceneSpec{broken}); err == nil {
		t.Fatal("truncated image accepted")
	}
	var jpg bytes.Buffer
	jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 40, 80)), nil)
	jpegLayer := im
	jpegLayer.Image = base64.StdEncoding.EncodeToString(jpg.Bytes())
	layers, err = resolveSceneLayers([]sceneSpec{jpegLayer})
	if err != nil || layers[0].Width != 80 || layers[0].Height != 160 {
		t.Fatal(layers, err)
	}
}
func TestSceneSourcesPreviewAndManifest(t *testing.T) {
	specs := []sceneSpec{testImageLayer(), testAnimationLayer(), testImageLayer()}
	layers, err := resolveSceneLayers(specs)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	old := themeDir
	themeDir = dir
	defer func() { themeDir = old }()
	if err = installSceneLayers(dir, layers); err != nil {
		t.Fatal(err)
	}
	saved, issue := installedSceneLayers(true)
	if issue != "" || len(saved) != 3 || saved[0].Image != "" || saved[1].Kind != "animation" {
		t.Fatal(saved, issue)
	}
	original, err := installedScenePreview(2, true)
	if err != nil || !bytes.Equal(original, sample()) {
		t.Fatal("original image was lost", err)
	}
	preview, err := installedScenePreview(0, false)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(preview))
	if err != nil || decoded.Bounds().Dx() != 160 || decoded.Bounds().Dy() != 80 {
		t.Fatal(err)
	}
	_, _, _, alpha := decoded.At(0, 0).RGBA()
	if alpha == 0 || alpha == 65535 {
		t.Fatal("PNG transparency was lost")
	}
	if _, err = installedScenePreview(1, true); err == nil {
		t.Fatal("animation exposed as image source")
	}
	if _, err = installedScenePreview(-1, false); err == nil {
		t.Fatal("negative index accepted")
	}
	preview, err = installedScenePreview(1, false)
	if err != nil || !bytes.Contains(preview, []byte("acTL")) {
		t.Fatal("animated preview missing", err)
	}
	// Missing originals must be reported, never silently replaced by the rendered copy.
	os.Remove(filepath.Join(dir, sceneSourceName(0)))
	if _, err = installedScenePreview(0, true); err == nil {
		t.Fatal("missing original ignored")
	}
	if err = installSceneLayers(dir, []sceneLayer{}); err != nil {
		t.Fatal(err)
	}
	saved, issue = installedSceneLayers(true)
	if issue != "" || saved == nil || len(saved) != 0 || exists(dir+"/sources") {
		t.Fatal("empty scene not preserved", issue)
	}
	os.WriteFile(dir+"/layers.json", []byte(`{"version":999,"layers":[]}`), 0644)
	if _, issue = installedSceneLayers(true); issue == "" {
		t.Fatal("invalid version accepted")
	}
}
func TestSceneHTTPAndPayload(t *testing.T) {
	called := false
	a := &app{host: "localhost", token: "secret", checkHost: func() error { return nil }, execute: func(action, path string) ([]byte, error) {
		called = true
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var p payload
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if p.Layers == nil || len(p.Layers) != 2 || p.Layers[0].Image != testImageLayer().Image {
			t.Error("scene/original missing from helper payload")
		}
		return nil, fmt.Errorf("mock privileged execution; no boot changes")
	}}
	request := func(path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(body))
		r.Header.Set("X-App-Token", token)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	raw, _ := json.Marshal(map[string]string{"image": testImageLayer().Image})
	if w := request("/api/inspect-image", string(raw), ""); w.Code != 403 {
		t.Fatal("image inspection missing authentication")
	}
	if w := request("/api/inspect-image", string(raw), "secret"); w.Code != 200 || !strings.Contains(w.Body.String(), `"width":80`) {
		t.Fatal(w.Code, w.Body.String())
	}
	bad := testImageLayer()
	bad.Image = "invalid"
	raw, _ = json.Marshal(payload{Layers: []sceneSpec{bad}})
	if w := request("/api/apply", string(raw), "secret"); w.Code != 400 || called || a.state.Busy {
		t.Fatal("bad scene reached authentication")
	}
	raw, _ = json.Marshal(payload{Layers: []sceneSpec{testImageLayer(), testAnimationLayer()}, Background: "#123456"})
	if w := request("/api/apply", string(raw), "secret"); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	a.wg.Wait()
	if !called {
		t.Fatal("scene apply not submitted")
	}
}
func TestSceneApplyArchiveAndRestore(t *testing.T) {
	oldState, oldTheme, oldConfig, oldHook, oldBoot, oldCommand := stateDir, themeDir, configPath, hookPath, bootImage, command
	defer func() {
		stateDir, themeDir, configPath, hookPath, bootImage, command = oldState, oldTheme, oldConfig, oldHook, oldBoot, oldCommand
	}()
	dir := t.TempDir()
	stateDir = dir + "/state"
	themeDir = dir + "/theme"
	configPath = dir + "/config"
	hookPath = dir + "/hook"
	bootImage = dir + "/boot/initrd"
	os.MkdirAll(stateDir, 0700)
	os.MkdirAll(filepath.Dir(bootImage), 0755)
	os.WriteFile(bootImage, []byte("original boot"), 0644)
	os.WriteFile(configPath, []byte("[Daemon]\nTheme=bgrt\n"), 0644)
	specs := []sceneSpec{testImageLayer(), testAnimationLayer(), testImageLayer()}
	layers, err := resolveSceneLayers(specs)
	if err != nil {
		t.Fatal(err)
	}
	listing := "usr/lib/aarch64-linux-gnu/plymouth/script.so\nusr/lib/aarch64-linux-gnu/plymouth/label.so\nusr/share/plymouth/themes/opi-custom-logo/layers.json\nusr/share/plymouth/themes/opi-custom-logo/opi-custom-logo.script\nusr/share/plymouth/themes/opi-custom-logo/watermark.png\nlib/modules/" + kernelVersion + "\n"
	for i, l := range layers {
		for f := 1; f <= l.Frames; f++ {
			listing += "usr/share/plymouth/themes/opi-custom-logo/" + animationFrameName(i, f) + "\n"
		}
	}
	bin := dir + "/bin"
	os.MkdirAll(bin, 0755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	writeListing := func(s string) {
		t.Helper()
		if err := os.WriteFile(bin+"/lsinitramfs", []byte("#!/bin/sh\ncat <<'FILES'\n"+s+"FILES\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	command = func(name string, args ...string) error {
		if name == "mkinitramfs" {
			return os.WriteFile(args[1], []byte("new boot"), 0644)
		}
		return oldCommand(name, args...)
	}
	// An omitted middle frame is rejected, with full rollback before touching the boot image.
	writeListing(strings.ReplaceAll(listing, animationFrameName(1, 2), "missing.png"))
	if err = applyThemeComposition(nil, "#123456", "default", 32, nil, specs); err == nil {
		t.Fatal("incomplete archive accepted")
	}
	boot, _ := os.ReadFile(bootImage)
	if string(boot) != "original boot" || exists(themeDir) {
		t.Fatal("failed archive did not roll back")
	}
	writeListing(listing)
	if err = applyThemeComposition(nil, "#123456", "default", 32, nil, specs); err != nil {
		t.Fatal(err)
	}
	installed, err := readSceneManifest(themeDir)
	if err != nil || len(installed) != 3 {
		t.Fatal(installed, err)
	}
	script, _ := os.ReadFile(themeDir + "/opi-custom-logo.script")
	if strings.Contains(string(script), `Image("watermark.png")`) {
		t.Fatal("legacy logo rendered twice")
	}
	original, _ := os.ReadFile(filepath.Join(themeDir, sceneSourceName(2)))
	if !bytes.Equal(original, sample()) {
		t.Fatal("original not stored")
	}
	saved, err := backupCurrent()
	if err != nil {
		t.Fatal(err)
	}
	// A failed legacy conversion must recover every mixed layer and its original.
	command = func(name string, args ...string) error {
		if name == "mkinitramfs" {
			if exists(themeDir+"/layers.json") || exists(themeDir+"/sources") {
				t.Error("stale scene in legacy conversion")
			}
			return fmt.Errorf("simulated failure")
		}
		return oldCommand(name, args...)
	}
	if err = applyThemeState(sample(), "#000000", "default", 32, nil); err == nil {
		t.Fatal("expected failure")
	}
	original, _ = os.ReadFile(filepath.Join(themeDir, sceneSourceName(2)))
	if !bytes.Equal(original, sample()) {
		t.Fatal("rollback lost scene originals")
	}
	os.RemoveAll(themeDir)
	if err = restoreBackup(saved); err != nil {
		t.Fatal(err)
	}
	restored, _ := readSceneManifest(themeDir)
	if len(restored) != 3 {
		t.Fatal("restore lost order/count")
	}
	original, _ = os.ReadFile(filepath.Join(themeDir, sceneSourceName(0)))
	if !bytes.Equal(original, sample()) {
		t.Fatal("restore lost original")
	}
}

func TestScenePlymouthRuntime(t *testing.T) {
	plugins, _ := filepath.Glob("/usr/lib/*/plymouth/script.so")
	python, err := exec.LookPath("python3")
	if len(plugins) == 0 || err != nil {
		t.Skip("installed Plymouth runtime unavailable")
	}
	layers, err := resolveSceneLayers([]sceneSpec{testImageLayer(), testAnimationLayer(), testImageLayer()})
	if err != nil {
		t.Fatal(err)
	}
	layers[1].Frames = 12
	_, script := sceneScriptTheme(layers, "#123456")
	dir := t.TempDir()
	for i, l := range layers {
		for f := 1; f <= l.Frames; f++ {
			path := filepath.Join(dir, animationFrameName(i, f))
			os.MkdirAll(filepath.Dir(path), 0755)
			os.WriteFile(path, sample(), 0644)
		}
		script += fmt.Sprintf("\nloaded_width_%d=frames_%d[0].GetWidth();\nloaded_height_%d=frames_%d[0].GetHeight();\n", i, i, i, i)
	}
	os.WriteFile(dir+"/theme.script", []byte(script), 0644)
	out, err := exec.Command(python, "tests/script-runtime.py", plugins[0], dir+"/theme.script", dir, "scene").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Log(string(out))
}
