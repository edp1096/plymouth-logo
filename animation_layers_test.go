package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnimationLayersValidation(t *testing.T) {
	spec := animationSpec{Item: "default", Size: 32, FPS: 10, Position: position{20, 80}}
	other := spec
	other.FPS = 7
	other.Position = position{90, 10}
	layers, err := resolveAnimationLayers([]animationSpec{spec, other})
	if err != nil || len(layers) != 2 || layers[0].FPS != 10 || layers[1].FPS != 7 || layers[1].Position.X != 90 {
		t.Fatal(layers, err)
	}
	empty, err := resolveAnimationLayers([]animationSpec{})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty animation list must remain distinct from legacy", err)
	}
	for _, fps := range []float64{-1, 61, math.NaN(), math.Inf(1)} {
		bad := spec
		bad.FPS = fps
		if _, err = resolveAnimationLayers([]animationSpec{bad}); err == nil {
			t.Fatal("invalid FPS accepted", fps)
		}
	}
	for _, size := range []int{-1, 31, 321} {
		bad := spec
		bad.Size = size
		if _, err = resolveAnimationLayers([]animationSpec{bad}); err == nil {
			t.Fatal("invalid size accepted", size)
		}
	}
	bad := spec
	bad.Position.X = 101
	if _, err = resolveAnimationLayers([]animationSpec{bad}); err == nil {
		t.Fatal("invalid position accepted")
	}
	bad = spec
	bad.Item = "../escape"
	if _, err = resolveAnimationLayers([]animationSpec{bad}); err == nil {
		t.Fatal("invalid item accepted")
	}
	if _, err = resolveAnimationLayers(make([]animationSpec, 9)); err == nil {
		t.Fatal("layer count unchecked")
	}
	large := spec
	large.Size = 320
	specs := []animationSpec{large, large, large, large, large, large}
	if _, err = resolveAnimationLayers(specs[:5]); err != nil {
		t.Fatal(err)
	}
	if _, err = resolveAnimationLayers(specs); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatal("aggregate budget unchecked", err)
	}
	if w, h := animationDimensions(160, 87, 99); w != 99 || h != 53 {
		t.Fatal(w, h)
	}
	// Invalid lists are rejected before administrator authentication.
	a := &app{host: "localhost", token: "secret", checkHost: func() error { return nil }, execute: func(string, string) ([]byte, error) {
		t.Error("invalid layers reached privileged execution")
		return nil, nil
	}}
	body, _ := json.Marshal(payload{Image: base64.StdEncoding.EncodeToString(sample()), Size: 320, Animations: specs})
	r := httptest.NewRequest("POST", "http://localhost/api/apply", bytes.NewReader(body))
	r.Header.Set("X-App-Token", "secret")
	rec := httptest.NewRecorder()
	a.ServeHTTP(rec, r)
	if rec.Code != 400 || a.state.Busy || !strings.Contains(rec.Body.String(), "64 MiB") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestInstalledAnimationLayersAndArchive(t *testing.T) {
	layers, err := resolveAnimationLayers([]animationSpec{{Item: "default", Size: 32, FPS: 10, Position: position{20, 80}}, {Item: "default", Size: 32, FPS: 7, Position: position{80, 20}}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = installAnimationLayers(dir, layers); err != nil {
		t.Fatal(err)
	}
	old := themeDir
	themeDir = dir
	defer func() { themeDir = old }()
	installed, issue := installedAnimationLayers(true)
	if issue != "" || len(installed) != 2 || installed[1].FPS != 7 {
		t.Fatal(installed, issue)
	}
	for i := range layers {
		preview, err := installedAnimationLayerPreview(i)
		if err != nil || !bytes.Contains(preview, []byte("acTL")) {
			t.Fatal(i, err)
		}
	}
	if _, err = installedAnimationLayerPreview(-1); err == nil {
		t.Fatal("invalid index accepted")
	}
	listing := "usr/lib/aarch64-linux-gnu/plymouth/script.so\nusr/lib/aarch64-linux-gnu/plymouth/label.so\nusr/share/plymouth/themes/opi-custom-logo/animations.json\nusr/share/plymouth/themes/opi-custom-logo/opi-custom-logo.script\n"
	for i, layer := range layers {
		for f := 1; f <= layer.Frames; f++ {
			listing += "./usr/share/plymouth/themes/opi-custom-logo/" + animationFrameName(i, f) + "\n"
		}
	}
	if err = verifyAnimationArchive(listing, layers); err != nil {
		t.Fatal(err)
	}
	missing := strings.ReplaceAll(listing, animationFrameName(1, 2), "wrong.png")
	if err = verifyAnimationArchive(missing, layers); err == nil {
		t.Fatal("missing middle frame accepted")
	}
	// Installed previews remain readable even if the editable catalog changes.
	layers[0].Item = "removed-from-catalog"
	raw, _ := json.Marshal(animationManifest{1, layers})
	os.WriteFile(dir+"/animations.json", raw, 0644)
	if _, err = installedAnimationLayerPreview(0); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, animationFrameName(1, 2)))
	if _, err = installedAnimationLayerPreview(1); err == nil {
		t.Fatal("missing installed frame accepted")
	}
	if err = installAnimationLayers(dir, []animationLayer{}); err != nil {
		t.Fatal(err)
	}
	installed, issue = installedAnimationLayers(true)
	if issue != "" || installed == nil || len(installed) != 0 || exists(dir+"/animations") {
		t.Fatal("empty list was not preserved", installed, issue)
	}
	os.WriteFile(dir+"/animations.json", []byte(`{"version":2,"layers":[]}`), 0644)
	if _, issue = installedAnimationLayers(true); issue == "" {
		t.Fatal("invalid manifest accepted")
	}
}

func TestMultiScriptRuntime(t *testing.T) {
	layers := []animationLayer{{animationSpec: animationSpec{FPS: 10, Position: position{20, 80}}, Frames: 12}, {animationSpec: animationSpec{FPS: 7, Position: position{80, 20}, Center: true}, Frames: 9}}
	plugins, _ := filepath.Glob("/usr/lib/*/plymouth/script.so")
	python, err := exec.LookPath("python3")
	if len(plugins) == 0 || err != nil {
		t.Skip("installed Plymouth runtime unavailable")
	}
	for _, count := range []int{2, 0} {
		dir := t.TempDir()
		_, script := multiScriptTheme(layers[:count], "#234567", position{50, 50})
		for i := range layers[:count] {
			script += fmt.Sprintf("\nloaded_width_%d = frames_%d[0].GetWidth();\nloaded_height_%d = frames_%d[0].GetHeight();\n", i, i, i, i)
		}
		os.WriteFile(dir+"/theme.script", []byte(script), 0644)
		os.WriteFile(dir+"/watermark.png", sample(), 0644)
		for i, layer := range layers[:count] {
			for f := 1; f <= layer.Frames; f++ {
				path := filepath.Join(dir, animationFrameName(i, f))
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, sample(), 0644)
			}
		}
		out, err := exec.Command(python, "tests/script-runtime.py", plugins[0], dir+"/theme.script", dir, fmt.Sprint(count)).CombinedOutput()
		if err != nil {
			t.Fatalf("%d layers: %v\n%s", count, err, out)
		}
		t.Log(string(out))
	}
}
