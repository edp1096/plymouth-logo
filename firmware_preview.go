package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BGRT uses BMP, not PNG. Decode bounded, uncompressed 24/32-bit DIBs without
// architecture-specific code or invoking a privileged graphics daemon.
func decodeFirmwareBMP(data []byte) (*image.NRGBA, error) {
	bad := func() (*image.NRGBA, error) { return nil, fmt.Errorf("unsupported or invalid BGRT bitmap") }
	if len(data) < 54 || string(data[:2]) != "BM" {
		return bad()
	}
	u32 := func(pos int) uint32 { return binary.LittleEndian.Uint32(data[pos : pos+4]) }
	header := int64(u32(14))
	offset := int64(u32(10))
	w, h := int64(int32(u32(18))), int64(int32(u32(22)))
	bits := binary.LittleEndian.Uint16(data[28:30])
	topDown := h < 0
	if topDown {
		h = -h
	}
	if header < 40 || offset < 14+header || w < 1 || h < 1 || w*h > 20_000_000 || (bits != 24 && bits != 32) || u32(30) != 0 || binary.LittleEndian.Uint16(data[26:28]) != 1 {
		return bad()
	}
	stride := ((w*int64(bits) + 31) / 32) * 4
	if offset+stride*h > int64(len(data)) {
		return bad()
	}
	out := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	for y := int64(0); y < h; y++ {
		row := h - 1 - y
		if topDown {
			row = y
		}
		for x := int64(0); x < w; x++ {
			p := offset + row*stride + x*int64(bits/8)
			out.SetNRGBA(int(x), int(y), color.NRGBA{R: data[p+2], G: data[p+1], B: data[p], A: 255})
		}
	}
	return out, nil
}
func firmwarePreview(path string) (themePreviewLayer, error) {
	data, err := readLimited(path, 32*1024*1024)
	if err != nil {
		return themePreviewLayer{}, err
	}
	bitmap, err := decodeFirmwareBMP(data)
	if err != nil {
		return themePreviewLayer{}, err
	}
	dir := filepath.Dir(path)
	// Rotated panels need renderer/panel metadata; do not guess their geometry.
	if raw, e := os.ReadFile(filepath.Join(dir, "status")); e == nil {
		status, e := strconv.ParseUint(strings.TrimSpace(string(raw)), 0, 8)
		if e != nil || status&6 != 0 {
			return themePreviewLayer{}, fmt.Errorf("rotated BGRT panels are not supported by this preview")
		}
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, bitmap); err != nil {
		return themePreviewLayer{}, err
	}
	w, h := bitmap.Bounds().Dx(), bitmap.Bounds().Dy()
	y := 38.2
	readOffset := func(name string) int {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			return -1
		}
		n, e := strconv.Atoi(strings.TrimSpace(string(b)))
		if e != nil {
			return -1
		}
		return n
	}
	if readOffset("xoffset") == (1920-w)/2 && readOffset("yoffset") == (1080-h)/2 {
		y = 50
	}
	return themePreviewLayer{Image: pngDataURL(encoded.Bytes()), Width: w, Height: h, X: 50, Y: y, Center: true}, nil
}
