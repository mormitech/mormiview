package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image/color/palette"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// saveFormats are the types Save As writes, by extension, with the group each belongs to (jpg, jpeg and so on are one type).
var saveFormats = map[string]string{
	".png": "png", ".jpg": "jpeg", ".jpeg": "jpeg", ".jpe": "jpeg", ".jfif": "jpeg", ".gif": "gif", ".bmp": "bmp", ".tif": "tiff", ".tiff": "tiff",
}

// SaveAs is the S key (and File → Save As…): it asks where to save the picture, and saves it there, in the type the name ends with
// (PNG, JPEG, GIF, BMP or TIFF; a name without one gets .png). A picture that was pasted becomes the file it was saved to. Saving to
// another folder makes that folder the one this window works in: the arrow keys go through its pictures from then on. The text
// returned is for the UI to show; it is empty when the dialog was cancelled.
//
// A picture that came from a file and has changes (or is turned) first asks: overwrite the file, save as a new one (the dialog then offers
// the name with the date and time after it), or cancel.
func (a *App) SaveAs(rot int) (string, error) { return a.saveAsDialog(rot, true) }

// saveAsDialog is SaveAs; ask is false when the user was asked about the changes already (see discardOK).
func (a *App) saveAsDialog(rot int, ask bool) (string, error) {
	a.mu.Lock()
	if a.saving {
		a.mu.Unlock()
		return "", nil
	}
	a.saving = true
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.saving = false; a.mu.Unlock() }()

	cur := a.current()
	if cur.URL == "" {
		return "", nil
	}
	name := cur.Name
	stamp := false
	var gm gifSaveChoice // what of an animation is saved (see askGifSave), and the number of the frame that is shown
	frameNo := 0
	if g := a.gifSaving(); g != nil { // an animation: the whole of it, or the frame that is shown
		a.mu.Lock()
		frame, count := g.cur, len(g.frames)
		a.mu.Unlock()
		mode := a.askGifSave(frame, count)
		if mode == gifSaveCancel {
			return "", nil
		}
		a.mu.Lock()
		a.gifMode = mode
		a.mu.Unlock()
		defer func() { a.mu.Lock(); a.gifMode = 0; a.mu.Unlock() }()
		gm, frameNo = mode, frame+1
		if mode == gifSaveFrame {
			ask = false // a new file: nothing to overwrite
		}
	}
	_, writable := saveFormats[strings.ToLower(filepath.Ext(cur.Path))]
	if ask && cur.Path != "" && writable && (cur.Modified || (rot%4+4)%4 != 0) {
		switch a.askOverwrite(cur.Name) {
		case overwriteCancel:
			return "", nil
		case overwriteFile:
			return a.saveTo(cur.Path, rot)
		}
		stamp = true
	}
	if cur.Path == "" {
		name = "clipboard-" + time.Now().Format("20060102-150405") + ".png"
	} else if _, ok := saveFormats[strings.ToLower(filepath.Ext(name))]; !ok { // a type that is not written (WebP, SVG…): PNG
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".png"
	}
	// a picture with transparent parts is offered as PNG, which keeps them (JPEG cannot; Save As to JPEG still works, with white)
	if ext := strings.ToLower(filepath.Ext(name)); saveFormats[ext] == "jpeg" || saveFormats[ext] == "bmp" {
		if a.HasTransparency() {
			name = strings.TrimSuffix(name, filepath.Ext(name)) + ".png"
		}
	}
	filter := wruntime.FileFilter{DisplayName: "PNG, JPEG, GIF, BMP, TIFF", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.tif;*.tiff"}
	if gm != 0 { // an animation: the whole of it is a GIF (and only a GIF is offered), a frame is a picture
		stem := "animation-" + time.Now().Format("20060102-150405")
		if cur.Path != "" {
			stem = strings.TrimSuffix(cur.Name, filepath.Ext(cur.Name))
		}
		if gm == gifSaveAll {
			name = stem + ".gif"
			filter = wruntime.FileFilter{DisplayName: "GIF animation", Pattern: "*.gif"}
		} else {
			name = fmt.Sprintf("%s-frame-%d.png", stem, frameNo)
		}
	}
	if stamp {
		name = changedName(name, time.Now())
	}
	target, err := a.pickSave(wruntime.SaveDialogOptions{
		Title:                "Save image as",
		DefaultDirectory:     cur.Dir,
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters:              []wruntime.FileFilter{filter},
	})
	if err != nil || target == "" {
		return "", err
	}
	return a.saveTo(target, rot)
}

// saveAs is File → Save As…: the UI knows how the picture is turned, so the menu only tells it to save.
func (a *App) saveAs() { a.emit("menu-save") }

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b) // two names for one file are not told apart; saving over it is then refused by the rename check
}

