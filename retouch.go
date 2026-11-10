package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"sort"
)

// Retouching for the chat panel's tools (see mcp.go): the model looks at the picture and decides where to take pixels from and what to blend; these functions
// do the pixel work. clone_region copies a shape of the picture somewhere else (with soft edges), blend_edges hides the seam of such a patch, and
// fill_region fills a gap from what is around it. Every call is one step of the history, and none of them moves a pointer.

type retouchSpec struct {
	Op       string   `json:"op"` // clone, blend, fill
	X        int      `json:"x"`
	Y        int      `json:"y"`
	W        int      `json:"w"`
	H        int      `json:"h"`
	Pts      [][2]int `json:"pts"` // the shape, when it is not a box
	TX       int      `json:"tx"`  // clone: where the top left corner of the box of the shape lands
	TY       int      `json:"ty"`
	FlipH    bool     `json:"flip_h"`
	FlipV    bool     `json:"flip_v"`
	Rotate   int      `json:"rotate"`  // 0, 90, 180, 270 (clockwise)
	Feather  int      `json:"feather"` // clone: pixels of soft edge
	Opacity  *float64 `json:"opacity"`
	Method   string   `json:"method"` // blend: smooth, color_match, poisson; fill: patch (the default), smooth, nearest, average, solid
	Width    int      `json:"width"`  // blend: pixels of the edge that are worked on
	Strength *float64 `json:"strength"`
	Patch    int      `json:"patch"` // fill: size of the patches it copies (odd, 9 by default)
	Color    string   `json:"color"` // fill, method solid: #rrggbb
	Grain    bool     `json:"grain"` // fill: add noise like the surroundings have, so that a smooth fill does not look flat
	Ring     int      `json:"ring"`  // fill, methods average and grain: how many pixels around the shape are looked at (6 by default)

	Mask       string   `json:"mask"`        // the shape is this mask (see masks.go) instead of a box or a polygon
	SourceMask string   `json:"source_mask"` // fill: copy only from inside this mask
	AvoidMask  string   `json:"avoid_mask"`  // fill: never copy from inside this mask
	Scale      *float64 `json:"scale"`       // clone: 1 is the size it has
	Angle      float64  `json:"angle"`       // clone: degrees clockwise, any
	Blend      string   `json:"blend"`       // clone: "" (put it over, with the soft edge) or "seamless" (the patch takes the light and color of where it lands, like a healing brush)
}

// shape is a part of the picture: its box and, for each pixel of the box, how much of it is inside (0 or 255).
type shape struct {
	r image.Rectangle
	m []uint8
}

func (s *shape) in(x, y int) bool { // x, y of the picture
	p := image.Pt(x, y)
	return p.In(s.r) && s.m[(y-s.r.Min.Y)*s.r.Dx()+x-s.r.Min.X] >= 128
}

// newShape reads the shape of a spec: a polygon (the pixels whose middle is inside it) or a box.
func newShape(sp retouchSpec, bounds image.Rectangle) (*shape, error) {
	if len(sp.Pts) >= 3 {
		minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, math.MinInt, math.MinInt
		for _, p := range sp.Pts {
			minX, maxX, minY, maxY = min(minX, p[0]), max(maxX, p[0]), min(minY, p[1]), max(maxY, p[1])
		}
		r := image.Rect(minX, minY, maxX+1, maxY+1)
		if !r.In(bounds) {
			return nil, fmt.Errorf("the shape (box %d,%d to %d,%d) is not inside the image (%d × %d)", r.Min.X, r.Min.Y, r.Max.X-1, r.Max.Y-1, bounds.Dx(), bounds.Dy())
		}
		s := &shape{r: r, m: make([]uint8, r.Dx()*r.Dy())}
		n := len(sp.Pts)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			yc := float64(y) + 0.5
			var xs []float64
			for i := 0; i < n; i++ {
				a, b := sp.Pts[i], sp.Pts[(i+1)%n]
				if (float64(a[1]) <= yc) != (float64(b[1]) <= yc) {
					xs = append(xs, float64(a[0])+(yc-float64(a[1]))*float64(b[0]-a[0])/float64(b[1]-a[1]))
				}
			}
			sort.Float64s(xs)
			for i := 0; i+1 < len(xs); i += 2 {
				for x := int(math.Ceil(xs[i] - 0.5)); float64(x)+0.5 < xs[i+1]; x++ {
					if x >= r.Min.X && x < r.Max.X {
						s.m[(y-r.Min.Y)*r.Dx()+x-r.Min.X] = 255
					}
				}
			}
		}
		return s, nil
	}
	r := image.Rect(sp.X, sp.Y, sp.X+sp.W, sp.Y+sp.H)
	if sp.W < 1 || sp.H < 1 || !r.In(bounds) {
		return nil, fmt.Errorf("the part %d,%d %d × %d is not inside the image (%d × %d); give x, y, w, h or a polygon of three points or more", sp.X, sp.Y, sp.W, sp.H, bounds.Dx(), bounds.Dy())
	}
	s := &shape{r: r, m: make([]uint8, r.Dx()*r.Dy())}
	for i := range s.m {
		s.m[i] = 255
	}
	return s, nil
}

