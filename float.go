package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"time"

	xdraw "golang.org/x/image/draw"
)

// A selection can be moved and stretched like in Paint: the selected part is lifted (the place it came from turns white), and drawn
// again where the user drops it, at the size it was dragged to. Until the selection is let go (Esc, a click, anything that changes the
// picture) it is "floating": the pixels of the picture are not changed yet, and every move or stretch is worked out from the lifted part
// as it was at the start (src), never from the result before, so a stretch and back loses nothing. Applying it makes the modified
// picture. The UI shows the floating part itself while it is dragged; this side holds it, so that nothing is lost when the window is
// closed, a picture is saved, or something is pasted.

// floatSession is a lifted part of the picture of this window.
type floatSession struct {
	state   int             // the picture state it was lifted from (a new picture ends the session)
	rot     int             // quarter turns of the view; the picture is turned for good when applied, so all rectangles are in the turned picture
	srcRect image.Rectangle // where it came from
	src     *image.NRGBA    // the lifted pixels, as they were
	dest    image.Rectangle // where it is dropped now
	smooth  bool            // smoothed when stretched; false: whole pixels (the pixelated view)
	pasted  bool            // a picture from the clipboard laid over this one: src is its pixels, and nothing was lifted, so nothing is left white (srcRect is empty)
}

// moved is whether the session changes the picture.
func (f *floatSession) moved() bool { return f != nil && (f.pasted || f.dest != f.srcRect) }

// SetFloat tells this side where the selection src (x, y, w, h in the picture as shown, turned rot quarters) is dropped now: dest. The first
// call of a session lifts the part. A session that is there already (same picture, same part) just gets the new place.
func (a *App) SetFloat(rot, sx, sy, sw, sh, dx, dy, dw, dh int, smooth bool) error {
	if err := a.setFloat(rot, sx, sy, sw, sh, dx, dy, dw, dh, smooth); err != nil {
		return err
	}
	a.titleWindow() // a moved part makes the picture modified: the circle in the title
	return nil
}

func (a *App) setFloat(rot, sx, sy, sw, sh, dx, dy, dw, dh int, smooth bool) error {
	src, dest := image.Rect(sx, sy, sx+sw, sy+sh), image.Rect(dx, dy, dx+dw, dy+dh)
	if src.Empty() || dest.Empty() {
		return fmt.Errorf("an empty selection cannot be moved")
	}
	a.mu.Lock()
	f, state := a.fl, a.stateID
	a.mu.Unlock()
	if f == nil || f.state != state || f.rot != rot || f.srcRect != src {
		base, err := a.shownPixels(rot)
		if err != nil {
			return err
		}
		if !src.In(base.Bounds()) {
			return fmt.Errorf("the selection is not inside the image")
		}
		lifted := image.NewNRGBA(image.Rect(0, 0, src.Dx(), src.Dy()))
		draw.Draw(lifted, lifted.Bounds(), base, src.Min, draw.Src)
		f = &floatSession{state: state, rot: rot, srcRect: src, src: lifted}
	}
	f.dest, f.smooth = dest, smooth
	a.mu.Lock()
	if a.stateID == state {
		a.fl = f
	}
	a.mu.Unlock()
	return nil
}

// RestoreFloat is RestoreState for a state that was left with a part lifted: the picture and the lifted part come back together.
func (a *App) RestoreFloat(n, rot, sx, sy, sw, sh, dx, dy, dw, dh int, smooth bool) (ImageInfo, error) {
	return a.restoreState(n, func() error { return a.setFloat(rot, sx, sy, sw, sh, dx, dy, dw, dh, smooth) })
}

// ClearFloat forgets the floating part without applying it (undo went back to before it was lifted).
func (a *App) ClearFloat() { a.mu.Lock(); a.fl = nil; a.mu.Unlock(); a.titleWindow() }

