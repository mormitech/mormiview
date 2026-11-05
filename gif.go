package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The GIF frame editor. An animated GIF that is open (unchanged) becomes a session: its frames are decoded and composed (each one the whole picture as it is at that
// moment), the page shows them with a player, and what is drawn goes into the frame that is shown. The frames are kept as a list of pictures that are never
// changed (an edit puts a new picture in the place of the frame), and every state of the undo history keeps its own list, so undo and redo bring the frames back.
// Saving offers the whole animation or only the frame that is shown.

const maxGifPixels = 25_000_000 // all frames together

var gifSessions atomic.Int64 // numbers the sessions: the keys of the frames tell the pictures of one animation from those of another

type gifSession struct {
	id         int64          // (see gifSessions)
	path       string         // the file, and the picture of the window as long as the window shows it
	w, h       int            // every frame has this size: a change of the size (resize, crop, canvas) ends the session
	orig       []*image.NRGBA // the frames of the file (or the one picture an animation was started from)
	frames     []*image.NRGBA // the frames now
	delays     []int          // milliseconds, of every frame now
	origDelays []int          // ... of the frames of orig
	cur        int            // the frame that is shown (and that the tools work on)
	started    bool           // made from a picture (Start animation), not opened as a GIF
	ovr        bool           // a tool of the chat works on another frame than the one the user has shown: cur is that frame, userCur the user's
	userCur    int
}

// shown is the frame the user has on the screen (what the chat does on other frames does not move it).
func (g *gifSession) shown() int {
	if g.ovr {
		return g.userCur
	}
	return g.cur
}

// gifSnap is the frames and their delays as one state of the history has them.
type gifSnap struct {
	frames []*image.NRGBA
	delays []int
}

// GifMeta tells the page whether the picture is an animation it can show frame by frame, and where it is.
type GifMeta struct {
	Frames int      `json:"frames"` // 0: not an animated GIF (or too big)
	Delays []int    `json:"delays"` // milliseconds of each frame
	Keys   []string `json:"keys"`   // a frame that is changed gets a new key (the page keeps the pictures it has made by key)
	Cur    int      `json:"cur"`    // the frame the tools work on
	Fresh  bool     `json:"fresh"`  // the session has just begun
	Edited bool     `json:"edited"` // a frame was changed
}

// gifSnapshotLocked is the frames as they are now, for the state the window gets (App.mu held); nil without a session.
func (a *App) gifSnapshotLocked() *gifSnap {
	g := a.gifs
	if g == nil || g.path != a.image {
		return nil
	}
	return &gifSnap{frames: append([]*image.NRGBA(nil), g.frames...), delays: append([]int(nil), g.delays...)}
}

// gifNoteEditLocked is called when the window gets new pixels (App.mu held): they are the new picture of the frame that is shown; a picture of another size ends the session.
func (a *App) gifNoteEditLocked(img *image.NRGBA, file string) {
	g := a.gifs
	if g == nil || file != g.path {
		return
	}
	if b := img.Bounds(); b.Dx() != g.w || b.Dy() != g.h {
		a.gifs = nil
		return
	}
	frames := append([]*image.NRGBA(nil), g.frames...)
	frames[g.cur] = img
	g.frames = frames
	if g.ovr && g.userCur != g.cur && g.userCur < len(frames) { // the picture of the window (and what the page shows of it) stays the frame of the user; the tools read g.cur's frame from pix
		var buf bytes.Buffer
		if (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, frames[g.userCur]) == nil {
			a.clip, a.clipPNG = frames[g.userCur], buf.Bytes()
		}
	}
}

// gifOpen decodes and composes the frames of a GIF file; nil when it has one frame only, or is too big.
func gifOpen(path string) (*gifSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, err
	}
	w, h := g.Config.Width, g.Config.Height
	if len(g.Image) < 2 || w < 1 || h < 1 || w*h*len(g.Image) > maxGifPixels {
		return nil, nil
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	s := &gifSession{id: gifSessions.Add(1), path: path, w: w, h: h}
	for i, fr := range g.Image {
		var before *image.NRGBA
		if i < len(g.Disposal) && g.Disposal[i] == gif.DisposalPrevious {
			before = cloneNRGBA(canvas)
		}
		draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		s.orig = append(s.orig, cloneNRGBA(canvas))
		d := 100
		if i < len(g.Delay) && g.Delay[i] > 1 { // (a delay of 0 or 1 hundredth is shown as 100 ms, as the browsers do)
			d = g.Delay[i] * 10
		}
		s.delays = append(s.delays, d)
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalBackground:
				draw.Draw(canvas, fr.Bounds(), image.Transparent, image.Point{}, draw.Src)
			case gif.DisposalPrevious:
				canvas = before
			}
		}
	}
	s.frames = append([]*image.NRGBA(nil), s.orig...)
	s.origDelays = append([]int(nil), s.delays...)
	return s, nil
}

