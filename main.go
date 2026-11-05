//go:build !js

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// windowFlags reads --about (show the About window instead of the viewer: mode is "about", "" for the viewer), --theme, --cascade, and the file to open (`mormiview picture.jpg`, which is also how Windows and Linux hand over a
// double-clicked file). The app starts itself with --about, because Wails v2 has one window per process.
func windowFlags(args []string) (mode, theme, file string, cascade int) {
	fs := flag.NewFlagSet("mormiview", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("demo", false, "control mode on a free port, with settings of its own that start empty (for screenshots); --mock is the same")
	fs.Bool("mock", false, "same as --demo")
	var about bool
	fs.BoolVar(&about, "about", false, "show the About window")
	fs.StringVar(&theme, "theme", "", "light or dark, instead of the system's setting (for screenshots)")
	fs.IntVar(&cascade, "cascade", 0, "open this many steps down and to the right of the centre (used for the windows a window starts)")
	fs.Parse(args) // other arguments, such as the ones macOS adds when it launches an app, are ignored
	switch {
	case about:
		mode = "about"
	}
	return mode, theme, fileArg(fs.Args()), cascade
}

func main() {
	startThroughRealPath() // (macOS, started through a link: see reexec.go)
	trace("main")
	app := NewApp()
	title, width, height := "mormiView", 1024, 640
	startDemo(os.Args[1:])
	mode, theme, file, cascade := windowFlags(os.Args[1:])
	about := mode != "" // (a window of its own, not a viewer)
	switch {
	case mode == "about":
		app.mode, title, width, height = "about", "About mormiView", 440, 520
	case file != "":
		app.child = true
		if err := app.setImage(file); err != nil {
			fmt.Fprintln(os.Stderr, "mormiview:", err)
		}
	}
	if !about {
		if st, ok := loadWinState(); ok { // the window comes up the way it was left
			width, height = st.W, st.H
			app.saved = &st
		}
	}
	app.theme, app.cascade = theme, cascade
	app.winW, app.winH = width, height

	var appMenuBar *menu.Menu
	if !about { // the About window has no menu of its own
		appMenuBar = appMenu(app)
		app.menuBar = appMenuBar
	}

	trace("wails.Run")
	err := wails.Run(&options.App{
		Menu:          appMenuBar,
		Title:         title,
		Width:         width,
		Height:        height,
		DisableResize: about,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.imageHandler(),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// Without Mac options Wails leaves the green zoom (maximise) button disabled. OnFileOpen is how macOS hands over the
		// files opened with the app.
		Mac:        macOptions(theme, app),
		Windows:    &windows.Options{Theme: windowsTheme(theme)},
		OnStartup:  app.startup,
		OnDomReady: func(context.Context) { trace("dom ready"); app.reveal() },
		OnShutdown: app.shutdown,
		OnBeforeClose: func(context.Context) bool {
			if !app.discardOK("close the window") {
				return true // the window stays open
			}
			app.SaveWindow()
			return false
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// The window follows the system's light or dark appearance, unless --theme says otherwise (used for screenshots).
func macAppearance(theme string) mac.AppearanceType {
	switch theme {
	case "dark":
		return mac.NSAppearanceNameDarkAqua
	case "light":
		return mac.NSAppearanceNameAqua
	}
	return mac.DefaultAppearance
}

func windowsTheme(theme string) windows.Theme {
	switch theme {
	case "dark":
		return windows.Dark
	case "light":
		return windows.Light
	}
	return windows.SystemDefault
}

// macOptions: OnFileOpen is how macOS hands over the files opened with the app. MORMIVIEW_NO_FILEOPEN is for tracking down slow
// starts: it leaves the handler out.
func macOptions(theme string, app *App) *mac.Options {
	// Esc does not leave full screen (F and Enter do): it is for two Escs, which close the picture
	o := &mac.Options{Appearance: macAppearance(theme), DisableEscapeExitsFullscreen: true}
	if os.Getenv("MORMIVIEW_NO_FILEOPEN") == "" {
		o.OnFileOpen = app.onFileOpen
	}
	return o
}
