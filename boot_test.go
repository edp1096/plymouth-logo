package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootDetection(t *testing.T) {
	for _, kind := range []string{"grub", "armbian", "raspberry", "rockchip"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			kernel := "6.18.44-current-rockchip64"
			if kind == "raspberry" {
				kernel = "6.12.1+rpt-rpi-2712"
			}
			write := func(name, data string) {
				t.Helper()
				p := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(data), 0755); err != nil {
					t.Fatal(err)
				}
			}
			os.MkdirAll(filepath.Join(root, "lib/modules", kernel), 0755)
			for _, tool := range []string{"mkinitramfs", "lsinitramfs", "plymouth", "mkimage"} {
				write("bin/"+tool, "#!/bin/sh\nexit 0\n")
			}
			t.Setenv("PATH", filepath.Join(root, "bin"))
			write("proc/cmdline", "quiet splash")
			expected := "boot/initrd.img-" + kernel
			switch kind {
			case "grub":
				write("boot/grub/grub.cfg", "initrd /initrd.img-"+kernel)
			case "armbian":
				write("etc/armbian-release", "INITRD_ARCH=arm64\n")
				write("boot/boot.cmd", "load ${devtype} ${devnum}:${distro_bootpart} ${ramdisk_addr_r} ${prefix}uInitrd\n")
				write("boot/uInitrd-"+kernel, "wrapped")
				os.Symlink("uInitrd-"+kernel, filepath.Join(root, "boot/uInitrd"))
			case "raspberry":
				write("proc/device-tree/model", "Raspberry Pi 5 Model B")
				write("boot/firmware/config.txt", "auto_initramfs=1\n[pi5]\ndtoverlay=vc4-kms-v3d\n")
				expected = "boot/firmware/initramfs_2712"
			case "rockchip":
				write("proc/cmdline", "splash rknpu_boot.path=stable-fallback")
				for _, flag := range []string{"rnold.flg", "rnnew.flg"} {
					write("boot/firmware/"+flag, "0")
				}
				expected = "boot/firmware/initrd.img"
			}
			write(expected, "original")
			got, err := detectBootKernel(root, kernel)
			if err != nil || got.Image != filepath.Join(root, expected) {
				t.Fatal(got, err)
			}
			if kind == "armbian" {
				if !strings.HasSuffix(got.Ramdisk, "uInitrd-"+kernel) {
					t.Fatal(got)
				}
				os.Remove(filepath.Join(root, "boot/uInitrd"))
				write("boot/uInitrd", "wrong kernel")
				if _, err = detectBootKernel(root, kernel); err == nil {
					t.Fatal("unmatched ramdisk accepted")
				}
			}
			write("proc/cmdline", "quiet")
			if _, err = detectBootKernel(root, kernel); err == nil {
				t.Fatal("missing splash accepted")
			}
		})
	}
}

func TestRaspberryConfiguration(t *testing.T) {
	for _, config := range []string{"", "[pi5]\nauto_initramfs=1", "include other.txt", "auto_initramfs=1\nkernel=custom.img", "initramfs ../image followkernel"} {
		if _, err := raspberryImage(config, "6.12-rpi-2712"); err == nil {
			t.Fatal("ambiguous config accepted", config)
		}
	}
	got, err := raspberryImage("initramfs initrd.img followkernel", "custom")
	if err != nil || got != "initrd.img" {
		t.Fatal(got, err)
	}
}

