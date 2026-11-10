// The page's side of the web version (see web_main.go): what Wails does on the desktop.
//
//   window.go.main.App.*  the methods of App, as the Wails bindings (wailsjs/go/main/App.js) call them, answered by the Go program;
//   window.runtime.*      the Wails runtime as wailsjs/runtime/runtime.js calls it (events, the browser, the window);
//   window.mvHost.*       what the Go program asks of the page (dialogs, the menu bar, full screen, the clipboard, new tabs).
//
// The picture URLs Go gives out (/image/N, which the asset server serves on the desktop) become blob: URLs of what imageHandler serves.

import {PICTURES, DOWNLOADS, writeFile, freeName, flush, base, setDownloadHandler} from './memfs.js';
import {renderMenu} from './menubar.js';
import {messageDialog, saveDialog, pickerDialog} from '../src/dialogs.js';
import {zip} from './zip.js';

const MAC = navigator.platform.startsWith('Mac');
export const IMAGE_EXTS = ['.jpg', '.jpeg', '.jpe', '.jfif', '.png', '.gif', '.bmp', '.webp', '.ico', '.svg', '.avif', '.tif', '.tiff'];
const isImageName = (n) => IMAGE_EXTS.some((e) => n.toLowerCase().endsWith(e));

// ---- Events --------------------------------------------------------------------------------------------------------------------

const listeners = new Map(); // name → [{cb, left}]

function on(name, cb, max) {
  const l = {cb, left: max};
  if (!listeners.has(name)) listeners.set(name, []);
  listeners.get(name).push(l);
  return () => { const a = listeners.get(name); if (a) listeners.set(name, a.filter((x) => x !== l)); };
}

function dispatch(name, data) {
  for (const l of [...(listeners.get(name) || [])]) {
    if (l.left > 0 && --l.left === 0) listeners.set(name, listeners.get(name).filter((x) => x !== l));
    try { l.cb(...data); } catch (e) { console.error(e); }
  }
}

// ---- Picture URLs ----------------------------------------------------------------------------------------------------------------

const served = new Map(); // /image/N → blob: URL
const SERVED_MAX = 16;

function serve(path) {
  if (served.has(path)) return served.get(path);
  const p = new Promise((resolve) => {
    window.mvServe(path, (status, type, bytes) => {
      if (status !== 200) { served.delete(path); resolve(path); return; }
      resolve(URL.createObjectURL(new Blob([bytes], {type: type || 'application/octet-stream'})));
    });
  });
  served.set(path, p);
  if (served.size > SERVED_MAX) { // the oldest are let go
    const [k, v] = served.entries().next().value;
    served.delete(k);
    v.then((u) => { if (u.startsWith('blob:')) setTimeout(() => URL.revokeObjectURL(u), 30000); });
  }
  return p;
}

async function fixUrls(v) {
  if (Array.isArray(v)) { for (let i = 0; i < v.length; i++) v[i] = await fixUrls(v[i]); return v; }
  if (v && typeof v === 'object') {
    for (const k of Object.keys(v)) {
      if (k === 'url' && typeof v[k] === 'string' && /^\/image\/\d+$/.test(v[k])) v[k] = await serve(v[k]);
      else if (v[k] && typeof v[k] === 'object') v[k] = await fixUrls(v[k]);
    }
  }
  return v;
}

// Go's events, in order: each waits for the one before it (the URLs of a picture are made first).
let eventChain = Promise.resolve();
function emitFromGo(name, json) {
  const data = JSON.parse(json || '[]') || []; // (an event without data comes as null)
  eventChain = eventChain.then(() => fixUrls(data)).then((d) => dispatch(name, d)).catch((e) => console.error(e));
}

// ---- Bindings -----------------------------------------------------------------------------------------------------------------

function callGo(name, args) {
  return new Promise((resolve, reject) => {
    window.mvCall(name, JSON.stringify(args), (err, res) => {
      if (err != null) { reject(err); return; }
      fixUrls(JSON.parse(res)).then(resolve, reject);
    });
  });
}