// BakeFloat applies the floating part: the picture of this window becomes the one with the part dropped where it is now. It returns the
// picture (it is not emitted, so that the UI can show it where it is waiting for it). Without a floating part nothing changes.
func (a *App) BakeFloat() (ImageInfo, error) {
	if err := a.bakeFloat(false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

func (a *App) bakeFloat(emit bool) error {
	a.mu.Lock()
	f, state := a.fl, a.stateID
	a.mu.Unlock()
	if f == nil || f.state != state {
		return nil
	}
	if !f.moved() { // lifted and put back where it was: nothing to do
		a.ClearFloat()
		return nil
	}
	base, err := a.shownPixels(f.rot)
	if err != nil {
		return err
	}
	op := draw.Src
	if f.pasted { // laid over, with its transparency; nothing was lifted, so there is no hole
		op = draw.Over
	} else {
		hole := image.Image(image.NewUniform(color.White)) // the place it came from: white, or transparent where the type can be and the picture has transparency already
		if canAlpha(a.current().Path) && !base.Opaque() {  // a picture that has no transparency yet does not get any
			hole = image.Transparent
		}
		draw.Draw(base, f.srcRect.Intersect(base.Bounds()), hole, image.Point{}, draw.Src)
	}
	dst := f.dest.Intersect(base.Bounds())
	switch {
	case f.dest.Size() == f.src.Bounds().Size():
		draw.Draw(base, dst, f.src, dst.Min.Sub(f.dest.Min), op)
	case f.smooth:
		xdraw.CatmullRom.Scale(base, f.dest.Intersect(base.Bounds()), f.src, f.src.Bounds(), xdraw.Op(op), nil)
	default:
		xdraw.NearestNeighbor.Scale(base, f.dest.Intersect(base.Bounds()), f.src, f.src.Bounds(), xdraw.Op(op), nil)
	}
	return a.setPixelsEmit(base, a.current().Path, emit)
}

// DrawStroke is the pencil: a stroke through the pixels pts (x, y, x, y, … of the picture as shown, turned rot quarters) is drawn width pixels wide, round, in
// the color col ("#rrggbb"), and the picture of this window becomes the one with it. It returns the picture (not emitted, as BakeFloat).
func (a *App) DrawStroke(rot int, pts []int, col string, width int) (ImageInfo, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(col, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return ImageInfo{}, fmt.Errorf("color %q is not #rrggbb", col)
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	c := color.NRGBA{r, g, b, 255}
	bb := base.Bounds()
	width = max(1, min(width, 200))
	lim := (float64(width)/2 - 0.25) * (float64(width)/2 - 0.25) // the same round tip as the one the UI shows (see stampOffsets in main.js)
	mid := float64(width-1) / 2
	put := func(x, y int) {
		for j := 0; j < width; j++ {
			for i := 0; i < width; i++ {
				if (float64(i)-mid)*(float64(i)-mid)+(float64(j)-mid)*(float64(j)-mid) > lim {
					continue
				}
				if p := image.Pt(x+i-(width-1)/2, y+j-(width-1)/2); p.In(bb) {
					base.SetNRGBA(p.X, p.Y, c)
				}
			}
		}
	}
	for i := 0; i+1 < len(pts); i += 2 {
		x0, y0 := pts[i], pts[i+1]
		if i == 0 {
			put(x0, y0)
			continue
		}
		x1, y1 := pts[i-2], pts[i-1] // the line from the point before (Bresenham)
		dx, dy := abs(x0-x1), -abs(y0-y1)
		sx, sy := 1, 1
		if x1 > x0 {
			sx = -1
		}
		if y1 > y0 {
			sy = -1
		}
		e := dx + dy
		for {
			put(x1, y1)
			if x1 == x0 && y1 == y0 {
				break
			}
			e2 := 2 * e
			if e2 >= dy {
				e += dy
				x1 += sx
			}
			if e2 <= dx {
				e += dx
				y1 += sy
			}
		}
	}
	if err := a.setPixelsEmit(base, a.current().Path, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// PastedLayer is what PasteLayer puts on the picture: the clipboard picture as a PNG in base64, its size, and where it is laid.
type PastedLayer struct {
	PNG string `json:"png"`
	X   int    `json:"x"`
	Y   int    `json:"y"`
	W   int    `json:"w"`
	H   int    `json:"h"`
}

// PasteLayer is Ctrl+V (Cmd+V, Edit → Paste). A picture on the clipboard that is about as big as this one is probably a new picture, not a part to
// lay over it: the user is asked (see askLarge) whether to make it the picture of this window or to lay it over this one. Otherwise, and when the user
// says so, the clipboard picture is laid over this one, at its own size, with its top left corner at x, y (the UI says the top left corner of what is
// visible; it is moved in when the picture would stick out). Like a lifted part it floats until it is let go: it can be dragged and stretched, and it
// is only part of the picture when it is applied. rot is how the picture is turned. A window without a picture gets the clipboard picture as its
// picture instead, and so does a "new image" answer (W is 0 then, and the window shows it by itself).
func (a *App) PasteLayer(rot, x, y int) (PastedLayer, error) {
	a.mu.Lock()
	if time.Since(a.lastPaste) < 400*time.Millisecond { // the menu and the key can both come for one press
		a.mu.Unlock()
		return PastedLayer{}, nil
	}
	a.lastPaste = time.Now()
	a.mu.Unlock()
	img, err := readClipboardImage()
	if err != nil {
		return PastedLayer{}, err
	}
	return a.pasteClipboard(img, rot, x, y)
}

// PasteNew is Ctrl+V with edit mode off: the clipboard picture becomes the picture of the window, whatever its size, and the one that was open is
// dropped (undo brings it back: the window shows the new picture by itself, which is a step of the history).
func (a *App) PasteNew() error {
	a.mu.Lock()
	if time.Since(a.lastPaste) < 400*time.Millisecond {
		a.mu.Unlock()
		return nil
	}
	a.lastPaste = time.Now()
	a.mu.Unlock()
	img, err := readClipboardImage()
	if err != nil {
		return err
	}
	return a.setClip(img)
}

func (a *App) pasteClipboard(img image.Image, rot, x, y int) (PastedLayer, error) {
	sb := img.Bounds()
	if sb.Dx() < 1 || sb.Dy() < 1 || sb.Dx()*sb.Dy() > maxPixels {
		return PastedLayer{}, fmt.Errorf("the image on the clipboard is %d × %d: too small or too big", sb.Dx(), sb.Dy())
	}
	if a.current().URL == "" {
		return PastedLayer{}, a.setClip(img)
	}
	if a.floating() { // another layer, or a lifted part, is applied first; the picture it makes is turned for good
		if err := a.bakeFloat(true); err != nil {
			return PastedLayer{}, err
		}
		rot = 0
	}
	base, err := a.shownPixels(rot)
	if err != nil {
		return PastedLayer{}, err
	}
	bb := base.Bounds()
	if isLargePaste(sb.Dx(), sb.Dy(), bb.Dx(), bb.Dy()) {
		ask := a.confirmLarge
		if ask == nil {
			ask = a.askLarge
		}
		switch ask(sb.Dx(), sb.Dy(), bb.Dx(), bb.Dy()) {
		case pasteCancel:
			return PastedLayer{}, nil
		case pasteNew:
			if !a.discardOK("paste a new image over it") {
				return PastedLayer{}, nil
			}
			return PastedLayer{}, a.setClip(img)
		}
	}
	n := image.NewNRGBA(image.Rect(0, 0, sb.Dx(), sb.Dy()))
	draw.Draw(n, n.Bounds(), img, sb.Min, draw.Src)
	x, y = max(0, min(x, bb.Dx()-n.Bounds().Dx())), max(0, min(y, bb.Dy()-n.Bounds().Dy()))
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, n); err != nil {
		return PastedLayer{}, err
	}
	if err := a.setPasted(rot, n, image.Rect(x, y, x+n.Bounds().Dx(), y+n.Bounds().Dy()), true); err != nil {
		return PastedLayer{}, err
	}
	a.titleWindow()
	return PastedLayer{PNG: base64.StdEncoding.EncodeToString(buf.Bytes()), X: x, Y: y, W: n.Bounds().Dx(), H: n.Bounds().Dy()}, nil
}

// isLargePaste is whether a clipboard picture of w × h is big enough, next to a picture of pw × ph, to be a new picture and not a part to lay over it:
// it covers about half of it or more, or does not fit in it.
func isLargePaste(w, h, pw, ph int) bool {
	return w > pw || h > ph || 2*w*h >= pw*ph
}

func (a *App) setPasted(rot int, img *image.NRGBA, dest image.Rectangle, smooth bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fl = &floatSession{state: a.stateID, rot: rot, src: img, dest: dest, smooth: smooth, pasted: true}
	return nil
}

func decodeLayer(b64 string) (*image.NRGBA, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	n := image.NewNRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

// SetPasted lays a layer again that the UI has the PNG of (undo and redo go through steps in which it was there).
func (a *App) SetPasted(rot int, b64 string, dx, dy, dw, dh int, smooth bool) error {
	if err := a.setPastedFrom(rot, b64, dx, dy, dw, dh, smooth); err != nil {
		return err
	}
	a.titleWindow()
	return nil
}

func (a *App) setPastedFrom(rot int, b64 string, dx, dy, dw, dh int, smooth bool) error {
	img, err := decodeLayer(b64)
	if err != nil {
		return err
	}
	return a.setPasted(rot, img, image.Rect(dx, dy, dx+dw, dy+dh), smooth)
}

// RestoreLayer is RestoreState for a state that was left with a layer over it: the picture and the layer come back together.
func (a *App) RestoreLayer(n, rot int, b64 string, dx, dy, dw, dh int, smooth bool) (ImageInfo, error) {
	return a.restoreState(n, func() error { return a.setPastedFrom(rot, b64, dx, dy, dw, dh, smooth) })
}

// MovePasted tells this side where the layer is now (dragged or stretched).
func (a *App) MovePasted(dx, dy, dw, dh int, smooth bool) error {
	a.mu.Lock()
	f, state := a.fl, a.stateID
	if f == nil || !f.pasted || f.state != state || dw < 1 || dh < 1 {
		a.mu.Unlock()
		return fmt.Errorf("there is no pasted image to move")
	}
	f.dest, f.smooth = image.Rect(dx, dy, dx+dw, dy+dh), smooth
	a.mu.Unlock()
	a.titleWindow()
	return nil
}
