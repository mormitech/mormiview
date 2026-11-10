package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// The picture tools of the chat panel (see chat.go): Claude Code, started by the panel, reaches the picture of this window through MCP (the Model Context
// Protocol) with tools like snapshot, draw_arrow and clone_region. The server is private: it listens on 127.0.0.1 on a free port while the panel works,
// with a token that exists only in this program and in a temporary file (mode 0600) for the length of one answer. There is no setting, no other program
// can be given access, and it serves this window only. Each tool call becomes commands for the window (control.go, without the dialog queue).
//
// The tools work in pixels of the picture as it is shown, like control mode.

const mcpServerName = "mormiview"

// ---- The snapshot -------------------------------------------------------------------------------------------

// snapshot is the picture of this window as a PNG, the way it is shown (turned as the view is). m: x, y, w, h for a part of it, max for the
// longest side of the answer (1568 by default; a bigger part is scaled down, a smaller one is not scaled up).
func (a *App) snapshot(m map[string]any) (map[string]any, error) {
	a.mu.Lock()
	turn := a.turn
	a.mu.Unlock()
	img, err := a.shownPixels(turn)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, fmt.Errorf("there is no image in this window")
	}
	full := img.Bounds()
	r := full
	if _, ok := m["w"]; ok {
		x, y, w, h := num(m, "x", 0), num(m, "y", 0), num(m, "w", full.Dx()), num(m, "h", full.Dy())
		r = image.Rect(x, y, x+w, y+h).Intersect(full)
		if r.Empty() {
			return nil, fmt.Errorf("the part %d,%d %d×%d is outside the image (%d × %d)", x, y, w, h, full.Dx(), full.Dy())
		}
	}
	var md *maskData
	if id := str(m, "mask"); id != "" { // the area is tinted in the answer only
		if md, err = a.masks.get(id, full.Dx(), full.Dy()); err != nil {
			return nil, err
		}
	}
	maxSide := max(num(m, "max", 1568), 16)
	compare := m["compare"] == true
	var orig *image.NRGBA
	if compare { // the original file next to the picture as it is now
		if orig, err = a.originalPixels(turn); err != nil {
			return nil, err
		}
		if orig.Bounds().Dx() != full.Dx() || orig.Bounds().Dy() != full.Dy() {
			return nil, fmt.Errorf("the image has another size than the file (%d × %d, the file is %d × %d), so the two cannot be put side by side", full.Dx(), full.Dy(), orig.Bounds().Dx(), orig.Bounds().Dy())
		}
		maxSide = max(maxSide/2, 16)
	}
	out, scale, zoom := renderPart(img, r, maxSide, num(m, "zoom", 1), m["outline"], md)
	if compare {
		left, _, _ := renderPart(orig, r, maxSide, num(m, "zoom", 1), nil, nil)
		const gap = 6
		both := image.NewNRGBA(image.Rect(0, 0, left.Bounds().Dx()+gap+out.Bounds().Dx(), max(left.Bounds().Dy(), out.Bounds().Dy())))
		draw.Draw(both, both.Bounds(), image.NewUniform(color.NRGBA{255, 0, 255, 255}), image.Point{}, draw.Src)
		draw.Draw(both, image.Rect(0, 0, left.Bounds().Dx(), left.Bounds().Dy()), left, image.Point{}, draw.Src)
		draw.Draw(both, image.Rect(left.Bounds().Dx()+gap, 0, both.Bounds().Dx(), out.Bounds().Dy()), out, image.Point{}, draw.Src)
		out = both
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, out); err != nil {
		return nil, err
	}
	b := out.Bounds()
	var grid map[string]any
	if r.Dx()*r.Dy() <= 4096 && !compare { // a small picture (a sprite) is also told in text, pixel for pixel
		grid = pixelGrid(img, r)
	}
	return map[string]any{
		"grid": grid, "ok": true, "png": base64.StdEncoding.EncodeToString(buf.Bytes()), "width": b.Dx(), "height": b.Dy(), "scale": scale, "zoom": zoom, "compare": compare,
		"region": []int{r.Min.X, r.Min.Y, r.Dx(), r.Dy()}, "picture": []int{full.Dx(), full.Dy()},
	}, nil
}

// renderPart is the part r of img for a snapshot: scaled down to maxSide, or enlarged in whole pixels (zoomWant, 2 to 32) when it is small; the area of the mask
// is tinted, and the shape of outline ([[x, y], …]) is drawn over it in magenta.
func renderPart(img *image.NRGBA, r image.Rectangle, maxSide, zoomWant int, outline any, md *maskData) (*image.NRGBA, float64, int) {
	part := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(part, part.Bounds(), img, r.Min, draw.Src)
	if md != nil {
		for y := 0; y < r.Dy(); y++ {
			for x := 0; x < r.Dx(); x++ {
				if md.at(r.Min.X+x, r.Min.Y+y) {
					c := part.NRGBAAt(x, y)
					part.SetNRGBA(x, y, color.NRGBA{lerp8(c.R, 255, 0.45), lerp8(c.G, 0, 0.45), lerp8(c.B, 255, 0.45), c.A})
				}
			}
		}
	}
	out := part
	scale, zoom := 1.0, 1
	if long := max(r.Dx(), r.Dy()); long > maxSide {
		scale = float64(maxSide) / float64(long)
		dst := image.NewNRGBA(image.Rect(0, 0, max(1, int(float64(r.Dx())*scale+0.5)), max(1, int(float64(r.Dy())*scale+0.5))))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), part, part.Bounds(), xdraw.Src, nil)
		out = dst
	} else if z := min(max(zoomWant, 1), 32); z > 1 { // a small part, enlarged in whole pixels (nearest neighbour) so that single pixels can be seen
		if zoom = min(z, maxSide/max(r.Dx(), r.Dy())); zoom > 1 {
			big := image.NewNRGBA(image.Rect(0, 0, r.Dx()*zoom, r.Dy()*zoom))
			for y := 0; y < big.Rect.Dy(); y++ {
				for x := 0; x < big.Rect.Dx(); x++ {
					big.SetNRGBA(x, y, part.NRGBAAt(x/zoom, y/zoom))
				}
			}
			out = big
		} else {
			zoom = 1
		}
	}
	if raw, ok := outline.([]any); ok && len(raw) >= 2 {
		f := float64(out.Bounds().Dx()) / float64(r.Dx())
		var pts [][2]int
		for _, p := range raw {
			if xy, _ := p.([]any); len(xy) == 2 {
				x, _ := xy[0].(float64)
				y, _ := xy[1].(float64)
				pts = append(pts, [2]int{int((x - float64(r.Min.X) + 0.5) * f), int((y - float64(r.Min.Y) + 0.5) * f)})
			}
		}
		for i := range pts {
			a, c := pts[i], pts[(i+1)%len(pts)]
			walk(a[0], a[1], c[0], c[1], func(x, y int) {
				for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
					if p := image.Pt(x+d[0], y+d[1]); p.In(out.Bounds()) {
						out.SetNRGBA(p.X, p.Y, color.NRGBA{255, 0, 255, 255})
					}
				}
			})
		}
	}
	return out, scale, zoom
}

// originalPixels is the file of the picture as it is on the disk (turned the way the window shows it), for the comparison with what has been done to it.
func (a *App) originalPixels(turn int) (*image.NRGBA, error) {
	path := a.current().Path
	if path == "" {
		return nil, fmt.Errorf("this image has no file, so there is no original to compare with")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("the original cannot be read: %v", err)
	}
	orient := 1
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".jpe", ".jfif":
		orient = exifOrientation(path)
	}
	return turnImage(shownImage(pixelCache{img: img, orient: orient}), turn), nil
}

// ---- The server: MCP over HTTP -------------------------------------------------------------------------------

// chatHub is the MCP server of one window. do gives a command to the window and returns its answer (an "error" in the answer is an error here).
type chatHub struct {
	token  string
	do     func(cmd map[string]any) (map[string]any, error)
	mu     sync.Mutex // one tool call at a time works on the window
	framed bool       // the tool that runs works on a frame it was asked for (see applyFrame)
}

func (h *chatHub) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", h.guard(h.serveMCP))
	return mux
}