// What a page cannot do the way the desktop does it.
const overrides = {
  async PrintPNG(b64) { await printImage(`data:image/png;base64,${b64}`); return ''; },
  async ScanImage() { throw 'Scanning needs the desktop app: a web page cannot reach a scanner'; },
  async CopyImageNative() { throw 'The browser did not let the page copy the image'; },
  async ClipboardHasImage() {
    if (pasted && Date.now() - pasted.at < 60000) return true;
    try {
      const st = await navigator.permissions.query({name: 'clipboard-read'});
      if (st.state === 'denied') return false;
      if (st.state === 'prompt') return true; // (asking would put up the browser's question now: the button asks when it is used)
      const items = await navigator.clipboard.read();
      return items.some((it) => it.types.some((t) => t.startsWith('image/')));
    } catch { return false; }
  },
  // Full screen is asked for by the page at once, inside the key press or the click: the browser allows it only then (a round trip
  // through Go is too late).
  ToggleFullscreen() { fullscreen(!document.fullscreenElement); return Promise.resolve(); },
  ExitFullscreen() { fullscreen(false); return Promise.resolve(); },
  async IsFullscreen() { return !!document.fullscreenElement; },
  async SetMenuBar(show) { document.body.classList.toggle('nomenu', !show); },
  async SaveWindow() {},
  async Reveal() {},
};

// ---- Opening files ---------------------------------------------------------------------------------------------------------------

// importFiles copies picked or dropped files into the browser's file system (a folder of their own when they came as a folder) and
// returns their paths, pictures first in the order of their names.
async function importFiles(files, dir = PICTURES) {
  const paths = [];
  for (const f of files) {
    const rel = f.webkitRelativePath || f.relPath || f.name;
    if (!isImageName(rel)) continue;
    const parts = rel.split('/');
    const d = parts.length > 1 ? `${dir}/${parts.slice(0, -1).join('/')}` : dir;
    paths.push(writeFile(`${d}/${parts.at(-1)}`, new Uint8Array(await f.arrayBuffer())));
  }
  await flush();
  return paths.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
}

function pick({multiple = true, folder = false} = {}) {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = IMAGE_EXTS.join(',');
    input.multiple = multiple;
    if (folder) input.webkitdirectory = true;
    input.style.display = 'none';
    document.body.append(input);
    let done = false;
    const finish = (v) => { if (done) return; done = true; input.remove(); resolve(v); };
    input.addEventListener('change', () => finish([...input.files]));
    input.addEventListener('cancel', () => finish([]));
    const open = () => input.click();
    if (navigator.userActivation?.isActive === false) { // too long after the click or the key: the browser shows no file chooser now
      pickerDialog(folder ? 'Open a folder of images' : 'Open an image', folder ? 'Choose folder…' : 'Choose file…').then((ok) => ok ? open() : finish([]));
    } else open();
  });
}

async function openFiles(folder) {
  const files = await pick({folder});
  if (!files.length) return '';
  const paths = await importFiles(files);
  if (!paths.length) { dispatch('problem', ['None of those files is an image type mormiView opens']); return ''; }
  return paths[0];
}

// File → Open Folder… (web only): all the pictures of a folder, so that the arrow keys go through them.
export async function openFolder() {
  const p = await openFiles(true);
  if (p) await window.go.main.App.OpenRecent(p).catch((e) => dispatch('problem', [String(e)]));
}

// Pictures dropped on the window open the way Open does (the first of them; the others are in the same folder for the arrow keys).
async function filesFromDrop(dt) {
  const out = [];
  const walk = async (entry, prefix) => {
    if (entry.isFile) {
      const f = await new Promise((res, rej) => entry.file(res, rej));
      f.relPath = prefix + f.name;
      out.push(f);
    } else if (entry.isDirectory) {
      const reader = entry.createReader();
      for (;;) {
        const batch = await new Promise((res, rej) => reader.readEntries(res, rej));
        if (!batch.length) break;
        for (const e of batch) await walk(e, `${prefix}${entry.name}/`);
      }
    }
  };
  const entries = [...dt.items].map((it) => it.webkitGetAsEntry?.()).filter(Boolean);
  if (entries.length) for (const e of entries) await walk(e, '');
  else out.push(...dt.files);
  return out;
}

function initDrop() {
  window.addEventListener('dragover', (ev) => { if (ev.dataTransfer?.types.includes('Files')) { ev.preventDefault(); document.body.classList.add('dropping'); } });
  window.addEventListener('dragleave', (ev) => { if (!ev.relatedTarget) document.body.classList.remove('dropping'); });
  window.addEventListener('drop', async (ev) => {
    document.body.classList.remove('dropping');
    if (!ev.dataTransfer?.types.includes('Files')) return;
    ev.preventDefault();
    const paths = await importFiles(await filesFromDrop(ev.dataTransfer));
    if (!paths.length) { dispatch('problem', ['None of those files is an image type mormiView opens']); return; }
    window.go.main.App.OpenRecent(paths[0]).catch((e) => dispatch('problem', [String(e)]));
  });
}

