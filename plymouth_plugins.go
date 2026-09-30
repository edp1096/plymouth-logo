package main

import (
	"fmt"
	"strings"
)

func verifyPlymouthPlugins(listing, module string) error {
	plugins := map[string]bool{}
	for _, line := range strings.Split(listing, "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(line), "./")
		for _, plugin := range []string{module + ".so", "label-pango.so", "label-freetype.so", "label.so"} {
			if strings.HasSuffix(name, "/plymouth/"+plugin) {
				plugins[plugin] = true
			}
		}
	}
	if !plugins[module+".so"] {
		return fmt.Errorf("generated initramfs is missing %s.so", module)
	}
	if !plugins["label-pango.so"] && !plugins["label-freetype.so"] && !plugins["label.so"] {
		return fmt.Errorf("generated initramfs is missing a Plymouth label backend (label-pango.so, label-freetype.so or label.so)")
	}
	return nil
}
