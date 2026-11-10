package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Scaling, turning and flipping the picture, and laying a picture from a file over it: the chat panel's resize_image, rotate_image, flip_image and
// place_image (see mcp.go). Each is one step of the history, like the drawing tools.

type transformSpec struct {
	Op         string   `json:"op"` // resize, rotate, flip, place
	W          int      `json:"w"`  // resize: the new size; with one of the two the other keeps the proportions
	H          int      `json:"h"`
	Scale      float64  `json:"scale"`  // resize, place: 1 is the size it has
	Filter     string   `json:"filter"` // resize: smooth (the default, for photos), sharp (bilinear), nearest (whole pixels, for pixel art)
	Angle      float64  `json:"angle"`  // rotate: degrees clockwise
	Bg         string   `json:"bg"`     // rotate: the color of the corners (#rrggbbaa; transparent where the file can be, else white)
	Crop       bool     `json:"crop"`   // rotate: keep the size of the picture and cut the corners off, instead of making the canvas bigger
	Horizontal bool     `json:"horizontal"`
	Vertical   bool     `json:"vertical"`
	Path       string   `json:"path"` // place: the picture file
	X          int      `json:"x"`    // place: where its top left corner goes
	Y          int      `json:"y"`
	Opacity    *float64 `json:"opacity"` // place: 0 to 1
}

func scalerOf(name string) xdraw.Interpolator {
	switch name {
	case "nearest":
		return xdraw.NearestNeighbor
	case "sharp":
		return xdraw.BiLinear
	}
	return xdraw.CatmullRom
}

// Transform does one of resize, rotate, flip and place on the picture as shown (turned rot quarters). The UI shows the answer.
func (a *App) Transform(rot int, spec string) (ImageInfo, error) {
	var sp transformSpec
	if err := json.Unmarshal([]byte(spec), &sp); err != nil {
		return ImageInfo{}, fmt.Errorf("the change is not understood: %v", err)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	file := a.current().Path
	var out, orig *image.NRGBA
	var rotKey string
	switch sp.Op {
	case "resize":
		out, err = resizePicture(base, sp)
	case "rotate":
		// consecutive turns (nothing else changed the picture in between) are all made from the picture as it was before the first one, by the angles added up
		rotKey = fmt.Sprintf("whole %d %v %q", rot, sp.Crop, sp.Bg)
		var total float64
		orig, total = a.rotationBase(base, rotKey, sp.Angle, true)
		sp.Angle = total
		if math.Mod(total, 360) == 0 {
			out = cloneNRGBA(orig)
		} else {
			out, err = rotatePicture(orig, sp, canAlpha(file))
		}
	case "flip":
		out, err = flipPicture(base, sp)
	case "place":
		out, err = placePicture(base, sp)
	default:
		err = fmt.Errorf("the change is resize, rotate, flip or place, not %q", sp.Op)
	}
	if err != nil {
		return ImageInfo{}, err
	}
	if err := a.setPixelsEmit(out, file, false); err != nil {
		return ImageInfo{}, err
	}
	if orig != nil {
		a.keepRotation(orig, rotKey, sp.Angle)
	}
	return a.Image(), nil
}

func resizePicture(src *image.NRGBA, sp transformSpec) (*image.NRGBA, error) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	w, h := sp.W, sp.H
	switch {
	case sp.Scale > 0:
		w, h = int(math.Round(float64(sw)*sp.Scale)), int(math.Round(float64(sh)*sp.Scale))
	case w > 0 && h == 0:
		h = int(math.Round(float64(sh) * float64(w) / float64(sw)))
	case h > 0 && w == 0:
		w = int(math.Round(float64(sw) * float64(h) / float64(sh)))
	}
	if w < 1 || h < 1 || w*h > maxPixels {
		return nil, fmt.Errorf("an image of %d × %d is too small or too big (up to %d pixels); give w and/or h, or scale", w, h, maxPixels)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	scalerOf(sp.Filter).Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst, nil
}

func flipPicture(src *image.NRGBA, sp transformSpec) (*image.NRGBA, error) {
	if !sp.Horizontal && !sp.Vertical {
		return nil, fmt.Errorf("flip needs horizontal and/or vertical")
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(src.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			if sp.Horizontal {
				sx = w - 1 - x
			}
			if sp.Vertical {
				sy = h - 1 - y
			}
			dst.SetNRGBA(x, y, src.NRGBAAt(sx, sy))
		}
	}
	return dst, nil
}

func rotatePicture(src *image.NRGBA, sp transformSpec, alpha bool) (*image.NRGBA, error) {
	deg := math.Mod(sp.Angle, 360)
	if deg < 0 {
		deg += 360
	}
	if deg == 0 {
		return nil, fmt.Errorf("an angle of 0 turns nothing")
	}
	if deg == 90 || deg == 180 || deg == 270 { // quarter turns lose nothing
		return turnImage(src, int(deg/90)), nil
	}
	bg := "#00000000"
	if !alpha {
		bg = "#FFFFFFFF"
	}
	if sp.Bg != "" {
		bg = sp.Bg
	}
	c, err := parseRGBA(bg)
	if err != nil {
		return nil, err
	}
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	rad := deg * math.Pi / 180
	cs, sn := math.Cos(rad), math.Sin(rad)
	dw, dh := sw, sh
	if !sp.Crop {
		dw, dh = math.Ceil(math.Abs(sw*cs)+math.Abs(sh*sn)), math.Ceil(math.Abs(sw*sn)+math.Abs(sh*cs))
	}
	if int(dw)*int(dh) > maxPixels {
		return nil, fmt.Errorf("the turned image would be too big")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, int(dw), int(dh)))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	// source (x, y) lands at the middle of the new picture plus the turned offset from the middle of the old one
	aff := f64.Aff3{cs, -sn, dw/2 - (sw/2)*cs + (sh/2)*sn, sn, cs, dh/2 - (sw/2)*sn - (sh/2)*cs}
	xdraw.BiLinear.Transform(dst, aff, src, src.Bounds(), xdraw.Over, nil)
	return dst, nil
}

