package main

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// More information for the chat's tools (see mcp.go): what changed against the file, the pictures of the folder, and the fonts the text tool can use.

// diffCommand is the control command "diff": how the picture is different from the file on the disk (the box of the change, how much of it changed).
func (a *App) diffCommand(m map[string]any) (map[string]any, error) {
	a.mu.Lock()
	turn := a.turn
	a.mu.Unlock()
	now, err := a.shownPixels(turn)
	if err != nil {
		return nil, err
	}
	orig, err := a.originalPixels(turn)
	if err != nil {
		return nil, err
	}
	nb, ob := now.Bounds(), orig.Bounds()
	res := map[string]any{"ok": true, "size": []int{nb.Dx(), nb.Dy()}, "originalSize": []int{ob.Dx(), ob.Dy()}}
	if nb.Dx() != ob.Dx() || nb.Dy() != ob.Dy() {
		res["same"] = false
		res["note"] = "the image has another size than the file, so the pixels cannot be compared one by one"
		return res, nil
	}
	changed, sum := 0, 0.0
	box := image.Rectangle{}
	for y := 0; y < nb.Dy(); y++ {
		for x := 0; x < nb.Dx(); x++ {
			i, j := y*now.Stride+4*x, y*orig.Stride+4*x
			d := 0
			for k := 0; k < 4; k++ {
				d = max(d, int(math.Abs(float64(now.Pix[i+k])-float64(orig.Pix[j+k]))))
			}
			if d > 0 {
				changed++
				sum += float64(d)
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	res["same"] = changed == 0
	res["changedPixels"] = changed
	res["changedPct"] = math.Round(1000*float64(changed)/float64(nb.Dx()*nb.Dy())) / 10
	if changed > 0 {
		res["box"] = []int{box.Min.X, box.Min.Y, box.Dx(), box.Dy()}
		res["averageChange"] = math.Round(sum / float64(changed)) // 0 to 255, over the pixels that changed
	}
	return res, nil
}

// folderCommand is the control command "folder": the pictures next to this one, by file name (no path), and the place of this one among them.
func (a *App) folderCommand(m map[string]any) (map[string]any, error) {
	cur := a.current()
	if cur.Path == "" {
		return nil, fmt.Errorf("this image has no file, so there is no folder to list")
	}
	names, err := listPictures(cur.Dir)
	if err != nil {
		return nil, err
	}
	const most = 500
	res := map[string]any{"ok": true, "current": cur.Name, "index": indexOf(names, cur.Name) + 1, "count": len(names)}
	if len(names) > most {
		names = names[:most]
		res["note"] = fmt.Sprintf("the first %d of %d", most, res["count"])
	}
	res["pictures"] = names
	return res, nil
}

// fontNames are the font families the text tool can be asked for: the ones every system has, and the names of the font files found in the fonts folders
// (which are close to the family names, not the same).
func fontNames() []string {
	common := []string{"Arial", "Helvetica", "Times New Roman", "Georgia", "Courier New", "Verdana", "Trebuchet MS", "Impact", "Comic Sans MS", "sans-serif", "serif", "monospace"}
	var dirs []string
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		dirs = []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library", "Fonts")}
	case "windows":
		dirs = []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
	default:
		dirs = []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts")}
	}
	seen := map[string]bool{}
	var found []string
	for _, d := range dirs {
		filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
				return nil
			}
			n := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if k := strings.ToLower(n); !seen[k] && len(found) < 400 {
				seen[k] = true
				found = append(found, n)
			}
			return nil
		})
	}
	sort.Strings(found)
	return append(common, found...)
}
