package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const isWeb = runtime.GOOS == "js"

// The chat panel can also talk to a model API itself, from the page (frontend/src/chat-api.js): the Claude API or OpenRouter, with the
// user's own API key. Then the page runs the conversation, and the Go side only gives it the picture tools (the very same as Claude Code
// gets through the MCP server, mcp.go), the system prompt and the settings. The web version always works this way (it cannot start
// Claude Code); the desktop app does when that provider is chosen in AI → AI Provider and Key….

// chatKeys are the API keys, in a file of their own that only the user can read.
type chatKeys struct {
	Anthropic  string `json:"anthropic,omitempty"`
	OpenRouter string `json:"openRouter,omitempty"`
}

func chatKeysPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "keys.json")
	}
	return ""
}

func loadChatKeys() chatKeys {
	var k chatKeys
	if path := chatKeysPath(); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, &k)
		}
	}
	return k
}

func saveChatKeys(k chatKeys) error {
	path := chatKeysPath()
	if path == "" {
		return fmt.Errorf("there is no settings folder")
	}
	if k == (chatKeys{}) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	b, err := json.Marshal(k)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ChatAPIConfig is the AI settings and the remembered keys, for the page.
func (a *App) ChatAPIConfig() map[string]any {
	s, k := loadAI(), loadChatKeys()
	return map[string]any{
		"noWeb": s.NoWeb, "provider": s.Provider, "anthropicModel": s.AnthropicModel, "openRouterModel": s.OpenRouterModel,
		"anthropicKey": k.Anthropic, "openRouterKey": k.OpenRouter, "desktop": !isWeb,
	}
}

// SetChatAPIConfig keeps what was chosen in AI → AI Provider and Key…: the provider, the models, and the keys to remember (an empty key
// is forgotten; a key that is not to be remembered stays in the page only).
func (a *App) SetChatAPIConfig(provider, anthropicModel, openRouterModel, anthropicKey, openRouterKey string) error {
	switch provider {
	case "", "anthropic", "openrouter":
	default:
		return fmt.Errorf("there is no provider %q", provider)
	}
	if isWeb && provider == "" {
		provider = "anthropic" // (no Claude Code in a browser)
	}
	s := loadAI()
	s.Provider, s.AnthropicModel, s.OpenRouterModel = provider, anthropicModel, openRouterModel
	saveAI(s)
	if provider != "" {
		a.ChatStop() // (a Claude Code answer that is being written does not go on under another provider)
	}
	return saveChatKeys(chatKeys{Anthropic: anthropicKey, OpenRouter: openRouterKey})
}

// ChatTools is the list of the picture tools (the MCP tool definitions, as JSON).
func (a *App) ChatTools() string {
	b, _ := json.Marshal(mcpTools())
	return string(b)
}

// ChatPrompt is the system prompt of the chat (the one Claude Code gets appended).
func (a *App) ChatPrompt() string { return chatSystemPrompt }

// ChatTool calls one picture tool with its arguments (a JSON object) and returns its content (MCP content: text and images, as JSON).
func (a *App) ChatTool(name, argsJSON string) (res string, err error) {
	var in map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s failed: %v", name, r)
		}
	}()
	content, err := a.toolHub().callTool(name, in)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(content)
	return string(b), err
}

// toolHub is the hub of the picture tools for calls from the page (the MCP server's, when Claude Code has started it: one tool at a time
// works on the window either way).
func (a *App) toolHub() *chatHub {
	a.chat.mu.Lock()
	defer a.chat.mu.Unlock()
	if a.chat.hub == nil {
		a.chat.hub = &chatHub{do: a.windowCommands().do}
	}
	return a.chat.hub
}
