package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

var stateDir = "/var/lib/opi-plymouth-logo"
var themeDir = "/usr/share/plymouth/themes/opi-custom-logo"
var configPath = "/etc/plymouth/plymouthd.conf"
var hookPath = "/etc/initramfs-tools/hooks/opi-custom-logo"
var bootImage = "/boot/firmware/initrd.img"

const kernelVersion = "5.10.160-rockchip"

var command = func(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", name, err, out)
	}
	return nil
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".logo-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	syscall.Sync()
	return nil
}
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicWrite(dst, data, 0644)
}
func checkHost() error {
	board, err := os.ReadFile("/proc/device-tree/model")
	if err != nil || !strings.Contains(string(board), "Orange Pi 5 Plus") {
		return fmt.Errorf("this tool is for Orange Pi 5 Plus")
	}
	kernel, err := exec.Command("uname", "-r").Output()
	if err != nil || strings.TrimSpace(string(kernel)) != kernelVersion {
		return fmt.Errorf("boot the original %s kernel first", kernelVersion)
	}
	cmdline, err := os.ReadFile("/proc/cmdline")
	if err != nil || !strings.Contains(string(cmdline), "rknpu_boot.path=stable-fallback") {
		return fmt.Errorf("use the normal original-kernel boot path first")
	}
	if err = command("mountpoint", "-q", "/boot/firmware"); err != nil {
		return err
	}
	info, err := os.Lstat(bootImage)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("unexpected normal boot initramfs")
	}
	for _, flag := range []string{"rnold.flg", "rnnew.flg"} {
		b, e := os.ReadFile("/boot/firmware/" + flag)
		if e != nil || string(b) != "0" {
			return fmt.Errorf("cancel pending kernel trials first")
		}
	}
	return nil
}
func configTheme(original string) string {
	lines := strings.Split(original, "\n")
	result := []string{}
	inDaemon := false
	found := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			inDaemon = trim == "[Daemon]"
			result = append(result, line)
			if inDaemon {
				result = append(result, "Theme=opi-custom-logo")
				found = true
			}
			continue
		}
		if inDaemon && strings.TrimSpace(strings.SplitN(trim, "=", 2)[0]) == "Theme" && strings.Contains(trim, "=") {
			continue
		}
		result = append(result, line)
	}
	if !found {
		result = append(result, "[Daemon]", "Theme=opi-custom-logo")
	}
	return strings.Join(result, "\n") + "\n"
}
func backupCurrent() (string, error) {
	dir, err := os.MkdirTemp(stateDir, "backup-"+time.Now().Format("20060102-150405")+"-")
	if err != nil {
		return "", err
	}
	if err = copyFile(bootImage, dir+"/initrd.img"); err != nil {
		return "", err
	}
	for name, path := range map[string]string{"config": configPath, "hook": hookPath} {
		if exists(path) {
			if err = copyFile(path, dir+"/"+name); err != nil {
				return "", err
			}
		}
	}
	if exists(themeDir) {
		if err = command("cp", "-a", themeDir, dir+"/theme"); err != nil {
			return "", err
		}
	}
	data, err := os.ReadFile(dir + "/initrd.img")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if err = atomicWrite(dir+"/sha256", []byte(hex.EncodeToString(sum[:])), 0600); err != nil {
		return "", err
	}
	return dir, nil
}
func restoreBackup(dir string) error {
	data, err := os.ReadFile(dir + "/initrd.img")
	if err != nil {
		return err
	}
	expected, err := os.ReadFile(dir + "/sha256")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != strings.TrimSpace(string(expected)) {
		return fmt.Errorf("backup checksum mismatch")
	}
	if err = atomicWrite(bootImage, data, 0644); err != nil {
		return err
	}
	for name, path := range map[string]string{"config": configPath, "hook": hookPath} {
		if exists(dir + "/" + name) {
			if err = copyFile(dir+"/"+name, path); err != nil {
				return err
			}
			if name == "hook" {
				if err = os.Chmod(path, 0755); err != nil {
					return err
				}
			}
		} else {
			if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	if err = os.RemoveAll(themeDir); err != nil {
		return err
	}
	if exists(dir + "/theme") {
		return command("cp", "-a", dir+"/theme", themeDir)
	}
	return nil
}
func applyLogo(pngData []byte, positions ...position) error {
	return applyTheme(pngData, "#000000", positions...)
}

func applyTheme(pngData []byte, background string, positions ...position) error {
	return applyThemeItem(pngData, background, "default", positions...)
}

func applyThemeItem(pngData []byte, background, item string, positions ...position) error {
	return applyThemeSized(pngData, background, item, 0, positions...)
}

func applyThemeSized(pngData []byte, background, item string, size int, positions ...position) (err error) {
	size, err = spinnerSize(item, size)
	if err != nil {
		return err
	}
	item, err = spinnerItem(item)
	if err != nil {
		return err
	}
	background, err = backgroundColor(background)
	if err != nil {
		return err
	}
	p := position{50, 70}
	if len(positions) > 0 {
		p = positions[0]
	}
	if _, err = spinnerPosition(&p); err != nil {
		return err
	}
	lp := position{50, 50}
	if len(positions) > 1 {
		lp = positions[1]
	}
	if _, err = logoPosition(&lp); err != nil {
		return err
	}
	var animation spinnerEntry
	if item != "default" {
		animation, err = spinnerMetadata(item)
		if err != nil {
			return err
		}
		if err = spinnerMemoryBudget(animation, size); err != nil {
			return err
		}
		// Validate editable frames again immediately before any privileged mutation.
		frames, e := spinnerFrameFiles("assets/spinners/" + spinnerDirectories[item])
		if e != nil {
			return e
		}
		if len(frames) != animation.Frames {
			return fmt.Errorf("spinner changed; restart the app")
		}
	}
	backup, err := backupCurrent()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rollback := restoreBackup(backup); rollback != nil {
				err = fmt.Errorf("%w; recovery failed: %v; backup: %s", err, rollback, backup)
			}
		}
	}()
	if err = os.MkdirAll(themeDir, 0755); err != nil {
		return err
	}
	oldFrames, err := filepath.Glob(filepath.Join(themeDir, "throbber-*.png"))
	if err != nil {
		return err
	}
	for _, path := range oldFrames {
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	files, err := assets.ReadDir("assets/theme")
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".png") {
			continue
		}
		data, err := assets.ReadFile("assets/theme/" + file.Name())
		if err != nil {
			return err
		}
		if err = atomicWrite(filepath.Join(themeDir, file.Name()), data, 0644); err != nil {
			return err
		}
	}

	if err = installSpinnerItem(themeDir, item, size); err != nil {
		return err
	}
	if err = atomicWrite(themeDir+"/watermark.png", pngData, 0644); err != nil {
		return err
	}
	template, err := assets.ReadFile("assets/theme/bgrt.plymouth")
	if err != nil {
		return err
	}
	text := strings.NewReplacer("Name=BGRT", "Name=OPi custom logo", "ImageDir=/usr/share/plymouth/themes/spinner", "ImageDir="+themeDir, "WatermarkVerticalAlignment=.96", "WatermarkVerticalAlignment=.5", "UseFirmwareBackground=true", "UseFirmwareBackground=false", "DialogClearsFirmwareBackground=false", "DialogClearsFirmwareBackground=true").Replace(string(template))
	text = themeBackground(themeLogoPosition(themePosition(text, p), lp), background)
	if item != "default" {
		var script string
		text, script = scriptTheme(animation, background, p, lp)
		if err = atomicWrite(themeDir+"/opi-custom-logo.script", []byte(script), 0644); err != nil {
			return err
		}
	} else {
		if err = os.Remove(themeDir + "/opi-custom-logo.script"); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err = atomicWrite(themeDir+"/opi-custom-logo.plymouth", []byte(text), 0644); err != nil {
		return err
	}
	if err = atomicWrite(hookPath, []byte(customThemeHook), 0755); err != nil {
		return err
	}

	original, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = atomicWrite(configPath, []byte(configTheme(string(original))), 0644); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(stateDir, "build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	image := dir + "/initrd.img"
	if err = command("mkinitramfs", "-o", image, kernelVersion); err != nil {
		return err
	}
	listing, err := exec.Command("lsinitramfs", image).Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(listing), "usr/share/plymouth/themes/opi-custom-logo/watermark.png") || !strings.Contains(string(listing), "lib/modules/"+kernelVersion) {
		return fmt.Errorf("generated initramfs is missing the logo or matching modules")
	}
	if item != "default" {
		for _, required := range []string{"/plymouth/script.so", "/plymouth/label.so", "usr/share/plymouth/themes/opi-custom-logo/opi-custom-logo.script", fmt.Sprintf("usr/share/plymouth/themes/opi-custom-logo/throbber-%04d.png", animation.Frames)} {
			if !strings.Contains(string(listing), required) {
				return fmt.Errorf("generated initramfs is missing %s", required)
			}
		}
	}
	info, err := os.Stat(image)
	if err != nil {
		return err
	}
	var fs syscall.Statfs_t
	if err = syscall.Statfs(filepath.Dir(bootImage), &fs); err != nil {
		return err
	}
	if fs.Bavail*uint64(fs.Bsize) < uint64(info.Size())+16*1024*1024 {
		return fmt.Errorf("insufficient boot partition space")
	}
	if err = copyFile(image, bootImage); err != nil {
		return err
	}
	if err = atomicWrite(stateDir+"/latest", []byte(filepath.Base(backup)), 0600); err != nil {
		return err
	}
	fmt.Printf("Applied. Reboot manually when ready.\nBackup: %s\nKernel trial slots are unchanged.\n", backup)
	return nil
}
func helper(args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator authentication is required")
	}
	// Do not inherit executable search paths or interpreter settings from the desktop.
	os.Setenv("PATH", "/usr/sbin:/usr/bin:/sbin:/bin")
	if len(args) < 1 || (args[0] != "apply" && args[0] != "restore" && args[0] != "restore-original") {
		return fmt.Errorf("expected apply or restore")
	}
	if err := checkHost(); err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(stateDir+"/lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another logo operation is running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if args[0] == "restore-original" {
		if len(args) != 1 {
			return fmt.Errorf("unexpected restore arguments")
		}
		backup, err := originalBackup(stateDir)
		if err != nil {
			return err
		}
		if err = restoreBackup(backup); err != nil {
			return err
		}
		fmt.Println("Restored the original pre-customization backup. Reboot manually when ready.")
		return nil
	}
	if args[0] == "restore" {
		if len(args) != 1 {
			return fmt.Errorf("unexpected restore arguments")
		}
		name, err := os.ReadFile(stateDir + "/latest")
		if err != nil {
			return fmt.Errorf("no backup is available: %w", err)
		}
		base := strings.TrimSpace(string(name))
		if filepath.Base(base) != base || !strings.HasPrefix(base, "backup-") {
			return fmt.Errorf("invalid backup path")
		}
		if err = restoreBackup(stateDir + "/" + base); err != nil {
			return err
		}
		fmt.Println("Restored the state before the most recent apply. Reboot manually when ready.")
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("apply requires an image")
	}
	raw, err := readLimited(args[1], 17*1024*1024)
	if err != nil {
		return err
	}
	var request payload
	if err = json.Unmarshal(raw, &request); err != nil {
		return err
	}
	pos, err := spinnerPosition(request.Spinner)
	if err != nil {
		return err
	}
	raw, err = base64.StdEncoding.DecodeString(request.Image)
	if err != nil {
		return err
	}
	data, err := normalize(raw, 1024)
	if err != nil {
		return err
	}
	lp, err := logoPosition(request.LogoPosition)
	if err != nil {
		return err
	}
	return applyThemeSized(data, request.Background, request.SpinnerItem, request.SpinnerSize, pos, lp)
}

// Earlier versions did not record a permanent original pointer. Find the earliest
// backup that predates this tool's custom theme, and let restoreBackup verify it.
func originalBackup(root string) (string, error) {
	entries, err := filepath.Glob(filepath.Join(root, "backup-*"))
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	for _, dir := range entries {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if !exists(dir+"/initrd.img") || !exists(dir+"/sha256") {
			continue
		}
		conf, err := os.ReadFile(dir + "/config")
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		// A saved custom theme is also evidence this is not the original state.
		if exists(dir+"/theme") || strings.Contains(string(conf), "opi-custom-logo") {
			continue
		}
		return dir, nil
	}
	return "", fmt.Errorf("no original pre-customization backup is available; no changes made")
}
