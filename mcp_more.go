package main

import (
	"fmt"
	"os"
)

// More tools for the chat panel: effects and color adjustments, scaling and turning, laying in a picture from a file, the drawing extras, what changed
// against the file, the folder and the fonts, and going back in the history. They are told to the model in mcpTools (see moreTools) and run from callTool.

var (
	pEffects = []string{"blur", "pixelize", "emboss", "explosion", "grayscale", "negative", "sepia", "censor", "highlight", "fill", "gradient",
		"brightness", "contrast", "saturation", "hue", "temperature", "gamma", "levels", "auto_levels", "sharpen", "rotate"}
	pFeather = prop("integer", "Pixels over which the edge of the part fades into the rest (0 = hard edge)")
	pShadow  = map[string]any{"type": "object", "description": "A soft shadow under the shape: {dx, dy, blur, color} (default 3, 3, 4, black)", "properties": map[string]any{
		"dx": map[string]any{"type": "integer"}, "dy": map[string]any{"type": "integer"}, "blur": map[string]any{"type": "integer"}, "color": map[string]any{"type": "string"}}}
	pOpacity = prop("number", "0 (invisible) to 1 (solid, the default)")
)

func moreTools() []toolDef {
	return []toolDef{
		{"apply_effect", "Applies an effect or a color adjustment to part of the image: a box (x, y, w, h), a polygon (any number of points), a mask, what the user selected (selection: true), or, when none of these is given, all of the image. Effects: blur (radius), pixelize (size = the side of a block), emboss (angle = where the light comes from, amount), explosion (a burst from cx, cy, amount 0 to 100), grayscale, negative, sepia, censor (a mosaic too coarse to read, or bar: true for a solid bar; use it to hide faces, plates, names, screens), highlight (the part stays as it is and gets a thin border; the rest is pushed back by level: light = a faint shade, medium = dimmed and paler but still readable, with a soft shadow round the part, full (default) = also blurred; amount 0 to 100 scales it, default 60; color is the border, white by default), fill (color, opacity), gradient (color to color2, angle, kind linear or radial, opacity), sharpen (amount 0 to 500, radius), and the adjustments brightness, contrast, saturation, temperature (amount -100 to 100; temperature above 0 is warmer), hue (angle in degrees), gamma (gamma, 1 = no change), levels (black, white, gamma), auto_levels. One step of the history, so undo takes it back; use feather for a soft edge. For a polygon of the outline of a thing use create_mask or select_by_color first and pass the mask.", schema([]string{"effect"}, map[string]any{
			"effect": map[string]any{"type": "string", "enum": pEffects},
			"x":      pX, "y": pY, "w": prop("integer", "width of the box in pixels"), "h": prop("integer", "height of the box in pixels"), "polygon": pPoly, "mask": pMask,
			"selection": prop("boolean", "Work on what the user selected in the window (the polygon, else the box)"), "feather": pFeather,
			"radius": prop("number", "blur and sharpen: pixels (blur default 6, sharpen 2)"), "size": prop("integer", "pixelize and censor: the side of a block in pixels (default 12; censor: about a sixth of the part)"),
			"amount": prop("number", "The strength: -100 to 100 for the adjustments, 0 to 100 for emboss and explosion, 0 to 500 for sharpen"),
			"angle":  prop("number", "degrees: emboss (the light), hue (the turn), gradient (0 = left to right, 90 = top to bottom)"),
			"cx":     prop("number", "explosion: x of the middle of the burst (default the middle of the part)"), "cy": prop("number", "explosion: y of the middle"),
			"color": prop("string", "fill, gradient (start) and the bar of censor: a color (#rrggbb, #rrggbbaa, a name)"), "color2": prop("string", "gradient: the end color"),
			"kind": map[string]any{"type": "string", "enum": []string{"linear", "radial"}, "description": "gradient"}, "opacity": pOpacity,
			"bar": prop("boolean", "censor: a solid bar (color, black by default) instead of a mosaic"), "gamma": prop("number", "gamma and levels: above 1 lightens the middle tones"),
			"level": map[string]any{"type": "string", "enum": []string{"light", "medium", "full"}, "description": "highlight: how far the rest is pushed back (default full)"},
			"black": prop("integer", "levels: the value (0 to 254) that becomes black"), "white": prop("integer", "levels: the value (1 to 255) that becomes white"), "snapshot": pSnap})},
		{"resize_image", "Scales the image to a new size (resampling the pixels, unlike resize_canvas which adds or cuts room). Give w and h, or only one of them to keep the proportions, or scale (2 = twice as big). One step of the history.", schema(nil, map[string]any{
			"w": prop("integer", "new width in pixels"), "h": prop("integer", "new height in pixels"), "scale": prop("number", "factor instead of w and h (0.5 = half)"),
			"filter": map[string]any{"type": "string", "enum": []string{"smooth", "sharp", "nearest"}, "description": "smooth (default, best for photos), sharp, nearest (whole pixels, for pixel art)"}, "snapshot": pSnap})},
		{"rotate_image", "Turns the image clockwise by angle degrees. Quarter turns (90, 180, 270) lose nothing. Other angles make the canvas bigger to hold the corners (the corners are transparent where the file type can be, else white, or bg), or with crop keep the size and cut the corners off. One step of the history.", schema([]string{"angle"}, map[string]any{
			"angle": prop("number", "degrees clockwise (negative: counter-clockwise)"), "bg": prop("string", "Color of the new corners"), "crop": prop("boolean", "Keep the size of the image (default false)"), "snapshot": pSnap})},
		{"flip_image", "Mirrors the image: horizontal (left to right) and/or vertical (top to bottom). One step of the history.", schema(nil, map[string]any{
			"horizontal": prop("boolean", "Mirror left to right"), "vertical": prop("boolean", "Mirror top to bottom"), "snapshot": pSnap})},
		{"place_image", "Lays an image from a file (png, jpg, gif, bmp, tif, webp) over this image, its top left corner at (x, y), optionally scaled and see-through (a logo, a watermark, a sticker). What hangs over the edge is cut off. One step of the history.", schema([]string{"path", "x", "y"}, map[string]any{
			"path": prop("string", "Full path of the image file"), "x": pX, "y": pY, "scale": prop("number", "1 = the size it has"), "w": prop("integer", "or the width to scale it to (the height follows)"), "h": prop("integer", "or the height (the width follows)"),
			"opacity": pOpacity, "snapshot": pSnap})},
		{"draw_polygon", "Draws a polygon through the points (closed): an outline, or filled with fill_color (also with the outline). Good for arrows, stars, tails, any shape. Can be see-through (opacity) and have a shadow.", schema([]string{"polygon"}, map[string]any{
			"polygon": pPoly, "color": pColor, "width": pWidth, "fill_color": prop("string", "Fill the inside with this color"), "style": map[string]any{"type": "string", "enum": []string{"solid", "dashed", "dotted"}},
			"opacity": pOpacity, "shadow": pShadow, "snapshot": pSnap})},
		{"draw_callout", "Draws a speech bubble: a rounded box at (x, y), w by h, with a tail that points at (tail_x, tail_y), filled (white by default) and outlined. The answer tells where to put the text (add_text with box_w and align center).", schema([]string{"x", "y", "w", "h", "tail_x", "tail_y"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the bubble"), "h": prop("integer", "height of the bubble"), "tail_x": prop("integer", "x the tail points at"), "tail_y": prop("integer", "y the tail points at"),
			"radius": prop("integer", "corner radius (default 12)"), "color": pColor, "fill_color": prop("string", "Fill color (default white)"), "width": pWidth, "opacity": pOpacity, "shadow": pShadow, "snapshot": pSnap})},
		{"clear_selection", "Clears (cuts out) a part of the image: fills it with the window's second (background) color, the way Delete does in Paint. By default it is the part the user selected in the window (the polygon, else the box); or give a box (x, y, w, h), a polygon or a mask. color overrides the second color. One step of the history.", schema(nil, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the box in pixels"), "h": prop("integer", "height of the box in pixels"), "polygon": pPoly, "mask": pMask,
			"color": prop("string", "Fill with this color instead of the window's second color (a name, #rrggbb or #rrggbbaa; transparent makes a hole)"), "feather": pFeather, "snapshot": pSnap})},
		{"cut_selection", "Cuts what the user selected in the window (the polygon, else the box): copies it to the clipboard and clears it with the second color, like Cut in a menu. Fails when nothing is selected. One step of the history.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"crop_to_selection", "Crops the image to what the user selected in the window (the box, or the box round the polygon). Fails when nothing is selected. One step of the history.", schema(nil, map[string]any{"snapshot": pSnap})},
		{"save_selection", "Saves only what the user selected in the window, as an image of its own, to a file (the type is the one the name ends with: png, jpg, gif, bmp, tif); the window keeps its image. Fails when nothing is selected, and will not replace a file that exists unless overwrite is true.", schema([]string{"path"}, map[string]any{
			"path": prop("string", "full path of the file"), "overwrite": prop("boolean", "replace the file if it exists"), "quality": prop("integer", "JPEG quality 1 to 100 (default 95)")})},
		{"print_selection", "Prints what the user selected in the window (or the whole image when nothing is selected) on the default printer. Only do this when the user asked for it: it uses paper.", schema(nil, map[string]any{})},
		{"animation_start", "Makes the image the first frame of an animation (100 ms per frame). The window shows a frame player then. Only needed to say so explicitly: add_frame starts the animation by itself. To make an animated GIF: draw the first frame, then add_frame and draw what changes in it, again and again; the user plays it in the window and saves it as a GIF when it is good. Images up to 2048 pixels on a side, and the frames together up to about 25 million pixels.", schema(nil, map[string]any{})},
		{"add_frame", "Adds a frame to the animation (it starts one when the image is not one yet). By default it is a copy of the last frame, added at the end; the answer gives its number: draw what changes in it with the tools' frame parameter set to that number. The frame the user has on the screen does not change. blank gives an empty (transparent) frame instead, copy_of a copy of another frame, after puts it after that frame number instead of at the end. One step of the history. Frame numbers start at 1.", schema(nil, map[string]any{
			"blank": prop("boolean", "an empty, transparent frame instead of a copy"), "copy_of": prop("integer", "the number of the frame to copy (default: the one it follows)"), "after": prop("integer", "the number of the frame it comes after (default: the last)")})},
		{"duplicate_frame", "Puts a copy of one frame in another place of the animation: source is the number of the frame to copy, target the number the copy has afterwards (the frames from there on move up by one; a target one more than the number of frames puts it at the end). The frame the user has on the screen does not change. One step of the history. Frame numbers start at 1.", schema([]string{"source", "target"}, map[string]any{
			"source": prop("integer", "the number of the frame to copy"), "target": prop("integer", "the number the copy gets")})},
		{"delete_frame", "Takes a frame out of the animation. The animation keeps at least one frame. One step of the history.", schema([]string{"frame"}, map[string]any{"frame": prop("integer", "the number of the frame (the first is 1)")})},
		{"move_frame", "Moves a frame to another place in the animation: from and to are frame numbers (the first is 1). One step of the history.", schema([]string{"from", "to"}, map[string]any{"from": prop("integer", "the number of the frame to move"), "to": prop("integer", "the number it has afterwards")})},
		{"set_frame_delay", "Sets how long a frame is shown, in milliseconds (20 to 60000; 100 is 10 frames a second). Without a frame number, all frames get the delay. One step of the history.", schema([]string{"ms"}, map[string]any{"ms": prop("integer", "milliseconds"), "frame": prop("integer", "the number of the frame (default: all)")})},
		{"animation_info", "Tells whether the image is an animation, how many frames it has, which frame the user has on the screen (only needed when the user asks about the current frame or what they see), and the delay of each frame. Does not change anything.", schema(nil, map[string]any{})},
		{"frames_sheet", "Returns all frames of the animation in one image (rows, left to right and top to bottom in the order of the animation, a magenta grid between them) so that you can check the whole motion at once. Does not change anything.", schema(nil, map[string]any{"max": prop("integer", "longest side of the sheet in pixels (default 1568)")})},
		{"diff_original", "Compares the image as it is now with the file on the disk: whether it changed, how many pixels, the box of the change and how strong it is on average. Use it to check that an edit touched only what you meant. Does not change the image.", schema(nil, map[string]any{})},
		{"list_folder", "Lists the images in the folder of the open file, by file name (not the path), and where the current one is among them. Does not change anything.", schema(nil, map[string]any{})},
		{"list_fonts", "Lists font families that add_text can be asked for (the common ones, and the names of the font files found on this computer). Does not change anything.", schema(nil, map[string]any{})},
		{"goto_step", "Goes to a step of the undo history (the numbers come from history): undoes or redoes as many steps as it takes. Step 0 is the start. One call instead of many undo calls.", schema([]string{"step"}, map[string]any{"step": prop("integer", "the step number"), "snapshot": pSnap})},
		{"checkpoint", "Remembers the image as it is now under a name, so that you can try something and come back with goto_checkpoint. The user's and your later changes are in the history; a checkpoint is lost when the step it points at is forgotten (a new change after an undo drops the steps that were undone).", schema([]string{"name"}, map[string]any{"name": prop("string", "a short name")})},
		{"goto_checkpoint", "Goes back to (or forward to) a named checkpoint made with checkpoint.", schema([]string{"name"}, map[string]any{"name": prop("string", "the name"), "snapshot": pSnap})},
		{"show_area", "Scrolls and zooms the window so that the part x, y, w, h fills it, so that the user sees what you are talking about. With mark: true it is also selected (marching dashes). Does not change the image.", schema([]string{"x", "y", "w", "h"}, map[string]any{
			"x": pX, "y": pY, "w": prop("integer", "width of the part"), "h": prop("integer", "height of the part"), "mark": prop("boolean", "Also select it (default false)")})},
	}
}

