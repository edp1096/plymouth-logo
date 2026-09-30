package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlymouthLabelBackends(t *testing.T) {
	for _, module := range []string{"script", "two-step"} {
		for _, backend := range []string{"label.so", "label-pango.so", "label-freetype.so"} {
			listing := "usr/lib/x86_64-linux-gnu/plymouth/" + module + ".so\n./usr/lib/aarch64-linux-gnu/plymouth/" + backend + "\n"
			if err := verifyPlymouthPlugins(listing, module); err != nil {
				t.Fatal(err)
			}
		}
		if err := verifyPlymouthPlugins("usr/lib/plymouth/"+module+".so\nusr/lib/plymouth/label.so.backup", module); err == nil {
			t.Fatal("missing label backend accepted")
		}
	}
}

func TestThemeHookLabelBackends(t *testing.T) {
	for _, backends := range [][]string{{"label.so"}, {"label-pango.so"}, {"label-freetype.so"}, {"label.so", "label-pango.so", "label-freetype.so"}, {}} {
		t.Run(strings.Join(backends, "+"), func(t *testing.T) {
			root := t.TempDir()
			plugins := root + "/plugins"
			theme := root + "/theme"
			bin := root + "/bin"
			for _, dir := range []string{plugins, theme, bin, root + "/dest"} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, data string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(data), 0755); err != nil {
					t.Fatal(err)
				}
			}
			write(theme+"/opi-custom-logo.plymouth", "ModuleName=script\n")
			write(plugins+"/script.so", "module")
			for _, name := range backends {
				write(plugins+"/"+name, "backend")
			}
			write(bin+"/plymouth", "#!/bin/sh\nprintf '%s\\n' '"+plugins+"'\n")
			write(root+"/hook-functions", `copy_exec() { test -f "$1" || return 1; printf '%s\n' "$1" >> "$COPY_LOG"; }
`)
			hook := strings.ReplaceAll(customThemeHook, "/usr/share/plymouth/themes/opi-custom-logo", theme)
			hook = strings.ReplaceAll(hook, ". /usr/share/initramfs-tools/hook-functions", ". "+root+"/hook-functions")
			write(root+"/hook", hook)
			cmd := exec.Command("sh", root+"/hook")
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "DESTDIR="+root+"/dest", "COPY_LOG="+root+"/copied")
			output, err := cmd.CombinedOutput()
			if len(backends) == 0 {
				if err == nil || !strings.Contains(string(output), "No Plymouth label backend") {
					t.Fatal(err, string(output))
				}
				return
			}
			if err != nil {
				t.Fatal(err, string(output))
			}
			copied, _ := os.ReadFile(root + "/copied")
			for _, name := range append([]string{"script.so"}, backends...) {
				if !strings.Contains(string(copied), filepath.Join(plugins, name)+"\n") {
					t.Fatal("plugin not included", name, string(copied))
				}
			}
		})
	}
}

func TestHostHookSharedDependencies(t *testing.T) {
	if os.Getenv("PLYMOUTH_TEST_HOST") != "1" {
		t.Skip("opt-in installed hook dependency check")
	}
	root := t.TempDir()
	theme := root + "/theme"
	dest := root + "/dest"
	os.MkdirAll(theme, 0755)
	os.MkdirAll(dest, 0755)
	if err := os.WriteFile(theme+"/opi-custom-logo.plymouth", []byte("ModuleName=script\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hook := strings.ReplaceAll(customThemeHook, "/usr/share/plymouth/themes/opi-custom-logo", theme)
	script := root + "/hook"
	os.WriteFile(script, []byte(hook), 0755)
	cmd := exec.Command("sh", script)
	cmd.Env = append(os.Environ(), "DESTDIR="+dest, "verbose=n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	path, err := exec.Command("plymouth", "--get-splash-plugin-path").Output()
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := filepath.Glob(filepath.Join(strings.TrimSpace(string(path)), "label*.so"))
	if err != nil || len(plugins) == 0 {
		t.Fatal("no installed label backend", err)
	}
	names := map[string]bool{}
	filepath.WalkDir(dest, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
		return nil
	})
	dependencies := 0
	for _, plugin := range plugins {
		if !names[filepath.Base(plugin)] {
			t.Fatal("backend omitted", plugin)
		}
		out, err := exec.Command("ldd", plugin).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		if strings.Contains(string(out), "not found") {
			t.Fatal("unresolved native dependency", string(out))
		}
		for _, field := range strings.Fields(string(out)) {
			if strings.HasPrefix(field, "/") {
				if !names[filepath.Base(field)] {
					t.Fatal("dependency omitted", field)
				}
				dependencies++
			}
		}
	}
	t.Logf("Actual custom hook copied %d installed label backend(s) and all %d ldd dependencies into a temporary directory", len(plugins), dependencies)
}
