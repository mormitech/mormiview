package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// The pictures opened last: kept in recent.json in the user config dir (shared by all windows, which are programs of their own), the latest first.
const recentMax = 10

// RecentFile is one entry of the recent pictures, for the home screen.
type RecentFile struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
	Path string `json:"path"`
}

func recentPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "recent.json")
	}
	return ""
}

// loadRecent reads the list; files that are gone are left out.
func loadRecent() []string {
	p := recentPath()
	if p == "" {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var all, out []string
	if json.Unmarshal(b, &all) != nil {
		return nil
	}
	for _, f := range all {
		if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
			out = append(out, f)
		}
	}
	return out
}

func storeRecent(list []string) {
	p := recentPath()
	if p == "" {
		return
	}
	if len(list) > recentMax {
		list = list[:recentMax]
	}
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	if b, err := json.Marshal(list); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

// addRecent puts path first in the list (a picture in the temp folder, such as a scan or a test file, is not worth remembering).
func addRecent(path string) {
	if strings.HasPrefix(path, os.TempDir()) {
		return
	}
	list := []string{path}
	for _, f := range loadRecent() {
		if !samePath(f, path) {
			list = append(list, f)
		}
	}
	storeRecent(list)
}

// Recent is the list for the home screen: up to eight pictures, the latest first.
func (a *App) Recent() []RecentFile {
	var out []RecentFile
	for _, f := range loadRecent() {
		out = append(out, RecentFile{Name: filepath.Base(f), Dir: filepath.Dir(f), Path: f})
		if len(out) == 8 {
			break
		}
	}
	return out
}

// OpenRecent opens a picture of the list (the question about unsaved changes comes first).
func (a *App) OpenRecent(path string) error {
	if !a.discardOK("open another picture") {
		return nil
	}
	return a.setImage(path)
}

// ClearRecent empties the list.
func (a *App) ClearRecent() {
	storeRecent(nil)
	a.refreshRecentMenu()
}

// recentMenu builds File → Open Recent into sub.
func recentMenu(app *App, parent *menu.Menu) {
	app.mu.Lock()
	app.recentSub = parent.AddSubmenu("Open Recent")
	app.mu.Unlock()
	app.fillRecentMenu()
}

func (a *App) fillRecentMenu() bool {
	a.mu.Lock()
	sub := a.recentSub
	a.mu.Unlock()
	if sub == nil {
		return false
	}
	list := loadRecent()
	sig := strings.Join(list, "\n")
	a.mu.Lock()
	if sig == a.recentSig && len(sub.Items) > 0 {
		a.mu.Unlock()
		return false
	}
	a.recentSig = sig
	a.mu.Unlock()
	sub.Items = nil
	for i, f := range list {
		if i == recentMax {
			break
		}
		path := f
		sub.AddText(filepath.Base(f)+"  —  "+filepath.Base(filepath.Dir(f)), nil, func(*menu.CallbackData) {
			if err := a.OpenRecent(path); err != nil {
				a.emit("problem", err.Error())
			}
		})
	}
	if len(list) == 0 {
		sub.AddText("No recent pictures", nil, nil).Disabled = true
	} else {
		sub.AddSeparator()
		sub.AddText("Clear Recent Pictures", nil, func(*menu.CallbackData) { a.ClearRecent() })
	}
	return true
}

// refreshRecentMenu rebuilds the menu when the list changed (this window, or another one, opened something).
func (a *App) refreshRecentMenu() {
	if a.fillRecentMenu() && a.ctx != nil {
		wruntime.MenuUpdateApplicationMenu(a.ctx)
	}
}
