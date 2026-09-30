package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type bootTarget struct{ Kernel, Image, Ramdisk, Architecture string }

func runningKernel() string {
	b, _ := exec.Command("uname", "-r").Output()
	return strings.TrimSpace(string(b))
}

// root allows detection against a fixture without touching the host boot files.
func detectBoot(root string) (bootTarget, error) {
	return detectBootKernel(root, runningKernel())
}

func detectBootKernel(root, kernel string) (bootTarget, error) {
	t := bootTarget{Kernel: kernel}
	path := func(p string) string { return filepath.Join(root, p) }
	read := func(p string) string { b, _ := os.ReadFile(path(p)); return string(b) }
	if t.Kernel == "" || !exists(path("/lib/modules/"+t.Kernel)) {
		return t, fmt.Errorf("running kernel modules are unavailable")
	}
	for _, tool := range []string{"mkinitramfs", "lsinitramfs", "plymouth"} {
		if _, err := exec.LookPath(tool); err != nil {
			return t, fmt.Errorf("required tool is unavailable: %s (initramfs-tools is required)", tool)
		}
	}
	cmdline := read("/proc/cmdline")
	for _, arg := range strings.Fields(cmdline) {
		if arg == "plymouth.enable=0" || arg == "rd.plymouth=0" || (strings.HasPrefix(arg, "plymouth.theme=") && arg != "plymouth.theme=opi-custom-logo") {
			return t, fmt.Errorf("boot arguments disable or override the custom Plymouth theme")
		}
	}
	resolve := func(p string) (string, error) {
		resolved, err := filepath.EvalSymlinks(path(p))
		if err != nil {
			return "", err
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("invalid boot image: %s", p)
		}
		if root == "" && !strings.HasPrefix(resolved, "/boot/") {
			return "", fmt.Errorf("boot image is outside /boot")
		}
		return resolved, nil
	}
	var image string
	switch {
	case strings.Contains(cmdline, "rknpu_boot.path="):
		if !strings.Contains(cmdline, "rknpu_boot.path=stable-fallback") {
			return t, fmt.Errorf("use the normal original-kernel boot path first")
		}
		for _, flag := range []string{"rnold.flg", "rnnew.flg"} {
			if strings.TrimSpace(read("/boot/firmware/"+flag)) != "0" {
				return t, fmt.Errorf("cancel pending kernel trials first")
			}
		}
		image = "/boot/firmware/initrd.img"
	case exists(path("/etc/armbian-release")) && exists(path("/boot/boot.cmd")):
		if !regexp.MustCompile(`(?m)^load .*\$\{prefix\}uInitrd\s*$`).MatchString(read("/boot/boot.cmd")) {
			return t, fmt.Errorf("unrecognized Armbian boot script")
		}
		release := read("/etc/armbian-release")
		re := regexp.MustCompile(`(?m)^INITRD_ARCH=['"]?(arm64|arm|x86_64|x86)['"]?\s*$`)
		m := re.FindStringSubmatch(release)
		if m == nil {
			return t, fmt.Errorf("U-Boot initrd architecture is unavailable")
		}
		t.Architecture = m[1]
		if _, err := exec.LookPath("mkimage"); err != nil {
			return t, fmt.Errorf("required tool is unavailable: mkimage")
		}
		var err error
		t.Ramdisk, err = resolve("/boot/uInitrd")
		if err != nil {
			return t, err
		}
		if filepath.Base(t.Ramdisk) != "uInitrd-"+t.Kernel {
			return t, fmt.Errorf("U-Boot ramdisk does not match the running kernel")
		}
		image = "/boot/initrd.img-" + t.Kernel
	case exists(path("/boot/grub/grub.cfg")):
		image = "/boot/initrd.img-" + t.Kernel
		if !strings.Contains(read("/boot/grub/grub.cfg"), filepath.Base(image)) {
			return t, fmt.Errorf("GRUB does not reference the running kernel initramfs")
		}
	case strings.Contains(read("/proc/device-tree/model"), "Raspberry Pi"):
		base := "/boot/firmware"
		if !exists(path(base + "/config.txt")) {
			base = "/boot"
		}
		name, err := raspberryImage(read(base+"/config.txt"), t.Kernel)
		if err != nil {
			return t, err
		}
		image = base + "/" + name
	default:
		return t, fmt.Errorf("boot configuration could not be determined automatically (supported: GRUB with initramfs-tools, Armbian U-Boot, Raspberry Pi firmware, Rockchip stable-fallback)")
	}
	var err error
	t.Image, err = resolve(image)
	if err != nil {
		return t, err
	}
	if !strings.Contains(" "+cmdline+" ", " splash ") {
		return t, fmt.Errorf("enable splash in the boot configuration first")
	}
	return t, nil
}

func raspberryImage(config, kernel string) (string, error) {
	section, automatic, explicit := "all", "", ""
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		fields := strings.Fields(line)
		if fields[0] == "include" {
			return "", fmt.Errorf("Raspberry Pi config includes require manual boot configuration support")
		}
		relevant := strings.HasPrefix(line, "auto_initramfs=") || fields[0] == "initramfs" || strings.HasPrefix(line, "kernel=") || strings.HasPrefix(line, "os_prefix=")
		if !relevant {
			continue
		}
		if section != "all" {
			return "", fmt.Errorf("conditional Raspberry Pi initramfs configuration cannot be determined automatically")
		}
		if strings.HasPrefix(line, "kernel=") || strings.HasPrefix(line, "os_prefix=") {
			return "", fmt.Errorf("custom Raspberry Pi kernel paths require manual boot configuration support")
		}
		if strings.HasPrefix(line, "auto_initramfs=") {
			automatic = strings.TrimPrefix(line, "auto_initramfs=")
		}
		if fields[0] == "initramfs" {
			if len(fields) != 3 || fields[2] != "followkernel" || filepath.Base(fields[1]) != fields[1] {
				return "", fmt.Errorf("unsupported Raspberry Pi initramfs directive")
			}
			explicit = fields[1]
		}
	}
	if explicit != "" {
		return explicit, nil
	}
	if automatic != "1" {
		return "", fmt.Errorf("enable auto_initramfs=1 in Raspberry Pi config.txt first")
	}
	for suffix, name := range map[string]string{"-rpi-2712": "initramfs_2712", "-rpi-v8": "initramfs8", "-rpi-v7l": "initramfs7l", "-rpi-v7": "initramfs7", "-rpi-v6": "initramfs", "-v8+": "initramfs8", "-v7l+": "initramfs7l", "-v7+": "initramfs7"} {
		if strings.HasSuffix(kernel, suffix) {
			return name, nil
		}
	}
	return "", fmt.Errorf("Raspberry Pi kernel initramfs name could not be determined")
}