// moreCall runs a tool of moreTools; handled says whether the name was one of them.
func (h *chatHub) moreCall(name string, args map[string]any) (content []any, handled bool, err error) {
	switch name {
	case "apply_effect":
		content, err = h.effectTool(args)
	case "clear_selection":
		content, err = h.clearTool(args)
	case "resize_image", "rotate_image", "flip_image", "place_image":
		content, err = h.transformTool(name, args)
	case "cut_selection", "crop_to_selection", "save_selection", "print_selection":
		cmd := map[string]any{"cmd": map[string]string{"cut_selection": "cutsel", "crop_to_selection": "cropsel", "save_selection": "savesel", "print_selection": "printsel"}[name]}
		if name == "save_selection" {
			path := str(args, "path")
			if path == "" {
				return nil, true, fmt.Errorf("\"path\" is missing")
			}
			if fi, e := os.Stat(path); e == nil && !fi.IsDir() && args["overwrite"] != true {
				return nil, true, fmt.Errorf("%s exists: call again with overwrite=true to replace it, or choose another name", path)
			}
			cmd["path"], cmd["overwrite"], cmd["quality"] = path, args["overwrite"] == true, args["quality"]
		}
		var res map[string]any
		if res, err = h.window(cmd); err == nil {
			note, _ := res["note"].(string)
			if note == "" {
				note = "Done: " + name
			}
			content = []any{textContent(note)}
		}
	case "animation_start", "add_frame", "duplicate_frame", "delete_frame", "move_frame", "set_frame_delay", "animation_info", "frames_sheet":
		content, err = h.animationTool(name, args)
	case "diff_original", "list_folder", "list_fonts":
		cmd := map[string]string{"diff_original": "diff", "list_folder": "folder", "list_fonts": "fonts"}[name]
		var res map[string]any
		if res, err = h.window(map[string]any{"cmd": cmd}); err == nil {
			delete(res, "ok")
			content = []any{jsonText(res)}
		}
		return content, true, err
	case "goto_step":
		var n int
		if n, err = intArg(args, "step"); err == nil {
			_, err = h.window(map[string]any{"cmd": "goto", "step": n})
			content = []any{textContent(fmt.Sprintf("Went to step %d of the history", n))}
		}
	case "checkpoint":
		if str(args, "name") == "" {
			return nil, true, fmt.Errorf("\"name\" is missing")
		}
		var res map[string]any
		if res, err = h.window(map[string]any{"cmd": "checkpoint", "name": str(args, "name")}); err == nil {
			content = []any{textContent(fmt.Sprintf("Checkpoint %q is step %v of the history", str(args, "name"), res["step"]))}
		}
		return content, true, err
	case "goto_checkpoint":
		if _, err = h.window(map[string]any{"cmd": "goto", "checkpoint": str(args, "name")}); err == nil {
			content = []any{textContent(fmt.Sprintf("Went back to the checkpoint %q", str(args, "name")))}
		}
	case "show_area":
		var v []int
		if v, err = ints(args, "x", "y", "w", "h"); err == nil {
			if v[2] < 1 || v[3] < 1 {
				return nil, true, fmt.Errorf("w and h must be 1 or more")
			}
			_, err = h.window(map[string]any{"cmd": "show", "x": v[0], "y": v[1], "w": v[2], "h": v[3], "mark": args["mark"] == true})
			content = []any{textContent("The window shows that part now")}
		}
		return content, true, err
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("%s: %w", name, err)
	}
	if args["snapshot"] == true {
		if snap, serr := h.snapshotTool(map[string]any{}); serr == nil {
			content = append(content, snap...)
		}
	}
	return content, true, nil
}

