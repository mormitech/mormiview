package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Effects and color adjustments for the Effects menu and the chat panel's apply_effect tool (see mcp.go). An effect works on a part of the picture: a box
// (x, y, w, h), a polygon (pts) or a mask, or on all of it when none is given. The edge of the part can be made soft (feather). Every call is one step of
// the history.

type effectSpec struct {
	Op string `json:"op"`
	// the part
	X       int      `json:"x"`
	Y       int      `json:"y"`
	W       int      `json:"w"`
	H       int      `json:"h"`
	Pts     [][2]int `json:"pts"`
	Mask    string   `json:"mask"`
	Feather int      `json:"feather"` // pixels over which the edge of the part fades
	Again   bool     `json:"again"`   // rotate: this turn goes on from the last one of the same part (the page says so while the selection is kept)
	// the strength and the rest; each effect says which ones it reads
	Radius  float64  `json:"radius"` // blur, sharpen: pixels
	Size    int      `json:"size"`   // pixelize, censor: the side of a block in pixels
	Amount  *float64 `json:"amount"` // the strength: -100 to 100 for the adjustments, 0 to 100 for explosion and emboss, 0 to 500 for sharpen
	Angle   float64  `json:"angle"`  // emboss: where the light comes from; hue: the turn; gradient: the direction (0 is left to right); rotate: the turn of the part, clockwise; all in degrees
	CX      *float64 `json:"cx"`     // explosion: the middle of the burst (the middle of the part by default)
	CY      *float64 `json:"cy"`
	Color   string   `json:"color"`   // fill, gradient (the start), censor with a bar: #rrggbb or #rrggbbaa
	Color2  string   `json:"color2"`  // gradient: the end
	Kind    string   `json:"kind"`    // gradient: linear (the default) or radial
	Opacity *float64 `json:"opacity"` // fill, gradient: 0 to 1
	Bar     bool     `json:"bar"`     // censor: a solid bar instead of a mosaic
	Gamma   float64  `json:"gamma"`   // gamma, levels
	Black   *int     `json:"black"`   // levels: the value that becomes black (0 to 254)
	White   *int     `json:"white"`   // levels: the value that becomes white (1 to 255)
	Level   string   `json:"level"`   // highlight: light, medium or full (the default)
}

var effectOps = []string{"blur", "pixelize", "emboss", "explosion", "grayscale", "negative", "sepia", "censor", "fill", "gradient",
	"brightness", "contrast", "saturation", "hue", "temperature", "gamma", "levels", "auto_levels", "sharpen", "rotate"}

func (sp effectSpec) amount(def float64) float64 {
	if sp.Amount == nil {
		return def
	}
	return *sp.Amount
}

func clamp8(f float64) uint8 {
	if f <= 0 {
		return 0
	}
	if f >= 255 {
		return 255
	}
	return uint8(f + 0.5)
}

// effectShape is the part an effect works on. A box is cut to the picture; with nothing given it is the whole picture.
func (a *App) effectShape(sp effectSpec, bounds image.Rectangle) (*shape, error) {
	rs := retouchSpec{X: sp.X, Y: sp.Y, W: sp.W, H: sp.H, Pts: sp.Pts, Mask: sp.Mask}
	if rs.Mask == "" && len(rs.Pts) < 3 {
		if rs.W == 0 && rs.H == 0 {
			rs.X, rs.Y, rs.W, rs.H = bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy()
		} else if r := image.Rect(rs.X, rs.Y, rs.X+rs.W, rs.Y+rs.H).Intersect(bounds); !r.Empty() {
			rs.X, rs.Y, rs.W, rs.H = r.Min.X, r.Min.Y, r.Dx(), r.Dy()
		}
	}
	return a.shapeOf(rs, bounds)
}

