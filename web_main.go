//go:build js && wasm

package main

// The web version of mormiView: this same program, built for WebAssembly (GOOS=js GOARCH=wasm), runs in the browser with the same
// page (frontend/src) as the desktop app. What Wails does on the desktop is done here by the browser:
//
//   - the methods of App are bound by reflection, the way Wails binds them, as window.go.main.App.<Name> (see frontend/web/boot.js);
//   - the Wails runtime calls of the Go side (dialogs, the title, full screen, the menu) go to webFrontend, which hands them to the page
//     (frontend/web/host.js) — wruntime finds it in the context, as it finds Wails' own frontend on the desktop;
//   - events go to the page's EventsOn listeners (emitEvent);
//   - the files are an in-memory file system that the browser keeps (frontend/web/memfs.js): opened pictures are copied in, and what is saved
//     is also downloaded;
//   - the picture of the window is served by the same imageHandler, asked by the page (mvServe) instead of the asset server.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"syscall/js"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var errType = reflect.TypeOf((*error)(nil)).Elem()

// await waits for a JavaScript promise (from a goroutine of its own: never from a callback of the page, which would block the page).
func await(p js.Value) (js.Value, error) {
	if p.Type() != js.TypeObject || p.Get("then").Type() != js.TypeFunction {
		return p, nil
	}
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	var ok, fail js.Func
	ok = js.FuncOf(func(_ js.Value, args []js.Value) any {
		v := js.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		ch <- result{v: v}
		return nil
	})
	fail = js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "failed"
		if len(args) > 0 {
			if m := args[0].Get("message"); m.Type() == js.TypeString {
				msg = m.String()
			} else {
				msg = args[0].String()
			}
		}
		ch <- result{err: errors.New(msg)}
		return nil
	})
	p.Call("then", ok, fail)
	r := <-ch
	ok.Release()
	fail.Release()
	return r.v, r.err
}

// host is the page's side of the bridge (frontend/web/host.js).
func host(name string, args ...any) js.Value {
	return js.Global().Get("mvHost").Call(name, args...)
}

func hostAwait(name string, args ...any) (js.Value, error) { return await(host(name, args...)) }

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// ---- The frontend that wruntime calls ------------------------------------------------------------------------------

type webFrontend struct {
	app *App

	mu    sync.Mutex
	menu  *menu.Menu
	items map[int]*menu.MenuItem // the items of the menu bar as the page has them, by number
}

func (f *webFrontend) Run(context.Context) error { return nil }
func (f *webFrontend) RunMainLoop()              {}
func (f *webFrontend) ExecJS(code string)        { js.Global().Call("eval", code) }
func (f *webFrontend) Hide()                     {}
func (f *webFrontend) Show()                     {}
func (f *webFrontend) Quit()                     { host("quit") }

func (f *webFrontend) OpenFileDialog(o wruntime.OpenDialogOptions) (string, error) {
	v, err := hostAwait("openFile", toJSON(o))
	if err != nil || v.Type() != js.TypeString {
		return "", err
	}
	return v.String(), nil
}

func (f *webFrontend) OpenMultipleFilesDialog(o wruntime.OpenDialogOptions) ([]string, error) {
	p, err := f.OpenFileDialog(o)
	if p == "" {
		return nil, err
	}
	return []string{p}, err
}

// OpenDirectoryDialog: a page cannot write into a folder of the user; what is written into the downloads folder is downloaded (see memfs.js).
func (f *webFrontend) OpenDirectoryDialog(wruntime.OpenDialogOptions) (string, error) {
	v, err := hostAwait("pickFolder")
	if err != nil || v.Type() != js.TypeString {
		return "", err
	}
	return v.String(), nil
}

func (f *webFrontend) SaveFileDialog(o wruntime.SaveDialogOptions) (string, error) {
	v, err := hostAwait("saveFile", toJSON(o))
	if err != nil || v.Type() != js.TypeString {
		return "", err
	}
	return v.String(), nil
}

