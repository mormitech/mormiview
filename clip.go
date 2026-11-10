package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// clipboardName is what the title of a window says instead of the name of a file, for a picture pasted from the clipboard.
const clipboardName = "[clipboard image]"

// unsavedName is the title of a picture that has no file yet, by where it came from: "new" (Create image), "scan", else the clipboard.
func unsavedName(origin string) string {
	switch origin {
	case "new":
		return "[new image]"
	case "scan":
		return "[scanned image]"
	}
	return clipboardName
}

// A picture pasted from the clipboard has no file: it lives in this window (App.clip) until it is saved with Save As. Its "folder" is
// the Pictures folder of the user, and the folder Save As starts in; when it is saved somewhere else, that folder becomes the one the
// window works in (see saveTo).

// workDirLocked is the folder of a picture without a file, App.mu held.
func (a *App) workDirLocked() string {
	if a.workDir == "" {
		a.workDir = picturesDir()
	}
	return a.workDir
}

// picturesDir is the Pictures folder of the user (the "Images" of the system), or their home folder when there is none.
func picturesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "linux" { // it may have another name or place: ~/.config/user-dirs.dirs says
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		if b, err := os.ReadFile(filepath.Join(cfg, "user-dirs.dirs")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(line), "XDG_PICTURES_DIR="); ok {
					d := strings.ReplaceAll(strings.Trim(v, `"`), "$HOME", home)
					if fi, err := os.Stat(d); err == nil && fi.IsDir() {
						return d
					}
				}
			}
		}
	}
	if d := filepath.Join(home, "Pictures"); isDir(d) {
		return d
	}
	return home
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// ClipboardHasImage is whether the clipboard has a picture now (the empty window offers it).
func (a *App) ClipboardHasImage() bool {
	img, err := readClipboardImage()
	return err == nil && img != nil && img.Bounds().Dx() > 0 && img.Bounds().Dy() > 0
}

// NewBlank makes a new picture of w × h pixels, white or transparent, the picture of this window (it has no file yet, like a clipboard picture).
func (a *App) NewBlank(w, h int, transparent bool) error {
	if w < 1 || h < 1 || w*h > maxPixels {
		return fmt.Errorf("an image of %d × %d is too small or too big", w, h)
	}
	n := image.NewNRGBA(image.Rect(0, 0, w, h))
	if !transparent {
		draw.Draw(n, n.Bounds(), image.White, image.Point{}, draw.Src)
	}
	a.mu.Lock()
	a.origin = "new"
	a.mu.Unlock()
	return a.setPixelsBlank(n, true)
}

// setClip makes the picture from the clipboard the picture of this window. It has no file, but it is not modified until something is done to it (the clipboard still
// has it), so closing it, or going to another picture, asks nothing; the first change makes it modified.
func (a *App) setClip(img image.Image) error {
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 || b.Dx()*b.Dy() > maxPixels {
		return fmt.Errorf("the image on the clipboard is %d × %d: too small or too big", b.Dx(), b.Dy())
	}
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy())) // the colors as they are, not premultiplied
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	a.mu.Lock()
	a.origin = "clip"
	a.mu.Unlock()
	return a.setPixelsBlank(n, true)
}

// setPixels makes img the picture of this window, kept in memory: a picture with no file (file is ""), or the file with something
// pasted into it (the file is still where Save As starts, and the name in the title, with a * after it).
func (a *App) setPixels(img *image.NRGBA, file string) error { return a.setPixelsEmit(img, file, true) }

func (a *App) setPixelsEmit(img *image.NRGBA, file string, emit bool) error {
	return a.setPixelsFull(img, file, emit, false)
}

// setPixelsBlank is a new picture (a blank one, see NewBlank, or one from the clipboard, see setClip) that nothing has been done to: it has no changes, so nothing asks
// about it when it is closed.
func (a *App) setPixelsBlank(img *image.NRGBA, blank bool) error {
	return a.setPixelsFull(img, "", true, blank)
}

func (a *App) setPixelsFull(img *image.NRGBA, file string, emit, blank bool) error {
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return err
	}
	png := buf.Bytes()
	if file != "" { // the colors are shown as they were in the file
		png = withChunks(png, colorChunks(file))
	}
	a.mu.Lock()
	a.image = file
	a.clip, a.clipPNG = img, png
	a.blank = blank
	if blank { // a new or pasted picture is the original that Discard changes goes back to
		a.origImg = img
	}
	a.imgSeq++
	a.pix = pixelCache{seq: a.imgSeq, img: img, orient: 1} // it is decoded already: the readout and Ctrl+C read from it
	a.workDirLocked()
	a.fl = nil
	if blank { // a new picture is not part of an animation
		a.gifs = nil
	}
	a.gifNoteEditLocked(img, file)
	a.newStateLocked()
	a.mu.Unlock()
	if a.ctx != nil {
		a.titleWindow()
		if emit {
			a.emit("image", a.Image())
		}
	}
	return nil
}

