package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestParseColor(t *testing.T) {
	for in, want := range map[string]string{"": "#E53935", "red": "#E53935", "#abc": "#AABBCC", "#1e6fe0": "#1E6FE0", " Blue ": "#1E6FE0", "1E6FE0": "#1E6FE0"} {
		if got, err := parseColor(in, "red"); err != nil || got != want {
			t.Errorf("parseColor(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"reddish", "#12", "#gggggg", "#12345"} {
		if _, err := parseColor(bad, "red"); err == nil {
			t.Errorf("parseColor(%q) worked, want an error", bad)
		}
	}
}

func TestArrowPoints(t *testing.T) {
	p := arrowPoints(10, 100, 200, 100, 5) // to the right: the barbs lie behind the tip, above and below
	if len(p) != 5 || p[0] != [2]int{10, 100} || p[1] != [2]int{200, 100} || p[3] != [2]int{200, 100} {
		t.Fatalf("arrowPoints = %v", p)
	}
	if p[2][0] >= 200 || p[4][0] >= 200 || p[2][1] == 100 || p[2][1]+p[4][1] != 200 {
		t.Errorf("the barbs %v %v are not behind the tip and either side of the shaft", p[2], p[4])
	}
	if q := arrowPoints(5, 5, 5, 5, 3); len(q) != 2 { // no direction: just a dot
		t.Errorf("a zero-length arrow = %v", q)
	}
	if s := arrowPoints(0, 0, 10, 0, 3); 10-s[2][0] > 7 { // a short arrow's head is in proportion (60% of its length)
		t.Errorf("short arrow %v", s)
	}
}

// fakeWindow answers like the commands of a window and notes them.
type fakeWindow struct {
	mu    sync.Mutex
	cmds  []map[string]any
	size  []any
	extra map[string]any // more for the state: a selection, a polygon
}

func (f *fakeWindow) do(m map[string]any) (map[string]any, error) {
	f.mu.Lock()
	f.cmds = append(f.cmds, m)
	f.mu.Unlock()
	switch m["cmd"] {
	case "state":
		st := map[string]any{"ok": true, "name": "a.png", "size": f.size, "modified": false, "tool": "select"}
		for k, v := range f.extra {
			st[k] = v
		}
		return st, nil
	case "mask":
		return map[string]any{"ok": true, "mask": "m1", "pixels": 12, "box": []int{1, 2, 3, 4}, "picture": []int{20, 10}}, nil
	case "snapshot":
		return map[string]any{"ok": true, "png": "AAAA", "width": 10, "height": 5, "scale": 0.5, "region": []int{0, 0, 20, 10}, "picture": []int{20, 10}}, nil
	case "pixel":
		return map[string]any{"ok": true, "pixel": map[string]any{"hex": "#112233"}}, nil
	}
	return map[string]any{"ok": true}, nil
}

func (f *fakeWindow) sent() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.cmds...)
}

// testHub is the server of the tools of one window that is a fake (the ids are not used any more).
func testHub(t *testing.T, ids ...string) (*chatHub, []*fakeWindow) {
	f := &fakeWindow{size: []any{1000.0, 500.0}}
	return &chatHub{token: "secret", do: f.do}, []*fakeWindow{f}
}

func rpc(t *testing.T, h *chatHub, token, body string) (int, map[string]any) {
	req := httptest.NewRequest("POST", "http://127.0.0.1:1/mcp", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.handler().ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHubAuth(t *testing.T) {
	h, _ := testHub(t, "aaaa")
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	if code, _ := rpc(t, h, "", body); code != 401 {
		t.Errorf("no token: %d, want 401", code)
	}
	if code, _ := rpc(t, h, "wrong", body); code != 401 {
		t.Errorf("wrong token: %d, want 401", code)
	}
	if code, _ := rpc(t, h, "secret", body); code != 200 {
		t.Errorf("right token: %d, want 200", code)
	}
	req := httptest.NewRequest("POST", "http://127.0.0.1:1/mcp", strings.NewReader(body)) // a web page
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.handler().ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("with an Origin: %d, want 403", rec.Code)
	}
	req = httptest.NewRequest("POST", "http://evil.example:1/mcp", strings.NewReader(body)) // DNS rebinding
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.handler().ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("another host: %d, want 403", rec.Code)
	}
	h.token = "" // no token at all: nothing gets in
	if code, _ := rpc(t, h, "", body); code != 401 {
		t.Errorf("an empty token: %d, want 401", code)
	}
}