// gifSession is the session of the picture of the window: the one that is going (it goes on while the window shows its file), or a new one when the window shows
// an unchanged GIF file. nil when the picture is not an animation.
func (a *App) gifSession() (*gifSession, bool, error) {
	a.mu.Lock()
	g, image := a.gifs, a.image
	a.mu.Unlock()
	if g != nil && g.path == image {
		return g, false, nil
	}
	cur := a.current()
	if cur.URL == "" || cur.Path == "" || cur.Modified || strings.ToLower(filepath.Ext(cur.Path)) != ".gif" {
		return nil, false, nil
	}
	s, err := gifOpen(cur.Path)
	if err != nil || s == nil {
		return nil, false, err
	}
	a.mu.Lock()
	if a.image == cur.Path {
		a.gifs = s
	}
	a.mu.Unlock()
	return s, true, nil
}

// GifInfo says how many frames the picture has and how long each one is shown (0 frames when it is not an animation that can be stepped through).
func (a *App) GifInfo() (GifMeta, error) {
	g, fresh, err := a.gifSession()
	if err != nil || g == nil {
		return GifMeta{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	m := GifMeta{Frames: len(g.frames), Delays: g.delays, Cur: g.shown(), Fresh: fresh}
	m.Edited = g.started || len(g.frames) != len(g.orig)
	for i, f := range g.frames {
		m.Keys = append(m.Keys, fmt.Sprintf("%d:%p", g.id, f))
		m.Edited = m.Edited || i >= len(g.orig) || f != g.orig[i] || g.delays[i] != g.origDelays[i]
	}
	return m, nil
}

// GifFrame is frame i as a PNG data URL.
func (a *App) GifFrame(i int) (string, error) {
	g, _, err := a.gifSession()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	var fr *image.NRGBA
	if g != nil && i >= 0 && i < len(g.frames) {
		fr = g.frames[i]
	}
	a.mu.Unlock()
	if fr == nil {
		return "", fmt.Errorf("there is no frame %d", i)
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, fr); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SetGifFrame makes frame i the one that the pixel tools read and an edit goes into (the page shows it paused); the picture and the history do not change.
func (a *App) SetGifFrame(i int) error {
	g, _, err := a.gifSession()
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if g == nil || i < 0 || i >= len(g.frames) {
		return fmt.Errorf("there is no frame %d", i)
	}
	g.cur = i
	a.pix = pixelCache{seq: a.imgSeq, img: g.frames[i], orient: 1}
	return nil
}

// restoreGifLocked brings the frames of a state back (App.mu held): the picture of the window is what the player shows, the frame that is shown.
func (a *App) restoreGifLocked(st picState) {
	g := a.gifs
	if g == nil || g.path != st.image {
		return
	}
	if st.gif != nil {
		g.frames, g.delays = append([]*image.NRGBA(nil), st.gif.frames...), append([]int(nil), st.gif.delays...)
	} else { // the state from before the first change
		g.frames, g.delays = append([]*image.NRGBA(nil), g.orig...), append([]int(nil), g.origDelays...)
	}
	g.cur = min(g.cur, len(g.frames)-1)
	a.pix = pixelCache{seq: a.imgSeq, img: g.frames[g.cur], orient: 1}
}

// ---- saving ----

type gifSaveChoice int

const (
	gifSaveCancel gifSaveChoice = iota
	gifSaveAll                  // the whole animation
	gifSaveFrame                // the frame that is shown, as a picture
)

// gifSaving is whether the window has a GIF session with a frame list to save (the session of the picture that is shown).
func (a *App) gifSaving() *gifSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g := a.gifs; g != nil && g.path == a.image {
		return g
	}
	return nil
}

// askGifSave asks what to save when the picture is an animation: the whole GIF or the frame that is shown.
func (a *App) askGifSave(frame, count int) gifSaveChoice {
	if v, ok := a.mockAnswer("gifsave", fmt.Sprint(frame+1)); ok { // control mode: all, frame or cancel
		switch v {
		case "all":
			return gifSaveAll
		case "frame":
			return gifSaveFrame
		}
		return gifSaveCancel
	}
	if a.confirmGif != nil {
		return a.confirmGif(frame, count)
	}
	if a.ctx == nil {
		return gifSaveAll
	}
	text := fmt.Sprintf("This is an animation of %d frames; frame %d is shown.\n\nSave the whole animation, or only this frame?", count, frame+1)
	switch runtime.GOOS {
	case "windows":
		switch windowsDiscardChoice("Save animation", text+"\n\nYes: the whole animation   No: only this frame") {
		case choiceSave:
			return gifSaveAll
		case choiceDiscard:
			return gifSaveFrame
		}
		return gifSaveCancel
	case "darwin", "js": // (js: the web version, whose dialogs have buttons of their own, see web_main.go)
		res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
			Type: wruntime.QuestionDialog, Title: "Save animation", Message: text,
			Buttons: []string{"Whole animation", "This frame", "Cancel"}, DefaultButton: "Whole animation", CancelButton: "Cancel",
		})
		switch {
		case err != nil:
			return gifSaveCancel
		case res == "Whole animation":
			return gifSaveAll
		case res == "This frame":
			return gifSaveFrame
		}
		return gifSaveCancel
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Save animation", Message: "Save the whole animation? (No: only this frame)", DefaultButton: "Yes"})
	switch {
	case err != nil:
		return gifSaveCancel
	case res == "Yes":
		return gifSaveAll
	}
	return gifSaveFrame
}

// encodeGifAnimation writes frames (all of one size) as an animated GIF that repeats for ever; delays are milliseconds.
func encodeGifAnimation(w io.Writer, frames []*image.NRGBA, delays []int) error {
	g := &gif.GIF{LoopCount: 0}
	for i, f := range frames {
		opaque := f.Opaque()
		var pal color.Palette = palette.Plan9
		disposal := byte(gif.DisposalNone)
		if !opaque { // a transparent color of its own, and each frame is cleared before the next (the frames are whole pictures)
			pal = transparentPlan9{}.Quantize(nil, f)
			disposal = gif.DisposalBackground
		}
		p := image.NewPaletted(f.Bounds(), pal)
		draw.FloydSteinberg.Draw(p, f.Bounds(), f, image.Point{})
		g.Image = append(g.Image, p)
		d := 10
		if i < len(delays) {
			d = max(2, delays[i]/10)
		}
		g.Delay = append(g.Delay, d)
		g.Disposal = append(g.Disposal, disposal)
	}
	return gif.EncodeAll(w, g)
}

// ---- making and changing an animation ----

// gifLive is the session of the picture of the window, or an error that says how to get one.
func (a *App) gifLive() (*gifSession, error) {
	g, _, err := a.gifSession()
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("this picture is not an animation: start one first (animation_start)")
	}
	return g, nil
}