// ---- The clipboard ---------------------------------------------------------------------------------------------------------------
// Ctrl+V (Cmd+V) lets the browser's paste happen, whose picture the page may read without asking; the picture is kept for Go, which
// asks for it (clipboardImage) when the page pastes. The menu's Paste and the home screen's button read the clipboard (the browser asks
// the user the first time).

let pasted = null; // {bytes, at}

async function blobBytes(b) { return new Uint8Array(await b.arrayBuffer()); }

function initPaste() {
  let waiting = 0;
  window.addEventListener('keydown', (ev) => {
    const t = ev.target;
    if (!(ev.ctrlKey || ev.metaKey) || ev.altKey || (ev.key !== 'v' && ev.key !== 'V')) return;
    if (t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t?.isContentEditable) return;
    if (!document.getElementById('newdlg').hidden || !document.getElementById('rotdlg').hidden || document.querySelector('.mv-modal')) return;
    ev.stopImmediatePropagation(); // (no preventDefault: the browser's paste comes, with the picture)
    clearTimeout(waiting);
    waiting = setTimeout(() => dispatch('menu-paste', []), 300); // no paste event: the page asks the clipboard itself
  }, true);
  document.addEventListener('paste', async (ev) => {
    const t = ev.target;
    if (t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t?.isContentEditable) return;
    ev.preventDefault();
    clearTimeout(waiting);
    const file = [...(ev.clipboardData?.files || [])].find((f) => f.type.startsWith('image/'));
    pasted = file ? {bytes: await blobBytes(file), at: Date.now()} : null;
    dispatch('menu-paste', []);
  });
}

async function clipboardImage() {
  if (pasted && Date.now() - pasted.at < 5000) { const b = pasted.bytes; return b; }
  try {
    for (const it of await navigator.clipboard.read()) {
      const type = it.types.find((t) => t.startsWith('image/'));
      if (type) { const b = await blobBytes(await it.getType(type)); pasted = {bytes: b, at: Date.now()}; return b; }
    }
  } catch { /* not allowed, or nothing */ }
  if (pasted) return pasted.bytes; // the last picture pasted in this tab
  return null;
}

// ---- Printing ------------------------------------------------------------------------------------------------------------------

function printImage(src) {
  return new Promise((resolve) => {
    const f = document.createElement('iframe');
    f.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0;';
    document.body.append(f);
    const d = f.contentDocument;
    d.open();
    d.write('<!doctype html><title>mormiView</title><style>@page{margin:10mm}html,body{margin:0;height:100%}img{display:block;max-width:100%;max-height:100vh;margin:auto;object-fit:contain}</style><img>');
    d.close();
    const img = d.querySelector('img');
    img.onload = () => { f.contentWindow.focus(); f.contentWindow.print(); setTimeout(() => { f.remove(); resolve(); }, 1000); };
    img.onerror = () => { f.remove(); resolve(); };
    img.src = src;
  });
}

// ---- Downloads ------------------------------------------------------------------------------------------------------------------

const TYPES = {png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', bmp: 'image/bmp', tif: 'image/tiff', tiff: 'image/tiff', webp: 'image/webp'};

function saveBlob(name, blob) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = name; a.style.display = 'none';
  document.body.append(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 60000);
  window.mvLastDownload = {name, size: blob.size, at: Date.now()}; // (for checks from the console)
}

// Files written within a moment of each other (the frames of an animation) are one download, a zip: the browser lets a page start only
// one download without asking.
let pending = [], pendingTimer = 0;
function download(path, bytes) {
  pending = pending.filter((p) => p.path !== path);
  pending.push({path, bytes});
  clearTimeout(pendingTimer);
  pendingTimer = setTimeout(() => {
    const files = pending;
    pending = [];
    if (files.length === 1) {
      const {path: p, bytes: b} = files[0], ext = p.slice(p.lastIndexOf('.') + 1).toLowerCase();
      saveBlob(base(p), new Blob([b], {type: TYPES[ext] || 'application/octet-stream'}));
      return;
    }
    const first = base(files[0].path), stem = first.slice(0, first.lastIndexOf('.')).replace(/[-_ ]?\d+$/, '') || 'mormiview';
    saveBlob(`${stem}.zip`, zip(files.map((f) => ({name: base(f.path), bytes: f.bytes}))));
  }, 400);
}

