package main

import (
	"fmt"
	"image"
)

// Discard changes: the picture goes back to what it was when it came into the window: the file as it is on the disk, or, for a picture that has no file, the
// one that was made or pasted. It is a step of the history like any other, so undo brings the changes back and redo discards them again.

// DiscardChanges returns the picture of this window to its original. With nothing to discard it changes nothing (the state number stays, so the page can tell).
func (a *App) DiscardChanges() (ImageInfo, error) {
	cur := a.current()
	if cur.URL == "" {
		return ImageInfo{}, fmt.Errorf("there is no image")
	}
	if !cur.Modified {
		return a.Image(), nil
	}
	a.mu.Lock()
	switch {
	case a.image != "": // the file is still on the disk: it is the original
		a.clip, a.clipPNG = nil, nil
		a.blank = false
		a.imgSeq++
		a.pix = pixelCache{}
		a.fl = nil
		a.gifs = nil // (the file as it is on the disk again)
		a.newStateLocked()
		a.mu.Unlock()
		a.titleWindow()
	case a.origImg != nil:
		orig := a.origImg
		a.mu.Unlock()
		if err := a.setPixelsFull(image.Image(orig).(*image.NRGBA), "", false, true); err != nil {
			return ImageInfo{}, err
		}
	default:
		a.mu.Unlock()
		return ImageInfo{}, fmt.Errorf("there is no original of this image to go back to")
	}
	return a.Image(), nil
}