// gifBump makes the change of the frames a step of the history (the picture of the window is the frame that is shown, as the history keeps the frames with every state).
func (a *App) gifBump(g *gifSession) (ImageInfo, error) {
	a.mu.Lock()
	img, file := g.frames[g.cur], a.image
	a.mu.Unlock()
	if err := a.setPixelsEmit(img, file, false); err != nil {
		return ImageInfo{}, err
	}
	return a.Image(), nil
}

// GifStart makes the picture the first frame of an animation (rot is how the view turns it). Nothing changes in the picture; the player can be shown.
func (a *App) GifStart(rot int) (ImageInfo, error) {
	if g, _, err := a.gifSession(); err != nil || g != nil {
		return a.Image(), err
	}
	if a.floating() {
		if err := a.bakeFloat(true); err != nil {
			return ImageInfo{}, err
		}
		rot = 0
	}
	img, err := a.shownPixels(rot)
	if err != nil {
		return ImageInfo{}, err
	}
	if img == nil || a.current().URL == "" {
		return ImageInfo{}, fmt.Errorf("there is no picture to make an animation of")
	}
	b := img.Bounds()
	if b.Dx() > 2048 || b.Dy() > 2048 {
		return ImageInfo{}, fmt.Errorf("the picture is %d × %d: an animation is made of pictures up to 2048 on a side", b.Dx(), b.Dy())
	}
	a.mu.Lock()
	a.gifs = &gifSession{id: gifSessions.Add(1), path: a.image, w: b.Dx(), h: b.Dy(), orig: []*image.NRGBA{img}, frames: []*image.NRGBA{img}, delays: []int{100}, origDelays: []int{100}, started: true}
	a.mu.Unlock()
	return a.Image(), nil
}

