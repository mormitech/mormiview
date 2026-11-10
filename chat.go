package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// The chat panel (the side panel of the viewer window): the user's own Claude Code, started as a program in the background, with mormiView's AI connection as
// its only tool. It signs in the way Claude Code does (`claude auth login` opens the browser; the token is kept by Claude Code, mormiView never sees it), and
// it uses the user's subscription, not an API key. Every answer is sent to the page as the event "chat" (see chatEvents).

type chatState struct {
	mu      sync.Mutex
	hub     *chatHub // the private MCP server of the picture tools (see mcp.go), started with the first message
	srv     *http.Server
	url     string
	cmd     *exec.Cmd // the answer that is being worked on
	login   *exec.Cmd // the sign-in that waits for the browser
	session string    // the Claude Code session, so that the next message continues the conversation
	stopped bool
}

// chatDir is the folder Claude Code runs in: nothing of the user's projects is there, and the connection file is.
func chatDir() (string, error) {
	d := filepath.Join(os.TempDir(), "mormiview-chat")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

const chatSystemPrompt = `You are the assistant inside the image editor mormiView. The user is looking at the image of the window you work on; the mormiview tools change that image. ` +
	`Look at the image (snapshot) before you change it and again afterwards to check. Use the mormiview tools for every change to the image. ` +
	`To remove something from a photo, work in rounds as a retoucher: make a mask of it (select_by_color when it has its own color, else create_mask with a polygon; snapshot with mask and zoom shows it tinted), cover it with clone_region from clean places that look like what belongs there (different places, vary scale and angle, blend seamless, feather the edge), check each round with snapshot (zoom, compare) and seam_score, fix seams with blend_edges, and fill small leftovers with fill_region (give source_mask so it copies only from the right material). Stop when it looks natural. ` +
	`When the user speaks of a selection ("the selected part", "this region", "what I selected", "remove this" with something selected in the window), call selection_mask first: it shows you the selection and gives you its mask; look at what is inside it (a label, a pattern, text, an object) and decide whether the selection is the thing itself or has more around it, and then work on the thing: when it has its own color, make a mask of that color with select_by_color and keep only the part inside the selection (edit_mask with intersect_mask). ` +
	`Polygons may be as complicated as the shape needs: tens or hundreds of points along an outline are fine, and polygons joins many separate pieces (all the labels, all the berries) into one mask; do not settle for a rectangle or a rough triangle when the shape is not one. Zoom in with snapshot (zoom, outline) to read coordinates precisely and check the outline before you use it. ` +
	`To remove text: trace the letters themselves. Zoom in on the text (snapshot with zoom), then either use select_by_color on the ink (tolerance and a little grow; usually the most exact) or outline the letters with polygons, and work letter by letter or word by word: one mask for each, then fill_region on it with method smooth and grain true so that each letter is filled from the paper right around it, then look again (snapshot with zoom), fix what is left, and move to the next. Small pieces filled from their own surroundings look natural; one big patch over a whole line does not. The same goes for any small detail (a scratch, a wire, a speck, a logo, a strand of hair, a reflection): outline it exactly and fill it from its own surroundings, one piece at a time, and never remove several things with one big shape. ` +
	`To remove writing, a label, a print or a spot: never cover it with a rectangle. Make a mask that hugs only the thing (select_by_color on its color with a few pixels of grow, or create_mask with a polygon along its outline, or selection_mask when the user selected it), check it with snapshot (mask), then call fill_region on that mask: method smooth with grain true for flat or slowly changing surfaces (paper, post-its, walls, skin, sky), method patch for textures (fabric, grass, water, food), nearest for hard edges between colors. ` +
	`To make an animated GIF: draw the first frame, then add_frame (a copy of the last frame; the answer gives its number) and change only what moves in it (set_pixels, clone_region with to_x/to_y or an angle, draw tools), again and again; always pass the frame number as the frame parameter of the tools, which then work on that frame whatever the user has on the screen (animation_info tells which frame the user looks at, for "this frame"), duplicate_frame, move_frame and delete_frame reorder, set_frame_delay sets the speed, frames_sheet shows all frames at once so you can check the motion. Plan the motion first (how many frames, how far each part moves per frame, and that the last frame leads back to the first). Leave the animation ready in the window: the user plays it in the frame player and saves it themselves. ` +
	`The user may mark a part of the image with a highlight drawn in mormiView (a rectangle, circle, arrow, scribble or the like, usually in a strong color) and then ask you to change, remove or fix "this", "what I marked" or "the highlighted part", with or without saying how it is marked. ` +
	`Then: first take a snapshot and find the marking (say which shape you found if the user did not), and note exactly which area it points at, its box and what is inside it, before anything changes. ` +
	`The marking is not part of the image the user wants to keep: if it was the last step in the window, call undo once to take it away, take another snapshot, and check that only the marking is gone; if the undo took away something else, call redo. ` +
	`If the marking is not the last step, do not undo; leave the earlier edits alone and cover the marking as part of your change. Then make the change on the area you noted. If you cannot tell what is marked, ask the user instead of guessing. ` +
	`To hide something (a face, a plate, a name, a screen) use apply_effect censor on a polygon or mask that hugs it; to point at something use apply_effect highlight on it; blur, pixelize, grayscale, negative, sepia, emboss, explosion and the color adjustments (brightness, contrast, saturation, temperature, hue, levels) work on a box, a polygon, a mask or the user's selection the same way. For a speech bubble use draw_callout and then add_text inside it. After a change you can check that only the intended area changed with diff_original. ` +
	`Answer in the language of the user, briefly and plainly.`

// chatArgs are the arguments of the program for one message. The message itself goes in through the standard input.
func chatArgs(session, mcpFile string, web bool) []string {
	tools, allowed := "", "mcp__"+mcpServerName
	if web { // (AI → Web search and fetch)
		tools, allowed = "WebSearch,WebFetch", allowed+",WebSearch,WebFetch"
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--strict-mcp-config", "--mcp-config", mcpFile,
		"--tools", tools, // no file or shell tools: the picture, and the web when the AI menu allows it, are all it can touch
		"--allowedTools", allowed,
		"--append-system-prompt", chatSystemPrompt,
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	return args
}

// ChatInfo is what the panel needs to know to show the right thing: is Claude Code there, is the user signed in.
func (a *App) ChatInfo() map[string]any {
	info := map[string]any{"claude": false, "loggedIn": false}
	claude := findClaude()
	if claude == "" {
		return info
	}
	info["claude"] = true
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, claude, "auth", "status").Output()
	var st struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		Email            string `json:"email"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if json.Unmarshal(out, &st) != nil && err != nil { // (the program says "not logged in" with a failing exit code, with the JSON or without)
		return info
	}
	info["loggedIn"], info["method"], info["email"], info["plan"] = st.LoggedIn, st.AuthMethod, st.Email, st.SubscriptionType
	if demoDir != "" { // a demo run (screenshots) shows nothing of the account
		info["email"], info["plan"] = "", ""
	}
	a.chat.mu.Lock()
	info["busy"], info["signingIn"] = a.chat.cmd != nil, a.chat.login != nil
	a.chat.mu.Unlock()
	return info
}

var urlRe = regexp.MustCompile(`https://[^\s"'<>]+`)

// ChatLogin signs the user in to Claude Code with their Claude subscription: `claude auth login` opens a browser tab, and Claude Code keeps the
// token it gets there. It returns at once; the event "chat" {kind: "login", ok, message} comes when the sign-in is over, and {kind: "loginurl"} gives
// the address for the case that no browser opens.
func (a *App) ChatLogin() error {
	claude := findClaude()
	if claude == "" {
		return fmt.Errorf("Claude Code is not installed")
	}
	a.chat.mu.Lock()
	if a.chat.login != nil {
		a.chat.mu.Unlock()
		return fmt.Errorf("the sign-in is open already: finish it in the browser")
	}
	cmd := exec.Command(claude, "auth", "login", "--claudeai")
	out, err := cmd.StdoutPipe()
	if err != nil {
		a.chat.mu.Unlock()
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		a.chat.mu.Unlock()
		return err
	}
	a.chat.login = cmd
	a.chat.mu.Unlock()
	go func() {
		var tail []string
		sc := bufio.NewScanner(out)
		told := false
		for sc.Scan() {
			line := sc.Text()
			if u := urlRe.FindString(line); u != "" && !told {
				told = true
				a.emit("chat", map[string]any{"kind": "loginurl", "url": u})
			}
			if tail = append(tail, line); len(tail) > 5 {
				tail = tail[1:]
			}
		}
		werr := cmd.Wait()
		a.chat.mu.Lock()
		a.chat.login = nil
		a.chat.mu.Unlock()
		ev := map[string]any{"kind": "login", "ok": werr == nil}
		if werr != nil {
			ev["message"] = strings.TrimSpace(strings.Join(tail, "\n"))
		}
		a.emit("chat", ev)
	}()
	return nil
}

// ChatSend sends a message and starts the answer; the page gets it as the events "chat" until {kind: "end"}.
func (a *App) ChatSend(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("there is nothing to send")
	}
	claude := findClaude()
	if claude == "" {
		return fmt.Errorf("Claude Code is not installed")
	}
	dir, err := chatDir()
	if err != nil {
		return err
	}
	url, token, err := a.startChatTools()
	if err != nil {
		return err
	}
	cfg := map[string]any{"mcpServers": map[string]any{mcpServerName: map[string]any{
		"type": "http", "url": url, "headers": map[string]string{"Authorization": "Bearer " + token},
	}}}
	raw, _ := json.Marshal(cfg)
	file := filepath.Join(dir, fmt.Sprintf("mcp-%d.json", os.Getpid()))
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		return err
	}
	a.chat.mu.Lock()
	if a.chat.cmd != nil {
		a.chat.mu.Unlock()
		return fmt.Errorf("an answer is being written: wait for it, or stop it")
	}
	cmd := exec.Command(claude, chatArgs(a.chat.session, file, !loadAI().NoWeb)...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.StdoutPipe()
	if err != nil {
		a.chat.mu.Unlock()
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, n: 8192}
	if err := cmd.Start(); err != nil {
		a.chat.mu.Unlock()
		return err
	}
	a.chat.cmd, a.chat.stopped = cmd, false
	a.chat.mu.Unlock()
	go a.chatRun(cmd, out, &stderr, file)
	return nil
}

// chatRun reads what the program writes, line by line, and gives the page the events.
func (a *App) chatRun(cmd *exec.Cmd, out io.Reader, stderr *bytes.Buffer, file string) {
	r := bufio.NewReaderSize(out, 1<<20)
	gotResult := false
	for {
		line, err := r.ReadBytes('\n') // (a line can hold a whole picture, so no Scanner)
		if len(bytes.TrimSpace(line)) > 0 {
			var head struct {
				Type      string `json:"type"`
				Subtype   string `json:"subtype"`
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(line, &head) == nil {
				if head.SessionID != "" && head.Type != "stream_event" {
					a.chat.mu.Lock()
					a.chat.session = head.SessionID
					a.chat.mu.Unlock()
				}
				gotResult = gotResult || head.Type == "result"
			}
			for _, ev := range chatEvents(line) {
				a.emit("chat", ev)
			}
		}
		if err != nil {
			break
		}
	}
	werr := cmd.Wait()
	os.Remove(file)
	a.chat.mu.Lock()
	stopped := a.chat.stopped
	a.chat.cmd = nil
	a.chat.mu.Unlock()
	if !gotResult && !stopped {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && werr != nil {
			msg = werr.Error()
		}
		if msg == "" {
			msg = "Claude Code stopped without an answer"
		}
		a.chat.mu.Lock()
		a.chat.session = "" // (a session that could not be continued starts again)
		a.chat.mu.Unlock()
		a.emit("chat", map[string]any{"kind": "error", "text": msg})
	}
	a.emit("chat", map[string]any{"kind": "end", "stopped": stopped})
}

// ChatStop stops the answer that is being written, or the sign-in that waits for the browser.
func (a *App) ChatStop() {
	a.chat.mu.Lock()
	a.chat.stopped = true
	cmd, login := a.chat.cmd, a.chat.login
	a.chat.mu.Unlock()
	for _, c := range []*exec.Cmd{cmd, login} {
		if c != nil && c.Process != nil {
			c.Process.Kill()
		}
	}
}

// ChatReset starts a new conversation.
func (a *App) ChatReset() {
	a.ChatStop()
	a.chat.mu.Lock()
	a.chat.session = ""
	a.chat.mu.Unlock()
}

// limitedWriter keeps the first n bytes of what the program says on its error output.
type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		l.w.Write(p[:k])
		l.n -= k
	}
	return len(p), nil
}

