package main

import (
	"bytes"
	"image"
	"path/filepath"
)

func readLogoPreview(logo string) ([]byte, int, bool) {
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

// The daemon's theme selection can override plymouthd.conf (for example via
// plymouth.splash). Only use the custom preview when that exact theme is selected.
func selectedCustomPreview(paths themePreviewPaths, customDir string) ([]byte, int, bool) {
	selected, err := paths.resolve("current")
	if err != nil {
		return defaultPreview()
	}
	selected, err = filepath.EvalSymlinks(selected)
	if err != nil {
		return defaultPreview()
	}
	custom, err := filepath.EvalSymlinks(filepath.Join(customDir, "opi-custom-logo.plymouth"))
	if err != nil || selected != custom {
		return defaultPreview()
	}
	return readLogoPreview(filepath.Join(customDir, "watermark.png"))
}
func defaultPreview() ([]byte, int, bool) {
	data, _ := assets.ReadFile("assets/default.png")
	return data, 320, false
}
func (a *app) refreshPreview() {
	data, size, applied := selectedCustomPreview(installedThemePaths(), themeDir)
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
	a.state.Animations, a.state.AnimationError = installedAnimationLayers(applied)
	a.state.SpinnerFPS = installedAnimationFPS(applied)
	a.state.Layers, a.state.LayerError = installedSceneLayers(applied)
	a.state.Revision++
}