// shownPixels is a copy of the picture of this window the way it is shown (EXIF turn applied, then turned rot quarters clockwise).
func (a *App) shownPixels(rot int) (*image.NRGBA, error) {
	pc, source, err := a.decoded()
	if err != nil {
		return nil, err
	}
	if source != "go" {
		return nil, fmt.Errorf("mormiView cannot change %s: it can only paste into JPEG, PNG, GIF, BMP, WebP and TIFF images", a.current().Name)
	}
	return turnImage(shownImage(pc), rot), nil
}

// turnImage is a copy of src (as NRGBA) turned clockwise by quarter turns, 0 to 3 (any number is taken modulo 4).
func turnImage(src image.Image, quarters int) *image.NRGBA {
	quarters = (quarters%4 + 4) % 4
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if quarters%2 == 1 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var tx, ty int
			switch quarters {
			case 0:
				tx, ty = x, y
			case 1:
				tx, ty = h-1-y, x
			case 2:
				tx, ty = w-1-x, h-1-y
			default:
				tx, ty = y, w-1-x
			}
			dst.SetNRGBA(tx, ty, color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA))
		}
	}
	return dst
}

// pasteImage is Edit → Paste. It is the UI that knows the selection and the pointer, so the menu only tells it to paste.
func (a *App) pasteImage() { a.emit("menu-paste") }

var errNoClipboardPicture = errors.New("there is no image on the clipboard")

// readClipboardImage asks the system for the picture on its clipboard (the web build puts the browser's clipboard in, see web_main.go).
var readClipboardImage = readSystemClipboardImage

func readSystemClipboardImage() (image.Image, error) {
	f, err := os.CreateTemp("", "mormiview-*.img")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	var cmd *exec.Cmd
	toFile := true // the helper writes the file itself; otherwise its output is the picture
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript",
			"-e", "try", "-e", "set d to (the clipboard as «class PNGf»)", "-e", "on error", "-e", "set d to (the clipboard as «class TIFF»)", "-e", "end try",
			"-e", `set f to open for access POSIX file "`+filepath.ToSlash(path)+`" with write permission`,
			"-e", "set eof f to 0", "-e", "write d to f", "-e", "close access f")
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms,System.Drawing; $i = [System.Windows.Forms.Clipboard]::GetImage(); if ($i -eq $null) { exit 3 }; $i.Save('`+filepath.ToSlash(path)+`', [System.Drawing.Imaging.ImageFormat]::Png)`)
	default:
		toFile = false
		// The clipboard is asked which types it offers, and the best picture type is taken: not every program offers PNG (xrdp, the
		// clipboard of a remote desktop, offers only image/bmp).
		if p, err := exec.LookPath("wl-paste"); err == nil && os.Getenv("WAYLAND_DISPLAY") != "" {
			cmd = exec.Command(p, "--type", clipboardPictureType(exec.Command(p, "--list-types")))
		} else if p, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command(p, "-selection", "clipboard", "-t", clipboardPictureType(exec.Command(p, "-selection", "clipboard", "-t", "TARGETS", "-o")), "-o")
		} else {
			return nil, errors.New("to paste an image install xclip (X11) or wl-clipboard (Wayland)")
		}
	}
	hideWindow(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, errNoClipboardPicture
	}
	data := out.Bytes()
	if toFile {
		if data, err = os.ReadFile(path); err != nil {
			return nil, err
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errNoClipboardPicture
	}
	return img, nil
}

// clipboardPicturePreference is the order in which the picture types of a Linux clipboard are taken (all of them can be decoded).
var clipboardPicturePreference = []string{"image/png", "image/bmp", "image/x-bmp", "image/x-ms-bmp", "image/tiff", "image/webp", "image/jpeg", "image/gif"}

// clipboardPictureType runs list (it prints the types the clipboard offers, one a line) and returns the best picture type of them;
// image/png when the list cannot be had or has none of them (then the paste says there is no picture, as before).
func clipboardPictureType(list *exec.Cmd) string {
	hideWindow(list)
	out, err := list.Output()
	if err != nil {
		return "image/png"
	}
	return pickClipboardPictureType(strings.Fields(string(out)))
}

func pickClipboardPictureType(offered []string) string {
	for _, want := range clipboardPicturePreference {
		for _, t := range offered {
			if strings.EqualFold(strings.TrimSpace(t), want) {
				return t
			}
		}
	}
	return "image/png"
}