// chatEvents turns one line of the program's stream (stream-json) into what the page shows:
//
//	{kind: "start"}                       a new answer begins
//	{kind: "delta", text}                 a piece of the text, as it is written
//	{kind: "text", text}                  the whole text of a block (for when no pieces came)
//	{kind: "tool", name, input}           a tool of mormiView that is used
//	{kind: "toolerror", text}             a tool that failed
//	{kind: "done", error, text, cost}     the answer is over
func chatEvents(line []byte) []map[string]any {
	var m struct {
		Type    string          `json:"type"`
		Event   json.RawMessage `json:"event"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		IsError bool            `json:"is_error"`
		Result  string          `json:"result"`
		Cost    float64         `json:"total_cost_usd"`
		Parent  json.RawMessage `json:"parent_tool_use_id"`
	}
	if json.Unmarshal(line, &m) != nil {
		return nil
	}
	switch m.Type {
	case "stream_event":
		var e struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal(m.Event, &e) != nil {
			return nil
		}
		switch {
		case e.Type == "message_start":
			return []map[string]any{{"kind": "start"}}
		case e.Type == "content_block_delta" && e.Delta.Type == "text_delta" && e.Delta.Text != "":
			return []map[string]any{{"kind": "delta", "text": e.Delta.Text}}
		}
	case "assistant":
		var blocks []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if json.Unmarshal(m.Message.Content, &blocks) != nil {
			return nil
		}
		var out []map[string]any
		for _, b := range blocks {
			switch b.Type {
			case "text":
				out = append(out, map[string]any{"kind": "text", "text": b.Text})
			case "tool_use":
				out = append(out, map[string]any{"kind": "tool", "name": strings.TrimPrefix(b.Name, "mcp__"+mcpServerName+"__"), "input": shortJSON(b.Input, 160)})
			}
		}
		return out
	case "user":
		var blocks []struct {
			Type    string          `json:"type"`
			IsError bool            `json:"is_error"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(m.Message.Content, &blocks) != nil {
			return nil
		}
		var out []map[string]any
		for _, b := range blocks {
			if b.Type == "tool_result" && b.IsError {
				out = append(out, map[string]any{"kind": "toolerror", "text": toolResultText(b.Content)})
			}
		}
		return out
	case "result":
		return []map[string]any{{"kind": "done", "error": m.IsError, "text": m.Result, "cost": m.Cost}}
	}
	return nil
}

