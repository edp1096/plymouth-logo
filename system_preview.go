package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Read-only preview of installed two-step themes. Never executes theme scripts.
// Geometry and the two-second throbber cycle follow Plymouth 0.9.5 two-step.
type themePreviewLayer struct {
	Image  string  `json:"image"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Center bool    `json:"center"`
}
type systemThemePreview struct {
	Name      string              `json:"name"`
	Module    string              `json:"module"`
	Available bool                `json:"available"`
	Reason    string              `json:"reason,omitempty"`
	Note      string              `json:"note,omitempty"`
	Top       string              `json:"top"`
	Bottom    string              `json:"bottom"`
	Layers    []themePreviewLayer `json:"layers"`
}
type themePreviewPaths struct {
	Config   string
	Defaults []string
	Roots    []string
	Link     string
	Cmdline  string
	Firmware string
}

func installedThemePaths() themePreviewPaths {
	return themePreviewPaths{configPath, []string{"/run/plymouth/plymouthd.defaults", "/usr/share/plymouth/plymouthd.defaults", "/usr/lib/plymouth/plymouthd.defaults"}, []string{"/run/plymouth/themes", "/usr/share/plymouth/themes"}, "/usr/share/plymouth/themes/default.plymouth", "/proc/cmdline", "/sys/firmware/acpi/bgrt/image"}
}
func readThemeINI(path string) (map[string]map[string]string, error) {
	data, err := readLimited(path, 256*1024)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]string{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if result[section] == nil {
				result[section] = map[string]string{}
			}
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && result[section] != nil {
			result[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return result, nil
}
func (p themePreviewPaths) namedTheme(name, extraRoot string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid theme name")
	}
	roots := append([]string(nil), p.Roots...)
	if extraRoot != "" {
		roots = append([]string{p.Roots[0], extraRoot}, p.Roots[1:]...)
	}
	for _, root := range roots {
		path := filepath.Join(root, name, name+".plymouth")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", fmt.Errorf("theme file unavailable: %s", name)
}
func (p themePreviewPaths) resolve(source string) (string, error) {
	if source != "current" && source != "default" {
		return "", fmt.Errorf("invalid preview source")
	}
	if source == "current" {
		if data, err := os.ReadFile(p.Cmdline); err == nil {
			for _, arg := range strings.Fields(string(data)) {
				if name, ok := strings.CutPrefix(arg, "plymouth.splash="); ok {
					return p.namedTheme(name, "")
				}
			}
		}
		ini, err := readThemeINI(p.Config)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if name := ini["Daemon"]["Theme"]; name != "" {
			return p.namedTheme(name, ini["Daemon"]["ThemeDir"])
		}
	}
	for _, path := range p.Defaults {
		ini, err := readThemeINI(path)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if name := ini["Daemon"]["Theme"]; name != "" {
			return p.namedTheme(name, ini["Daemon"]["ThemeDir"])
		}
	}
	return filepath.EvalSymlinks(p.Link)
}
func previewAlignment(section map[string]string, key string, fallback float64) (float64, error) {
	value, ok := section[key]
	if !ok {
		return fallback, nil
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n * 100, nil
}
func previewColor(value string) (string, error) {
	if value == "" {
		return "#000000", nil
	}
	n, err := strconv.ParseUint(value, 0, 24)
	if err != nil {
		return "", fmt.Errorf("invalid background color")
	}
	return fmt.Sprintf("#%06x", n), nil
}
func previewPNG(path string) ([]byte, int, int, error) {
	data, err := readLimited(path, 12*1024*1024)
	if err != nil {
		return nil, 0, 0, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return nil, 0, 0, fmt.Errorf("invalid preview PNG: %s", filepath.Base(path))
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		return nil, 0, 0, err
	}
	return data, cfg.Width, cfg.Height, nil
}
func pngDataURL(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}
func readTwoStepPreview(path, firmware string) (out systemThemePreview, err error) {
	out.Name = strings.TrimSuffix(filepath.Base(path), ".plymouth")
	ini, err := readThemeINI(path)
	if err != nil {
		return out, err
	}
	out.Module = ini["Plymouth Theme"]["ModuleName"]
	if out.Module != "two-step" {
		return out, fmt.Errorf("이 테마 형식은 미리보기를 지원하지 않습니다.")
	}
	section, boot := ini["two-step"], ini["boot-up"]
	dir := section["ImageDir"]
	if !filepath.IsAbs(dir) {
		return out, fmt.Errorf("theme ImageDir must be absolute")
	}
	if out.Top, err = previewColor(section["BackgroundStartColor"]); err != nil {
		return out, err
	}
	if out.Bottom, err = previewColor(section["BackgroundEndColor"]); err != nil {
		return out, err
	}
	out.Layers = []themePreviewLayer{}
	addPNG := func(name string, x, y float64, center bool) error {
		data, w, h, e := previewPNG(filepath.Join(dir, name))
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		out.Layers = append(out.Layers, themePreviewLayer{pngDataURL(data), w, h, x, y, center})
		return nil
	}
	if boot["UseFirmwareBackground"] == "true" {
		if _, e := os.Stat(firmware); e == nil {
			return out, fmt.Errorf("펌웨어 BGRT 화면은 미리보기를 지원하지 않습니다.")
		} else if !os.IsNotExist(e) {
			return out, e
		}
		if err = addPNG("bgrt-fallback.png", 50, 38.2, true); err != nil {
			return out, err
		}
		if len(out.Layers) > 0 {
			out.Top, out.Bottom = "#000000", "#000000"
		}
	}
	// Do not silently omit visual elements that this preview does not render.
	for _, name := range []string{"background-tile.png", "header-image.png", "corner-image.png"} {
		if _, e := os.Stat(filepath.Join(dir, name)); e == nil {
			return out, fmt.Errorf("preview does not support %s", name)
		} else if !os.IsNotExist(e) {
			return out, e
		}
	}
	x, err := previewAlignment(section, "WatermarkHorizontalAlignment", 100)
	if err != nil {
		return out, err
	}
	y, err := previewAlignment(section, "WatermarkVerticalAlignment", 50)
	if err != nil {
		return out, err
	}
	if err = addPNG("watermark.png", x, y, false); err != nil {
		return out, err
	}
	if boot["UseProgressBar"] == "true" || boot["Title"] != "" || boot["SubTitle"] != "" {
		return out, fmt.Errorf("이 테마의 부팅 진행 화면은 미리보기를 지원하지 않습니다.")
	}
	if boot["UseAnimation"] != "false" {
		files, e := filepath.Glob(filepath.Join(dir, "throbber-*.png"))
		if e != nil {
			return out, e
		}
		if len(files) == 0 || len(files) > 120 {
			return out, fmt.Errorf("unsupported throbber frame count: %d", len(files))
		}
		for i, file := range files {
			if filepath.Base(file) != fmt.Sprintf("throbber-%04d.png", i+1) {
				return out, fmt.Errorf("spinner frame sequence is incomplete")
			}
		}
		_, w, h, e := previewPNG(files[0])
		if e != nil {
			return out, e
		}
		data, e := animationPNG(spinnerEntry{FPS: math.Max(1, float64(len(files))/2), Frames: len(files)}, func(i int) ([]byte, error) { return readLimited(files[i-1], 12*1024*1024) })
		if e != nil {
			return out, e
		}
		x, err = previewAlignment(section, "HorizontalAlignment", 50)
		if err != nil {
			return out, err
		}
		y, err = previewAlignment(section, "VerticalAlignment", 50)
		if err != nil {
			return out, err
		}
		out.Layers = append(out.Layers, themePreviewLayer{pngDataURL(data), w, h, x, y, true})
	}
	out.Available = true
	out.Note = "설치된 테마 파일 기준 · 일반 부팅 화면 · 1920 × 1080"
	return out, nil
}
func (p themePreviewPaths) preview(source string) systemThemePreview {
	path, err := p.resolve(source)
	out := systemThemePreview{}
	if err == nil {
		out, err = readTwoStepPreview(path, p.Firmware)
	}
	if err != nil {
		out.Available = false
		out.Layers = nil
		out.Reason = err.Error()
	}
	return out
}