func (f *webFrontend) MessageDialog(o wruntime.MessageDialogOptions) (string, error) {
	o.Icon = nil
	v, err := hostAwait("messageDialog", toJSON(o))
	if err != nil || v.Type() != js.TypeString {
		return "", err
	}
	return v.String(), nil
}

func (f *webFrontend) WindowSetTitle(title string) { js.Global().Get("document").Set("title", title) }
func (f *webFrontend) WindowShow()                 {}
func (f *webFrontend) WindowHide()                 {}
func (f *webFrontend) WindowCenter()               {}
func (f *webFrontend) WindowToggleMaximise()       {}
func (f *webFrontend) WindowMaximise()             {}
func (f *webFrontend) WindowUnmaximise()           {}
func (f *webFrontend) WindowMinimise()             {}
func (f *webFrontend) WindowUnminimise()           {}
func (f *webFrontend) WindowSetAlwaysOnTop(bool)   {}
func (f *webFrontend) WindowSetPosition(int, int)  {}
func (f *webFrontend) WindowGetPosition() (int, int) {
	return js.Global().Get("screenX").Int(), js.Global().Get("screenY").Int()
}
func (f *webFrontend) WindowSetSize(int, int) {}
func (f *webFrontend) WindowGetSize() (int, int) {
	return js.Global().Get("innerWidth").Int(), js.Global().Get("innerHeight").Int()
}
func (f *webFrontend) WindowSetMinSize(int, int)               {}
func (f *webFrontend) WindowSetMaxSize(int, int)               {}
func (f *webFrontend) WindowFullscreen()                       { host("fullscreen", true) }
func (f *webFrontend) WindowUnfullscreen()                     { host("fullscreen", false) }
func (f *webFrontend) WindowSetBackgroundColour(*options.RGBA) {}
func (f *webFrontend) WindowReload()                           { js.Global().Get("location").Call("reload") }
func (f *webFrontend) WindowReloadApp()                        { js.Global().Get("location").Call("reload") }
func (f *webFrontend) WindowSetSystemDefaultTheme()            {}
func (f *webFrontend) WindowSetLightTheme()                    {}
func (f *webFrontend) WindowSetDarkTheme()                     {}
func (f *webFrontend) WindowIsMaximised() bool                 { return false }
func (f *webFrontend) WindowIsMinimised() bool                 { return false }
func (f *webFrontend) WindowIsNormal() bool                    { return true }
func (f *webFrontend) WindowIsFullscreen() bool                { return host("isFullscreen").Truthy() }
func (f *webFrontend) WindowClose()                            { host("quit") }
func (f *webFrontend) WindowPrint()                            { js.Global().Call("print") }

func (f *webFrontend) ScreenGetAll() ([]wruntime.Screen, error) {
	s := js.Global().Get("screen")
	w, h := s.Get("availWidth").Int(), s.Get("availHeight").Int()
	size := struct{ Width, Height int }{w, h}
	sc := wruntime.Screen{IsCurrent: true, IsPrimary: true, Width: w, Height: h}
	sc.Size.Width, sc.Size.Height = size.Width, size.Height
	sc.PhysicalSize = sc.Size
	return []wruntime.Screen{sc}, nil
}

func (f *webFrontend) MenuSetApplicationMenu(m *menu.Menu) {
	f.mu.Lock()
	f.menu = m
	f.mu.Unlock()
	f.MenuUpdateApplicationMenu()
}

// MenuUpdateApplicationMenu draws the menu bar of the page again from the menu Go has (the items, what is on and off, what is checked).
func (f *webFrontend) MenuUpdateApplicationMenu() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.menu == nil {
		return
	}
	f.items = map[int]*menu.MenuItem{}
	host("setMenu", toJSON(f.webMenu(f.menu)))
}

type webMenuItem struct {
	ID       int           `json:"id"`
	Label    string        `json:"label,omitempty"`
	Type     string        `json:"type"`
	Accel    string        `json:"accel,omitempty"`
	Disabled bool          `json:"disabled,omitempty"`
	Checked  bool          `json:"checked,omitempty"`
	Items    []webMenuItem `json:"items,omitempty"`
}

