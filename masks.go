package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"sync"
)

// Masks of the picture for the chat panel's tools (see mcp.go): the model names an area by a color or a shape once, gets an id, and gives the id to the
// retouching tools (clone_region, blend_edges, fill_region) and to snapshot (which can tint the area). A mask is a bitmap of the picture as it was shown when
// it was made; it is not part of the picture and not of the history. It stops being usable when the size of the picture changes.

type maskData struct {
	w, h int
	bits []uint8 // 0 or 255
}

type maskStore struct {
	mu    sync.Mutex
	next  int
	order []string
	m     map[string]*maskData
}

const maxMasks = 12

func (s *maskStore) put(md *maskData) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]*maskData{}
	}
	s.next++
	id := "m" + strconv.Itoa(s.next)
	s.m[id] = md
	s.order = append(s.order, id)
	for len(s.order) > maxMasks { // the oldest go
		delete(s.m, s.order[0])
		s.order = s.order[1:]
	}
	return id
}

func (s *maskStore) get(id string, w, h int) (*maskData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	md, ok := s.m[id]
	if !ok {
		return nil, fmt.Errorf("there is no mask %q (masks come from select_by_color and create_mask; the oldest are dropped after %d)", id, maxMasks)
	}
	if md.w != w || md.h != h {
		return nil, fmt.Errorf("the mask %s was made for an image of %d × %d, and the image is %d × %d now: make it again", id, md.w, md.h, w, h)
	}
	return md, nil
}

func (md *maskData) count() (n int, box image.Rectangle) {
	box = image.Rect(md.w, md.h, 0, 0)
	for y := 0; y < md.h; y++ {
		for x := 0; x < md.w; x++ {
			if md.bits[y*md.w+x] != 0 {
				n++
				box.Min.X, box.Min.Y, box.Max.X, box.Max.Y = min(box.Min.X, x), min(box.Min.Y, y), max(box.Max.X, x+1), max(box.Max.Y, y+1)
			}
		}
	}
	if n == 0 {
		box = image.Rectangle{}
	}
	return
}

// shape is the mask as a shape (its box and the part of the bitmap in it).
func (md *maskData) shape() (*shape, error) {
	n, box := md.count()
	if n == 0 {
		return nil, fmt.Errorf("the mask is empty")
	}
	s := &shape{r: box, m: make([]uint8, box.Dx()*box.Dy())}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		copy(s.m[(y-box.Min.Y)*box.Dx():(y-box.Min.Y+1)*box.Dx()], md.bits[y*md.w+box.Min.X:y*md.w+box.Max.X])
	}
	return s, nil
}

func (md *maskData) at(x, y int) bool {
	return x >= 0 && y >= 0 && x < md.w && y < md.h && md.bits[y*md.w+x] != 0
}

// morph grows (n > 0) or shrinks (n < 0) the mask by |n| pixels, with a round tip.
func (md *maskData) morph(n int) {
	grow := n > 0
	if n < 0 {
		n = -n
	}
	for i := 0; i < n; i++ {
		next := make([]uint8, len(md.bits))
		copy(next, md.bits)
		for y := 0; y < md.h; y++ {
			for x := 0; x < md.w; x++ {
				v := md.bits[y*md.w+x] != 0
				if v == grow {
					continue
				}
				// a pixel that is not in the mask joins when a neighbour is (grow); one that is in the mask leaves when a neighbour is not (shrink)
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					qx, qy := x+d[0], y+d[1]
					if qx < 0 || qy < 0 || qx >= md.w || qy >= md.h {
						if !grow {
							next[y*md.w+x] = 0 // (the edge of the picture counts as outside, so the area shrinks away from it too)
						}
						continue
					}
					if (md.bits[qy*md.w+qx] != 0) == grow {
						if grow {
							next[y*md.w+x] = 255
						} else {
							next[y*md.w+x] = 0
						}
						break
					}
				}
			}
		}
		md.bits = next
	}
}

