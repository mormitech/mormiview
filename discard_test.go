package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
)

func TestDiscardOnlyAsksForModifiedPictures(t *testing.T) {
	dir := t.TempDir()
	one, two := filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")
	writePNG(t, one)
	writePNG(t, two)
	a := NewApp()
	asked := 0
	answer := choiceCancel
	a.confirmDiscard = func(name, action string) discardChoice { asked++; return answer }
	a.setImage(one)

	if !a.discardOK("close the window") || asked != 0 {
		t.Fatalf("an unmodified picture asks nothing: asked %d", asked)
	}
	if _, err := a.Step(1); err != nil || asked != 0 || a.current().Name != "b.png" {
		t.Fatalf("stepping from an unmodified picture: asked %d, now %q, %v", asked, a.current().Name, err)
	}

	modify(a) // b.png, modified
	if !a.current().Modified {
		t.Fatal("the paste must modify the picture")
	}
	// cancel: nothing moves
	if _, err := a.Step(1); err != nil || asked != 1 || a.current().Name != "b.png" || !a.current().Modified {
		t.Errorf("Cancel must stay: asked %d, now %q", asked, a.current().Name)
	}
	if a.discardOK("close the window") || asked != 2 {
		t.Error("Cancel is not OK to close")
	}
	// discard: on to the other picture, the changes are gone
	answer = choiceDiscard
	if _, err := a.Step(1); err != nil || a.current().Name != "a.png" || a.current().Modified {
		t.Errorf("Discard must go on: now %q, modified %v, %v", a.current().Name, a.current().Modified, err)
	}
	// the alert says what the picture is called, and what is about to happen
	var gotName, gotAction string
	modify(a)
	a.confirmDiscard = func(name, action string) discardChoice { gotName, gotAction = name, action; return choiceDiscard }
	a.Step(-1)
	if gotName != "a.png" || gotAction != "go to the previous image" {
		t.Errorf("the alert was asked %q, %q", gotName, gotAction)
	}
}

func TestPastedPictureIsNotAskedAboutWhenStepping(t *testing.T) {
	a := NewApp()
	asked := 0
	a.confirmDiscard = func(string, string) discardChoice { asked++; return choiceDiscard }
	a.setClip(solid(2, 2, blue))
	if note, _ := a.Step(1); note != "" || asked != 0 {
		t.Errorf("stepping from a picture without a file says nothing (%q) and asks nothing (%d)", note, asked)
	}
	// nothing was done to it, so closing asks nothing either (the clipboard still has the picture)
	if a.current().Modified || !a.discardOK("close the window") || asked != 0 {
		t.Errorf("a pasted picture that nothing was done to asks nothing: modified %v, asked %d", a.current().Modified, asked)
	}
	// the first change makes it modified, and then closing asks
	pasted := a.current().State
	if _, err := a.PaintPixels(0, `[[0,0,"#FF0000FF"]]`); err != nil {
		t.Fatal(err)
	}
	if !a.current().Modified || !a.discardOK("close the window") || asked != 1 {
		t.Errorf("a changed pasted picture asks once: modified %v, asked %d", a.current().Modified, asked)
	}
	// undo back to the pasted picture: it is unmodified again
	if _, err := a.RestoreState(pasted); err != nil {
		t.Fatal(err)
	}
	if a.current().Modified {
		t.Errorf("undo back to the pasted picture leaves it modified")
	}
}

func TestBlankPictureIsNotModifiedUntilSomethingIsDone(t *testing.T) {
	a := NewApp()
	asked := 0
	a.confirmDiscard = func(string, string) discardChoice { asked++; return choiceCancel }
	if err := a.NewBlank(40, 30, false); err != nil {
		t.Fatal(err)
	}
	if a.current().Modified || !a.discardOK("close the window") || asked != 0 {
		t.Fatalf("a new blank picture asks nothing: modified %v, asked %d", a.current().Modified, asked)
	}
	blank := a.stateID
	if _, err := a.DrawStroke(0, []int{3, 3, 20, 20}, "#ff0000", 3); err != nil {
		t.Fatal(err)
	}
	if !a.current().Modified || a.discardOK("close the window") || asked != 1 {
		t.Fatalf("a drawn picture asks: modified %v, asked %d", a.current().Modified, asked)
	}
	if _, err := a.RestoreState(blank); err != nil || a.current().Modified {
		t.Fatalf("undo back to the blank picture: modified %v, %v", a.current().Modified, err)
	}
}

func TestDrawLayerPutsAPictureIntoTheWindowPicture(t *testing.T) {
	a := NewApp()
	if err := a.NewBlank(20, 20, false); err != nil {
		t.Fatal(err)
	}
	layer := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range layer.Pix {
		layer.Pix[i] = 0 // transparent, but for one pixel
	}
	layer.SetNRGBA(1, 1, color.NRGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	png.Encode(&buf, layer)
	info, err := a.DrawLayer(0, 5, 6, base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil || !info.Modified {
		t.Fatalf("DrawLayer: %v, modified %v", err, info.Modified)
	}
	pc, _, _ := a.decoded()
	if got := color.NRGBAModel.Convert(pc.img.At(6, 7)).(color.NRGBA); got != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("the pixel is %v, want red", got)
	}
	if got := color.NRGBAModel.Convert(pc.img.At(5, 6)).(color.NRGBA); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("a transparent pixel of the layer must leave the white: %v", got)
	}
}

func TestFloodFillStaysInsideItsRegion(t *testing.T) {
	a := NewApp()
	if err := a.NewBlank(10, 10, false); err != nil {
		t.Fatal(err)
	}
	// a black wall across the picture at x = 5, with a gap nowhere
	var wall []int
	for y := 0; y < 10; y++ {
		wall = append(wall, 5, y)
	}
	if _, err := a.DrawStroke(0, wall, "#000000", 1); err != nil {
		t.Fatal(err)
	}
	before := a.stateID
	if info, err := a.FloodFill(0, 1, 1, "#ff0000", 0); err != nil || info.State == before {
		t.Fatalf("fill: %v, state %d (was %d)", err, info.State, before)
	}
	pc, _, _ := a.decoded()
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(pc.img.At(x, y)).(color.NRGBA) }
	if at(2, 8) != (color.NRGBA{255, 0, 0, 255}) || at(8, 8) != (color.NRGBA{255, 255, 255, 255}) || at(5, 3) != (color.NRGBA{0, 0, 0, 255}) {
		t.Errorf("left %v, right %v, wall %v", at(2, 8), at(8, 8), at(5, 3))
	}
	same := a.stateID
	if info, _ := a.FloodFill(0, 1, 1, "#ff0000", 0); info.State != same {
		t.Error("a fill with the color that is there makes no step")
	}
}
