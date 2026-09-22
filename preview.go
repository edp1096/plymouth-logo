package main

import (
	"bytes"
	"image"
	"os"
	"strings"
)

// Read the installed theme, not a previous upload or a per-browser cache.
func installedPreview(config, logo string) ([]byte, int, bool) {
	conf, err := os.ReadFile(config)
	if err != nil {
		return defaultPreview()
	}
	section := ""
	theme := ""
	for _, line := range strings.Split(string(conf), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && section == "[Daemon]" && strings.TrimSpace(key) == "Theme" {
			theme = strings.TrimSpace(value)
		}
	}
	if theme != "opi-custom-logo" {
		return defaultPreview()
	}
	data, err := readLimited(logo, 12*1024*1024)
	if err != nil {
		return defaultPreview()
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return defaultPreview()
	}
	return data, max(cfg.Width, cfg.Height), true
}
func defaultPreview() ([]byte, int, bool) {
	data, _ := assets.ReadFile("assets/default.png")
	return data, 320, false
}
func (a *app) refreshPreview() {
	data, size, applied := installedPreview(configPath, themeDir+"/watermark.png")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.preview = data
	a.state.Size = size
	a.state.Applied = applied
	a.state.Spinner = installedPosition(applied)
	a.state.Background = installedBackground(applied)
	a.state.SpinnerItem = installedSpinnerItem(applied)
	a.state.SpinnerSize = installedSpinnerSize(applied)
	a.state.LogoPosition = installedLogoPosition(applied)
	a.state.Revision++
}