// Effect applies one effect (spec is JSON, effectSpec) to the picture as shown (turned rot quarters). The UI shows the answer.
func (a *App) Effect(rot int, spec string) (ImageInfo, error) {
	var sp effectSpec
	if err := json.Unmarshal([]byte(spec), &sp); err != nil {
		return ImageInfo{}, fmt.Errorf("the effect is not understood: %v", err)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	sh, err := a.effectShape(sp, base.Bounds())
	if err != nil {
		return ImageInfo{}, err
	}
	var orig *image.NRGBA
	var total float64
	var key string
	if sp.Op == "rotate" { // turning again starts from the picture as it was before the first turn: nothing is turned twice (see rotate.go)
		key = fmt.Sprintf("part %d %v", rot, [...]any{sh.r, sp.Pts, sp.Mask != ""})
		orig, total = a.rotationBase(base, key, sp.Angle, sp.Again)
		base = cloneNRGBA(orig)
		sp.Angle = total
	}
	note, err := applyEffect(base, sh, sp)
	if err != nil {
		return ImageInfo{}, err
	}
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	if orig != nil {
		a.keepRotation(orig, key, total)
	}
	info := a.Image()
	info.Note = note
	return info, nil
}

// applyEffect changes base inside the shape. The effect is worked out on a copy of the box of the shape (and a margin for a soft edge) and put back
// where the shape is.
func applyEffect(base *image.NRGBA, sh *shape, sp effectSpec) (string, error) {
	if sp.Op == "highlight" { // (it works outside the part: see highlight.go)
		return highlight(base, sh, sp)
	}
	known := false
	for _, op := range effectOps {
		known = known || op == sp.Op
	}
	if !known {
		return "", fmt.Errorf("the effect is one of %v, not %q", effectOps, sp.Op)
	}
	bounds := base.Bounds()
	pad := max(0, min(sp.Feather, 200))
	r := sh.r.Inset(-pad).Intersect(bounds)
	w, h := r.Dx(), r.Dy()
	if w < 1 || h < 1 {
		return "", fmt.Errorf("the part is outside the image")
	}
	tile := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		copy(tile.Pix[y*tile.Stride:y*tile.Stride+4*w], base.Pix[(r.Min.Y+y-bounds.Min.Y)*base.Stride+4*(r.Min.X-bounds.Min.X):])
	}
	out := image.NewNRGBA(tile.Bounds())
	copy(out.Pix, tile.Pix)
	box := sh.r.Sub(r.Min)
	if sp.CX != nil { // the middle of an explosion is given in pixels of the picture, the tile starts at r.Min
		v := *sp.CX - float64(r.Min.X)
		sp.CX = &v
	}
	if sp.CY != nil {
		v := *sp.CY - float64(r.Min.Y)
		sp.CY = &v
	}
	if err := runEffect(out, box, sp); err != nil {
		return "", err
	}
	cov := make([]float32, w*h)
	for y := sh.r.Min.Y; y < sh.r.Max.Y; y++ {
		for x := sh.r.Min.X; x < sh.r.Max.X; x++ {
			if sh.m[(y-sh.r.Min.Y)*sh.r.Dx()+x-sh.r.Min.X] >= 128 {
				cov[(y-r.Min.Y)*w+x-r.Min.X] = 1
			}
		}
	}
	if pad > 0 {
		cov = boxBlur(cov, w, h, max(1, pad/2))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cov[y*w+x]
			if c <= 0.003 {
				continue
			}
			i := (r.Min.Y+y-bounds.Min.Y)*base.Stride + 4*(r.Min.X+x-bounds.Min.X)
			j := y*out.Stride + 4*x
			for k := 0; k < 4; k++ {
				base.Pix[i+k] = lerp8(base.Pix[i+k], out.Pix[j+k], c)
			}
		}
	}
	return fmt.Sprintf("%s applied to %d × %d pixels at %d,%d", sp.Op, sh.r.Dx(), sh.r.Dy(), sh.r.Min.X, sh.r.Min.Y), nil
}

