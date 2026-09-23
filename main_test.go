package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample() []byte {
	im := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			im.SetNRGBA(x, y, color.NRGBA{200, 80, 30, 128})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, im)
	return b.Bytes()
}
func TestNormalize(t *testing.T) {
	data, err := normalize(sample(), 32)
	if err != nil {
		t.Fatal(err)
	}
	im, _, _ := image.Decode(bytes.NewReader(data))
	if im.Bounds().Dx() != 32 || im.Bounds().Dy() != 16 {
		t.Fatal(im.Bounds())
	}
	_, _, _, a := im.At(0, 0).RGBA()
	if a != 128*257 {
		t.Fatal("alpha lost")
	}
	if _, err = normalize([]byte("bad"), 320); err == nil {
		t.Fatal("invalid image accepted")
	}
}
func TestHTTPGuards(t *testing.T) {
	a := &app{host: "127.0.0.1:8090", token: "secret"}
	for _, tt := range []struct {
		path, token, origin, host string
		code                      int
	}{{"/api/status", "", "", "127.0.0.1:8090", 403}, {"/api/restore", "secret", "https://bad.example", "127.0.0.1:8090", 403}, {"/api/restore", "secret", "", "evil.example", 403}, {"/api/apply", "secret", "", "127.0.0.1:8090", 400}} {
		r := httptest.NewRequest("POST", "http://"+tt.host+tt.path, strings.NewReader("{}"))
		r.Header.Set("X-App-Token", tt.token)
		r.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tt.code {
			t.Fatalf("%s: %d", tt.path, w.Code)
		}
	}
}