// boxBlur blurs the w × h plane with a box of the radius k, twice (a soft, smooth falloff).
func boxBlur(p []float32, w, h, k int) []float32 {
	if k < 1 {
		return p
	}
	for pass := 0; pass < 2; pass++ {
		tmp := make([]float32, len(p))
		for y := 0; y < h; y++ { // horizontal, with a running sum (the edge counts as the same value going on)
			var sum float32
			row := p[y*w : (y+1)*w]
			for i := -k; i <= k; i++ {
				sum += row[min(max(i, 0), w-1)]
			}
			for x := 0; x < w; x++ {
				tmp[y*w+x] = sum / float32(2*k+1)
				sum += row[min(x+k+1, w-1)] - row[max(x-k, 0)]
			}
		}
		out := make([]float32, len(p))
		for x := 0; x < w; x++ {
			var sum float32
			for i := -k; i <= k; i++ {
				sum += tmp[min(max(i, 0), h-1)*w+x]
			}
			for y := 0; y < h; y++ {
				out[y*w+x] = sum / float32(2*k+1)
				sum += tmp[min(y+k+1, h-1)*w+x] - tmp[max(y-k, 0)*w+x]
			}
		}
		p = out
	}
	return p
}

// softMask is the mask padded by pad pixels on every side, as 0 to 1, with the edge made soft over about f pixels.
func softMask(m []uint8, w, h, pad, f int) ([]float32, int, int) {
	pw, ph := w+2*pad, h+2*pad
	out := make([]float32, pw*ph)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if m[y*w+x] >= 128 {
				out[(y+pad)*pw+x+pad] = 1
			}
		}
	}
	return boxBlur(out, pw, ph, f/2), pw, ph
}

// transformGrid flips and turns (clockwise, in quarters) a w × h grid. It returns the new grid and its size.
func transformGrid[T any](src []T, w, h int, flipH, flipV bool, quarters int) ([]T, int, int) {
	cur, cw, ch := src, w, h
	if flipH || flipV {
		n := make([]T, len(cur))
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				sx, sy := x, y
				if flipH {
					sx = cw - 1 - x
				}
				if flipV {
					sy = ch - 1 - y
				}
				n[y*cw+x] = cur[sy*cw+sx]
			}
		}
		cur = n
	}
	for q := 0; q < ((quarters%4)+4)%4; q++ {
		n := make([]T, len(cur))
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				n[x*ch+(ch-1-y)] = cur[y*cw+x] // (x, y) goes to (ch-1-y, x) of the turned grid, which is ch wide
			}
		}
		cur, cw, ch = n, ch, cw
	}
	return cur, cw, ch
}

func lerp8(a, b uint8, t float32) uint8 {
	return uint8(float32(a) + (float32(b)-float32(a))*t + 0.5)
}

// Retouch does one of the three: spec is JSON (retouchSpec). It returns the picture (not emitted, as DrawShape), with a note for the model in Note.
func (a *App) Retouch(rot int, spec string) (ImageInfo, error) {
	var sp retouchSpec
	if err := json.Unmarshal([]byte(spec), &sp); err != nil {
		return ImageInfo{}, fmt.Errorf("the retouch is not understood: %v", err)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if base == nil {
		return ImageInfo{}, fmt.Errorf("there is no image in this window")
	}
	sh, err := a.shapeOf(sp, base.Bounds())
	if err != nil {
		return ImageInfo{}, err
	}
	note := ""
	switch sp.Op {
	case "clone":
		note, err = a.cloneRegion(base, sp, sh)
	case "blend":
		err = blendEdges(base, sp, sh)
		if err == nil {
			note = seamReport(base, sh, 6)
		}
	case "fill":
		var src, avoid *maskData
		if sp.SourceMask != "" {
			if src, err = a.masks.get(sp.SourceMask, base.Bounds().Dx(), base.Bounds().Dy()); err != nil {
				return ImageInfo{}, err
			}
		}
		if sp.AvoidMask != "" {
			if avoid, err = a.masks.get(sp.AvoidMask, base.Bounds().Dx(), base.Bounds().Dy()); err != nil {
				return ImageInfo{}, err
			}
		}
		switch sp.Method {
		case "", "patch":
			err = fillRegion(base, sp, sh, src, avoid)
		case "smooth", "nearest", "average", "solid":
			err = fillPlain(base, sp, sh, src, avoid)
		default:
			err = fmt.Errorf("the fill method is patch, smooth, nearest, average or solid, not %q", sp.Method)
		}
	default:
		err = fmt.Errorf("unknown retouch %q", sp.Op)
	}
	if err != nil {
		return ImageInfo{}, err
	}
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	info := a.Image()
	info.Note = note
	return info, nil
}

// cloneLog remembers where parts were copied from and to, so that a second copy from the same place (a repeat that shows) can be told.
type cloneEntry struct{ from, to image.Rectangle }

func overlapArea(a, b image.Rectangle) int { r := a.Intersect(b); return r.Dx() * r.Dy() }

func (a *App) noteClone(from, to image.Rectangle) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	note := ""
	for _, e := range a.cloneLog {
		if f := from.Dx() * from.Dy(); f > 0 && overlapArea(e.from, from)*10 >= f*6 && overlapArea(e.to, to)*10 < to.Dx()*to.Dy()*3 {
			note = fmt.Sprintf("Careful: about the same part was already copied to %d,%d (%d × %d); copies of one place show as a repeat, take this one from somewhere else or change its scale and angle.", e.to.Min.X, e.to.Min.Y, e.to.Dx(), e.to.Dy())
			break
		}
	}
	a.cloneLog = append(a.cloneLog, cloneEntry{from, to})
	if len(a.cloneLog) > 24 {
		a.cloneLog = a.cloneLog[1:]
	}
	return note
}

