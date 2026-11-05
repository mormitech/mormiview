package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

// The AI menu holds what the Claude of the chat panel may use besides the picture tools. Every permission is on until the user turns it off, and
// the choice is kept in a small file of the user's settings folder (the fields say "no", so that a missing file or field means on).

type aiSettings struct {
	NoWeb bool `json:"noWeb"` // no web search and fetch

	// Who answers in the chat panel (AI → AI Provider and Key…): "" is Claude Code with the user's Claude subscription (the desktop's
	// default), "anthropic" the Claude API and "openrouter" OpenRouter, both with the user's own API key and called from the page
	// (frontend/src/chat-api.js). The models are the ones chosen for each; the keys are in keys.json (chatKeys).
	Provider        string `json:"provider,omitempty"`
	AnthropicModel  string `json:"anthropicModel,omitempty"`
	OpenRouterModel string `json:"openRouterModel,omitempty"`
}

func aiSettingsPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "ai.json")
	}
	return ""
}

// loadAI is read for every message, so that a change made in another window counts at once.
func loadAI() aiSettings {
	var s aiSettings
	if path := aiSettingsPath(); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, &s)
		}
	}
	return s
}

func saveAI(s aiSettings) {
	path := aiSettingsPath()
	b, err := json.Marshal(s)
	if path == "" || err != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, path)
	}
}

func aiMenu(app *App, m *menu.Menu) {
	ai := m.AddSubmenu("AI")
	ai.AddCheckbox("Web search and fetch", !loadAI().NoWeb, nil, func(cd *menu.CallbackData) {
		s := loadAI()
		s.NoWeb = !cd.MenuItem.Checked
		saveAI(s)
	})
	ai.AddText("AI Provider and Key…", nil, func(*menu.CallbackData) { app.emit("chatProvider") }) // (the page asks: chat-api.js)
}
