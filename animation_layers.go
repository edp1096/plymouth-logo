package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maxAnimationLayers = 8
const animationByteBudget int64 = 64 * 1024 * 1024

type animationSpec struct {
	Center   bool     `json:"center,omitempty"`
	Item     string   `json:"item"`
	Size     int      `json:"size"`
	FPS      float64  `json:"fps"`
	Position position `json:"position"`
}
type animationLayer struct {
	animationSpec
	Frames int `json:"frames"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type animationManifest struct {
	Version int              `json:"version"`
	Layers  []animationLayer `json:"layers"`
}

func animationSource(item string) (spinnerEntry, string, error) {
	item, err := spinnerItem(item)
	if err != nil {
		return spinnerEntry{}, "", err
	}
	dir := "assets/theme"
	var entry spinnerEntry
	if item == "default" {
		files, e := spinnerFrameFiles(dir)
		if e != nil {
			return entry, "", e
		}
		entry = spinnerEntry{ID: "default", Name: "Default ring", Frames: len(files), FPS: math.Max(1, float64(len(files))/2)}
	} else {
		dir = "assets/spinners/" + spinnerDirectories[item]
		entry, err = spinnerMetadata(item)
		if err != nil {
			return entry, "", err
		}
	}
	raw, err := assets.ReadFile(dir + "/throbber-0001.png")
	if err != nil {
		return entry, "", err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return entry, "", err
	}
	entry.Width, entry.Height = cfg.Width, cfg.Height
	return entry, dir, nil
}
func animationCatalog() ([]spinnerEntry, error) {
	result := []spinnerEntry{}
	for _, id := range append([]string{"default"}, animationIDs()...) {
		entry, _, err := animationSource(id)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}
func animationIDs() []string {
	ids := make([]string, 0, len(spinnerCatalog))
	for _, entry := range spinnerCatalog {
		ids = append(ids, entry.ID)
	}
	return ids
}
func animationDimensions(w, h, size int) (int, int) {
	if w >= h {
		return size, max(1, h*size/w)
	}
	return max(1, w*size/h), size
}
func validAnimationFPS(fps float64) bool {
	return !math.IsNaN(fps) && !math.IsInf(fps, 0) && fps >= 1 && fps <= 60
}
func resolveAnimationLayers(specs []animationSpec) ([]animationLayer, error) {
	if len(specs) > maxAnimationLayers {
		return nil, fmt.Errorf("use at most %d animations", maxAnimationLayers)
	}
	layers := make([]animationLayer, 0, len(specs))
	var total int64
	for i, spec := range specs {
		entry, dir, err := animationSource(spec.Item)
		if err != nil {
			return nil, fmt.Errorf("animation %d: %w", i+1, err)
		}
		spec.Item = entry.ID
		spec.Size, err = spinnerSize(spec.Item, spec.Size)
		if err != nil {
			return nil, err
		}
		if _, err = spinnerPosition(&spec.Position); err != nil {
			return nil, err
		}
		if spec.FPS == 0 {
			spec.FPS = entry.FPS
		}
		if !validAnimationFPS(spec.FPS) {
			return nil, fmt.Errorf("animation FPS must be between 1 and 60")
		}
		files, err := spinnerFrameFiles(dir)
		if err != nil {
			return nil, err
		}
		if len(files) != entry.Frames {
			return nil, fmt.Errorf("animation changed; restart the app")
		}
		w, h := animationDimensions(entry.Width, entry.Height, spec.Size)
		total += int64(w) * int64(h) * 4 * int64(entry.Frames)
		if total > animationByteBudget {
			return nil, fmt.Errorf("combined animations exceed 64 MiB; reduce sizes or remove an animation")
		}
		layers = append(layers, animationLayer{spec, entry.Frames, w, h})
	}
	return layers, nil
}
func animationFrameName(layer, frame int) string {
	return fmt.Sprintf("animations/layer-%02d/throbber-%04d.png", layer, frame)
}
func installAnimationLayers(dir string, layers []animationLayer) error {
	// The caller has already backed up the whole theme and handles rollback.
	if err := os.RemoveAll(filepath.Join(dir, "animations")); err != nil {
		return err
	}
	for i, layer := range layers {
		_, source, err := animationSource(layer.Item)
		if err != nil {
			return err
		}
		for frame := 1; frame <= layer.Frames; frame++ {
			raw, err := assets.ReadFile(fmt.Sprintf("%s/throbber-%04d.png", source, frame))
			if err != nil {
				return err
			}
			data, err := resizePNG(raw, layer.Size, true)
			if err != nil {
				return err
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || cfg.Width != layer.Width || cfg.Height != layer.Height {
				return fmt.Errorf("animation changed during apply; restart the app")
			}
			if err = atomicWrite(filepath.Join(dir, animationFrameName(i, frame)), data, 0644); err != nil {
				return err
			}
		}
	}
	data, err := json.Marshal(animationManifest{1, layers})
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "animations.json"), data, 0644)
}
func readAnimationManifest(dir string) ([]animationLayer, error) {
	raw, err := readLimited(filepath.Join(dir, "animations.json"), 64*1024)
	if err != nil {
		return nil, err
	}
	var m animationManifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || m.Layers == nil || len(m.Layers) > maxAnimationLayers {
		return nil, fmt.Errorf("invalid installed animation manifest")
	}
	var total int64
	for _, layer := range m.Layers {
		if !validAnimationFPS(layer.FPS) || layer.Frames < 1 || layer.Frames > 600 || layer.Size < 32 || layer.Size > 320 || layer.Width < 1 || layer.Height < 1 || max(layer.Width, layer.Height) != layer.Size {
			return nil, fmt.Errorf("invalid installed animation metadata")
		}
		if _, err = spinnerPosition(&layer.Position); err != nil {
			return nil, err
		}
		total += int64(layer.Width) * int64(layer.Height) * 4 * int64(layer.Frames)
	}
	if total > animationByteBudget {
		return nil, fmt.Errorf("installed animations exceed 64 MiB")
	}
	return m.Layers, nil
}
func installedAnimationLayers(applied bool) ([]animationLayer, string) {
	if !applied {
		return nil, ""
	}
	layers, err := readAnimationManifest(themeDir)
	if os.IsNotExist(err) {
		return nil, ""
	} // Legacy single-animation theme.
	if err != nil {
		return []animationLayer{}, "Could not read installed animations: " + err.Error()
	}
	return layers, ""
}
func animationLayerPreview(spec animationSpec) ([]byte, error) {
	layers, err := resolveAnimationLayers([]animationSpec{spec})
	if err != nil {
		return nil, err
	}
	layer := layers[0]
	_, dir, err := animationSource(layer.Item)
	if err != nil {
		return nil, err
	}
	return animationPNG(spinnerEntry{FPS: layer.FPS, Frames: layer.Frames}, func(frame int) ([]byte, error) {
		raw, err := assets.ReadFile(fmt.Sprintf("%s/throbber-%04d.png", dir, frame))
		if err != nil {
			return nil, err
		}
		return resizePNG(raw, layer.Size, true)
	})
}
func installedAnimationLayerPreview(index int) ([]byte, error) {
	layers, err := readAnimationManifest(themeDir)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(layers) {
		return nil, fmt.Errorf("invalid animation index")
	}
	layer := layers[index]
	return animationPNG(spinnerEntry{FPS: layer.FPS, Frames: layer.Frames}, func(frame int) ([]byte, error) {
		raw, err := readLimited(filepath.Join(themeDir, animationFrameName(index, frame)), 12*1024*1024)
		if err != nil {
			return nil, err
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width != layer.Width || cfg.Height != layer.Height {
			return nil, fmt.Errorf("installed animation dimensions differ from manifest")
		}
		return raw, nil
	})
}

func verifyAnimationArchive(listing string, layers []animationLayer) error {
	return verifyLayerArchive(listing, layers, "animations.json")
}
func verifyLayerArchive(listing string, layers []animationLayer, manifest string) error {
	files := map[string]bool{}
	for _, name := range strings.Split(listing, "\n") {
		files[strings.TrimPrefix(strings.TrimSpace(name), "./")] = true
	}
	required := []string{"usr/share/plymouth/themes/opi-custom-logo/" + manifest, "usr/share/plymouth/themes/opi-custom-logo/opi-custom-logo.script"}
	for _, plugin := range []string{"script.so", "label.so"} {
		found := false
		for name := range files {
			if strings.HasSuffix(name, "/plymouth/"+plugin) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("generated initramfs is missing %s", plugin)
		}
	}
	for i, layer := range layers {
		for frame := 1; frame <= layer.Frames; frame++ {
			required = append(required, "usr/share/plymouth/themes/opi-custom-logo/"+animationFrameName(i, frame))
		}
	}
	for _, name := range required {
		if !files[name] {
			return fmt.Errorf("generated initramfs is missing %s", name)
		}
	}
	return nil
}

func installedAnimationFPS(applied bool) float64 {
	if !applied {
		return 15
	}
	ini, err := readThemeINI(filepath.Join(themeDir, "opi-custom-logo.plymouth"))
	if err != nil {
		return 30
	}
	if ini["Plymouth Theme"]["ModuleName"] == "two-step" {
		files, _ := filepath.Glob(filepath.Join(themeDir, "throbber-*.png"))
		return math.Max(1, float64(len(files))/2)
	}
	fps, err := strconv.ParseFloat(ini["opi-logo"]["AnimationFPS"], 64)
	if err != nil || !validAnimationFPS(fps) {
		return 30
	}
	return fps
}
