package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// listPictures is the names of the pictures in a folder, in alphabetical order (upper and lower case alike). Folders, hidden
// files (and the "._" files macOS leaves on other disks) and everything that is not a picture type are left out.
func listPictures(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !isImage(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := strings.ToLower(names[i]), strings.ToLower(names[j])
		if a != b {
			return a < b
		}
		return names[i] < names[j]
	})
	return names, nil
}

func indexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return -1
}

// Step shows the previous (delta -1) or the next (+1) picture of the folder of the current one. It goes round: after the last
// picture comes the first, and before the first the last. The returned note is for the UI to show (there is none at the moment).
func (a *App) Step(delta int) (string, error) {
	cur := a.current()
	if cur.Path == "" { // a pasted or a new picture is in no folder: there is nothing to go to, and nothing is said
		return "", nil
	}
	if !a.discardOK("go to " + map[bool]string{true: "the next", false: "the previous"}[delta > 0] + " image") {
		return "", nil
	}
	cur = a.current() // saving may have made the picture a file in another folder
	names, err := listPictures(cur.Dir)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "No images in this folder", nil
	}
	i := indexOf(names, cur.Name)
	if i < 0 { // the picture is gone from the folder: start from the first
		return "", a.setImage(filepath.Join(cur.Dir, names[0]))
	}
	if len(names) == 1 {
		return "", nil
	}
	j := ((i+delta)%len(names) + len(names)) % len(names)
	return "", a.setImage(filepath.Join(cur.Dir, names[j]))
}
