![screenshot](./docs/screenshot.png)

App for customizing Plymouth boot screen. Arrange up to 8 images and 8 animations in one layer list, with independent position, size, and animation speed. PNG transparency and image proportions are preserved.

Use **Add image** for PNG/JPG files and **Add animation** for existing animations. Select a layer to edit it, replace an image, move it forward/backward, or remove it. Applied image originals are saved for later editing and restoration.

## Run

* Run `plymouth-logo`

## Build

```bash
make           # Build dist/plymouth-logo
make clean     # Remove the build output
```

## Append spinner

* Create `data/assets/spinners/my-spinner/` beside the executable.
* Add transparent PNG frames named `throbber-0001.png`, `throbber-0002.png`, etc. Use the same dimensions and consecutive numbers.
* Add `item.json` with a unique ID, display name, and playback FPS:

```json
{"id":"my-spinner","name":"My spinner","fps":24}
```