func TestHostPreflight(t *testing.T) {
	var hostErr error = fmt.Errorf("cancel pending kernel trials first")
	a := &app{host: "localhost", token: "secret", checkHost: func() error { return hostErr },
		execute: func(string, string) ([]byte, error) {
			t.Error("unsupported host must not request administrator authentication")
			return nil, nil
		}}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		r.Header.Set("X-App-Token", "secret")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/api/status", "")
	var state status
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil || state.HostError != hostErr.Error() {
		t.Fatal(w.Body.String(), err)
	}
	for _, action := range []string{"apply", "restore", "restore-original"} {
		w = request("POST", "/api/"+action, "{}")
		if w.Code != 409 || !strings.Contains(w.Body.String(), hostErr.Error()) || a.state.Busy {
			t.Fatal(action, w.Code, w.Body.String())
		}
	}
	// A cancelled trial can be cleared without restarting the application.
	hostErr = nil
	w = request("GET", "/api/status", "")
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil || state.HostError != "" {
		t.Fatal(w.Body.String(), err)
	}
	if w = request("POST", "/api/apply", "{}"); w.Code != 400 {
		t.Fatal("supported host must reach payload validation", w.Code)
	}
}
func TestEmbeddedUI(t *testing.T) {
	a := &app{host: "127.0.0.1:8090"}
	r := httptest.NewRequest("GET", "http://127.0.0.1:8090/", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Plymouth Logo") {
		t.Fatal("missing embedded UI")
	}
}
func TestJobAuthenticationFailure(t *testing.T) {
	a := &app{execute: func(action, path string) ([]byte, error) {
		return []byte("Authentication cancelled"), fmt.Errorf("cancelled")
	}}
	a.wg.Add(1)
	a.perform("restore", payload{})
	if a.state.Busy || a.state.OK || !strings.Contains(a.state.Message, "Authentication cancelled") {
		t.Fatal(a.state)
	}
}
func TestInvalidUploadNeverElevates(t *testing.T) {
	a := &app{execute: func(string, string) ([]byte, error) { t.Fatal("must not elevate"); return nil, nil }}
	a.wg.Add(1)
	a.perform("apply", payload{Image: "bad", Size: 320})
	if a.state.OK {
		t.Fatal("accepted invalid upload")
	}
}
func TestApplyRequest(t *testing.T) {
	a := &app{host: "127.0.0.1:8090", token: "secret", execute: func(action, path string) ([]byte, error) {
		if action != "apply" {
			t.Error(action)
		}
		if _, err := os.Stat(path); err != nil {
			t.Error(err)
		}
		return []byte("Applied"), nil
	}}
	body, _ := json.Marshal(payload{Image: base64.StdEncoding.EncodeToString(sample()), Size: 320})
	r := httptest.NewRequest("POST", "http://127.0.0.1:8090/api/apply", bytes.NewReader(body))
	r.Header.Set("X-App-Token", "secret")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	a.wg.Wait()
	if w.Code != 202 || !a.state.OK {
		t.Fatal(w.Code, a.state)
	}
}
func TestConfig(t *testing.T) {
	s := configTheme("[Daemon]\nTheme = bgrt\nShowDelay=0\n[Other]\nTheme=keep\n")
	if strings.Count(s, "Theme=opi-custom-logo") != 1 || !strings.Contains(s, "ShowDelay=0") || !strings.Contains(s, "Theme=keep") {
		t.Fatal(s)
	}
}
func TestBackupRestoreAndRollback(t *testing.T) {
	oldState, oldTheme, oldConf, oldHook, oldBoot := stateDir, themeDir, configPath, hookPath, bootImage
	defer func() {
		stateDir, themeDir, configPath, hookPath, bootImage = oldState, oldTheme, oldConf, oldHook, oldBoot
	}()
	root := t.TempDir()
	stateDir = filepath.Join(root, "state")
	themeDir = filepath.Join(root, "theme")
	configPath = filepath.Join(root, "config")
	hookPath = filepath.Join(root, "hook")
	bootImage = filepath.Join(root, "initrd")
	os.MkdirAll(stateDir, 0700)
	os.WriteFile(bootImage, []byte("original initrd"), 0644)
	os.WriteFile(configPath, []byte("[Daemon]\nTheme=bgrt\n"), 0644)
	backup, err := backupCurrent()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(bootImage, []byte("changed"), 0644)
	if err = restoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(bootImage)
	if string(data) != "original initrd" {
		t.Fatal("restore failed")
	}
	oldCommand := command
	defer func() { command = oldCommand }()
	command = func(name string, args ...string) error {
		if name == "mkinitramfs" {
			return fmt.Errorf("simulated build failure")
		}
		return oldCommand(name, args...)
	}
	if err = applyLogo(sample()); err == nil {
		t.Fatal("failure not reported")
	}
	data, _ = os.ReadFile(bootImage)
	if string(data) != "original initrd" || exists(themeDir) || exists(hookPath) {
		t.Fatal("rollback did not restore previous state")
	}
	// The script path must also roll back every staged resource on a build failure.
	command = func(name string, args ...string) error {
		if name == "mkinitramfs" {
			script, e := os.ReadFile(themeDir + "/opi-custom-logo.script")
			if e != nil || !strings.Contains(string(script), "Plymouth.SetDisplayPasswordFunction") {
				t.Fatal("script not staged before build")
			}
			hook, e := os.ReadFile(hookPath)
			if e != nil || !strings.Contains(string(hook), "copy_exec") {
				t.Fatal("script plugin inclusion hook missing")
			}
			return fmt.Errorf("simulated script-theme build failure")
		}
		return oldCommand(name, args...)
	}
	if err = applyThemeSized(sample(), "#123456", "karateka", 160); err == nil {
		t.Fatal("script build failure not reported")
	}
	data, _ = os.ReadFile(bootImage)
	if string(data) != "original initrd" || exists(themeDir) || exists(hookPath) {
		t.Fatal("script rollback did not restore original state")
	}

	// Multi-layer staging and legacy conversion must restore the complete old theme on failure.
	os.MkdirAll(themeDir+"/animations/sentinel", 0755)
	os.WriteFile(themeDir+"/animations/sentinel/old", []byte("old animation"), 0644)
	os.WriteFile(themeDir+"/animations.json", []byte("old manifest"), 0644)
	for _, modern := range []bool{true, false} {
		reached := false
		command = func(name string, args ...string) error {
			if name != "mkinitramfs" {
				return oldCommand(name, args...)
			}
			reached = true
			if modern {
				layers, e := readAnimationManifest(themeDir)
				if e != nil || len(layers) != 2 || layers[1].FPS != 7 {
					t.Fatal("multi-layer staging failed", layers, e)
				}
				if !exists(filepath.Join(themeDir, animationFrameName(1, 2))) {
					t.Fatal("second animation missing")
				}
			} else if exists(themeDir+"/animations.json") || exists(themeDir+"/animations") {
				t.Fatal("legacy conversion left stale layers")
			}
			return fmt.Errorf("simulated multi-layer build failure")
		}
		var specs []animationSpec
		if modern {
			specs = []animationSpec{{Item: "default", Size: 32, FPS: 10, Position: position{20, 80}}, {Item: "default", Size: 32, FPS: 7, Position: position{80, 20}}}
		}
		if err = applyThemeState(sample(), "#123456", "default", 32, specs); err == nil || !reached {
			t.Fatal("build failure not reached", err)
		}
		manifest, _ := os.ReadFile(themeDir + "/animations.json")
		frame, _ := os.ReadFile(themeDir + "/animations/sentinel/old")
		image, _ := os.ReadFile(bootImage)
		if string(manifest) != "old manifest" || string(frame) != "old animation" || string(image) != "original initrd" {
			t.Fatal("complete prior theme not restored")
		}
	}
	os.WriteFile(backup+"/initrd.img", []byte("tampered"), 0644)
	if err = restoreBackup(backup); err == nil {
		t.Fatal("tampered backup accepted")
	}
}
