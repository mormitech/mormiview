package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds what the UI can call: the version, the window mode and the picture of the window.
type App struct {
	ctx      context.Context
	mu       sync.Mutex
	winW     int // the size the window was asked for, before it is fitted to the screen
	winH     int
	theme    string               // --theme: light or dark instead of the system's setting
	mode     string               // "" for the viewer, "about" for the About window
	image    string               // the picture of this window, as a full path
	imgSeq   int                  // counts the pictures, so that each has its own URL
	child    bool                 // this window was started with a picture (by the system, or by another window)
	pix      pixelCache           // the decoded picture for the pixel readout (see pixel.go)
	infoWins map[string]*exec.Cmd // the windows of their own that are open, by key

	clip        image.Image      // a picture pasted from the clipboard, which has no file yet (image is "" then); see clip.go
	clipPNG     []byte           // the same as a PNG, for the web view
	workDir     string           // the folder of a pasted picture and the one Save As starts in: the Pictures folder, until a picture is saved elsewhere
	titleInfo   string           // size, place in the folder and zoom of the picture, for the title
	ctl         *control         // the commands of the window (see control.go); nil until control mode or the chat panel needs it
	masks       maskStore        // the areas the chat panel's tools named (see masks.go)
	cloneLog    []cloneEntry     // where parts were copied from and to (see retouch.go)
	deleteItem  *menu.MenuItem   // File → Delete Image…: only on while the picture has a file (see updateMenuItems)
	discardItem *menu.MenuItem   // File → Discard Changes: only on while the picture is modified
	effectItems []*menu.MenuItem // the Effects menu: only on while there is a picture
	saveQuality int              // the quality of the JPEG that SaveToPathQ is writing (0: the usual 95)
	saveItem    *menu.MenuItem   // File → Save As…: only on while there is a picture
	closeItem   *menu.MenuItem   // File → Close Image: only on while there is a picture
	origImg     *image.NRGBA     // the picture with no file as it was made or pasted, for Discard Changes (see revert.go)
	chat        chatState        // the chat panel: Claude Code in the background (see chat.go)
	blank       bool             // the picture is a new blank one or one from the clipboard that nothing was done to: it is not modified, and closing it asks nothing (see setClip)
	edit        bool             // edit mode is on: the title says so
	lastPaste   time.Time        // the menu and the key can both ask for one paste
	lastNew     time.Time        // the same for a new window
	deleting    bool             // the question about deleting a picture is open
	saving      bool             // the Save As dialog is open
	spawned     int              // the windows this one started, so that each opens a little further down and right
	cascade     int              // --cascade: how many steps of that this window was started at

	recentSub  *menu.Menu // File → Open Recent
	recentSig  string
	origin     string        // where a picture that has no file came from: new, scan or the clipboard (see unsavedName)
	gifs       *gifSession   // the animated GIF of the window that is edited frame by frame (see gif.go)
	gifMode    gifSaveChoice // what Save As saves of an animation (set while it runs)
	confirmGif func(frame, count int) gifSaveChoice
	rotc       *rotCache        // where a rotation started from, so that turning again does not turn the turned picture (see rotate.go)
	states     map[int]picState // every picture state this window has had, by number, for undo (see undo.go)
	stateID    int              // the number of the one it is in; 0 is the empty window
	nextState  int

	openWindow       func(path string) error                 // starts a window for a picture; spawn, except in the tests
	confirmDiscard   func(name, action string) discardChoice // asks before unsaved changes are thrown away; the system's alert, except in the tests (see discard.go)
	menuBar          *menu.Menu                              // the menu of the window (nil for the About window), put back when full screen ends
	menuPic          int                                     // whether the menu bar shown has the menus of a picture (Edit, Image, Effects): 0 not set yet, 1 no, 2 yes
	closeAnyItem     *menu.MenuItem                          // File → Close (off without a picture)
	menuOff          bool                                    // the menu bar is hidden (full screen on Windows, see SetMenuBar)
	confirmOverwrite func(name string) overwriteChoice       // asks overwrite, save as or cancel for S on a changed picture; the system's alert, except in the tests (see overwrite.go)
	confirmLarge     func(w, h, pw, ph int) largeChoice      // asks what to do with a big clipboard picture; the system's alert, except in the tests (see discard.go)
	fl               *floatSession                           // the part of the picture that is lifted and dropped somewhere else, not applied yet (see float.go)
	saved            *winState                               // the place and size the window was left in, from the last run (see winstate.go)
	turn             int                                     // how many quarter turns the view has (the UI tells), for the Save As that the alert can start
}

