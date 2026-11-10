package main

import (
	"fmt"
	"image"
)

// Undo. The history itself (what the picture was, the mode, the pixelated view, the turn) is kept by the UI, which knows the view.
// What only this side has is the picture: a file, or a picture pasted from the clipboard that is nowhere else. Every time the picture
// of the window changes, it gets a number here (ImageInfo.State) and what is needed to show it again is kept; the UI keeps the numbers
// in its history and asks for one with RestoreState. State 0 is the window with no picture.

// picState is a picture of the window as it was: everything that setImage and setClip change.
type picState struct {
	image   string
	clip    image.Image
	clipPNG []byte
	workDir string
	blank   bool // a new blank picture that nothing was done to
	origin  string
	gif     *gifSnap // the frames of an animated GIF that is edited frame by frame (nil: as in the file)
}

// newStateLocked numbers the picture the window has now, App.mu held.
func (a *App) newStateLocked() {
	a.statesLocked()
	a.nextState++
	a.stateID = a.nextState
	a.states[a.stateID] = picState{image: a.image, clip: a.clip, clipPNG: a.clipPNG, workDir: a.workDir, blank: a.blank, origin: a.origin, gif: a.gifSnapshotLocked()}
}

func (a *App) statesLocked() map[int]picState {
	if a.states == nil {
		a.states = map[int]picState{0: {workDir: a.workDir}}
	}
	return a.states
}

// RestoreState makes the picture state n the one of this window again, and returns it. The UI shows it (nothing is emitted).
func (a *App) RestoreState(n int) (ImageInfo, error) { return a.restoreState(n, nil) }

// restoreState brings a state back; with a lifted part (after) that is set before the window is titled, so that the title is not seen without the circle of a moved part.
func (a *App) restoreState(n int, after func() error) (ImageInfo, error) {
	a.mu.Lock()
	st, ok := a.statesLocked()[n]
	if !ok {
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("that image is not remembered any more")
	}
	a.image, a.clip, a.clipPNG, a.workDir, a.blank, a.origin = st.image, st.clip, st.clipPNG, st.workDir, st.blank, st.origin
	a.stateID = n
	a.fl = nil
	a.imgSeq++ // its own URL, so that the web view loads it again
	if st.clip != nil {
		a.pix = pixelCache{seq: a.imgSeq, img: st.clip, orient: 1}
	} else {
		a.pix = pixelCache{}
	}
	a.restoreGifLocked(st)
	a.mu.Unlock()
	if after != nil {
		if err := after(); err != nil {
			a.titleWindow()
			return a.Image(), err
		}
	}
	a.titleWindow()
	return a.Image(), nil
}

// KeepStates forgets the picture states that are not in n (and not the current one), so that a long history does not keep
// every picture that was pasted in memory.
func (a *App) KeepStates(n []int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	keep := map[int]bool{0: true, a.stateID: true}
	for _, i := range n {
		keep[i] = true
	}
	for i := range a.statesLocked() {
		if !keep[i] {
			delete(a.states, i)
		}
	}
}

// floating is whether a part of the picture is lifted and dropped somewhere else, not applied yet.
func (a *App) floating() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fl.moved()
}