// fillHoles makes the parts of the mask's background that do not reach the edge of the picture part of the mask.
func (md *maskData) fillHoles() {
	outside := make([]bool, md.w*md.h)
	var stack []int
	push := func(x, y int) {
		if x >= 0 && y >= 0 && x < md.w && y < md.h && !outside[y*md.w+x] && md.bits[y*md.w+x] == 0 {
			outside[y*md.w+x] = true
			stack = append(stack, y*md.w+x)
		}
	}
	for x := 0; x < md.w; x++ {
		push(x, 0)
		push(x, md.h-1)
	}
	for y := 0; y < md.h; y++ {
		push(0, y)
		push(md.w-1, y)
	}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := i%md.w, i/md.w
		push(x+1, y)
		push(x-1, y)
		push(x, y+1)
		push(x, y-1)
	}
	for i := range md.bits {
		if !outside[i] {
			md.bits[i] = 255
		}
	}
}

// selectByColor makes the mask of the pixels that have the color of the pixel x, y (give or take the tolerance, 0 to 100), the ones that are connected to it or all of
// them in the picture.
func selectByColor(img *image.NRGBA, x, y, tolerance int, contiguous bool) (*maskData, error) {
	b := img.Bounds()
	if !image.Pt(x, y).In(b) {
		return nil, fmt.Errorf("the pixel %d,%d is outside the image (%d × %d)", x, y, b.Dx(), b.Dy())
	}
	w, h := b.Dx(), b.Dy()
	seed := img.NRGBAAt(x, y)
	tol := max(0, min(tolerance, 100)) * 255 / 100
	near := func(px, py int) bool {
		c := img.NRGBAAt(px, py)
		return abs(int(c.R)-int(seed.R)) <= tol && abs(int(c.G)-int(seed.G)) <= tol && abs(int(c.B)-int(seed.B)) <= tol
	}
	md := &maskData{w: w, h: h, bits: make([]uint8, w*h)}
	if !contiguous {
		for py := 0; py < h; py++ {
			for px := 0; px < w; px++ {
				if near(px, py) {
					md.bits[py*w+px] = 255
				}
			}
		}
		return md, nil
	}
	stack := []int{y*w + x}
	md.bits[y*w+x] = 255
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		px, py := i%w, i/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			qx, qy := px+d[0], py+d[1]
			if qx >= 0 && qy >= 0 && qx < w && qy < h && md.bits[qy*w+qx] == 0 && near(qx, qy) {
				md.bits[qy*w+qx] = 255
				stack = append(stack, qy*w+qx)
			}
		}
	}
	return md, nil
}

func maskInfo(id string, md *maskData) map[string]any {
	n, box := md.count()
	return map[string]any{"ok": true, "mask": id, "pixels": n, "box": []int{box.Min.X, box.Min.Y, box.Dx(), box.Dy()}, "picture": []int{md.w, md.h}}
}

