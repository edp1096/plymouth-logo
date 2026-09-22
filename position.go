package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func spinnerPosition(p *position) (position, error) {
	if p == nil {
		return position{50, 70}, nil
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p.X < 0 || p.X > 100 || p.Y < 0 || p.Y > 100 {
		return position{}, fmt.Errorf("spinner position must be between 0 and 100 percent")
	}
	return *p, nil
}
func themePosition(text string, p position) string {
	section := ""
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			section = trim
		}
		key, _, ok := strings.Cut(trim, "=")
		if ok && section == "[two-step]" {
			switch strings.TrimSpace(key) {
			case "HorizontalAlignment":
				lines[i] = fmt.Sprintf("HorizontalAlignment=%.4f", p.X/100)
			case "VerticalAlignment":
				lines[i] = fmt.Sprintf("VerticalAlignment=%.4f", p.Y/100)
			}
		}
	}
	return strings.Join(lines, "\n")
}
func installedPosition(applied bool) position {
	p := position{50, 70}
	if !applied {
		return p
	}
	data, err := os.ReadFile(themeDir + "/opi-custom-logo.plymouth")
	if err != nil {
		return p
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || (section != "[two-step]" && section != "[opi-logo]") {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(key) {
		case "HorizontalAlignment":
			p.X = n * 100
		case "VerticalAlignment":
			p.Y = n * 100
		}
	}
	if _, err := spinnerPosition(&p); err != nil {
		return position{50, 70}
	}
	return p
}

func logoPosition(p *position) (position, error) {
	if p == nil {
		return position{50, 50}, nil
	}
	if _, err := spinnerPosition(p); err != nil {
		return position{}, fmt.Errorf("logo position must be between 0 and 100 percent")
	}
	return *p, nil
}
func themeLogoPosition(text string, p position) string {
	section := ""
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			section = trim
		}
		key, _, ok := strings.Cut(trim, "=")
		if ok && section == "[two-step]" {
			switch strings.TrimSpace(key) {
			case "WatermarkHorizontalAlignment":
				lines[i] = fmt.Sprintf("WatermarkHorizontalAlignment=%.4f", p.X/100)
			case "WatermarkVerticalAlignment":
				lines[i] = fmt.Sprintf("WatermarkVerticalAlignment=%.4f", p.Y/100)
			}
		}
	}
	return strings.Join(lines, "\n")
}
func installedLogoPosition(applied bool) position {
	p := position{50, 50}
	if !applied {
		return p
	}
	data, err := os.ReadFile(themeDir + "/opi-custom-logo.plymouth")
	if err != nil {
		return p
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || (section != "[two-step]" && section != "[opi-logo]") {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(key) {
		case "WatermarkHorizontalAlignment":
			p.X = n * 100
		case "WatermarkVerticalAlignment":
			p.Y = n * 100
		}
	}
	if _, err := logoPosition(&p); err != nil {
		return position{50, 50}
	}
	return p
}
