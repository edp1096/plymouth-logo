package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func backgroundColor(value string) (string, error) {
	if value == "" {
		return "#000000", nil
	}
	if !colorPattern.MatchString(value) {
		return "", fmt.Errorf("background must be a six-digit hex color")
	}
	return strings.ToLower(value), nil
}
func themeBackground(text, color string) string {
	lines := strings.Split(text, "\n")
	section := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			section = trim
		}
		key, _, ok := strings.Cut(trim, "=")
		if ok && section == "[two-step]" && (key == "BackgroundStartColor" || key == "BackgroundEndColor") {
			lines[i] = key + "=0x" + strings.TrimPrefix(color, "#")
		}
	}
	return strings.Join(lines, "\n")
}
func installedBackground(applied bool) string {
	if !applied {
		return "#000000"
	}
	data, err := os.ReadFile(themeDir + "/opi-custom-logo.plymouth")
	if err != nil {
		return "#000000"
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "BackgroundStartColor" {
			color, err := backgroundColor("#" + strings.TrimPrefix(value, "0x"))
			if err == nil {
				return color
			}
		}
	}
	return "#000000"
}