func (h *chatHub) effectTool(args map[string]any) ([]any, error) {
	op := str(args, "effect")
	if op == "" {
		return nil, fmt.Errorf("\"effect\" is missing")
	}
	cmd := map[string]any{"cmd": "effect", "op": op}
	for _, k := range []string{"x", "y", "w", "h", "feather", "radius", "size", "amount", "angle", "cx", "cy", "kind", "opacity", "bar", "gamma", "black", "white", "level", "mask", "selection"} {
		if v, ok := args[k]; ok {
			cmd[k] = v
		}
	}
	for _, k := range []string{"color", "color2"} {
		if s := str(args, k); s != "" {
			c, err := parseColorAlpha(s, "")
			if err != nil {
				return nil, err
			}
			cmd[k] = c
		}
	}
	pts, err := polygonArg(args)
	if err != nil {
		return nil, err
	}
	if pts != nil {
		cmd["pts"] = pts
	}
	res, err := h.window(cmd)
	if err != nil {
		return nil, fmt.Errorf("apply_effect: %w", err)
	}
	note, _ := res["note"].(string)
	if note == "" {
		note = "Applied " + op
	}
	return []any{textContent(note)}, nil
}

func (h *chatHub) clearTool(args map[string]any) ([]any, error) {
	cmd := map[string]any{"cmd": "effect", "op": "fill", "clear": true}
	for _, k := range []string{"x", "y", "w", "h", "mask", "feather"} {
		if v, ok := args[k]; ok {
			cmd[k] = v
		}
	}
	pts, err := polygonArg(args)
	if err != nil {
		return nil, err
	}
	if pts != nil {
		cmd["pts"] = pts
	}
	if _, ok := args["w"]; !ok && pts == nil && args["mask"] == nil {
		cmd["selection"] = true // no part given: what the user selected
	}
	if s := str(args, "color"); s != "" {
		c, err := parseColorAlpha(s, "")
		if err != nil {
			return nil, err
		}
		cmd["color"] = c
	}
	if _, err := h.window(cmd); err != nil {
		return nil, err
	}
	return []any{textContent("Cleared the part (filled with the second color)")}, nil
}