// GifAddFrame puts a new frame after frame number after (0 based; -1: the one that is shown), a copy of the one that is shown or an empty (transparent) one; it
// is the frame that is shown afterwards. The animation is started first when the picture is none yet.
func (a *App) GifAddFrame(rot int, blank bool, after int, follow bool) (ImageInfo, error) {
	if _, err := a.GifStart(rot); err != nil {
		return ImageInfo{}, err
	}
	g, err := a.gifLive()
	if err != nil {
		return ImageInfo{}, err
	}
	a.mu.Lock()
	if after == -2 {
		after = len(g.frames) - 1 // (the last frame)
	} else if after < 0 {
		after = g.cur
	}
	if after >= len(g.frames) {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("there is no frame %d (the animation has %d)", after+1, len(g.frames))
	}
	if (len(g.frames)+1)*g.w*g.h > maxGifPixels {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("the animation would be too big (%d frames of %d × %d)", len(g.frames)+1, g.w, g.h)
	}
	nf := image.NewNRGBA(image.Rect(0, 0, g.w, g.h))
	if !blank {
		copy(nf.Pix, g.frames[g.cur].Pix)
	}
	d := g.delays[g.cur]
	frames := append(append(append([]*image.NRGBA(nil), g.frames[:after+1]...), nf), g.frames[after+1:]...)
	delays := append(append(append([]int(nil), g.delays[:after+1]...), d), g.delays[after+1:]...)
	old := g.shown()
	oldFrame := g.frames[old]
	g.frames, g.delays = frames, delays
	g.keepShown(old, oldFrame, follow, after+1)
	a.mu.Unlock()
	return a.gifBump(g)
}

// GifDuplicateFrame puts a copy of frame source (0 based) in the place target (0 based, 0 to the number of frames: the copy has that number afterwards, the
// frames from there on move up); it is the frame that is shown afterwards. The animation is started first when the picture is none yet.
func (a *App) GifDuplicateFrame(rot, source, target int, follow bool) (ImageInfo, error) {
	if _, err := a.GifStart(rot); err != nil {
		return ImageInfo{}, err
	}
	g, err := a.gifLive()
	if err != nil {
		return ImageInfo{}, err
	}
	a.mu.Lock()
	n := len(g.frames)
	if source < 0 || source >= n || target < 0 || target > n {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("the animation has %d frames: the source is 1 to %d, the target 1 to %d", n, n, n+1)
	}
	if (n+1)*g.w*g.h > maxGifPixels {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("the animation would be too big (%d frames of %d × %d)", n+1, g.w, g.h)
	}
	old := g.shown()
	oldFrame := g.frames[old]
	g.frames = append(append(append([]*image.NRGBA(nil), g.frames[:target]...), cloneNRGBA(g.frames[source])), g.frames[target:]...)
	g.delays = append(append(append([]int(nil), g.delays[:target]...), g.delays[source]), g.delays[target:]...)
	g.keepShown(old, oldFrame, follow, target)
	a.mu.Unlock()
	return a.gifBump(g)
}

