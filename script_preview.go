package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// A script is executable program logic. Show its referenced artwork as a
// clearly labelled reference rather than claiming to emulate its callbacks.
func readScriptReference(path string, ini map[string]map[string]string) (out systemThemePreview, err error) {
	out.Name = strings.TrimSuffix(filepath.Base(path), ".plymouth")
	out.Module = "script"
	section := ini["script"]
	dir, script := section["ImageDir"], section["ScriptFile"]
	if !filepath.IsAbs(dir) || !filepath.IsAbs(script) {
		return out, fmt.Errorf("script ImageDir and ScriptFile must be absolute")
	}
	data, err := readLimited(script, 512*1024)
	if err != nil {
		return out, err
	}
	// Ignore commented-out image references.
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = strings.SplitN(line, "#", 2)[0]
	}
	refs := regexp.MustCompile(`\bImage\s*\(\s*"([^"]+)"\s*\)`).FindAllStringSubmatch(strings.Join(lines, "\n"), -1)
	seen := map[string]bool{}
	for _, ref := range refs {
		name := ref[1]
		// Password dialog/progress assets do not represent the normal boot logo.
		lower := strings.ToLower(filepath.Base(name))
		if !strings.Contains(lower, "logo") && !strings.Contains(lower, "watermark") && !strings.Contains(lower, "splash") {
			continue
		}
		filename := filepath.Join(dir, name)
		if filepath.IsAbs(name) {
			filename = name
		}
		if name == "special://logo" {
			filename = installedScriptLogo()
		}
		if filename == "" || seen[filename] {
			continue
		}
		seen[filename] = true
		raw, w, h, e := previewPNG(filename)
		if e != nil {
			return out, e
		}
		scale := 1.0
		if w > 512 {
			scale = 512 / float64(w)
		}
		if float64(h)*scale > 400 {
			scale = 400 / float64(h)
		}
		out.Layers = append(out.Layers, themePreviewLayer{Image: pngDataURL(raw), Width: int(float64(w) * scale), Height: int(float64(h) * scale), Y: 50, Center: true})
		if len(out.Layers) == 3 {
			break
		}
	}
	if len(out.Layers) == 0 {
		return out, fmt.Errorf("스크립트 테마에서 참고할 로고 이미지를 찾지 못했습니다.")
	}
	for i := range out.Layers {
		out.Layers[i].X = float64(i+1) * 100 / float64(len(out.Layers)+1)
	}
	out.Top, out.Bottom = "#000000", "#000000"
	out.Available = true
	out.Note = "설치된 스크립트 테마의 로고 참고 · 실제 배치·애니메이션은 재현하지 않음"
	return out, nil
}

func installedScriptLogo() string {
	output, err := exec.Command("plymouth", "--get-splash-plugin-path").Output()
	if err != nil {
		return ""
	}
	plugin := filepath.Join(strings.TrimSpace(string(output)), "script.so")
	data, err := readLimited(plugin, 8*1024*1024)
	if err != nil {
		return ""
	}
	for _, value := range bytes.Split(data, []byte{0}) {
		path := string(value)
		if filepath.IsAbs(path) && strings.HasSuffix(path, ".png") && strings.Contains(strings.ToLower(path), "logo") && exists(path) {
			return path
		}
	}
	return ""
}