// bilinear is the color at the place u, v of a w × h grid (the middle of the first pixel is 0, 0), the grid going on as its last pixels outside.
func bilinear(px []color.NRGBA, w, h int, u, v float64) color.NRGBA {
	u, v = math.Min(math.Max(u, 0), float64(w-1)), math.Min(math.Max(v, 0), float64(h-1))
	x0, y0 := int(u), int(v)
	x1, y1 := min(x0+1, w-1), min(y0+1, h-1)
	fx, fy := float32(u-float64(x0)), float32(v-float64(y0))
	mix := func(a, b, c, d uint8) uint8 {
		top, bot := float32(a)+(float32(b)-float32(a))*fx, float32(c)+(float32(d)-float32(c))*fx
		return uint8(top + (bot-top)*fy + 0.5)
	}
	p00, p10, p01, p11 := px[y0*w+x0], px[y0*w+x1], px[y1*w+x0], px[y1*w+x1]
	return color.NRGBA{mix(p00.R, p10.R, p01.R, p11.R), mix(p00.G, p10.G, p01.G, p11.G), mix(p00.B, p10.B, p01.B, p11.B), mix(p00.A, p10.A, p01.A, p11.A)}
}

func bilinearF(pl []float32, w, h int, u, v float64) float32 {
	if u < -0.5 || v < -0.5 || u > float64(w)-0.5 || v > float64(h)-0.5 {
		return 0
	}
	u, v = math.Min(math.Max(u, 0), float64(w-1)), math.Min(math.Max(v, 0), float64(h-1))
	x0, y0 := int(u), int(v)
	x1, y1 := min(x0+1, w-1), min(y0+1, h-1)
	fx, fy := float32(u-float64(x0)), float32(v-float64(y0))
	top := pl[y0*w+x0] + (pl[y0*w+x1]-pl[y0*w+x0])*fx
	bot := pl[y1*w+x0] + (pl[y1*w+x1]-pl[y1*w+x0])*fx
	return top + (bot-top)*fy
}

// cloneRegion copies the shape (a box, a polygon or a mask) so that the top left corner of its box lands on (tx, ty); then it is flipped, turned (any angle,
// clockwise, around its middle) and scaled if asked. The edge is soft over `feather` pixels, and `opacity` lets what is under it show. With blend "seamless" the
// patch keeps its own texture but takes the light and the color of the place (Poisson cloning, like a healing brush). What is copied is the picture as it is
// before the call. The answer is a note for the model.
func (a *App) cloneRegion(base *image.NRGBA, sp retouchSpec, sh *shape) (string, error) {
	b := base.Bounds()
	opacity := float32(1)
	if sp.Opacity != nil {
		opacity = float32(math.Min(1, math.Max(0, *sp.Opacity)))
	}
	scale := 1.0
	if sp.Scale != nil {
		scale = *sp.Scale
	}
	if scale < 0.1 || scale > 10 {
		return "", fmt.Errorf("scale is between 0.1 and 10")
	}
	seamless := sp.Blend == "seamless"
	if sp.Blend != "" && !seamless {
		return "", fmt.Errorf("blend is empty or seamless")
	}
	w, h := sh.r.Dx(), sh.r.Dy()
	pad := max(0, min(sp.Feather, 200))
	if seamless {
		pad = max(pad, 2) // (the guidance needs the pixels around the shape)
	}
	pw, ph := w+2*pad, h+2*pad
	px := make([]color.NRGBA, pw*ph) // the shape with a margin of the picture around it, so that a soft edge has colors to show
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			px[y*pw+x] = base.NRGBAAt(min(max(sh.r.Min.X+x-pad, 0), b.Max.X-1), min(max(sh.r.Min.Y+y-pad, 0), b.Max.Y-1))
		}
	}
	mask := make([]uint8, pw*ph)
	for y := 0; y < h; y++ {
		copy(mask[(y+pad)*pw+pad:(y+pad)*pw+pad+w], sh.m[y*w:(y+1)*w])
	}
	angle := sp.Angle + float64(sp.Rotate)
	exact := scale == 1 && math.Mod(angle, 90) == 0
	quarters := 0
	if exact {
		quarters = int(math.Round(angle / 90))
	}
	var src []color.NRGBA // the patch as it lands: size dw × dh, its top left at (ox, oy) of the picture
	var alpha []float32
	var dw, dh, ox, oy int
	if exact {
		px, dw, dh = transformGrid(px, pw, ph, sp.FlipH, sp.FlipV, quarters)
		mask, _, _ = transformGrid(mask, pw, ph, sp.FlipH, sp.FlipV, quarters)
		alpha, _, _ = softMask(mask, dw, dh, 0, pad) // (with no feather it is 0 outside and 1 inside)
		ox, oy = sp.TX-pad, sp.TY-pad
		src = px
	} else {
		px, _, _ = transformGrid(px, pw, ph, sp.FlipH, sp.FlipV, 0)
		mask, _, _ = transformGrid(mask, pw, ph, sp.FlipH, sp.FlipV, 0)
		soft, _, _ := softMask(mask, pw, ph, 0, pad)
		th := angle * math.Pi / 180
		cs, sn := math.Cos(th), math.Sin(th)
		hw, hh := float64(pw)*scale/2, float64(ph)*scale/2
		ex := math.Abs(hw*cs) + math.Abs(hh*sn)
		ey := math.Abs(hw*sn) + math.Abs(hh*cs)
		dw, dh = int(math.Ceil(2*ex)), int(math.Ceil(2*ey))
		cxd, cyd := float64(sp.TX)+float64(w)/2, float64(sp.TY)+float64(h)/2 // the middle of the box lands in the middle of where the box would have been
		ox, oy = int(math.Floor(cxd-ex)), int(math.Floor(cyd-ey))
		src, alpha = make([]color.NRGBA, dw*dh), make([]float32, dw*dh)
		for y := 0; y < dh; y++ {
			for x := 0; x < dw; x++ {
				rx, ry := float64(ox+x)+0.5-cxd, float64(oy+y)+0.5-cyd // from the middle, in the picture
				u := (rx*cs+ry*sn)/scale + float64(pw)/2 - 0.5
				v := (-rx*sn+ry*cs)/scale + float64(ph)/2 - 0.5
				src[y*dw+x] = bilinear(px, pw, ph, u, v)
				alpha[y*dw+x] = bilinearF(soft, pw, ph, u, v)
			}
		}
	}
	to := image.Rect(ox, oy, ox+dw, oy+dh).Intersect(b)
	if to.Empty() {
		return "", fmt.Errorf("the copy would land outside the image")
	}
	if seamless {
		poissonClone(base, src, alpha, dw, dh, ox, oy, opacity)
	} else {
		for y := 0; y < dh; y++ {
			for x := 0; x < dw; x++ {
				al := alpha[y*dw+x] * opacity
				bx, by := ox+x, oy+y
				if al <= 0 || bx < b.Min.X || by < b.Min.Y || bx >= b.Max.X || by >= b.Max.Y {
					continue
				}
				s, d := src[y*dw+x], base.NRGBAAt(bx, by)
				base.SetNRGBA(bx, by, color.NRGBA{lerp8(d.R, s.R, al), lerp8(d.G, s.G, al), lerp8(d.B, s.B, al), lerp8(d.A, s.A, al)})
			}
		}
	}
	return a.noteClone(sh.r, to), nil
}