// runEffect changes img (the tile) in place; box is the box of the shape in the tile.
func runEffect(img *image.NRGBA, box image.Rectangle, sp effectSpec) error {
	switch sp.Op {
	case "grayscale":
		each(img, func(r, g, b float64) (float64, float64, float64) { l := lum(r, g, b); return l, l, l })
	case "negative":
		each(img, func(r, g, b float64) (float64, float64, float64) { return 255 - r, 255 - g, 255 - b })
	case "sepia":
		each(img, func(r, g, b float64) (float64, float64, float64) {
			return 0.393*r + 0.769*g + 0.189*b, 0.349*r + 0.686*g + 0.168*b, 0.272*r + 0.534*g + 0.131*b
		})
	case "brightness":
		d := sp.amount(20) * 2.55
		each(img, func(r, g, b float64) (float64, float64, float64) { return r + d, g + d, b + d })
	case "contrast":
		c := math.Max(-254, math.Min(254, sp.amount(20)*2.55))
		f := 259 * (c + 255) / (255 * (259 - c))
		each(img, func(r, g, b float64) (float64, float64, float64) {
			return f*(r-128) + 128, f*(g-128) + 128, f*(b-128) + 128
		})
	case "saturation":
		s := 1 + sp.amount(30)/100
		each(img, func(r, g, b float64) (float64, float64, float64) {
			l := lum(r, g, b)
			return l + (r-l)*s, l + (g-l)*s, l + (b-l)*s
		})
	case "hue":
		a := sp.Angle * math.Pi / 180
		c, s := math.Cos(a), math.Sin(a)
		// a turn of the color around the gray axis (the matrix of the YIQ hue rotation)
		m := [9]float64{
			0.299 + 0.701*c + 0.168*s, 0.587 - 0.587*c + 0.330*s, 0.114 - 0.114*c - 0.497*s,
			0.299 - 0.299*c - 0.328*s, 0.587 + 0.413*c + 0.035*s, 0.114 - 0.114*c + 0.292*s,
			0.299 - 0.300*c + 1.250*s, 0.587 - 0.588*c - 1.050*s, 0.114 + 0.886*c - 0.203*s,
		}
		each(img, func(r, g, b float64) (float64, float64, float64) {
			return m[0]*r + m[1]*g + m[2]*b, m[3]*r + m[4]*g + m[5]*b, m[6]*r + m[7]*g + m[8]*b
		})
	case "temperature": // above 0 warmer (more red, less blue), below 0 cooler
		d := sp.amount(20) * 0.4
		each(img, func(r, g, b float64) (float64, float64, float64) { return r + d, g + d*0.1, b - d })
	case "gamma":
		g := sp.Gamma
		if g <= 0 {
			g = 1.2
		}
		lut := levelsLUT(0, 255, g)
		each(img, func(r, gg, b float64) (float64, float64, float64) {
			return lut[clamp8(r)], lut[clamp8(gg)], lut[clamp8(b)]
		})
	case "levels":
		black, white := 0, 255
		if sp.Black != nil {
			black = *sp.Black
		}
		if sp.White != nil {
			white = *sp.White
		}
		if black < 0 || white > 255 || black >= white {
			return fmt.Errorf("levels needs 0 <= black < white <= 255")
		}
		g := sp.Gamma
		if g <= 0 {
			g = 1
		}
		lut := levelsLUT(black, white, g)
		each(img, func(r, gg, b float64) (float64, float64, float64) {
			return lut[clamp8(r)], lut[clamp8(gg)], lut[clamp8(b)]
		})
	case "auto_levels":
		var hist [256]int
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i+3] > 0 {
				hist[clamp8(lum(float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2])))]++
				n++
			}
		}
		if n == 0 {
			return nil
		}
		lo, hi, acc := 0, 255, 0
		for lo = 0; lo < 255 && acc+hist[lo] <= n/100; lo++ { // the darkest and the brightest 1% are let go
			acc += hist[lo]
		}
		acc = 0
		for hi = 255; hi > 0 && acc+hist[hi] <= n/100; hi-- {
			acc += hist[hi]
		}
		if hi-lo < 8 {
			return nil
		}
		lut := levelsLUT(lo, hi, 1)
		each(img, func(r, g, b float64) (float64, float64, float64) {
			return lut[clamp8(r)], lut[clamp8(g)], lut[clamp8(b)]
		})
	case "fill":
		c, err := parseColorA(sp.Color, "#000000")
		if err != nil {
			return err
		}
		op := 1.0
		if sp.Opacity != nil {
			op = math.Max(0, math.Min(1, *sp.Opacity))
		}
		for i := 0; i < len(img.Pix); i += 4 { // the color replaces what is there (a transparent color makes a hole)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = lerp8(img.Pix[i], c.R, float32(op)), lerp8(img.Pix[i+1], c.G, float32(op)), lerp8(img.Pix[i+2], c.B, float32(op))
			img.Pix[i+3] = lerp8(img.Pix[i+3], c.A, float32(op))
		}
	case "gradient":
		return gradient(img, box, sp)
	case "blur":
		r := sp.Radius
		if r <= 0 {
			r = 6
		}
		blur(img, r)
	case "sharpen":
		radius := sp.Radius
		if radius <= 0 {
			radius = 2
		}
		k := sp.amount(100) / 100
		soft := image.NewNRGBA(img.Bounds())
		copy(soft.Pix, img.Pix)
		blur(soft, radius)
		for i := 0; i < len(img.Pix); i += 4 {
			for c := 0; c < 3; c++ {
				v, s := float64(img.Pix[i+c]), float64(soft.Pix[i+c])
				img.Pix[i+c] = clamp8(v + k*(v-s))
			}
		}
	case "pixelize":
		size := sp.Size
		if size < 2 {
			size = 12
		}
		mosaic(img, box, min(size, 400))
	case "censor":
		if sp.Bar {
			c, err := parseColorA(sp.Color, "#000000")
			if err != nil {
				return err
			}
			for y := box.Min.Y; y < box.Max.Y; y++ {
				for x := box.Min.X; x < box.Max.X; x++ {
					if image.Pt(x, y).In(img.Bounds()) {
						img.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, 255})
					}
				}
			}
			return nil
		}
		size := sp.Size
		if size < 2 { // big blocks, so that nothing can be read from it
			size = max(8, min(box.Dx(), box.Dy())/6)
		}
		mosaic(img, box, min(size, 400))
	case "rotate":
		rotatePart(img, box, sp.Angle)
	case "emboss":
		emboss(img, sp.Angle, sp.amount(100)/100)
	case "explosion":
		cx, cy := float64(box.Min.X+box.Max.X)/2, float64(box.Min.Y+box.Max.Y)/2
		explode(img, box, sp, cx, cy)
	}
	return nil
}