// toolResultText is the words of a tool result (a text, or a list of parts of which the texts count).
func toolResultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var t []string
		for _, p := range parts {
			if p.Type == "text" {
				t = append(t, p.Text)
			}
		}
		return strings.Join(t, "\n")
	}
	return ""
}

func shortJSON(raw json.RawMessage, n int) string {
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return ""
	}
	s := buf.String()
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// findClaude is the path of Claude Code's command. An app started from the Finder has a short PATH, so the usual places and the login shell are asked too.
func findClaude() string {
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".local", "bin", "claude"), filepath.Join(home, ".claude", "local", "claude"), filepath.Join(home, ".npm-global", "bin", "claude"),
		"/opt/homebrew/bin/claude", "/usr/local/bin/claude",
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	if runtime.GOOS != "windows" {
		sh := os.Getenv("SHELL")
		if sh == "" {
			sh = "/bin/sh"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, sh, "-lc", "command -v claude").Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" && filepath.IsAbs(p) {
				return p
			}
		}
	}
	return ""
}

// startChatTools starts the private MCP server of the picture tools for this window (once) and returns its address and its token. It listens on a free port
// of 127.0.0.1, answers only requests with the token, and ends with the window (stopChat).
func (a *App) startChatTools() (url, token string, err error) {
	a.chat.mu.Lock()
	defer a.chat.mu.Unlock()
	if a.chat.hub != nil && a.chat.srv != nil {
		return a.chat.url, a.chat.hub.token, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", err
	}
	c := a.windowCommands()
	h := &chatHub{token: newToken() + newToken(), do: c.do}
	a.chat.hub = h
	a.chat.srv = &http.Server{Handler: h.handler()}
	a.chat.url = "http://" + ln.Addr().String() + "/mcp"
	go a.chat.srv.Serve(ln)
	return a.chat.url, h.token, nil
}

// stopChat ends what the chat panel started: the answer that is being written, the sign-in, and the server of the tools.
func (a *App) stopChat() {
	a.ChatStop()
	a.chat.mu.Lock()
	srv := a.chat.srv
	a.chat.hub, a.chat.srv = nil, nil
	a.chat.mu.Unlock()
	if srv != nil {
		srv.Close()
	}
}