// maskCommand is the control command "mask": op color (x, y, tolerance, contiguous, grow, shrink, fill_holes), create (a shape: pts or x, y, w, h), edit
// (mask, grow, shrink, fill_holes, invert, add_mask, subtract_mask, intersect_mask).
func (a *App) maskCommand(m map[string]any) (map[string]any, error) {
	a.mu.Lock()
	turn := a.turn
	a.mu.Unlock()
	img, err := a.shownPixels(turn)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, fmt.Errorf("there is no image in this window")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	refine := func(md *maskData) {
		if g := num(m, "grow", 0); g > 0 {
			md.morph(min(g, 200))
		}
		if s := num(m, "shrink", 0); s > 0 {
			md.morph(-min(s, 200))
		}
	}
	switch str(m, "op") {
	case "color":
		contiguous := true
		if v, ok := m["contiguous"].(bool); ok {
			contiguous = v
		}
		tol := 15
		if _, ok := m["tolerance"]; ok {
			tol = num(m, "tolerance", 15)
		}
		md, err := selectByColor(img, num(m, "x", -1), num(m, "y", -1), tol, contiguous)
		if err != nil {
			return nil, err
		}
		if v, ok := m["fill_holes"].(bool); !ok || v {
			md.fillHoles()
		}
		refine(md)
		return maskInfo(a.masks.put(md), md), nil
	case "create":
		raw, _ := json.Marshal(m)
		var sp retouchSpec
		if err := json.Unmarshal(raw, &sp); err != nil {
			return nil, err
		}
		var more struct {
			Polys [][][2]int `json:"polys"`
		}
		json.Unmarshal(raw, &more)
		md := &maskData{w: w, h: h, bits: make([]uint8, w*h)}
		add := func(sh *shape) {
			for y := sh.r.Min.Y; y < sh.r.Max.Y; y++ {
				for x := sh.r.Min.X; x < sh.r.Max.X; x++ {
					if sh.m[(y-sh.r.Min.Y)*sh.r.Dx()+x-sh.r.Min.X] >= 128 {
						md.bits[y*w+x] = 255
					}
				}
			}
		}
		for i, pts := range more.Polys { // several polygons in one mask (the pieces do not have to touch)
			sh, err := newShape(retouchSpec{Pts: pts}, b)
			if err != nil {
				return nil, fmt.Errorf("polygon %d: %w", i+1, err)
			}
			add(sh)
		}
		if len(more.Polys) == 0 || len(sp.Pts) >= 3 || sp.W > 0 {
			sh, err := newShape(sp, b)
			if err != nil {
				return nil, err
			}
			add(sh)
		}
		refine(md)
		return maskInfo(a.masks.put(md), md), nil
	case "edit":
		id := str(m, "mask")
		md, err := a.masks.get(id, w, h)
		if err != nil {
			return nil, err
		}
		other := func(key string) (*maskData, error) {
			if str(m, key) == "" {
				return nil, nil
			}
			return a.masks.get(str(m, key), w, h)
		}
		add, err := other("add_mask")
		if err != nil {
			return nil, err
		}
		sub, err := other("subtract_mask")
		if err != nil {
			return nil, err
		}
		inter, err := other("intersect_mask")
		if err != nil {
			return nil, err
		}
		a.masks.mu.Lock()
		for i := range md.bits {
			if add != nil && add.bits[i] != 0 {
				md.bits[i] = 255
			}
			if sub != nil && sub.bits[i] != 0 {
				md.bits[i] = 0
			}
			if inter != nil && inter.bits[i] == 0 {
				md.bits[i] = 0
			}
			if m["invert"] == true {
				md.bits[i] = 255 - md.bits[i]
			}
		}
		if m["fill_holes"] == true {
			md.fillHoles()
		}
		refine(md)
		a.masks.mu.Unlock()
		return maskInfo(id, md), nil
	}
	return nil, fmt.Errorf("mask: op is color, create or edit")
}

// ---- Seams ------------------------------------------------------------------------------------------------

func lumaOf(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}
func absDiff(a, b color.NRGBA) float64 {
	return (math.Abs(float64(a.R)-float64(b.R)) + math.Abs(float64(a.G)-float64(b.G)) + math.Abs(float64(a.B)-float64(b.B))) / 3
}