func (h *chatHub) transformTool(name string, args map[string]any) ([]any, error) {
	op := map[string]string{"resize_image": "resize", "rotate_image": "rotate", "flip_image": "flip", "place_image": "place"}[name]
	cmd := map[string]any{"cmd": "transform", "op": op}
	for _, k := range []string{"w", "h", "scale", "filter", "angle", "crop", "horizontal", "vertical", "path", "x", "y", "opacity"} {
		if v, ok := args[k]; ok {
			cmd[k] = v
		}
	}
	if s := str(args, "bg"); s != "" {
		c, err := parseColorAlpha(s, "")
		if err != nil {
			return nil, err
		}
		cmd["bg"] = c
	}
	if _, err := h.window(cmd); err != nil {
		return nil, err
	}
	st, err := h.state()
	if err != nil {
		return []any{textContent("Done: " + name)}, nil
	}
	if pw, ph, err := pictureSize(st); err == nil {
		return []any{textContent(fmt.Sprintf("Done: %s. The image is %d × %d pixels now", name, pw, ph))}, nil
	}
	return []any{textContent("Done: " + name)}, nil
}

// addShapeExtras gives the shapes and the text the options that the extra tools have (corners, see-through, shadow, outline, wrapping).
func addShapeExtras(cmds []map[string]any, args map[string]any) {
	for _, c := range cmds {
		switch c["cmd"] {
		case "shape":
			for _, k := range []string{"radius", "opacity", "shadow"} {
				if v, ok := args[k]; ok {
					c[k] = v
				}
			}
		case "text":
			for _, k := range []string{"align", "box_w", "outline_width", "shadow"} {
				if v, ok := args[k]; ok {
					c[k] = v
				}
			}
			if s := str(args, "outline"); s != "" {
				if oc, err := parseColor(s, ""); err == nil {
					c["outline"] = oc
				}
			}
		}
	}
}