// guard lets a request through when it has the token, comes to the address of this program and was not made by a web page.
func (h *chatHub) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if hh, _, err := net.SplitHostPort(host); err == nil {
			host = hh
		}
		if r.Header.Get("Origin") != "" || (host != "127.0.0.1" && host != "localhost") {
			http.Error(w, "refused", http.StatusForbidden)
			return
		}
		if h.token == "" || r.Header.Get("Authorization") != "Bearer "+h.token {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mormiview"`)
			http.Error(w, "token missing or wrong", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// window gives one command to the window and returns its answer.
func (h *chatHub) window(cmd map[string]any) (map[string]any, error) {
	// the commands and the answers are JSON, as they are in control mode: numbers are float64, lists are []any (the commands of the window read them so)
	var in map[string]any
	b, _ := json.Marshal(cmd)
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, err
	}
	out, err := h.do(in)
	if err != nil {
		return nil, err
	}
	var res map[string]any
	b, _ = json.Marshal(out)
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	if e, ok := res["error"].(string); ok && e != "" {
		return nil, fmt.Errorf("%s", e)
	}
	return res, nil
}

// ---- JSON-RPC ---------------------------------------------------------------------------------------------

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

var mcpVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const mcpInstructions = `mormiView is an image viewer and editor, and you work on the image of the window the user is looking at. All positions are pixels of the image as it is shown: x to the right, y down, 0,0 is the top left pixel. Look first (get_image gives the whole image pixel for pixel, and a small one also as text; for a big photo, snapshot scales it down and can show a part), then draw, then look again to check. To make pixel art: new_image, then set_pixels with a grid and a palette. Every drawing is one step of the window's undo history. To remove something from a photo, work as a retoucher in rounds: draw a polygon around it (snapshot with outline and zoom shows it before anything changes), cover it with clone_region from a clean place that looks like what belongs there (feather the edge), look again, hide seams with blend_edges (poisson for different brightness, color_match for a color cast, smooth for a hard line), use fill_region for small leftovers, and check each round with snapshot. The images are changed in memory only until save_image is called.`

func (h *chatHub) serveMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "unreadable", http.StatusBadRequest)
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) > 0 && body[0] == '[' { // a batch
		var reqs []rpcRequest
		if json.Unmarshal(body, &reqs) != nil {
			writeJSON(w, 200, rpcError(nil, -32700, "parse error"))
			return
		}
		var out []any
		for _, q := range reqs {
			if res := h.dispatch(q); res != nil {
				out = append(out, res)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, 200, out)
		return
	}
	var q rpcRequest
	if json.Unmarshal(body, &q) != nil {
		writeJSON(w, 200, rpcError(nil, -32700, "parse error"))
		return
	}
	res := h.dispatch(q)
	if res == nil { // a notification, or an answer from the client
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, 200, res)
}

func rpcError(id json.RawMessage, code int, msg string) map[string]any {
	var rid any
	if len(id) > 0 {
		rid = id
	}
	return map[string]any{"jsonrpc": "2.0", "id": rid, "error": map[string]any{"code": code, "message": msg}}
}

func rpcResult(id json.RawMessage, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}
}

// dispatch answers one request; it returns nil for what needs no answer.
func (h *chatHub) dispatch(q rpcRequest) map[string]any {
	if len(q.ID) == 0 || string(q.ID) == "null" { // a notification (initialized, cancelled…)
		return nil
	}
	if q.Method == "" { // a response to a request of ours: there are none
		return nil
	}
	switch q.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      struct {
				Name string `json:"name"`
			} `json:"clientInfo"`
		}
		json.Unmarshal(q.Params, &p)
		ver := mcpVersions[0]
		for _, v := range mcpVersions {
			if v == p.ProtocolVersion {
				ver = v
			}
		}
		return rpcResult(q.ID, map[string]any{
			"protocolVersion": ver,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": mcpServerName, "title": "mormiView", "version": version()},
			"instructions":    mcpInstructions,
		})
	case "ping":
		return rpcResult(q.ID, map[string]any{})
	case "tools/list":
		return rpcResult(q.ID, map[string]any{"tools": mcpTools()})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(q.Params, &p) != nil || p.Name == "" {
			return rpcError(q.ID, -32602, "tools/call needs a name")
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		content, err := h.callTool(p.Name, p.Arguments)
		if err != nil { // a failure of the tool is an answer the model can read, not a protocol error
			return rpcResult(q.ID, map[string]any{"content": []any{textContent(err.Error())}, "isError": true})
		}
		return rpcResult(q.ID, map[string]any{"content": content})
	}
	return rpcError(q.ID, -32601, "method not found: "+q.Method)
}

func textContent(s string) map[string]any { return map[string]any{"type": "text", "text": s} }

func jsonText(v any) map[string]any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return textContent(string(b))
}

// ---- The tools --------------------------------------------------------------------------------------------

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func prop(typ, desc string) map[string]any { return map[string]any{"type": typ, "description": desc} }

func schema(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var (
	pX     = prop("integer", "x, in pixels of the image, from the left")
	pY     = prop("integer", "y, in pixels of the image, from the top")
	pColor = prop("string", "Color: #rrggbb, #rgb or a name (red, green, blue, yellow, orange, purple, magenta, cyan, black, white, gray). Default red")
	pWidth = prop("integer", "Line width in pixels (1 to 64). Default: about 1/250 of the image's longer side, at least 3")
	pSnap  = prop("boolean", "Also return a snapshot of the image after the change (default false)")
	pMask  = prop("string", "The id of a mask (from select_by_color or create_mask) instead of the box or the polygon")
	pPoly  = map[string]any{"type": "array", "description": "A shape [[x, y], …] of three points or more, in image pixels, instead of the box x, y, w, h. There is no limit to the number of points: trace an outline as closely as you need, with tens or hundreds of points, along curves and corners (look with snapshot zoom and outline to read the coordinates, and check the result before you use it)", "items": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}}
)

