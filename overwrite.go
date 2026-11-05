package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// overwriteChoice is the answer to "this picture has changes: replace the file, or save a new one?".
type overwriteChoice int

const (
	overwriteCancel overwriteChoice = iota // nothing is saved
	overwriteFile                          // the changes go into the file of the picture
	overwriteNew                           // Save As, offering the name with the date and time after it
)

// changedName is the name Save As offers for a changed picture that came from a file: the same name with the date and time before the
// type (photo.jpg becomes photo-20261026-153012.jpg), so that the original is not the one that is offered to be replaced.
func changedName(name string, at time.Time) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext) + "-" + at.Format("20060102-150405") + ext
}

// askOverwrite is the question of S for a picture that has changes: Overwrite, Save As, Cancel. The system's alert, as for discardOK.
//
//	macOS    an NSAlert with the buttons Overwrite, Save As…, Cancel
//	Windows  a message box with Yes (overwrite), No (save as), Cancel
//	Linux    GTK's question dialog has two buttons only: "overwrite?", and when it is No, "save as a new file?"
func (a *App) askOverwrite(name string) overwriteChoice {
	if v, ok := a.mockAnswer("overwrite", name); ok { // control mode: overwrite, saveas or cancel
		switch v {
		case "overwrite":
			return overwriteFile
		case "saveas":
			return overwriteNew
		}
		return overwriteCancel
	}
	if a.confirmOverwrite != nil {
		return a.confirmOverwrite(name)
	}
	if a.ctx == nil {
		return overwriteNew
	}
	text := fmt.Sprintf("%s has changes.\n\nOverwrite the file, or save the changes as a new file?", name)
	switch runtime.GOOS {
	case "windows":
		switch windowsDiscardChoice("Save changes", text+"\n\nYes: overwrite   No: save as a new file") {
		case choiceSave:
			return overwriteFile
		case choiceDiscard:
			return overwriteNew
		}
		return overwriteCancel
	case "darwin", "js": // (js: the web version, whose dialogs have buttons of their own, see web_main.go)
		res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
			Type: wruntime.QuestionDialog, Title: "Save changes", Message: text,
			Buttons: []string{"Overwrite", "Save As…", "Cancel"}, DefaultButton: "Save As…", CancelButton: "Cancel",
		})
		switch {
		case err != nil:
			return overwriteCancel
		case res == "Overwrite":
			return overwriteFile
		case res == "Save As…":
			return overwriteNew
		}
		return overwriteCancel
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Save changes", Message: fmt.Sprintf("Overwrite %s with the changes?", name), DefaultButton: "No"})
	if err != nil {
		return overwriteCancel
	}
	if res == "Yes" {
		return overwriteFile
	}
	res, err = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Save changes", Message: "Save the changes as a new file?", DefaultButton: "Yes"})
	if err == nil && res == "Yes" {
		return overwriteNew
	}
	return overwriteCancel
}