// GifDeleteFrame takes frame i (0 based; -1: the one that is shown) out; the animation keeps at least one frame.
func (a *App) GifDeleteFrame(i int, follow bool) (ImageInfo, error) {
	g, err := a.gifLive()
	if err != nil {
		return ImageInfo{}, err
	}
	a.mu.Lock()
	if i < 0 {
		i = g.shown()
	}
	if i >= len(g.frames) {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("there is no frame %d (the animation has %d)", i+1, len(g.frames))
	}
	if len(g.frames) < 2 {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("the animation has one frame only")
	}
	old := g.shown()
	oldFrame := g.frames[old]
	g.frames = append(append([]*image.NRGBA(nil), g.frames[:i]...), g.frames[i+1:]...)
	g.delays = append(append([]int(nil), g.delays[:i]...), g.delays[i+1:]...)
	g.keepShown(old, oldFrame, follow, min(i, len(g.frames)-1))
	a.mu.Unlock()
	return a.gifBump(g)
}

// GifMoveFrame moves frame from (0 based) to the place to; it is the frame that is shown afterwards.
func (a *App) GifMoveFrame(from, to int, follow bool) (ImageInfo, error) {
	g, err := a.gifLive()
	if err != nil {
		return ImageInfo{}, err
	}
	a.mu.Lock()
	n := len(g.frames)
	if from < 0 || from >= n || to < 0 || to >= n {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("the frames are 1 to %d", n)
	}
	old := g.shown()
	oldFrame := g.frames[old]
	fr, d := g.frames[from], g.delays[from]
	rest := append(append([]*image.NRGBA(nil), g.frames[:from]...), g.frames[from+1:]...)
	restD := append(append([]int(nil), g.delays[:from]...), g.delays[from+1:]...)
	g.frames = append(append(append([]*image.NRGBA(nil), rest[:to]...), fr), rest[to:]...)
	g.delays = append(append(append([]int(nil), restD[:to]...), d), restD[to:]...)
	g.keepShown(old, oldFrame, follow, to)
	a.mu.Unlock()
	return a.gifBump(g)
}

// GifSetDelay sets how long frame i (0 based; -1: all of them) is shown, in milliseconds (20 to 60000).
func (a *App) GifSetDelay(i, ms int) (ImageInfo, error) {
	g, err := a.gifLive()
	if err != nil {
		return ImageInfo{}, err
	}
	ms = min(max(ms, 20), 60000)
	a.mu.Lock()
	if i >= len(g.frames) {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("there is no frame %d (the animation has %d)", i+1, len(g.frames))
	}
	delays := append([]int(nil), g.delays...)
	for k := range delays {
		if i < 0 || k == i {
			delays[k] = ms
		}
	}
	g.delays = delays
	a.mu.Unlock()
	return a.gifBump(g)
}

// ExportFrames writes every frame as a PNG in dir, named prefix-001.png, prefix-002.png… (a file that is there is not replaced).
func (a *App) ExportFrames(dir, prefix string) (string, error) {
	g, err := a.gifLive()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%q is not a full path", dir)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%s is not a folder", dir)
	}
	if prefix == "" {
		prefix = "frame"
	}
	a.mu.Lock()
	frames := append([]*image.NRGBA(nil), g.frames...)
	a.mu.Unlock()
	digits := max(3, len(fmt.Sprint(len(frames))))
	for i := range frames {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%s-%0*d.png", prefix, digits, i+1))); err == nil {
			return "", fmt.Errorf("%s-%0*d.png is there already: choose another prefix or folder", prefix, digits, i+1)
		}
	}
	for i, f := range frames {
		var buf bytes.Buffer
		if err := png.Encode(&buf, f); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%0*d.png", prefix, digits, i+1)), buf.Bytes(), 0o644); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Saved %d frames as %s-%0*d.png … in %s", len(frames), prefix, digits, 1, dir), nil
}

