package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func windowArgs(url, profile string) []string {
	return []string{"--app=" + url, "--user-data-dir=" + profile, "--window-size=856,803", "--class=plymouth-logo", "--ozone-platform=x11", "--no-first-run", "--no-default-browser-check", "--disable-background-mode"}
}

func appWindow(url string) (*exec.Cmd, func(), error) {
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, nil, err
		}
		base := filepath.Join(cache, "plymouth-logo")
		if err := os.MkdirAll(base, 0700); err != nil {
			return nil, nil, err
		}
		profile, err := os.MkdirTemp(base, "window-")
		if err != nil {
			return nil, nil, err
		}
		return exec.Command(path, windowArgs(url, profile)...), func() { os.RemoveAll(profile) }, nil
	}
	return nil, nil, fmt.Errorf("Chrome, Chromium, or Edge is required for the app window; use --no-browser for manual access")
}