func TestHubProtocol(t *testing.T) {
	h, _ := testHub(t, "aaaa")
	_, res := rpc(t, h, "secret", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	r, _ := res["result"].(map[string]any)
	if r["protocolVersion"] != "2025-03-26" || r["serverInfo"].(map[string]any)["name"] != "mormiview" || r["instructions"] == "" {
		t.Errorf("initialize = %v", res)
	}
	_, res = rpc(t, h, "secret", `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	if res["result"].(map[string]any)["protocolVersion"] != mcpVersions[0] {
		t.Errorf("an unknown version must get the newest: %v", res)
	}
	if code, _ := rpc(t, h, "secret", `{"jsonrpc":"2.0","method":"notifications/initialized"}`); code != 202 {
		t.Errorf("a notification: %d, want 202", code)
	}
	_, res = rpc(t, h, "secret", `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	names := map[string]bool{}
	for _, tl := range res["result"].(map[string]any)["tools"].([]any) {
		m := tl.(map[string]any)
		names[m["name"].(string)] = true
		if m["inputSchema"].(map[string]any)["type"] != "object" || m["description"] == "" {
			t.Errorf("tool %v has no proper schema or description", m["name"])
		}
	}
	for _, n := range []string{"window_info", "snapshot", "draw_arrow", "draw_rect", "add_text", "flood_fill", "select_region", "copy", "paste", "save_image", "open_image"} {
		if !names[n] {
			t.Errorf("tool %s is missing", n)
		}
	}
	_, res = rpc(t, h, "secret", `{"jsonrpc":"2.0","id":4,"method":"nope"}`)
	if res["error"].(map[string]any)["code"] != -32601.0 {
		t.Errorf("unknown method = %v", res)
	}
	_, res = rpc(t, h, "secret", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"draw_arrow","arguments":{}}}`)
	if res["result"].(map[string]any)["isError"] != true { // missing arguments are an answer for the model, not a protocol error
		t.Errorf("a failing tool must say isError: %v", res)
	}
}

func call(t *testing.T, h *chatHub, name string, args string) (map[string]any, bool) {
	_, res := rpc(t, h, "secret", `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`)
	r := res["result"].(map[string]any)
	return r, r["isError"] == true
}

func firstText(r map[string]any) string {
	for _, c := range r["content"].([]any) {
		if m := c.(map[string]any); m["type"] == "text" {
			return m["text"].(string)
		}
	}
	return ""
}

func TestHubDrawArrow(t *testing.T) {
	h, fws := testHub(t, "aaaa")
	r, bad := call(t, h, "draw_arrow", `{"x1":100,"y1":100,"x2":400,"y2":100,"color":"blue","width":6}`)
	if bad {
		t.Fatalf("draw_arrow failed: %s", firstText(r))
	}
	sent := fws[0].sent()
	var shape map[string]any
	for _, c := range sent {
		switch c["cmd"] {
		case "shape":
			shape = c
		case "drag", "click", "move", "tool":
			t.Errorf("a draw tool must not use the pointer: %v", c)
		}
	}
	if shape["kind"] != "path" || shape["color"] != "#1E6FE0" {
		t.Errorf("shape %v", shape)
	}
	if pts, _ := shape["pts"].([]any); len(pts) != 5 {
		t.Errorf("the arrow has the points %v", shape["pts"])
	}
	// outside the picture: refused before anything is drawn
	n := len(fws[0].sent())
	if r, bad := call(t, h, "draw_arrow", `{"x1":100,"y1":100,"x2":4000,"y2":100}`); !bad || !strings.Contains(firstText(r), "outside the image") {
		t.Errorf("an arrow out of the picture: %v", r)
	}
	for _, c := range fws[0].sent()[n:] {
		if c["cmd"] == "shape" {
			t.Errorf("a shape was sent for a position outside the picture")
		}
	}
	// defaults: the width follows the size of the picture (1000 px: 4), the color is red
	fws[0].mu.Lock()
	fws[0].cmds = nil
	fws[0].mu.Unlock()
	call(t, h, "draw_rect", `{"x":10,"y":10,"w":100,"h":50,"fill_color":"#ff0"}`)
	var rect map[string]any
	for _, c := range fws[0].sent() {
		if c["cmd"] == "shape" {
			rect = c
		}
	}
	if rect["width"] != 4.0 || rect["fill"] != "both" || rect["color"] != "#E53935" || rect["kind"] != "rect" {
		t.Errorf("rect shape = %v", rect)
	}
}

func TestHubSnapshotAndGuards(t *testing.T) {
	h, _ := testHub(t, "aaaa")
	r, bad := call(t, h, "snapshot", `{}`)
	if bad {
		t.Fatal(firstText(r))
	}
	c := r["content"].([]any)
	if c[0].(map[string]any)["type"] != "image" || c[0].(map[string]any)["mimeType"] != "image/png" || !strings.Contains(firstText(r), "scaled down by 0.500") {
		t.Errorf("snapshot = %v", r)
	}
	path := filepath.Join(t.TempDir(), "x.png")
	os.WriteFile(path, []byte("x"), 0o644)
	if r, bad := call(t, h, "save_image", `{"path":`+strconv.Quote(path)+`}`); !bad || !strings.Contains(firstText(r), "overwrite=true") {
		t.Errorf("saving over a file must be refused: %v", r)
	}
}

func TestSnapshot(t *testing.T) {
	a := NewApp()
	if _, err := a.snapshot(map[string]any{}); err == nil {
		t.Error("a window with no picture must say so")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 400, 200))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	if err := a.setClip(img); err != nil {
		t.Fatal(err)
	}
	dims := func(m map[string]any) (int, int) {
		raw, _ := base64.StdEncoding.DecodeString(m["png"].(string))
		p, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		return p.Bounds().Dx(), p.Bounds().Dy()
	}
	res, err := a.snapshot(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := dims(res); w != 400 || h != 200 || res["scale"] != 1.0 {
		t.Errorf("whole picture = %d × %d, scale %v", w, h, res["scale"])
	}
	res, _ = a.snapshot(map[string]any{"max": 100.0})
	if w, h := dims(res); w != 100 || h != 50 {
		t.Errorf("max 100 = %d × %d, want 100 × 50", w, h)
	}
	res, _ = a.snapshot(map[string]any{"x": 300.0, "y": 100.0, "w": 500.0, "h": 500.0}) // runs over the edge: cut to the picture
	if w, h := dims(res); w != 100 || h != 100 {
		t.Errorf("a part at the edge = %d × %d, want 100 × 100", w, h)
	}
	if r := res["region"].([]int); r[0] != 300 || r[1] != 100 || r[2] != 100 || r[3] != 100 {
		t.Errorf("region = %v", r)
	}
	if _, err := a.snapshot(map[string]any{"x": 900.0, "y": 0.0, "w": 10.0, "h": 10.0}); err == nil {
		t.Error("a part outside the picture must be an error")
	}
}

func TestParseColorAlpha(t *testing.T) {
	for in, want := range map[string]string{"transparent": "#00000000", "red": "#E53935FF", "#ff0000": "#FF0000FF", "#ff000080": "#FF000080", "#f008": "#FF000088", "": "#FFFFFFFF"} {
		if got, err := parseColorAlpha(in, "white"); err != nil || got != want {
			t.Errorf("parseColorAlpha(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := parseColorAlpha("nope", ""); err == nil {
		t.Error("a bad color must be an error")
	}
}

func TestHubSetPixelsGrid(t *testing.T) {
	h, fws := testHub(t, "aaaa") // the fake window's picture is 1000 × 500
	r, bad := call(t, h, "set_pixels", `{"grid":["R.", " G"],"palette":{"R":"#f00",".":"transparent","G":"green"},"x":10,"y":20}`)
	if bad {
		t.Fatal(firstText(r))
	}
	var list []any
	for _, c := range fws[0].sent() {
		if c["cmd"] == "pixels" {
			list = c["list"].([]any)
		}
	}
	if len(list) != 3 { // the space is left as it is
		t.Fatalf("pixels sent: %v", list)
	}
	first := list[0].([]any)
	last := list[2].([]any)
	if first[0] != 10.0 || first[1] != 20.0 || first[2] != "#FF0000FF" || last[0] != 11.0 || last[1] != 21.0 || last[2] != "#2E9E44FF" {
		t.Errorf("pixels = %v", list)
	}
	for _, args := range []string{
		`{"grid":["X"],"palette":{"R":"red"}}`,          // a character that is not in the palette
		`{"grid":["R"],"palette":{"R":"red"},"x":1000}`, // outside the picture
		`{"pixels":[{"x":-1,"y":0,"color":"red"}]}`,     // outside the picture
		`{"pixels":[{"x":1,"y":0}]}`,                    // no color
		`{}`,                                            // nothing
	} {
		if r, bad := call(t, h, "set_pixels", args); !bad {
			t.Errorf("set_pixels %s must fail, got %v", args, r)
		}
	}
	if r, bad := call(t, h, "set_pixels", `{"pixels":[{"x":1,"y":2,"color":"#00f"},{"x":3,"y":4,"color":"transparent"}]}`); bad {
		t.Errorf("pixels form: %v", firstText(r))
	}
}

func TestHubNewImage(t *testing.T) {
	h, fws := testHub(t, "aaaa")
	if r, bad := call(t, h, "new_image", `{"width":20,"height":30,"background":"transparent"}`); bad {
		t.Fatal(firstText(r))
	}
	var n map[string]any
	for _, c := range fws[0].sent() {
		if c["cmd"] == "new" {
			n = c
		}
	}
	if n["w"] != 20.0 || n["h"] != 30.0 || n["color"] != "#00000000" {
		t.Errorf("new = %v", n)
	}
	if r, bad := call(t, h, "new_image", `{"width":20}`); !bad {
		t.Errorf("no height must fail: %v", r)
	}
}

func TestPaintPixelsAndGrid(t *testing.T) {
	a := NewApp()
	if err := a.NewBlankColor(4, 3, "#00000000"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PaintPixels(0, `[[0,0,"#FF0000FF"],[3,2,"#00FF0080"]]`); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	img := a.clip.(*image.NRGBA)
	a.mu.Unlock()
	if c := img.NRGBAAt(0, 0); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("(0,0) = %v", c)
	}
	if c := img.NRGBAAt(3, 2); c != (color.NRGBA{0, 255, 0, 128}) {
		t.Errorf("(3,2) = %v (alpha must be kept, not blended)", c)
	}
	if c := img.NRGBAAt(1, 1); c.A != 0 {
		t.Errorf("(1,1) = %v, want transparent", c)
	}
	if _, err := a.PaintPixels(0, `[[4,0,"#FF0000FF"]]`); err == nil {
		t.Error("a pixel outside the picture must be an error")
	}
	if _, err := a.PaintPixels(0, `[[0,0,"red"]]`); err == nil {
		t.Error("a color that is not #rrggbbaa must be an error")
	}
	// it is one step of the history, and the grid tells the picture back
	res, err := a.snapshot(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	g := res["grid"].(map[string]any)
	rows := g["rows"].([]string)
	pal := g["palette"].(map[string]string)
	if len(rows) != 3 || rows[0] != "a..." || rows[1] != "...." || rows[2] != "...b" || pal["a"] != "#FF0000" || pal["b"] != "#00FF0080" || pal["."] != "transparent" {
		t.Errorf("grid = %v %v", rows, pal)
	}
	if err := a.NewBlankColor(0, 5, "#FFFFFFFF"); err == nil {
		t.Error("a picture 0 pixels wide must be an error")
	}
	if err := a.NewBlankColor(5, 5, "white"); err == nil {
		t.Error("a color that is not #rrggbbaa must be an error")
	}
}

func TestHubResizeCanvasAndCrop(t *testing.T) {
	h, fws := testHub(t, "aaaa")
	if r, bad := call(t, h, "resize_canvas", `{"width":60,"height":30,"anchor":"top-left","background":"transparent"}`); bad {
		t.Fatalf("resize_canvas failed: %s", firstText(r))
	}
	if r, bad := call(t, h, "crop", `{"x":1,"y":2,"w":3,"h":4}`); bad {
		t.Fatalf("crop failed: %s", firstText(r))
	}
	var resize, crop map[string]any
	for _, c := range fws[0].sent() {
		switch c["cmd"] {
		case "resize":
			resize = c
		case "crop":
			crop = c
		case "drag", "click", "move":
			t.Errorf("the canvas tools must not use the pointer: %v", c)
		}
	}
	if resize["w"] != 60.0 || resize["h"] != 30.0 || resize["anchor"] != "top-left" || resize["bg"] != "#00000000" {
		t.Errorf("resize = %v", resize)
	}
	if crop["x"] != 1.0 || crop["w"] != 3.0 || crop["h"] != 4.0 {
		t.Errorf("crop = %v", crop)
	}
	if r, bad := call(t, h, "resize_canvas", `{"width":60,"height":30,"anchor":"sideways"}`); !bad || !strings.Contains(firstText(r), "anchor") {
		t.Errorf("a bad anchor: %v", r)
	}
}

func TestHubRetouchTools(t *testing.T) {
	h, fws := testHub(t, "aaaa")
	for _, c := range []struct{ tool, args string }{
		{"clone_region", `{"x":1,"y":2,"w":30,"h":20,"to_x":100,"to_y":50,"flip_h":true,"feather":5,"opacity":0.8}`},
		{"clone_region", `{"polygon":[[1,1],[20,1],[10,15]],"to_x":60,"to_y":60,"rotate":90}`},
		{"blend_edges", `{"x":100,"y":50,"w":30,"h":20,"method":"poisson","width":10}`},
		{"fill_region", `{"polygon":[[5,5],[50,5],[50,40],[5,40]],"patch":11}`},
	} {
		if r, bad := call(t, h, c.tool, c.args); bad {
			t.Fatalf("%s %s failed: %s", c.tool, c.args, firstText(r))
		}
	}
	var cmds []map[string]any
	for _, c := range fws[0].sent() {
		switch c["cmd"] {
		case "retouch":
			cmds = append(cmds, c)
		case "drag", "click", "move":
			t.Errorf("the retouching tools must not use the pointer: %v", c)
		}
	}
	if len(cmds) != 4 {
		t.Fatalf("%d retouch commands, want 4", len(cmds))
	}
	if c := cmds[0]; c["op"] != "clone" || c["tx"] != 100.0 || c["ty"] != 50.0 || c["flip_h"] != true || c["feather"] != 5.0 || c["opacity"] != 0.8 || c["w"] != 30.0 {
		t.Errorf("clone = %v", c)
	}
	if c := cmds[1]; c["rotate"] != 90.0 || c["pts"] == nil || c["w"] != nil {
		t.Errorf("clone with a polygon = %v", c)
	}
	if c := cmds[2]; c["op"] != "blend" || c["method"] != "poisson" || c["width"] != 10.0 {
		t.Errorf("blend = %v", c)
	}
	if c := cmds[3]; c["op"] != "fill" || c["patch"] != 11.0 {
		t.Errorf("fill = %v", c)
	}
	if r, bad := call(t, h, "clone_region", `{"to_x":1,"to_y":1}`); !bad || !strings.Contains(firstText(r), "box") {
		t.Errorf("a clone without a shape: %v", r)
	}
	if r, bad := call(t, h, "fill_region", `{"polygon":[[1,1],[2,2]]}`); !bad || !strings.Contains(firstText(r), "three points") {
		t.Errorf("a polygon of two points: %v", r)
	}
}

func TestHubMaskAndJudgeTools(t *testing.T) {
	h, fws := testHub(t, "aaaa")
	for _, c := range []struct{ tool, args string }{
		{"select_by_color", `{"x":10,"y":20,"tolerance":25,"grow":2,"contiguous":false,"fill_holes":false}`},
		{"create_mask", `{"polygon":[[1,1],[20,1],[10,15]]}`},
		{"edit_mask", `{"mask":"m1","add_mask":"m2","invert":true,"shrink":1}`},
		{"seam_score", `{"mask":"m1","width":8}`},
		{"clone_region", `{"mask":"m1","to_x":5,"to_y":6,"scale":1.5,"angle":20,"blend":"seamless"}`},
		{"fill_region", `{"mask":"m1","source_mask":"m2","avoid_mask":"m3"}`},
		{"blend_edges", `{"mask":"m1","method":"poisson"}`},
		{"snapshot", `{"mask":"m1","compare":true,"zoom":4}`},
	} {
		if r, bad := call(t, h, c.tool, c.args); bad {
			t.Fatalf("%s %s failed: %s", c.tool, c.args, firstText(r))
		}
	}
	got := map[string]map[string]any{}
	var retouch []map[string]any
	for _, c := range fws[0].sent() {
		switch c["cmd"] {
		case "mask", "seam", "snapshot":
			key := c["cmd"].(string)
			if op, _ := c["op"].(string); op != "" {
				key += ":" + op
			}
			got[key] = c
		case "retouch":
			retouch = append(retouch, c)
		case "drag", "click", "move":
			t.Errorf("no pointer: %v", c)
		}
	}
	if c := got["mask:color"]; c["tolerance"] != 25.0 || c["grow"] != 2.0 || c["contiguous"] != false || c["fill_holes"] != false {
		t.Errorf("select_by_color = %v", c)
	}
	if c := got["mask:create"]; c["pts"] == nil {
		t.Errorf("create_mask = %v", c)
	}
	if c := got["mask:edit"]; c["add_mask"] != "m2" || c["invert"] != true || c["shrink"] != 1.0 {
		t.Errorf("edit_mask = %v", c)
	}
	if c := got["seam"]; c["mask"] != "m1" || c["width"] != 8.0 {
		t.Errorf("seam_score = %v", c)
	}
	if c := got["snapshot"]; c["mask"] != "m1" || c["compare"] != true || c["zoom"] != 4.0 {
		t.Errorf("snapshot = %v", c)
	}
	if len(retouch) != 3 {
		t.Fatalf("%d retouch commands", len(retouch))
	}
	if c := retouch[0]; c["mask"] != "m1" || c["scale"] != 1.5 || c["angle"] != 20.0 || c["blend"] != "seamless" || c["x"] != nil {
		t.Errorf("clone = %v", c)
	}
	if c := retouch[1]; c["source_mask"] != "m2" || c["avoid_mask"] != "m3" {
		t.Errorf("fill = %v", c)
	}
}

func TestHubSelectionTools(t *testing.T) {
	h, fws := testHub(t)
	if r, bad := call(t, h, "selection_mask", `{}`); !bad || !strings.Contains(firstText(r), "nothing is selected") {
		t.Errorf("nothing selected: %v", firstText(r))
	}
	fws[0].extra = map[string]any{"selection": map[string]any{"x": 3, "y": 4, "w": 20, "h": 10}}
	if r, bad := call(t, h, "selection_mask", `{}`); bad || !strings.Contains(firstText(r), "Mask m1") || !strings.Contains(firstText(r), "rectangle") {
		t.Errorf("the rectangle: %v", firstText(r))
	}
	fws[0].extra = map[string]any{"selection": map[string]any{"x": 3, "y": 4, "w": 20, "h": 10}, "polygon": []any{[]any{1, 1}, []any{9, 1}, []any{5, 8}}}
	r, bad := call(t, h, "selection_mask", `{}`)
	if bad || !strings.Contains(firstText(r), "polygon of 3 points") || !strings.Contains(firstText(r), "[[1,1],[9,1],[5,8]]") {
		t.Errorf("the polygon wins over the rectangle, and its corners are told: %v", firstText(r))
	}
	images := 0
	for _, c := range r["content"].([]any) {
		if c.(map[string]any)["type"] == "image" {
			images++
		}
	}
	if images != 2 {
		t.Errorf("the selection comes with %d pictures, want 2 (the whole and the part)", images)
	}
	if r, bad := call(t, h, "select_polygon", `{"polygon":[[1,1],[9,1],[5,8]]}`); bad {
		t.Errorf("select_polygon: %v", firstText(r))
	}
	if r, bad := call(t, h, "select_polygon", `{"polygon":[]}`); bad || !strings.Contains(firstText(r), "let go") {
		t.Errorf("select_polygon with no points lets go: %v", firstText(r))
	}
	if _, bad := call(t, h, "select_polygon", `{"polygon":[[1,1],[2,2]]}`); !bad {
		t.Errorf("a polygon of two points must fail")
	}
	if r, bad := call(t, h, "discard_changes", `{}`); bad {
		t.Errorf("discard_changes: %v", firstText(r))
	}
	var polys, discards, masks, snaps int
	for _, c := range fws[0].sent() {
		switch c["cmd"] {
		case "snapshot":
			snaps++
			if c["mask"] != "m1" || c["outline"] == nil {
				t.Errorf("a snapshot of the selection must show the mask and the outline: %v", c)
			}
		case "polygon":
			polys++
		case "menu":
			if c["name"] == "discard" {
				discards++
			}
		case "mask":
			masks++
			if c["op"] != "create" || (c["pts"] == nil && c["w"] == nil) {
				t.Errorf("mask command = %v", c)
			}
		}
	}
	if snaps != 4 { // (two selections, each shown whole and in detail)
		t.Errorf("%d snapshots of the selection, want 4", snaps)
	}
	if polys != 2 || discards != 1 || masks != 2 {
		t.Errorf("commands: %d polygon, %d discard, %d mask", polys, discards, masks)
	}
}

func TestToolNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, tl := range mcpTools() {
		if seen[tl.Name] {
			t.Errorf("the tool %s is there twice", tl.Name)
		}
		seen[tl.Name] = true
	}
	for _, want := range []string{"apply_effect", "resize_image", "rotate_image", "flip_image", "place_image", "draw_polygon", "draw_callout", "clear_selection", "diff_original", "list_folder", "list_fonts", "goto_step", "checkpoint", "goto_checkpoint", "show_area", "image_info", "history", "discard_changes", "animation_start", "add_frame", "duplicate_frame", "delete_frame", "move_frame", "set_frame_delay", "animation_info", "frames_sheet"} {
		if !seen[want] {
			t.Errorf("no tool %s", want)
		}
	}
}
