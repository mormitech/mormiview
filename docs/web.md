# mormiView in the browser

The web version is mormiView as a static web page: no install, no account, no server behind it. It is not a copy of the app — it is the
same program:

- the Go code of the app (`*.go`) is built for WebAssembly (`GOOS=js GOARCH=wasm`) into `mormiview.wasm`; only `main.go` (the Wails
  window) is left out, and `web_main.go` takes its place;
- the page is the app's own `frontend/` (`index.html`, `src/main.js`, `src/chat.js`, the style), started by `frontend/web/boot.js`;
- what Wails does on the desktop is done by the page, in `frontend/web/`.

## Build and run

```
scripts/build_web.sh          # frontend/dist-web: the whole site
scripts/build_web.sh serve    # the same, then served on http://localhost:5180
```

`frontend/dist-web` can be put on any static web host (GitHub Pages, Netlify, S3, nginx…), in the root or in a folder. Serve `.wasm` as
`application/wasm` and, for speed, compressed (gzip or brotli: the 20 MB program becomes about 5 MB). The page needs a secure context
(https, or localhost) for the clipboard.

## How it fits together

| Desktop (Wails) | Web |
|---|---|
| `wails.Run` binds the methods of `App` as `window.go.main.App.*` | `web_main.go` binds them by reflection (`mvCall`); `frontend/web/host.js` makes `window.go.main.App` from them, with the same promises |
| The Wails runtime (dialogs, title, full screen, menu) | `webFrontend` in `web_main.go` is put into the context that `wruntime` reads, and hands each call to the page (`window.mvHost`): `frontend/src/dialogs.js`, `frontend/web/menubar.js` |
| Events (`EventsEmit` → `EventsOn`) | `emitEvent` (app.go) is set to the page's dispatcher; `window.runtime` is the page's own |
| The asset server serves `/image/N` from `imageHandler` | the page asks `imageHandler` for it (`mvServe`) and shows it as a `blob:` URL |
| The disk | `frontend/web/memfs.js`: Node's `fs` API, which Go's `os` uses on WebAssembly, in memory and in IndexedDB (`/home/user`; `/tmp` is not kept). A picture written outside the settings is also downloaded |
| The system clipboard | `readClipboardImage` (clip.go) is set to the browser's clipboard: the picture of a Ctrl+V paste, or `navigator.clipboard.read()` |
| One process per window | a tab per window (File → New Window, About) |
| Chat: Claude Code with a private MCP server | `frontend/src/chat-api.js` (shared with the desktop, where it is one of the providers): the Claude API from the page with the user's API key, or OpenRouter (`frontend/src/openrouter.js`, its chat completions API) with the user's OpenRouter key; the same tools and system prompt through the Go methods of `chat_api.go` |

The shared code knows nothing of the browser except a few small things: `emitEvent` and `readClipboardImage` are variables, the questions
use the macOS form (three buttons) on `js` too, and `chat.js` shows the sign-in text the web version gives it.

## What the browser cannot do

- **Scanning** (no access to a scanner), **control mode** (no server), **Open with…** from the file manager.
- **Full screen** works only right after a key press or a click (the browser's rule): F and Enter do it.
- **Ctrl+W, Ctrl+N, Ctrl+Q** belong to the browser; use the menu (File → New Window) instead of Ctrl+N.
- Files are in the browser's storage, not on the disk: saving downloads them.

## Privacy

Pictures never leave the browser, except when the chat is used: then what the model looks at (snapshots of the picture) goes to the
provider chosen in AI → AI Provider and Key… with the user's own key: the Claude API (api.anthropic.com), or OpenRouter (openrouter.ai,
which passes it on to the model's provider). The keys are kept with the settings in the browser's storage (or only until the page is closed, when "Remember the
keys" is off) and each is sent only to its own provider.
