package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func desktopIntegration(install bool, dataDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	base := userDataBase()
	binary := filepath.Join(home, ".local", "bin", "plymouth-logo")
	entry := filepath.Join(base, "applications", "plymouth-logo.desktop")
	icon := filepath.Join(base, "icons", "plymouth-logo.png")
	if install {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		binary = exe
		image, err := assets.ReadFile("web/icon.png")
		if err != nil {
			return err
		}
		if err = atomicWrite(icon, image, 0644); err != nil {
			return err
		}
		if strings.ContainsAny(binary+icon+dataDir, "\n\r") {
			return fmt.Errorf("paths containing newlines cannot be registered")
		}
		text := "[Desktop Entry]\nType=Application\nName=Plymouth Logo\nComment=Change the Plymouth boot logo\nExec=" + quoteDesktop(binary) + " --data-dir " + quoteDesktop(dataDir) + "\nIcon=" + icon + "\nStartupWMClass=plymouth-logo\nTerminal=false\nCategories=Settings;DesktopSettings;\nStartupNotify=false\n"
		if err = atomicWrite(entry, []byte(text), 0644); err != nil {
			return err
		}
		if check, err := exec.LookPath("desktop-file-validate"); err == nil {
			if err = command(check, entry); err != nil {
				return err
			}
		}
		fmt.Println("Registered the current executable and its icon in the application menu. No desktop shortcut created.")
	} else {
		for _, path := range []string{binary, entry, icon, filepath.Join(base, "icons/hicolor/scalable/apps/plymouth-logo.svg")} {
			if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		fmt.Println("Uninstalled desktop integration. Editable resources, boot theme, and backups are preserved.")
	}
	if refresh, err := exec.LookPath("update-desktop-database"); err == nil {
		if _, err = os.Stat(filepath.Dir(entry)); err == nil {
			return command(refresh, filepath.Dir(entry))
		}
	}
	return nil
}