// poissonClone puts the patch (src, with the weights alpha, dw × dh at ox, oy) into the picture so that its gradients stay and its colors follow the picture at the
// edge: the pixels with alpha of a half or more are the unknown, solved with successive over-relaxation (Pérez, Gangnet, Blake: Poisson image editing).
func poissonClone(base *image.NRGBA, src []color.NRGBA, alpha []float32, dw, dh, ox, oy int, opacity float32) {
	b := base.Bounds()
	omega := make([]bool, dw*dh)
	var idx []int
	for i, al := range alpha {
		x, y := ox+i%dw, oy+i/dw
		if al >= 0.5 && image.Pt(x, y).In(b) {
			omega[i] = true
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return
	}
	nb := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	g := func(i, c int) float32 {
		p := src[i]
		return [3]float32{float32(p.R), float32(p.G), float32(p.B)}[c]
	}
	target := func(x, y, c int) float32 {
		p := base.NRGBAAt(x, y)
		return [3]float32{float32(p.R), float32(p.G), float32(p.B)}[c]
	}
	for c := 0; c < 3; c++ {
		f := make([]float32, dw*dh)
		// start from the patch moved to the average color of the edge
		var off, n float32
		for _, i := range idx {
			x, y := i%dw, i/dw
			for _, d := range nb {
				qx, qy := x+d[0], y+d[1]
				if qx < 0 || qy < 0 || qx >= dw || qy >= dh || omega[qy*dw+qx] {
					continue
				}
				if image.Pt(ox+qx, oy+qy).In(b) {
					off += target(ox+qx, oy+qy, c) - g(i, c)
					n++
				}
			}
		}
		if n > 0 {
			off /= n
		}
		for _, i := range idx {
			f[i] = g(i, c) + off
		}
		iters := min(4000, max(300, 8*max(dw, dh)))
		for it := 0; it < iters; it++ {
			var biggest float32
			for _, i := range idx {
				x, y := i%dw, i/dw
				var sum, cnt float32
				for _, d := range nb {
					qx, qy := x+d[0], y+d[1]
					if qx < 0 || qy < 0 || qx >= dw || qy >= dh {
						continue // (the patch ends here: nothing to learn from beyond it)
					}
					j := qy*dw + qx
					switch {
					case omega[j]:
						sum += f[j] + g(i, c) - g(j, c)
					case image.Pt(ox+qx, oy+qy).In(b):
						sum += target(ox+qx, oy+qy, c) // (no guidance across the edge: what the source had outside the shape does not count, so a patch that was cut out of the middle of something does not bring its own border)
					default:
						continue
					}
					cnt++
				}
				if cnt == 0 {
					continue
				}
				nv := f[i] + 1.9*(sum/cnt-f[i])
				biggest = max(biggest, float32(math.Abs(float64(nv-f[i]))))
				f[i] = nv
			}
			if it%20 == 19 && biggest < 0.02 {
				break
			}
		}
		for _, i := range idx {
			x, y := ox+i%dw, oy+i/dw
			p := base.NRGBAAt(x, y)
			v := uint8(math.Min(255, math.Max(0, float64(f[i])+0.5)))
			cur := [3]uint8{p.R, p.G, p.B}
			cur[c] = lerp8(cur[c], v, opacity)
			base.SetNRGBA(x, y, color.NRGBA{cur[0], cur[1], cur[2], p.A})
		}
	}
}

// blendEdges hides the seam around the shape (a patch that was put there): the three methods work on the edge of the shape, `width` pixels each side.
//
//	smooth       the edge band is blurred
//	color_match  the colors inside are moved to the average and the spread of the pixels around
//	poisson      the seam is spread out smoothly over the inside (the inside gets a gentle correction that is the difference of the two sides at the edge)
func blendEdges(base *image.NRGBA, sp retouchSpec, sh *shape) error {
	width := sp.Width
	if width < 1 {
		width = 8
	}
	width = min(width, 200)
	strength := float32(1)
	if sp.Strength != nil {
		strength = float32(math.Min(1, math.Max(0, *sp.Strength)))
	}
	switch sp.Method {
	case "smooth", "":
		blendSmooth(base, sh, width, strength)
	case "color_match":
		blendColorMatch(base, sh, width, strength)
	case "poisson":
		blendPoisson(base, sh, strength)
	default:
		return fmt.Errorf("the method is smooth, color_match or poisson, not %q", sp.Method)
	}
	return nil
}

func blendSmooth(base *image.NRGBA, sh *shape, width int, strength float32) {
	b := base.Bounds()
	area := sh.r.Inset(-width * 2).Intersect(b)
	aw, ah := area.Dx(), area.Dy()
	// the shape's mask in the area, soft over twice the width: 1 inside, 0 outside, 1/2 on the edge
	full := make([]uint8, aw*ah)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if sh.in(x, y) {
				full[(y-area.Min.Y)*aw+x-area.Min.X] = 255
			}
		}
	}
	soft, _, _ := softMask(full, aw, ah, 0, width*2)
	var ch [4][]float32
	for c := range ch {
		ch[c] = make([]float32, aw*ah)
	}
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			p := base.NRGBAAt(area.Min.X+x, area.Min.Y+y)
			ch[0][y*aw+x], ch[1][y*aw+x], ch[2][y*aw+x], ch[3][y*aw+x] = float32(p.R), float32(p.G), float32(p.B), float32(p.A)
		}
	}
	for c := range ch {
		ch[c] = boxBlur(ch[c], aw, ah, max(1, width/2))
	}
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			wgt := (1 - float32(math.Abs(float64(2*soft[y*aw+x]-1)))) * strength // 1 on the edge, 0 away from it
			if wgt <= 0.01 {
				continue
			}
			p := base.NRGBAAt(area.Min.X+x, area.Min.Y+y)
			base.SetNRGBA(area.Min.X+x, area.Min.Y+y, color.NRGBA{
				lerp8(p.R, uint8(ch[0][y*aw+x]+0.5), wgt), lerp8(p.G, uint8(ch[1][y*aw+x]+0.5), wgt),
				lerp8(p.B, uint8(ch[2][y*aw+x]+0.5), wgt), lerp8(p.A, uint8(ch[3][y*aw+x]+0.5), wgt)})
		}
	}
}

