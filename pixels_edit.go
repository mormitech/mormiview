package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
)

// Setting single pixels and making a blank picture of a chosen size and color: the chat panel's set_pixels and new_image (see mcp.go).

// parseRGBA reads "#rrggbbaa" (what the tools send).
func parseRGBA(s string) (color.NRGBA, error) {
	if len(s) != 9 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("%q is not a color (#rrggbbaa)", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("%q is not a color (#rrggbbaa)", s)
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

const maxPaintPixels = 1 << 20

// PaintPixels sets pixels of the picture as it is shown (turned rot quarters): list is JSON, [[x, y, "#rrggbbaa"], …]. The color replaces what was there
// (alpha too, so a transparent color makes a hole). It is one step of the history, like a stroke of the pencil; the UI shows the answer.
func (a *App) PaintPixels(rot int, list string) (ImageInfo, error) {
	var px [][3]any
	if err := json.Unmarshal([]byte(list), &px); err != nil {
		return ImageInfo{}, fmt.Errorf("the pixels are not a list of [x, y, color]")
	}
	if len(px) == 0 || len(px) > maxPaintPixels {
		return ImageInfo{}, fmt.Errorf("%d pixels: between 1 and %d at a time", len(px), maxPaintPixels)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	b := base.Bounds()
	for i, p := range px {
		x, okx := p[0].(float64)
		y, oky := p[1].(float64)
		cs, okc := p[2].(string)
		if !okx || !oky || !okc {
			return ImageInfo{}, fmt.Errorf("pixel %d is not [x, y, color]", i)
		}
		c, err := parseRGBA(cs)
		if err != nil {
			return ImageInfo{}, err
		}
		if !image.Pt(int(x), int(y)).In(b) {
			return ImageInfo{}, fmt.Errorf("the pixel %d,%d is outside the image (%d × %d)", int(x), int(y), b.Dx(), b.Dy())
		}
		base.SetNRGBA(int(x), int(y), c)
	}
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

// NewBlankColor is NewBlank with a color (#rrggbbaa; transparent is alpha 0): the picture of this window is new, w × h, and has no file yet.
func (a *App) NewBlankColor(w, h int, rgba string) error {
	if w < 1 || h < 1 || w*h > maxPixels {
		return fmt.Errorf("an image of %d × %d is too small or too big (up to %d pixels)", w, h, maxPixels)
	}
	c, err := parseRGBA(rgba)
	if err != nil {
		return err
	}
	n := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(n, n.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	a.mu.Lock()
	a.origin = "new"
	a.mu.Unlock()
	return a.setPixelsBlank(n, true)
}

const gridChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// pixelGrid describes a small picture in text, pixel for pixel: a palette (a character for each color; "." is fully transparent) and one string
// for each row. It is what set_pixels takes as "grid" and "palette". It is nil for a picture with more than 62 colors.
func pixelGrid(img *image.NRGBA, r image.Rectangle) map[string]any {
	palette := map[string]string{}
	index := map[color.NRGBA]byte{}
	rows := make([]string, 0, r.Dy())
	next := 0 // the next character of gridChars
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := make([]byte, 0, r.Dx())
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A == 0 {
				c = color.NRGBA{}
			}
			ch, ok := index[c]
			if !ok {
				if c.A == 0 {
					ch = '.'
					palette["."] = "transparent"
				} else {
					if next >= len(gridChars) {
						return nil
					}
					ch = gridChars[next]
					next++
					hex := fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
					if c.A != 255 {
						hex += fmt.Sprintf("%02X", c.A)
					}
					palette[string(ch)] = hex
				}
				index[c] = ch
			}
			row = append(row, ch)
		}
		rows = append(rows, string(row))
	}
	return map[string]any{"palette": palette, "rows": rows}
}
