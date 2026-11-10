package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clipImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 5, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 40), G: uint8(y * 70), B: 9, A: 255})
		}
	}
	return img
}

func TestPastedPicture(t *testing.T) {
	a := NewApp()
	if err := a.setClip(clipImage()); err != nil {
		t.Fatal(err)
	}
	info := a.Image()
	if info.Name != "[clipboard image]" || info.Path != "" || info.URL == "" || info.Dir != picturesDir() || info.Count != 0 {
		t.Errorf("Image() = %+v, want the clipboard name, no path, the Pictures folder, no count", info)
	}
	// the web view gets it as a PNG
	rec := get(a.imageHandler(), info.URL)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("GET %s = %d %q", info.URL, rec.Code, rec.Header().Get("Content-Type"))
	}
	if img, err := png.Decode(rec.Body); err != nil || img.Bounds().Dx() != 5 || img.Bounds().Dy() != 3 {
		t.Errorf("the pasted picture is served as %v, %v", img, err)
	}
	// the pixel readout and the copy work on it, without a file
	if src, err := a.PreparePixels(); src != "go" || err != nil {
		t.Errorf("PreparePixels = %q, %v", src, err)
	}
	if p := a.PixelAt(3, 2, 5, 3); !p.Ready || p.R != 120 || p.G != 140 || p.B != 9 {
		t.Errorf("PixelAt = %+v, want 120 140 9", p)
	}
	if b64, err := a.SelectionPNG(1, 1, 2, 2, 5, 3); err != nil || decodePNG(t, b64).Bounds().Dx() != 2 {
		t.Errorf("SelectionPNG: %v", err)
	}
	// it is in no folder: stepping does nothing and says nothing
	if note, err := a.Step(1); note != "" || err != nil || a.current().URL == "" {
		t.Errorf("Step on a pasted picture = %q, %v; want nothing done", note, err)
	}
	// a file opened in the window replaces it, and the old URL is gone
	pic := filepath.Join(t.TempDir(), "p.png")
	var b bytes.Buffer
	png.Encode(&b, clipImage())
	os.WriteFile(pic, b.Bytes(), 0o644)
	a.setImage(pic)
	if get(a.imageHandler(), info.URL).Code != http.StatusNotFound || a.Image().Name != "p.png" || a.clip != nil {
		t.Error("a file must take the place of the pasted picture")
	}
	if a.setClip(image.NewNRGBA(image.Rect(0, 0, 0, 0))) == nil {
		t.Error("an empty picture must be refused")
	}
}

// A window that holds a pasted picture does not lose it to a file the system hands over: that file gets a window of its own.
func TestFileOpenWhilePasted(t *testing.T) {
	pic := filepath.Join(t.TempDir(), "p.png")
	var b bytes.Buffer
	png.Encode(&b, clipImage())
	os.WriteFile(pic, b.Bytes(), 0o644)
	a := NewApp()
	var opened []string
	a.openWindow = func(p string) error { opened = append(opened, p); return nil }
	a.setClip(clipImage())
	a.onFileOpen(pic)
	if len(opened) != 1 || a.clip == nil {
		t.Errorf("windows %v, pasted picture kept: %v; want one new window and the paste kept", opened, a.clip != nil)
	}
}

func TestNewWindow(t *testing.T) {
	a := NewApp()
	var opened []string
	a.openWindow = func(p string) error { opened = append(opened, "<"+p+">"); return nil }
	a.newWindow()
	if len(opened) != 1 || opened[0] != "<>" {
		t.Errorf("File → New Window opened %v, want one window without a picture", opened)
	}
	a.NewWindow() // the key of the page right after the menu's: still one window
	if len(opened) != 1 {
		t.Errorf("the menu and Ctrl+N for one press opened %d windows, want 1", len(opened))
	}
	a.lastNew = time.Time{}
	a.NewWindow()
	if len(opened) != 2 {
		t.Errorf("a later Ctrl+N opened %d windows in all, want 2", len(opened))
	}
}

func TestPicturesDir(t *testing.T) {
	d := picturesDir()
	if d == "" || !isDir(d) {
		t.Errorf("picturesDir() = %q, want a folder that exists", d)
	}
	if h, _ := os.UserHomeDir(); d != h && filepath.Base(d) == "" {
		t.Errorf("picturesDir() = %q", d)
	}
}

