package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The chat's image_info tool: what the picture is (its file name without the folder, its type and size, the EXIF block of a JPEG), and what is in it
// (how the colors are used). It reads the picture as it is in the window now, with the changes made so far.

// imageInfoCommand answers the control command "imageinfo".
func (a *App) imageInfoCommand(m map[string]any) (map[string]any, error) {
	cur := a.current()
	if cur.Name == "" && cur.URL == "" {
		return nil, fmt.Errorf("this window has no image")
	}
	a.mu.Lock()
	turn := a.turn
	a.mu.Unlock()
	file := map[string]any{"name": cur.Name, "hasFile": cur.Path != "", "unsavedChanges": cur.Modified}
	out := map[string]any{"file": file}
	if cur.Path != "" {
		if fi, err := os.Stat(cur.Path); err == nil {
			file["bytes"] = fi.Size()
			file["fileModified"] = fi.ModTime().Format(time.RFC3339)
		}
		file["type"] = strings.ToLower(strings.TrimPrefix(filepath.Ext(cur.Path), "."))
		if b := readHead(cur.Path, 256*1024); b != nil {
			if ex := readExif(b); len(ex) > 0 {
				out["exif"] = ex
			}
		}
	}
	img, err := a.shownPixels(turn)
	if err != nil {
		out["colors"] = "not available: " + err.Error()
		return out, nil
	}
	out["size"] = map[string]any{"width": img.Bounds().Dx(), "height": img.Bounds().Dy()}
	if _, ok := m["w"]; ok || m["mask"] != nil || m["pts"] != nil { // only a part: a box, a polygon or a mask
		sp := effectSpec{X: num(m, "x", 0), Y: num(m, "y", 0), W: num(m, "w", 0), H: num(m, "h", 0), Mask: str(m, "mask")}
		if pts, ok := m["pts"].([]any); ok {
			for _, p := range pts {
				if xy, ok := p.([]any); ok && len(xy) == 2 {
					x, _ := xy[0].(float64)
					y, _ := xy[1].(float64)
					sp.Pts = append(sp.Pts, [2]int{int(x), int(y)})
				}
			}
		}
		sh, err := a.effectShape(sp, img.Bounds())
		if err != nil {
			return nil, err
		}
		sub := image.NewNRGBA(image.Rect(0, 0, sh.r.Dx(), sh.r.Dy()))
		for y := 0; y < sh.r.Dy(); y++ {
			copy(sub.Pix[y*sub.Stride:y*sub.Stride+4*sh.r.Dx()], img.Pix[(sh.r.Min.Y+y)*img.Stride+4*sh.r.Min.X:])
		}
		sh.r = sh.r.Sub(sh.r.Min) // the box of the shape is the sub picture now
		out["colorsOfPart"] = colorStats(sub, sh)
		return out, nil
	}
	out["colors"] = colorStats(img, nil)
	return out, nil
}

func readHead(path string, n int) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b := make([]byte, n)
	k, _ := f.Read(b)
	return b[:k]
}

// ---- EXIF -------------------------------------------------------------------------------------------------

// readExif reads some tags of the EXIF block of a JPEG (the camera, the time, the exposure). The place of a GPS fix is not given, only that there is one.
func readExif(b []byte) map[string]any {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return nil
	}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return nil
		}
		marker, size := b[i+1], int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xDA || size < 2 {
			return nil
		}
		if marker == 0xE1 && i+2+size <= len(b) && bytes.HasPrefix(b[i+4:], []byte("Exif\x00\x00")) {
			return exifTags(b[i+10 : i+2+size])
		}
		i += 2 + size
	}
	return nil
}

func exifTags(t []byte) map[string]any {
	if len(t) < 8 {
		return nil
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil
	}
	out := map[string]any{}
	type entry struct {
		typ, count uint16
		off        int // where the value is
	}
	sizes := map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 7: 1, 9: 4, 10: 8}
	readIFD := func(at int, each func(tag uint16, e entry)) {
		if at < 0 || at+2 > len(t) {
			return
		}
		n := int(bo.Uint16(t[at:]))
		for k := 0; k < n; k++ {
			p := at + 2 + 12*k
			if p+12 > len(t) {
				return
			}
			e := entry{typ: bo.Uint16(t[p+2:]), count: uint16(min(int(bo.Uint32(t[p+4:])), 65535)), off: p + 8}
			if sz := sizes[e.typ] * int(bo.Uint32(t[p+4:])); sz > 4 {
				e.off = int(bo.Uint32(t[p+8:]))
			}
			each(bo.Uint16(t[p:]), e)
		}
	}
	str := func(e entry) string {
		if e.off < 0 || e.off+int(e.count) > len(t) {
			return ""
		}
		return strings.TrimSpace(strings.TrimRight(string(t[e.off:e.off+int(e.count)]), "\x00"))
	}
	intOf := func(e entry) (int, bool) {
		if e.off < 0 || e.off+2 > len(t) {
			return 0, false
		}
		if e.typ == 3 {
			return int(bo.Uint16(t[e.off:])), true
		}
		if e.typ == 4 && e.off+4 <= len(t) {
			return int(bo.Uint32(t[e.off:])), true
		}
		return 0, false
	}
	rat := func(e entry) (float64, bool) {
		if e.typ != 5 || e.off < 0 || e.off+8 > len(t) {
			return 0, false
		}
		d := bo.Uint32(t[e.off+4:])
		if d == 0 {
			return 0, false
		}
		return float64(bo.Uint32(t[e.off:])) / float64(d), true
	}
	exifIFD, gpsIFD := -1, -1
	readIFD(int(bo.Uint32(t[4:])), func(tag uint16, e entry) {
		switch tag {
		case 0x010F:
			out["make"] = str(e)
		case 0x0110:
			out["model"] = str(e)
		case 0x0131:
			out["software"] = str(e)
		case 0x0132:
			out["dateTime"] = str(e)
		case 0x0112:
			if v, ok := intOf(e); ok {
				out["orientation"] = v
			}
		case 0x8769:
			exifIFD, _ = intOf(e)
		case 0x8825:
			gpsIFD, _ = intOf(e)
		}
	})
	readIFD(exifIFD, func(tag uint16, e entry) {
		switch tag {
		case 0x829A:
			if v, ok := rat(e); ok && v > 0 {
				if v < 1 {
					out["exposureTime"] = fmt.Sprintf("1/%d s", int(math.Round(1/v)))
				} else {
					out["exposureTime"] = fmt.Sprintf("%g s", v)
				}
			}
		case 0x829D:
			if v, ok := rat(e); ok {
				out["fNumber"] = fmt.Sprintf("f/%.1f", v)
			}
		case 0x8827:
			if v, ok := intOf(e); ok {
				out["iso"] = v
			}
		case 0x9003:
			out["dateTimeOriginal"] = str(e)
		case 0x920A:
			if v, ok := rat(e); ok {
				out["focalLength"] = fmt.Sprintf("%.0f mm", v)
			}
		case 0xA434:
			out["lens"] = str(e)
		}
	})
	if gpsIFD > 0 {
		out["hasGPS"] = true
	}
	for k, v := range out {
		if v == "" {
			delete(out, k)
		}
	}
	return out
}

