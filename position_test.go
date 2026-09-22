package main

import (
	"strings"
	"testing"
)

func TestSpinnerPosition(t *testing.T) {
	p, err := spinnerPosition(nil)
	if err != nil || p.X != 50 || p.Y != 70 {
		t.Fatal(p, err)
	}
	for _, p := range []position{{-1, 70}, {50, 101}} {
		if _, err := spinnerPosition(&p); err == nil {
			t.Fatal("out of range accepted")
		}
	}
	source := "[two-step]\nHorizontalAlignment=.5\nVerticalAlignment=.7\nWatermarkVerticalAlignment=.5\nDialogVerticalAlignment=.7\n[other]\nVerticalAlignment=.9\n"
	got := themePosition(source, position{25, 85})
	for _, line := range []string{"HorizontalAlignment=0.2500", "VerticalAlignment=0.8500", "WatermarkVerticalAlignment=.5", "DialogVerticalAlignment=.7", "[other]\nVerticalAlignment=.9"} {
		if !strings.Contains(got, line) {
			t.Fatal(got)
		}
	}
}

func TestLogoPosition(t *testing.T) {
	p, err := logoPosition(nil)
	if err != nil || p.X != 50 || p.Y != 50 {
		t.Fatal(p, err)
	}
	if _, err = logoPosition(&position{101, 50}); err == nil {
		t.Fatal("invalid position accepted")
	}
	text := "[two-step]\nWatermarkHorizontalAlignment=.5\nWatermarkVerticalAlignment=.5\nHorizontalAlignment=.5\nVerticalAlignment=.7\nDialogVerticalAlignment=.7\n"
	result := themeLogoPosition(text, position{20, 35})
	for _, want := range []string{"WatermarkHorizontalAlignment=0.2000", "WatermarkVerticalAlignment=0.3500", "\nHorizontalAlignment=.5", "\nVerticalAlignment=.7", "DialogVerticalAlignment=.7"} {
		if !strings.Contains(result, want) {
			t.Fatal(result)
		}
	}
}