// Reads the real clipboard of the machine, so it only runs when asked (see TestCopyImageNative).
func TestPasteFromSystemClipboard(t *testing.T) {
	if os.Getenv("MORMIVIEW_TEST_CLIPBOARD") == "" {
		t.Skip("set MORMIVIEW_TEST_CLIPBOARD=1 to run: it overwrites the clipboard")
	}
	src := NewApp()
	src.setImage(gridPNG(t))
	b64, _ := src.SelectionPNG(2, 1, 4, 3, 10, 6)
	if err := src.CopyImageNative(b64); err != nil {
		t.Skip("cannot put a picture on the clipboard here: ", err)
	}
	a := NewApp()
	if _, err := a.PasteLayer(0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if info := a.Image(); info.Name != "[clipboard image]" {
		t.Errorf("Image() = %+v", info)
	}
	if p := a.PixelAt(1, 1, 4, 3); !p.Ready || p.R != 60 || p.G != 80 || p.B != 77 {
		t.Errorf("the pasted pixel = %+v, want 60 80 77 (pixel 3,2 of the original)", p)
	}
}

func TestSaveTo(t *testing.T) {
	// a pasted picture is saved in every type, and becomes the file
	for _, c := range []struct {
		name   string
		exact  bool
		format string
	}{{"a.png", true, "png"}, {"b.jpg", false, "jpeg"}, {"c.bmp", true, "bmp"}, {"d.tiff", true, "tiff"}, {"e.gif", false, "gif"}, {"f.JPEG", false, "jpeg"}} {
		a := NewApp()
		a.setClip(clipImage())
		dir := t.TempDir()
		target := filepath.Join(dir, c.name)
		msg, err := a.saveTo(target, 0)
		if err != nil || !strings.Contains(msg, filepath.Base(target)) {
			t.Fatalf("saveTo(%s) = %q, %v", c.name, msg, err)
		}
		if cur := a.current(); cur.Path != target || cur.Dir != dir || a.clip != nil {
			t.Errorf("%s: after saving the picture is %+v, want the saved file", c.name, cur)
		}
		f, _ := os.Open(target)
		img, format, err := image.Decode(f)
		f.Close()
		if err != nil || img.Bounds().Dx() != 5 || img.Bounds().Dy() != 3 || format != strings.TrimPrefix(c.format, "") {
			t.Errorf("%s: decoded as %q %v, %v", c.name, format, img, err)
			continue
		}
		if c.exact {
			if got := color.NRGBAModel.Convert(img.At(3, 2)).(color.NRGBA); got != (color.NRGBA{R: 120, G: 140, B: 9, A: 255}) {
				t.Errorf("%s: pixel 3,2 = %v, want 120 140 9", c.name, got)
			}
		}
		if left, _ := filepath.Glob(filepath.Join(dir, ".mormiview-save-*")); len(left) != 0 {
			t.Errorf("%s: temporary files left: %v", c.name, left)
		}
	}
	// no extension: PNG; an unknown one: refused, and nothing written
	a := NewApp()
	a.setClip(clipImage())
	dir := t.TempDir()
	if _, err := a.saveTo(filepath.Join(dir, "noext"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "noext.png")); err != nil {
		t.Error("a name without an extension must get .png")
	}
	b := NewApp()
	b.setClip(clipImage())
	if _, err := b.saveTo(filepath.Join(dir, "x.webp"), 0); err == nil || b.clip == nil {
		t.Errorf("saving as .webp must be refused and the paste kept, err %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "x.*")); len(left) != 0 {
		t.Errorf("a refused save wrote %v", left)
	}
}

func TestSaveFileToOtherFolder(t *testing.T) {
	src := t.TempDir()
	pic := filepath.Join(src, "photo.jpg")
	var b bytes.Buffer
	jpeg.Encode(&b, clipImage(), &jpeg.Options{Quality: 90})
	os.WriteFile(pic, b.Bytes(), 0o644)
	os.WriteFile(filepath.Join(src, "other.png"), []byte("x"), 0o644)

	a := NewApp()
	a.setImage(pic)
	// the same folder under another name: the window stays on the picture, the file of the same type is a copy byte for byte
	if msg, err := a.saveTo(filepath.Join(src, "copy.jpeg"), 0); err != nil || a.current().Path != pic {
		t.Fatalf("same folder: %q, %v, picture %s", msg, err, a.current().Path)
	}
	if got, _ := os.ReadFile(filepath.Join(src, "copy.jpeg")); !bytes.Equal(got, b.Bytes()) {
		t.Error("a JPEG saved as JPEG must be a copy of the file")
	}
	// itself: nothing happens, and the file is not damaged
	if _, err := a.saveTo(pic, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(pic); !bytes.Equal(got, b.Bytes()) {
		t.Error("saving a picture over itself damaged it")
	}
	// another folder: the new file is the picture now, and its folder the one the arrow keys go through
	dst := t.TempDir()
	if _, err := a.saveTo(filepath.Join(dst, "moved.png"), 0); err != nil {
		t.Fatal(err)
	}
	if info := a.Image(); info.Path != filepath.Join(dst, "moved.png") || info.Dir != dst || info.Count != 1 || info.Index != 1 {
		t.Errorf("after saving to another folder Image() = %+v, want moved.png, 1 of 1 in %s", info, dst)
	}
	if a.workDir != dst {
		t.Errorf("working folder %q, want %q", a.workDir, dst)
	}
}

func TestSaveTurnedPicture(t *testing.T) {
	// a JPEG with the EXIF orientation 6, saved as PNG, is saved the way it is shown: turned
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 4)), &jpeg.Options{Quality: 90})
	src := filepath.Join(t.TempDir(), "turned.jpg")
	os.WriteFile(src, withOrientation(t, b.Bytes(), 6, binary.LittleEndian, "II"), 0o644)
	a := NewApp()
	a.setImage(src)
	out := filepath.Join(t.TempDir(), "turned.png")
	if _, err := a.saveTo(out, 0); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(out)
	defer f.Close()
	if cfg, _, err := image.DecodeConfig(f); err != nil || cfg.Width != 4 || cfg.Height != 8 {
		t.Errorf("saved size %v, %v; want 4 × 8", cfg, err)
	}
}