// ---- Colors -----------------------------------------------------------------------------------------------

const uniqueCap = 1 << 20

// colorStats says how (of the part sh of the picture, or all of it when sh is nil) the colors of a picture are used: the number of different colors, the main colors with their share, the average and the
// spread of the brightness, and how much of it is transparent.
func colorStats(img *image.NRGBA, sh *shape) map[string]any {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	total := w * h
	if sh != nil {
		total = 0
	}
	unique := make(map[uint32]struct{})
	capped := false
	type bucket struct{ n, r, g, b int }
	buckets := map[uint16]*bucket{}
	var hist [8]int
	var sumR, sumG, sumB, opaque, clear, partial int
	var sumL, sumL2 float64
	minL, maxL := 255.0, 0.0
	gray := true
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+4*w]
		for x := 0; x < w; x++ {
			if sh != nil {
				if !sh.in(x, y) {
					continue
				}
				total++
			}
			r, g, bl, al := row[4*x], row[4*x+1], row[4*x+2], row[4*x+3]
			if !capped {
				unique[uint32(r)<<24|uint32(g)<<16|uint32(bl)<<8|uint32(al)] = struct{}{}
				capped = len(unique) >= uniqueCap
			}
			if al == 0 {
				clear++
				continue
			}
			if al < 255 {
				partial++
			}
			opaque++
			if r != g || g != bl {
				gray = false
			}
			sumR, sumG, sumB = sumR+int(r), sumG+int(g), sumB+int(bl)
			l := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)
			sumL, sumL2 = sumL+l, sumL2+l*l
			minL, maxL = math.Min(minL, l), math.Max(maxL, l)
			hist[min(int(l)/32, 7)]++
			key := uint16(r>>4)<<8 | uint16(g>>4)<<4 | uint16(bl>>4)
			bk := buckets[key]
			if bk == nil {
				bk = &bucket{}
				buckets[key] = bk
			}
			bk.n++
			bk.r, bk.g, bk.b = bk.r+int(r), bk.g+int(g), bk.b+int(bl)
		}
	}
	pct := func(n, of int) float64 {
		if of == 0 {
			return 0
		}
		return math.Round(1000*float64(n)/float64(of)) / 10
	}
	res := map[string]any{
		"pixels":          total,
		"transparentPct":  pct(clear, total),
		"semiTransparent": pct(partial, total),
	}
	if capped {
		res["differentColors"] = fmt.Sprintf("more than %d", uniqueCap)
	} else {
		res["differentColors"] = len(unique)
	}
	if opaque == 0 {
		return res
	}
	mean := sumL / float64(opaque)
	res["averageColor"] = fmt.Sprintf("#%02X%02X%02X", sumR/opaque, sumG/opaque, sumB/opaque)
	res["grayscale"] = gray
	res["brightness"] = map[string]any{ // 0 (black) to 255 (white)
		"average": math.Round(mean), "spread": math.Round(math.Sqrt(math.Max(sumL2/float64(opaque)-mean*mean, 0))),
		"darkest": math.Round(minL), "brightest": math.Round(maxL),
	}
	hp := make([]float64, 8)
	for i, n := range hist {
		hp[i] = pct(n, opaque)
	}
	res["brightnessHistogramPct"] = hp // eight equal steps from black to white
	list := make([]*bucket, 0, len(buckets))
	for _, bk := range buckets {
		list = append(list, bk)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	var main []map[string]any
	for _, bk := range list[:min(len(list), 8)] {
		main = append(main, map[string]any{"color": fmt.Sprintf("#%02X%02X%02X", bk.r/bk.n, bk.g/bk.n, bk.b/bk.n), "pct": pct(bk.n, opaque)})
	}
	res["mainColors"] = main // the colors of the picture in groups of similar ones, the biggest first
	return res
}