func NewApp() *App {
	a := &App{}
	a.openWindow = a.spawn
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	trace("startup")
	fitWindow(ctx, a.winW, a.winH) // every window, the small ones too
	if a.saved != nil && a.mode == "" {
		restorePosition(ctx, *a.saved)
	}
	if a.cascade > 0 && a.mode == "" { // a window started by another one opens a little down and to the right of it, not on top of it
		x, y := wruntime.WindowGetPosition(ctx)
		wruntime.WindowSetPosition(ctx, x+28*a.cascade, y+28*a.cascade)
	}
	trace("window fitted")
	// macOS starts an app hidden when it is launched with a file (Open With, double-click, `open -a`), and Wails does not unhide
	// it: the window came up about 5 seconds later, and the picture with it. Both calls are needed to show it at once (either
	// alone did not).
	a.reveal()
	a.titleWindow()
	if port, ok := controlPort(os.Args[1:], os.Getenv("MORMIVIEW_CONTROL")); ok && a.mode == "" {
		if addr, err := a.startControl(port); err != nil {
			fmt.Fprintln(os.Stderr, "mormiview: control mode:", err)
		} else {
			fmt.Fprintln(os.Stderr, "mormiview: control mode on", addr)
		}
	}
}

// reveal shows the window and the app. It is called at several steps of the start, because it is a race with macOS (see startup).
func (a *App) reveal() {
	if a.ctx == nil || a.mode != "" {
		return
	}
	wruntime.WindowShow(a.ctx)
	wruntime.Show(a.ctx)
	trace("revealed")
}

// Reveal is for the UI: it asks again while the page still finds itself hidden.
func (a *App) Reveal() { a.reveal() }

// ToggleFullscreen is the Enter key: the window goes full screen, and back. On macOS that is the system's full screen, in a Space
// of its own.
func (a *App) ToggleFullscreen() {
	if a.ctx == nil {
		return
	}
	if wruntime.WindowIsFullscreen(a.ctx) {
		wruntime.WindowUnfullscreen(a.ctx)
	} else {
		wruntime.WindowFullscreen(a.ctx)
	}
}

// SetMenuBar shows or hides the menu bar of the window on Windows, where full screen leaves it at the top (macOS has its own, which
// slides down by itself). Hiding sets an empty menu; showing sets the real one again.
func (a *App) SetMenuBar(show bool) {
	if a.ctx == nil || a.menuBar == nil || runtime.GOOS != "windows" {
		return
	}
	a.mu.Lock()
	a.menuOff = !show
	pic := a.menuPic == 2
	a.mu.Unlock()
	if show {
		wruntime.MenuSetApplicationMenu(a.ctx, a.shownMenu(pic))
	} else {
		wruntime.MenuSetApplicationMenu(a.ctx, menu.NewMenu())
	}
}

// SetTurn tells this side how many quarter turns clockwise the picture is shown with, for the Save As that an alert about unsaved changes can start.
func (a *App) SetTurn(rot int) { a.mu.Lock(); a.turn = (rot%4 + 4) % 4; a.mu.Unlock() }

