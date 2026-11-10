package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// DrawLayer puts a picture the UI drew (a line, a rectangle, a text: a PNG with transparent parts) into the picture of this window, with its top left
// corner at x, y, as the picture is shown (rot quarter turns clockwise). It is a step of the history, like a stroke of the pencil. The part that
// sticks out of the picture is cut off.
func (a *App) DrawLayer(rot, x, y int, pngBase64 string) (ImageInfo, error) {
	raw, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("the drawing is not an image")
	}
	layer, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return ImageInfo{}, fmt.Errorf("the drawing is not a PNG image")
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	lb := layer.Bounds()
	draw.Draw(base, image.Rect(x, y, x+lb.Dx(), y+lb.Dy()), layer, lb.Min, draw.Over)
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}
