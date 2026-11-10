package main

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// scanCommand is the program that asks a scanner for a picture and leaves it as a PNG file at path (nil when the system has no such thing).
func scanCommand(goos, path string) *exec.Cmd {
	switch goos {
	case "windows":
		script := `$d = New-Object -ComObject WIA.CommonDialog; ` +
			`$i = $d.ShowAcquireImage(1, 1, 65536, '{B96B3CAF-0728-11D3-9D7B-0000F81EF32E}', $false, $true, $false); ` +
			`if ($i -ne $null) { $i.SaveFile('` + strings.ReplaceAll(path, "'", "''") + `') }`
		return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	case "linux":
		return exec.Command("sh", "-c", `scanimage --format=png > "$1"`, "sh", path)
	}
	return nil
}

// ScanImage asks a scanner for a picture (Windows: the system scan dialog, Linux: SANE's scanimage) and opens it as a new, unsaved picture.
// macOS has no scan command: Image Capture is opened and the first new picture file in the usual folders is opened.
func (a *App) ScanImage() error {
	if runtime.GOOS == "darwin" { // Image Capture saves the scan as a file: mormiView opens the first new picture that turns up in the usual places
		home, _ := os.UserHomeDir()
		since := time.Now()
		wasOpen := exec.Command("pgrep", "-x", "Image Capture").Run() == nil
		if err := exec.Command("open", "-a", "Image Capture").Run(); err != nil {
			return errors.New("Image Capture could not be opened")
		}
		dirs := []string{"Pictures", "Desktop", "Documents", "Downloads"}
		for i := range dirs {
			dirs[i] = filepath.Join(home, dirs[i])
		}
		if path := waitForScan(dirs, since, 10*time.Minute, time.Second); path != "" {
			if !wasOpen { // the scan app was started for this: it goes away, and mormiView has the focus again
				_ = exec.Command("osascript", "-e", `tell application "Image Capture" to quit`).Run()
			}
			if a.ctx != nil {
				wruntime.WindowShow(a.ctx)
			}
			if f, err := os.Open(path); err == nil {
				img, _, derr := image.Decode(f)
				f.Close()
				if derr == nil {
					return a.setScan(img, path)
				}
			}
			return a.setImage(path) // (a type that is not decoded here)
		}
		return nil
	}
	dir, err := os.MkdirTemp("", "mormiview-scan")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "scan.png")
	cmd := scanCommand(runtime.GOOS, path)
	if cmd == nil {
		return errors.New("scanning is not available on this system")
	}
	var out bytes.Buffer
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(out.String()); msg != "" {
			return errors.New("the scan failed: " + msg)
		}
		return errors.New("the scan failed (is a scanner connected and, on Linux, is SANE installed?)")
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("nothing was scanned")
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return errors.New("nothing was scanned")
	}
	return a.setScan(img, "")
}

// setScan makes a scanned picture the picture of the window and counts it as a change that is not saved: a scan took time and a scanner, so closing it asks first
// (file is the file the scan was saved as, or "" for one that is only in memory).
func (a *App) setScan(img image.Image, file string) error {
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 || b.Dx()*b.Dy() > maxPixels {
		return errors.New("the scanned image is too small or too big")
	}
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	a.mu.Lock()
	a.gifs = nil // (a scan is a picture of its own)
	a.origin = "scan"
	a.mu.Unlock()
	if err := a.setPixelsFull(n, file, true, false); err != nil {
		return err
	}
	a.mu.Lock()
	a.origImg = n // (Discard changes goes back to the scan)
	a.mu.Unlock()
	return nil
}

var scanExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".tif": true, ".tiff": true, ".heic": true}

// waitForScan waits (up to timeout) for a picture file that is newer than since to appear in dirs and to stop growing, and returns its path ("" when none came).
func waitForScan(dirs []string, since time.Time, timeout, every time.Duration) string {
	end := time.Now().Add(timeout)
	sizes := map[string]int64{}
	for time.Now().Before(end) {
		for _, d := range dirs {
			ents, _ := os.ReadDir(d)
			for _, e := range ents {
				info, err := e.Info()
				if err != nil || e.IsDir() || !scanExts[strings.ToLower(filepath.Ext(e.Name()))] || !info.ModTime().After(since) || info.Size() == 0 {
					continue
				}
				p := filepath.Join(d, e.Name())
				if prev, ok := sizes[p]; ok && prev == info.Size() {
					return p
				}
				sizes[p] = info.Size()
			}
		}
		time.Sleep(every)
	}
	return ""
}