func TestRamdiskBackupAndKernelMismatch(t *testing.T) {
	oldState, oldImage, oldRamdisk, oldKernel := stateDir, bootImage, bootRamdisk, kernelVersion
	oldTheme, oldConfig, oldHook := themeDir, configPath, hookPath
	defer func() {
		stateDir, bootImage, bootRamdisk, kernelVersion = oldState, oldImage, oldRamdisk, oldKernel
		themeDir, configPath, hookPath = oldTheme, oldConfig, oldHook
	}()
	root := t.TempDir()
	stateDir = root
	bootImage = root + "/initrd"
	bootRamdisk = root + "/uInitrd"
	kernelVersion = "test-kernel"
	themeDir = root + "/theme"
	configPath = root + "/config"
	hookPath = root + "/hook"
	os.WriteFile(bootImage, []byte("original"), 0644)
	os.WriteFile(bootRamdisk, []byte("wrapped"), 0644)
	saved, err := backupCurrent()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(bootRamdisk, []byte("new"), 0644)
	if err = restoreBackup(saved); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(bootRamdisk)
	if string(raw) != "wrapped" {
		t.Fatal("ramdisk not restored")
	}
	kernelVersion = "another-kernel"
	if err = restoreBackup(saved); err == nil {
		t.Fatal("different kernel backup accepted")
	}
	kernelVersion = "test-kernel"
	os.WriteFile(saved+"/ramdisk", []byte("tampered"), 0644)
	os.WriteFile(bootImage, []byte("untouched"), 0644)
	if err = restoreBackup(saved); err == nil {
		t.Fatal("corrupt ramdisk accepted")
	}
	raw, _ = os.ReadFile(bootImage)
	if string(raw) != "untouched" {
		t.Fatal("image changed before validation")
	}
}

func TestCurrentHostDetection(t *testing.T) {
	if os.Getenv("PLYMOUTH_TEST_HOST") != "1" {
		t.Skip("opt-in read-only host check")
	}
	target, err := detectBoot("")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Detected: %+v", target)
}

func TestUbootApplyAndRollback(t *testing.T) {
	oldState, oldTheme, oldConfig, oldHook, oldImage := stateDir, themeDir, configPath, hookPath, bootImage
	oldRamdisk, oldArch, oldCommand := bootRamdisk, bootArchitecture, command
	defer func() {
		stateDir, themeDir, configPath, hookPath, bootImage = oldState, oldTheme, oldConfig, oldHook, oldImage
		bootRamdisk, bootArchitecture, command = oldRamdisk, oldArch, oldCommand
	}()
	root := t.TempDir()
	stateDir = root + "/state"
	themeDir = root + "/theme"
	configPath = root + "/config"
	hookPath = root + "/hook"
	bootImage = root + "/initrd"
	bootRamdisk = root + "/uInitrd-version"
	bootArchitecture = "arm64"
	os.MkdirAll(stateDir, 0700)
	os.WriteFile(bootImage, []byte("original"), 0644)
	os.WriteFile(bootRamdisk, []byte("wrapped-original"), 0644)
	os.Symlink("uInitrd-version", root+"/uInitrd")
	bin := root + "/bin"
	os.MkdirAll(bin, 0755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	listing := "usr/lib/plymouth/two-step.so\nusr/lib/plymouth/label-pango.so\nusr/share/plymouth/themes/opi-custom-logo/watermark.png\nlib/modules/" + kernelVersion + "\n"
	os.WriteFile(bin+"/lsinitramfs", []byte("#!/bin/sh\ncat <<'FILES'\n"+listing+"FILES\n"), 0755)
	fail := false
	converted := false
	command = func(name string, args ...string) error {
		switch name {
		case "mkinitramfs":
			if args[2] != kernelVersion {
				t.Fatalf("wrong kernel: %v", args)
			}
			return os.WriteFile(args[1], []byte("new-initrd"), 0644)
		case "mkimage":
			converted = true
			if len(args) != 13 || args[1] != "arm64" {
				t.Fatalf("bad U-Boot conversion: %v", args)
			}
			if fail {
				return fmt.Errorf("conversion failed")
			}
			return os.WriteFile(args[len(args)-1], []byte("new-wrapped"), 0644)
		}
		return oldCommand(name, args...)
	}
	if err := applyLogo(sample()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(bootRamdisk)
	if !converted || string(raw) != "new-wrapped" {
		t.Fatal("ramdisk not updated")
	}
	link, err := os.Readlink(root + "/uInitrd")
	if err != nil || link != "uInitrd-version" {
		t.Fatal("boot link changed")
	}
	fail = true
	if err = applyLogo(sample()); err == nil {
		t.Fatal("conversion failure ignored")
	}
	raw, _ = os.ReadFile(bootImage)
	if string(raw) != "new-initrd" {
		t.Fatal("initrd rollback failed")
	}
	raw, _ = os.ReadFile(bootRamdisk)
	if string(raw) != "new-wrapped" {
		t.Fatal("ramdisk rollback failed")
	}
}