// CloseWindow is Cmd+W (Ctrl+W): it closes the window the way its close button does, which ends this program (one window per process).
func (a *App) CloseWindow() {
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// IsFullscreen tells the UI whether the window is in full screen (it can also be put there with the green button of macOS).
func (a *App) IsFullscreen() bool { return a.ctx != nil && wruntime.WindowIsFullscreen(a.ctx) }

// ExitFullscreen leaves full screen, and does nothing otherwise (the UI does not call it any more: Esc is for double-Esc, see main.js).
func (a *App) ExitFullscreen() {
	if a.ctx != nil && wruntime.WindowIsFullscreen(a.ctx) {
		wruntime.WindowUnfullscreen(a.ctx)
	}
}

// Version is the app version from wails.json.
func (a *App) Version() string { return version() }

// Mode tells the UI which window it is in: "" for the viewer, "about" for the About window.
func (a *App) Mode() string { return a.mode }

// modifiedMark follows the name of a picture with unsaved changes, in the title and in the bottom bar (the UI has the same in main.js).
const modifiedMark = " ●"

// titleWindow names the window after its picture.
func (a *App) titleWindow() {
	if a.ctx == nil || a.mode != "" {
		return
	}
	a.updateMenuItems()
	if cur := a.current(); cur.Name != "" {
		name := cur.Name
		if cur.Modified {
			name += modifiedMark
		}
		wruntime.WindowSetTitle(a.ctx, name+a.sizeMark()+" – mormiView"+a.editMark())
	} else { // undo can take the window back to empty
		wruntime.WindowSetTitle(a.ctx, "mormiView"+a.editMark())
	}
}

// updateMenuItems turns off the items of the File menu that have nothing to work on: Save As… and Close Image without a picture, Delete Image… without a picture or
// with one that has no file (made here, or from the clipboard).
func (a *App) updateMenuItems() {
	a.mu.Lock()
	del, save, closeImg, closeAny, discard := a.deleteItem, a.saveItem, a.closeItem, a.closeAnyItem, a.discardItem
	a.mu.Unlock()
	cur := a.current()
	changed := a.fillRecentMenu() // (another window may have opened something)
	for item, off := range map[*menu.MenuItem]bool{del: cur.Path == "", save: cur.URL == "", closeImg: cur.URL == "", closeAny: cur.URL == "", discard: !cur.Modified} {
		if item != nil && item.Disabled != off {
			item.Disabled = off
			changed = true
		}
	}
	a.mu.Lock()
	fx := append([]*menu.MenuItem(nil), a.effectItems...)
	a.mu.Unlock()
	for _, item := range fx {
		if item.Disabled != (cur.URL == "") {
			item.Disabled = cur.URL == ""
			changed = true
		}
	}
	a.mu.Lock()
	pic := 1 + boolInt(cur.URL != "")
	swap := a.menuBar != nil && pic != a.menuPic // Edit, Image and Effects come and go with the picture
	a.menuPic = pic
	off := a.menuOff
	a.mu.Unlock()
	if swap {
		if !off {
			wruntime.MenuSetApplicationMenu(a.ctx, a.shownMenu(pic == 2))
		}
		return
	}
	if changed {
		wruntime.MenuUpdateApplicationMenu(a.ctx)
	}
}

// pictureMenus are the menus that only work on a picture: without one the menu bar does not show them.
var pictureMenus = map[string]bool{"Edit": true, "Image": true, "Effects": true}

// shownMenu is the menu bar to show: all of a.menuBar with a picture, else without pictureMenus. The items are the same ones, so what
// updateMenuItems turns on and off is in both. (Wails cannot hide a menu of the bar itself, so the bar is set again.)
func (a *App) shownMenu(pic bool) *menu.Menu {
	if pic {
		return a.menuBar
	}
	m := menu.NewMenu()
	for _, item := range a.menuBar.Items {
		if !pictureMenus[item.Label] {
			m.Append(item)
		}
	}
	return m
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (a *App) sizeMark() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.titleInfo != "" {
		return " – " + a.titleInfo
	}
	return ""
}

// SetTitleInfo tells Go what the title of the window says after the name of the picture: its size, its place in the folder and the zoom.
func (a *App) SetTitleInfo(text string) {
	a.mu.Lock()
	a.titleInfo = text
	a.mu.Unlock()
	a.titleWindow()
}

func (a *App) editMark() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.edit {
		return " [edit]"
	}
	return ""
}

