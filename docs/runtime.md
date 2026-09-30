# Runtime and recovery reference

- The normal UI runs as the desktop user. Apply/restore invoke the same executable through `pkexec` and pass the selected resource directory explicitly.
- CPU architecture and kernel version are detected rather than restricted to Orange Pi 5 Plus or kernel 5.10.
- Supported boot layouts: Debian/Ubuntu GRUB with initramfs-tools; Armbian U-Boot using `/boot/uInitrd`; Ubuntu Rockchip `stable-fallback`; Raspberry Pi firmware with a global `initramfs ... followkernel` directive or `auto_initramfs=1` and a recognized Raspberry Pi kernel suffix.
- Plymouth, `mkinitramfs`, `lsinitramfs`, `pkexec`, and a desktop authentication agent are required. U-Boot layouts also require `mkimage`. The running kernel's module directory and `splash` boot argument must exist. Conflicting Plymouth boot arguments are rejected.
- Editing remains available on unsupported hosts. Apply/restore check the layout before authentication and the privileged helper detects it again before changes. Unknown layouts, dracut/mkinitcpio/UKI-only setups, custom Raspberry Pi kernel paths, includes, and conditional initramfs directives are rejected with a reason.
- A separate initramfs is built and checked before replacing the resolved boot image. Armbian also receives a matching U-Boot ramdisk generated with its configured architecture. Existing boot symlinks are preserved. Kernel images, DTBs, bootloader settings, and trial slots are not changed.
- Rockchip trial boots and pending trials remain blocked. There is no fixed kernel restriction for new operations.
- Backups include the theme, configuration, hook, initramfs, and any U-Boot ramdisk under `/var/lib/opi-plymouth-logo/backup-*`. Boot images have checksums and backups record the kernel and resolved paths; restoring a backup for another kernel/layout is rejected. Original restore selects a compatible backup. Legacy backups are accepted only on the original 5.10.160-rockchip layout. Application failures roll back the saved state.
- Raspberry Pi and x86 support is tested with filesystem fixtures; booting those machines has not been verified on physical hardware.
- Restore validates the backup checksum. Original restore refuses if no pre-customization backup remains.
- Uninstall removes only the app-menu entry, legacy installed executable (if any), and registered icons. Editable resources, source files, applied boot theme, and recovery backups remain.
- The app window uses X11/XWayland with `WM_CLASS=plymouth-logo`. Desktop integration registers `StartupWMClass=plymouth-logo`; no desktop shortcut is created.
- `--no-browser` prints the local URL for manual access. `--port NUMBER` overrides automatic port selection. The default window size is 856×823 logical pixels.

## Theme editing

