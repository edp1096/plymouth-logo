package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func previewFixture(t *testing.T) (themePreviewPaths, string) {
	t.Helper()
	root := t.TempDir()
	themes := filepath.Join(root, "themes")
	dir := filepath.Join(themes, "bgrt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	conf := "[Plymouth Theme]\nModuleName=two-step\n[two-step]\nImageDir=" + dir + "\nBackgroundStartColor=0x123456\nBackgroundEndColor=0x654321\nHorizontalAlignment=.2\nVerticalAlignment=.7\nWatermarkHorizontalAlignment=.5\nWatermarkVerticalAlignment=.96\n[boot-up]\nUseFirmwareBackground=true\nUseEndAnimation=false\n"
	if err := os.WriteFile(dir+"/bgrt.plymouth", []byte(conf), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"watermark.png", "bgrt-fallback.png"} {
		os.WriteFile(dir+"/"+name, sample(), 0644)
	}
	for i := 1; i <= 30; i++ {
		os.WriteFile(fmt.Sprintf("%s/throbber-%04d.png", dir, i), sample(), 0644)
	}
	p := themePreviewPaths{Config: root + "/config", Defaults: []string{root + "/defaults"}, Roots: []string{themes}, Link: themes + "/default.plymouth", Cmdline: root + "/cmdline", Firmware: root + "/firmware"}
	if err := os.Symlink(dir+"/bgrt.plymouth", p.Link); err != nil {
		t.Fatal(err)
	}
	return p, dir
}
func TestSystemPreviewSeparatesCurrentAndDefault(t *testing.T) {
	p, dir := previewFixture(t)
	custom := filepath.Join(p.Roots[0], "opi-custom-logo")
	os.MkdirAll(custom, 0755)
	os.WriteFile(custom+"/opi-custom-logo.plymouth", []byte("[Plymouth Theme]\nModuleName=script\n"), 0644)
	os.WriteFile(custom+"/watermark.png", sample(), 0644)
	os.WriteFile(p.Config, []byte("[Daemon]\nTheme = opi-custom-logo\n"), 0644)
	if _, _, applied := selectedCustomPreview(p, custom); !applied {
		t.Fatal("selected custom theme not detected")
	}
	current := p.preview("current")
	if current.Available || current.Name != "opi-custom-logo" || current.Module != "script" {
		t.Fatal(current)
	}
	def := p.preview("default")
	if !def.Available || def.Name != "bgrt" {
		t.Fatal(def.Reason, def.Name)
	}
	// Restoring a system theme changes current, without changing the default reference.
	os.WriteFile(p.Config, []byte("[Daemon]\nTheme=bgrt\n"), 0644)
	if current = p.preview("current"); !current.Available || current.Name != "bgrt" {
		t.Fatal(current.Reason)
	}
	os.WriteFile(p.Config, []byte("[Daemon]\nTheme=missing\n"), 0644)
	if p.preview("current").Available {
		t.Fatal("missing selected theme was silently replaced with default")
	}
	os.WriteFile(p.Config, []byte("[Daemon]\nTheme=opi-custom-logo\n"), 0644)
	os.WriteFile(p.Cmdline, []byte("quiet splash plymouth.splash=bgrt"), 0644)
	if _, _, applied := selectedCustomPreview(p, custom); applied {
		t.Fatal("kernel override incorrectly showed custom theme as current")
	}
	if got, err := p.resolve("current"); err != nil || got != dir+"/bgrt.plymouth" {
		t.Fatal(got, err)
	}
	os.WriteFile(p.Defaults[0], []byte("[Daemon]\nTheme=opi-custom-logo\n"), 0644)
	if got := p.preview("default"); got.Name != "opi-custom-logo" || got.Available {
		t.Fatal("default must follow configuration, not assume BGRT")
	}
}
func TestTwoStepPreviewAssetsAndGeometry(t *testing.T) {
	p, dir := previewFixture(t)
	got := p.preview("default")
	if !got.Available || len(got.Layers) != 3 {
		t.Fatal(got.Reason, len(got.Layers))
	}
	fallback, logo, spinner := got.Layers[0], got.Layers[1], got.Layers[2]
	if got.Top != "#000000" || got.Bottom != "#000000" || !fallback.Center || fallback.X != 50 || fallback.Y != 38.2 {
		t.Fatal("BGRT fallback geometry or black background differs")
	}
	if logo.Center || logo.X != 50 || logo.Y != 96 || logo.Width != 80 || logo.Height != 40 {
		t.Fatal("watermark must align within remaining space")
	}
	if !spinner.Center || spinner.X != 20 || spinner.Y != 70 {
		t.Fatal("two-step spinner must align by center")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(spinner.Image, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	frames := 0
	for pos := 8; pos+12 <= len(raw); {
		n := int(binary.BigEndian.Uint32(raw[pos : pos+4]))
		kind := string(raw[pos+4 : pos+8])
		body := raw[pos+8 : pos+8+n]
		if kind == "fcTL" {
			frames++
			if binary.BigEndian.Uint16(body[20:22]) != 67 {
				t.Fatal("wrong two-second throbber timing")
			}
		}
		pos += n + 12
	}
	if frames != 30 {
		t.Fatal(frames)
	}
	data, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(logo.Image, "data:image/png;base64,"))
	if !bytes.Equal(data, sample()) {
		t.Fatal("preview did not use installed image")
	}
	os.Remove(dir + "/bgrt-fallback.png")
	got = p.preview("default")
	if !got.Available || got.Top != "#123456" || got.Bottom != "#654321" || len(got.Layers) != 2 {
		t.Fatal(got.Reason, got.Top)
	}
}
func TestSystemPreviewUnavailableAssets(t *testing.T) {
	for _, test := range []string{"firmware", "gap", "corrupt", "alignment", "tile"} {
		t.Run(test, func(t *testing.T) {
			p, dir := previewFixture(t)
			switch test {
			case "firmware":
				os.WriteFile(p.Firmware, []byte("firmware bitmap"), 0644)
			case "gap":
				os.Remove(dir + "/throbber-0002.png")
			case "corrupt":
				os.WriteFile(dir+"/watermark.png", []byte("broken"), 0644)
			case "alignment":
				data, _ := os.ReadFile(dir + "/bgrt.plymouth")
				os.WriteFile(dir+"/bgrt.plymouth", bytes.ReplaceAll(data, []byte("HorizontalAlignment=.2"), []byte("HorizontalAlignment=NaN")), 0644)
			case "tile":
				os.WriteFile(dir+"/background-tile.png", sample(), 0644)
			}
			got := p.preview("default")
			if got.Available || got.Reason == "" || len(got.Layers) != 0 {
				t.Fatal("incomplete preview must not look valid", got.Reason)
			}
		})
	}
}
func TestSystemPreviewHTTP(t *testing.T) {
	p, _ := previewFixture(t)
	a := &app{host: "localhost", token: "secret", themePreview: p.preview}
	for _, test := range []struct {
		source, token string
		code          int
	}{{"default", "", 403}, {"../../etc/passwd", "secret", 400}, {"default", "secret", 200}} {
		r := httptest.NewRequest("GET", "http://localhost/api/theme-preview?source="+test.source, nil)
		r.Header.Set("X-App-Token", test.token)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != test.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if w.Code == 200 {
			var got systemThemePreview
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || !got.Available || got.Name != "bgrt" {
				t.Fatal(err, got.Reason)
			}
		}
	}
}