func lum(r, g, b float64) float64 { return 0.299*r + 0.587*g + 0.114*b }

// each changes the color of every pixel that is not fully transparent; the alpha stays.
func each(img *image.NRGBA, f func(r, g, b float64) (float64, float64, float64)) {
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 0 {
			continue
		}
		r, g, b := f(float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2]))
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = clamp8(r), clamp8(g), clamp8(b)
	}
}

func levelsLUT(black, white int, gamma float64) [256]float64 {
	var lut [256]float64
	for v := 0; v < 256; v++ {
		t := math.Max(0, math.Min(1, float64(v-black)/float64(white-black)))
		lut[v] = 255 * math.Pow(t, 1/gamma)
	}
	return lut
}

// parseColorA reads #rrggbb or #rrggbbaa ("" is def).
func parseColorA(s, def string) (color.NRGBA, error) {
	if s == "" {
		s = def
	}
	if len(s) == 7 {
		return parseRGB(s)
	}
	return parseRGBA(s)
}

// premultiplied planes of the picture (r, g, b, a as 0 to 255 floats), so that blurring does not bleed the color of transparent pixels in.
func planes(img *image.NRGBA) [4][]float32 {
	n := img.Bounds().Dx() * img.Bounds().Dy()
	var p [4][]float32
	for c := range p {
		p[c] = make([]float32, n)
	}
	w := img.Bounds().Dx()
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + 4*x
			a := float32(img.Pix[i+3])
			p[0][y*w+x], p[1][y*w+x], p[2][y*w+x], p[3][y*w+x] = float32(img.Pix[i])*a/255, float32(img.Pix[i+1])*a/255, float32(img.Pix[i+2])*a/255, a
		}
	}
	return p
}

func fromPlanes(img *image.NRGBA, p [4][]float32) {
	w := img.Bounds().Dx()
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + 4*x
			a := p[3][y*w+x]
			if a < 0.5 {
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 0, 0, 0
				continue
			}
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = clamp8(float64(p[0][y*w+x]*255/a)), clamp8(float64(p[1][y*w+x]*255/a)), clamp8(float64(p[2][y*w+x]*255/a)), clamp8(float64(a))
		}
	}
}

// blur is a smooth blur of about the radius r pixels (box blurs one after the other come close to a Gaussian one).
func blur(img *image.NRGBA, r float64) {
	k := int(math.Ceil(math.Min(r, 300) / 2))
	if k < 1 {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	p := planes(img)
	for c := range p {
		p[c] = boxBlur(boxBlur(p[c], w, h, k), w, h, k)
	}
	fromPlanes(img, p)
}

// mosaic makes every block of size × size pixels one color (the average). The blocks start at the top left corner of the box.
func mosaic(img *image.NRGBA, box image.Rectangle, size int) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	p := planes(img)
	floor := func(v int) int {
		if v < 0 {
			return -((-v + size - 1) / size)
		}
		return v / size
	}
	type sum struct{ c [4]float64 }
	blocks := map[[2]int]*sum{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			k := [2]int{floor(x - box.Min.X), floor(y - box.Min.Y)}
			s := blocks[k]
			if s == nil {
				s = &sum{}
				blocks[k] = s
			}
			for c := 0; c < 4; c++ {
				s.c[c] += float64(p[c][y*w+x])
			}
		}
	}
	count := map[[2]int]float64{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			count[[2]int{floor(x - box.Min.X), floor(y - box.Min.Y)}]++
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			k := [2]int{floor(x - box.Min.X), floor(y - box.Min.Y)}
			for c := 0; c < 4; c++ {
				p[c][y*w+x] = float32(blocks[k].c[c] / count[k])
			}
		}
	}
	fromPlanes(img, p)
}

