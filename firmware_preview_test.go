package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func testBMP(bits uint16, topDown bool) []byte {
	stride := ((2*int(bits) + 31) / 32) * 4
	data := make([]byte, 54+stride*2)
	copy(data, "BM")
	put := func(p int, v uint32) { binary.LittleEndian.PutUint32(data[p:p+4], v) }
	put(2, uint32(len(data)))
	put(10, 54)
	put(14, 40)
	put(18, 2)
	put(22, 2)
	if topDown {
		put(22, 0xfffffffe)
	}
	binary.LittleEndian.PutUint16(data[26:28], 1)
	binary.LittleEndian.PutUint16(data[28:30], bits)
	data[54], data[55], data[56] = 1, 2, 3
	data[54+stride], data[55+stride], data[56+stride] = 4, 5, 6
	return data
}
func TestFirmwareBMP(t *testing.T) {
	for _, bits := range []uint16{24, 32} {
		for _, top := range []bool{false, true} {
			bitmap, err := decodeFirmwareBMP(testBMP(bits, top))
			if err != nil {
				t.Fatal(err)
			}
			expected := uint8(6)
			if top {
				expected = 3
			}
			if bitmap.NRGBAAt(0, 0).R != expected || bitmap.NRGBAAt(0, 0).A != 255 {
				t.Fatal("orientation or color mismatch")
			}
		}
	}
	for _, data := range [][]byte{nil, []byte("BM"), testBMP(16, false), testBMP(24, false)[:54]} {
		if _, err := decodeFirmwareBMP(data); err == nil {
			t.Fatal("invalid BMP accepted")
		}
	}
	data := testBMP(24, false)
	binary.LittleEndian.PutUint32(data[18:22], 0x7fffffff)
	if _, err := decodeFirmwareBMP(data); err == nil {
		t.Fatal("oversized BMP accepted")
	}
}
func TestFirmwareThemePreview(t *testing.T) {
	p, _ := previewFixture(t)
	dir := filepath.Dir(p.Firmware)
	os.WriteFile(p.Firmware, testBMP(24, false), 0644)
	got := p.preview("default")
	if !got.Available || len(got.Layers) != 3 || got.Layers[0].Width != 2 || got.Layers[0].Y != 38.2 {
		t.Fatal(got)
	}
	os.WriteFile(dir+"/xoffset", []byte("959"), 0644)
	os.WriteFile(dir+"/yoffset", []byte("539"), 0644)
	got = p.preview("default")
	if !got.Available || got.Layers[0].Y != 50 {
		t.Fatal(got)
	}
	os.WriteFile(dir+"/status", []byte("3"), 0644)
	if got = p.preview("default"); got.Available || got.Reason == "" {
		t.Fatal("rotated panel silently simulated")
	}
}
