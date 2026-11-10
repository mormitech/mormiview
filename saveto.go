package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// SaveToPathQ is SaveToPath with the quality (1 to 100) of a JPEG file; 0 is the usual 95. (A JPEG that was not changed is copied as it is.)
func (a *App) SaveToPathQ(path string, rot int, overwrite bool, quality int) (string, error) {
	if quality < 0 || quality > 100 {
		return "", fmt.Errorf("the quality is 1 to 100")
	}
	a.mu.Lock()
	a.saveQuality = quality
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.saveQuality = 0; a.mu.Unlock() }()
	return a.SaveToPath(path, rot, overwrite)
}

// SaveToPath saves the picture of this window to path without a dialog (the chat tools' save_image and the control command saveto); rot is how the view is turned.
// A file that exists is only replaced with overwrite.
func (a *App) SaveToPath(path string, rot int, overwrite bool) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%q is not a full path", path)
	}
	if fi, err := os.Stat(path); err == nil && (fi.IsDir() || !overwrite) {
		return "", fmt.Errorf("%s exists already", path)
	}
	if cur := a.current(); cur.URL == "" {
		return "", fmt.Errorf("there is no image to save")
	}
	return a.saveTo(path, rot)
}

// SaveRegionTo writes a picture (base64 PNG, the selected part) to a full path without a dialog; the type is the one the name ends with. A file that
// exists is only replaced with overwrite; quality (1 to 100, 0 is the usual 95) is that of a JPEG.
func (a *App) SaveRegionTo(b64, path string, overwrite bool, quality int) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%q is not a full path", path)
	}
	if quality < 0 || quality > 100 {
		return "", fmt.Errorf("the quality is 1 to 100")
	}
	if quality == 0 {
		quality = 95
	}
	format, ok := saveFormats[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", fmt.Errorf("cannot save as %q: use png, jpg, gif, bmp or tif", filepath.Ext(path))
	}
	if fi, err := os.Stat(path); err == nil && (fi.IsDir() || !overwrite) {
		return "", fmt.Errorf("%s exists already", path)
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := encodePictureQ(&buf, img, format, quality); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return "Saved " + path, nil
}
