package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

// Without a picture the menu bar has no Edit, Image or Effects menu; with one it is the whole menu, and the items are the same ones.
func TestShownMenuHidesPictureMenus(t *testing.T) {
	a := &App{}
	a.menuBar = appMenu(a)
	labels := func(m *menu.Menu) (l []string) {
		for _, it := range m.Items {
			l = append(l, it.Label)
		}
		return
	}
	full, bare := a.shownMenu(true), a.shownMenu(false)
	if full != a.menuBar {
		t.Fatal("with a picture the menu bar is not the whole menu")
	}
	for _, l := range labels(bare) {
		if pictureMenus[l] {
			t.Fatalf("without a picture the menu bar has %s: %v", l, labels(bare))
		}
	}
	if len(bare.Items) != len(full.Items)-len(pictureMenus) {
		t.Fatalf("menus without a picture %v, with %v", labels(bare), labels(full))
	}
	for i, it := range bare.Items {
		if it.Label == "File" && it != a.menuBar.Items[i] {
			t.Fatal("the File menu is not the same item")
		}
	}
}
