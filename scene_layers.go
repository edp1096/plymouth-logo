package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

const maxSceneLayers = 16
const maxImageLayers = 8
const sceneImageByteBudget int64 = 16 * 1024 * 1024
const sceneSourceBudget = 32 * 1024 * 1024
const sceneRequestLimit = 44 * 1024 * 1024

type sceneSpec struct {
	animationSpec
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`
}
type sceneLayer struct {
	sceneSpec
	Frames       int `json:"frames"`
	Width        int `json:"width"`
	Height       int `json:"height"`
	SourceWidth  int `json:"source_width,omitempty"`
	SourceHeight int `json:"source_height,omitempty"`
}
type sceneManifest struct {
	Version int          `json:"version"`
	Layers  []sceneLayer `json:"layers"`
}

func decodeSceneImage(encoded string) ([]byte, image.Config, error) {
	if len(encoded) > base64.StdEncoding.EncodedLen(12*1024*1024) {
		return nil, image.Config{}, fmt.Errorf("image exceeds 12 MiB")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, image.Config{}, fmt.Errorf("invalid image encoding")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return nil, image.Config{}, fmt.Errorf("use a valid PNG/JPG image up to 20 megapixels")
	}
	return raw, cfg, nil
}
func resolveSceneLayers(specs []sceneSpec) ([]sceneLayer, error) {
	if len(specs) > maxSceneLayers {
		return nil, fmt.Errorf("use at most 8 images and 8 animations")
	}
	layers := make([]sceneLayer, 0, len(specs))
	images, animations := 0, 0
	var imageMemory, animationMemory int64
	sourceBytes := 0
	for i, spec := range specs {
		if _, err := spinnerPosition(&spec.Position); err != nil {
			return nil, err
		}
		if len(spec.Name) > 256 {
			return nil, fmt.Errorf("layer name is too long")
		}
		layer := sceneLayer{sceneSpec: spec}
		switch spec.Kind {
		case "image":
			images++
			if spec.Size < 1 || spec.Size > 1920 {
				return nil, fmt.Errorf("image size must be 1–1920 pixels")
			}
			raw, cfg, err := decodeSceneImage(spec.Image)
			if err != nil {
				return nil, fmt.Errorf("layer %d: %w", i+1, err)
			}
			sourceBytes += len(raw)
			if sourceBytes > sceneSourceBudget {
				return nil, fmt.Errorf("combined source images exceed 32 MiB")
			}
			// Decode fully before authentication/mutation; a valid header alone is insufficient.
			if _, _, err = image.Decode(bytes.NewReader(raw)); err != nil {
				return nil, fmt.Errorf("layer %d: invalid image: %w", i+1, err)
			}
			layer.Width, layer.Height = animationDimensions(cfg.Width, cfg.Height, spec.Size)
			layer.SourceWidth, layer.SourceHeight = cfg.Width, cfg.Height
			layer.Frames, layer.FPS = 1, 1
			layer.Item = ""
		case "animation":
			animations++
			if spec.Image != "" {
				return nil, fmt.Errorf("animation cannot contain a static image")
			}
			resolved, err := resolveAnimationLayers([]animationSpec{spec.animationSpec})
			if err != nil {
				return nil, err
			}
			a := resolved[0]
			layer.animationSpec = a.animationSpec
			layer.Frames, layer.Width, layer.Height = a.Frames, a.Width, a.Height
		default:
			return nil, fmt.Errorf("unknown layer type")
		}
		if images > maxImageLayers || animations > maxAnimationLayers {
			return nil, fmt.Errorf("use at most 8 images and 8 animations")
		}
		memory := int64(layer.Width) * int64(layer.Height) * 4 * int64(layer.Frames)
		if layer.Kind == "image" {
			imageMemory += memory
		} else {
			animationMemory += memory
		}
		if imageMemory > sceneImageByteBudget {
			return nil, fmt.Errorf("combined static images exceed 16 MiB")
		}
		if animationMemory > animationByteBudget {
			return nil, fmt.Errorf("combined animations exceed 64 MiB")
		}
		layers = append(layers, layer)
	}
	return layers, nil
}
func sceneSourceName(i int) string { return fmt.Sprintf("sources/layer-%02d", i) }
func sceneAnimations(layers []sceneLayer) []animationLayer {
	result := make([]animationLayer, 0, len(layers))
	for _, l := range layers {
		result = append(result, animationLayer{l.animationSpec, l.Frames, l.Width, l.Height})
	}
	return result
}
func installSceneLayers(dir string, layers []sceneLayer) error {
	for _, name := range []string{"animations", "sources"} {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	saved := make([]sceneLayer, 0, len(layers))
	for i, l := range layers {
		if l.Kind == "image" {
			raw, _, err := decodeSceneImage(l.Image)
			if err != nil {
				return err
			}
			scaled, err := resizePNG(raw, l.Size, true)
			if err != nil {
				return err
			}
			if err = atomicWrite(filepath.Join(dir, sceneSourceName(i)), raw, 0644); err != nil {
				return err
			}
			if err = atomicWrite(filepath.Join(dir, animationFrameName(i, 1)), scaled, 0644); err != nil {
				return err
			}
		} else {
			_, source, err := animationSource(l.Item)
			if err != nil {
				return err
			}
			for f := 1; f <= l.Frames; f++ {
				raw, err := assets.ReadFile(fmt.Sprintf("%s/throbber-%04d.png", source, f))
				if err != nil {
					return err
				}
				scaled, err := resizePNG(raw, l.Size, true)
				if err != nil {
					return err
				}
				cfg, _, err := image.DecodeConfig(bytes.NewReader(scaled))
				if err != nil || cfg.Width != l.Width || cfg.Height != l.Height {
					return fmt.Errorf("animation changed during apply")
				}
				if err = atomicWrite(filepath.Join(dir, animationFrameName(i, f)), scaled, 0644); err != nil {
					return err
				}
			}
		}
		l.Image = ""
		saved = append(saved, l)
	}
	raw, err := json.Marshal(sceneManifest{1, saved})
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "layers.json"), raw, 0644)
}
func readSceneManifest(dir string) ([]sceneLayer, error) {
	raw, err := readLimited(filepath.Join(dir, "layers.json"), 64*1024)
	if err != nil {
		return nil, err
	}
	var m sceneManifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || m.Layers == nil || len(m.Layers) > maxSceneLayers {
		return nil, fmt.Errorf("invalid layer manifest")
	}
	images, animations := 0, 0
	var imageMemory, animationMemory int64
	for _, l := range m.Layers {
		if _, err = spinnerPosition(&l.Position); err != nil {
			return nil, err
		}
		if l.Image != "" || len(l.Name) > 256 || l.Width < 1 || l.Height < 1 || max(l.Width, l.Height) != l.Size {
			return nil, fmt.Errorf("invalid layer metadata")
		}
		switch l.Kind {
		case "image":
			images++
			if l.Size < 1 || l.Size > 1920 || l.Frames != 1 || l.FPS != 1 || l.SourceWidth < 1 || l.SourceHeight < 1 || l.SourceWidth > 20_000_000 || l.SourceHeight > 20_000_000 || int64(l.SourceWidth)*int64(l.SourceHeight) > 20_000_000 {
				return nil, fmt.Errorf("invalid image metadata")
			}
			w, h := animationDimensions(l.SourceWidth, l.SourceHeight, l.Size)
			if w != l.Width || h != l.Height {
				return nil, fmt.Errorf("invalid image proportions")
			}
		case "animation":
			animations++
			if l.Size < 32 || l.Size > 320 || l.Frames < 1 || l.Frames > 600 || !validAnimationFPS(l.FPS) {
				return nil, fmt.Errorf("invalid animation metadata")
			}
		default:
			return nil, fmt.Errorf("unknown installed layer type")
		}
		memory := int64(l.Width) * int64(l.Height) * 4 * int64(l.Frames)
		if l.Kind == "image" {
			imageMemory += memory
		} else {
			animationMemory += memory
		}
	}
	if images > maxImageLayers || animations > maxAnimationLayers || imageMemory > sceneImageByteBudget || animationMemory > animationByteBudget {
		return nil, fmt.Errorf("installed layers exceed limits")
	}
	return m.Layers, nil
}
func installedSceneLayers(applied bool) ([]sceneLayer, string) {
	if !applied {
		return nil, ""
	}
	layers, err := readSceneManifest(themeDir)
	if os.IsNotExist(err) {
		return nil, ""
	}
	if err != nil {
		return []sceneLayer{}, "Could not read installed layers: " + err.Error()
	}
	return layers, ""
}
func installedScenePreview(index int, source bool) ([]byte, error) {
	layers, err := readSceneManifest(themeDir)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(layers) {
		return nil, fmt.Errorf("invalid layer index")
	}
	l := layers[index]
	if source {
		if l.Kind != "image" {
			return nil, fmt.Errorf("layer is not a static image")
		}
		raw, err := readLimited(filepath.Join(themeDir, sceneSourceName(index)), 12*1024*1024)
		if err != nil {
			return nil, err
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width != l.SourceWidth || cfg.Height != l.SourceHeight {
			return nil, fmt.Errorf("invalid installed source image")
		}
		return raw, nil
	}
	frame := func(f int) ([]byte, error) {
		raw, err := readLimited(filepath.Join(themeDir, animationFrameName(index, f)), 16*1024*1024)
		if err != nil {
			return nil, err
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || cfg.Width != l.Width || cfg.Height != l.Height {
			return nil, fmt.Errorf("installed layer dimensions differ from manifest")
		}
		return raw, nil
	}
	if l.Kind == "image" {
		return frame(1)
	}
	return animationPNG(spinnerEntry{FPS: l.FPS, Frames: l.Frames}, frame)
}
func transparentWatermark() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}
func sceneScriptTheme(layers []sceneLayer, background string) (string, string) {
	a := sceneAnimations(layers)
	prefixes := make([]string, len(a))
	for i := range prefixes {
		prefixes[i] = fmt.Sprintf("animations/layer-%02d/", i)
	}
	return renderScriptThemeWithLogo(a, prefixes, background, position{50, 50}, false)
}