func blendColorMatch(base *image.NRGBA, sh *shape, width int, strength float32) {
	b := base.Bounds()
	ring := width * 2
	area := sh.r.Inset(-ring * 2).Intersect(b)
	aw, ah := area.Dx(), area.Dy()
	inside := make([]uint8, aw*ah)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if sh.in(x, y) {
				inside[(y-area.Min.Y)*aw+x-area.Min.X] = 255
			}
		}
	}
	near, _, _ := softMask(inside, aw, ah, 0, ring*2) // above 0 within about the ring of the shape
	var si, so, qi, qo [3]float64
	var ni, no float64
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			p := base.NRGBAAt(area.Min.X+x, area.Min.Y+y)
			v := [3]float64{float64(p.R), float64(p.G), float64(p.B)}
			switch {
			case inside[y*aw+x] >= 128:
				ni++
				for c := range v {
					si[c] += v[c]
					qi[c] += v[c] * v[c]
				}
			case near[y*aw+x] > 0.05:
				no++
				for c := range v {
					so[c] += v[c]
					qo[c] += v[c] * v[c]
				}
			}
		}
	}
	if ni == 0 || no == 0 {
		return
	}
	var mi, mo, ratio [3]float64
	for c := 0; c < 3; c++ {
		mi[c], mo[c] = si[c]/ni, so[c]/no
		vi, vo := qi[c]/ni-mi[c]*mi[c], qo[c]/no-mo[c]*mo[c]
		ratio[c] = 1
		if vi > 1 && vo > 1 {
			ratio[c] = math.Min(1.4, math.Max(0.7, math.Sqrt(vo/vi)))
		}
	}
	soft, _, _ := softMask(inside, aw, ah, 0, width) // the correction fades out at the edge, so that it makes no new seam
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			wgt := soft[y*aw+x] * strength
			if wgt <= 0.01 {
				continue
			}
			p := base.NRGBAAt(area.Min.X+x, area.Min.Y+y)
			v := [3]float64{float64(p.R), float64(p.G), float64(p.B)}
			var o [3]uint8
			for c := 0; c < 3; c++ {
				t := (v[c]-mi[c])*ratio[c] + mo[c]
				o[c] = lerp8(uint8(v[c]), uint8(math.Min(255, math.Max(0, t))+0.5), wgt)
			}
			base.SetNRGBA(area.Min.X+x, area.Min.Y+y, color.NRGBA{o[0], o[1], o[2], p.A})
		}
	}
}

