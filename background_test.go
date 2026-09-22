package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPresetBackground(t *testing.T) {
	var presets []struct {
		ID         string   `json:"id"`
		Size       int      `json:"size"`
		Spinner    position `json:"spinner"`
		Background string   `json:"background"`
	}
	data, err := assets.ReadFile("web/presets.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &presets); err != nil {
		t.Fatal(err)
	}
	if len(presets) != 5 || presets[0].ID != "basic" {
		t.Fatal("missing presets")
	}
	for _, p := range presets {
		if _, err := spinnerPosition(&p.Spinner); err != nil {
			t.Fatal(err)
		}
		if _, err := backgroundColor(p.Background); err != nil {
			t.Fatal(err)
		}
		if p.Size < 32 || p.Size > 1024 {
			t.Fatal(p)
		}
	}
	got := themeBackground("[two-step]\nBackgroundStartColor=0x000000\nBackgroundEndColor=0x000000\nWatermarkVerticalAlignment=.5\n", "#101827")
	if strings.Count(got, "0x101827") != 2 || !strings.Contains(got, "WatermarkVerticalAlignment=.5") {
		t.Fatal(got)
	}
	if _, err = backgroundColor("#000000\nModuleName=script"); err == nil {
		t.Fatal("invalid color accepted")
	}
}