// emboss makes a gray relief: what is lighter on the side the light comes from than on the other side stands out.
func emboss(img *image.NRGBA, angle, strength float64) {
	if angle == 0 {
		angle = 135
	}
	a := angle * math.Pi / 180
	dx, dy := int(math.Round(math.Cos(a))), int(math.Round(math.Sin(a)))
	if dx == 0 && dy == 0 {
		dx = 1
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	l := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + 4*x
			l[y*w+x] = lum(float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2]))
		}
	}
	at := func(x, y int) float64 { return l[min(max(y, 0), h-1)*w+min(max(x, 0), w-1)] }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + 4*x
			v := clamp8(128 + 2*strength*(at(x-dx, y-dy)-at(x+dx, y+dy)))
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = v, v, v
		}
	}
}

// explode is a burst from a middle: every pixel is smeared along the line to the middle (a zoom blur), and the middle glows.
func explode(img *image.NRGBA, box image.Rectangle, sp effectSpec, cx, cy float64) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if sp.CX != nil {
		cx = *sp.CX
	}
	if sp.CY != nil {
		cy = *sp.CY
	}
	s := math.Max(0, math.Min(1, sp.amount(60)/100))
	src := planes(img)
	out := [4][]float32{}
	for c := range out {
		out[c] = make([]float32, w*h)
	}
	reach := math.Hypot(math.Max(cx, float64(w)-cx), math.Max(cy, float64(h)-cy))
	const steps = 24
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var acc [4]float64
			for k := 0; k < steps; k++ {
				t := float64(k) / (steps - 1)
				sx := int(math.Round(cx + (float64(x)-cx)*(1-s*t)))
				sy := int(math.Round(cy + (float64(y)-cy)*(1-s*t)))
				sx, sy = min(max(sx, 0), w-1), min(max(sy, 0), h-1)
				for c := 0; c < 4; c++ {
					acc[c] += float64(src[c][sy*w+sx])
				}
			}
			glow := 1 + 0.9*s*math.Pow(math.Max(0, 1-math.Hypot(float64(x)-cx, float64(y)-cy)/reach), 2)
			for c := 0; c < 3; c++ {
				out[c][y*w+x] = float32(math.Min(acc[c]/steps*glow, acc[3]/steps)) // (not brighter than opaque)
			}
			out[3][y*w+x] = float32(acc[3] / steps)
		}
	}
	fromPlanes(img, out)
}

// gradient paints a linear or radial gradient from color to color2 over the box of the shape.
func gradient(img *image.NRGBA, box image.Rectangle, sp effectSpec) error {
	c1, err := parseColorA(sp.Color, "#000000")
	if err != nil {
		return err
	}
	c2, err := parseColorA(sp.Color2, "#FFFFFF")
	if err != nil {
		return err
	}
	op := 1.0
	if sp.Opacity != nil {
		op = math.Max(0, math.Min(1, *sp.Opacity))
	}
	cx, cy := float64(box.Min.X+box.Max.X)/2, float64(box.Min.Y+box.Max.Y)/2
	hw, hh := float64(box.Dx())/2, float64(box.Dy())/2
	a := sp.Angle * math.Pi / 180
	ux, uy := math.Cos(a), math.Sin(a)
	span := math.Abs(ux)*hw + math.Abs(uy)*hh // half the length of the box along the direction
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			px, py := float64(x)+0.5-cx, float64(y)+0.5-cy
			var t float64
			if sp.Kind == "radial" {
				t = math.Hypot(px/math.Max(hw, 1), py/math.Max(hh, 1)) / math.Sqrt2
			} else {
				t = (px*ux+py*uy)/math.Max(span, 1)/2 + 0.5
			}
			t = math.Max(0, math.Min(1, t))
			i := y*img.Stride + 4*x
			f := float32(op)
			img.Pix[i] = lerp8(img.Pix[i], lerp8(c1.R, c2.R, float32(t)), f)
			img.Pix[i+1] = lerp8(img.Pix[i+1], lerp8(c1.G, c2.G, float32(t)), f)
			img.Pix[i+2] = lerp8(img.Pix[i+2], lerp8(c1.B, c2.B, float32(t)), f)
			img.Pix[i+3] = lerp8(img.Pix[i+3], lerp8(c1.A, c2.A, float32(t)), f)
		}
	}
	return nil
}