func (f *webFrontend) webMenu(m *menu.Menu) []webMenuItem {
	var out []webMenuItem
	for _, it := range m.Items {
		if it == nil || it.Hidden || it.Role != 0 {
			continue
		}
		id := len(f.items) + 1
		f.items[id] = it
		w := webMenuItem{ID: id, Label: it.Label, Type: string(it.Type), Disabled: it.Disabled, Checked: it.Checked}
		if a := it.Accelerator; a != nil {
			parts := []string{}
			for _, mod := range a.Modifiers {
				parts = append(parts, string(mod))
			}
			w.Accel = strings.Join(append(parts, a.Key), "+")
		}
		if it.SubMenu != nil {
			w.Type = string(menu.SubmenuType)
			w.Items = f.webMenu(it.SubMenu)
		}
		out = append(out, w)
	}
	return out
}

// menuClick is an item of the page's menu bar that was chosen.
func (f *webFrontend) menuClick(id int) {
	f.mu.Lock()
	it := f.items[id]
	f.mu.Unlock()
	if it == nil || it.Disabled {
		return
	}
	switch it.Label {
	case "About mormiView": // a window of its own on the desktop (another copy of the program): a dialog over the page here
		host("openAbout")
		return
	}
	if it.Type == menu.CheckboxType {
		it.Checked = !it.Checked
	}
	if it.Click != nil {
		it.Click(&menu.CallbackData{MenuItem: it})
	}
	if it.Type == menu.CheckboxType {
		f.MenuUpdateApplicationMenu()
	}
}

func (f *webFrontend) Notify(string, ...interface{}) {}
func (f *webFrontend) BrowserOpenURL(u string)       { host("openURL", u) }

func (f *webFrontend) ClipboardGetText() (string, error) {
	v, err := await(js.Global().Get("navigator").Get("clipboard").Call("readText"))
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func (f *webFrontend) ClipboardSetText(text string) error {
	_, err := await(js.Global().Get("navigator").Get("clipboard").Call("writeText", text))
	return err
}

func (f *webFrontend) InitializeNotifications() error                           { return nil }
func (f *webFrontend) CleanupNotifications()                                    {}
func (f *webFrontend) IsNotificationAvailable() bool                            { return false }
func (f *webFrontend) RequestNotificationAuthorization() (bool, error)          { return false, nil }
func (f *webFrontend) CheckNotificationAuthorization() (bool, error)            { return false, nil }
func (f *webFrontend) OnNotificationResponse(func(wruntime.NotificationResult)) {}
func (f *webFrontend) SendNotification(wruntime.NotificationOptions) error      { return nil }
func (f *webFrontend) SendNotificationWithActions(wruntime.NotificationOptions) error {
	return nil
}
func (f *webFrontend) RegisterNotificationCategory(wruntime.NotificationCategory) error { return nil }
func (f *webFrontend) RemoveNotificationCategory(string) error                          { return nil }
func (f *webFrontend) RemoveAllPendingNotifications() error                             { return nil }
func (f *webFrontend) RemovePendingNotification(string) error                           { return nil }
func (f *webFrontend) RemoveAllDeliveredNotifications() error                           { return nil }
func (f *webFrontend) RemoveDeliveredNotification(string) error                         { return nil }
func (f *webFrontend) RemoveNotification(string) error                                  { return nil }

// ---- The bindings -----------------------------------------------------------------------------------------------------

// callMethod calls the exported method name of a with the arguments the page sent (a JSON list), and returns its first result as JSON.
// Like Wails: a last result of type error that is not nil rejects the promise.
func callMethod(target reflect.Value, name, argsJSON string) (res string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s failed: %v", name, r)
		}
	}()
	m := target.MethodByName(name)
	if !m.IsValid() {
		return "", fmt.Errorf("there is no method %s", name)
	}
	var raw []json.RawMessage
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
			return "", err
		}
	}
	mt := m.Type()
	in := make([]reflect.Value, mt.NumIn())
	for i := range in {
		p := reflect.New(mt.In(i))
		if i < len(raw) && string(raw[i]) != "null" {
			if err := json.Unmarshal(raw[i], p.Interface()); err != nil {
				return "", fmt.Errorf("%s: argument %d: %v", name, i+1, err)
			}
		}
		in[i] = p.Elem()
	}
	out := m.Call(in)
	if n := len(out); n > 0 && mt.Out(n-1) == errType {
		if e := out[n-1]; !e.IsNil() {
			return "", e.Interface().(error)
		}
		out = out[:n-1]
	}
	if len(out) == 0 {
		return "null", nil
	}
	b, err := json.Marshal(out[0].Interface())
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func methodNames(t reflect.Type) []any {
	var names []any
	for i := 0; i < t.NumMethod(); i++ {
		names = append(names, t.Method(i).Name)
	}
	return names
}

