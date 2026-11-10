package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
)

// Lines, rectangles, ellipses and freehand paths put into the picture by Go, at once: the chat panel's draw tools (see mcp.go). They use no pointer and
// no overlay; they make the same shapes as the tools of the window (see drawShape in main.js), with the same round pen, dashes and fills.

type shapeSpec struct {
	Kind  string   `json:"kind"` // line, rect, ellipse, path
	X0    int      `json:"x0"`   // line, rect, ellipse: the two corners (or ends), both pixels of the picture
	Y0    int      `json:"y0"`
	X1    int      `json:"x1"`
	Y1    int      `json:"y1"`
	Pts   [][2]int `json:"pts"`   // path: the points
	Color string   `json:"color"` // #rrggbb, the outline
	Bg    string   `json:"bg"`    // #rrggbb, what fills a rect or an ellipse when Fill is "both"
	Width int      `json:"width"`
	Style string   `json:"style"` // solid, dashed, dotted
	Fill  string   `json:"fill"`  // outline, both, fill
	// Radius rounds the corners of a rect or a callout; Opacity (0 to 1) makes the whole shape see-through; Shadow puts a soft shadow under it;
	// TX, TY are the point a callout's tail points at.
	Radius  int      `json:"radius"`
	Opacity *float64 `json:"opacity"`
	Shadow  *struct {
		DX    int    `json:"dx"`
		DY    int    `json:"dy"`
		Blur  int    `json:"blur"`
		Color string `json:"color"` // #rrggbb (black by default); the shadow is half see-through
	} `json:"shadow"`
	TX int `json:"tx"`
	TY int `json:"ty"`
}

// pen puts down the tip: round (a circle as wide as the width, like the pencil) or square (the corners of a rectangle).
type pen struct {
	img    *image.NRGBA
	c      color.NRGBA
	width  int
	square bool
}

func (p *pen) put(x, y int) {
	w, bb := p.width, p.img.Bounds()
	lim := (float64(w)/2 - 0.25) * (float64(w)/2 - 0.25) // the same round tip as the one the UI shows (see stampOffsets in main.js)
	mid := float64(w-1) / 2
	off := (w - 1) / 2
	if p.square {
		off = w / 2
	}
	for j := 0; j < w; j++ {
		for i := 0; i < w; i++ {
			if !p.square && (float64(i)-mid)*(float64(i)-mid)+(float64(j)-mid)*(float64(j)-mid) > lim {
				continue
			}
			if q := image.Pt(x+i-off, y+j-off); q.In(bb) {
				p.img.SetNRGBA(q.X, q.Y, p.c)
			}
		}
	}
}

// walk gives the pixels from (x0, y0) to (x1, y1), both included (Bresenham).
func walk(x0, y0, x1, y1 int, f func(x, y int)) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		f(x0, y0)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// path draws through the points with the tip; dash is {on, off} in pixels along the path (nil: a solid line), an "on" of 0 is a dot at the start of each period.
func (p *pen) path(pts [][2]int, dash []float64) {
	if len(pts) == 0 {
		return
	}
	if len(pts) == 1 {
		p.put(pts[0][0], pts[0][1])
		return
	}
	var line [][2]int // every pixel of the way, a joint once
	for i := 1; i < len(pts); i++ {
		walk(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], func(x, y int) {
			if i > 1 && len(line) > 0 && line[len(line)-1] == [2]int{x, y} {
				return
			}
			line = append(line, [2]int{x, y})
		})
	}
	for d, q := range line {
		if dash == nil {
			p.put(q[0], q[1])
			continue
		}
		ph := math.Mod(float64(d), dash[0]+dash[1])
		if (dash[0] == 0 && ph < 1) || (dash[0] > 0 && ph < dash[0]) {
			p.put(q[0], q[1])
		}
	}
}

func dashPattern(w int, style string) []float64 { // the same as dashes() in main.js
	switch style {
	case "dashed":
		return []float64{math.Max(4, float64(w*3)), math.Max(3, float64(w*2))}
	case "dotted":
		return []float64{0, math.Max(3, float64(w*2))}
	}
	return nil
}

func parseRGB(s string) (color.NRGBA, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, fmt.Errorf("color %q is not #rrggbb", s)
	}
	return color.NRGBA{r, g, b, 255}, nil
}

