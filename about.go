package main

import (
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// helpURL is the page of the manual; Help → mormiView Help opens it in the browser of the system.
const helpURL = "https://mormitech.com/mctools/mormiview/docs/"

func (a *App) openHelp() {
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, helpURL)
	}
}

// appMenu is the menu of the main window, the way each system does it. On macOS that is the application menu (About,
// Hide, Quit) plus File, Edit and Window. On Windows and Linux there are File, Edit and Help menus (mormiView Help, with F1, and About mormiView are in Help; macOS has a Help menu of its own after Window). About opens a
// window of its own, so that it can have real links (the system's message boxes cannot).
func appMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu := m.AddSubmenu("mormiView")
		appMenu.AddText("About mormiView", nil, func(*menu.CallbackData) { app.showAbout() })
		appMenu.AddSeparator()
		appMenu.AddText("Hide mormiView", keys.CmdOrCtrl("h"), func(*menu.CallbackData) { wruntime.Hide(app.ctx) })
		appMenu.AddSeparator()
		appMenu.AddText("Quit mormiView", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { wruntime.Quit(app.ctx) })
		fileMenu(app, m)
		editMenu(app, m)
		imageMenu(app, m)
		effectsMenu(app, m)
		aiMenu(app, m)
		m.Append(menu.WindowMenu())
		m.AddSubmenu("Help").AddText("mormiView Help", nil, func(*menu.CallbackData) { app.openHelp() }) // the last menu, where macOS has it
		return m
	}
	fileMenu(app, m)
	editMenu(app, m)
	imageMenu(app, m)
	effectsMenu(app, m)
	aiMenu(app, m)
	help := m.AddSubmenu("Help")
	help.AddText("mormiView Help", keys.Key("f1"), func(*menu.CallbackData) { app.openHelp() })
	help.AddSeparator()
	help.AddText("About mormiView", nil, func(*menu.CallbackData) { app.showAbout() })
	return m
}

func fileMenu(app *App, m *menu.Menu) {
	file := m.AddSubmenu("File")
	file.AddText("New Window", keys.CmdOrCtrl("n"), func(*menu.CallbackData) { app.newWindow() })
	file.AddText("Open File…", keys.CmdOrCtrl("o"), func(*menu.CallbackData) { app.chooseImage() })
	recentMenu(app, file)
	file.AddText("Create Image", nil, func(*menu.CallbackData) { app.emit("menu-create") }) // a new 1280 × 720 image, in edit mode
	app.saveItem = file.AddText("Save As…", keys.CmdOrCtrl("s"), func(*menu.CallbackData) { app.saveAs() })
	app.closeItem = file.AddText("Close Image", keys.Combo("w", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { app.emit("menu-close-image") })
	app.discardItem = file.AddText("Discard Changes", nil, func(*menu.CallbackData) { app.emit("menu-discard") }) // (greyed out until the picture is changed)
	app.discardItem.Disabled = true
	app.deleteItem = file.AddText("Delete Image…", nil, func(*menu.CallbackData) { app.emit("menu-delete") }) // (the key D does the same; no accelerator, so that a plain letter is not taken from the menu)
	app.saveItem.Disabled, app.closeItem.Disabled = true, true
	app.deleteItem.Disabled = true                                                                  // (until a picture is shown, see updateMenuItems)
	file.AddText("Close", keys.CmdOrCtrl("w"), func(*menu.CallbackData) { app.emit("menu-close") }) // the picture, or the window when there is none (the UI knows)
	if runtime.GOOS != "darwin" {
		file.AddSeparator()
		file.AddText("Exit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) { wruntime.Quit(app.ctx) })
	}
}

// The Edit menu is mormiView's own, not the system's: the system's Copy and Paste work on text in the page, and a picture is not that.
func editMenu(app *App, m *menu.Menu) {
	edit := m.AddSubmenu("Edit")
	edit.AddText("Undo", keys.CmdOrCtrl("z"), func(*menu.CallbackData) { app.emit("menu-undo") })
	edit.AddText("Redo", keys.Combo("z", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { app.emit("menu-redo") })
	edit.AddSeparator()
	edit.AddText("Copy", keys.CmdOrCtrl("c"), func(*menu.CallbackData) { app.emit("menu-copy") })
	edit.AddText("Paste", keys.CmdOrCtrl("v"), func(*menu.CallbackData) { app.pasteImage() })
	edit.AddSeparator()
	edit.AddText("Select All", keys.CmdOrCtrl("a"), func(*menu.CallbackData) { app.emit("menu-select-all") })
}