func toF(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// polygonArg reads "polygon" ([[x, y], …]); nil when it is not there.
func polygonArg(args map[string]any) ([][2]int, error) {
	raw, ok := args["polygon"].([]any)
	if !ok {
		return nil, nil
	}
	if len(raw) < 3 {
		return nil, fmt.Errorf("a polygon needs three points or more: [[x, y], …]")
	}
	pts := make([][2]int, 0, len(raw))
	for _, p := range raw {
		xy, _ := p.([]any)
		if len(xy) != 2 {
			return nil, fmt.Errorf("each point of the polygon is [x, y]")
		}
		x, okx := xy[0].(float64)
		y, oky := xy[1].(float64)
		if !okx || !oky {
			return nil, fmt.Errorf("each point of the polygon is [x, y] of numbers")
		}
		pts = append(pts, [2]int{int(x), int(y)})
	}
	return pts, nil
}

// retouchCmd is the command of the window for clone_region, blend_edges and fill_region: the shape (box or polygon) and the options.
func retouchCmd(op string, args map[string]any) (map[string]any, string, error) {
	cmd := map[string]any{"cmd": "retouch", "op": op}
	pts, err := polygonArg(args)
	if err != nil {
		return nil, "", err
	}
	what := ""
	if id := str(args, "mask"); id != "" {
		cmd["mask"] = id
		what = "the mask " + id
	} else if pts != nil {
		cmd["pts"] = pts
		what = fmt.Sprintf("the polygon of %d points", len(pts))
	} else {
		v, err := ints(args, "x", "y", "w", "h")
		if err != nil {
			return nil, "", fmt.Errorf("give a mask, the box x, y, w, h or a polygon: %w", err)
		}
		cmd["x"], cmd["y"], cmd["w"], cmd["h"] = v[0], v[1], v[2], v[3]
		what = fmt.Sprintf("the box %d,%d %d × %d", v[0], v[1], v[2], v[3])
	}
	return cmd, what, nil
}

func mcpTools() []toolDef { return withFrameParam(append(baseTools(), moreTools()...)) }

func baseTools() []toolDef {
	return []toolDef{
		{"window_info", "Returns the state of the window: the name of the image, its folder, size, whether it has unsaved changes, the tool in use, the selection and the zoom.", schema(nil, map[string]any{})},
		{"image_info", "Tells you about the image (or about a part of it: a box x, y, w, h, a polygon or a mask, for the color statistics): the file name (without its folder), type, size in bytes and in pixels, whether there are unsaved changes, the EXIF data of a photo (camera, lens, time, exposure, ISO, focal length; only whether there is a GPS place, not where), and how the colors are used: the number of different colors, the main colors with their share, the average color, the brightness (average, spread, darkest, brightest, a histogram), how much is transparent, whether it is gray. It reads the image as it is now, with the changes made so far. Does not change the image.", schema(nil, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of a box to describe instead of all of the image"), "h": prop("integer", "height of the box"), "polygon": pPoly, "mask": pMask})},
		{"history", "Lists the steps of the window's undo history (and the named checkpoints), each with a number, what was done and who did it (the user or you). \"undoable\" are the steps undo would take back (the last one first), \"redoable\" the ones that were undone and redo brings back. Use it to see what has been done to the image so far before you decide what to undo or redo; goto_step jumps to a step. Does not change the image.", schema(nil, map[string]any{})},
		{"get_image", "Returns the whole image of the window, with the changes made so far, pixel for pixel (a PNG at its real size; one over 4096 pixels on a side is scaled down, and the text says so). An image of up to 64 × 64 pixels is also returned as text, a palette and one string per row, which is exactly what set_pixels takes: the pixel art can be read, changed and written back.", schema(nil, map[string]any{})},
		{"snapshot", "A scaled-down view of a big image (a photo), or a part of it at full detail with x, y, w, h. For a small image use get_image.", schema(nil, map[string]any{
			"x": prop("integer", "region: left edge (default: the whole image)"), "y": prop("integer", "region: top edge"),
			"w": prop("integer", "region: width"), "h": prop("integer", "region: height"),
			"max":     prop("integer", "longest side of the image returned, in pixels (default 1568)"),
			"zoom":    prop("integer", "enlarge a small part 2 to 32 times, in whole pixels, to see single pixels and seams"),
			"outline": map[string]any{"type": "array", "description": "a shape [[x, y], …] (image pixels) drawn in magenta over the answer only, to check a mask or a patch before changing anything", "items": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}},
		})},
		{"draw_arrow", "Draws an arrow from (x1,y1) to (x2,y2); the head is at (x2,y2). One undo step.", schema([]string{"x1", "y1", "x2", "y2"}, map[string]any{
			"x1": pX, "y1": pY, "x2": pX, "y2": pY, "color": pColor, "width": pWidth, "snapshot": pSnap})},
		{"draw_line", "Draws a straight line from (x1,y1) to (x2,y2).", schema([]string{"x1", "y1", "x2", "y2"}, map[string]any{
			"x1": pX, "y1": pY, "x2": pX, "y2": pY, "color": pColor, "width": pWidth,
			"style": map[string]any{"type": "string", "enum": []string{"solid", "dashed", "dotted"}, "description": "Default solid"}, "snapshot": pSnap})},
		{"draw_rect", "Draws a rectangle with its top left corner at (x,y), w by h pixels.", schema([]string{"x", "y", "w", "h"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width in pixels"), "h": prop("integer", "height in pixels"), "color": pColor, "width": pWidth,
			"fill_color": prop("string", "Fill the inside with this color (same notation as color); left out: outline only"), "radius": prop("integer", "Round the corners by this many pixels"), "opacity": pOpacity, "shadow": pShadow,
			"style": map[string]any{"type": "string", "enum": []string{"solid", "dashed", "dotted"}, "description": "Default solid"}, "snapshot": pSnap})},
		{"draw_ellipse", "Draws an ellipse (a circle when w = h) in the box with its top left corner at (x,y), w by h pixels.", schema([]string{"x", "y", "w", "h"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width in pixels"), "h": prop("integer", "height in pixels"), "color": pColor, "width": pWidth,
			"fill_color": prop("string", "Fill the inside with this color; left out: outline only"),
			"style":      map[string]any{"type": "string", "enum": []string{"solid", "dashed", "dotted"}, "description": "Default solid"}, "snapshot": pSnap})},
		{"draw_path", "Draws a freehand line through the points, with the pencil (a polyline; also good for a highlight or a scribble).", schema([]string{"points"}, map[string]any{
			"points": map[string]any{"type": "array", "description": "[[x, y], [x, y], …], at least two points", "items": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}},
			"color":  pColor, "width": pWidth, "snapshot": pSnap})},
		{"add_text", "Writes text with its top left corner at (x,y).", schema([]string{"x", "y", "text"}, map[string]any{
			"x": pX, "y": pY, "text": prop("string", "The text (a new line starts a new row)"), "color": pColor,
			"size": prop("integer", "Font size in pixels. Default: about 1/40 of the image's longer side, at least 16"),
			"bold": prop("boolean", "Bold"), "italic": prop("boolean", "Italic"), "font": prop("string", "Font family (default: the app's; see list_fonts)"),
			"align": map[string]any{"type": "string", "enum": []string{"left", "center", "right"}, "description": "Alignment of the rows inside box_w (default left)"}, "box_w": prop("integer", "Wrap the text into rows at most this wide, in pixels; the box starts at x"),
			"outline": prop("string", "Color of an outline round the letters (for text on a busy image)"), "outline_width": prop("integer", "Width of the outline in pixels (default 2)"), "shadow": pShadow, "snapshot": pSnap})},
		{"new_image", "Makes a new blank image in the window (it has no file until save_image). Refused when the window has unsaved changes, unless discard is true.", schema([]string{"width", "height"}, map[string]any{
			"width": prop("integer", "pixels"), "height": prop("integer", "pixels"),
			"background": prop("string", "A color (#rrggbb, #rrggbbaa or a name) or \"transparent\". Default white"),
			"discard":    prop("boolean", "Throw away unsaved changes (default false)")})},
		{"set_pixels", "Sets single pixels: pixel art, a sprite, an icon. Give either pixels, a list of {x, y, color}, or grid, a list of strings (one per row, one character per pixel) with a palette that says what each character is. The new colors replace what was there (\"transparent\" makes a hole). One undo step, instant even for thousands of pixels.", schema(nil, map[string]any{
			"pixels":  map[string]any{"type": "array", "description": "[{\"x\": 3, \"y\": 4, \"color\": \"#ff0000\"}, …]", "items": map[string]any{"type": "object", "properties": map[string]any{"x": pX, "y": pY, "color": prop("string", "#rrggbb, #rrggbbaa, a name or transparent")}, "required": []string{"x", "y", "color"}}},
			"grid":    map[string]any{"type": "array", "description": "Rows of the pixels, from the top, e.g. [\"..RR..\", \".RRRR.\"]. A space leaves the pixel as it is", "items": map[string]any{"type": "string"}},
			"palette": map[string]any{"type": "object", "description": "For grid: the color of each character, e.g. {\"R\": \"#ff0000\", \".\": \"transparent\"}", "additionalProperties": map[string]any{"type": "string"}},
			"x":       prop("integer", "For grid: the column of its left edge (default 0)"), "y": prop("integer", "For grid: the row of its top edge (default 0)"),
			"snapshot": pSnap})},
		{"resize_canvas", "Makes the canvas of the image width × height pixels without scaling or changing what is drawn: the image stays where the anchor says and the new room around it gets the background. With the default anchor bottom-left the new room is above the image and to its right; top-left adds it below and to the right. A smaller canvas cuts the image. One undo step.", schema([]string{"width", "height"}, map[string]any{
			"width": prop("integer", "the new width in pixels"), "height": prop("integer", "the new height in pixels"),
			"anchor":     prop("string", "Where the image stays: top-left, top, top-right, left, center, right, bottom-left (default), bottom, bottom-right"),
			"background": prop("string", "Color of the new room: #rrggbb, #rrggbbaa, a name or transparent. Default: transparent, white for a type of file that has no transparency (JPEG, BMP)"),
			"snapshot":   pSnap})},
		{"crop", "Keeps only the part of the image with its top left corner at (x,y), w by h pixels, and cuts the rest away. One undo step.", schema([]string{"x", "y", "w", "h"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width in pixels"), "h": prop("integer", "height in pixels"), "snapshot": pSnap})},
		{"clone_region", "Copies a part of the image to another place, like a clone stamp: the part is the box x,y,w,h or the shape polygon (absolute image pixels, three points or more), and the top left corner of its box lands on (to_x, to_y). It can be flipped or turned, its edge softened (feather, pixels) and made see-through (opacity). Take pixels from a place that looks like what should be where the gap is, and check the result with snapshot. One undo step.", schema([]string{"to_x", "to_y"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box"), "h": prop("integer", "height of the box"), "polygon": pPoly, "mask": pMask,
			"to_x": prop("integer", "x where the top left corner of the part lands"), "to_y": prop("integer", "y where the top left corner of the part lands"),
			"flip_h": prop("boolean", "mirror left to right"), "flip_v": prop("boolean", "mirror top to bottom"), "rotate": prop("integer", "0, 90, 180 or 270 degrees clockwise"),
			"feather": prop("integer", "pixels of soft edge (0 is hard), for patches that must not show an edge"), "opacity": prop("number", "0 to 1 (default 1)"),
			"scale": prop("number", "0.1 to 10 (default 1), smooth; the middle of the part stays where it would have been"), "angle": prop("number", "degrees clockwise, any angle (smooth), around the middle of the part; added to rotate"),
			"blend": map[string]any{"type": "string", "enum": []string{"seamless"}, "description": "seamless: like a healing brush, the part keeps its own texture but takes the light and color of where it lands (no seam to blend afterwards)"}, "snapshot": pSnap})},
		{"blend_edges", "Hides the seam around a patch: region is the box x,y,w,h or the polygon of the area that was put there. Methods: smooth (the edge band is blurred), color_match (the colors inside are moved to the average and spread of the pixels around the area), poisson (the difference at the edge is spread smoothly over the inside, the best for a patch of different brightness). width is the pixels of the edge that are worked on, strength 0 to 1. One undo step.", schema([]string{"method"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box"), "h": prop("integer", "height of the box"), "polygon": pPoly, "mask": pMask,
			"method": map[string]any{"type": "string", "enum": []string{"smooth", "color_match", "poisson"}, "description": "smooth, color_match or poisson"},
			"width":  prop("integer", "pixels of the edge worked on (default 8)"), "strength": prop("number", "0 to 1 (default 1)"), "snapshot": pSnap})},
		{"selection_mask", "Shows you what the user has selected in the window: the polygon of the polygon select tool, or else the rectangle of the select tool. Call it first when the user says \"this\", \"the selected part\", \"what I selected\", \"the selection\" or \"this region\". It returns the corners of the polygon (or the rectangle), a mask id for the other tools, and two images: the whole image with the outline, and the selected part enlarged and tinted with its surroundings, so you can see what is in it (a label, a pattern, a stain…). Says so when nothing is selected. Does not change the image.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"select_polygon", "Selects a polygon in the window, so that the user sees it (marching dashes) and can copy it or delete it; polygon is [[x, y], …] of three points or more, as many as the outline needs. An empty polygon lets the selection go. Does not change the image.", schema(nil, map[string]any{"polygon": pPoly})},
		{"discard_changes", "Discards all changes to the image: it goes back to the file as it is on the disk (or, for an image that has no file, as it was made or pasted). One undo step, so undo brings the changes back. Use it to start again.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"select_by_color", "Makes a mask of the pixels that have the color of the pixel (x,y), give or take the tolerance: the ones connected to it (default) or all in the image. Holes inside are filled (fill_holes, default true); grow or shrink the edge by pixels. Returns the mask id, which clone_region, blend_edges, fill_region and snapshot take as mask. Use it to outline an object that has a distinct color (the berries, the sky) exactly, instead of guessing a polygon; snapshot with the mask id tints it so that you can check it. Does not change the image.", schema([]string{"x", "y"}, map[string]any{
			"x": pX, "y": pY, "tolerance": prop("integer", "0 to 100: how different a color may be (default 15)"), "contiguous": prop("boolean", "only the area connected to the pixel (default true)"),
			"fill_holes": prop("boolean", "include the holes inside the area (default true)"), "grow": prop("integer", "make the area this many pixels bigger"), "shrink": prop("integer", "make the area this many pixels smaller"), "snapshot": pSnap})},
		{"create_mask", "Makes a mask of a polygon (three points or more, as many as the outline needs: a complicated shape such as a hand, a tree or a letter can have a hundred points) or the box x,y,w,h, to use as mask, source_mask or avoid_mask in the other tools. polygons gives several shapes at once, joined into one mask (the pieces need not touch; a hole is made by subtracting another mask with edit_mask). Returns its id. Does not change the image.", schema(nil, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box"), "h": prop("integer", "height of the box"), "polygon": pPoly,
			"polygons": map[string]any{"type": "array", "description": "several polygons, each [[x, y], …], joined into one mask", "items": pPoly},
			"grow":     prop("integer", "make the area this many pixels bigger"), "shrink": prop("integer", "make the area this many pixels smaller"), "snapshot": pSnap})},
		{"edit_mask", "Changes a mask: add_mask, subtract_mask or intersect_mask (ids of other masks), invert, fill_holes, grow or shrink by pixels. The id stays the same. Does not change the image.", schema([]string{"mask"}, map[string]any{
			"mask": prop("string", "the id of the mask"), "add_mask": prop("string", "id of a mask to add"), "subtract_mask": prop("string", "id of a mask to take away"), "intersect_mask": prop("string", "id of a mask to keep only the common part with"),
			"invert": prop("boolean", "swap inside and outside"), "fill_holes": prop("boolean", "include the holes inside"), "grow": prop("integer", "bigger by pixels"), "shrink": prop("integer", "smaller by pixels"), "snapshot": pSnap})},
		{"seam_score", "Judges how much the edge of an area shows (use it after clone_region or blend_edges, so that you do not have to guess from a small image): the area is a mask, a polygon or the box x,y,w,h. It returns the seam ratio (about 1 cannot be seen, over 2 shows, over 3 is obvious), the step in brightness against the surroundings, and the worst sides (compass directions). Does not change the image.", schema(nil, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box"), "h": prop("integer", "height of the box"), "polygon": pPoly, "mask": pMask, "width": prop("integer", "pixels each side of the edge that are looked at (default 6)")})},
		{"fill_region", "Fills an area from the pixels around it (content-aware fill): a mask, a polygon or the box x,y,w,h. method patch (default) copies small patches of the surroundings from the edge inward so that textures go on; smooth, nearest, average and solid fill without patches (see method). It does not invent new objects. Make the area hug what is to be removed (a mask from select_by_color or a polygon), not a rectangle around it. One undo step.", schema(nil, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box"), "h": prop("integer", "height of the box"), "polygon": pPoly, "mask": pMask,
			"patch":       prop("integer", "size of the patches (odd, 9 by default; bigger for big structures, smaller for fine detail)"),
			"source_mask": prop("string", "use only pixels inside this mask (for example the porridge, when the gap is next to apples and berries)"), "avoid_mask": prop("string", "never use pixels inside this mask"),
			"method": map[string]any{"type": "string", "enum": []string{"patch", "smooth", "nearest", "average", "solid"}, "description": "patch (default): copy patches of the surroundings, for textures (fabric, grass, water). smooth: spread the colors of the edge inward smoothly, for flat or slowly changing surfaces (paper, post-its, walls, skin, sky). nearest: every pixel takes the color of the nearest pixel outside. average: one flat color, the average around. solid: one flat color, give color"},
			"color":  prop("string", "for method solid: #rrggbb or a name"), "grain": prop("boolean", "add the noise the surroundings have, so that smooth, nearest, average or solid do not look flat (use it on photos)"),
			"ring": prop("integer", "pixels around the area that average and grain look at (default 6)"), "snapshot": pSnap})},
		{"flood_fill", "Fills the area of similar color around (x,y) with a color (the paint bucket).", schema([]string{"x", "y"}, map[string]any{
			"x": pX, "y": pY, "color": pColor, "tolerance": prop("integer", "0 to 100: how different a color may be and still be filled (default 10)"), "snapshot": pSnap})},
		{"select_region", "Selects the rectangle with its top left corner at (x,y), w by h pixels. The selection is also put on the clipboard (copy it into another window with paste there).", schema([]string{"x", "y", "w", "h"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width in pixels"), "h": prop("integer", "height in pixels")})},
		{"copy", "Copies the selection (or the whole image when nothing is selected) to the clipboard.", schema(nil, map[string]any{})},
		{"paste", "Pastes the image on the clipboard into the window: with edit mode on (any drawing tool chosen) as a part that is placed in the image, otherwise as the window's new image (undo brings the old one back). Together with select_region and copy in another window this moves a part of one image into another.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"undo", "Undoes the last step in the window.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"redo", "Redoes the step that was undone.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"get_pixel", "Returns the color of the pixel at (x,y).", schema([]string{"x", "y"}, map[string]any{"x": pX, "y": pY})},
		{"open_image", "Opens an image file in the window. Refused when the window has unsaved changes, unless discard is true.", schema([]string{"path"}, map[string]any{
			"path": prop("string", "Full path of the image"), "discard": prop("boolean", "Throw away unsaved changes (default false)")})},
		{"save_image", "Saves the image of the window to a file; the type is the one the name ends with (png, jpg, gif, bmp, tif). The window then shows that file. Refuses to replace a file that exists unless overwrite is true.", schema([]string{"path"}, map[string]any{
			"path": prop("string", "Full path to save to"), "overwrite": prop("boolean", "Replace the file if it exists (default false)"),
			"quality": prop("integer", "JPEG quality 1 to 100 (default 95; a JPEG that was not changed is copied as it is)")})},
	}
}

var colorNames = map[string]string{
	"red": "#E53935", "green": "#2E9E44", "blue": "#1E6FE0", "yellow": "#FFD600", "orange": "#FF8A00", "purple": "#8E3FC8",
	"magenta": "#E0209C", "cyan": "#00BCD4", "black": "#000000", "white": "#FFFFFF", "gray": "#808080", "grey": "#808080", "pink": "#FF8FB1", "brown": "#7B4A22",
}

// parseColor reads #rrggbb, #rgb or a name; def is used for "".
func parseColor(s, def string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		s = def
	}
	if c, ok := colorNames[s]; ok {
		return c, nil
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if _, err := hex.DecodeString(h); err != nil || len(h) != 6 {
		return "", fmt.Errorf("%q is not a color: use #rrggbb or a name such as red", s)
	}
	return "#" + strings.ToUpper(h), nil
}

func intArg(m map[string]any, k string) (int, error) {
	f, ok := m[k].(float64)
	if !ok {
		return 0, fmt.Errorf("%q is missing (a number)", k)
	}
	return int(f + 0.5*sign(f)), nil
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

func ints(m map[string]any, keys ...string) ([]int, error) {
	out := make([]int, len(keys))
	for i, k := range keys {
		v, err := intArg(m, k)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// arrowPoints is the pen path of an arrow: the shaft to the tip, one barb of the head, back to the tip, the other barb.
func arrowPoints(x1, y1, x2, y2, width int) [][2]int {
	dx, dy := float64(x2-x1), float64(y2-y1)
	l := math.Hypot(dx, dy)
	if l < 1 {
		return [][2]int{{x1, y1}, {x2, y2}}
	}
	head := max(14.0, 4.5*float64(width))
	head = min(head, l*0.6) // a short arrow keeps a head in proportion
	ux, uy := dx/l, dy/l
	pt := func(deg float64) [2]int {
		a := deg * math.Pi / 180
		c, s := math.Cos(a), math.Sin(a)
		bx, by := -(ux*c-uy*s)*head, -(ux*s+uy*c)*head
		return [2]int{x2 + int(bx+0.5*sign(bx)), y2 + int(by+0.5*sign(by))}
	}
	return [][2]int{{x1, y1}, {x2, y2}, pt(28), {x2, y2}, pt(-28)}
}

// state is the state command of the window (see docs/control.md).
func (h *chatHub) state() (map[string]any, error) {
	return h.window(map[string]any{"cmd": "state"})
}

func pictureSize(st map[string]any) (int, int, error) {
	sz, ok := st["size"].([]any)
	if !ok || len(sz) != 2 {
		return 0, 0, fmt.Errorf("this window has no image: open one first (open_image)")
	}
	pw, _ := sz[0].(float64)
	ph, _ := sz[1].(float64)
	return int(pw), int(ph), nil
}

func (h *chatHub) callTool(name string, args map[string]any) ([]any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if name == "window_info" {
		return h.windowInfo()
	}
	h.framed = false
	defer func() {
		if h.framed { // the frame the user has on the screen was not moved by a tool that worked on another one
			_, _ = h.window(map[string]any{"cmd": "gifframe", "frame": 0})
		}
	}()
	if err := h.applyFrame(name, args); err != nil {
		return nil, err
	}
	if content, handled, err := h.moreCall(name, args); handled {
		return content, err
	}
	var content []any
	snapMask := "" // the mask a snapshot asked for with the answer shows tinted
	switch name {
	case "snapshot":
		return h.snapshotTool(args)
	case "image_info":
		cmd := map[string]any{"cmd": "imageinfo"}
		for _, k := range []string{"x", "y", "w", "h", "mask"} {
			if v, ok := args[k]; ok {
				cmd[k] = v
			}
		}
		if pts, err := polygonArg(args); err != nil {
			return nil, err
		} else if pts != nil {
			cmd["pts"] = pts
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, err
		}
		delete(res, "ok")
		return []any{jsonText(res)}, nil
	case "history":
		res, err := h.window(map[string]any{"cmd": "history"})
		if err != nil {
			return nil, err
		}
		return []any{jsonText(historyReport(res))}, nil
	case "get_image":
		return h.snapshotTool(map[string]any{"max": 4096})
	case "new_image":
		return h.newImage(args)
	case "set_pixels":
		list, summary, err := h.pixelList(args)
		if err != nil {
			return nil, err
		}
		if _, err := h.window(map[string]any{"cmd": "pixels", "list": list}); err != nil {
			return nil, err
		}
		content = []any{textContent(summary)}
	case "clone_region", "blend_edges", "fill_region":
		op := map[string]string{"clone_region": "clone", "blend_edges": "blend", "fill_region": "fill"}[name]
		cmd, what, err := retouchCmd(op, args)
		if err != nil {
			return nil, err
		}
		summary := ""
		switch op {
		case "clone":
			tx, ty, err := func() (int, int, error) {
				v, err := ints(args, "to_x", "to_y")
				if err != nil {
					return 0, 0, err
				}
				return v[0], v[1], nil
			}()
			if err != nil {
				return nil, err
			}
			cmd["tx"], cmd["ty"] = tx, ty
			for _, k := range []string{"flip_h", "flip_v"} {
				cmd[k] = args[k] == true
			}
			for _, k := range []string{"rotate", "feather"} {
				if _, ok := args[k]; ok {
					if cmd[k], err = intArg(args, k); err != nil {
						return nil, err
					}
				}
			}
			if v, ok := args["opacity"].(float64); ok {
				cmd["opacity"] = v
			}
			for _, k := range []string{"scale", "angle"} {
				if v, ok := args[k].(float64); ok {
					cmd[k] = v
				}
			}
			if b := str(args, "blend"); b != "" {
				cmd["blend"] = b
			}
			summary = fmt.Sprintf("Copied %s to %d,%d", what, tx, ty)
		case "blend":
			cmd["method"] = str(args, "method")
			if cmd["method"] == "" {
				cmd["method"] = "smooth"
			}
			if _, ok := args["width"]; ok {
				if cmd["width"], err = intArg(args, "width"); err != nil {
					return nil, err
				}
			}
			if v, ok := args["strength"].(float64); ok {
				cmd["strength"] = v
			}
			summary = fmt.Sprintf("Blended the edges of %s (%s)", what, cmd["method"])
		case "fill":
			if _, ok := args["patch"]; ok {
				if cmd["patch"], err = intArg(args, "patch"); err != nil {
					return nil, err
				}
			}
			for _, k := range []string{"source_mask", "avoid_mask", "method"} {
				if v := str(args, k); v != "" {
					cmd[k] = v
				}
			}
			if v := str(args, "color"); v != "" {
				c, err := parseColor(v, "")
				if err != nil {
					return nil, err
				}
				cmd["color"] = c
			}
			if args["grain"] == true {
				cmd["grain"] = true
			}
			if _, ok := args["ring"]; ok {
				if cmd["ring"], err = intArg(args, "ring"); err != nil {
					return nil, err
				}
			}
			how := str(args, "method")
			if how == "" {
				how = "patch"
			}
			summary = "Filled " + what + " from its surroundings (" + how + ")"
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if n, _ := res["note"].(string); n != "" {
			summary += ". " + n
		}
		if id, _ := cmd["mask"].(string); id != "" {
			snapMask = id
		}
		content = []any{textContent(summary)}
	case "selection_mask":
		st, err := h.state()
		if err != nil {
			return nil, err
		}
		cmd := map[string]any{"cmd": "mask", "op": "create"}
		what := ""
		if poly, ok := st["polygon"].([]any); ok && len(poly) >= 3 {
			cmd["pts"], what = poly, fmt.Sprintf("the polygon of %d points", len(poly))
		} else if sel, ok := st["selection"].(map[string]any); ok {
			for _, k := range []string{"x", "y", "w", "h"} {
				cmd[k] = sel[k]
			}
			what = fmt.Sprintf("the rectangle %v,%v %v × %v", sel["x"], sel["y"], sel["w"], sel["h"])
		} else {
			return nil, fmt.Errorf("nothing is selected in the window: the user has to select something with the select or polygon select tool, or you can make a mask yourself (select_by_color, create_mask)")
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, fmt.Errorf("selection_mask: %w", err)
		}
		snapMask, _ = res["mask"].(string)
		box, _ := res["box"].([]any)
		text := fmt.Sprintf("The user selected %s. Mask %v: %v pixels, box %v (x, y, w, h). Pass it as mask to clone_region, blend_edges, fill_region or snapshot.", what, res["mask"], res["pixels"], box)
		var outline any
		if poly, ok := cmd["pts"].([]any); ok {
			outline = poly
			shown := poly
			if len(shown) > 120 {
				shown = shown[:120]
			}
			j, _ := json.Marshal(shown)
			text += fmt.Sprintf(" The polygon's corners in image pixels (%d of them%s): %s.", len(poly), map[bool]string{true: ", the first 120 shown", false: ""}[len(poly) > 120], j)
		} else if x, y, w, hh := cmd["x"], cmd["y"], cmd["w"], cmd["h"]; x != nil {
			fx, fy, fw, fh := toF(x), toF(y), toF(w), toF(hh)
			outline = []any{[]any{fx, fy}, []any{fx + fw - 1, fy}, []any{fx + fw - 1, fy + fh - 1}, []any{fx, fy + fh - 1}}
		}
		content = []any{textContent(text)}
		// what the user selected, as pictures: the whole picture with the outline (where it is), and the part itself, enlarged and tinted (what is in it)
		if pw, ph, err := pictureSize(st); err == nil && len(box) == 4 {
			bx, by, bw, bh := int(toF(box[0])), int(toF(box[1])), int(toF(box[2])), int(toF(box[3]))
			pad := max(12, max(bw, bh)/5)
			x0, y0 := max(0, bx-pad), max(0, by-pad)
			x1, y1 := min(pw, bx+bw+pad), min(ph, by+bh+pad)
			detail := map[string]any{"x": x0, "y": y0, "w": x1 - x0, "h": y1 - y0, "mask": snapMask, "outline": outline}
			if z := 900 / max(x1-x0, y1-y0, 1); z > 1 {
				detail["zoom"] = min(z, 32)
			}
			context := map[string]any{"max": 1000, "mask": snapMask, "outline": outline}
			if snap, err := h.snapshotTool(context); err == nil {
				content = append(content, snap...)
			}
			if snap, err := h.snapshotTool(detail); err == nil {
				content = append(content, snap...)
			}
		}
	case "select_polygon":
		pts := [][2]int{}
		if arr, ok := args["polygon"].([]any); !ok || len(arr) > 0 { // [] lets the selection go
			var err error
			if pts, err = polygonArg(args); err != nil {
				return nil, err
			}
			if pts == nil {
				return nil, fmt.Errorf("give polygon: [[x, y], …] of three points or more (or [] to let the selection go)")
			}
		}
		if _, err := h.window(map[string]any{"cmd": "polygon", "pts": pts}); err != nil {
			return nil, fmt.Errorf("select_polygon: %w", err)
		}
		if len(pts) == 0 {
			content = []any{textContent("The polygon selection is let go")}
		} else {
			content = []any{textContent(fmt.Sprintf("Selected the polygon of %d points (the window shows it)", len(pts)))}
		}
	case "discard_changes":
		if _, err := h.window(map[string]any{"cmd": "menu", "name": "discard"}); err != nil {
			return nil, fmt.Errorf("discard_changes: %w", err)
		}
		content = []any{textContent("The changes are discarded: the image is as it was when it came into the window (undo brings them back)")}
	case "select_by_color", "create_mask", "edit_mask":
		cmd := map[string]any{"cmd": "mask", "op": map[string]string{"select_by_color": "color", "create_mask": "create", "edit_mask": "edit"}[name]}
		for _, k := range []string{"x", "y", "w", "h", "tolerance", "contiguous", "fill_holes", "grow", "shrink", "mask", "add_mask", "subtract_mask", "intersect_mask", "invert", "polys"} {
			if v, ok := args[k]; ok {
				cmd[k] = v
			}
		}
		if pts, err := polygonArg(args); err != nil {
			return nil, err
		} else if pts != nil {
			cmd["pts"] = pts
		}
		if polys, ok := args["polygons"].([]any); ok { // each of them is checked like a polygon
			var all [][][2]int
			for i, p := range polys {
				pts, err := polygonArg(map[string]any{"polygon": p})
				if err != nil || pts == nil {
					return nil, fmt.Errorf("polygons[%d]: a polygon is [[x, y], …] of three points or more", i)
				}
				all = append(all, pts)
			}
			cmd["polys"] = all
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		snapMask, _ = res["mask"].(string)
		box, _ := res["box"].([]any)
		content = []any{textContent(fmt.Sprintf("Mask %v: %v pixels, box %v (x, y, w, h) of an image of %v. Pass it as mask to clone_region, blend_edges, fill_region or snapshot (snapshot shows it tinted).", res["mask"], res["pixels"], box, res["picture"]))}
	case "seam_score":
		cmd := map[string]any{"cmd": "seam"}
		for _, k := range []string{"x", "y", "w", "h", "mask", "width"} {
			if v, ok := args[k]; ok {
				cmd[k] = v
			}
		}
		if pts, err := polygonArg(args); err != nil {
			return nil, err
		} else if pts != nil {
			cmd["pts"] = pts
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, fmt.Errorf("seam_score: %w", err)
		}
		content = []any{textContent(fmt.Sprint(res["report"]))}
	case "resize_canvas":
		v, err := ints(args, "width", "height")
		if err != nil {
			return nil, err
		}
		cmd := map[string]any{"cmd": "resize", "w": v[0], "h": v[1], "anchor": str(args, "anchor")}
		if b := str(args, "background"); b != "" {
			if cmd["bg"], err = parseColorAlpha(b, ""); err != nil {
				return nil, err
			}
		}
		if _, _, err := canvasOffset(str(args, "anchor"), 1, 1, 1, 1); err != nil {
			return nil, err
		}
		res, err := h.window(cmd)
		if err != nil {
			return nil, err
		}
		pw, ph, _ := pictureSize(res)
		anchor := str(args, "anchor")
		if anchor == "" {
			anchor = "bottom-left"
		}
		content = []any{textContent(fmt.Sprintf("The canvas is %d × %d pixels now (the image stays at %s)", pw, ph, anchor))}
	case "crop":
		v, err := ints(args, "x", "y", "w", "h")
		if err != nil {
			return nil, err
		}
		res, err := h.window(map[string]any{"cmd": "crop", "x": v[0], "y": v[1], "w": v[2], "h": v[3]})
		if err != nil {
			return nil, err
		}
		pw, ph, _ := pictureSize(res)
		content = []any{textContent(fmt.Sprintf("Cropped to %d,%d %d × %d: the image is %d × %d pixels now", v[0], v[1], v[2], v[3], pw, ph))}
	case "get_pixel":
		xy, err := ints(args, "x", "y")
		if err != nil {
			return nil, err
		}
		res, err := h.window(map[string]any{"cmd": "pixel", "x": xy[0], "y": xy[1]})
		if err != nil {
			return nil, err
		}
		return []any{jsonText(res["pixel"])}, nil
	case "select_region":
		v, err := ints(args, "x", "y", "w", "h")
		if err != nil {
			return nil, err
		}
		if v[2] < 2 || v[3] < 2 {
			return nil, fmt.Errorf("a selection needs w and h of 2 or more")
		}
		res, err := h.window(map[string]any{"cmd": "select", "x0": v[0], "y0": v[1], "x1": v[0] + v[2] - 1, "y1": v[1] + v[3] - 1})
		if err != nil {
			return nil, err
		}
		return []any{jsonText(map[string]any{"selection": res["selection"]})}, nil
	case "copy", "paste", "undo", "redo":
		if _, err := h.window(map[string]any{"cmd": "menu", "name": name}); err != nil {
			return nil, err
		}
		content = []any{textContent("Done: " + name)}
	case "open_image":
		path := str(args, "path")
		if path == "" {
			return nil, fmt.Errorf("\"path\" is missing")
		}
		if st, err := h.state(); err == nil && st["modified"] == true && args["discard"] != true {
			return nil, fmt.Errorf("the window has unsaved changes: save_image first, or call again with discard=true")
		}
		if _, err := h.window(map[string]any{"cmd": "open", "path": path}); err != nil {
			return nil, err
		}
		content = []any{textContent("Opened " + path)}
	case "save_image":
		path := str(args, "path")
		if path == "" {
			return nil, fmt.Errorf("\"path\" is missing")
		}
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() && args["overwrite"] != true {
			return nil, fmt.Errorf("%s exists: call again with overwrite=true to replace it, or choose another name", path)
		}
		save := map[string]any{"cmd": "saveto", "path": path, "overwrite": args["overwrite"] == true}
		if q, ok := args["quality"]; ok {
			save["quality"] = q
		}
		res, err := h.window(save)
		if err != nil {
			return nil, err
		}
		note, _ := res["note"].(string)
		if note == "" {
			note = "Saved " + path
		}
		return []any{textContent(note)}, nil
	default:
		cmds, summary, err := h.drawCommands(name, args)
		if err != nil {
			return nil, err
		}
		addShapeExtras(cmds, args)
		for _, c := range cmds {
			if _, err := h.window(c); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		}
		content = []any{textContent(summary)}
	}
	if args["snapshot"] == true {
		sargs := map[string]any{}
		if snapMask != "" {
			sargs["mask"] = snapMask
		}
		snap, err := h.snapshotTool(sargs)
		if err == nil {
			content = append(content, snap...)
		}
	}
	return content, nil
}

func (h *chatHub) windowInfo() ([]any, error) {
	st, err := h.state()
	if err != nil {
		return nil, err
	}
	e := map[string]any{}
	for _, k := range []string{"name", "dir", "size", "modified", "tool", "edit", "selection", "index", "count", "zoom"} {
		if v, ok := st[k]; ok {
			e[k] = v
		}
	}
	return []any{jsonText(e)}, nil
}

func (h *chatHub) snapshotTool(args map[string]any) ([]any, error) {
	cmd := map[string]any{"cmd": "snapshot"}
	for _, k := range []string{"x", "y", "w", "h", "max", "zoom", "outline", "mask", "compare"} {
		if v, ok := args[k]; ok {
			cmd[k] = v
		}
	}
	res, err := h.window(cmd)
	if err != nil {
		return nil, err
	}
	data, _ := res["png"].(string)
	scale, _ := res["scale"].(float64)
	note := "Image size and region unknown"
	if pic, _ := res["picture"].([]any); len(pic) == 2 {
		note = fmt.Sprintf("Image is %v × %v pixels. This image shows the region [x, y, w, h] = %v", pic[0], pic[1], res["region"])
	}
	if res["compare"] == true {
		note += ". LEFT is the original file, RIGHT is the image now (a magenta bar between them); both show the same region"
	}
	if z, _ := res["zoom"].(float64); z > 1 {
		note += fmt.Sprintf(", enlarged %d× (every image pixel is %d × %d pixels here: image x = region x + image x / %d)", int(z), int(z), int(z), int(z))
	} else if scale < 1 {
		note += fmt.Sprintf(", scaled down by %.3f (multiply positions you read in the image by %.3f to get image pixels)", scale, 1/scale)
	} else {
		note += ", at full size (positions in the image are image pixels plus the region's x and y)"
	}
	out := []any{map[string]any{"type": "image", "data": data, "mimeType": "image/png"}, textContent(note)}
	if grid, ok := res["grid"].(map[string]any); ok {
		out = append(out, jsonText(grid))
	}
	return out, nil
}

// parseColorAlpha is parseColor for pixels: the answer is "#rrggbbaa"; "transparent" is alpha 0.
func parseColorAlpha(s, def string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "transparent", "clear", "none":
		return "#00000000", nil
	}
	if t == "" {
		t = def
	}
	h := strings.TrimPrefix(t, "#")
	if len(h) == 8 || len(h) == 4 { // with alpha
		if len(h) == 4 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2], h[3], h[3]})
		}
		if _, err := hex.DecodeString(h); err != nil {
			return "", fmt.Errorf("%q is not a color: use #rrggbb, #rrggbbaa, a name or transparent", s)
		}
		return "#" + strings.ToUpper(h), nil
	}
	c, err := parseColor(t, def)
	if err != nil {
		return "", fmt.Errorf("%q is not a color: use #rrggbb, #rrggbbaa, a name or transparent", s)
	}
	return c + "FF", nil
}

// pixelList turns the arguments of set_pixels (pixels, or grid and palette) into the list [[x, y, "#rrggbbaa"], …] the window takes.
func (h *chatHub) pixelList(args map[string]any) ([][3]any, string, error) {
	st, err := h.state()
	if err != nil {
		return nil, "", err
	}
	pw, ph, err := pictureSize(st)
	if err != nil {
		return nil, "", err
	}
	var list [][3]any
	add := func(x, y int, c string) error {
		if x < 0 || y < 0 || x >= pw || y >= ph {
			return fmt.Errorf("the pixel %d,%d is outside the image (%d × %d pixels)", x, y, pw, ph)
		}
		list = append(list, [3]any{x, y, c})
		return nil
	}
	if raw, ok := args["pixels"].([]any); ok {
		for i, p := range raw {
			m, _ := p.(map[string]any)
			v, err := ints(m, "x", "y")
			if err != nil {
				return nil, "", fmt.Errorf("pixels[%d]: %w", i, err)
			}
			c, err := parseColorAlpha(str(m, "color"), "")
			if str(m, "color") == "" {
				err = fmt.Errorf("\"color\" is missing")
			}
			if err != nil {
				return nil, "", fmt.Errorf("pixels[%d]: %w", i, err)
			}
			if err := add(v[0], v[1], c); err != nil {
				return nil, "", err
			}
		}
	} else if rows, ok := args["grid"].([]any); ok {
		pal := map[rune]string{}
		pm, _ := args["palette"].(map[string]any)
		for k, v := range pm {
			r := []rune(k)
			if len(r) != 1 {
				return nil, "", fmt.Errorf("the palette key %q must be one character", k)
			}
			c, err := parseColorAlpha(fmt.Sprint(v), "")
			if err != nil {
				return nil, "", fmt.Errorf("palette %q: %w", k, err)
			}
			pal[r[0]] = c
		}
		ox, oy := num(args, "x", 0), num(args, "y", 0)
		for y, row := range rows {
			for x, ch := range []rune(fmt.Sprint(row)) {
				if ch == ' ' {
					continue
				}
				c, ok := pal[ch]
				if !ok {
					return nil, "", fmt.Errorf("row %d, column %d: %q is not in the palette", y, x, string(ch))
				}
				if err := add(ox+x, oy+y, c); err != nil {
					return nil, "", err
				}
			}
		}
	} else {
		return nil, "", fmt.Errorf("give \"pixels\" or \"grid\" with \"palette\"")
	}
	if len(list) == 0 {
		return nil, "", fmt.Errorf("no pixels to set")
	}
	if len(list) > 1<<20 {
		return nil, "", fmt.Errorf("%d pixels at once is too many (up to %d)", len(list), 1<<20)
	}
	return list, fmt.Sprintf("Set %d pixels (image %d × %d)", len(list), pw, ph), nil
}

func (h *chatHub) newImage(args map[string]any) ([]any, error) {
	v, err := ints(args, "width", "height")
	if err != nil {
		return nil, err
	}
	bg, err := parseColorAlpha(str(args, "background"), "white")
	if err != nil {
		return nil, err
	}
	if st, err := h.state(); err == nil && st["modified"] == true && args["discard"] != true {
		return nil, fmt.Errorf("the window has unsaved changes: save_image first, or call again with discard=true")
	}
	if _, err := h.window(map[string]any{"cmd": "new", "w": v[0], "h": v[1], "color": bg}); err != nil {
		return nil, err
	}
	return []any{textContent(fmt.Sprintf("New image %d × %d, background %s. Draw with set_pixels or the drawing tools; look with get_image.", v[0], v[1], bg))}, nil
}

// drawCommands are the commands for a drawing tool: each one puts its shape, fill or text into the picture at once (no pointer, no tool to choose).
func (h *chatHub) drawCommands(name string, args map[string]any) ([]map[string]any, string, error) {
	st, err := h.state()
	if err != nil {
		return nil, "", err
	}
	pw, ph, err := pictureSize(st)
	if err != nil {
		return nil, "", err
	}
	long := max(pw, ph)
	width := 0
	if _, ok := args["width"]; ok {
		if width, err = intArg(args, "width"); err != nil {
			return nil, "", err
		}
		width = min(max(width, 1), 64)
	} else {
		width = min(max(long/250, 3), 64)
	}
	color, err := parseColor(str(args, "color"), "red")
	if err != nil {
		return nil, "", err
	}
	inside := func(x, y int) error {
		if x < 0 || y < 0 || x >= pw || y >= ph {
			return fmt.Errorf("the position %d,%d is outside the image (%d × %d pixels)", x, y, pw, ph)
		}
		return nil
	}
	clamp := func(x, y int) [2]int { return [2]int{min(max(x, 0), pw-1), min(max(y, 0), ph-1)} }
	style := str(args, "style")
	if style == "" {
		style = "solid"
	}
	switch name {
	case "draw_arrow", "draw_line":
		v, err := ints(args, "x1", "y1", "x2", "y2")
		if err != nil {
			return nil, "", err
		}
		if err := inside(v[0], v[1]); err != nil {
			return nil, "", err
		}
		if err := inside(v[2], v[3]); err != nil {
			return nil, "", err
		}
		if name == "draw_line" {
			cmds := []map[string]any{{"cmd": "shape", "kind": "line", "x0": v[0], "y0": v[1], "x1": v[2], "y1": v[3], "color": color, "width": width, "style": style}}
			return cmds, fmt.Sprintf("Drew a %s line from %d,%d to %d,%d (%s, width %d)", style, v[0], v[1], v[2], v[3], color, width), nil
		}
		pts := arrowPoints(v[0], v[1], v[2], v[3], width)
		for i := range pts {
			pts[i] = clamp(pts[i][0], pts[i][1])
		}
		cmds := []map[string]any{{"cmd": "shape", "kind": "path", "pts": pts, "color": color, "width": width}}
		return cmds, fmt.Sprintf("Drew an arrow from %d,%d to %d,%d (%s, width %d)", v[0], v[1], v[2], v[3], color, width), nil
	case "draw_rect", "draw_ellipse":
		v, err := ints(args, "x", "y", "w", "h")
		if err != nil {
			return nil, "", err
		}
		if v[2] < 2 || v[3] < 2 {
			return nil, "", fmt.Errorf("w and h must be 2 or more")
		}
		if err := inside(v[0], v[1]); err != nil {
			return nil, "", err
		}
		c1 := clamp(v[0]+v[2]-1, v[1]+v[3]-1)
		cmd := map[string]any{"cmd": "shape", "kind": "rect", "x0": v[0], "y0": v[1], "x1": c1[0], "y1": c1[1], "color": color, "width": width, "style": style, "fill": "outline"}
		if f := str(args, "fill_color"); f != "" {
			fc, err := parseColor(f, "")
			if err != nil {
				return nil, "", err
			}
			cmd["bg"], cmd["fill"] = fc, "both"
		}
		what := "rectangle"
		if name == "draw_ellipse" {
			cmd["kind"], what = "ellipse", "ellipse"
		}
		cmds := []map[string]any{cmd}
		return cmds, fmt.Sprintf("Drew a %s at %d,%d, %d × %d (%s, width %d)", what, v[0], v[1], v[2], v[3], color, width), nil
	case "draw_polygon":
		pts, err := polygonArg(args)
		if err != nil {
			return nil, "", err
		}
		if pts == nil {
			return nil, "", fmt.Errorf("\"polygon\" is missing")
		}
		for i := range pts {
			pts[i] = clamp(pts[i][0], pts[i][1])
		}
		cmd := map[string]any{"cmd": "shape", "kind": "polygon", "pts": pts, "color": color, "width": width, "style": style, "fill": "outline"}
		if f := str(args, "fill_color"); f != "" {
			fc, err := parseColor(f, "")
			if err != nil {
				return nil, "", err
			}
			cmd["bg"], cmd["fill"] = fc, "both"
		}
		return []map[string]any{cmd}, fmt.Sprintf("Drew a polygon of %d points (%s, width %d)", len(pts), color, width), nil
	case "draw_callout":
		v, err := ints(args, "x", "y", "w", "h", "tail_x", "tail_y")
		if err != nil {
			return nil, "", err
		}
		if v[2] < 20 || v[3] < 20 {
			return nil, "", fmt.Errorf("a bubble must be at least 20 × 20 pixels")
		}
		if err := inside(v[0], v[1]); err != nil {
			return nil, "", err
		}
		c1 := clamp(v[0]+v[2]-1, v[1]+v[3]-1)
		tip := clamp(v[4], v[5])
		fill, err := parseColor(str(args, "fill_color"), "white")
		if err != nil {
			return nil, "", err
		}
		radius := 12
		if _, ok := args["radius"]; ok {
			radius, _ = intArg(args, "radius")
		}
		cmd := map[string]any{"cmd": "shape", "kind": "callout", "x0": v[0], "y0": v[1], "x1": c1[0], "y1": c1[1], "tx": tip[0], "ty": tip[1], "color": color, "bg": fill, "width": width, "fill": "both"}
		pad := max(8, width+radius/3)
		return []map[string]any{cmd}, fmt.Sprintf("Drew a speech bubble at %d,%d, %d × %d, its tail pointing at %d,%d. The text fits in the box x=%d, y=%d, w=%d, h=%d: use add_text there with box_w=%d and align center",
			v[0], v[1], v[2], v[3], tip[0], tip[1], v[0]+pad, v[1]+pad, v[2]-2*pad, v[3]-2*pad, v[2]-2*pad), nil
	case "draw_path":
		raw, _ := args["points"].([]any)
		if len(raw) < 2 {
			return nil, "", fmt.Errorf("\"points\" needs at least two points [[x, y], …]")
		}
		pts := make([][2]int, 0, len(raw))
		for _, p := range raw {
			xy, _ := p.([]any)
			if len(xy) != 2 {
				return nil, "", fmt.Errorf("each point is [x, y]")
			}
			x, _ := xy[0].(float64)
			y, _ := xy[1].(float64)
			pts = append(pts, clamp(int(x), int(y)))
		}
		cmds := []map[string]any{{"cmd": "shape", "kind": "path", "pts": pts, "color": color, "width": width}}
		return cmds, fmt.Sprintf("Drew a path through %d points (%s, width %d)", len(pts), color, width), nil
	case "add_text":
		v, err := ints(args, "x", "y")
		if err != nil {
			return nil, "", err
		}
		if err := inside(v[0], v[1]); err != nil {
			return nil, "", err
		}
		text := str(args, "text")
		if text == "" {
			return nil, "", fmt.Errorf("\"text\" is missing")
		}
		size := min(max(long/40, 16), 400)
		if _, ok := args["size"]; ok {
			if size, err = intArg(args, "size"); err != nil {
				return nil, "", err
			}
		}
		opts := map[string]any{"size": size, "bold": args["bold"] == true, "italic": args["italic"] == true}
		if f := str(args, "font"); f != "" {
			opts["font"] = f
		}
		cmds := []map[string]any{{"cmd": "option", "size": opts["size"], "bold": opts["bold"], "italic": opts["italic"]}}
		if f, ok := opts["font"]; ok {
			cmds[0]["font"] = f
		}
		cmds = append(cmds, map[string]any{"cmd": "text", "x": v[0], "y": v[1], "value": text, "color": color})
		return cmds, fmt.Sprintf("Wrote %q at %d,%d (%s, size %d)", text, v[0], v[1], color, size), nil
	case "flood_fill":
		v, err := ints(args, "x", "y")
		if err != nil {
			return nil, "", err
		}
		if err := inside(v[0], v[1]); err != nil {
			return nil, "", err
		}
		opts := map[string]any{}
		if _, ok := args["tolerance"]; ok {
			t, err := intArg(args, "tolerance")
			if err != nil {
				return nil, "", err
			}
			opts["tolerance"] = min(max(t, 0), 100)
		}
		cmd := map[string]any{"cmd": "fill", "x": v[0], "y": v[1], "color": color}
		if t, ok := opts["tolerance"]; ok {
			cmd["tolerance"] = t
		}
		cmds := []map[string]any{cmd}
		return cmds, fmt.Sprintf("Filled the area around %d,%d with %s", v[0], v[1], color), nil
	}
	return nil, "", fmt.Errorf("unknown tool %q", name)
}

// historyReport shapes the steps the window sent (oldest first, "at" is the one shown now) for the model: the steps undo takes back, last first, and the ones redo brings back.
func historyReport(res map[string]any) map[string]any {
	steps, _ := res["steps"].([]any)
	at := 0
	if f, ok := res["at"].(float64); ok {
		at = int(f)
	}
	line := func(s any) map[string]any {
		m, _ := s.(map[string]any)
		return map[string]any{"step": m["n"], "what": m["label"], "by": m["by"]}
	}
	undoable, redoable := []any{}, []any{}
	for i := len(steps) - 1; i >= 0; i-- {
		switch {
		case i > at:
			redoable = append([]any{line(steps[i])}, redoable...) // the next redo first
		case i > 0 && i <= at:
			undoable = append(undoable, line(steps[i]))
		}
	}
	out := map[string]any{"undoable": undoable, "redoable": redoable}
	if cp, ok := res["checkpoints"].([]any); ok && len(cp) > 0 {
		out["checkpoints"] = cp // named steps, for goto_checkpoint
	}
	if len(steps) > 0 {
		out["start"] = line(steps[0])["what"]
	}
	return out
}