// SetEditMode tells Go that edit mode (key E) is on or off, so that the title says so.
func (a *App) SetEditMode(on bool) {
	a.mu.Lock()
	a.edit = on
	a.mu.Unlock()
	a.titleWindow()
}

func (a *App) emit(event string, data ...interface{}) {
	if a.ctx != nil {
		emitEvent(a.ctx, event, data...)
	}
}

// emitEvent sends an event to the page: Wails' events on the desktop; the web build (web_main.go) puts its own in.
var emitEvent = wruntime.EventsEmit

// imageFilter is the file type list of the Open dialog, from imageTypes.
func imageFilter() wruntime.FileFilter {
	exts := make([]string, 0, len(imageTypes))
	for e := range imageTypes {
		exts = append(exts, "*"+e)
	}
	sort.Strings(exts)
	return wruntime.FileFilter{DisplayName: "Images", Pattern: strings.Join(exts, ";")}
}

// ChooseImage shows the Open dialog and opens the picture in this window. Cancelling is not an error.
func (a *App) ChooseImage() error {
	if !a.discardOK("open another image") { // the question about unsaved changes comes before the dialog
		return nil
	}
	dir := a.current().Dir
	path, err := a.pickOpen(wruntime.OpenDialogOptions{
		Title:            "Open file",
		DefaultDirectory: dir,
		Filters:          []wruntime.FileFilter{imageFilter()},
	})
	if err != nil || path == "" {
		return err
	}
	return a.setImage(path)
}

// chooseImage is File → Open…: the same as ChooseImage, with the error shown by the UI.
func (a *App) chooseImage() {
	if err := a.ChooseImage(); err != nil {
		a.emit("problem", err.Error())
	}
}

// spawn opens a window of its own: a second copy of this program, because Wails v2 has one window per process. It shows the picture
// at path, or is empty when path is "". Unlike the About window it lives on when this window is closed. Each window it starts opens
// a little further down and to the right than the one before.
func (a *App) spawn(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.spawned = a.spawned%8 + 1
	n := a.cascade + a.spawned
	a.mu.Unlock()
	args := []string{"--cascade", strconv.Itoa(n)}
	if a.theme != "" {
		args = append(args, "--theme", a.theme)
	}
	if path != "" {
		args = append(args, path)
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// newWindow is File → New Window: an empty window, for a picture to be pasted into or opened.
func (a *App) newWindow() {
	a.mu.Lock()
	if time.Since(a.lastNew) < 400*time.Millisecond { // the menu's key and the page's key can both come for one press
		a.mu.Unlock()
		return
	}
	a.lastNew = time.Now()
	a.mu.Unlock()
	if err := a.openWindow(""); err != nil {
		a.emit("problem", err.Error())
	}
}

// NewWindow is Ctrl+N from the page (on a Mac the menu has Cmd+N; Ctrl+N works there too).
func (a *App) NewWindow() { a.newWindow() }

// showAbout opens the About window. It is closed together with the app, and is not opened twice.
func (a *App) showAbout() { a.showInfo("about") }

// showInfo starts the window of its own for kind ("about"): a copy of this program with --kind. One of each at a time.
func (a *App) showInfo(kind string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c := a.infoWins[kind]; c != nil && c.ProcessState == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	args := []string{"--" + kind}
	if a.theme != "" {
		args = append([]string{"--theme", a.theme}, args...)
	}
	cmd := exec.Command(exe, args...)
	if cmd.Start() != nil {
		return
	}
	if a.infoWins == nil {
		a.infoWins = map[string]*exec.Cmd{}
	}
	a.infoWins[kind] = cmd
	go func() {
		cmd.Wait()
		a.mu.Lock()
		delete(a.infoWins, kind)
		a.mu.Unlock()
	}()
}

// shutdown closes the About window together with the app.
func (a *App) shutdown(ctx context.Context) {
	a.stopChat()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.infoWins {
		if c.Process != nil {
			c.Process.Kill()
		}
	}
}
