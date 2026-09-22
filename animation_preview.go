package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
)

func pngChunk(out *bytes.Buffer, kind string, data []byte) {
	binary.Write(out, binary.BigEndian, uint32(len(data)))
	out.WriteString(kind)
	out.Write(data)
	sum := crc32.NewIEEE()
	sum.Write([]byte(kind))
	sum.Write(data)
	binary.Write(out, binary.BigEndian, sum.Sum32())
}

// Build the browser preview from the same frames/timing used at boot.
// Re-encode RGBA canvases so indexed PNG palettes and transparency agree.
func animationPNG(entry spinnerEntry, read func(int) ([]byte, error)) ([]byte, error) {
	if entry.Frames < 1 || entry.Frames > 600 || entry.FPS < 1 || entry.FPS > 60 {
		return nil, fmt.Errorf("invalid animation timing")
	}
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	seq := uint32(0)
	var bounds image.Rectangle
	var pixels int64
	for i := 0; i < entry.Frames; i++ {
		raw, err := read(i + 1)
		if err != nil {
			return nil, err
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		pixels += int64(cfg.Width) * int64(cfg.Height)
		if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 1024 || cfg.Height > 1024 || pixels*4 > 64*1024*1024 {
			return nil, fmt.Errorf("animation preview exceeds image budget")
		}
		im, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if i == 0 {
			bounds = im.Bounds()
		}
		if im.Bounds() != bounds {
			return nil, fmt.Errorf("animation frame dimensions differ")
		}
		// Force RGBA so opaque and transparent frames share one color type.
		rgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(rgba, bounds, im, bounds.Min, draw.Src)
		var encoded bytes.Buffer
		if err = png.Encode(&encoded, alphaCanvas{rgba}); err != nil {
			return nil, err
		}
		// Every frame now has the same PNG color format.
		data := encoded.Bytes()
		var idat []byte
		for pos := 8; pos+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[pos : pos+4]))
			kind := string(data[pos+4 : pos+8])
			chunk := data[pos+8 : pos+8+n]
			if i == 0 && kind == "IHDR" {
				pngChunk(&out, "IHDR", chunk)
				var control bytes.Buffer
				binary.Write(&control, binary.BigEndian, uint32(entry.Frames))
				binary.Write(&control, binary.BigEndian, uint32(0))
				pngChunk(&out, "acTL", control.Bytes())
			}
			if kind == "IDAT" {
				idat = append(idat, chunk...)
			}
			pos += n + 12
		}
		var control bytes.Buffer
		for _, v := range []uint32{seq, uint32(bounds.Dx()), uint32(bounds.Dy()), 0, 0} {
			binary.Write(&control, binary.BigEndian, v)
		}
		seq++
		binary.Write(&control, binary.BigEndian, uint16(math.Round(1000/entry.FPS)))
		binary.Write(&control, binary.BigEndian, uint16(1000))
		control.Write([]byte{0, 0}) // Full canvas replacement, no blending with the last frame.
		pngChunk(&out, "fcTL", control.Bytes())
		if i == 0 {
			pngChunk(&out, "IDAT", idat)
		} else {
			var frame bytes.Buffer
			binary.Write(&frame, binary.BigEndian, seq)
			seq++
			frame.Write(idat)
			pngChunk(&out, "fdAT", frame.Bytes())
		}
	}
	pngChunk(&out, "IEND", nil)
	return out.Bytes(), nil
}

func customSpinnerPreview(id string) ([]byte, error) {
	entry, err := spinnerMetadata(id)
	if err != nil {
		return nil, err
	}
	return animationPNG(entry, func(i int) ([]byte, error) {
		return assets.ReadFile(fmt.Sprintf("assets/spinners/%s/throbber-%04d.png", spinnerDirectories[entry.ID], i))
	})
}

func installedSpinnerPreview() ([]byte, error) {
	conf, err := os.ReadFile(themeDir + "/opi-custom-logo.plymouth")
	if err != nil {
		return nil, err
	}
	entry := spinnerEntry{FPS: 30, Frames: 60}
	// Previously applied two-step animations retain their old timing until re-applied.
	for _, line := range strings.Split(string(conf), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "AnimationFPS":
			entry.FPS, err = strconv.ParseFloat(value, 64)
		case "AnimationFrames":
			entry.Frames, err = strconv.Atoi(value)
		}
		if err != nil {
			return nil, err
		}
	}
	return animationPNG(entry, func(i int) ([]byte, error) {
		return readLimited(fmt.Sprintf("%s/throbber-%04d.png", themeDir, i), 32*1024*1024)
	})
}

type alphaCanvas struct{ *image.NRGBA }

func (alphaCanvas) Opaque() bool { return false }
