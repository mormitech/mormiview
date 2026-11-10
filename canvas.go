package main

import (
	"fmt"
	"image"
	"image/draw"
	"strings"
)

// The canvas of the picture changed without touching what is drawn: the chat panel's resize_canvas and crop (see mcp.go).

// canvasOffset is where the old picture (w × h) sits in the new canvas (W × H): the anchor says which edge or corner it keeps, as words
// ("bottom-left", "top", "center"…). A canvas that is smaller than the picture gives a negative offset: the picture is cut at the other sides.
func canvasOffset(anchor string, w, h, W, H int) (int, int, error) {
	a := strings.ToLower(strings.TrimSpace(anchor))
	if a == "" {
		a = "bottom-left"
	}
	ox, oy := (W-w)/2, (H-h)/2 // (center)
	words := strings.FieldsFunc(a, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for _, wd := range words {
		switch wd {
		case "left":
			ox = 0
		case "right":
			ox = W - w
		case "top":
			oy = 0
		case "bottom":
			oy = H - h
		case "center", "centre", "middle":
		default:
			return 0, 0, fmt.Errorf("the anchor %q is not top, bottom, left, right, center or a pair of them (top-left, bottom-right…)", anchor)
		}
	}
	return ox, oy, nil
}

// ResizeCanvas makes the canvas of the picture (as shown, rot quarter turns) w × h pixels. The picture itself is not scaled: it stays where the anchor says
// ("bottom-left": the new room is above it and to its right), and the new room has the color bg (#rrggbbaa; "" is transparent where the type of the
// file can be, white otherwise). A smaller canvas cuts the picture. One step of the history, so undo gives the old canvas back.
func (a *App) ResizeCanvas(rot, w, h int, anchor, bg string) (ImageInfo, error) {
	if w < 1 || h < 1 || w*h > maxPixels {
		return ImageInfo{}, fmt.Errorf("a canvas of %d × %d is too small or too big (up to %d pixels)", w, h, maxPixels)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	file := a.current().Path
	if bg == "" {
		bg = "#00000000"
		if !canAlpha(file) {
			bg = "#FFFFFFFF"
		}
	}
	c, err := parseRGBA(bg)
	if err != nil {
		return ImageInfo{}, err
	}
	b := base.Bounds()
	ox, oy, err := canvasOffset(anchor, b.Dx(), b.Dy(), w, h)
	if err != nil {
		return ImageInfo{}, err
	}
	n := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(n, n.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	r := image.Rect(ox, oy, ox+b.Dx(), oy+b.Dy()).Intersect(n.Bounds())
	draw.Draw(n, r, base, b.Min.Add(r.Min.Sub(image.Pt(ox, oy))), draw.Src)
	if err := a.setPixelsEmit(n, file, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

// CropPicture keeps the part x, y, w × h of the picture (as shown, rot quarter turns) and cuts the rest away. One step of the history.
func (a *App) CropPicture(rot, x, y, w, h int) (ImageInfo, error) {
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	b := base.Bounds()
	r := image.Rect(x, y, x+w, y+h)
	if w < 1 || h < 1 || !r.In(b) {
		return ImageInfo{}, fmt.Errorf("the part %d,%d %d × %d is not inside the image (%d × %d)", x, y, w, h, b.Dx(), b.Dy())
	}
	n := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(n, n.Bounds(), base, r.Min, draw.Src)
	if err := a.setPixelsEmit(n, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}