- The layer editor combines up to 8 static images and 8 animations in one list. Each layer has independent position and size; lower list entries appear in front regardless of type. Images can be added together, replaced without changing placement, reordered, or removed. Removing every layer leaves the solid background and Plymouth prompts.
- Image layers accept PNG/JPG (12 MiB and 20 megapixels per original; 32 MiB original file total). Image sizes range from 1–1920px along the longer dimension, preserving aspect ratio and PNG transparency. Animation sizes remain 32–320px, with independent FPS from 1–60. Enlarging small images can soften edges.
- Runtime decoded pixels are bounded separately: all animations together retain the previous 64 MiB limit; all static images together have a 16 MiB limit. This preserves existing configurations that used nearly 64 MiB for animations plus a logo. The UI reports each budget, and the server/helper enforce both before mutation.
- Existing logo-plus-animation configurations migrate into a first image layer followed by the animations, preserving dimensions, alignment, FPS, and order. Existing two-step default rings retain their center-based position. Legacy requests and old manifests remain supported.
- New configurations save their ordered layer metadata in `layers.json`, rendered PNG frames under `animations/`, and exact uploaded originals under `sources/`. Reopening the app loads those originals, so reducing a rendered size does not discard source pixels. The originals and metadata are included in backups. The initramfs hook excludes originals from the boot image; only the rendered files are needed at boot.
- Image inspection and the draft preview use the same Go decoder as installation. PNG previews use the first decoded frame, and JPEG metadata orientation is not applied. Browser EXIF/APNG behavior therefore cannot silently disagree with the rendered boot image.
- Do not change the template's `ModuleName=two-step` or remove the expected alignment/color fields unless modifying the application too.
- The current preview reflects the installed custom theme. The right-hand draft remains independent during edits.
- Custom animations are 160px source frames, adjustable to 32–320px. Enlarging them can soften edges.
- Custom animation timing is independent of the two-step plugin's 2-second cycle. The script refreshes at 60 Hz and advances frames at the requested FPS; actual cadence depends on boot rendering load. Existing items without `fps` default to 30 FPS. Karateka uses all 288 original frames at 60 ms each (17.28 seconds per cycle).
- Background removal was performed offline. Fine edge halos can remain. The app includes no background-removal model or Python dependency.
- Web previews approximate 1920×1080 placement; actual boot appearance requires a reboot test.
- Custom animation previews use the script's alignment within the remaining screen space, keeping edge-aligned sprites inside the screen. New draft and multi-layer installed previews use actual PNG frames as APNGs, including the default ring. Legacy installed two-step rings retain the CSS approximation. Migrating a legacy default ring preserves its center-based position.
- The left preview selector separates the currently selected theme from the system default reference. Selecting the reference does not change the draft, selected boot theme, or restore target. The system default is resolved from distribution defaults in `/run`, `/usr/share`, `/usr/lib`, or `/lib`, and then `default.plymouth` in the configured theme roots; it is not assumed to be BGRT or to match the original backup.
- Current theme selection respects `plymouth.splash=`, the daemon configuration (including `ThemeDir`), distribution defaults, and the default theme link. The custom preview is used only when this resolves to this app's installed theme.
- Installed BGRT/two-step normal-boot previews read their own `ImageDir`, colors, watermark, and throbber PNG frames. Frame sequences may start at 0000 (Armbian) or 0001; gaps are rejected. The throbber uses a two-second cycle and center-based positioning; the watermark aligns within the remaining screen space. Without firmware BGRT, the fallback logo is centered at 38.2% of screen height on black.
- Preview geometry and timing follow the [Plymouth 0.9.5 source shipped for this host](https://archive.ubuntu.com/ubuntu/pool/main/p/plymouth/plymouth_0.9.5+git20211018.orig.tar.xz), particularly `src/plugins/splash/two-step/plugin.c` and `src/libply-splash-graphics/ply-throbber.c`.
- These previews represent installed files at 1920×1080, not a capture of the initramfs or display. Uncompressed 24/32-bit firmware BGRT BMPs are decoded into PNG for the reference preview. Upright BGRT logos use the standard 38.2% vertical position, or 50% when firmware offsets exactly center the logo at the reference resolution. Rotated panels are rejected because panel metadata is unavailable.
- Script themes can show up to three referenced logo/watermark/splash PNGs as an explicitly labelled artwork reference. Scripts are never executed and their layout, timing, prompts, and progress are not simulated. `special://logo` is resolved from the installed native `script.so` plugin's compiled PNG path rather than guessing an OS logo. Themes without supported artwork, extra tiled/header/corner imagery, boot progress bars/titles, and invalid or incomplete assets show an unavailable reason instead of an invented rendering. Normal boot previews do not simulate prompts or end animations. The draft still starts with Tux when no supported custom logo is selected.

## Custom spinners

Create `data/assets/spinners/my-spinner/` with an `item.json` and consecutively numbered PNG files:

```json
{"id":"my-spinner","name":"My spinner","fps":24}
```

- Frames: `throbber-0001.png`, `throbber-0002.png`, and so on, without gaps. All frames must have identical dimensions; 160×160 transparent PNGs are a useful starting point.
- Length = frame count / FPS. For example, 120 frames at 24 FPS make a 5-second loop.
- `fps` accepts 1–60, including fractional values. Omit it to keep the previous 30 FPS behavior. The `frames` field is computed by the app; do not set it yourself.
- App limits: 1–600 frames, at most 1024×1024 per source frame, and a 64 MiB decoded image budget per source and across all animation layers after resizing. Static images have their separate 16 MiB budget. Duplicate layers count separately; rectangular images use their actual resized dimensions. These are memory limits for early boot, not a Plymouth 2-second restriction.
- `preview.png` is no longer required or read. Both previews are generated as APNGs from the actual frame files. The installed preview reads the installed theme; draft timing comes from the selected data folder.
- Restart the app after editing resources. Select the item and Apply to update the boot theme. Merely changing data does not alter the installed theme or initramfs.
- Every configured frame, the manifest, script, and required plugins are checked in the generated initramfs before replacing the boot image. Failed builds restore the complete previous theme, including all layer directories.
- The initramfs hook explicitly includes the selected splash module, all installed `label*.so` backends (including `label-pango.so`, `label-freetype.so`, and legacy `label.so`), and their library dependencies. Generated scripts handle password/question prompts, messages, and update progress. Backup and restore cover the script and its images together.
- The web UI, presets, and icon are embedded; `data/web/` from older versions is ignored. No resources under `data/assets/` are overwritten automatically on startup.

## Development checks

```bash
make check
make race
node tests/editor.cjs
```

Node is only required for the UI regression check. When Python 3 and the installed Plymouth script plugin are available, Go tests also execute the generated script in that plugin without starting a daemon or touching a display. Tests cover mixed-layer Plymouth execution, source preservation, authenticated image inspection, complete archive validation, successful staged application, failure rollback, and backup restoration. They use temporary resources and mocked privileged execution; they do not change live boot files.
