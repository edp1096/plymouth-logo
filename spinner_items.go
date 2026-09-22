package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type spinnerEntry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
	FPS     float64  `json:"fps,omitempty"`
	Frames  int      `json:"frames"`
}

var spinnerDirectories, spinnerCatalog, spinnerAliases = loadSpinnerCatalog()

func loadSpinnerCatalog() (map[string]string, []spinnerEntry, map[string]string) {
	dirs := map[string]string{}
	catalog := []spinnerEntry{}
	aliases := map[string]string{}
	entries, err := assets.ReadDir("assets/spinners")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		raw, err := assets.ReadFile("assets/spinners/" + entry.Name() + "/item.json")
		if err != nil {
			panic(err)
		}
		var item spinnerEntry
		if err = json.Unmarshal(raw, &item); err != nil {
			panic(err)
		}
		if item.ID == "" || item.ID == "default" || strings.ContainsAny(item.ID, "/\\ .") || item.Name == "" {
			panic("invalid spinner metadata: " + entry.Name())
		}
		if _, exists := dirs[item.ID]; exists {
			panic("duplicate spinner id: " + item.ID)
		}
		if item.FPS == 0 {
			item.FPS = 30
		}
		if item.FPS < 1 || item.FPS > 60 {
			panic("spinner fps must be between 1 and 60: " + item.ID)
		}
		frames, err := spinnerFrameFiles("assets/spinners/" + entry.Name())
		if err != nil {
			panic(err)
		}
		item.Frames = len(frames)
		dirs[item.ID] = entry.Name()
		catalog = append(catalog, item)
		for _, alias := range item.Aliases {
			if _, exists := aliases[alias]; exists {
				panic("duplicate spinner alias")
			}
			aliases[alias] = item.ID
		}
	}
	for alias := range aliases {
		if _, exists := dirs[alias]; exists || alias == "default" {
			panic("spinner alias conflicts with id")
		}
	}
	return dirs, catalog, aliases
}

func spinnerItem(value string) (string, error) {
	if value == "" || value == "default" {
		return "default", nil
	}
	if canonical, ok := spinnerAliases[value]; ok {
		value = canonical
	}
	if _, ok := spinnerDirectories[value]; !ok {
		return "", fmt.Errorf("unknown spinner item")
	}
	return value, nil
}
func installedSpinnerItem(applied bool) string {
	if !applied {
		return "default"
	}
	data, err := os.ReadFile(themeDir + "/spinner-item")
	if err != nil {
		return "default"
	}
	item, err := spinnerItem(strings.TrimSpace(string(data)))
	if err != nil {
		return "default"
	}
	return item
}

// All custom frames are bundled and validated; no upload paths reach the theme directory.
func installSpinnerItem(dir, item string, sizes ...int) error {
	requested := 0
	if len(sizes) > 0 {
		requested = sizes[0]
	}
	size, err := spinnerSize(item, requested)
	if err != nil {
		return err
	}
	item, err = spinnerItem(item)
	if err != nil {
		return err
	}
	if item != "default" {
		files, err := filepath.Glob(filepath.Join(dir, "throbber-*.png"))
		if err != nil {
			return err
		}
		for _, path := range files {
			if err = os.Remove(path); err != nil {
				return err
			}
		}
		entry, err := spinnerMetadata(item)
		if err != nil {
			return err
		}
		for i := 1; i <= entry.Frames; i++ {
			name := fmt.Sprintf("throbber-%04d.png", i)
			data, err := assets.ReadFile("assets/spinners/" + spinnerDirectories[item] + "/" + name)
			if err != nil {
				return err
			}
			data, err = resizePNG(data, size, true)
			if err != nil {
				return err
			}
			if err = atomicWrite(filepath.Join(dir, name), data, 0644); err != nil {
				return err
			}
		}
	}
	if item == "default" {
		files, err := filepath.Glob(filepath.Join(dir, "throbber-*.png"))
		if err != nil {
			return err
		}
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			data, err = resizePNG(data, size, true)
			if err != nil {
				return err
			}
			if err = atomicWrite(path, data, 0644); err != nil {
				return err
			}
		}
	}
	return atomicWrite(filepath.Join(dir, "spinner-item"), []byte(item), 0644)
}

func spinnerSize(item string, requested int) (int, error) {
	if _, err := spinnerItem(item); err != nil {
		return 0, err
	}
	if requested == 0 {
		if item == "" || item == "default" {
			return 32, nil
		}
		return 160, nil
	}
	if requested < 32 || requested > 320 {
		return 0, fmt.Errorf("spinner size must be 32–320 pixels")
	}
	return requested, nil
}
func installedSpinnerSize(applied bool) int {
	if !applied {
		return 32
	}
	data, err := os.ReadFile(themeDir + "/throbber-0001.png")
	if err == nil {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err == nil {
			return max(cfg.Width, cfg.Height)
		}
	}
	size, _ := spinnerSize(installedSpinnerItem(applied), 0)
	return size
}

// Frames are discovered, not taken from an untrusted metadata count.
func spinnerFrameFiles(dir string) ([]string, error) {
	entries, err := assets.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	pattern := regexp.MustCompile(`^throbber-[0-9]{4}\.png$`)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "throbber-") {
			if entry.IsDir() || !pattern.MatchString(entry.Name()) {
				return nil, fmt.Errorf("invalid spinner frame: %s", entry.Name())
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 || len(names) > 600 {
		return nil, fmt.Errorf("%s: use 1–600 spinner frames", dir)
	}
	var pixels int64
	var first image.Config
	for i, name := range names {
		if name != fmt.Sprintf("throbber-%04d.png", i+1) {
			return nil, fmt.Errorf("%s: frame numbers must start at 0001 without gaps", dir)
		}
		raw, err := assets.ReadFile(dir + "/" + name)
		if err != nil {
			return nil, err
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || format != "png" || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 1024 || cfg.Height > 1024 {
			return nil, fmt.Errorf("invalid PNG frame: %s/%s", dir, name)
		}
		if i == 0 {
			first = cfg
		}
		if cfg.Width != first.Width || cfg.Height != first.Height {
			return nil, fmt.Errorf("spinner frames must have identical dimensions: %s", dir)
		}
		pixels += int64(cfg.Width) * int64(cfg.Height)
	}
	if pixels*4 > 64*1024*1024 {
		return nil, fmt.Errorf("spinner exceeds 64 MiB decoded source image budget: %s", dir)
	}
	return names, nil
}
func spinnerMetadata(id string) (spinnerEntry, error) {
	id, err := spinnerItem(id)
	if err != nil {
		return spinnerEntry{}, err
	}
	for _, entry := range spinnerCatalog {
		if entry.ID == id {
			return entry, nil
		}
	}
	return spinnerEntry{}, fmt.Errorf("no custom animation metadata for %s", id)
}
func spinnerMemoryBudget(entry spinnerEntry, size int) error {
	// Conservative upper bound after resizing; keeps early-boot memory bounded.
	if size < 1 || int64(entry.Frames)*int64(size)*int64(size)*4 > 64*1024*1024 {
		return fmt.Errorf("animation at this size exceeds 64 MiB; reduce spinner size or frame count")
	}
	return nil
}
