package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SelectionPNG is the part x, y, w × h of the picture as it is shown (the web view's natural size dispW × dispH), cut out of the
// decoded file, as a PNG in base64: what Ctrl+C puts on the clipboard. Like the pixel readout it has the colors of the file, with no
// conversion. For a type Go cannot decode it returns "" and the UI cuts the part out of what the web view drew.
func (a *App) SelectionPNG(x, y, w, h, dispW, dispH int) (string, error) {
	pc, source, err := a.decoded()
	if err != nil {
		return "", err
	}
	if source != "go" {
		return "", nil
	}
	b := pc.img.Bounds()
	_, _, dw, dh := displayToSource(pc.orient, b.Dx(), b.Dy(), 0, 0)
	if dw != dispW || dh != dispH {
		return "", nil // not the picture the web view shows
	}
	if w < 1 || h < 1 || x < 0 || y < 0 || x+w > dw || y+h > dh {
		return "", fmt.Errorf("the selection %d,%d %d × %d is not inside the image", x, y, w, h)
	}
	var out image.Image
	if sub, ok := pc.img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok && pc.orient == 1 {
		out = sub.SubImage(image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+w, b.Min.Y+y+h)) // no copy, and no color conversion
	} else {
		n := image.NewNRGBA(image.Rect(0, 0, w, h))
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				sx, sy, _, _ := displayToSource(pc.orient, b.Dx(), b.Dy(), x+i, y+j)
				n.SetNRGBA(i, j, color.NRGBAModel.Convert(pc.img.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA))
			}
		}
		out = n
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, out); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// RotatePNG turns a PNG (in base64) clockwise by quarter turns, 1 to 3, without any color conversion: a selection that is copied
// from a turned view (R) has to be turned the way it is shown. The turn is only of the view; no file is changed.
func (a *App) RotatePNG(pngBase64 string, quarters int) (string, error) {
	quarters = (quarters%4 + 4) % 4
	if quarters == 0 {
		return pngBase64, nil
	}
	data, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		return "", err
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	dst := turnImage(src, quarters)
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, dst); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// CopyImageNative puts a PNG (in base64) on the system clipboard with the means of the system. The UI uses it when the web view's own
// clipboard does not take the picture.
func (a *App) CopyImageNative(pngBase64 string) error {
	data, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "mormiview-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return copyImageFile(f.Name())
}

// copyImageFile asks the system to put the picture of a PNG file on the clipboard.
func copyImageFile(path string) error {
	path = filepath.ToSlash(path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", `set the clipboard to (read (POSIX file "`+path+`") as «class PNGf»)`)
	case "windows":
		// System.Windows.Forms needs a single-threaded apartment
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms,System.Drawing; $i = [System.Drawing.Image]::FromFile('`+path+`'); [System.Windows.Forms.Clipboard]::SetImage($i); $i.Dispose()`)
	default:
		if p, err := exec.LookPath("wl-copy"); err == nil && os.Getenv("WAYLAND_DISPLAY") != "" {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			cmd = exec.Command(p, "--type", "image/png")
			cmd.Stdin = f
		} else if p, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command(p, "-selection", "clipboard", "-t", "image/png", "-i", path)
		} else {
			return errors.New("to copy an image install xclip (X11) or wl-clipboard (Wayland)")
		}
	}
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copying to the clipboard failed: %v %s", err, out)
	}
	return nil
}