func blendPoisson(base *image.NRGBA, sh *shape, strength float32) {
	b := base.Bounds()
	w, h := sh.r.Dx(), sh.r.Dy()
	at := func(x, y int) int { return y*w + x }
	nb := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	edge := make([]bool, w*h)
	var cor [3][]float32 // the correction: given at the edge, smooth inside
	for c := range cor {
		cor[c] = make([]float32, w*h)
	}
	var inner []int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sh.m[at(x, y)] < 128 {
				continue
			}
			px, py := sh.r.Min.X+x, sh.r.Min.Y+y
			var sum [3]float32
			var n float32
			for _, d := range nb {
				qx, qy := px+d[0], py+d[1]
				if !image.Pt(qx, qy).In(b) || sh.in(qx, qy) {
					continue
				}
				q := base.NRGBAAt(qx, qy)
				sum[0], sum[1], sum[2] = sum[0]+float32(q.R), sum[1]+float32(q.G), sum[2]+float32(q.B)
				n++
			}
			if n == 0 {
				inner = append(inner, at(x, y))
				continue
			}
			p := base.NRGBAAt(px, py)
			edge[at(x, y)] = true
			cor[0][at(x, y)], cor[1][at(x, y)], cor[2][at(x, y)] = sum[0]/n-float32(p.R), sum[1]/n-float32(p.G), sum[2]/n-float32(p.B)
		}
	}
	iters := min(3000, 6*max(w, h))
	const omega = 1.9
	for it := 0; it < iters; it++ {
		for _, i := range inner {
			x, y := i%w, i/w
			for c := 0; c < 3; c++ {
				var s float32
				var n float32
				for _, d := range nb {
					qx, qy := x+d[0], y+d[1]
					if qx < 0 || qy < 0 || qx >= w || qy >= h || sh.m[at(qx, qy)] < 128 {
						continue
					}
					s += cor[c][at(qx, qy)]
					n++
				}
				if n > 0 {
					cor[c][i] += omega * (s/n - cor[c][i])
				}
			}
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sh.m[at(x, y)] < 128 {
				continue
			}
			p := base.NRGBAAt(sh.r.Min.X+x, sh.r.Min.Y+y)
			clamp := func(v float32) uint8 { return uint8(math.Min(255, math.Max(0, float64(v)+0.5))) }
			base.SetNRGBA(sh.r.Min.X+x, sh.r.Min.Y+y, color.NRGBA{
				clamp(float32(p.R) + cor[0][at(x, y)]*strength), clamp(float32(p.G) + cor[1][at(x, y)]*strength), clamp(float32(p.B) + cor[2][at(x, y)]*strength), p.A})
		}
	}
}

// fillRegion fills the shape from the pixels around it: it copies small patches of the surroundings that fit what is already there, starting at the
// edge and working in, as the exemplar-based fill (Criminisi) does. It makes textures go on; it does not invent new things.
func fillRegion(base *image.NRGBA, sp retouchSpec, sh *shape, srcOnly, avoid *maskData) error {
	b := base.Bounds()
	patch := sp.Patch
	if patch < 3 {
		patch = 9
	}
	patch = min(patch, 31) | 1
	r := patch / 2
	margin := max(max(sh.r.Dx(), sh.r.Dy()), patch*4)
	win := sh.r.Inset(-margin).Intersect(b)
	if srcOnly != nil { // the window reaches the part it may copy from
		if n, box := srcOnly.count(); n > 0 {
			win = win.Union(box)
		}
	}
	ww, wh := win.Dx(), win.Dy()
	if ww < patch || wh < patch {
		return fmt.Errorf("the image is too small for patches of %d pixels", patch)
	}
	pix := make([]uint8, ww*wh*4)
	known := make([]bool, ww*wh)
	conf := make([]float32, ww*wh)
	remaining := 0
	for y := 0; y < wh; y++ {
		for x := 0; x < ww; x++ {
			p := base.NRGBAAt(win.Min.X+x, win.Min.Y+y)
			i := y*ww + x
			pix[i*4], pix[i*4+1], pix[i*4+2], pix[i*4+3] = p.R, p.G, p.B, p.A
			if sh.in(win.Min.X+x, win.Min.Y+y) {
				remaining++
			} else {
				known[i], conf[i] = true, 1
			}
		}
	}
	if remaining == 0 {
		return nil
	}
	// the places a patch can be taken from: whole patches of known pixels that are allowed (a summed-area table of the others tells)
	sat := make([]int32, (ww+1)*(wh+1))
	for y := 0; y < wh; y++ {
		var row int32
		for x := 0; x < ww; x++ {
			bad := !known[y*ww+x]
			if srcOnly != nil && !srcOnly.at(win.Min.X+x, win.Min.Y+y) {
				bad = true
			}
			if avoid != nil && avoid.at(win.Min.X+x, win.Min.Y+y) {
				bad = true
			}
			if bad {
				row++
			}
			sat[(y+1)*(ww+1)+x+1] = sat[y*(ww+1)+x+1] + row
		}
	}
	unknownIn := func(x0, y0, x1, y1 int) int32 { // [x0, x1) × [y0, y1)
		return sat[y1*(ww+1)+x1] - sat[y0*(ww+1)+x1] - sat[y1*(ww+1)+x0] + sat[y0*(ww+1)+x0]
	}
	stride := 1
	var cands []int
	for {
		cands = cands[:0]
		for y := r; y < wh-r; y += stride {
			for x := r; x < ww-r; x += stride {
				if unknownIn(x-r, y-r, x+r+1, y+r+1) == 0 {
					cands = append(cands, y*ww+x)
				}
			}
		}
		if len(cands) <= 40000 {
			break
		}
		stride++
	}
	if len(cands) == 0 {
		return fmt.Errorf("there is no part to copy from that is whole enough (and inside source_mask, outside avoid_mask): try a smaller patch or a bigger source")
	}
	rng := rand.New(rand.NewSource(1))
	k := min(len(cands), 1500)
	type front struct {
		i int
		c float32
	}
	for pass := 0; remaining > 0 && pass < 4000; pass++ {
		var fr []front
		for y := 0; y < wh; y++ {
			for x := 0; x < ww; x++ {
				i := y*ww + x
				if known[i] {
					continue
				}
				if (x > 0 && known[i-1]) || (x < ww-1 && known[i+1]) || (y > 0 && known[i-ww]) || (y < wh-1 && known[i+ww]) {
					var sum float32
					for dy := -r; dy <= r; dy++ {
						for dx := -r; dx <= r; dx++ {
							if qx, qy := x+dx, y+dy; qx >= 0 && qy >= 0 && qx < ww && qy < wh {
								sum += conf[qy*ww+qx]
							}
						}
					}
					fr = append(fr, front{i, sum})
				}
			}
		}
		if len(fr) == 0 {
			break
		}
		sort.Slice(fr, func(a, c int) bool { return fr[a].c > fr[c].c })
		sample := make([]int, k) // the candidates of this pass
		for j := range sample {
			sample[j] = cands[rng.Intn(len(cands))]
		}
		taken := make(map[int]bool) // the front pixels that are worked on in this pass are a patch apart
		spacing := max(1, r)
		for _, f := range fr {
			x, y := f.i%ww, f.i/ww
			if known[f.i] || taken[(y/spacing)*ww+x/spacing] {
				continue
			}
			taken[(y/spacing)*ww+x/spacing] = true
			bestD, best := int64(math.MaxInt64), -1
			for _, c := range sample {
				cx, cy := c%ww, c/ww
				var d int64
				for dy := -r; dy <= r && d < bestD; dy++ {
					ty := y + dy
					if ty < 0 || ty >= wh {
						continue
					}
					for dx := -r; dx <= r; dx++ {
						tx := x + dx
						if tx < 0 || tx >= ww || !known[ty*ww+tx] {
							continue
						}
						s := ((cy+dy)*ww + cx + dx) * 4
						t := (ty*ww + tx) * 4
						dr, dg, db := int64(pix[t])-int64(pix[s]), int64(pix[t+1])-int64(pix[s+1]), int64(pix[t+2])-int64(pix[s+2])
						d += dr*dr + dg*dg + db*db
					}
				}
				if d < bestD {
					bestD, best = d, c
				}
			}
			if best < 0 {
				continue
			}
			cx, cy := best%ww, best/ww
			c := f.c / float32(patch*patch)
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					tx, ty := x+dx, y+dy
					if tx < 0 || ty < 0 || tx >= ww || ty >= wh || known[ty*ww+tx] {
						continue
					}
					s := ((cy+dy)*ww + cx + dx) * 4
					t := (ty*ww + tx) * 4
					copy(pix[t:t+4], pix[s:s+4])
					known[ty*ww+tx], conf[ty*ww+tx] = true, c
					remaining--
				}
			}
		}
	}
	for y := 0; y < wh; y++ {
		for x := 0; x < ww; x++ {
			if sh.in(win.Min.X+x, win.Min.Y+y) {
				t := (y*ww + x) * 4
				base.SetNRGBA(win.Min.X+x, win.Min.Y+y, color.NRGBA{pix[t], pix[t+1], pix[t+2], pix[t+3]})
			}
		}
	}
	return nil
}