// ---- Full screen ------------------------------------------------------------------------------------------------------------------

function fullscreen(on) {
  if (on && !document.fullscreenElement) document.documentElement.requestFullscreen?.().catch(() => {});
  if (!on && document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
}

// ---- Setting it up ------------------------------------------------------------------------------------------------------------------

let readyResolve;
export const goReady = new Promise((r) => { readyResolve = r; });

window.mvHost = {
  ready() { readyResolve(); },
  emit: emitFromGo,
  setMenu(json) { renderMenu(JSON.parse(json), {openFolder}); },
  openFile: async () => openFiles(false),
  pickFolder: async () => DOWNLOADS,
  saveFile: async (json) => {
    const o = JSON.parse(json);
    const name = await saveDialog(o);
    if (!name) return '';
    const dir = o.DefaultDirectory || PICTURES;
    return `${dir}/${name}`;
  },
  messageDialog: async (json) => messageDialog(JSON.parse(json)),
  fullscreen,
  isFullscreen: () => !!document.fullscreenElement,
  quit() { // a tab that was opened by the page can close itself; any other just goes back to the home screen
    window.close();
  },
  openURL(u) { window.open(u, '_blank', 'noopener'); },
  openAbout() { window.dispatchEvent(new Event('mv-about')); }, // a dialog over the page (src/main.js), not a tab of its own
  async openWindow(path) {
    await flush(); // the new tab reads the files from IndexedDB
    window.open(`${location.pathname}${path ? `?open=${encodeURIComponent(path)}` : '?new'}`, '_blank');
  },
  clipboardImage,
};

window.runtime = {
  EventsOnMultiple: (name, cb, max) => on(name, cb, max),
  EventsOn: (name, cb) => on(name, cb, -1),
  EventsOnce: (name, cb) => on(name, cb, 1),
  EventsOff: (name, ...more) => { for (const n of [name, ...more]) listeners.delete(n); },
  EventsOffAll: () => listeners.clear(),
  EventsEmit: (name, ...data) => dispatch(name, data),
  BrowserOpenURL: (u) => window.open(u, '_blank', 'noopener'),
  WindowSetTitle: (t) => { document.title = t; },
  WindowFullscreen: () => fullscreen(true),
  WindowUnfullscreen: () => fullscreen(false),
  WindowIsFullscreen: async () => !!document.fullscreenElement,
  WindowReload: () => location.reload(), WindowReloadApp: () => location.reload(),
  WindowGetSize: async () => ({w: innerWidth, h: innerHeight}),
  ClipboardGetText: () => navigator.clipboard.readText(),
  ClipboardSetText: (t) => navigator.clipboard.writeText(t).then(() => true, () => false),
  Environment: async () => ({buildType: 'production', platform: 'web', arch: 'wasm'}),
  Quit: () => window.close(), Hide() {}, Show() {},
  LogPrint: console.log, LogTrace: console.debug, LogDebug: console.debug, LogInfo: console.info, LogWarning: console.warn, LogError: console.error, LogFatal: console.error,
};
for (const n of ['WindowSetAlwaysOnTop', 'WindowSetSystemDefaultTheme', 'WindowSetLightTheme', 'WindowSetDarkTheme', 'WindowCenter', 'WindowSetSize', 'WindowSetMaxSize', 'WindowSetMinSize', 'WindowSetPosition', 'WindowHide', 'WindowShow', 'WindowMaximise', 'WindowToggleMaximise', 'WindowUnmaximise', 'WindowMinimise', 'WindowUnminimise', 'WindowSetBackgroundColour']) window.runtime[n] = () => {};

// bind makes window.go.main.App from the methods the Go program has, with the page's own versions of some (overrides, and the chat's).
export function bind(extra = {}) {
  const app = {};
  for (const name of window.mvMethods) app[name] = (...args) => callGo(name, args);
  Object.assign(app, overrides, extra);
  window.go = {main: {App: app}};
  setDownloadHandler(download);
  initPaste();
  initDrop();
  window.addEventListener('beforeunload', (ev) => { // unsaved changes: the browser asks before the tab is closed or reloaded
    flush();
    try { if (window.mvModified()) { ev.preventDefault(); ev.returnValue = ''; } } catch { /* (the program is not running) */ }
  });
}

export {dispatch, MAC};