// saveTo writes the picture of this window to target, turned rot quarters clockwise as the view shows it, with whatever was pasted into
// it. When that is a new folder, or the picture had no file, was changed or is turned, the saved file becomes the picture of the window
// (and is not modified or turned any more).
func (a *App) saveTo(target string, rot int) (string, error) {
	ext := strings.ToLower(filepath.Ext(target))
	if ext == "" {
		target, ext = target+".png", ".png"
	}
	format, ok := saveFormats[ext]
	if !ok {
		return "", fmt.Errorf("mormiView saves PNG, JPEG, GIF, BMP and TIFF; %s is none of them", ext)
	}
	if a.floating() { // the lifted part is part of what is saved; the picture it makes is turned for good
		if err := a.bakeFloat(false); err != nil {
			return "", err
		}
		rot = 0
	}
	cur := a.current()
	if cur.URL == "" {
		return "", nil
	}
	rot = (rot%4 + 4) % 4
	if cur.Path != "" && samePath(cur.Path, target) && !cur.Modified && rot == 0 {
		return cur.Name + " is this image already", nil
	}
	transparent := a.HasTransparency() // asked now: the file that is saved is what is shown at this moment
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mormiview-save-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // nothing is left behind when it did not work, and nothing when it is renamed
	err = a.writePicture(tmp, cur, format, rot)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	lost := ""
	if format == "jpeg" && transparent {
		lost = "; the transparent parts are white (JPEG has none)"
	}
	os.Chmod(target, 0o644)

	dir := filepath.Dir(target)
	if cur.Path == "" || cur.Modified || rot != 0 || !samePath(filepath.Dir(cur.Path), dir) {
		a.mu.Lock()
		a.workDir = dir
		a.mu.Unlock()
		newFolder := cur.Path == "" || !samePath(filepath.Dir(cur.Path), dir)
		if err := a.setImage(target); err != nil {
			return "", err
		}
		if !newFolder { // changed or turned, saved next to where it came from
			return "Saved " + filepath.Base(target) + lost, nil
		}
		return fmt.Sprintf("Saved %s; %s is the folder now%s", filepath.Base(target), dir, lost), nil
	}
	return "Saved " + filepath.Base(target) + lost, nil
}

// writePicture writes the picture to f in the given format. A file that is of that type already is copied as it is, which keeps
// everything in it (EXIF, profiles, animation); anything else is decoded and written again, turned the way it is shown.
func (a *App) writePicture(f *os.File, cur ImageInfo, format string, rot int) error {
	if g := a.gifSaving(); g != nil { // an animation: all of it as a GIF, else the frame that is shown (the file itself is not copied then)
		a.mu.Lock()
		mode, frames, delays, shown := a.gifMode, g.frames, g.delays, g.frames[g.cur]
		a.mu.Unlock()
		if mode == 0 { // not asked (a tool saves): a GIF is the whole animation, any other type the frame that is shown
			mode = gifSaveFrame
			if format == "gif" {
				mode = gifSaveAll
			}
		}
		if mode == gifSaveAll && format == "gif" {
			return encodeGifAnimation(f, frames, delays)
		}
		a.mu.Lock()
		q := a.saveQuality
		a.mu.Unlock()
		if q == 0 {
			q = 95
		}
		return encodePictureQ(f, turnImage(shown, rot), format, q)
	}
	if cur.Path != "" && !cur.Modified && rot == 0 && saveFormats[strings.ToLower(filepath.Ext(cur.Path))] == format {
		src, err := os.Open(cur.Path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(f, src)
		return err
	}
	pc, source, err := a.decoded()
	if err != nil {
		return err
	}
	if source != "go" {
		return fmt.Errorf("mormiView cannot convert %s to another type", cur.Name)
	}
	a.mu.Lock()
	q := a.saveQuality
	a.mu.Unlock()
	if q == 0 {
		q = 95
	}
	return encodePictureQ(f, turnImage(shownImage(pc), rot), format, q)
}

// shownImage is the picture the way it is shown, with the EXIF orientation applied.
func shownImage(pc pixelCache) image.Image {
	if pc.orient == 1 {
		return pc.img
	}
	b := pc.img.Bounds()
	_, _, dw, dh := displayToSource(pc.orient, b.Dx(), b.Dy(), 0, 0)
	n := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy, _, _ := displayToSource(pc.orient, b.Dx(), b.Dy(), x, y)
			n.SetNRGBA(x, y, color.NRGBAModel.Convert(pc.img.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA))
		}
	}
	return n
}

func encodePicture(w io.Writer, img image.Image, format string) error {
	return encodePictureQ(w, img, format, 95)
}

// encodePictureQ is encodePicture with the quality (1 to 100) of a JPEG.
func encodePictureQ(w io.Writer, img image.Image, format string, quality int) error {
	switch format {
	case "png":
		return png.Encode(w, img)
	case "jpeg": // no transparency in JPEG: a transparent pixel becomes white
		b := img.Bounds()
		flat := image.NewRGBA(b)
		draw.Draw(flat, b, image.White, image.Point{}, draw.Src)
		draw.Draw(flat, b, img, b.Min, draw.Over)
		return jpeg.Encode(w, flat, &jpeg.Options{Quality: min(max(quality, 1), 100)})
	case "gif":
		if o, ok := img.(interface{ Opaque() bool }); !ok || !o.Opaque() { // a transparent color of its own, or the transparent parts would be black
			return gif.Encode(w, img, &gif.Options{NumColors: 256, Quantizer: transparentPlan9{}, Drawer: draw.FloydSteinberg})
		}
		return gif.Encode(w, img, &gif.Options{NumColors: 256, Drawer: draw.FloydSteinberg})
	case "bmp":
		return bmp.Encode(w, img)
	case "tiff":
		return tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
	}
	return fmt.Errorf("cannot write %s", format)
}

// transparentPlan9 is the 256 color palette of Go with the first color made transparent, for a GIF of a picture with transparent parts.
type transparentPlan9 struct{}

func (transparentPlan9) Quantize(p color.Palette, m image.Image) color.Palette {
	return append(color.Palette{color.Transparent}, palette.Plan9[:255]...)
}