// fillPlain fills the shape without copying patches, from the pixels around it (the ones outside the shape, and inside source_mask / outside avoid_mask when
// those are given):
//
//	smooth   the colors of the edge are spread inward smoothly (a harmonic interpolation, like a stretched membrane): for paper, post-its, walls, skin, sky
//	nearest  every pixel takes the color of the nearest pixel outside the shape (nearest neighbour fill)
//	average  one flat color: the average of the pixels around
//	solid    one flat color that is given
//
// With grain the noise that the surroundings have (the grain of a photo) is added, so that the fill does not look like a flat patch.
func fillPlain(base *image.NRGBA, sp retouchSpec, sh *shape, src, avoid *maskData) error {
	b := base.Bounds()
	w, h := sh.r.Dx(), sh.r.Dy()
	ring := sp.Ring
	if ring < 1 {
		ring = 6
	}
	ring = min(ring, 100)
	eligible := func(x, y int) bool {
		if !image.Pt(x, y).In(b) || sh.in(x, y) {
			return false
		}
		return (src == nil || src.at(x, y)) && (avoid == nil || !avoid.at(x, y))
	}
	idx := func(x, y int) int { return y*w + x }
	inHole := func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && sh.m[idx(x, y)] >= 128 }
	nb := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	res := make([][3]float32, w*h)
	rgbOf := func(x, y int) [3]float32 {
		p := base.NRGBAAt(x, y)
		return [3]float32{float32(p.R), float32(p.G), float32(p.B)}
	}

	// the pixels around: the ones that are eligible within `ring` of the shape (a box around each shape pixel is looked at through a blurred mask)
	area := sh.r.Inset(-ring * 2).Intersect(b)
	aw, ah := area.Dx(), area.Dy()
	around := func() []image.Point {
		inside := make([]uint8, aw*ah)
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if sh.in(x, y) {
					inside[(y-area.Min.Y)*aw+x-area.Min.X] = 255
				}
			}
		}
		near, _, _ := softMask(inside, aw, ah, 0, ring*2)
		var pts []image.Point
		for y := 0; y < ah; y++ {
			for x := 0; x < aw; x++ {
				if px, py := area.Min.X+x, area.Min.Y+y; near[y*aw+x] > 0.03 && eligible(px, py) {
					pts = append(pts, image.Pt(px, py))
				}
			}
		}
		return pts
	}()
	if len(around) == 0 {
		return fmt.Errorf("there are no pixels around the shape to fill from (check source_mask and avoid_mask)")
	}

	// nearest: a flood from the pixels next to the shape, through the shape (also the start of smooth)
	nearest := func() bool {
		type item struct{ x, y int }
		var q []item
		have := make([]bool, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if !inHole(x, y) {
					continue
				}
				for _, d := range nb {
					if px, py := sh.r.Min.X+x+d[0], sh.r.Min.Y+y+d[1]; eligible(px, py) {
						res[idx(x, y)] = rgbOf(px, py)
						have[idx(x, y)] = true
						q = append(q, item{x, y})
						break
					}
				}
			}
		}
		for len(q) > 0 {
			c := q[0]
			q = q[1:]
			for _, d := range nb {
				nx, ny := c.x+d[0], c.y+d[1]
				if inHole(nx, ny) && !have[idx(nx, ny)] {
					res[idx(nx, ny)], have[idx(nx, ny)] = res[idx(c.x, c.y)], true
					q = append(q, item{nx, ny})
				}
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if inHole(x, y) && !have[idx(x, y)] {
					return false // a part of the shape that no eligible pixel touches
				}
			}
		}
		return true
	}

	switch sp.Method {
	case "solid":
		c, err := parseRGB(sp.Color)
		if err != nil {
			return fmt.Errorf("the fill method solid needs a color: %w", err)
		}
		for i := range res {
			res[i] = [3]float32{float32(c.R), float32(c.G), float32(c.B)}
		}
	case "average":
		var sum [3]float64
		for _, p := range around {
			c := rgbOf(p.X, p.Y)
			sum[0], sum[1], sum[2] = sum[0]+float64(c[0]), sum[1]+float64(c[1]), sum[2]+float64(c[2])
		}
		n := float64(len(around))
		for i := range res {
			res[i] = [3]float32{float32(sum[0] / n), float32(sum[1] / n), float32(sum[2] / n)}
		}
	case "nearest":
		if !nearest() {
			return fmt.Errorf("a part of the shape touches no usable pixel: widen source_mask or use another method")
		}
	case "smooth":
		if !nearest() {
			return fmt.Errorf("a part of the shape touches no usable pixel: widen source_mask or use another method")
		}
		var hole []int
		for i := range sh.m {
			if sh.m[i] >= 128 {
				hole = append(hole, i)
			}
		}
		iters := min(6000, max(400, 8*max(w, h)))
		for it := 0; it < iters; it++ {
			var biggest float32
			for _, i := range hole {
				x, y := i%w, i/w
				var sum [3]float32
				var n float32
				for _, d := range nb {
					nx, ny := x+d[0], y+d[1]
					switch {
					case inHole(nx, ny):
						v := res[idx(nx, ny)]
						sum[0], sum[1], sum[2] = sum[0]+v[0], sum[1]+v[1], sum[2]+v[2]
						n++
					case eligible(sh.r.Min.X+nx, sh.r.Min.Y+ny):
						v := rgbOf(sh.r.Min.X+nx, sh.r.Min.Y+ny)
						sum[0], sum[1], sum[2] = sum[0]+v[0], sum[1]+v[1], sum[2]+v[2]
						n++
					}
				}
				if n == 0 {
					continue
				}
				for c := 0; c < 3; c++ {
					nv := res[i][c] + 1.9*(sum[c]/n-res[i][c])
					biggest = max(biggest, float32(math.Abs(float64(nv-res[i][c]))))
					res[i][c] = nv
				}
			}
			if it%10 == 9 && biggest < 0.02 {
				break
			}
		}
	}

	if sp.Grain {
		// the noise of the surroundings: how much a pixel differs from the average of its four neighbours, over the pixels around
		var s2, n float64
		for _, p := range around {
			c := rgbOf(p.X, p.Y)
			var avg [3]float32
			ok := true
			for _, d := range nb {
				if !image.Pt(p.X+d[0], p.Y+d[1]).In(b) || sh.in(p.X+d[0], p.Y+d[1]) {
					ok = false
					break
				}
				v := rgbOf(p.X+d[0], p.Y+d[1])
				avg[0], avg[1], avg[2] = avg[0]+v[0]/4, avg[1]+v[1]/4, avg[2]+v[2]/4
			}
			if !ok {
				continue
			}
			l := float64((c[0]+c[1]+c[2])/3 - (avg[0]+avg[1]+avg[2])/3)
			s2 += l * l
			n++
		}
		if n > 0 {
			sigma := math.Sqrt(s2/n) / 1.12 // (the difference to the mean of four neighbours has about 1.12 times the noise)
			rng := rand.New(rand.NewSource(7))
			for i := range res {
				if sh.m[i] >= 128 {
					g := float32(rng.NormFloat64() * sigma)
					res[i][0], res[i][1], res[i][2] = res[i][0]+g, res[i][1]+g, res[i][2]+g
				}
			}
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !inHole(x, y) {
				continue
			}
			v := res[idx(x, y)]
			cl := func(f float32) uint8 { return uint8(math.Min(255, math.Max(0, float64(f)+0.5))) }
			p := base.NRGBAAt(sh.r.Min.X+x, sh.r.Min.Y+y)
			base.SetNRGBA(sh.r.Min.X+x, sh.r.Min.Y+y, color.NRGBA{cl(v[0]), cl(v[1]), cl(v[2]), p.A})
		}
	}
	return nil
}

// PolygonPNG is the part of the picture (as shown, rot quarter turns) inside the polygon pts (JSON [[x, y], …]) as a PNG in base64: the box of the polygon, with
// what is outside the polygon transparent. It is what the polygon selection copies to the clipboard.
func (a *App) PolygonPNG(rot int, pts string) (string, error) {
	var list [][2]int
	if err := json.Unmarshal([]byte(pts), &list); err != nil {
		return "", fmt.Errorf("the polygon is not a list of [x, y]")
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return "", err
	}
	if base == nil {
		return "", fmt.Errorf("there is no image in this window")
	}
	sh, err := newShape(retouchSpec{Pts: list}, base.Bounds())
	if err != nil {
		return "", err
	}
	w, h := sh.r.Dx(), sh.r.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sh.m[y*w+x] >= 128 {
				out.SetNRGBA(x, y, base.NRGBAAt(sh.r.Min.X+x, sh.r.Min.Y+y))
			}
		}
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, out); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