func TestSaveWhatCannotBeConverted(t *testing.T) {
	src := filepath.Join(t.TempDir(), "v.svg")
	os.WriteFile(src, []byte("<svg/>"), 0o644)
	a := NewApp()
	a.setImage(src)
	out := filepath.Join(t.TempDir(), "v.png")
	if _, err := a.saveTo(out, 0); err == nil {
		t.Error("an SVG cannot be saved as PNG: want an error")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a failed save left a file")
	}
}

func TestPickClipboardPictureType(t *testing.T) {
	for _, c := range []struct {
		offered []string
		want    string
	}{
		{[]string{"TARGETS", "TIMESTAMP", "MULTIPLE", "image/bmp"}, "image/bmp"}, // xrdp
		{[]string{"image/jpeg", "image/png", "text/plain"}, "image/png"},
		{[]string{"text/plain", "image/jpeg"}, "image/jpeg"},
		{[]string{"text/plain"}, "image/png"},
		{nil, "image/png"},
	} {
		if got := pickClipboardPictureType(c.offered); got != c.want {
			t.Errorf("%v: got %s, want %s", c.offered, got, c.want)
		}
	}
}

// A 32-bit BI_RGB bitmap with a 40-byte header, as xrdp puts on the clipboard: its alpha bytes are 0, and the picture must be opaque.
func TestClipboardBMPZeroAlphaIsOpaque(t *testing.T) {
	const w, h = 2, 2
	data := make([]byte, 54+w*h*4)
	copy(data, "BM")
	le := func(off int, v uint32) {
		data[off], data[off+1], data[off+2], data[off+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	le(2, uint32(len(data)))
	le(10, 54)
	le(14, 40)
	le(18, w)
	le(22, h)
	data[26], data[28] = 1, 32
	for i := 54; i < len(data); i += 4 {
		data[i], data[i+1], data[i+2], data[i+3] = 30, 20, 10, 0 // B G R, alpha 0
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "bmp" {
		t.Fatalf("decode: %v %q", err, format)
	}
	if r, g, b, a := img.At(1, 1).RGBA(); r>>8 != 10 || g>>8 != 20 || b>>8 != 30 || a>>8 != 255 {
		t.Fatalf("pixel %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	}
}