// animationTool is the tools that make an animation: frames are numbered from 1 here, and every drawing tool takes a frame number (see applyFrame).
func (h *chatHub) animationTool(name string, args map[string]any) ([]any, error) {
	info := func() (frames, shown int, delays []any, err error) {
		res, err := h.window(map[string]any{"cmd": "gifinfo"})
		if err != nil {
			return 0, 0, nil, err
		}
		if res["animation"] != true {
			return 0, 0, nil, nil
		}
		f, _ := res["frames"].(float64)
		c, _ := res["frame"].(float64)
		d, _ := res["delays_ms"].([]any)
		return int(f), int(c), d, nil
	}
	gif := func(cmd map[string]any) error {
		cmd["cmd"] = "gif"
		_, err := h.window(cmd)
		return err
	}
	switch name {
	case "animation_info":
		frames, shown, delays, err := info()
		if err != nil {
			return nil, err
		}
		if frames == 0 {
			return []any{jsonText(map[string]any{"animation": false})}, nil
		}
		return []any{jsonText(map[string]any{"animation": true, "frames": frames, "frame_shown": shown, "delays_ms": delays})}, nil
	case "frames_sheet":
		res, err := h.window(map[string]any{"cmd": "gifsheet", "max": args["max"]})
		if err != nil {
			return nil, err
		}
		data, _ := res["png"].(string)
		return []any{map[string]any{"type": "image", "data": data, "mimeType": "image/png"},
			textContent(fmt.Sprintf("%v frames, %v columns × %v rows, in the order of the animation (left to right, top to bottom); each frame is %v × %v pixels, enlarged %v×", res["frames"], res["cols"], res["rows"], res["w"], res["h"], res["zoom"]))}, nil
	case "animation_start":
		if err := gif(map[string]any{"op": "start"}); err != nil {
			return nil, err
		}
		return []any{textContent("The image is frame 1 of an animation now (100 ms per frame). add_frame adds the next frames.")}, nil
	case "add_frame":
		frames, _, _, err := info()
		if err != nil {
			return nil, err
		}
		n := max(frames, 1)
		after := n
		if _, ok := args["after"]; ok {
			if after, err = intArg(args, "after"); err != nil {
				return nil, err
			}
		}
		if after < 1 || after > n {
			return nil, fmt.Errorf("there is no frame %d (the animation has %d)", after, n)
		}
		blank := args["blank"] == true
		from := after
		if _, ok := args["copy_of"]; ok {
			if from, err = intArg(args, "copy_of"); err != nil {
				return nil, err
			}
		}
		if !blank && frames > 0 { // the copy is of the frame it follows (or the one asked for)
			if _, err := h.window(map[string]any{"cmd": "gifframe", "frame": from}); err != nil {
				return nil, err
			}
		}
		if err := gif(map[string]any{"op": "add", "blank": blank, "after": after - 1}); err != nil {
			return nil, err
		}
		what := fmt.Sprintf("a copy of frame %d", from)
		if blank || frames == 0 && false {
			what = "an empty frame"
		}
		return []any{textContent(fmt.Sprintf("Frame %d added (%s). The animation has %d frames. Draw on it with the frame parameter set to %d.", after+1, what, n+1, after+1))}, nil
	case "duplicate_frame":
		v, err := ints(args, "source", "target")
		if err != nil {
			return nil, err
		}
		if err := gif(map[string]any{"op": "duplicate", "source": v[0] - 1, "target": v[1] - 1}); err != nil {
			return nil, err
		}
		return []any{textContent(fmt.Sprintf("Frame %d was copied: the copy is frame %d now (give that number as the frame parameter to draw on it)", v[0], v[1]))}, nil
	case "delete_frame":
		f, err := intArg(args, "frame")
		if err != nil {
			return nil, err
		}
		if err := gif(map[string]any{"op": "delete", "frame": f - 1}); err != nil {
			return nil, err
		}
		return []any{textContent(fmt.Sprintf("Frame %d is gone", f))}, nil
	case "move_frame":
		v, err := ints(args, "from", "to")
		if err != nil {
			return nil, err
		}
		if err := gif(map[string]any{"op": "move", "from": v[0] - 1, "to": v[1] - 1}); err != nil {
			return nil, err
		}
		return []any{textContent(fmt.Sprintf("Frame %d is frame %d now", v[0], v[1]))}, nil
	case "set_frame_delay":
		ms, err := intArg(args, "ms")
		if err != nil {
			return nil, err
		}
		f := 0
		if _, ok := args["frame"]; ok {
			if f, err = intArg(args, "frame"); err != nil {
				return nil, err
			}
		}
		if err := gif(map[string]any{"op": "delay", "frame": f - 1, "ms": ms}); err != nil {
			return nil, err
		}
		return []any{textContent("Delay set")}, nil
	}
	return nil, fmt.Errorf("unknown tool %s", name)
}