// seamReport judges the edge of a shape: how much the picture jumps across it compared with how much it changes anyway on each side (about 1 is a seam
// that cannot be seen, over 2 it shows), the step in brightness, and which side of the shape is the worst.
func seamReport(img *image.NRGBA, sh *shape, width int) string {
	b := img.Bounds()
	cx, cy := float64(sh.r.Min.X+sh.r.Max.X)/2, float64(sh.r.Min.Y+sh.r.Max.Y)/2
	names := [8]string{"E", "SE", "S", "SW", "W", "NW", "N", "NE"}
	var jump [8]float64
	var cnt [8]float64
	var total, nTotal float64
	var lumIn, lumOut, nIn, nOut float64
	for y := sh.r.Min.Y - width; y < sh.r.Max.Y+width; y++ {
		for x := sh.r.Min.X - width; x < sh.r.Max.X+width; x++ {
			if !image.Pt(x, y).In(b) {
				continue
			}
			in := sh.in(x, y)
			p := img.NRGBAAt(x, y)
			for _, d := range [2][2]int{{1, 0}, {0, 1}} {
				qx, qy := x+d[0], y+d[1]
				if !image.Pt(qx, qy).In(b) {
					continue
				}
				q := img.NRGBAAt(qx, qy)
				qin := sh.in(qx, qy)
				diff := absDiff(p, q)
				sector := int(math.Mod(math.Atan2(float64(y)-cy, float64(x)-cx)/(math.Pi/4)+8.5, 8)) % 8
				switch {
				case in != qin: // across the edge
					jump[sector] += diff
					cnt[sector]++
					total += diff
					nTotal++
				}
			}
			if in {
				lumIn += lumaOf(p)
				nIn++
			} else if x >= sh.r.Min.X-width && x < sh.r.Max.X+width && y >= sh.r.Min.Y-width && y < sh.r.Max.Y+width {
				lumOut += lumaOf(p)
				nOut++
			}
		}
	}
	if nTotal == 0 {
		return "the shape has no edge inside the image"
	}
	// what the picture does by itself next to the edge: the changes between neighbours within `width` of it on both sides
	var near, nNear float64
	for y := sh.r.Min.Y - width; y < sh.r.Max.Y+width; y++ {
		for x := sh.r.Min.X - width; x < sh.r.Max.X+width; x++ {
			if !image.Pt(x, y).In(b) || !image.Pt(x+1, y).In(b) || sh.in(x, y) != sh.in(x+1, y) {
				continue
			}
			if nearEdge(sh, x, y, width) {
				near += absDiff(img.NRGBAAt(x, y), img.NRGBAAt(x+1, y))
				nNear++
			}
		}
	}
	base := math.Max(1.5, near/math.Max(nNear, 1))
	ratio := (total / nTotal) / base
	verdict := "invisible"
	switch {
	case ratio > 3:
		verdict = "obvious"
	case ratio > 2:
		verdict = "visible"
	case ratio > 1.4:
		verdict = "faint"
	}
	type sec struct {
		n string
		r float64
	}
	var secs []sec
	for i := range names {
		if cnt[i] > 3 {
			secs = append(secs, sec{names[i], (jump[i] / cnt[i]) / base})
		}
	}
	sort.Slice(secs, func(i, j int) bool { return secs[i].r > secs[j].r })
	out := fmt.Sprintf("Seam ratio %.1f (%s; about 1 cannot be seen, over 2 shows).", ratio, verdict)
	if nIn > 0 && nOut > 0 {
		out += fmt.Sprintf(" Brightness inside is %+.0f against the surroundings (0 to 255).", lumIn/nIn-lumOut/nOut)
	}
	if len(secs) > 0 {
		out += " Worst sides:"
		for i := 0; i < min(3, len(secs)); i++ {
			out += fmt.Sprintf(" %s %.1f", secs[i].n, secs[i].r)
		}
		out += "."
	}
	return out
}

// nearEdge says whether the pixel is within d pixels (a box) of the edge of the shape.
func nearEdge(sh *shape, x, y, d int) bool {
	in := sh.in(x, y)
	for dy := -d; dy <= d; dy += max(1, d/2) {
		for dx := -d; dx <= d; dx += max(1, d/2) {
			if sh.in(x+dx, y+dy) != in {
				return true
			}
		}
	}
	return false
}

// seamCommand is the control command "seam": the report on the edge of a shape (a box, a polygon or a mask), without changing anything.
func (a *App) seamCommand(m map[string]any) (map[string]any, error) {
	a.mu.Lock()
	turn := a.turn
	a.mu.Unlock()
	img, err := a.shownPixels(turn)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, fmt.Errorf("there is no image in this window")
	}
	raw, _ := json.Marshal(m)
	var sp retouchSpec
	if err := json.Unmarshal(raw, &sp); err != nil {
		return nil, err
	}
	sh, err := a.shapeOf(sp, img.Bounds())
	if err != nil {
		return nil, err
	}
	width := sp.Width
	if width < 1 {
		width = 6
	}
	return map[string]any{"ok": true, "report": seamReport(img, sh, min(width, 50))}, nil
}

// shapeOf is newShape, or the mask that the spec names.
func (a *App) shapeOf(sp retouchSpec, bounds image.Rectangle) (*shape, error) {
	if sp.Mask != "" {
		md, err := a.masks.get(sp.Mask, bounds.Dx(), bounds.Dy())
		if err != nil {
			return nil, err
		}
		return md.shape()
	}
	return newShape(sp, bounds)
}
