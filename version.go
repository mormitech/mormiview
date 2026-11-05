package main

import (
	_ "embed"
	"encoding/json"
)

// wails.json is the single source of the version: it ends up in the macOS Info.plist and the Windows file properties,
// and is read from here for the window title and the UI.
//
//go:embed wails.json
var projectJSON []byte

func version() string {
	var p struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if json.Unmarshal(projectJSON, &p) != nil || p.Info.ProductVersion == "" {
		return "dev"
	}
	return p.Info.ProductVersion
}