// inEllipse says whether the middle of the pixel is inside the ellipse with the middle cx, cy and the radii rx, ry.
func inEllipse(x, y int, cx, cy, rx, ry float64) bool {
	if rx <= 0 || ry <= 0 {
		return false
	}
	dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
	return dx*dx+dy*dy <= 1
}

// DrawShape puts a shape into the picture of this window (as shown, turned rot quarters) and makes one step of the history. spec is JSON (shapeSpec).
func (a *App) DrawShape(rot int, spec string) (ImageInfo, error) {
	var s shapeSpec
	if err := json.Unmarshal([]byte(spec), &s); err != nil {
		return ImageInfo{}, fmt.Errorf("the shape is not understood: %v", err)
	}
	c1, err := parseRGB(s.Color)
	if err != nil {
		return ImageInfo{}, err
	}
	c2 := c1
	if s.Bg != "" {
		if c2, err = parseRGB(s.Bg); err != nil {
			return ImageInfo{}, err
		}
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	bb := base.Bounds()
	w := max(1, min(s.Width, 200))
	dash := dashPattern(w, s.Style)
	opacity := 1.0
	if s.Opacity != nil {
		opacity = math.Max(0, math.Min(1, *s.Opacity))
	}
	layered := opacity < 1 || s.Shadow != nil // such a shape is drawn on a layer of its own, and the layer is put over the picture
	dst := base
	if layered {
		dst = image.NewNRGBA(bb)
	}
	switch s.Kind {
	case "polygon":
		if len(s.Pts) < 3 {
			return ImageInfo{}, fmt.Errorf("a polygon needs three points or more")
		}
		if s.Fill == "fill" || s.Fill == "both" {
			fc := c1
			if s.Fill == "both" {
				fc = c2
			}
			fillPolygon(dst, s.Pts, fc)
		}
		if s.Fill != "fill" {
			closed := append(append([][2]int{}, s.Pts...), s.Pts[0])
			(&pen{img: dst, c: c1, width: w}).path(closed, dash)
		}
	case "callout":
		drawCallout(dst, s, c1, c2, w)
	case "path":
		if len(s.Pts) < 2 {
			return ImageInfo{}, fmt.Errorf("a path needs two points or more")
		}
		(&pen{img: dst, c: c1, width: w}).path(s.Pts, nil)
	case "line":
		(&pen{img: dst, c: c1, width: w}).path([][2]int{{s.X0, s.Y0}, {s.X1, s.Y1}}, dash)
	case "rect", "ellipse":
		xa, ya := min(s.X0, s.X1), min(s.Y0, s.Y1)
		W, H := abs(s.X1-s.X0)+1, abs(s.Y1-s.Y0)+1
		round := s.Kind == "ellipse"
		cx, cy, rx, ry := float64(xa)+float64(W)/2, float64(ya)+float64(H)/2, float64(W)/2, float64(H)/2
		area := func(c color.NRGBA, in func(x, y int) bool) {
			for y := max(ya, bb.Min.Y); y < min(ya+H, bb.Max.Y); y++ {
				for x := max(xa, bb.Min.X); x < min(xa+W, bb.Max.X); x++ {
					if in(x, y) {
						dst.SetNRGBA(x, y, c)
					}
				}
			}
		}
		rad := min(max(s.Radius, 0), min(W, H)/2)
		whole := func(x, y int) bool {
			if round {
				return inEllipse(x, y, cx, cy, rx, ry)
			}
			return rad == 0 || inRoundRect(x, y, xa, ya, W, H, rad)
		}
		switch {
		case s.Fill == "fill" || W <= w || H <= w: // (a shape that is all outline is a filled one)
			area(c1, whole)
		default:
			if s.Fill == "both" {
				area(c2, whole)
			}
			switch {
			case !round && rad > 0 && dash == nil: // a rounded outline: the rounded box without the one that is smaller by the width
				area(c1, func(x, y int) bool {
					return whole(x, y) && !inRoundRect(x, y, xa+w, ya+w, W-2*w, H-2*w, max(0, rad-w))
				})
			case !round && dash == nil: // the outline is inside the box
				ring := func(x, y int) bool { return x < xa+w || x >= xa+W-w || y < ya+w || y >= ya+H-w }
				area(c1, ring)
			case !round:
				o := w / 2
				p := &pen{img: dst, c: c1, width: w, square: s.Style == "dashed"}
				x0, y0, x1, y1 := xa+o, ya+o, xa+W-w+o, ya+H-w+o
				p.path([][2]int{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}, {x0, y0}}, dash)
			case dash == nil:
				area(c1, func(x, y int) bool {
					return inEllipse(x, y, cx, cy, rx, ry) && !inEllipse(x, y, cx, cy, rx-float64(w), ry-float64(w))
				})
			default: // the middle of the outline, as many points as the ellipse is long
				mrx, mry := rx-float64(w)/2, ry-float64(w)/2
				n := max(16, int(2*math.Pi*math.Max(mrx, mry)))
				pts := make([][2]int, 0, n+1)
				for i := 0; i <= n; i++ {
					t := 2 * math.Pi * float64(i) / float64(n)
					pts = append(pts, [2]int{int(math.Floor(cx + mrx*math.Cos(t))), int(math.Floor(cy + mry*math.Sin(t)))})
				}
				(&pen{img: dst, c: c1, width: w, square: s.Style == "dashed"}).path(pts, dash)
			}
		}
	default:
		return ImageInfo{}, fmt.Errorf("a shape is a line, a rect, an ellipse, a path, a polygon or a callout, not %q", s.Kind)
	}
	if layered {
		if err := layerOver(base, dst, s, opacity); err != nil {
			return ImageInfo{}, err
		}
	}
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

// inRoundRect says whether the middle of the pixel is inside the box xa, ya, W × H with corners of the radius r.
func inRoundRect(x, y, xa, ya, W, H, r int) bool {
	if W <= 0 || H <= 0 || x < xa || y < ya || x >= xa+W || y >= ya+H {
		return false
	}
	r = min(r, min(W, H)/2)
	if r <= 0 {
		return true
	}
	cx, cy := x, y // the corner circle the pixel is near, if any
	switch {
	case x < xa+r:
		cx = xa + r
	case x >= xa+W-r:
		cx = xa + W - r - 1
	}
	switch {
	case y < ya+r:
		cy = ya + r
	case y >= ya+H-r:
		cy = ya + H - r - 1
	}
	if cx == x && cy == y {
		return true
	}
	dx, dy := float64(x-cx), float64(y-cy)
	return dx*dx+dy*dy <= float64(r)*float64(r)
}

// fillPolygon fills the inside of the polygon (the pixels whose middle is inside), the part that is in the picture.
func fillPolygon(img *image.NRGBA, pts [][2]int, c color.NRGBA) {
	bb := img.Bounds()
	n := len(pts)
	minY, maxY := math.MaxInt, math.MinInt
	for _, p := range pts {
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	for y := max(minY, bb.Min.Y); y <= min(maxY, bb.Max.Y-1); y++ {
		yc := float64(y) + 0.5
		var xs []float64
		for i := 0; i < n; i++ {
			a, b := pts[i], pts[(i+1)%n]
			if (float64(a[1]) <= yc) != (float64(b[1]) <= yc) {
				xs = append(xs, float64(a[0])+(yc-float64(a[1]))*float64(b[0]-a[0])/float64(b[1]-a[1]))
			}
		}
		sort.Float64s(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			for x := max(int(math.Ceil(xs[i]-0.5)), bb.Min.X); float64(x)+0.5 < xs[i+1] && x < bb.Max.X; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// drawCallout draws a speech bubble: a rounded box (x0, y0 to x1, y1) with a tail that points at (tx, ty), filled with bg and outlined with color.
func drawCallout(dst *image.NRGBA, s shapeSpec, outline, fill color.NRGBA, w int) {
	bb := dst.Bounds()
	xa, ya := min(s.X0, s.X1), min(s.Y0, s.Y1)
	W, H := abs(s.X1-s.X0)+1, abs(s.Y1-s.Y0)+1
	rad := s.Radius
	if rad <= 0 {
		rad = 12
	}
	rad = min(rad, min(W, H)/2)
	xb, yb := xa+W-1, ya+H-1
	var tail [][2]int // the triangle of the tail, its base a little inside the box so that nothing shows between
	half := min(18, max(6, min(W, H)/6))
	clampX := func(v int) int { return max(xa+rad+half, min(v, xb-rad-half)) }
	clampY := func(v int) int { return max(ya+rad+half, min(v, yb-rad-half)) }
	switch {
	case s.TY > yb:
		cx := clampX(s.TX)
		tail = [][2]int{{cx - half, yb - 2}, {cx + half, yb - 2}, {s.TX, s.TY}}
	case s.TY < ya:
		cx := clampX(s.TX)
		tail = [][2]int{{cx - half, ya + 2}, {cx + half, ya + 2}, {s.TX, s.TY}}
	case s.TX < xa:
		cy := clampY(s.TY)
		tail = [][2]int{{xa + 2, cy - half}, {xa + 2, cy + half}, {s.TX, s.TY}}
	case s.TX > xb:
		cy := clampY(s.TY)
		tail = [][2]int{{xb - 2, cy - half}, {xb - 2, cy + half}, {s.TX, s.TY}}
	}
	cov := &maskData{w: bb.Dx(), h: bb.Dy(), bits: make([]uint8, bb.Dx()*bb.Dy())}
	for y := max(ya, bb.Min.Y); y <= min(yb, bb.Max.Y-1); y++ {
		for x := max(xa, bb.Min.X); x <= min(xb, bb.Max.X-1); x++ {
			if inRoundRect(x, y, xa, ya, W, H, rad) {
				cov.bits[y*cov.w+x] = 255
			}
		}
	}
	if tail != nil {
		tmp := image.NewNRGBA(bb)
		fillPolygon(tmp, tail, color.NRGBA{A: 255})
		for i := 0; i < len(cov.bits); i++ {
			if tmp.Pix[4*i+3] != 0 {
				cov.bits[i] = 255
			}
		}
	}
	inner := &maskData{w: cov.w, h: cov.h, bits: append([]uint8(nil), cov.bits...)}
	inner.morph(-w)
	for i, v := range cov.bits {
		if v == 0 {
			continue
		}
		c := outline
		if inner.bits[i] != 0 {
			if s.Fill == "outline" {
				continue
			}
			c = fill
		}
		dst.SetNRGBA(i%cov.w, i/cov.w, c)
	}
}

// layerOver puts the layer over base: first its shadow (if it has one), then the layer itself, as see-through as opacity says.
func layerOver(base, layer *image.NRGBA, s shapeSpec, opacity float64) error {
	bb := base.Bounds()
	if sh := s.Shadow; sh != nil {
		c := color.NRGBA{A: 255}
		if sh.Color != "" {
			var err error
			if c, err = parseRGB(sh.Color); err != nil {
				return err
			}
		}
		dx, dy := sh.DX, sh.DY
		if dx == 0 && dy == 0 {
			dx, dy = 3, 3
		}
		blurPx := sh.Blur
		if blurPx <= 0 {
			blurPx = 4
		}
		w, h := bb.Dx(), bb.Dy()
		a := make([]float32, w*h) // the shadow is the shape's outline moved over
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if v := layer.Pix[y*layer.Stride+4*x+3]; v != 0 {
					if tx, ty := x+dx, y+dy; tx >= 0 && ty >= 0 && tx < w && ty < h {
						a[ty*w+tx] = float32(v)
					}
				}
			}
		}
		a = boxBlur(a, w, h, max(1, blurPx/2))
		shadow := image.NewNRGBA(bb)
		for i, v := range a {
			if v >= 1 {
				shadow.Pix[4*i], shadow.Pix[4*i+1], shadow.Pix[4*i+2], shadow.Pix[4*i+3] = c.R, c.G, c.B, clamp8(float64(v)*0.5*opacity)
			}
		}
		draw.Draw(base, bb, shadow, bb.Min, draw.Over)
	}
	if opacity < 1 {
		for i := 3; i < len(layer.Pix); i += 4 {
			layer.Pix[i] = clamp8(float64(layer.Pix[i]) * opacity)
		}
	}
	draw.Draw(base, bb, layer, bb.Min, draw.Over)
	return nil
}