// placePicture lays the picture of a file over base with its top left corner at x, y (what hangs over the edge is cut off).
func placePicture(base *image.NRGBA, sp transformSpec) (*image.NRGBA, error) {
	if !isImage(sp.Path) {
		return nil, fmt.Errorf("%q is not an image file", sp.Path)
	}
	f, err := os.Open(sp.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("the image cannot be read: %v", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if sp.Scale > 0 && sp.Scale != 1 {
		w, h = max(1, int(math.Round(float64(w)*sp.Scale))), max(1, int(math.Round(float64(h)*sp.Scale)))
	}
	if sp.W > 0 || sp.H > 0 {
		switch {
		case sp.W > 0 && sp.H > 0:
			w, h = sp.W, sp.H
		case sp.W > 0:
			h = max(1, int(math.Round(float64(h)*float64(sp.W)/float64(w))))
			w = sp.W
		default:
			w = max(1, int(math.Round(float64(w)*float64(sp.H)/float64(h))))
			h = sp.H
		}
	}
	if w*h > maxPixels {
		return nil, fmt.Errorf("the image is too big to place")
	}
	layer := image.NewNRGBA(image.Rect(0, 0, w, h))
	scalerOf(sp.Filter).Scale(layer, layer.Bounds(), img, b, xdraw.Src, nil)
	if sp.Opacity != nil && *sp.Opacity < 1 {
		o := math.Max(0, *sp.Opacity)
		for i := 3; i < len(layer.Pix); i += 4 {
			layer.Pix[i] = clamp8(float64(layer.Pix[i]) * o)
		}
	}
	out := image.NewNRGBA(base.Bounds())
	copy(out.Pix, base.Pix)
	r := image.Rect(sp.X, sp.Y, sp.X+w, sp.Y+h)
	if r.Intersect(out.Bounds()).Empty() {
		return nil, fmt.Errorf("the image at %d,%d (%d × %d) is outside this image (%d × %d)", sp.X, sp.Y, w, h, out.Bounds().Dx(), out.Bounds().Dy())
	}
	draw.Draw(out, r, layer, image.Point{}, draw.Over)
	return out, nil
}
