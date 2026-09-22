# Runtime and recovery reference

- The normal UI runs as the desktop user. Apply/restore invoke the same executable through `pkexec` and pass the selected resource directory explicitly.
- For this OPi5 Plus host's Ubuntu Rockchip 22.04 and original `5.10.160-rockchip` normal `stable-fallback` boot. Pending kernel trials must be cancelled first.
- The host must provide Plymouth, `mkinitramfs`, `lsinitramfs`, `pkexec`, and a desktop authentication agent. Base theme assets are exported from bundled defaults.
- A separate initramfs is built and checked, then replaces `/boot/firmware/initrd.img`. The distribution's kernel/DTB copying post-update hook is avoided.
- Kernel Images, DTBs, SPI, selector hashes, and kernel trial slots are preserved. Trial boots keep their previous theme.
- The custom theme, configuration, inclusion hook, and normal boot initramfs are backed up under `/var/lib/opi-plymouth-logo/backup-*`. Application failure restores the previous state.
- Restore validates the backup checksum. Original restore refuses if no pre-customization backup remains.
- Uninstall removes only the app-menu entry, legacy installed executable (if any), and registered icons. Editable resources, source files, applied boot theme, and recovery backups remain.
- The app window uses X11/XWayland with `WM_CLASS=plymouth-logo`. Desktop integration registers `StartupWMClass=plymouth-logo`; no desktop shortcut is created.
- `--no-browser` prints the local URL for manual access. `--port NUMBER` overrides automatic port selection. The default window size is 856×803 logical pixels.

## Theme editing

- The app controls logo size/alignment, spinner item/size/alignment, and solid background color when applying. These UI settings override the corresponding base-template fields.
- The default ring still uses the exported `assets/theme/` two-step template. Custom animations use a generated script theme; the exported template does not control their timing.
- Do not change the template's `ModuleName=two-step` or remove the expected alignment/color fields unless modifying the application too.
- The current preview reflects the installed custom theme. The right-hand draft remains independent during edits.
- Custom animations are 160px source frames, adjustable to 32–320px. Enlarging them can soften edges.
- Custom animation timing is independent of the two-step plugin's 2-second cycle. The script refreshes at 60 Hz and advances frames at the requested FPS; actual cadence depends on boot rendering load. Existing items without `fps` default to 30 FPS. Karateka uses all 288 original frames at 60 ms each (17.28 seconds per cycle).
- Background removal was performed offline. Fine edge halos can remain. The app includes no background-removal model or Python dependency.
- Web previews approximate 1920×1080 placement; actual boot appearance requires a reboot test.

## Custom spinners

Create `data/assets/spinners/my-spinner/` with an `item.json` and consecutively numbered PNG files:

```json
{"id":"my-spinner","name":"My spinner","fps":24}
```

- Frames: `throbber-0001.png`, `throbber-0002.png`, and so on, without gaps. All frames must have identical dimensions; 160×160 transparent PNGs are a useful starting point.
- Length = frame count / FPS. For example, 120 frames at 24 FPS make a 5-second loop.
- `fps` accepts 1–60, including fractional values. Omit it to keep the previous 30 FPS behavior. The `frames` field is computed by the app; do not set it yourself.
- App limits: 1–600 frames, at most 1024×1024 per source frame, and 64 MiB decoded image budget both before and after resizing. These are memory limits for early boot, not a Plymouth 2-second restriction.
- `preview.png` is no longer required or read. Both previews are generated as APNGs from the actual frame files. The installed preview reads the installed theme; draft timing comes from the selected data folder.
- Restart the app after editing resources. Select the item and Apply to update the boot theme. Merely changing data does not alter the installed theme or initramfs.
- The initramfs hook explicitly includes `script.so`, `label.so`, and their library dependencies. Generated scripts handle password/question prompts, messages, and update progress. Backup and restore cover the script and its images together.
- The web UI, presets, and icon are embedded; `data/web/` from older versions is ignored. No resources under `data/assets/` are overwritten automatically on startup.

## Development checks

```bash
make check
make race
node tests/editor.cjs
```

Node is only required for the UI regression check. When Python 3 and the installed Plymouth script plugin are available, Go tests also execute the generated script in that plugin without starting a daemon or touching a display. Tests use temporary resources and mocked privileged execution; they do not change live boot files.
