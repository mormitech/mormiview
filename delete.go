package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DeleteImage is the D key: after the system's question the picture file is deleted (on macOS it goes to the Trash), and the picture that is
// now at its place in the folder is shown (the last one when it was the last; the empty window when the folder has no more pictures). The
// returned text is for the UI to show; it is empty when the user said no.
func (a *App) DeleteImage() (string, error) {
	a.mu.Lock()
	if a.deleting {
		a.mu.Unlock()
		return "", nil
	}
	a.deleting = true
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.deleting = false; a.mu.Unlock() }()

	cur := a.current()
	if cur.URL == "" {
		return "", nil
	}
	if cur.Path == "" {
		return "This image has no file to delete", nil
	}
	if a.ctx == nil || !a.confirmDelete(cur.Name) {
		return "", nil
	}
	if err := trashFile(cur.Path); err != nil {
		return "", err
	}
	names, err := listPictures(cur.Dir)
	if err != nil {
		return "", err
	}
	i := indexOf(names, cur.Name) // (not there any more; the list is the folder without it)
	a.emit("history-reset")
	note := "Deleted " + cur.Name
	if len(names) == 0 {
		info, err := a.RestoreState(0)
		if err != nil {
			return "", err
		}
		a.emit("image", info)
		return note, nil
	}
	k := 0
	if i < 0 { // the place the deleted one had, found by its name among the others
		k = insertionIndex(names, cur.Name)
	}
	k = min(k, len(names)-1)
	return note, a.setImage(filepath.Join(cur.Dir, names[k]))
}

// insertionIndex is where name would be in names, which are in the order of listPictures: the place of the picture that was deleted.
func insertionIndex(names []string, name string) int {
	lower := strings.ToLower(name)
	for i, n := range names {
		if l := strings.ToLower(n); l > lower || (l == lower && n > name) {
			return i
		}
	}
	return len(names)
}

func (a *App) confirmDelete(name string) bool {
	if v, ok := a.mockAnswer("delete", name); ok { // control mode: yes, or (nothing queued) no
		return v == "yes"
	}
	msg := fmt.Sprintf("Delete %s?", name)
	yes, buttons := "Delete", []string{"Delete", "Cancel"}
	switch runtime.GOOS {
	case "darwin":
		msg, yes, buttons = fmt.Sprintf("Move %s to the Trash?", name), "Move to Trash", []string{"Move to Trash", "Cancel"}
	default:
		msg += "\n\nThis cannot be undone."
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type: wruntime.QuestionDialog, Title: "Delete image", Message: msg,
		Buttons: buttons, DefaultButton: "Cancel", CancelButton: "Cancel",
	})
	return err == nil && (res == yes || res == "Yes")
}

// trashFile moves a file to the Trash on macOS, and deletes it elsewhere.
func trashFile(path string) error {
	if runtime.GOOS == "darwin" {
		esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
		out, err := exec.Command("osascript", "-e", `tell application "Finder" to delete POSIX file "`+esc+`"`).CombinedOutput()
		if err != nil {
			return fmt.Errorf("cannot move %s to the Trash: %s", filepath.Base(path), strings.TrimSpace(string(out)))
		}
		return nil
	}
	return os.Remove(path)
}

// ConfirmQuit is asked when Esc is pressed twice on the home screen: the system's alert, with Quit and Cancel.
func (a *App) ConfirmQuit() bool {
	if v, ok := a.mockAnswer("quit", ""); ok { // control mode: yes, or (nothing queued) no
		return v == "yes"
	}
	if a.ctx == nil {
		return true
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type: wruntime.QuestionDialog, Title: "Quit mormiView?",
		Message:       "Do you want to quit mormiView? Nothing is open, so nothing is lost.",
		Buttons:       []string{"Quit", "Cancel"},
		DefaultButton: "Quit", CancelButton: "Cancel",
	})
	return err == nil && (res == "Quit" || res == "Yes" || res == "Ok")
}