// memResponse is a ResponseWriter that keeps what imageHandler writes.
type memResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (r *memResponse) Header() http.Header         { return r.header }
func (r *memResponse) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *memResponse) WriteHeader(code int)        { r.code = code }

func main() {
	app := NewApp()
	fe := &webFrontend{app: app}

	q, _ := url.ParseQuery(strings.TrimPrefix(js.Global().Get("location").Get("search").String(), "?"))
	if q.Has("about") {
		app.mode = "about"
	} else if p := q.Get("open"); p != "" {
		app.child = true
		if err := app.setImage(p); err != nil {
			js.Global().Get("console").Call("warn", "mormiview:", err.Error())
		}
	}

	ctx := context.WithValue(context.Background(), "frontend", fe)
	emitEvent = func(_ context.Context, name string, data ...interface{}) {
		host("emit", name, toJSON(data))
	}
	readClipboardImage = func() (image.Image, error) {
		v, err := hostAwait("clipboardImage")
		if err != nil || v.Type() != js.TypeObject {
			return nil, errNoClipboardPicture
		}
		b := make([]byte, v.Get("length").Int())
		js.CopyBytesToGo(b, v)
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, errNoClipboardPicture
		}
		return img, nil
	}
	app.openWindow = func(path string) error {
		_, err := hostAwait("openWindow", path)
		return err
	}

	target := reflect.ValueOf(app)
	js.Global().Set("mvCall", js.FuncOf(func(_ js.Value, args []js.Value) any {
		name, argsJSON, cb := args[0].String(), args[1].String(), args[2]
		go func() {
			res, err := callMethod(target, name, argsJSON)
			if err != nil {
				cb.Invoke(err.Error(), js.Null())
				return
			}
			cb.Invoke(js.Null(), res)
		}()
		return nil
	}))
	js.Global().Set("mvMethods", js.ValueOf(methodNames(target.Type())))

	// mvServe(path, cb): the picture of the window, as imageHandler serves it. cb(status, contentType, bytes).
	js.Global().Set("mvServe", js.FuncOf(func(_ js.Value, args []js.Value) any {
		path, cb := args[0].String(), args[1]
		go func() {
			req, _ := http.NewRequest("GET", path, nil)
			rw := &memResponse{header: http.Header{}, code: 200}
			app.imageHandler().ServeHTTP(rw, req)
			b := js.Global().Get("Uint8Array").New(rw.body.Len())
			js.CopyBytesToJS(b, rw.body.Bytes())
			cb.Invoke(rw.code, rw.header.Get("Content-Type"), b)
		}()
		return nil
	}))

	js.Global().Set("mvMenuClick", js.FuncOf(func(_ js.Value, args []js.Value) any {
		id := args[0].Int()
		go fe.menuClick(id)
		return nil
	}))

	// mvModified: whether the picture has changes that are not saved (the page asks before it is closed). It only reads.
	js.Global().Set("mvModified", js.FuncOf(func(js.Value, []js.Value) any {
		return app.current().Modified
	}))

	app.ctx = ctx
	if app.mode == "" {
		m := appMenu(app)
		app.menuBar = m
		fe.MenuSetApplicationMenu(m)
	}
	app.titleWindow()
	host("ready")
	select {}
}