// applyFrame lets every tool that works on the picture take a frame number (the first is 1): the frame is made the one the tools work on, without the user's player
// being told to go anywhere. It is for an animation only.
func (h *chatHub) applyFrame(name string, args map[string]any) error {
	switch name {
	case "add_frame", "duplicate_frame", "delete_frame", "move_frame", "set_frame_delay", "animation_info", "animation_start", "frames_sheet", "window_info", "history", "goto_step", "checkpoint", "goto_checkpoint":
		return nil
	}
	v, ok := args["frame"]
	if !ok {
		return nil
	}
	n, err := intArg(args, "frame")
	if err != nil {
		return fmt.Errorf("\"frame\" is %v: give the number of the frame (the first is 1)", v)
	}
	if _, err = h.window(map[string]any{"cmd": "gifframe", "frame": n}); err != nil {
		return err
	}
	h.framed = true // (callTool puts the user's frame back when the tool is done)
	return nil
}

// withFrameParam adds the optional frame number to the tools that read or change pixels.
func withFrameParam(tools []toolDef) []toolDef {
	skip := map[string]bool{"window_info": true, "history": true, "goto_step": true, "checkpoint": true, "goto_checkpoint": true, "animation_start": true, "add_frame": true, "duplicate_frame": true, "delete_frame": true, "move_frame": true,
		"set_frame_delay": true, "animation_info": true, "frames_sheet": true, "list_folder": true, "list_fonts": true, "open_image": true, "save_image": true, "new_image": true, "undo": true, "redo": true,
		"discard_changes": true, "image_info": true, "diff_original": true, "show_area": true, "select_region": true, "copy": true, "paste": true, "save_selection": true, "print_selection": true}
	for i := range tools {
		if skip[tools[i].Name] {
			continue
		}
		if props, ok := tools[i].InputSchema["properties"].(map[string]any); ok {
			props["frame"] = prop("integer", "The frame of an animation to look at or change (the first is 1). Always give it when you work on an animation: without it the tool works on the frame that the user has on the screen. It does not move the user's player.")
		}
	}
	return tools
}
