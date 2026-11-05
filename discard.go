package main

import (
	"fmt"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// discardChoice is the answer to the question "what about the changes that are not saved?".
type discardChoice int

const (
	choiceCancel  discardChoice = iota // stay where we are
	choiceDiscard                      // go on, the changes are lost
	choiceSave                         // save them (Save As…), then go on
)

// discardOK is asked before something that throws away what is not saved yet: another picture, closing the window. A picture is not saved
// when it is modified (something pasted or drawn into it; a new or clipboard picture that nothing was done to is not; see ImageInfo.Modified). It is true when there is
// nothing to lose, or the changes were saved or the user chose to discard them; it is false when the user cancelled (or cancelled the
// Save As). Undo brings a picture back after going to another one; closing the window cannot be undone.
func (a *App) discardOK(action string) bool {
	cur := a.current()
	if !cur.Modified {
		return true
	}
	ask := a.confirmDiscard
	if ask == nil {
		ask = a.askDiscard
	}
	switch ask(cur.Name, action) {
	case choiceDiscard:
		return true
	case choiceSave:
		msg, err := a.saveAsDialog(a.turn, false) // empty when the Save As dialog was cancelled
		return err == nil && msg != ""
	}
	return false
}

// DiscardOK is for the UI, which asks it before Close Image: false when the user cancelled.
func (a *App) DiscardOK(action string) bool { return a.discardOK(action) }

// askDiscard is the system's own alert, so it looks like every other app on that system:
//
//	macOS    an NSAlert with the buttons Save, Discard, Cancel
//	Windows  a message box with Yes, No, Cancel ("Do you want to save…")
//	Linux    GTK's question dialog has two buttons only: "save?", and when it is No, "discard?"
func (a *App) askDiscard(name, action string) discardChoice {
	if v, ok := a.mockAnswer("discard", name+" | "+action); ok { // control mode: save, discard or (nothing queued) cancel
		switch v {
		case "save":
			return choiceSave
		case "discard":
			return choiceDiscard
		}
		return choiceCancel
	}
	if a.ctx == nil {
		return choiceDiscard
	}
	save := fmt.Sprintf("Do you want to save the changes to %s?\n\nIf you don't save, they are lost, and you %s.", name, action)
	switch runtime.GOOS {
	case "windows":
		return windowsDiscardChoice("Unsaved changes", save)
	case "darwin", "js": // (js: the web version, whose dialogs have buttons of their own, see web_main.go)
		res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
			Type: wruntime.QuestionDialog, Title: "Unsaved changes", Message: save,
			Buttons: []string{"Save", "Discard", "Cancel"}, DefaultButton: "Save", CancelButton: "Cancel",
		})
		switch {
		case err != nil:
			return choiceCancel
		case res == "Save":
			return choiceSave
		case res == "Discard":
			return choiceDiscard
		}
		return choiceCancel
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Unsaved changes", Message: fmt.Sprintf("Do you want to save the changes to %s first?", name)})
	if err != nil {
		return choiceCancel
	}
	if res == "Yes" {
		return choiceSave
	}
	res, err = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Unsaved changes", Message: fmt.Sprintf("Discard the changes to %s and %s?", name, action), DefaultButton: "No"})
	if err == nil && res == "Yes" {
		return choiceDiscard
	}
	return choiceCancel
}

// largeChoice is the answer to "this clipboard picture is big: a new picture, or a part to lay over this one?".
type largeChoice int

const (
	pasteCancel largeChoice = iota // do nothing
	pasteNew                       // it becomes the picture of the window
	pasteOver                      // it is laid over the picture of the window
)

// askLarge is the alert for a big clipboard picture, in the same form as askDiscard (macOS: three buttons; Windows: Yes, No, Cancel; Linux: two questions).
func (a *App) askLarge(w, h, pw, ph int) largeChoice {
	if v, ok := a.mockAnswer("large", fmt.Sprintf("%d × %d over %d × %d", w, h, pw, ph)); ok { // control mode: new, over or cancel
		switch v {
		case "new":
			return pasteNew
		case "over":
			return pasteOver
		}
		return pasteCancel
	}
	if a.ctx == nil {
		return pasteOver
	}
	msg := fmt.Sprintf("The clipboard picture is large (%d × %d, this picture is %d × %d): you probably want a whole new image.", w, h, pw, ph)
	switch runtime.GOOS {
	case "windows":
		switch windowsDiscardChoice("Large image content is about to be pasted", msg+"\n\nYes: paste as a new image\nNo: paste into the current image") { // (Yes comes back as choiceSave, No as choiceDiscard)
		case choiceSave:
			return pasteNew
		case choiceDiscard:
			return pasteOver
		}
		return pasteCancel
	case "darwin", "js": // (js: the web version, whose dialogs have buttons of their own, see web_main.go)
		res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
			Type: wruntime.QuestionDialog, Title: "Large image content is about to be pasted", Message: msg,
			Buttons: []string{"Paste as new image", "Paste into current image", "Cancel"}, DefaultButton: "Paste as new image", CancelButton: "Cancel",
		})
		switch {
		case err != nil:
			return pasteCancel
		case res == "Paste as new image":
			return pasteNew
		case res == "Paste into current image":
			return pasteOver
		}
		return pasteCancel
	}
	res, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Large image content is about to be pasted", Message: msg + "\n\nPaste it as a new image?"})
	if err != nil {
		return pasteCancel
	}
	if res == "Yes" {
		return pasteNew
	}
	res, err = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: "Large image content is about to be pasted", Message: "Paste it into the current image instead?"})
	if err == nil && res == "Yes" {
		return pasteOver
	}
	return pasteCancel
}
