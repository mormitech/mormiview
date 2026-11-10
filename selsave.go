package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveSelectionAs is the S key with a selection: it asks where to save the part x, y, w, h of the picture as shown (turned rot quarters) and saves
// only that part, as a picture of its own. The picture of the window is not touched, and the window does not move to the new file.
func (a *App) SaveSelectionAs(rot, x, y, w, h int) (string, error) {
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
	base, err := a.shownPixels(rot)
	if err != nil {
		return "", err
	}
	if w < 1 || h < 1 || !image.Rect(x, y, x+w, y+h).In(base.Bounds()) {
		return "", fmt.Errorf("the selection %d,%d %d × %d is not inside the image", x, y, w, h)
	}
	stem := "clipboard"
	if cur.Path != "" {
		stem = strings.TrimSuffix(cur.Name, filepath.Ext(cur.Name))
	}
	target, err := a.pickSave(wruntime.SaveDialogOptions{
		Title:                "Save selection as",
		DefaultDirectory:     cur.Dir,
		DefaultFilename:      stem + "-selection-" + time.Now().Format("150405") + ".png",
		CanCreateDirectories: true,
		Filters:              []wruntime.FileFilter{{DisplayName: "PNG, JPEG, GIF, BMP, TIFF", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.tif;*.tiff"}},
	})
	if err != nil || target == "" {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(target))
	if ext == "" {
		target, ext = target+".png", ".png"
	}
	format, ok := saveFormats[ext]
	if !ok {
		return "", fmt.Errorf("mormiView saves PNG, JPEG, GIF, BMP and TIFF; %s is none of them", ext)
	}
	part := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(part, part.Bounds(), base, image.Pt(x, y), draw.Src)
	tmp, err := os.CreateTemp(filepath.Dir(target), ".mormiview-save-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	err = encodePicture(tmp, part, format)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	os.Chmod(target, 0o644)
	return fmt.Sprintf("Saved the selection (%d × %d) as %s", w, h, filepath.Base(target)), nil
}