// ExportFramesDialog asks for a folder and exports the frames there (File → Export Frames…).
func (a *App) ExportFramesDialog() (string, error) {
	g, err := a.gifLive()
	if err != nil {
		return "", err
	}
	cur := a.current()
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Export the frames as PNG files to the folder", DefaultDirectory: cur.Dir, CanCreateDirectories: true})
	if err != nil || dir == "" {
		return "", err
	}
	a.mu.Lock()
	n := len(g.frames)
	a.mu.Unlock()
	prefix := "frame"
	if cur.Path != "" {
		prefix = strings.TrimSuffix(cur.Name, filepath.Ext(cur.Name))
	}
	_ = n
	return a.ExportFrames(dir, prefix)
}

// GifSheet is all frames in one picture (rows, left to right, top to bottom, in the order of the animation), for a look at the whole motion; each frame is scaled
// to fit the cell (up to 8 times in whole pixels for a small one).
func (a *App) GifSheet(maxSide int) (map[string]any, error) {
	g, err := a.gifLive()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	frames := append([]*image.NRGBA(nil), g.frames...)
	w, h := g.w, g.h
	a.mu.Unlock()
	maxSide = min(max(maxSide, 256), 4096)
	n := len(frames)
	cols := 1
	for cols*cols < n {
		cols++
	}
	rows := (n + cols - 1) / cols
	z := min(8, max(1, min(maxSide/cols/w, maxSide/rows/h)))
	const gap = 4
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*(w*z+gap)+gap, rows*(h*z+gap)+gap))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.NRGBA{255, 0, 255, 255}), image.Point{}, draw.Src) // (a magenta grid between the frames)
	for i, f := range frames {
		x0, y0 := gap+(i%cols)*(w*z+gap), gap+(i/cols)*(h*z+gap)
		cell := image.NewNRGBA(image.Rect(0, 0, w*z, h*z))
		for y := 0; y < h*z; y++ {
			for x := 0; x < w*z; x++ {
				cell.SetNRGBA(x, y, f.NRGBAAt(x/z, y/z))
			}
		}
		draw.Draw(sheet, image.Rect(x0, y0, x0+w*z, y0+h*z), cell, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, sheet); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "png": base64.StdEncoding.EncodeToString(buf.Bytes()), "frames": n, "cols": cols, "rows": rows, "zoom": z, "w": w, "h": h}, nil
}

// GifTarget is for the chat's tools: frame n (1 based) is the one they work on until it is called with 0, and the frame the user has on the screen stays where it is.
func (a *App) GifTarget(n int) error {
	g, _, err := a.gifSession()
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if g == nil {
		if n == 0 {
			return nil
		}
		return fmt.Errorf("this picture is not an animation")
	}
	if n == 0 { // back to the user's frame
		if g.ovr {
			g.ovr, g.cur = false, min(g.userCur, len(g.frames)-1)
			a.pix = pixelCache{seq: a.imgSeq, img: g.frames[g.cur], orient: 1}
		}
		return nil
	}
	if n < 1 || n > len(g.frames) {
		return fmt.Errorf("there is no frame %d (the animation has %d)", n, len(g.frames))
	}
	if !g.ovr {
		g.ovr, g.userCur = true, g.cur
	}
	g.cur = n - 1
	a.pix = pixelCache{seq: a.imgSeq, img: g.frames[g.cur], orient: 1}
	return nil
}

// keepShown puts g.cur on the frame the user has on the screen after the frames were changed (the same picture, wherever it is now), or follows the change (the
// new frame) for the user's own buttons. old is the user's frame before, oldFrame its picture, follow the index to go to.
func (g *gifSession) keepShown(old int, oldFrame *image.NRGBA, follow bool, to int) {
	g.ovr = false
	if follow {
		g.cur = min(max(to, 0), len(g.frames)-1)
		return
	}
	for i, f := range g.frames {
		if f == oldFrame {
			g.cur = i
			return
		}
	}
	g.cur = min(old, len(g.frames)-1)
}
