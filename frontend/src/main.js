import './style.css';
import {Effect, Transform, SaveToPathQ, Version, Mode, PolygonPNG, DiscardChanges, PaintPixels, NewBlankColor, SaveToPath, Image, ChooseImage, Step, Trace, Reveal, ToggleFullscreen, ExitFullscreen, IsFullscreen, CloseWindow, PreparePixels, PixelAt, SelectionPNG, RotatePNG, SetTurn, SetFloat, ClearFloat, BakeFloat, HasTransparency, RestoreState, RestoreFloat, KeepStates, CopyImageNative, PasteLayer, PasteNew, DrawStroke, DrawShape, Retouch, ResizeCanvas, CropPicture, DrawLayer, FloodFill, DeleteImage, DiscardOK, ConfirmQuit, ControlReply, ClipboardHasImage, NewBlank, SaveSelectionAs, SetEditMode, SetTitleInfo, SetPasted, MovePasted, RestoreLayer, SaveAs, SaveWindow, SetMenuBar, NewWindow, ScanImage, PrintPNG, SaveRegionTo, GifInfo, GifFrame, SetGifFrame, Recent, OpenRecent, ClearRecent, GifStart, GifAddFrame, GifDuplicateFrame, GifDeleteFrame, GifMoveFrame, GifSetDelay, ExportFramesDialog} from '../wailsjs/go/main/App';
import {BrowserOpenURL, EventsOn} from '../wailsjs/runtime/runtime';
import logo from './assets/mormitech-mark.svg';
import {aiDialog, initChat, toggleChat} from './chat.js';
import {initTips} from './tooltip.js';
import {infoDialog} from './dialogs.js';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));
const errText = (e) => {
  const t = String(e?.message ?? e).replace(/^Error:\s*/, '');
  return t.charAt(0).toUpperCase() + t.slice(1);
};

// ---- About window ----------------------------------------------------------------------------------------
// The program started with --about shows this. Links open in the system's browser.

const COFFEE = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.5 9.5h12v5a5 5 0 0 1-5 5h-2a5 5 0 0 1-5-5zM16.5 11h1.5a2.5 2.5 0 0 1 0 5h-1.8M8 3.5c-.9 1 .9 1.8 0 2.8M12 3.5c-.9 1 .9 1.8 0 2.8"/></svg>';
const REPO = 'https://github.com/mormitech/mormiview';
const WEB = document.body.classList.contains('web'); // the web version (frontend/web): the same page, started in a browser

async function aboutHTML() {
  return `<img class="logo" src="${logo}" alt="" width="132">
    <h1><span class="wordmark"><span class="wm-mormi">mormi</span><span class="wm-product">View</span></span></h1>
    <p class="muted">Version ${esc(await Version())}</p>
    <p>${WEB ? 'An image viewer and editor that runs in your browser. Your images stay in this browser; only the AI chat, when you use it, sends them to the provider you chose (the Claude API or OpenRouter).' : 'An image viewer for macOS, Windows and Linux. Open an image from your file manager (Open with…) or with File → Open.'}</p>
    <p class="muted">Free and open source, MIT license. No accounts, no ads, no telemetry.</p>
    <p class="links"><a href="${REPO}">Website and source code</a><a href="${REPO}/issues">Report a problem</a><a href="${REPO}/blob/main/LICENSE">License</a><a class="coffee" href="https://mormitech.com/support-us/">${COFFEE}Buy me a coffee</a></p>`;
}

async function showAbout() {
  document.body.classList.add('about-mode');
  window.addEventListener('keydown', (ev) => { // the About window has no menu: Cmd+W (Ctrl+W) and Esc close it
    if (ev.key === 'Escape' || ((ev.metaKey || ev.ctrlKey) && !ev.altKey && (ev.key === 'w' || ev.key === 'W'))) { ev.preventDefault(); CloseWindow(); }
  });
  $('about').hidden = false;
  $('about').innerHTML = await aboutHTML();
}

// In the web version About is a dialog over the page (web/host.js sends mv-about), not a window of its own.
window.addEventListener('mv-about', async () => {
  if (document.querySelector('.mv-about')) return;
  infoDialog(`<div id="about-body">${await aboutHTML()}</div>`, 'mv-about');
});

window.addEventListener('error', (e) => Trace(`error: ${e.message} at ${e.lineno}:${e.colno}`)); // (with MORMIVIEW_TRACE set, a failure of the page shows in the trace)
window.addEventListener('unhandledrejection', (e) => Trace(`rejection: ${e.reason}`));
document.addEventListener('click', (ev) => {
  const a = ev.target.closest('a[href^="http"]');
  if (a) { ev.preventDefault(); BrowserOpenURL(a.href); }
});

// ---- Viewer ----------------------------------------------------------------------------------------------
// How the picture sits in the window is one of six modes, the way IrfanView has them on the keys 1 to 6, and the mode stays when
// the next or the previous picture is shown (until the window is closed; every new window starts with 2):
//   1  original  - the pixels of the picture, 100%, however small or big it is (a big one is centred and can be moved)
//   2  large     - every picture is zoomed, up or down, to fill 85% of the window, keeping its proportions (the default)
//   3  contain   - every picture is zoomed, up or down, to touch the borders of the window, keeping its proportions
//   4  stretch   - every picture is stretched border to border, proportions or not
// The scroll wheel (or a pinch on the trackpad) zooms around the pointer (on a Mac two fingers on the trackpad move the picture instead),
// dragging moves the picture, + and - zoom: that leaves
// the mode until the next picture or the next of the keys 1 to 6. A double click does nothing to the zoom.
// The arrow keys go to the previous and the next picture of the folder (so do Backspace and Space, as in IrfanView), Enter toggles full screen (Esc no longer leaves it), E or Tab the edit mode (with the pixelated view), Shift and drag selects, Ctrl+C copies, Ctrl+V pastes, S saves, R turns the picture (Shift+R the other way), Ctrl+Z undoes and Ctrl+Shift+Z redoes, Ctrl+N opens a new window.

const MODES = {'1': 'original', '2': 'large', '3': 'contain', '4': 'stretch', '5': 'width', '6': 'height'};
const pic = $('pic');
const view = {tool: 'select', name: '', dir: '', index: 0, count: 0, mode: 'large', pixelated: false, state: 0, modified: false, canAlpha: false, fl: null, tdrag: null, hlMode: '', token: 0, sel: null, selDrag: null, rot: 0, angle: 0, free: false, ox: 0, oy: 0, sx: 1, sy: 1, x: 0, y: 0, drag: null};
const CHECKER = 'repeating-conic-gradient(#cdd7ec 0 25%, #fff 0 50%)'; // the squares of a picture with transparent parts, a bluish one and a white one
let problemTimer = 0;
let pointerDown = false;
let rightHold = null; // the right button is down with a lasso tool: only a drag draws a new polygon, a click leaves the selection alone and opens the menu

// R turns the picture clockwise by a quarter, Shift+R counterclockwise: only the view (the file is not touched). view.rot is the number
// of quarter turns, 0 to 3; view.angle counts them without wrapping, so that the animation always turns the short way. Everything below
// that is about the picture on the screen (the mode, the pan limits, the selection, the readout) works in the turned picture, whose
// size is dispW × dispH; the file's own pixels (natural size) are only needed to read a pixel or cut a part out.
const dispW = () => view.rot % 2 ? pic.naturalHeight : pic.naturalWidth;
const dispH = () => view.rot % 2 ? pic.naturalWidth : pic.naturalHeight;

// A position of the turned picture (fractions included) as a position of the file's picture.
function toNatural(X, Y) {
  const nw = pic.naturalWidth, nh = pic.naturalHeight;
  switch (view.rot) {
    case 1: return [Y, nh - X];
    case 2: return [nw - X, nh - Y];
    case 3: return [nw - Y, X];
    default: return [X, Y];
  }
}

// The same for a rectangle {x, y, w, h}.
function toNaturalRect(r) {
  const [x0, y0] = toNatural(r.x, r.y), [x1, y1] = toNatural(r.x + r.w, r.y + r.h);
  return {x: Math.min(x0, x1), y: Math.min(y0, y1), w: Math.abs(x1 - x0), h: Math.abs(y1 - y0)};
}

let turnTimer = 0;

function rotate(dir) {
  if (!pic.naturalWidth) return;
  stopBounce();
  // A picture that was zoomed or moved keeps its scale, and the pixel in the middle of the window stays in the middle. (One that
  // the mode places is placed by the mode again, fitted to the turned picture.)
  const b = stageSize(), W = dispW(), H = dispH();
  const X = (b.w / 2 - view.x) / view.sx, Y = (b.h / 2 - view.y) / view.sy; // the middle of the window, in the picture as shown
  const mid = dir > 0 ? [H - Y, X] : [Y, W - X]; // and where that point is in the picture after the turn
  view.rot = (view.rot + dir + 4) % 4; view.angle += dir * 90; SetTurn(view.rot);
  view.sel = null; view.selDrag = null; view.hl = null; clearPixel();
  if (view.free) {
    [view.sx, view.sy] = [view.sy, view.sx]; // what was the vertical scale is the horizontal one now
    view.x = b.w / 2 - mid[0] * view.sx; view.y = b.h / 2 - mid[1] * view.sy;
  }
  pic.classList.add('turning'); void pic.offsetWidth; // the style changes of draw() are animated, for a moment
  draw();
  clearTimeout(turnTimer);
  turnTimer = setTimeout(() => pic.classList.remove('turning'), 240);
}

const stageSize = () => ({w: $('stage').clientWidth, h: $('stage').clientHeight});
const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// The part of the window the modes 4, 5 and 6 (stretch, width, height) fill: in edit mode without the tool rail and the status bar that cover its edges.
const FILL_MODES = ['stretch', 'width', 'height'];
function visibleBox() {
  const b = stageSize(), ins = insets();
  return FILL_MODES.includes(view.mode) ? {x: ins.l, y: 0, w: b.w - ins.l, h: b.h - ins.b} : {x: 0, y: 0, w: b.w, h: b.h};
}

// The scale of the picture, for each axis, that the mode asks for.
function modeScale() {
  const b = stageSize(), w = dispW(), h = dispH(), fit = Math.min(b.w / w, b.h / h), v = visibleBox();
  switch (view.mode) {
    case 'original': return [1, 1];
    case 'large': return [fit * 0.85, fit * 0.85]; // 85% of the window, up or down
    case 'contain': return [fit, fit];
    case 'width': return [v.w / w, v.w / w]; // the side edges touch the window (the rail); the top and bottom go over, or stay short
    case 'height': return [v.h / h, v.h / h]; // the top and bottom edges touch the window (the bar)
    default: return [v.w / w, v.h / h];
  }
}

// Keeps a picture that was moved or zoomed on the screen: smaller than the window it is centred, bigger it cannot leave a gap.
function constrain() {
  const bd = bounds();
  view.x = clamp(view.x, bd.x[0], bd.x[1]);
  view.y = clamp(view.y, bd.y[0], bd.y[1]);
}

// What covers the picture at its edges in edit mode: the tool rail on the left and the status bar at the bottom. A picture that is bigger than the
// window may be moved that far past its edge, so that the whole of its left and bottom edge can be brought into the open.
function insets() {
  if (!document.body.classList.contains('edit')) return {l: 0, b: 0};
  const s = $('status');
  return {l: $('tools').offsetWidth, b: (s.hidden ? 0 : s.offsetHeight + 20) + (parseFloat(document.body.style.getPropertyValue('--dockh')) || 0)}; // (the capsule floats 20px above the dock)
}

// ---- Bounce at the edge ----------------------------------------------------------------------------------
// The picture can be pulled a little past its edge (at most BOUNCE_MAX pixels, harder the further it goes, the way a list on iOS gives
// at its end) with the mouse button down or two fingers on the trackpad. It is shown at its position plus a "pull" (view.ox, view.oy),
// and a spring takes the pull away again, every frame, as soon as nothing pushes. A picture that fits the window on an axis cannot be
// moved there, but it gives all the same, so that it is plain that there is nowhere to go.
//
// The trackpad keeps sending scroll events for about a second after the fingers have left, each smaller than the one before (the
// system lets the movement coast). Those must not keep the picture out: a pull that is already there is not made bigger by events that
// only get smaller, so it comes back at once.

const BOUNCE_MAX = 20;  // the most the picture can be pulled past its edge, in pixels
const BOUNCE_TAU = 70;  // ms: the spring takes away 63% of the pull in this time
let springRaf = 0, springLast = 0;
const bounce = {peak: 0, lastPush: 0, over: false, lastWheel: 0, swipeOver: false, prev: {x: 0, y: 0}, dec: {x: 0, y: 0}};

// Where the top left corner of the picture may be, per axis: [lowest, highest]. A picture that fits has one place only.
function bounds() {
  const b = stageSize(), w = dispW() * view.sx, h = dispH() * view.sy, ins = insets();
  return {x: w <= b.w ? [(b.w - w) / 2, (b.w - w) / 2] : [b.w - w, ins.l], y: h <= b.h ? [(b.h - h) / 2, (b.h - h) / 2] : [b.h - h - ins.b, 0]};
}

// What a pull of d pixels past the edge shows: less than d, and never more than BOUNCE_MAX.
const pullOffset = (d) => BOUNCE_MAX * (1 - Math.exp(-d / (BOUNCE_MAX * 2.5)));

function stopBounce() {
  cancelAnimationFrame(springRaf); springRaf = 0;
  view.ox = view.oy = 0; bounce.over = false; bounce.peak = 0;
}

function startSpring() {
  if (!springRaf) { springLast = performance.now(); springRaf = requestAnimationFrame(springStep); }
}

function springStep(now) {
  springRaf = 0;
  if (!view.drag) { // while the mouse button is down the pointer decides; the spring only works after it is let go
    const k = Math.exp(-clamp(now - springLast, 0, 50) / BOUNCE_TAU); // (the frame's time can be older than an event's, so never negative)
    view.ox *= k; view.oy *= k;
  }
  springLast = now;
  const pull = Math.max(Math.abs(view.ox), Math.abs(view.oy));
  bounce.peak = Math.max(bounce.peak, pull);
  if (!view.drag && pull < 0.4) { // back
    if (bounce.over) Trace(`bounce: peak ${bounce.peak.toFixed(1)} px, back ${Math.round(now - bounce.lastPush)} ms after the last push`);
    view.ox = view.oy = 0; bounce.over = false; bounce.peak = 0;
    draw();
    return;
  }
  draw();
  springRaf = requestAnimationFrame(springStep);
}

function draw() {
  if (!pic.naturalWidth) return;
  if (view.free) {
    constrain();
  } else { // the mode decides, and the picture is centred (a big one at 100% shows its middle)
    [view.sx, view.sy] = modeScale();
    const v = visibleBox();
    view.x = v.x + (v.w - dispW() * view.sx) / 2; view.y = v.y + (v.h - dispH() * view.sy) / 2;
  }
  // The picture gets its size on screen, not a scale: scaling a 36 megapixel picture with a CSS transform made WebKit draw
  // all of it, 5 seconds before anything showed. At its displayed size it is decoded and drawn at that size.
  // A turned picture is the box bw × bh on the screen; the image itself is as wide as that box is high, and is turned around its middle.
  const bw = Math.max(1, Math.round(dispW() * view.sx)), bh = Math.max(1, Math.round(dispH() * view.sy));
  const ew = view.rot % 2 ? bh : bw, eh = view.rot % 2 ? bw : bh;
  pic.style.width = `${ew}px`;
  pic.style.height = `${eh}px`;
  pic.style.transform = `translate(${Math.round(view.x + view.ox) + (bw - ew) / 2}px, ${Math.round(view.y + view.oy) + (bh - eh) / 2}px) rotate(${view.angle}deg)`; // the pull included
  // the name of the picture is in the title of the window; down here is where it lives and which one of the folder it is
  const place = view.count ? `${view.index} / ${view.count}  ·  ` : '';
  const pct = (v) => `${Math.round(v * 100)}%`;
  const zoom = Math.abs(view.sx - view.sy) > 0.005 ? `${pct(view.sx)} × ${pct(view.sy)}` : pct(view.sx);
  const sel = selectionRect();
  // the window title has the size, place and zoom; the bar repeats them only in full screen, where there is no title
  const full = document.body.classList.contains('full');
  const info = `${place}${dispW()} × ${dispH()}  ·  ${zoom}${view.pixelated ? '  ·  pixels' : ''}`;
  setStatus(view.dir, full ? info : '', view.name + (view.modified || floatChanges(view.fl) ? ' ●' : ''), sel);
  document.body.classList.toggle('sel', !!(sel || view.fl)); // in full screen the bar stays for as long as there is a selection
  tellInfo(pic.naturalWidth ? `${dispW()} × ${dispH()}${view.count ? `  ·  ${view.index} / ${view.count}` : ''}  ·  ${zoom}${view.pixelated ? '  ·  pixels' : ''}` : '');
  renderFloat();
  renderInk();
  renderText();
  renderSelection();
  renderPoly();
  $('discard-img').disabled = !(view.modified || floatChanges(view.fl));
  if (view.pixelated && lastPointer) readPixel(lastPointer.x, lastPointer.y); // the picture moved under the pointer
}

// What the status bar used to say about the picture (size, place in the folder, zoom) goes to Go, for the title of the window.
let toldInfo = '';
function tellInfo(text) {
  if (text === toldInfo) return;
  toldInfo = text;
  SetTitleInfo(text).catch(() => {});
}

// The capsule at the bottom: the folder (its last part; all of it in the tooltip), what is said about the picture, and the selection: its first and
// last pixel (both in it) and its size.
function setStatus(folder, info = '', name = '', sel = null) {
  $('status').hidden = !folder && !info && !sel;
  $('name').textContent = name; $('name').title = name;
  $('folder').textContent = folder.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || folder; $('folder').title = folder;
  $('info').textContent = info;
  const corner = (k) => { const i = document.createElement('i'); i.className = `ico ${k}`; i.style.setProperty('--i', `var(--i-${k})`); return i; };
  const size = document.createElement('b'); if (sel) size.textContent = `${sel.w} × ${sel.h}`;
  $('selinfo').replaceChildren(...(sel ? [corner('tl'), `${sel.x}, ${sel.y}`, corner('br'), `${sel.x + sel.w - 1}, ${sel.y + sel.h - 1}`, size] : []));
  $('selinfo').title = sel ? 'Selection: the top left and the bottom right pixel, and the size' : '';
  $('status').classList.remove('bad', 'msg');
}

// The keys 1 to 6: choose a mode, and keep it.
function setMode(mode) {
  stopBounce();
  view.mode = mode; view.free = false;
  draw();
}

function zoomAt(factor, px, py) {
  stopBounce();
  const sx = clamp(view.sx * factor, 0.02, 32), f = sx / view.sx; // the same factor on both axes: a stretched picture stays stretched
  view.x = px - (px - view.x) * f; view.y = py - (py - view.y) * f;
  view.sx = sx; view.sy = clamp(view.sy * f, 0.02, 32);
  view.free = true;
  draw();
}

// opts.rot: the turn the picture is shown with (undo brings a picture back as it was turned); a new picture is not turned.
// The empty window offers the picture that is on the clipboard (if there is one) next to Open file…
function offerRecent() { // the pictures opened last, on the home screen
  Recent().then((list) => {
    const box = $('recent-list');
    box.replaceChildren(...(list || []).slice(0, 5).map((f) => {
      const b = document.createElement('button'), n = document.createElement('b'), d = document.createElement('span');
      n.textContent = f.name; d.textContent = `\u200E${f.dir.replace(/^\/(Users|home)\/[^/]+/, '~')}\u200E`; // (the marks keep "~/" in front in the right-to-left box, which cuts a long path at its start)
      b.append(n, d); b.title = f.path;
      b.onclick = () => OpenRecent(f.path).catch((e) => problem(errText(e)));
      return b;
    }));
    $('recent').hidden = !list?.length;
  }).catch(() => {});
}

$('recent-clear').onclick = () => ClearRecent().then(offerRecent).catch(() => {});

function offerClipboard() {
  offerRecent();
  ClipboardHasImage().then((has) => { if (!$('empty').hidden) $('open-clip').hidden = !has; }).catch(() => {});
}

// The empty window offers the picture that was closed last, when closing it is the last step of the history (undo brings it back, changes and history with it).
function offerReopen() {
  const e = history.list[history.at], p = history.list[history.at - 1];
  $('reopen-pic').hidden = $('empty').hidden || !(e && e.label === 'image closed' && p && p.state !== 0);
}

// ---- The GIF frame editor: the player (play and pause, one frame back and forth, the frame counter) and the tools on the frame that is shown --------------------------------
// The player shows the frames itself (the web view's own animation cannot be stopped or stepped). Paused (or stepped) on a frame, that frame is what the pixel tools
// read and what an edit goes into: the animation stays, with that frame changed (undo and redo bring it back). Save As offers the whole animation or the frame.
const gif = {meta: null, url: '', i: 0, playing: false, timer: 0, urls: new Map(), shown: false};
function gifSyncFade() { $('gifbar').classList.toggle('fade', gif.playing && !editOn()); } // (running outside edit mode: the controls fade away; pausing or stepping brings them back)
function gifStop() { clearTimeout(gif.timer); gif.timer = 0; gif.playing = false; document.body.classList.remove('gif-playing'); }
function gifHide(restore = false) { // the player goes; with restore the picture is the file's own animation again (edit mode left), not when another picture has come
  gifStop();
  $('gifbar').hidden = true;
  if (restore && gif.shown && gif.url && pic.src !== new URL(gif.url, location.href).href) { pic.src = gif.url; }
  gif.shown = false;
}
async function gifFrameURL(i) { // (a frame that was changed has a new key, so what was made for the old one is not shown again)
  const k = gif.meta.keys[i];
  let u = gif.urls.get(k);
  if (!u) { u = await GifFrame(i); if (gif.urls.size > 300) gif.urls.clear(); gif.urls.set(k, u); }
  return u;
}
async function gifShowFrame(i) {
  const url = await gifFrameURL(i);
  if (!gif.meta || !gif.shown) return;
  gif.i = i; pic.src = url; $('gif-count').textContent = `${i + 1} / ${gif.meta.frames}`;
  if (document.activeElement !== $('gif-ms')) $('gif-ms').value = gif.meta.delays[i] || 100; // (the delay of the frame that is shown)
}
function gifTick() {
  if (!gif.playing) return;
  const i = gif.i;
  gif.timer = setTimeout(async () => {
    if (!gif.playing) return;
    await gifShowFrame((gif.i + 1) % gif.meta.frames);
    gifTick();
  }, gif.meta.delays[i] || 100);
}
function gifSetPlaying(on) { // (the promise is for the pause: what is shown is then what the tools read)
  gifStop();
  gif.playing = on; document.body.classList.toggle('gif-playing', on); // (the tools are out of the way until a frame is chosen)
  $('gifbar').classList.toggle('paused', !on); gifSyncFade();
  $('gif-play').title = on ? 'Pause' : 'Play'; $('gif-play').setAttribute('aria-label', on ? 'Pause' : 'Play');
  if (on) { clearSelection(); polyClear(); gifTick(); return Promise.resolve(); } // (a selection belongs to one frame: playing lets it go)
  return SetGifFrame(gif.i).catch(() => {});
}
async function gifStep(d) {
  if (!gif.meta) return;
  await gifSetPlaying(false);
  await gifShowFrame((gif.i + d + gif.meta.frames) % gif.meta.frames);
  await SetGifFrame(gif.i).catch(() => {});
}
// Shift+wheel (up: a frame back, down: a frame forward; the animation stops) and the up and down arrow keys step through the frames, round and round.
let gifWheelAcc = 0;
function gifWheel(ev) {
  const d = Math.abs(ev.deltaY) >= Math.abs(ev.deltaX) ? ev.deltaY : ev.deltaX; // (Shift turns the wheel into a sideways one on a Mac)
  if (!d) return;
  if (Math.abs(d) >= 100) { gifWheelAcc = 0; gifStepAny(Math.sign(d)); return; } // a notch of a mouse wheel: one frame
  gifWheelAcc += d; // a trackpad: a frame for every 40 pixels
  while (Math.abs(gifWheelAcc) >= 40) { const s = Math.sign(gifWheelAcc); gifWheelAcc -= 40 * s; gifStepAny(s); }
}
let gifStepping = Promise.resolve();
function gifStepAny(d) { // (also outside edit mode: the player comes up, paused)
  if (!gif.meta || !pic.naturalWidth) return;
  gifStepping = gifStepping.then(async () => { if (!gif.shown) await gifPresent(false, true); await gifStep(d); }); // (one after the other: the wheel comes in bursts)
}

// A change of the frames (the player's buttons, the Image menu and Claude's tools): the answer of Go is the picture, and the player follows it.
async function gifDo(m) {
  gifStop();
  switch (m.op) {
    case 'start': return GifStart(view.rot);
    case 'add': return GifAddFrame(view.rot, !!m.blank, m.after ?? -1, !!m.follow);
    case 'duplicate': return GifDuplicateFrame(view.rot, m.source, m.target, !!m.follow);
    case 'delete': return GifDeleteFrame(m.frame ?? -1, !!m.follow);
    case 'move': return GifMoveFrame(m.from, m.to, !!m.follow);
    case 'delay': return GifSetDelay(m.frame ?? -1, m.ms);
  }
  throw new Error(`no animation operation "${m.op}"`);
}
async function gifEdit(m, label) { // the same, for the user: one step of the history
  await commitText();
  try {
    if (view.fl) await flushFloat({clear: true});
    const info = await gifDo(m);
    await showImage(info, {keepView: true, rot: 0}); commit(label);
  } catch (e) { problem(errText(e)); }
}

function gifSpace() { // Space: play or pause an animated GIF (outside edit mode the first press brings the player, paused)
  if (!gif.meta || !pic.naturalWidth) return;
  if (!gif.shown) gifPresent(false, true); else gifSetPlaying(!gif.playing);
}
async function gifPresent(play, force = false) { // the player is on: playing from the first frame, or paused where the tools are
  gif.shown = true;
  pic.onload = () => draw(); // (a frame is on the screen: only the drawing)
  $('gifbar').hidden = false; $('gifbar').classList.toggle('edited', !!gif.meta.edited || force);
  if (play) { await gifShowFrame(0); gifSetPlaying(true); } else { await gifShowFrame(gif.meta.cur); await gifSetPlaying(false); }
}
function gifOffer() { // edit mode begins: an animated GIF plays in the player (one with changed frames waits on its frame)
  if (gif.meta && !gif.shown && pic.naturalWidth) gifPresent(!gif.meta.edited);
}
function gifReset() { // another picture: nothing of the animation that was shown stays (frames, count, timer, player)
  gifStop(); gif.urls.clear(); gif.meta = null; gif.url = ''; gif.i = 0;
  gifHide();
  $('gif-count').textContent = '';
}
async function gifSetup(info) { // a picture was shown
  const wasPlaying = gif.playing && gif.shown;
  gifStop();
  if (!info?.url) { gifHide(); gif.meta = null; return; }
  try {
    const meta = await GifInfo();
    if (view.state !== info.state) return; // (another picture was asked for meanwhile)
    if (!meta.frames) { gifHide(); gif.meta = null; return; }
    gif.meta = meta; gif.url = info.url;
    if (editOn() || meta.edited) {
      await gifPresent(meta.fresh || (!gif.shown && !meta.edited));
      if (wasPlaying && !meta.fresh) gifSetPlaying(true); // (the chat or the buttons changed a frame while it ran: it runs on)
    } else gifHide();
  } catch { gifHide(); gif.meta = null; }
}

// In full screen the info bar is shown for two seconds when another picture arrives (opened, pasted, or stepped to), then it hides again.
let arriveTimer = 0;
function arriveBar() {
  if (!document.body.classList.contains('full')) return;
  document.body.classList.add('arrive');
  clearTimeout(arriveTimer); arriveTimer = setTimeout(() => document.body.classList.remove('arrive'), 2000);
}

function showImage(info, opts = {}) {
  if (gif.meta && (info?.name !== view.name || info?.dir !== view.dir)) gifReset(); // (another picture: the player of the one before goes at once, not when the new one is known)
  let done; const loaded = new Promise((r) => { done = r; }); // resolved when the picture is on the screen
  view.state = info?.state || 0;
  view.fl = opts.fl || null; view.tdrag = null; if (!opts.keepView && !opts.fl) renderFloat(); // (the same picture with a change applied: what is lifted stays on the screen until the new one is there, so nothing flashes)
  clearTimeout(turnTimer); pic.classList.remove('turning'); view.rot = opts.rot || 0; view.angle = view.rot * 90; SetTurn(view.rot);
  if (!info?.url) { // no picture: the window as it is when it starts
    stopBounce(); pic.onload = null; pic.removeAttribute('src'); cancelText();
    view.name = ''; view.modified = false; view.dir = ''; view.index = 0; view.count = 0; view.token++; view.sel = null; view.selDrag = null; view.hl = null; $('stage').style.cursor = ''; clearPixel();
    polyClear(); pic.hidden = true; $('empty').hidden = false; $('stage').classList.remove('checker'); setStatus(''); renderSelection(); offerClipboard(); // (the squares belong to a picture with transparency)
    gifHide(); gif.meta = null;
    if (editOn()) setEdit(false); // no image: no edit mode (closing it, deleting the last one, undo back to the empty window)
    done();
    return loaded;
  }
  if (!opts.keepView && (info.name !== view.name || info.dir !== view.dir)) arriveBar(); // (another picture)
  view.name = info.name; view.modified = !!info.modified; view.canAlpha = !!info.canAlpha; view.dir = info.dir; view.index = info.index; view.count = info.count; if (!opts.keepView) view.free = false; // (a picture that is the same, with something applied, keeps its zoom)
  stopBounce();
  if (!opts.keepView) polyClear(); view.token++; pixelState = ''; pixelCanvas = null; pixelPromise = null; view.sel = opts.sel ? rectSel(opts.sel) : null; view.selDrag = null; view.hl = null; $('stage').style.cursor = ''; clearPixel();
  pic.onload = () => {
    Trace(`img onload, visibility ${document.visibilityState}, focus ${document.hasFocus()}`);
    pic.hidden = false; $('empty').hidden = true; draw();
    checkTransparency(view.token);
    if (view.pixelated) preparePixels();
    pic.decode().then(() => Trace('decoded'), () => {});
    requestAnimationFrame(() => Trace('first paint after load'));
    done();
  };
  pic.onerror = () => { problem(`Cannot show ${info.name}`); done(); };
  Trace('img src set');
  // The new picture is decoded off screen first, and then it takes the place of the old one in a single step: the old one stays on the screen until
  // then, so nothing is cleared in between (that flashed when the next or the previous picture of a folder was shown).
  const pre = document.createElement('img'), token = view.token; // (not new Image(): that name is the binding of Go's Image)
  pre.src = info.url;
  pre.decode().catch(() => {}).then(() => {
    if (token !== view.token) { done(); return; } // another picture was asked for meanwhile
    if (!opts.keepView) $('stage').classList.remove('checker');
    pic.src = info.url;
  });
  loaded.then(() => { if (token === view.token) gifSetup(info); });
  return loaded;
}

function problem(text, bad = true, ms = 4000) {
  $('status').hidden = false;
  $('name').textContent = ''; $('folder').textContent = text; $('folder').title = ''; $('info').textContent = ''; $('selinfo').replaceChildren();
  $('status').classList.toggle('bad', bad); $('status').classList.add('msg');
  clearTimeout(problemTimer);
  document.body.classList.add('peek');
  problemTimer = setTimeout(() => { document.body.classList.remove('peek'); draw(); }, ms);
}

// The Effects menu: the effect works on the part that is selected (the polygon, else the box), or on all of the picture when nothing is.
// A selected part that is turned again (and again) is turned from the pixels it had before the first turn, so that it does not get blurrier: Go keeps those while this
// session lasts, which is until the selection is let go or the picture is changed by something else (the state is not the one the turn made).
let rotSession = null;
// The effects used last (at most three, the latest first), offered at the top of the effects in the right-click menu; kept for next time.
let recentEffects = [];
try { recentEffects = JSON.parse(localStorage.getItem('recentEffects') || '[]').filter((n) => typeof n === 'string').slice(0, 3); } catch {}
function noteRecentEffect(name) {
  if (!effectLabel(name)) return;
  recentEffects = [name, ...recentEffects.filter((n) => n !== name)].slice(0, 3);
  try { localStorage.setItem('recentEffects', JSON.stringify(recentEffects)); } catch {}
}
// effectLabel is the label of an effect for the Recent list (an item of a submenu says which one: "Highlight: light").
function effectLabel(name) {
  for (const e of EFFECT_LABELS) {
    if (e && e[1] === name) return e[0];
    const s = e && e[2] && e[2].find((x) => x[1] === name);
    if (s) return `${e[0]}: ${s[0].toLowerCase()}`;
  }
  return '';
}
const EFFECT_LABELS = [['Blur', 'blur'], ['Pixelize', 'pixelize'], ['Emboss', 'emboss'], ['Explosion', 'explosion'], ['Censor', 'censor'], ['Highlight', null, [['Light', 'highlight-light'], ['Medium', 'highlight-medium'], ['Full', 'highlight-full']]], ['Sharpen', 'sharpen'], null,
  ['Grayscale', 'grayscale'], ['Negative', 'negative'], ['Sepia', 'sepia'], null, ['Brighter', 'brightness+'], ['Darker', 'brightness-'], ['More contrast', 'contrast+'], ['Less contrast', 'contrast-'],
  ['More color', 'saturation+'], ['Less color', 'saturation-'], ['Warmer', 'warmer'], ['Cooler', 'cooler'], ['Auto levels', 'auto-levels'], null, ['Fill with foreground color', 'fill-fg']];
const EFFECT_MENU = {
  blur: {op: 'blur', radius: 6}, pixelize: {op: 'pixelize', size: 12}, emboss: {op: 'emboss'}, explosion: {op: 'explosion'}, censor: {op: 'censor'}, 'highlight-light': {op: 'highlight', level: 'light'}, 'highlight-medium': {op: 'highlight', level: 'medium'}, 'highlight-full': {op: 'highlight', level: 'full'},
  grayscale: {op: 'grayscale'}, negative: {op: 'negative'}, sepia: {op: 'sepia'}, sharpen: {op: 'sharpen'}, 'auto-levels': {op: 'auto_levels'},
  'brightness+': {op: 'brightness', amount: 15}, 'brightness-': {op: 'brightness', amount: -15}, 'contrast+': {op: 'contrast', amount: 15}, 'contrast-': {op: 'contrast', amount: -15},
  'saturation+': {op: 'saturation', amount: 25}, 'saturation-': {op: 'saturation', amount: -25}, warmer: {op: 'temperature', amount: 20}, cooler: {op: 'temperature', amount: -20},
  'fill-fg': {op: 'fill'}, clear: {op: 'fill'},
};
function effectPart() {
  if (poly.closed && poly.pts.length >= 3) return {pts: poly.pts.map((p) => [p[0], p[1]])};
  const r = selectionRect();
  return r ? {x: r.x, y: r.y, w: r.w, h: r.h} : {};
}
async function applyEffect(spec, label) {
  await commitText(); if (view.fl) await flushFloat({clear: true});
  const keep = selectionRect(), prev = history.list[history.at]; // the selection stays, also when the effect is undone and done again, so that one can try one effect after the other
  const info = await Effect(view.rot, JSON.stringify(spec));
  await showImage(info, {keepView: true, rot: 0, sel: keep}); commit(label || `effect: ${spec.op}`);
  const made = history.list[history.at];
  if (keep && made !== prev) { made.sel = {...keep}; if (prev) prev.undoSel = {...keep}; }
  return info;
}
// Turns the selected part by angle degrees (a quarter turn is stretched to fill the frame again, another angle is blown up so that no corner is empty).
async function rotatePart(angle) {
  const part = effectPart(), key = JSON.stringify(part);
  const again = !!rotSession && rotSession.key === key && rotSession.state === view.state;
  try {
    await applyEffect({op: 'rotate', angle, again, ...part}, `rotated the part ${angle}°`);
    rotSession = {key, state: view.state};
  } catch (e) { problem(errText(e)); }
}

async function applyEffectMenu(name) {
  const base = EFFECT_MENU[name];
  if (!base || !view.state || !pic.naturalWidth) return;
  const spec = {...base, ...effectPart()};
  if (name === 'fill-fg') spec.color = paint.fg; else if (name === 'clear') spec.color = paint.bg;
  try { await applyEffect(spec); noteRecentEffect(name); } catch (e) { problem(errText(e)); }
}

// Rotate by angle (Effects menu, and the right-click menu of the whole picture): any angle, smooth; the corners either make the picture bigger or are cut off.
function askRotation(part) {
  const dlg = $('rotdlg'), form = $('rotdlg-form'), A = $('rd-a'), R = $('rd-r'), C = $('rd-c'), B = $('rd-bg'), err = $('rd-err');
  for (const row of [C, B]) row.closest('.row').hidden = !!part; // (a turned part always fills its frame: no corners to choose)
  $('rotdlg-title').textContent = part ? 'Rotate the selected part' : 'Rotate by angle';
  A.value = '0'; R.value = '0'; err.hidden = true; dlg.hidden = false; A.focus(); A.select();
  return new Promise((resolve) => {
    const close = (v) => { dlg.hidden = true; form.onsubmit = null; dlg.onkeydown = null; $('rd-cancel').onclick = null; dlg.onmousedown = null; A.oninput = null; R.oninput = null; resolve(v); };
    A.oninput = () => { R.value = A.value; };
    R.oninput = () => { A.value = R.value; };
    form.onsubmit = (ev) => {
      ev.preventDefault();
      const angle = +A.value;
      if (!Number.isFinite(angle) || Math.abs(angle) > 360 || angle % 360 === 0) { err.textContent = 'Give an angle between -360 and 360, other than 0.'; err.hidden = false; return; }
      close({angle, crop: C.value === 'crop', bg: B.value});
    };
    $('rd-cancel').onclick = () => close(null);
    dlg.onmousedown = (ev) => { if (ev.target === dlg) close(null); };
    dlg.onkeydown = (ev) => { if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(null); } };
  });
}

async function rotateByAngle() {
  if (!pic.naturalWidth) return;
  await commitText();
  const part = !view.fl && !!selectionRect();
  const o = await askRotation(part);
  if (!o) return;
  if (part) { await rotatePart(o.angle); return; }
  const bg = {auto: '', transparent: '#00000000', white: '#FFFFFFFF', second: paint.bg.length === 7 ? paint.bg + 'FF' : paint.bg}[o.bg];
  try {
    if (view.fl) await flushFloat({clear: true});
    const info = await Transform(view.rot, JSON.stringify({op: 'rotate', angle: o.angle, crop: o.crop, bg}));
    clearSelection();
    await showImage(info, {rot: 0});
    commit(`rotated ${o.angle}°`);
  } catch (e) { problem(errText(e)); }
}

// The Image menu: turn (a quarter, as the R key does) or flip the whole picture, or turn it by any angle.
async function imageMenuAction(name) {
  if (!pic.naturalWidth) return;
  if (name === 'anim-start') { gifEdit({op: 'start'}, 'animation started'); return; }
  if (name === 'anim-add') { gifEdit({op: 'duplicate', source: gif.meta ? gif.i : 0, target: (gif.meta ? gif.i : 0) + 1, follow: true}, 'frame added'); return; }
  if (name === 'anim-delete') { if (gif.meta) gifEdit({op: 'delete', frame: gif.i, follow: true}, 'frame deleted'); return; }
  if (name === 'anim-export') { ExportFramesDialog().then((note) => { if (note) problem(note, false, 3500); }).catch((e) => problem(errText(e))); return; }
  if (name === 'rotate-angle') { rotateByAngle(); return; }
  await commitText();
  try {
    if ((name === 'rotate-right' || name === 'rotate-left') && !view.fl && selectionRect()) { await rotatePart(name === 'rotate-right' ? 90 : -90); return; } // (a selected part is turned)
    if (view.fl) await flushFloat({clear: true}); // what is lifted is applied first
    if (name === 'rotate-right' || name === 'rotate-left') { rotate(name === 'rotate-right' ? 1 : -1); commit(); return; }
    const info = await Transform(view.rot, JSON.stringify({op: 'flip', horizontal: name === 'flip-h', vertical: name === 'flip-v'}));
    clearSelection();
    await showImage(info, {rot: 0});
    commit(name === 'flip-h' ? 'flipped left to right' : 'flipped top to bottom');
  } catch (e) { problem(errText(e)); }
}

// Left and right arrow: the previous and the next picture of the folder, in alphabetical order.
async function step(delta) {
  try {
    const note = await Step(delta);
    if (note) problem(note, false);
  } catch (e) { problem(errText(e)); }
}

async function choose() {
  try { await ChooseImage(); } catch (e) { problem(errText(e)); }
}

// ---- Pixel readout ---------------------------------------------------------------------------------------
// Tab switches the picture between smoothed and pixelated (every pixel a sharp square, nothing blended), and in the pixelated view
// the bottom bar shows the color of the pixel under the pointer. The numbers come from Go, which decodes the file a second time
// (the web view converts colors for the screen, so what it shows is not the number in the file); for types Go cannot decode they are
// read from what the web view drew, and the bar says so. The pixelated view stays when the next picture is shown, but not the next
// time mormiView starts.

let pixelState = '';   // '' not asked, 'preparing', 'go' or 'browser' (where the numbers come from), 'none' (cannot be read)
let pixelCanvas = null;
let lastPointer = null;
let pixelPending = null, pixelBusy = false;

// Edit mode (E): the edit bar, the status bar in full screen, [edit] in the title and the status bar, and the pixelated view (leaving it smooths the picture again).
// The pixelated view follows it, so it is not a step of the history of its own: the steps always show the view the mode has.
const editOn = () => document.body.classList.contains('edit');
function setEdit(on) {
  if (on && !pic.naturalWidth) return; // nothing to edit
  const x0 = view.x, y0 = view.y;
  document.body.classList.toggle('edit', on);
  if (!on) { closeDock(); polyClear(); }
  if (on) setTool('select');
  syncPenDock();
  $('editmark').hidden = !on;
  SetEditMode(on).catch(() => {});
  if (pic.naturalWidth && view.pixelated !== on) setPixelated(on); else view.pixelated = on;
  if (!on && view.free && (Math.abs(x0 - view.x) > 0.5 || Math.abs(y0 - view.y) > 0.5)) { // the picture was in the room the rail and the bar had: it slides back
    view.ox = x0 - view.x; view.oy = y0 - view.y; draw(); startSpring();
  }
  updateHover(false);
  gifSyncFade();
  if (on) gifOffer(); else if (!gif.meta?.edited) gifHide(true); // (an animated GIF: its frame player is for the edit mode, and for a GIF whose frames were changed)
}

// ---- Tools of the edit bar: select (the default when edit mode starts), pencil, line, rectangle, text and the color picker ----------------------------------
// With edit mode off the selection is the only thing there is. The pencil draws one pixel wide strokes with the left button in the first color,
// with the right button in the second; a stroke is shown on #ink while the button is down and then drawn into the picture by Go (a step of the history).
const tool = () => (editOn() ? view.tool : 'select');

function setTool(t) {
  if (t !== 'select' && view.tool !== t) clearSelection(); // drawing has no selection
  if (view.tool === 'poly' && t !== 'poly') polyClear(); // (the polygon belongs to its tool)
  if (view.tool !== t && txt) commitText(); // a text that is being typed is applied when the tool changes
  if (t === 'pick' && view.tool !== 'pick') view.prevTool = view.tool; // the picker gives the tool back
  view.tool = t;
  syncPenDock();
  for (const k of ['select', 'poly', 'pen', 'line', 'rect', 'ellipse', 'fill', 'text', 'pick']) $(`tool-${k}`).classList.toggle('on', k === t);
  updateHover(false);
}

// ---- Polygon select: draw round the part freely; when the button is let go, the end is joined to the start and mormiView turns the line into a polygon ------------
// What is drawn is a free-hand line (every move of the pointer), but what is kept is the polygon that follows it: the line is simplified (Ramer-Douglas-Peucker, with a
// tolerance of about a screen pixel and a quarter, so the polygon stays within a pixel or so of what was drawn) to a few corners, in picture pixels. Once it is set:
// Delete or Backspace fills it with the second color (a step of the history), Ctrl+C copies what is inside it (the rest transparent), Esc lets it go. The AI of the
// chat panel can use it as a mask (selection_mask).
const poly = {pts: [], closed: false, down: false, raw: [], button: 0};
const polyActive = () => tool() === 'poly' || poly.closed || poly.down; // (a polygon that is set answers to its keys also when it was drawn with the right button)

function polyClear() {
  rotSession = null;
  if (!poly.pts.length && !poly.closed && !poly.down) return;
  poly.pts = []; poly.closed = false; poly.down = false; poly.raw = [];
  renderPoly();
}

function polyScreen(p) { return [view.x + view.ox + p[0] * view.sx, view.y + view.oy + p[1] * view.sy]; }

function distToSegment(p, a, b) {
  const dx = b[0] - a[0], dy = b[1] - a[1], l2 = dx * dx + dy * dy;
  if (!l2) return Math.hypot(p[0] - a[0], p[1] - a[1]);
  const t = Math.max(0, Math.min(1, ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / l2));
  return Math.hypot(p[0] - a[0] - t * dx, p[1] - a[1] - t * dy);
}

// Ramer-Douglas-Peucker: the corners of an open line that keep it within eps of the line.
function simplifyLine(pts, eps) {
  if (pts.length < 3) return pts.slice();
  let dmax = 0, at = 0;
  for (let i = 1; i < pts.length - 1; i++) { const d = distToSegment(pts[i], pts[0], pts[pts.length - 1]); if (d > dmax) { dmax = d; at = i; } }
  if (dmax <= eps) return [pts[0], pts[pts.length - 1]];
  return simplifyLine(pts.slice(0, at + 1), eps).slice(0, -1).concat(simplifyLine(pts.slice(at), eps));
}

// The free-hand line (picture pixels, not rounded) as a closed polygon with whole corners; null when it is not a shape (a click, a line).
function vectorize(raw) {
  if (raw.length < 3) return null;
  const eps = clamp(1.25 / Math.max(view.sx, 0.01), 0.6, 12);
  let far = 0, fd = 0; // the ring is cut in two at the point that is farthest from where it started, and each half is simplified
  raw.forEach((q, i) => { const d = Math.hypot(q[0] - raw[0][0], q[1] - raw[0][1]); if (d > fd) { fd = d; far = i; } });
  if (!far) return null;
  const half1 = simplifyLine(raw.slice(0, far + 1), eps), half2 = simplifyLine(raw.slice(far).concat([raw[0]]), eps);
  const ring = half1.slice(0, -1).concat(half2.slice(0, -1));
  const pts = [];
  for (const q of ring) {
    const r = [Math.round(clamp(q[0], 0, dispW())), Math.round(clamp(q[1], 0, dispH()))], last = pts[pts.length - 1];
    if (!last || last[0] !== r[0] || last[1] !== r[1]) pts.push(r);
  }
  while (pts.length > 1 && pts[0][0] === pts[pts.length - 1][0] && pts[0][1] === pts[pts.length - 1][1]) pts.pop();
  if (pts.length < 3) return null;
  let area = 0; // the shoelace formula: a line drawn back on itself has no area
  pts.forEach((q, i) => { const n = pts[(i + 1) % pts.length]; area += q[0] * n[1] - n[0] * q[1]; });
  return Math.abs(area) / 2 >= 4 ? pts : null;
}

function renderPoly() {
  const svg = $('poly');
  const pts = poly.down ? poly.raw : poly.pts;
  if (!pts.length || !pic.naturalWidth) { svg.toggleAttribute('hidden', true); svg.innerHTML = ''; return; } // (an SVG element has no .hidden property: the attribute is set)
  svg.toggleAttribute('hidden', false);
  svg.classList.toggle('set', poly.closed && !poly.down);
  const d = pts.map(polyScreen).map((q) => `${q[0].toFixed(1)},${q[1].toFixed(1)}`).join(' ');
  // while the button is down it is the line as it was drawn (open: the end is joined to the start only when the button is up)
  svg.innerHTML = poly.down ? `<polyline class="k" points="${d}"/><polyline class="w" points="${d}"/>` : `<polygon class="fillp" points="${d}"/><polygon class="k" points="${d}"/><polygon class="w" points="${d}"/>`;
}

function polyDown(ev) {
  poly.pts = []; poly.closed = false; poly.down = true; poly.button = ev.button;
  poly.raw = [pointInPicture(ev.clientX, ev.clientY)];
  renderPoly();
}

function polyMove(ev) {
  if (!poly.down) return;
  const p = pointInPicture(ev.clientX, ev.clientY), last = poly.raw[poly.raw.length - 1];
  if (Math.hypot((p[0] - last[0]) * view.sx, (p[1] - last[1]) * view.sy) >= 1.5) { poly.raw.push(p); renderPoly(); }
}

function polyUp() {
  const raw = poly.raw, pts = vectorize(raw);
  poly.down = false; poly.raw = [];
  if (pts) { poly.pts = pts; poly.closed = true; } else { poly.pts = []; poly.closed = false; }
  renderPoly();
}

// A key for the polygon; true when it was used.
function polyKey(ev) {
  const cmd = ev.ctrlKey || ev.metaKey;
  if (cmd && !ev.altKey && (ev.key === 'c' || ev.key === 'C') && poly.closed) { polyCopy(); return true; }
  if (ev.key === 'Escape' && (poly.pts.length || poly.closed || poly.down)) { polyClear(); return true; }
  if ((ev.key === 'Backspace' || ev.key === 'Delete') && poly.closed) { clearSelectedPart(); return true; }
  return false;
}

// Clear (Effects → Clear, and Delete or Backspace while something is selected): the selected part is filled with the second color, as Delete does in Paint.
// The selection stays, so that undo and another try are at hand.
async function clearSelectedPart() {
  const part = effectPart();
  if (!part.pts && part.x === undefined) return false;
  try { await applyEffect({op: 'fill', color: paint.bg, ...part}, 'cleared'); } catch (e) { problem(errText(e)); }
  return true;
}

// The right-click menu of a selection (a box or a polygon): Cut, Copy, Crop to, Save as, Print, Clear.
function closeSelectionMenu() { const m = $('ctx'); if (m) m.remove(); }
function showSelectionMenu(x, y) {
  closeSelectionMenu();
  const isPoly = !selectionRect() && poly.closed && poly.pts.length >= 3; // (a free-hand selection: the box items that need a rectangle are left out)
  const part = selectionRect() || (isPoly ? {x: 0, y: 0, w: dispW(), h: dispH()} : null), r = part || {x: 0, y: 0, w: dispW(), h: dispH()}; // (no selection: the menu of the whole picture)
  if (!pic.naturalWidth) return;
  const run = (f) => () => { closeSelectionMenu(); f().catch((e) => problem(errText(e))); };
  const recentItems = () => { // the last effects used, the latest first (nothing, until one was used)
    const list = recentEffects.filter((n) => effectLabel(n)).map((n) => [effectLabel(n), () => applyEffectMenu(n)]);
    return list.length ? [['Recent', null, '', null, true], ...list, null] : [];
  };
  const effectItems = () => EFFECT_LABELS.map((e) => e && (e[2] ? [e[0], null, '', e[2].map((s) => [s[0], () => applyEffectMenu(s[1])])] : [e[0], () => applyEffectMenu(e[1])])); // (on the selection, or on all of the picture)
  const items = part ? [
    ['Cut', async () => { await (isPoly ? polyCopy() : copySelection()); await clearSelectedPart(); }],
    ['Copy', () => (isPoly ? polyCopy() : copySelection()), `${MAC ? '⌘' : 'Ctrl+'}C`],
    ...(isPoly ? [] : [['Crop to selection', cropToSelection], ['Save selection as…', saveAs], ['Print…', printSelection]]),
    null,
    ...recentItems(),
    ['Effects', null, '', effectItems()],
    ['Rotate right', () => rotatePart(90), 'R'],
    ['Rotate left', () => rotatePart(-90), '⇧R'],
    ['Rotate by angle…', rotateByAngle],
    null,
    ['Clear', async () => { await clearSelectedPart(); }, '⌫'],
  ] : [
    ['Copy', () => copySelection(), `${MAC ? '⌘' : 'Ctrl+'}C`],
    ['Select all', async () => { view.sel = {x0: 0, y0: 0, x1: dispW(), y1: dispH()}; view.selDrag = null; draw(); }, `${MAC ? '⌘' : 'Ctrl+'}A`],
    ['Save as…', saveAs],
    ['Print…', printSelection],
    null,
    ...recentItems(),
    ['Effects', null, '', effectItems()],
    ['Rotate right', () => imageMenuAction('rotate-right'), 'R'],
    ['Rotate left', () => imageMenuAction('rotate-left'), '⇧R'],
    ['Rotate by angle…', rotateByAngle],
  ];
  const menu = document.createElement('div');
  menu.id = 'ctx'; menu.setAttribute('role', 'menu');
  const fill = (box, list) => {
    for (const it of list) {
      if (!it) { box.appendChild(document.createElement('hr')); continue; }
      if (it[4]) { const c = document.createElement('div'); c.className = 'cap'; c.textContent = it[0]; box.appendChild(c); continue; } // a caption
      const b = document.createElement('button');
      b.setAttribute('role', 'menuitem');
      b.innerHTML = `<span></span><kbd></kbd>`;
      b.firstChild.textContent = it[0]; b.lastChild.textContent = it[2] || '';
      if (it[3]) { // a submenu, open while the pointer is over it
        b.classList.add('has-sub'); b.lastChild.textContent = '▸';
        const sm = document.createElement('div'); sm.className = 'sub'; fill(sm, it[3]); b.appendChild(sm);
        b.onmouseenter = () => {
          sm.style.display = 'flex'; sm.style.top = '-6px'; sm.style.left = '100%'; sm.style.right = 'auto';
          const r = sm.getBoundingClientRect();
          if (r.right > innerWidth - 4) { sm.style.left = 'auto'; sm.style.right = '100%'; }
          if (r.bottom > innerHeight - 4) sm.style.top = `${-6 - (r.bottom - innerHeight + 4)}px`;
        };
        b.onmouseleave = () => { sm.style.display = 'none'; };
        b.onclick = (ev) => ev.stopPropagation();
      } else b.onclick = run(it[1]);
      box.appendChild(b);
    }
  };
  fill(menu, items);
  document.body.appendChild(menu);
  const w = menu.offsetWidth, h = menu.offsetHeight;
  menu.style.left = `${Math.max(4, Math.min(x, innerWidth - w - 4))}px`;
  menu.style.top = `${Math.max(4, Math.min(y, innerHeight - h - 4))}px`;
  const away = (ev) => { if (!ev.target.closest?.('#ctx')) { closeSelectionMenu(); cleanup(); } };
  const key = (ev) => { if (ev.key === 'Escape') { ev.stopPropagation(); closeSelectionMenu(); cleanup(); } };
  const cleanup = () => { window.removeEventListener('pointerdown', away, true); window.removeEventListener('keydown', key, true); window.removeEventListener('blur', closeSelectionMenu); };
  window.addEventListener('pointerdown', away, true); window.addEventListener('keydown', key, true); window.addEventListener('blur', closeSelectionMenu);
}

async function cropToSelection() {
  if (view.fl) await flushFloat();
  const r = selectionRect();
  if (!r) return;
  const info = await CropPicture(view.rot, r.x, r.y, r.w, r.h);
  clearSelection();
  await showImage(info, {rot: 0});
  commit('cropped');
}

async function printSelection() {
  if (view.fl) await flushFloat();
  const r = selectionRect();
  if (!r) return;
  const note = await PrintPNG(await cutPng(r));
  if (note) problem(note, false);
}

async function polyCopy() {
  try {
    if (view.fl) await flushFloat();
    const b64 = await PolygonPNG(view.rot, JSON.stringify(poly.pts));
    try { await navigator.clipboard.write([new ClipboardItem({'image/png': base64ToBlob(b64, 'image/png')})]); } catch { await CopyImageNative(b64); }
    flashClipboard();
  } catch (e) { problem(errText(e)); }
}

let ink = null; // the stroke that is being drawn: {pts: [x, y, x, y, …], color}
let fin = null; // the stroke that is let go and is being drawn into the picture by Go: only its overlay is still shown

function inkPoint(ev) {
  const [px, py] = pointInPicture(ev.clientX, ev.clientY);
  return [clamp(Math.floor(px), 0, dispW() - 1), clamp(Math.floor(py), 0, dispH() - 1)];
}

function renderInk() {
  const cv = $('ink');
  if (!ink && !fin && !shape) { cv.hidden = true; return; }
  cv.style.left = `${view.x + view.ox}px`; cv.style.top = `${view.y + view.oy}px`;
  cv.style.width = `${dispW() * view.sx}px`; cv.style.height = `${dispH() * view.sy}px`;
  cv.hidden = false;
}

// The round tip of the pencil: the pixels around the point that are inside a circle of the width (Go draws the same, see DrawStroke).
function stampOffsets(width) {
  const lim = (width / 2 - 0.25) ** 2, mid = (width - 1) / 2, c = Math.floor((width - 1) / 2), out = [];
  for (let j = 0; j < width; j++) for (let i = 0; i < width; i++) if ((i - mid) ** 2 + (j - mid) ** 2 <= lim) out.push([i - c, j - c]);
  return out;
}

function startInk(ev) {
  const cv = $('ink');
  cv.width = dispW(); cv.height = dispH();
  ink = {button: ev.button, pts: [], width: paint.width, color: ev.button === 2 ? paint.bg : paint.fg, ctx: cv.getContext('2d')};
  ink.tip = stampOffsets(ink.width);
  ink.ctx.fillStyle = ink.color;
  addInk(inkPoint(ev));
  renderInk();
}

function addInk([x, y]) {
  const n = ink.pts.length;
  if (n && ink.pts[n - 2] === x && ink.pts[n - 1] === y) return;
  let [x1, y1] = n ? [ink.pts[n - 2], ink.pts[n - 1]] : [x, y];
  const dx = Math.abs(x - x1), dy = -Math.abs(y - y1), sx = x1 < x ? 1 : -1, sy = y1 < y ? 1 : -1;
  let e = dx + dy;
  for (;;) { // the same line Go draws (Bresenham)
    for (const [i, j] of ink.tip) ink.ctx.fillRect(x1 + i, y1 + j, 1, 1);
    if (x1 === x && y1 === y) break;
    const e2 = 2 * e;
    if (e2 >= dy) { e += dy; x1 += sx; }
    if (e2 <= dx) { e += dx; y1 += sy; }
  }
  ink.pts.push(x, y);
}

async function finishInk(k) {
  fin = k; // the overlay stays until the picture has the stroke, but the stroke itself is over
  try {
    const info = await DrawStroke(view.rot, k.pts, k.color, k.width);
    await showImage(info, {keepView: true, rot: 0});
    commit('pen stroke');
  } catch (e) { problem(errText(e)); }
  if (fin === k) fin = null; // (a stroke that started meanwhile has the overlay now)
  renderInk();
}

// ---- Line, rectangle and ellipse: dragged out on #ink (Shift: a line at a multiple of 45°, a square or a circle), and drawn into the picture by Go from the pixels of #ink ----
// The left button draws with the first color, the right button with the second (the rectangle: the other way round, as in Paint).
// A stroke, line or rectangle that is being drawn is dropped at once by Esc, or by the other mouse button, even though the first is still down.
// Esc or a right click drops what is being drawn or selected: a stroke, a shape, the frame of a selection that is being dragged, the line of the polygon select.
// It is true when there was something to drop.
function cancelDraw() {
  let did = false;
  if (poly.down) { polyClear(); did = true; }
  if (view.selDrag && !view.selDrag.edge) { // a new frame (not the move of an edge of one that is set)
    view.selDrag = null; view.sel = null; view.hl = null; did = true;
    draw();
  }
  if (shape || ink) {
    shape = null; ink = null;
    renderInk(); updateHover(false);
    did = true;
  }
  return did;
}
const otherButton = (ev, d) => (ev.buttons & ~(d.button === 2 ? 2 : 1)) !== 0; // a button that is not the one the drawing was started with

let shape = null; // the one that is dragged: {kind, x0, y0, x1, y1, fg, bg, swap, width, style, fill, ctx, moved}

const dashes = (w, style) => style === 'dashed' ? [Math.max(4, w * 3), Math.max(3, w * 2)] : style === 'dotted' ? [0, Math.max(3, w * 2)] : [];

function drawShape(ctx, s) {
  const w = s.width, [c1, c2] = s.swap ? [s.bg, s.fg] : [s.fg, s.bg]; // the outline, and what fills a rectangle with an outline
  ctx.save();
  ctx.strokeStyle = c1; ctx.lineWidth = w; ctx.lineJoin = 'miter';
  ctx.lineCap = s.style === 'solid' || s.style === 'dotted' ? 'round' : 'butt';
  if (s.kind === 'line') {
    ctx.setLineDash(dashes(w, s.style));
    ctx.beginPath(); ctx.moveTo(s.x0 + 0.5, s.y0 + 0.5); ctx.lineTo(s.x1 + 0.5, s.y1 + 0.5); ctx.stroke();
  } else { // a rectangle, or the ellipse in the box that is dragged
    const xa = Math.min(s.x0, s.x1), ya = Math.min(s.y0, s.y1), W = Math.abs(s.x1 - s.x0) + 1, H = Math.abs(s.y1 - s.y0) + 1;
    const round = s.kind === 'ellipse';
    const path = (inset) => { ctx.beginPath(); if (round) ctx.ellipse(xa + W / 2, ya + H / 2, Math.max(0, W / 2 - inset), Math.max(0, H / 2 - inset), 0, 0, Math.PI * 2); else ctx.rect(xa + inset, ya + inset, W - 2 * inset, H - 2 * inset); };
    if (s.fill === 'fill' || W <= w || H <= w) { ctx.fillStyle = c1; path(0); ctx.fill(); } // (a shape that is all outline is a filled one)
    else {
      if (s.fill === 'both') { ctx.fillStyle = c2; path(0); ctx.fill(); }
      ctx.setLineDash(dashes(w, s.style));
      path(w / 2); ctx.stroke(); // the outline is inside the dragged box
    }
  }
  ctx.restore();
}

function renderShape() {
  const cv = $('ink');
  shape.ctx.clearRect(0, 0, cv.width, cv.height);
  drawShape(shape.ctx, shape);
  renderInk();
}

function startShape(ev, kind) {
  const cv = $('ink');
  cv.width = dispW(); cv.height = dispH();
  const [x, y] = inkPoint(ev);
  shape = {kind, x0: x, y0: y, x1: x, y1: y, fg: paint.fg, bg: paint.bg, swap: ev.button === 2, width: paint.width, style: paint.style, fill: paint.fill, ctx: cv.getContext('2d'), moved: false, button: ev.button};
  renderShape();
}

// With Shift: the line goes at the nearest multiple of 45°, the rectangle is a square (as big as the picture lets it be).
function constrainShape(s, x, y) {
  const dx = x - s.x0, dy = y - s.y0;
  if (s.kind === 'line') {
    const a = Math.round(Math.atan2(dy, dx) / (Math.PI / 4)) * (Math.PI / 4), len = Math.hypot(dx, dy);
    return [clamp(s.x0 + Math.round(Math.cos(a) * len), 0, dispW() - 1), clamp(s.y0 + Math.round(Math.sin(a) * len), 0, dispH() - 1)];
  }
  const sx = dx < 0 ? -1 : 1, sy = dy < 0 ? -1 : 1;
  const side = Math.min(Math.max(Math.abs(dx), Math.abs(dy)), sx > 0 ? dispW() - 1 - s.x0 : s.x0, sy > 0 ? dispH() - 1 - s.y0 : s.y0);
  return [s.x0 + sx * side, s.y0 + sy * side];
}

function moveShape(ev) {
  let [x, y] = inkPoint(ev);
  if (ev.shiftKey) [x, y] = constrainShape(shape, x, y);
  shape.x1 = x; shape.y1 = y;
  shape.moved = shape.moved || x !== shape.x0 || y !== shape.y0;
  renderShape();
}

async function finishShape(s) {
  if (!s.moved) { renderInk(); return; } // a click is not a shape
  fin = s; // the overlay stays until the picture has it
  try {
    const cv = $('ink'), pad = s.kind === 'line' ? Math.ceil(s.width / 2) + 2 : 2; // (what is drawn, with a little room for the edges)
    const bx = Math.max(0, Math.min(s.x0, s.x1) - pad), by = Math.max(0, Math.min(s.y0, s.y1) - pad);
    const bw = Math.min(cv.width, Math.max(s.x0, s.x1) + pad + 1) - bx, bh = Math.min(cv.height, Math.max(s.y0, s.y1) + pad + 1) - by;
    const part = document.createElement('canvas'); part.width = bw; part.height = bh;
    part.getContext('2d').drawImage(cv, bx, by, bw, bh, 0, 0, bw, bh); // only the box around it goes to Go
    const info = await DrawLayer(view.rot, bx, by, part.toDataURL('image/png').split(',')[1]);
    await showImage(info, {keepView: true, rot: 0});
    commit(`${s.kind} drawn`);
  } catch (e) { problem(errText(e)); }
  if (fin === s) fin = null;
  renderInk();
}

// ---- Fill: a click fills the area of the same color (give or take the tolerance) with the first color, a right click with the second ----
async function fillAt(ev) {
  const at = pixelUnder(ev.clientX, ev.clientY);
  if (!at) return;
  try {
    if (view.fl) await flushFloat({clear: true});
    const info = await FloodFill(view.rot, at[0], at[1], ev.button === 2 ? paint.bg : paint.fg, Math.round(paint.tol * 2.55));
    if (info.state === view.state) return; // nothing changed
    await showImage(info, {keepView: true, rot: 0});
    commit('paint bucket fill');
  } catch (e) { problem(errText(e)); }
}

// ---- Color picker: a click takes the color of the pixel as the first color (right click: the second), and the tool that was used comes back ----
async function fetchPixel(X, Y) {
  const [fx, fy] = toNatural(X + 0.5, Y + 0.5), x = Math.floor(fx), y = Math.floor(fy); // the pixel in the file; the readout says where it is on the screen
  let p = null;
  try {
    if (pixelState === 'go') p = await PixelAt(x, y, pic.naturalWidth, pic.naturalHeight);
    else if (pixelState === 'browser' && pixelCanvas) {
      const d = pixelCanvas.getImageData(x, y, 1, 1).data;
      p = {ready: true, x, y, r: d[0], g: d[1], b: d[2], a: d[3]};
    }
    if (p) { p.x = X; p.y = Y; }
  } catch {}
  return p;
}

async function pickColor(ev) {
  const at = pixelUnder(ev.clientX, ev.clientY);
  if (!at) return;
  if (pixelState === 'preparing') await preparePixels();
  const p = await fetchPixel(at[0], at[1]);
  if (!p?.ready) return;
  paint[ev.button === 2 ? 'bg' : 'fg'] = '#' + [p.r, p.g, p.b].map((v) => v.toString(16).padStart(2, '0')).join('').toUpperCase();
  showColors();
  setTool(view.prevTool && view.prevTool !== 'pick' ? view.prevTool : 'pen');
}

// ---- Text: a click puts a box on the picture to type in; a click elsewhere (or another tool, or Ctrl+Enter) draws it into the picture, Esc drops it ----
let txt = null; // the text that is being typed: {x, y, color}, in picture pixels
const textFont = (px) => `${paint.italic ? 'italic ' : ''}${paint.bold ? 'bold ' : ''}${px}px ${paint.font}`;

function renderText() {
  const tb = $('textbox');
  if (!txt) { tb.hidden = true; return; }
  tb.style.left = `${view.x + view.ox + txt.x * view.sx}px`; tb.style.top = `${view.y + view.oy + txt.y * view.sy}px`;
  tb.style.font = textFont(paint.size * view.sy); tb.style.lineHeight = '1.2'; tb.style.color = txt.color;
  const lines = tb.value.split('\n'), m = document.createElement('canvas').getContext('2d');
  m.font = textFont(paint.size);
  const w = Math.max(...lines.map((l) => m.measureText(l).width), paint.size / 2);
  tb.style.width = `${(w + 4) * view.sx}px`; tb.style.height = `${lines.length * paint.size * 1.2 * view.sy}px`;
  tb.hidden = false;
}

function startText(ev) {
  const at = pixelUnder(ev.clientX, ev.clientY);
  if (txt) commitText();
  if (!at) return;
  txt = {x: at[0], y: at[1], color: paint.fg};
  const tb = $('textbox'); tb.value = '';
  renderText();
  setTimeout(() => tb.focus(), 0); // (after the click has moved the focus)
}

// A line broken into lines that are at most maxW wide (at the spaces; a word that is longer stays whole).
function wrapLine(ctx, line, maxW) {
  const out = []; let cur = '';
  for (const word of line.split(' ')) {
    const next = cur ? `${cur} ${word}` : word;
    if (cur && ctx.measureText(next).width > maxW) { out.push(cur); cur = word; } else cur = next;
  }
  out.push(cur);
  return out;
}

function cancelText() { txt = null; const tb = $('textbox'); tb.value = ''; tb.hidden = true; }

async function commitText() {
  const t = txt;
  if (!t) return;
  const tb = $('textbox'), text = tb.value.replace(/\s+$/, '');
  cancelText();
  if (!text.trim()) return;
  // the options of the chat's text tool: {align, boxW (wrap at this width), outline, outlineW, shadow {dx, dy, blur, color}}
  const o = t.opts || {}, ow = o.outline ? Math.max(1, o.outlineW || 2) : 0;
  const sh = o.shadow ? {dx: o.shadow.dx ?? 2, dy: o.shadow.dy ?? 2, blur: o.shadow.blur ?? 3, color: o.shadow.color || '#000000'} : null;
  const lh = paint.size * 1.2, cv = document.createElement('canvas');
  let ctx = cv.getContext('2d'); ctx.font = textFont(paint.size);
  let lines = text.split('\n');
  if (o.boxW > 0) lines = lines.flatMap((l) => wrapLine(ctx, l, o.boxW));
  const widest = Math.ceil(Math.max(...lines.map((l) => ctx.measureText(l).width))), bw = o.boxW > 0 ? o.boxW : widest;
  const pad = ow + (sh ? Math.max(Math.abs(sh.dx), Math.abs(sh.dy)) + sh.blur : 0) + 1;
  const lx = Math.max(0, t.x - pad), ly = Math.max(0, t.y - pad), ox = t.x - lx, oy = t.y - ly; // (the layer starts a little before the text, for the outline and the shadow)
  cv.width = Math.ceil(ox + bw + pad) + 1; cv.height = Math.ceil(oy + lines.length * lh + pad) + 1;
  ctx = cv.getContext('2d'); ctx.font = textFont(paint.size); ctx.fillStyle = t.color; ctx.textBaseline = 'middle'; ctx.lineJoin = 'round';
  lines.forEach((l, i) => {
    const w = ctx.measureText(l).width, x = ox + (o.align === 'center' ? (bw - w) / 2 : o.align === 'right' ? bw - w : 0), y = oy + i * lh + lh / 2;
    if (sh) { ctx.shadowColor = sh.color; ctx.shadowBlur = sh.blur; ctx.shadowOffsetX = sh.dx; ctx.shadowOffsetY = sh.dy; }
    if (ow) { ctx.strokeStyle = o.outline; ctx.lineWidth = ow * 2; ctx.strokeText(l, x, y); ctx.shadowColor = 'transparent'; }
    ctx.fillText(l, x, y);
    ctx.shadowColor = 'transparent';
  });
  try {
    const info = await DrawLayer(view.rot, lx, ly, cv.toDataURL('image/png').split(',')[1]);
    await showImage(info, {keepView: true, rot: 0});
    commit('text added');
  } catch (e) { problem(errText(e)); }
}

// The controls of the ribbon for these tools (the width is the one of the pencil, see showPenWidth).
function showToolOptions() {
  for (const b of $('seg-style').children) b.classList.toggle('on', b.dataset.v === paint.style);
  for (const b of $('seg-fill').children) b.classList.toggle('on', b.dataset.v === paint.fill);
  $('fill-tol').value = paint.tol; $('fill-tolv').textContent = `${paint.tol}%`;
  $('txt-b').classList.toggle('on', paint.bold); $('txt-i').classList.toggle('on', paint.italic);
  $('txt-size').value = paint.size; $('txt-font').value = paint.font;
}

const FONTS = [['Sans', 'system-ui, sans-serif'], ['Serif', 'Georgia, serif'], ['Monospace', 'ui-monospace, Menlo, Consolas, monospace'], ['Arial', 'Arial, sans-serif'],
  ['Times New Roman', '"Times New Roman", serif'], ['Courier New', '"Courier New", monospace'], ['Verdana', 'Verdana, sans-serif'], ['Impact', 'Impact, sans-serif']];

function initToolOptions() {
  for (const [name, css] of FONTS) { const o = document.createElement('option'); o.value = css; o.textContent = name; $('txt-font').append(o); }
  for (const b of $('seg-style').children) b.onclick = () => { paint.style = b.dataset.v; showToolOptions(); };
  for (const b of $('seg-fill').children) b.onclick = () => { paint.fill = b.dataset.v; showToolOptions(); };
  $('fill-tol').addEventListener('input', () => { paint.tol = +$('fill-tol').value; showToolOptions(); });
  const again = () => { showToolOptions(); renderText(); };
  $('txt-font').onchange = () => { paint.font = $('txt-font').value; again(); $('txt-font').blur(); };
  $('txt-size').oninput = () => { const v = +$('txt-size').value; if (v >= 6 && v <= 400) { paint.size = v; renderText(); } };
  $('txt-b').onclick = () => { paint.bold = !paint.bold; again(); };
  $('txt-i').onclick = () => { paint.italic = !paint.italic; again(); };
  const tb = $('textbox');
  tb.addEventListener('input', renderText);
  tb.addEventListener('keydown', (ev) => {
    if (ev.key === 'Escape') { ev.preventDefault(); cancelText(); }
    else if (ev.key === 'Enter' && (ev.metaKey || ev.ctrlKey)) { ev.preventDefault(); commitText(); }
  });
  showToolOptions();
}

// ---- Colors: the two wells at the bottom of the edit bar, and the palette that slides up from the status bar ----------
// A click on a well opens the palette for that color; a click anywhere else closes it, and so does the pointer leaving it once a color was changed in it.
const PALETTE = ['#000000', '#7F7F7F', '#880015', '#ED1C24', '#FF7F27', '#FFF200', '#22B14C', '#00A2E8', '#3F48CC', '#A349A4',
  '#FFFFFF', '#C3C3C3', '#B97A57', '#FFAEC9', '#FFC90E', '#EFE4B0', '#B5E61D', '#99D9EA', '#7092BE', '#C8BFE7'];
const paint = {fg: '#ED1C24', bg: '#FFFFFF', target: 'fg', changed: false, native: false, width: 5, style: 'solid', fill: 'outline', tol: 10,
  font: 'system-ui, sans-serif', size: 32, bold: false, italic: false};

// The ribbon with the options of the tool (width, style, fill, font…): there while a drawing tool is the tool (and the palette is not).
function syncPenDock() {
  const open = editOn() && view.tool !== 'select' && !$('dock').classList.contains('open');
  for (const g of $('pendock').querySelectorAll('.grp')) g.hidden = !g.dataset.for.split(' ').includes(view.tool);
  $('pendock').classList.toggle('open', open);
  dockSpace();
}

// An open dock sits at the bottom edge; the status capsule, the chat button and the frame player float above it (--dockh).
function dockSpace() {
  const h = Math.max(...['dock', 'pendock'].map((id) => ($(id).classList.contains('open') ? $(id).offsetHeight : 0)));
  document.body.style.setProperty('--dockh', `${h}px`);
}

function showPenWidth() {
  $('pen-wv').textContent = `${paint.width} px`;
  const d = Math.min(paint.width, 28);
  $('pen-dot').style.width = $('pen-dot').style.height = `${d}px`;
  $('pen-dot').style.background = paint.fg;
}

function showColors() {
  showPenWidth();
  $('well-fg').firstElementChild.style.background = paint.fg;
  $('well-bg').firstElementChild.style.background = paint.bg;
  for (const c of $('palette').children) c.classList.toggle('sel', c.dataset.color === paint[paint.target]);
}

function openDock(target) {
  paint.target = target; paint.changed = false;
  $('dock-native').value = paint[target].toLowerCase();
  $('dock').classList.add('open');
  dockSpace();
  syncPenDock();
  showColors();
}

function closeDock() {
  if (paint.native) return; // the system picker is open: it closes the palette when it is done
  $('dock').classList.remove('open');
  syncPenDock();
}

function setColor(c) {
  paint[paint.target] = c.toUpperCase(); paint.changed = true;
  showColors();
}

function initDock() {
  for (const c of PALETTE) {
    const b = document.createElement('button'); b.className = 'cell'; b.dataset.color = c; b.title = c; b.style.background = c;
    b.onclick = () => setColor(c);
    $('palette').append(b);
  }
  $('pen-w').addEventListener('input', () => { paint.width = +$('pen-w').value; showPenWidth(); });
  showPenWidth();
  initToolOptions();
  for (const t of ['fg', 'bg']) {
    $(`well-${t}`).onclick = () => { if ($('dock').classList.contains('open') && paint.target === t) closeDock(); else openDock(t); };
  }
  $('dock-more').onclick = () => { paint.native = true; $('dock-native').click(); };
  $('dock-native').addEventListener('input', (ev) => setColor(ev.target.value));
  $('dock-native').addEventListener('change', () => { paint.native = false; });
  window.addEventListener('focus', () => { paint.native = false; }); // back from the system picker, however it was closed
  $('dock').addEventListener('mouseleave', () => { if (paint.changed) closeDock(); });
  window.addEventListener('pointerdown', (ev) => { if (!ev.target.closest('#dock, #wells, #pendock')) closeDock(); }, true);
  showColors();
}

function setPixelated(on) {
  view.pixelated = on;
  pic.classList.toggle('pixelated', on);
  clearPixel();
  if (on && pic.naturalWidth) preparePixels();
  draw();
}

let pixelPromise = null;

// Decodes the picture for the readout and for copying (once per picture); everyone who needs it waits for the same answer.
function preparePixels() {
  if (!pixelPromise) pixelPromise = doPreparePixels();
  return pixelPromise;
}

async function doPreparePixels() {
  const token = view.token;
  pixelState = 'preparing';
  let source = '';
  try { source = await PreparePixels(); } catch (e) { problem(errText(e), false); }
  if (token !== view.token) return; // another picture meanwhile
  if (source === 'browser') {
    try {
      const w = pic.naturalWidth, h = pic.naturalHeight;
      if (w * h > 100e6) throw new Error('too big');
      const c = document.createElement('canvas'); c.width = w; c.height = h;
      pixelCanvas = c.getContext('2d', {willReadFrequently: true, colorSpace: 'srgb'});
      pixelCanvas.drawImage(pic, 0, 0);
    } catch { pixelCanvas = null; source = ''; }
  }
  pixelState = source || 'none';
  if (lastPointer) readPixel(lastPointer.x, lastPointer.y);
}

function clearPixel() { pixelPending = null; $('pixel').hidden = true; }

const hex2 = (v) => v.toString(16).toUpperCase().padStart(2, '0');

function showPixel(p) {
  if (!view.pixelated || !p?.ready) { if (pixelState !== 'preparing') clearPixel(); return; }
  // x 12  y 34 | R ▬ 255  G ▬ 128  B ▬ 0 | (#rrggbb, swatch); a pixel that is not opaque has its alpha as a fourth meter and at the end of the hex code
  const code = `#${hex2(p.r)}${hex2(p.g)}${hex2(p.b)}${p.a < 255 ? hex2(p.a) : ''}`;
  const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e; };
  const pos = el('span', 'pos');
  for (const [k, v] of [['x', p.x], ['y', p.y]]) { const s = el('span'); s.append(el('i', '', k), String(v)); pos.append(s); }
  const chans = el('span', 'chans');
  const channels = [['R', p.r, 'r'], ['G', p.g, 'g'], ['B', p.b, 'b']];
  if (p.a < 255) channels.push(['A', p.a, 'a']);
  for (const [name, v, cls] of channels) {
    const meter = el('span', 'm'), fill = el('s'); fill.style.width = `${v / 255 * 100}%`; meter.append(fill);
    const ch = el('span', `ch ${cls}`); ch.append(el('em', '', name), meter, el('span', 'n', String(v))); chans.append(ch);
  }
  const sw = el('span', 'sw');
  const fill = el('b'); fill.style.background = `rgba(${p.r},${p.g},${p.b},${p.a / 255})`; sw.append(fill);
  const chip = el('span', 'chip'); chip.append(el('span', '', code), sw);
  const items = [pos, el('span', 'sep'), chans, el('span', 'sep'), chip];
  if (pixelState === 'browser') items.push(el('span', 'from', 'from the screen'));
  $('pixel').replaceChildren(...items);
  $('pixel').hidden = false;
}

// The pixel of the picture under a position of the window, or nothing when the pointer is beside the picture.
function pixelUnder(clientX, clientY) {
  const r = $('stage').getBoundingClientRect();
  const x = Math.floor((clientX - r.left - view.x - view.ox) / view.sx), y = Math.floor((clientY - r.top - view.y - view.oy) / view.sy);
  return x < 0 || y < 0 || x >= dispW() || y >= dispH() ? null : [x, y];
}

// One question at a time: while Go answers, only the newest position is kept.
async function readPixel(clientX, clientY) {
  if (!view.pixelated || !pic.naturalWidth) return;
  const at = pixelUnder(clientX, clientY);
  if (!at) { clearPixel(); return; }
  if (pixelState === 'preparing') {
    $('pixel').replaceChildren(document.createTextNode('reading the pixels…')); $('pixel').hidden = false; return;
  }
  pixelPending = at;
  if (pixelBusy) return;
  pixelBusy = true;
  const token = view.token;
  while (pixelPending) {
    const [X, Y] = pixelPending; pixelPending = null;
    const p = await fetchPixel(X, Y);
    if (token === view.token) showPixel(p);
  }
  pixelBusy = false;
}

// ---- Selection and copy ----------------------------------------------------------------------------------
// Shift and drag selects a part of the picture; Ctrl+C (Cmd+C on a Mac, Ctrl+Insert) puts just that part on the clipboard, or the whole
// picture when nothing is selected. The part is cut out of the decoded file (see SelectionPNG), so it has the colors of the file.
// In the pixelated view the selection snaps to whole pixels; in the smoothed view it follows the pointer, and what is copied is the
// pixels it touches. The selection is kept in picture pixels, so it stays on its place when the picture is zoomed or moved.

// view.sel is the frame, {x0, y0, x1, y1} in picture pixels, and its end is the edge of the frame, not a pixel: 3 to 6 is the pixels 3, 4 and 5.
// With the mouse in the pixelated view it is made of the pixel under each end, both included (see dragFrame), so it is whole pixels already.

// The selection as whole pixels of the picture: {x, y, w, h}, or null. In the smoothed view a frame that is not on whole pixels
// covers every pixel it touches.
function selectionRect() {
  const s = view.sel, nw = dispW(), nh = dispH();
  if (!s || !nw) return null;
  let x0 = Math.floor(Math.min(s.x0, s.x1)), x1 = Math.ceil(Math.max(s.x0, s.x1));
  let y0 = Math.floor(Math.min(s.y0, s.y1)), y1 = Math.ceil(Math.max(s.y0, s.y1));
  x0 = clamp(x0, 0, nw); x1 = clamp(x1, 0, nw); y0 = clamp(y0, 0, nh); y1 = clamp(y1, 0, nh);
  return x1 > x0 && y1 > y0 ? {x: x0, y: y0, w: x1 - x0, h: y1 - y0} : null;
}

// The frame that a drag from (ax, ay) to (bx, by), in picture pixels, makes: exactly where the pointer went when smoothed, and the pixel under each
// end (both included) when pixelated.
function dragFrame(ax, ay, bx, by) {
  const x0 = Math.min(ax, bx), x1 = Math.max(ax, bx), y0 = Math.min(ay, by), y1 = Math.max(ay, by);
  if (!view.pixelated) return {x0, y0, x1, y1};
  return {x0: Math.floor(x0), y0: Math.floor(y0), x1: Math.floor(x1) + 1, y1: Math.floor(y1) + 1};
}

// The frame on the screen: on whole pixels when pixelated, exactly where the pointer went when smoothed.
// The frame in picture pixels: {x0, x1, y0, y1}, or null.
function selectionBox() {
  const s = view.sel;
  if (!s || !pic.naturalWidth) return null;
  if (view.pixelated) {
    const r = selectionRect();
    return r && {x0: r.x, x1: r.x + r.w, y0: r.y, y1: r.y + r.h};
  }
  return {x0: Math.min(s.x0, s.x1), x1: Math.max(s.x0, s.x1), y0: Math.min(s.y0, s.y1), y1: Math.max(s.y0, s.y1)};
}

// What was put on the clipboard last, as the frame it was (null: the whole picture, or something that is not here any more). The frame
// of that part is drawn with marching ants for as long as it is the frame that is there; clearing the selection does not touch the clipboard.
let copiedKey = null;
// (by the pixels it takes: a frame moved by less than a pixel is the same part)
const selKey = () => { const r = view.sel && selectionRect(); return r ? `${view.state}:${view.rot}:${r.x},${r.y},${r.w},${r.h}` : null; };

function renderSelection() {
  const el = $('sel'), box = selectionBox();
  if (!box) { el.hidden = true; return; }
  el.classList.toggle('drawing', !!(view.selDrag || view.tdrag)); // while the button is down the dashes stand still; set, they move
  el.classList.toggle('copied', copiedKey !== null && copiedKey === selKey());
  el.classList.toggle('content', view.hlMode === 'content');
  el.classList.toggle('movable', !!view.fl || document.body.classList.contains('shift')); // blue dashes and a move sign: a floating part moves with a plain drag, the picture under a selection with Shift
  const dr = selectionRect(); el.querySelector('.dims').textContent = dr ? `${dr.w} × ${dr.h}` : '';
  const {x0, x1, y0, y1} = box;
  const tl = el.querySelector('.corner.tl'), br = el.querySelector('.corner.br'); // the first and the last pixel of the frame (both in it)
  tl.textContent = dr ? `[${dr.x}, ${dr.y}]` : ''; br.textContent = dr ? `[${dr.x + dr.w - 1}, ${dr.y + dr.h - 1}]` : '';
  for (const e of ['t', 'b', 'l', 'r']) el.querySelector(`.edge.${e}`).classList.toggle('on', !!view.hl?.[e]);
  el.style.left = `${view.x + view.ox + x0 * view.sx}px`; el.style.top = `${view.y + view.oy + y0 * view.sy}px`;
  const w = Math.max(3, Math.round((x1 - x0) * view.sx)), h = Math.max(3, Math.round((y1 - y0) * view.sy));
  el.style.width = `${w}px`; el.style.height = `${h}px`;
  el.classList.toggle('tiny', w < 48 || h < 48); // (the move sign would cover all of it)
  { // the corner labels go above and below the frame; where the window has no room for one, it goes inside. Zoomed in, a corner (or the middle)
    // may be out of the window: its label then goes inside the part of the frame that shows, at that corner of it (the size: in its middle)
    const L = view.x + view.ox + x0 * view.sx, T = view.y + view.oy + y0 * view.sy, st = $('stage'), cap = $('status');
    const vx0 = editOn() ? $('tools').offsetWidth : 0, vy0 = 0, vx1 = st.clientWidth, vy1 = st.clientHeight - (cap.hidden ? 0 : cap.offsetHeight + 28);
    const ix0 = Math.max(L, vx0), ix1 = Math.min(L + w, vx1), iy0 = Math.max(T, vy0), iy1 = Math.min(T + h, vy1); // what shows of the frame
    const seen = (x, y) => x >= vx0 && x <= vx1 && y >= vy0 && y <= vy1;
    const pinTl = !seen(L, T), pinBr = !seen(L + w, T + h), M = 8;
    Object.assign(tl.style, pinTl ? {left: `${ix0 - L + M}px`, top: `${iy0 - T + M}px`, right: 'auto', bottom: 'auto'} : {left: '', top: '', right: '', bottom: ''});
    Object.assign(br.style, pinBr ? {right: `${L + w - ix1 + M}px`, bottom: `${T + h - iy1 + M}px`, left: 'auto', top: 'auto'} : {left: '', top: '', right: '', bottom: ''});
    tl.classList.toggle('in-y', !pinTl && T < 16); br.classList.toggle('in-y', !pinBr && T + h + 16 > st.clientHeight);
    const dims = el.querySelector('.dims');
    if (seen(L + w / 2, T + h / 2)) dims.style.left = dims.style.top = '';
    else { dims.style.left = `${(ix0 + ix1) / 2 - L}px`; dims.style.top = `${(iy0 + iy1) / 2 - T}px`; }
  }
  const svg = el.querySelector('.ants'); // the dashes: a rectangle of numbers, so that nothing of an earlier size is left on the screen
  svg.setAttribute('width', w); svg.setAttribute('height', h);
  for (const r of svg.querySelectorAll('rect')) { r.setAttribute('x', 1); r.setAttribute('y', 1); r.setAttribute('width', w - 2); r.setAttribute('height', h - 2); }
  el.hidden = false;
}

// Shift held over an edge (or a corner) of the frame: the edge is lit, the pointer says "resize", and Shift and drag moves the edge.
const EDGE_GRAB = 6; // screen pixels

// Which edges of the frame are within reach of a position of the window: {t, b, l, r} (corners have two), or null.
function edgeAt(clientX, clientY, inside = false) {
  const box = selectionBox();
  if (!box) return null;
  const r = $('stage').getBoundingClientRect(), ox = r.left + view.x + view.ox, oy = r.top + view.y + view.oy;
  const l = ox + box.x0 * view.sx, rt = ox + box.x1 * view.sx, t = oy + box.y0 * view.sy, bt = oy + box.y1 * view.sy;
  const near = (v, a, b) => Math.abs(v - a) <= EDGE_GRAB && Math.abs(v - a) <= Math.abs(v - b); // the closer of two edges that are near
  const inX = clientX >= l - EDGE_GRAB && clientX <= rt + EDGE_GRAB, inY = clientY >= t - EDGE_GRAB && clientY <= bt + EDGE_GRAB;
  const hit = {
    l: inY && near(clientX, l, rt), r: inY && near(clientX, rt, l) && !(Math.abs(clientX - l) === Math.abs(clientX - rt)),
    t: inX && near(clientY, t, bt), b: inX && near(clientY, bt, t) && !(Math.abs(clientY - t) === Math.abs(clientY - bt)),
  };
  if (hit.l || hit.r || hit.t || hit.b) return hit;
  // inside the frame, away from the edges: where Paint moves the part (without Shift)
  return inside && clientX > l && clientX < rt && clientY > t && clientY < bt ? {move: true} : null;
}

const RIGHT_TOOLS = ['pen', 'line', 'rect', 'ellipse', 'fill', 'pick']; // the right button works in these (the second color)
const RIGHT_LASSO = ['select', 'poly']; // in these the right button, held, draws a free-hand polygon select
const TOOL_CURSOR = {pen: 'crosshair', line: 'crosshair', rect: 'crosshair', ellipse: 'crosshair', fill: 'crosshair', pick: 'crosshair', text: 'text'};
const RESIZE_CURSORS = {t: 'ns-resize', b: 'ns-resize', l: 'ew-resize', r: 'ew-resize', tl: 'nwse-resize', br: 'nwse-resize', tr: 'nesw-resize', bl: 'nesw-resize'};

// Lights the edge under the pointer, in two ways: with Shift ("content": the selected part is stretched by its edges and moved by its inside; see below)
// and without ("frame": the frame is moved and resized, the picture is not touched). It does not depend on edit mode.
function updateHover(shift) {
  if (view.selDrag?.edge || view.tdrag || view.drag) return;
  if (TOOL_CURSOR[tool()]) { // the cursor of a drawing tool is for the picture; beside it the pointer is an ordinary one (while a stroke is drawn it stays)
    view.hl = null; view.hlMode = '';
    $('stage').style.cursor = ink || shape || (lastPointer && pixelUnder(lastPointer.x, lastPointer.y)) ? TOOL_CURSOR[tool()] : '';
    return;
  }
  const hit = lastPointer && !view.drag ? edgeAt(lastPointer.x, lastPointer.y, true) : null;
  const was = JSON.stringify([view.hl, view.hlMode]);
  view.hl = hit;
  view.hlMode = hit ? (shift || view.fl ? 'content' : 'frame') : ''; // Shift: the selected part itself is moved or stretched; without it the frame
  const key = hit ? (hit.t ? 't' : hit.b ? 'b' : '') + (hit.l ? 'l' : hit.r ? 'r' : '') : '';
  $('stage').style.cursor = hit?.move || (view.hlMode === 'content' && key === 'tl') ? 'move' : key ? RESIZE_CURSORS[key] : '';
  if (was !== JSON.stringify([view.hl, view.hlMode])) renderSelection();
}

function clearSelection() {
  rotSession = null;
  stopSelectionFlash();
  if (view.fl) { flushFloat({clear: true}); return; } // a lifted part is applied: the selection is let go
  if (!view.sel) return;
  view.sel = null; view.selDrag = null; view.hl = null; view.hlMode = ''; $('stage').style.cursor = '';
  draw();
}

// ---- Move and stretch the selected part, like Paint ----------------------------------------------------------
// With Shift (or while the part floats: a pasted picture, or one that was lifted), the edges of the selection stretch the part, and its inside (and its top left corner) moves it. The first time, the part is lifted: the
// place it came from is white, and the part is shown where it is dropped (#float, over the picture, which is not changed yet; #hole is the
// white). It is worked out from the part as it was lifted, never from the result of the last drag, so stretching and going back loses
// nothing. It stays like that, and can be dragged again, until the selection is let go (Esc, a click, a new selection, a picture that
// changes): then it is applied and the picture is modified. Every drop is a step of the undo history, and so is the applying; undoing the
// applying brings the lifted part back, selected, and undoing the first drag selects the frame it started from.
// view.fl = {src: {x, y, w, h} where it came from, dest: {x, y, w, h} where it is, url: its pixels as a PNG}, in the picture as shown.

const rectSel = (r) => ({x0: r.x, y0: r.y, x1: r.x + r.w, y1: r.y + r.h});
const sameRect = (a, b) => a.x === b.x && a.y === b.y && a.w === b.w && a.h === b.h;
const floatChanges = (f) => !!f && (f.pasted || !sameRect(f.src, f.dest)); // a pasted layer is always a change; a lifted part is one once it is moved

function renderFloat() {
  const f = view.fl, el = $('float'), hole = $('hole');
  if (!f || !pic.naturalWidth) { el.hidden = true; hole.hidden = true; return; }
  const ox = view.x + view.ox, oy = view.y + view.oy;
  const place = (e, r) => { e.style.left = `${ox + r.x * view.sx}px`; e.style.top = `${oy + r.y * view.sy}px`; e.style.width = `${r.w * view.sx}px`; e.style.height = `${r.h * view.sy}px`; };
  place(hole, f.src); hole.hidden = !!f.pasted; // (a pasted picture leaves no hole)
  // what it leaves is white, or transparent where the type can be: the squares of the window, in step with them
  hole.style.background = $('stage').classList.contains('checker') ? `${CHECKER} ${-parseFloat(hole.style.left)}px ${-parseFloat(hole.style.top)}px / 16px 16px` : '#fff';
  el.classList.toggle('pixelated', view.pixelated);
  place(el, f.dest); el.hidden = !f.url;
}

// The lifted pixels, for #float.
async function loadFloatPixels(f) {
  const b64 = await cutPng(f.src);
  if (view.fl !== f) return; // let go or changed meanwhile
  f.url = `data:image/png;base64,${b64}`;
  for (const en of history.list) if (en.fl && !en.fl.url && en.state === f.state && en.rot === f.rot && sameRect(en.fl.src, f.src)) en.fl.url = f.url; // the history keeps the pixels, so that undo can show them at once
  $('float').onload = () => renderFloat();
  $('float').src = f.url;
}

// A picture with transparent parts gets the squares as the background of the window (worked out after it is shown: Go decodes the file).
async function checkTransparency(token) {
  let has = false;
  try { has = await HasTransparency(); } catch {}
  if (token === view.token) $('stage').classList.toggle('checker', has);
}

function dropFloat() {
  view.fl = null; view.tdrag = null;
  renderFloat();
  ClearFloat().catch(() => {});
}

// Tells Go where the part is, so that it can apply it whenever the picture is saved, pasted into or closed.
function sendFloat() {
  const f = view.fl;
  if (f.pasted) return MovePasted(f.dest.x, f.dest.y, f.dest.w, f.dest.h, !view.pixelated);
  return SetFloat(view.rot, f.src.x, f.src.y, f.src.w, f.src.h, f.dest.x, f.dest.y, f.dest.w, f.dest.h, !view.pixelated);
}

let floatBusy = false;

// Applies the lifted part: the picture becomes the one with it dropped (a new step); opts.clear lets the selection go too.
async function flushFloat(opts = {}) {
  const f = view.fl;
  if (!f || floatBusy) return;
  floatBusy = true;
  try {
    const dest = f.dest;
    if (floatChanges(f)) {
      await sendFloat(); // (the pixelated view may have been switched since: stretched in whole pixels or smoothed)
      const info = await BakeFloat();
      await showImage(info, {keepView: true, rot: 0});
      if (opts.clear) { // the drops and the applying are one step: undoing it selects the frame again, redoing it moves the part and keeps it selected
        history.list.length = history.at + 1;
        while (history.at > 0 && history.list[history.at].fl) { history.list.pop(); history.at--; }
      }
      commit();
      if (opts.clear) { const e = history.list[history.at]; if (e) e.sel = {...dest}; }
      if (!opts.clear) view.sel = rectSel(dest);
    } else {
      dropFloat();
      if (opts.clear) view.sel = null;
    }
    draw();
  } catch (e) { problem(errText(e)); }
  floatBusy = false;
}

function startTransform(ev, edge) {
  const r = selectionRect();
  if (!r) return;
  const first = !view.fl;
  if (first) { view.fl = {src: {...r}, dest: {...r}, url: '', state: view.state, rot: view.rot}; loadFloatPixels(view.fl).catch((e) => problem(errText(e))); }
  const [px, py] = pointInPicture(ev.clientX, ev.clientY);
  view.tdrag = {edge, px, py, start: {...view.fl.dest}, first};
  view.sel = rectSel(view.fl.dest); view.hl = edge; view.hlMode = 'content';
  draw();
}

function dragTransform(ev) {
  const t = view.tdrag, s = t.start, W = dispW(), H = dispH();
  const [px, py] = pointInPicture(ev.clientX, ev.clientY);
  const dx = Math.round(px - t.px), dy = Math.round(py - t.py);
  let x0 = s.x, y0 = s.y, x1 = s.x + s.w, y1 = s.y + s.h;
  if (t.edge.move || (t.edge.t && t.edge.l)) { // the inside and the top left corner move it
    x0 = clamp(s.x + dx, 0, Math.max(0, W - s.w)); y0 = clamp(s.y + dy, 0, Math.max(0, H - s.h)); x1 = x0 + s.w; y1 = y0 + s.h;
  } else { // the others stretch it (the other edges stay where they are)
    if (t.edge.l) x0 = clamp(s.x + dx, 0, x1 - 1);
    if (t.edge.r) x1 = clamp(s.x + s.w + dx, x0 + 1, W);
    if (t.edge.t) y0 = clamp(s.y + dy, 0, y1 - 1);
    if (t.edge.b) y1 = clamp(s.y + s.h + dy, y0 + 1, H);
  }
  view.fl.dest = {x: x0, y: y0, w: x1 - x0, h: y1 - y0};
  view.sel = rectSel(view.fl.dest);
  draw();
}

async function finishTransform() {
  const t = view.tdrag, f = view.fl;
  view.tdrag = null;
  if (!f) return;
  if (sameRect(f.dest, t.start)) { // not moved
    if (t.first) dropFloat();
    draw();
    return;
  }
  try { await sendFloat(); } catch (e) { problem(errText(e)); }
  if (t.first) { const prev = history.list[history.at]; if (prev) prev.undoSel = {...f.src}; } // undo (not redo) selects the frame it started from again
  commit();
}

// A position of the window as a position in the picture (in pixels, fractions included), kept inside the picture.
function pointInPicture(clientX, clientY) {
  const r = $('stage').getBoundingClientRect();
  return [clamp((clientX - r.left - view.x - view.ox) / view.sx, 0, dispW()), clamp((clientY - r.top - view.y - view.oy) / view.sy, 0, dispH())];
}

function base64ToBlob(b64, type) {
  const bin = atob(b64), bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], {type});
}

const blobToBase64 = (blob) => new Promise((resolve, reject) => {
  const fr = new FileReader();
  fr.onload = () => resolve(String(fr.result).split(',')[1]);
  fr.onerror = () => reject(fr.error);
  fr.readAsDataURL(blob);
});

// The part r {x, y, w, h} of the picture as it is shown, as a PNG in base64 (cut out of the decoded file; see SelectionPNG).
async function cutPng(r) {
  const nw = pic.naturalWidth, nh = pic.naturalHeight;
  const n = toNaturalRect(r); // the same part in the file's picture, which is cut out and then turned the way it is shown
  await preparePixels();
  let b64 = '';
  if (pixelState === 'go') b64 = await SelectionPNG(n.x, n.y, n.w, n.h, nw, nh);
  if (!b64) { // a type Go cannot decode: cut it out of what the web view drew
    if (!pixelCanvas) throw new Error('this image cannot be copied');
    const c = document.createElement('canvas'); c.width = n.w; c.height = n.h;
    c.getContext('2d').putImageData(pixelCanvas.getImageData(n.x, n.y, n.w, n.h), 0, 0);
    b64 = await blobToBase64(await new Promise((res) => c.toBlob(res, 'image/png')));
  }
  return view.rot ? RotatePNG(b64, view.rot) : b64;
}

let lastCopy = 0;

// A copy lights things up for a moment: the border of the selection (orange and white, with an orange clipboard in the middle), and an orange
// clipboard at the end of the folder label of the status bar (for as long as the one in the middle).
let flashTimer = 0, clipFlagTimer = 0;
function flashSelection() {
  const el = $('sel');
  el.classList.remove('flash'); void el.offsetWidth; el.classList.add('flash'); // (taken off and put on again, so that a second copy starts it again)
  clearTimeout(flashTimer); flashTimer = setTimeout(() => el.classList.remove('flash'), 1700);
}
// A selection that is let go, or a new one begun, ends the flash of the old one: #sel is the same element, and would show it again.
function stopSelectionFlash() {
  clearTimeout(flashTimer); $('sel').classList.remove('flash');
}
function flashClipboard() {
  const el = $('clipflag');
  el.classList.remove('on'); void el.offsetWidth; el.classList.add('on');
  document.body.classList.add('peek'); // (the bar is there for the flag, also when there is no selection)
  clearTimeout(clipFlagTimer); clipFlagTimer = setTimeout(() => { el.classList.remove('on'); document.body.classList.remove('peek'); }, 1700);
}

async function copySelection(auto = false) {
  if (!pic.naturalWidth) return;
  // a click inside or on the edge of a selection that is on the clipboard already, and that left it as it was to the pixel: nothing to copy again
  // (Ctrl+C and Copy still copy; checked before the guard below, so that a Ctrl+C right after is not swallowed)
  if (auto && !view.fl && copiedKey !== null && copiedKey === selKey()) return;
  const now = performance.now();
  if (now - lastCopy < 400) return; // the key and the system's copy command can both come for one press
  lastCopy = now;
  if (view.fl) await flushFloat(); // what was lifted is applied first, so that the copy is what is seen
  const r = selectionRect() || {x: 0, y: 0, w: dispW(), h: dispH()};
  try {
    const b64 = await cutPng(r);
    try { // the web view's own clipboard, and the system's way when it does not take the picture
      await navigator.clipboard.write([new ClipboardItem({'image/png': base64ToBlob(b64, 'image/png')})]);
    } catch { await CopyImageNative(b64); }
    copiedKey = selKey();
    renderSelection();
    flashClipboard();
    if (view.sel) flashSelection();
  } catch (e) { problem(errText(e)); }
}

// On a Mac the wheel events of the trackpad (two fingers up and down) move the picture, and those of a mouse wheel zoom it; a pinch
// (which the web view reports as a wheel event with Ctrl) zooms. The two are told apart by their size: a mouse wheel moves in whole
// lines, which come as multiples of 120 in wheelDelta, and the trackpad in small steps of any size. A trackpad step that happens to be
// a multiple of 120 stays a move while the trackpad is still going. Other systems: the wheel zooms, as before.
const MAC = navigator.platform.startsWith('Mac');
let lastTrackpad = 0;

function isWheelNotch(ev) {
  return ev.deltaMode !== 0 || ((ev.wheelDeltaY % 120 === 0) && (ev.wheelDeltaX % 120 === 0) && (ev.wheelDeltaY !== 0 || ev.wheelDeltaX !== 0));
}

function panBy(dx, dy) {
  const bd = bounds(), now = performance.now();
  if (now - bounce.lastWheel > 120) { // a new swipe
    bounce.prev.x = bounce.prev.y = 0; bounce.dec.x = bounce.dec.y = 0; bounce.swipeOver = false;
  }
  bounce.lastWheel = now;
  let moved = false;
  for (const ax of ['x', 'y']) {
    const o = ax === 'x' ? 'ox' : 'oy', [lo, hi] = bd[ax];
    let d = -(ax === 'x' ? dx : dy); // how far this event wants the picture to go
    const mag = Math.abs(d);
    bounce.dec[ax] = mag <= bounce.prev[ax] ? bounce.dec[ax] + 1 : 0; // how many events in a row were no bigger than the one before
    bounce.prev[ax] = mag;
    if (view[o] !== 0 && d !== 0 && Math.sign(d) === -Math.sign(view[o])) { // going back first takes the pull away
      const use = Math.min(mag, Math.abs(view[o]));
      view[o] += Math.sign(d) * use; d -= Math.sign(d) * use;
    }
    const target = view[ax] + d, at = clamp(target, lo, hi), excess = target - at; // what goes past the edge is a push
    if (at !== view[ax]) { view[ax] = at; moved = true; }
    // the trackpad running out (events that only get smaller): once this swipe has pulled the picture out they do not push it again,
    // not even when the spring has taken the pull away already; coasting that first reaches the edge on its own does bounce, once
    const coasting = bounce.dec[ax] >= 2 && (view[o] !== 0 || bounce.swipeOver);
    if (Math.abs(excess) >= 0.5 && !coasting) {
      view[o] = clamp(view[o] + Math.sign(excess) * Math.abs(excess) * (1 - Math.abs(view[o]) / BOUNCE_MAX) ** 2, -BOUNCE_MAX, BOUNCE_MAX);
      bounce.over = true; bounce.swipeOver = true; bounce.lastPush = now;
    }
  }
  if (moved) view.free = true;
  draw();
  startSpring();
}

function onWheel(ev) {
  ev.preventDefault();
  if (!pic.naturalWidth) return;
  if (ev.shiftKey && gif.meta) { gifWheel(ev); return; } // Shift+wheel steps through the frames of an animated GIF
  const r = $('stage').getBoundingClientRect(), now = performance.now();
  const zoom = () => zoomAt(Math.exp(-ev.deltaY * (ev.ctrlKey ? 0.01 : 0.0025)), ev.clientX - r.left, ev.clientY - r.top);
  if (ev.ctrlKey || !MAC) { zoom(); return; }
  if (!isWheelNotch(ev)) lastTrackpad = now;
  if (isWheelNotch(ev) && now - lastTrackpad > 120) zoom(); else panBy(ev.deltaX, ev.deltaY);
}

// Paste does not depend on edit mode: a small clipboard picture is laid over the picture, a big one is asked about (see below), a window without a picture takes it.
// Ctrl+V (Cmd+V, Shift+Insert, Edit → Paste): the picture on the clipboard becomes the picture of the window. It has no file yet:
// the title says [clipboard image], and S saves it.
// Paste (Ctrl+V, Cmd+V, Edit → Paste): the picture on the clipboard is laid over the picture, near the top left corner of what is visible, as a part that
// floats and is selected, like a lifted part: it can be dragged and stretched, Delete, Backspace or Esc take it away again, and a click beside it lays
// it down. A part that was lifted and moved is applied first. A clipboard picture that is big (about as big as the picture) is probably a whole new
// image: Go asks whether it becomes the picture of the window or is laid over this one, and then the window shows it by itself. A window without a picture takes it as its picture.
async function pasteImage() {
  try {
    if (view.fl) await flushFloat({clear: false});
    let px = 0, py = 0;
    if (pic.naturalWidth) { const r = $('stage').getBoundingClientRect(); [px, py] = pointInPicture(r.left + 12, r.top + 12); }
    const L = await PasteLayer(view.rot, Math.ceil(px), Math.ceil(py)); // (rounded up: the first pixel that is seen whole, 12 screen pixels in from the corner)
    if (!L.w) return;
    const f = {src: {x: 0, y: 0, w: L.w, h: L.h}, dest: {x: L.x, y: L.y, w: L.w, h: L.h}, url: `data:image/png;base64,${L.png}`, pasted: true, state: view.state, rot: view.rot};
    view.fl = f;
    $('float').onload = () => renderFloat();
    $('float').src = f.url;
    view.sel = rectSel(f.dest);
    draw();
    commit('pasted part placed');
  } catch (e) { problem(errText(e)); }
}

// Delete, Backspace and Esc take a pasted picture away again (a step of the history, like laying it).
function removeLayer() {
  dropFloat();
  view.sel = null; view.selDrag = null; view.hl = null; view.hlMode = ''; $('stage').style.cursor = '';
  draw();
  commit('pasted part removed');
}

// File → Close Image: the window goes back to the way it starts, with no picture. It is a step of the history like any other, so undo brings
// the picture back with its changes, but a picture that has some is asked about first (Save, Discard, Cancel), as when the window is closed.
async function closeImage() {
  if (!pic.naturalWidth) return;
  await commitText();
  try { if (!await DiscardOK('close the image')) return; } catch (e) { problem(errText(e)); return; }
  if (view.fl) await flushFloat({clear: true}); // what is lifted is part of the picture that comes back
  if (!view.state || history.busy) return;
  try {
    const info = await RestoreState(0);
    await showImage(info);
    commit('image closed'); offerReopen();
  } catch (e) { problem(errText(e)); }
}

// Ctrl+W, Cmd+W (File → Close): the picture is closed; the window, when there is none.
let lastEsc = 0;
function closeOrQuit() { if (pic.naturalWidth) closeImage(); else CloseWindow(); }

// S (Ctrl+S, Cmd+S, File → Save As…): asks where to save the picture. Saving it to another folder makes that folder the one the
// window works in, and the arrow keys go through its pictures.
// Create Image (File menu, and the empty window): a new white 1280 × 720 image, and edit mode starts with it.
let editAfterImage = false;
// Shift-click on Create image (or Shift+N) asks for the size and the background first; the plain click and N make the white 1280 × 720.
async function createImage(custom = false) {
  try {
    let w = 1280, h = 720, rgba = '#FFFFFFFF';
    if (custom === true) { const spec = await askNewImage(); if (!spec) return; ({w, h, rgba} = spec); }
    if (!await DiscardOK('create a new image')) return;
    editAfterImage = true;
    if (custom === true) await NewBlankColor(w, h, rgba); else await NewBlank(w, h, false);
  } catch (e) { editAfterImage = false; problem(errText(e)); }
}

// The dialog: width, height and the background (white, transparent or a color). It answers {w, h, rgba}, or null when it is cancelled.
// What was chosen last time is offered again.
function askNewImage() {
  const dlg = $('newdlg'), form = $('newdlg-form'), W = $('nd-w'), H = $('nd-h'), bg = $('nd-bg'), col = $('nd-color'), err = $('nd-err');
  try { const last = JSON.parse(localStorage.getItem('newImage') || 'null'); if (last) { W.value = last.w; H.value = last.h; bg.value = last.bg; col.value = last.color; } } catch {}
  const showColor = () => { col.hidden = bg.value !== 'color'; };
  showColor(); err.hidden = true; dlg.hidden = false; W.focus(); W.select();
  return new Promise((resolve) => {
    const close = (v) => { dlg.hidden = true; bg.onchange = null; form.onsubmit = null; dlg.onkeydown = null; $('nd-cancel').onclick = null; dlg.onmousedown = null; resolve(v); };
    bg.onchange = showColor;
    form.onsubmit = (ev) => {
      ev.preventDefault();
      const w = Math.round(+W.value), h = Math.round(+H.value);
      if (!(w >= 1 && h >= 1)) { err.textContent = 'Width and height are 1 pixel or more.'; err.hidden = false; return; }
      if (w * h > 150_000_000) { err.textContent = 'That is too big: up to 150 million pixels (for example 12000 × 12000).'; err.hidden = false; return; }
      try { localStorage.setItem('newImage', JSON.stringify({w, h, bg: bg.value, color: col.value})); } catch {}
      close({w, h, rgba: bg.value === 'transparent' ? '#00000000' : bg.value === 'color' ? col.value.toUpperCase() + 'FF' : '#FFFFFFFF'});
    };
    $('nd-cancel').onclick = () => close(null);
    dlg.onmousedown = (ev) => { if (ev.target === dlg) close(null); };
    dlg.onkeydown = (ev) => { if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(null); } };
  });
}

// D: the picture is deleted (the system asks first); the next picture of the folder is shown in its place, with the folder's new count.
// Discard changes: the picture goes back to its original (the file on the disk, or the pasted or new picture) as one more step of the history, so undo brings the changes
// back and redo discards them again.
async function discardChanges() {
  try {
    await commitText();
    if (view.fl) await flushFloat({clear: true});
    const was = view.state;
    const info = await DiscardChanges();
    if (info.state === was) { problem('There are no changes to discard', false, 2000); return; }
    await showImage(info);
    commit('changes discarded');
    problem('Changes discarded (undo brings them back)', false, 2500);
  } catch (e) { problem(errText(e)); }
}

async function deleteImage() {
  try { const note = await DeleteImage(); if (note) problem(note, false, 2000); } catch (e) { problem(errText(e)); }
}

async function saveAs() {
  await commitText(); // a text that is being typed is part of what is saved
  if (gif.playing) await gifSetPlaying(false); // an animation is stopped on a frame: Save As then offers the animation or the frame
  try {
    if (view.fl) await flushFloat(); // what was lifted is part of what is saved
    const r = selectionRect(); // with a selection only that part is saved, as a picture of its own; the window keeps its picture
    const note = r ? await SaveSelectionAs(view.rot, r.x, r.y, r.w, r.h) : await SaveAs(view.rot); if (note) problem(note, false); } catch (e) { problem(errText(e)); }
}

// When the window changes its size (full screen with Enter, and back, or the window dragged bigger), a picture that was zoomed or
// moved keeps the pixel that was in the middle of the window in the middle. The pixel is remembered, because a picture near its edge
// cannot be held in the middle while the window is small (it is pushed back to the edge) but should be again when the window is big
// again; the memory is only good while the picture has not been moved or zoomed since. A picture a mode places is fitted again.
let lastStage = null, anchor = null;

// Full screen has no bar at the bottom; Shift shows it for as long as it is held (and a message for a few seconds).
// On Windows the menu bar is hidden in full screen and comes back while the pointer is at the top edge.
let menuShown = true;
function showMenuBar(on) { if (on !== menuShown) { menuShown = on; SetMenuBar(on).catch(() => {}); } }
async function refreshFullscreen() {
  try {
    const full = await IsFullscreen();
    document.body.classList.toggle('full', full);
    showMenuBar(!full);
    if (pic.naturalWidth) draw(); // the bar says the size and zoom in full screen only
  } catch {}
}
window.addEventListener('mousemove', (ev) => {
  if (!document.body.classList.contains('full')) return;
  document.body.classList.toggle('nearbottom', !editOn() && ev.clientY >= window.innerHeight - 44); // the bar comes while the pointer is at the bottom (edit mode has the bar already)
  if (ev.clientY <= 1) showMenuBar(true); else if (ev.clientY > 40) showMenuBar(false);
});

document.documentElement.addEventListener('mouseleave', () => { document.body.classList.remove('nearbottom'); capsuleDodge(null); });

// The status capsule gets out of the way of the pointer, in any mode it shows in: when the pointer comes over it at the bottom, it fades over to the
// top of the window, and back as soon as the pointer leaves the place where it was at the bottom (its height there, and its width as it is now).
// Not in full screen outside edit mode, where the pointer at the bottom is what calls the bar up.
let capsuleUp = false, capsuleLow = null, capsuleTimer = 0;
function capsuleDodge(ev) {
  const s = $('status'), r = s.getBoundingClientRect(), pad = 6, b = document.body.classList;
  const shown = !s.hidden && (capsuleUp || getComputedStyle(s).opacity > 0.5) && !(b.contains('full') && !editOn());
  if (!ev || !shown) { if (capsuleUp) capsuleMove(false); return; }
  const overX = ev.clientX >= r.left - pad && ev.clientX <= r.right + pad;
  if (!capsuleUp) { if (overX && ev.clientY >= r.top - pad && ev.clientY <= r.bottom + pad) { capsuleLow = {top: r.top, bottom: r.bottom}; capsuleMove(true); } }
  else if (!(overX && ev.clientY >= capsuleLow.top - pad && ev.clientY <= capsuleLow.bottom + pad)) capsuleMove(false);
}
function capsuleMove(up) { // a quick crossfade: out, over, in
  capsuleUp = up;
  const s = $('status');
  s.classList.add('swap');
  clearTimeout(capsuleTimer);
  capsuleTimer = setTimeout(() => { document.body.classList.toggle('captop', up); s.classList.remove('swap'); }, 90);
}
window.addEventListener('pointermove', capsuleDodge, true); // (pointer events: the mouse ones stop while a drag holds the pointer, as when a selection is drawn)

// The system takes a moment to get in and out of full screen; ask again until it is done.
function settleFullscreen() { for (const ms of [100, 400, 1000, 1800]) setTimeout(refreshFullscreen, ms); }

function onResize() {
  refreshFullscreen();
  const b = stageSize();
  if (view.free && pic.naturalWidth && lastStage && b.w > 0 && b.h > 0) {
    const fresh = !anchor || anchor.x !== view.x || anchor.y !== view.y || anchor.sx !== view.sx || anchor.sy !== view.sy || anchor.rot !== view.rot;
    const a = fresh ? {X: (lastStage.w / 2 - view.x) / view.sx, Y: (lastStage.h / 2 - view.y) / view.sy} : anchor;
    view.x = b.w / 2 - a.X * view.sx; view.y = b.h / 2 - a.Y * view.sy;
    draw(); // keeps it inside the window (changes view.x and view.y)
    anchor = {X: a.X, Y: a.Y, x: view.x, y: view.y, sx: view.sx, sy: view.sy, rot: view.rot};
  } else {
    draw(); anchor = null;
  }
  lastStage = b;
}

// ---- Undo and redo ---------------------------------------------------------------------------------------
// Every change of what the window shows is a step that can be undone with Ctrl+Z (Cmd+Z) and done again with Ctrl+Shift+Z (Cmd+Shift+Z,
// Ctrl+Y): another picture (Open, the arrow keys, paste, save to another folder, a file opened with mormiView), the mode (1 to 6, F), the
// pixelated view (Tab) and the turn (R). Not steps: how far it is zoomed, where it is moved, the selection.
// The history is a list of whole states, {picture, mode, pixelated, turn}, and an undo shows the one before. The picture is a number
// that Go knows (ImageInfo.state; 0 is no picture at all), because a picture pasted from the clipboard exists only there.
const HISTORY_MAX = 100;
const history = {list: [], at: -1, busy: false};
let lastUndo = 0;

const snapshot = () => ({state: view.state, mode: view.mode, pixelated: view.pixelated, rot: view.rot, pic: `${view.dir}|${view.name}`, view: {free: view.free, sx: view.sx, sy: view.sy, x: view.x, y: view.y}, fl: view.fl ? {src: {...view.fl.src}, dest: {...view.fl.dest}, url: view.fl.url, pasted: !!view.fl.pasted} : null});
const flKey = (f) => (f ? JSON.stringify([f.src, f.dest]) : '');
const sameSnapshot = (a, b) => a.state === b.state && a.mode === b.mode && a.pixelated === b.pixelated && a.rot === b.rot && flKey(a.fl) === flKey(b.fl);

// Who makes the steps (the chat's Claude, through the control commands, or the user) and what each is called: the history tool tells Claude both.
let actor = 'user';
const checkpoints = new Map(); // name → the step of the history (the object itself, so that it is found again when the list changes)
function autoLabel(prev, s) {
  if (!prev) return 'start';
  if (s.state !== prev.state) return 'image changed';
  if (s.rot !== prev.rot) return 'turned';
  if (s.mode !== prev.mode) return `view mode: ${s.mode}`;
  if (s.pixelated !== prev.pixelated) return 'pixelated view toggled';
  return s.fl ? 'part moved or stretched' : 'change';
}

// Makes the state the window is in now a step of the history (and ends what could be redone).
function commit(label) {
  if (history.busy) return;
  const s = snapshot();
  if (history.at >= 0 && sameSnapshot(history.list[history.at], s)) return;
  s.by = actor; s.label = label || autoLabel(history.list[history.at], s);
  history.list.length = history.at + 1;
  history.list.push(s);
  if (history.list.length > HISTORY_MAX) history.list.shift();
  history.at = history.list.length - 1;
  KeepStates([...new Set(history.list.map((e) => e.state))]).catch(() => {}); // Go keeps no other picture
}

function turnTo(rot) {
  const d = (rot - view.rot + 4) % 4;
  if (d === 1 || d === 3) rotate(d === 1 ? 1 : -1); else if (d === 2) { rotate(1); rotate(1); }
}

// Shows the state at place `to` of the history.
async function travel(to) {
  if (history.busy || to < 0 || to >= history.list.length) return;
  history.busy = true;
  const e = history.list[to];
  const hi = history.list[Math.max(to, history.at)], lo = history.list[Math.min(to, history.at)]; // the step that is undone or done again, and the one next to it
  const hadFloat = !!view.fl;
  if (hadFloat) { view.fl = null; view.tdrag = null; } // (what is shown of it stays on the screen until the picture that follows is there)
  const sel = e.fl ? e.fl.dest : e.sel || (e.undoSel && to < history.at ? e.undoSel : null);
  const fl = e.fl ? {src: {...e.fl.src}, dest: {...e.fl.dest}, url: e.fl.url || '', state: e.state, rot: e.rot, pasted: !!e.fl.pasted} : null;
  try {
    if (e.state !== view.state) { // another picture (or none): Go brings it back, the view is set as it was
      // Go brings the picture back with its lifted part in one call, so that the circle of the title does not go out for a moment
      const info = fl?.pasted ? await RestoreLayer(e.state, e.rot, fl.url.split(',')[1], fl.dest.x, fl.dest.y, fl.dest.w, fl.dest.h, !editOn()) : fl ? await RestoreFloat(e.state, e.rot, fl.src.x, fl.src.y, fl.src.w, fl.src.h, fl.dest.x, fl.dest.y, fl.dest.w, fl.dest.h, !editOn()) : await RestoreState(e.state);
      const same = view.state !== 0 && info?.url && info.name === view.name && info.dir === view.dir && e.mode === view.mode; // the same picture, with a change undone or redone: the zoom stays
      view.mode = e.mode; if (!same) view.free = false; view.pixelated = editOn(); pic.classList.toggle('pixelated', editOn());
      if (fl?.url) { const fe = $('float'); fe.src = fl.url; await fe.decode().catch(() => {}); } // the lifted part is ready before the picture changes
      await showImage(info, {rot: e.rot, keepView: !!same, fl: fl?.url ? fl : null, sel});
    } else {
      if (hadFloat && !e.fl) ClearFloat().catch(() => {});
      if (e.mode !== view.mode) setMode(e.mode);
      if (editOn() !== view.pixelated) setPixelated(editOn());
      turnTo(e.rot);
    }
    if (hi.view && hi.pic === lo.pic && hi.rot === lo.rot && view.state) { // the view (zoom and which part is visible) the step was made in
      Object.assign(view, hi.view); draw();
    }
    if (e.fl) { // the lifted part is back where it was dropped, and selected
      if (!view.fl) { view.fl = fl; if (fl.pasted) { $('float').onload = () => renderFloat(); $('float').src = fl.url; } else loadFloatPixels(view.fl).catch(() => {}); } // (without its pixels in the history they are cut out again)
      view.sel = rectSel(e.fl.dest);
      if (fl.pasted) await SetPasted(e.rot, fl.url.split(',')[1], fl.dest.x, fl.dest.y, fl.dest.w, fl.dest.h, !editOn()); else await sendFloat();
    } else view.sel = sel ? rectSel(sel) : null; // the frame that was selected before it was lifted, or the part that was dropped and let go
    draw();
    history.at = to; offerReopen();
  } catch (err) { problem(errText(err)); }
  history.busy = false;
}

// The key and the menu item can both come for one press: that is one step.
function undoRedo(delta) {
  const now = performance.now();
  if (now - lastUndo < 120) return;
  lastUndo = now;
  if (delta < 0 && view.state && history.list[history.at + delta]?.state === 0) { problem('Nothing more to undo (closing the image is what Reopen last closed is for)', false, 2200); return; } // undo does not go back to the home screen: a picture that was opened, scanned or pasted there stays
  travel(history.at + delta);
}
const undo = () => undoRedo(-1), redo = () => undoRedo(1);

// Drag to select (with Shift on an edge of the frame, the frame is moved without touching the picture).
function beginSelection(clientX, clientY) {
  rotSession = null; // (a new frame: turning starts afresh)
  stopSelectionFlash();
  polyClear(); // (a new frame replaces a polygon)
  const edge = edgeAt(clientX, clientY, true); // without Shift the frame alone is moved and resized, in any mode
  const [qx, qy] = pointInPicture(clientX, clientY);
  if (edge) { // an edge of the frame: it is moved, not a new frame drawn
    const box = selectionBox();
    view.sel = {x0: box.x0, x1: box.x1, y0: box.y0, y1: box.y1}; // the first corner is the left top one, so l and t are x0 and y0
    view.selDrag = {edge, from: [clientX, clientY], p0: [qx, qy], start: {...view.sel}};
    view.hl = edge;
    draw();
    return;
  }
  const [x, y] = pointInPicture(clientX, clientY);
  view.selDrag = {x0: x, y0: y, from: [clientX, clientY]};
  view.sel = null; // the frame appears once the drag covers more than one pixel
  draw();
}

function startViewer() {
  initDock();
  initChat();
  $('tool-select').onclick = () => setTool('select');
  $('tool-poly').onclick = () => setTool('poly');
  $('tool-pen').onclick = () => setTool('pen');
  $('tool-line').onclick = () => setTool('line');
  $('tool-rect').onclick = () => setTool('rect');
  $('tool-ellipse').onclick = () => setTool('ellipse');
  $('tool-fill').onclick = () => setTool('fill');
  $('tool-text').onclick = () => setTool('text');
  $('tool-pick').onclick = () => setTool('pick');
  $('stage').addEventListener('contextmenu', (ev) => {
    if (RIGHT_TOOLS.includes(tool()) || RIGHT_LASSO.includes(tool())) ev.preventDefault(); // (no system menu: the right button draws)
    if (poly.down && poly.button === 2) return; // (the lasso that this press started)
    if (cancelDraw()) ev.preventDefault();
  });
  const cmd = navigator.platform.startsWith('Mac') ? '⌘' : 'Ctrl+';
  $('hint').textContent = WEB ? 'Open an image, paste one from the clipboard, or start a blank one. You can also drop images or a whole folder on this window; they stay in this browser.' : 'Open an image, paste one from the clipboard, or start a blank one. You can also open images with mormiView from your file manager.';
  $('k-clip').textContent = `${cmd}V`; $('k-reopen').textContent = `${cmd}Z`;
  $('reopen-pic').onclick = () => undo();
  $('open').onclick = choose;
  $('open-clip').onclick = () => PasteNew().catch((e) => problem(errText(e)));
  $('new-pic').onclick = (ev) => createImage(ev.shiftKey);
  $('scan-pic').onclick = () => ScanImage().catch((e) => problem(errText(e)));
  initTips();
  $('gif-add').onclick = () => { if (gif.meta) gifEdit({op: 'duplicate', source: gif.i, target: gif.i + 1, follow: true}, 'frame added'); };
  $('gif-del').onclick = () => { if (gif.meta && gif.meta.frames > 1) gifEdit({op: 'delete', frame: gif.i, follow: true}, 'frame deleted'); };
  $('gif-ms').onchange = () => { const v = Math.round(+$('gif-ms').value); if (gif.meta && v >= 20) gifEdit({op: 'delay', frame: -1, ms: v}, 'frame delay'); };
  $('gif-prev').onclick = () => gifStep(-1); $('gif-next').onclick = () => gifStep(1); $('gif-play').onclick = () => gifSetPlaying(!gif.playing);
  $('stage').addEventListener('pointerdown', () => { if (gif.playing) gifSetPlaying(false); }, true); // (a tool is used on the frame that is shown)
  $('discard-img').onclick = discardChanges;
  $('close-img').onclick = closeImage; // back to the home screen (which ends edit mode)
  EventsOn('menu-effect', (name) => applyEffectMenu(name)); // Effects menu
  EventsOn('menu-image', (name) => imageMenuAction(name)); // Image menu
  EventsOn('menu-create', () => createImage()); // File → Create Image
  window.addEventListener('focus', () => { if (!$('empty').hidden) offerClipboard(); });
  offerClipboard();
  EventsOn('history-reset', () => { history.list.length = 0; history.at = -1; }); // a deleted picture cannot be brought back by undo
  EventsOn('image', (info) => { const edit = editAfterImage; editAfterImage = false; showImage(info).then(() => { if (edit) setEdit(true); }); commit(`image shown: ${info.name}`); });
  const typing = () => { const el = document.activeElement; return !!el && (el.tagName === 'TEXTAREA' || el.tagName === 'INPUT' || el.isContentEditable); }; // the Edit menu then works on the text (of the picture, or of the chat box)
  EventsOn('menu-undo', () => typing() ? document.execCommand('undo') : undo());
  EventsOn('menu-redo', () => typing() ? document.execCommand('redo') : redo());
  EventsOn('problem', (t) => problem(errText(t)));
  EventsOn('note', (t) => problem(t, false));
  EventsOn('menu-select-all', () => { // Edit → Select All: the text that is being typed, else the whole picture
    if (typing()) { document.execCommand('selectAll'); return; }
    if (!pic.naturalWidth) return;
    view.sel = {x0: 0, y0: 0, x1: dispW(), y1: dispH()}; view.selDrag = null; draw();
  });
  EventsOn('menu-copy', () => typing() ? document.execCommand('copy') : copySelection()); // Edit → Copy
  EventsOn('menu-paste', () => typing() ? navigator.clipboard.readText().then((t) => document.execCommand('insertText', false, t)).catch(() => {}) : pasteImage()); // Edit → Paste
  EventsOn('menu-save', saveAs); // File → Save As…
  EventsOn('menu-close', closeOrQuit); // File → Close
  EventsOn('menu-close-image', closeImage); // File → Close Image
  EventsOn('menu-discard', discardChanges); // File → Discard Changes
  EventsOn('menu-delete', deleteImage); // File → Delete Image…
  lastStage = stageSize();
  window.addEventListener('resize', onResize);
  let saveTimer = 0; // the place and size of the window are remembered for the next start (Go also does it when the window closes)
  const rememberWindow = () => { clearTimeout(saveTimer); saveTimer = setTimeout(() => SaveWindow().catch(() => {}), 600); };
  window.addEventListener('resize', rememberWindow);
  window.addEventListener('blur', rememberWindow);
  $('stage').addEventListener('wheel', onWheel, {passive: false});
  $('stage').addEventListener('pointermove', (ev) => { lastPointer = {x: ev.clientX, y: ev.clientY}; readPixel(ev.clientX, ev.clientY); document.body.classList.toggle('shift', ev.shiftKey); if (!view.selDrag && !view.tdrag) updateHover(ev.shiftKey); });
  $('stage').addEventListener('pointerleave', () => { lastPointer = null; clearPixel(); updateHover(false); });
  window.addEventListener('keyup', (ev) => { if (ev.key === 'Shift') { document.body.classList.remove('shift'); updateHover(false); renderSelection(); } });
  window.addEventListener('blur', () => { document.body.classList.remove('shift'); updateHover(false); renderSelection(); });
  $('stage').addEventListener('pointerdown', (ev) => {
    if (ev.target === $('textbox')) return; // (the text box is typed in)
    if (cancelDraw()) return; // a press while a drawing is on (the other button, where the system reports it as a press) drops it
    if (!pic.naturalWidth || (ev.button !== 0 && ev.button !== 1 && !(ev.button === 2 && (RIGHT_TOOLS.includes(tool()) || RIGHT_LASSO.includes(tool()))))) return;
    try { $('stage').setPointerCapture(ev.pointerId); } catch {}
    if (ev.button === 1) { // the middle button pans the picture
      ev.preventDefault();
      stopBounce(); draw(); // a picture that was still coming back stops where it is, inside its bounds
      view.drag = {x: ev.clientX - view.x, y: ev.clientY - view.y, from: [ev.clientX, ev.clientY]};
      $('stage').style.cursor = 'grabbing';
      return;
    }
    pointerDown = true;
    const t = tool();
    if (ev.button === 2 && RIGHT_LASSO.includes(t)) { // the polygon starts only when the pointer moves; a click is the menu (of the selection, or of the whole picture)
      rightHold = {x: ev.clientX, y: ev.clientY, ev: {clientX: ev.clientX, clientY: ev.clientY, button: 2}};
      return;
    }
    if (ev.button === 2 && RIGHT_LASSO.includes(t)) { // the right button, held: a free-hand polygon (what the rectangle had is let go)
      (view.fl ? flushFloat({clear: true}) : Promise.resolve()).then(() => { if (pointerDown && !view.fl) { clearSelection(); polyDown(ev); } });
      return;
    }
    if (t === 'pen' || t === 'line' || t === 'rect' || t === 'ellipse') { // what is lifted is applied first, then the stroke starts (if the button is still down)
      (view.fl ? flushFloat({clear: true}) : Promise.resolve()).then(() => { if (pointerDown && !view.fl) (t === 'pen' ? startInk : startShape)(ev, t); });
      return;
    }
    if (t === 'poly') { (view.fl ? flushFloat({clear: true}) : Promise.resolve()).then(() => { if (pointerDown && !view.fl) polyDown(ev); }); return; }
    if (t === 'fill') { fillAt(ev); return; }
    if (t === 'pick') { pickColor(ev); return; }
    if (t === 'text') { startText(ev); return; }
    if (view.sel && (ev.shiftKey || view.fl)) { // a floating part (pasted, or lifted with Shift) moves and stretches with a plain drag; the picture itself only with Shift: an edge of the selection stretches the part, its inside moves it (see above)
      const edge = edgeAt(ev.clientX, ev.clientY, true);
      if (edge) { startTransform(ev, edge); return; }
    }
    if (view.fl) { // what is lifted is applied first; then it is a selection like any other, or, when the button is up by then, a click that lets go of it
      const [cx, cy] = [ev.clientX, ev.clientY];
      flushFloat({clear: false}).then(() => {
        if (view.fl) return;
        if (pointerDown) beginSelection(cx, cy); else clearSelection();
      });
      return;
    }
    beginSelection(ev.clientX, ev.clientY);
  });
  $('stage').addEventListener('pointermove', (ev) => {
    if (rightHold) {
      if (Math.hypot(ev.clientX - rightHold.x, ev.clientY - rightHold.y) < 5) return; // (still a click)
      const h = rightHold; rightHold = null;
      (view.fl ? flushFloat({clear: true}) : Promise.resolve()).then(() => { if (pointerDown && !view.fl) { clearSelection(); polyDown(h.ev); polyMove(ev); } });
      return;
    }
    if (poly.down && (poly.button === 2 ? ev.buttons & 1 : ev.buttons & 2)) { cancelDraw(); return; } // (the other button drops it)
    if (poly.down) { polyMove(ev); return; }
    if ((shape || ink) && otherButton(ev, shape || ink)) { cancelDraw(); return; } // (a second button comes as a move, not as a press)
    if (view.selDrag && !view.selDrag.edge && (ev.buttons & 2)) { cancelDraw(); return; } // (the same for the frame of a selection that is being drawn)
    if (shape) { moveShape(ev); return; }
    if (ink) { addInk(inkPoint(ev)); return; }
    if (view.tdrag) { dragTransform(ev); return; }
    if (view.selDrag?.edge) {
      const [px, py] = pointInPicture(ev.clientX, ev.clientY), e = view.selDrag.edge;
      if (e.move) { // the inside of the frame: the frame moves, the picture is not touched
        const d = view.selDrag, st = d.start, W = dispW(), H = dispH();
        const dx = clamp(Math.round(px - d.p0[0]), -st.x0, W - st.x1), dy = clamp(Math.round(py - d.p0[1]), -st.y0, H - st.y1);
        view.sel = {x0: st.x0 + dx, x1: st.x1 + dx, y0: st.y0 + dy, y1: st.y1 + dy};
        draw();
        return;
      }
      const snap = view.pixelated ? Math.round : (v) => v; // (an edge of the frame is between two pixels)
      if (e.l) view.sel.x0 = snap(px); if (e.r) view.sel.x1 = snap(px);
      if (e.t) view.sel.y0 = snap(py); if (e.b) view.sel.y1 = snap(py);
      draw();
      return;
    }
    if (view.selDrag) {
      const [px, py] = pointInPicture(ev.clientX, ev.clientY);
      view.sel = dragFrame(view.selDrag.x0, view.selDrag.y0, px, py);
      const r = selectionRect();
      if (!r || r.w * r.h < 2) view.sel = null; // one pixel is not a selection (also not when the drag comes back to it)
      draw();
      return;
    }
    if (!view.drag) return;
    const bd = bounds(), rx = ev.clientX - view.drag.x, ry = ev.clientY - view.drag.y; // where the pointer would put the picture
    const cx = clamp(rx, bd.x[0], bd.x[1]), cy = clamp(ry, bd.y[0], bd.y[1]);
    if (cx !== view.x || cy !== view.y) view.free = true;
    view.x = cx; view.y = cy;
    view.ox = Math.sign(rx - cx) * pullOffset(Math.abs(rx - cx)); view.oy = Math.sign(ry - cy) * pullOffset(Math.abs(ry - cy));
    if (view.ox || view.oy) { bounce.over = true; bounce.lastPush = performance.now(); }
    draw();
  });
  const endDrag = (ev) => {
    const rh = rightHold;
    pointerDown = false; rightHold = null;
    if (rh && ev.type === 'pointerup' && ev.button === 2 && Math.hypot(ev.clientX - rh.x, ev.clientY - rh.y) < 5) { showSelectionMenu(ev.clientX, ev.clientY); return; } // a right click on a selection (no drag): its menu
    if (poly.down) { polyUp(); return; }
    setTimeout(renderSelection, 0); // the button is up: the dashes move again
    if (shape) { const k = shape; shape = null; finishShape(k).then(() => updateHover(ev.shiftKey)); return; } // (see the pencil below)
    if (ink) { const k = ink; ink = null; finishInk(k).then(() => updateHover(ev.shiftKey)); return; } // the button is up: drawing stops now, not when Go is done
    if (view.tdrag) { finishTransform().then(() => updateHover(ev.shiftKey)); return; }
    if (view.selDrag) { // a selection dragged less than 3 screen pixels is a click, not a selection
      const [fx, fy] = view.selDrag.from, moved = !!view.selDrag.edge;
      view.selDrag = null;
      if (moved) { updateHover(ev.shiftKey); if (selectionRect()) copySelection(true); }
      else if (Math.abs(ev.clientX - fx) < 3 && Math.abs(ev.clientY - fy) < 3) { view.sel = null; draw(); }
      else if (selectionRect()) copySelection(true); // a finished selection goes to the clipboard at once
    }
    if (view.drag) { startSpring(); view.drag = null; updateHover(ev.shiftKey); } // let go: the spring takes the pull away
  };
  $('stage').addEventListener('mousedown', (ev) => { if (ev.button === 1) ev.preventDefault(); }); // no auto-scroll cursor
  $('stage').addEventListener('pointerup', endDrag);
  $('stage').addEventListener('pointercancel', endDrag);
  window.addEventListener('keydown', (ev) => {
    if (!$('newdlg').hidden || !$('rotdlg').hidden) return; // (the dialogs have their own keys)
    if (ev.key === 'Escape') ev.preventDefault(); // a handled key is not passed on to the system, which leaves full screen with it (see also DisableEscapeExitsFullscreen in main.go)
    if ((ev.target instanceof HTMLInputElement && ev.target.type !== 'range') || ev.target instanceof HTMLTextAreaElement || ev.target instanceof HTMLSelectElement) return; // typing in a field is not a command
    if (polyActive() && polyKey(ev)) { ev.preventDefault(); return; }
    if (ev.key === 'Shift') { document.body.classList.add('shift'); updateHover(true); renderSelection(); return; }
    if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && (ev.key === 'c' || ev.key === 'C' || ev.key === 'Insert')) {
      ev.preventDefault(); copySelection(); return; // Ctrl+C, Cmd+C and Ctrl+Insert
    }
    if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && (ev.key === 'v' || ev.key === 'V')) { ev.preventDefault(); pasteImage(); return; }
    if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && (ev.key === 'z' || ev.key === 'Z')) { ev.preventDefault(); if (ev.shiftKey) redo(); else undo(); return; }
    if (ev.ctrlKey && !ev.metaKey && !ev.altKey && (ev.key === 'y' || ev.key === 'Y')) { ev.preventDefault(); redo(); return; }
    if (ev.shiftKey && ev.key === 'Insert') { ev.preventDefault(); pasteImage(); return; } // Shift+Insert
    if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && (ev.key === 's' || ev.key === 'S')) { ev.preventDefault(); saveAs(); return; }
    if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && (ev.key === 'n' || ev.key === 'N')) { ev.preventDefault(); NewWindow(); return; } // Ctrl+N, Cmd+N
    if (!menuShown && ev.ctrlKey && !ev.metaKey && !ev.altKey) { // the menu bar is hidden (Windows full screen), and with it the shortcuts of its items that this page does not handle
      const k = ev.key.toLowerCase();
      if (k === 'o') { ev.preventDefault(); choose(); return; }
      if (k === 'w' && ev.shiftKey) { ev.preventDefault(); closeImage(); return; }
      if (k === 'w') { ev.preventDefault(); closeOrQuit(); return; }
      if (k === 'q') { ev.preventDefault(); CloseWindow(); return; }
    }
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return;
    if (ev.key === 'Enter') { ev.preventDefault(); ToggleFullscreen(); settleFullscreen(); return; }
    if (ev.key === 'Escape' && cancelDraw()) return;
    if (ev.key === 'Escape' && !ev.repeat) { // two presses within 250 ms: the picture is closed, or the window when there is none (the first one did its own job already)
      const now = performance.now();
      if (now - lastEsc < 250) { lastEsc = 0; ev.preventDefault(); if (pic.naturalWidth) closeImage(); else ConfirmQuit().then((ok) => { if (ok) CloseWindow(); }).catch(() => {}); return; } // (the home screen asks before it quits)
      lastEsc = now;
    }
    if (view.fl?.pasted && (ev.key === 'Escape' || ev.key === 'Delete' || ev.key === 'Backspace')) { ev.preventDefault(); removeLayer(); return; }
    if ((ev.key === 'Delete' || ev.key === 'Backspace') && pic.naturalWidth && !view.fl && view.sel && selectionRect()) { ev.preventDefault(); clearSelectedPart(); return; } // a selected part is cleared (Backspace goes to the previous picture only without one)
    if (ev.key === 'Escape' && editOn() && view.tool !== 'select') { setTool('select'); return; } // another tool: back to the rectangular selection
    if (ev.key === 'Escape') { clearSelection(); return; } // (Esc does not leave full screen: F and Enter do, and two Escs close the picture)
    if (ev.key === 'f' || ev.key === 'F') { ev.preventDefault(); ToggleFullscreen(); settleFullscreen(); return; } // (also on the home screen)
    if ((ev.key === 'c' || ev.key === 'C') && !ev.ctrlKey && !ev.metaKey && !ev.altKey) { ev.preventDefault(); toggleChat(); return; } // the chat panel
    if (ev.key === 'o' || ev.key === 'O') { ev.preventDefault(); choose(); return; } // (asks about unsaved changes first, then the Open dialog)
    if (ev.key === 'n' || ev.key === 'N') { ev.preventDefault(); createImage(ev.shiftKey); return; }
    if (ev.key === 'e' || ev.key === 'E' || ev.key === 'Tab') { ev.preventDefault(); setEdit(!editOn()); return; } // the edit bar (Tab does the same)
    if (!pic.naturalWidth) return;
    const b = stageSize();
    if (ev.key === 'ArrowLeft') { ev.preventDefault(); step(-1); }
    else if ((ev.key === 'ArrowUp' || ev.key === 'ArrowDown') && gif.meta) { ev.preventDefault(); gifStepAny(ev.key === 'ArrowDown' ? 1 : -1); } // an animated GIF: the frames
    else if (ev.key === 'ArrowRight') { ev.preventDefault(); step(1); }
    else if (ev.key === ' ') { ev.preventDefault(); gifSpace(); } // (Space does not go to the next picture: with an animated GIF it plays and pauses it)
    else if (ev.key === 'Backspace') { ev.preventDefault(); step(-1); }
    else if (MODES[ev.key]) { setMode(MODES[ev.key]); commit(); }
    else if (ev.key === 'r' || ev.key === 'R') {
      const dir = ev.shiftKey ? -1 : 1;
      if (!view.fl && selectionRect()) rotatePart(90 * dir); // a selected part is turned, not the picture
      else if (view.fl) flushFloat({clear: true}).then(() => { rotate(dir); commit(); }); // what is lifted is applied before the picture is turned
      else { rotate(dir); commit(); }
    }
    else if (ev.key === 's' || ev.key === 'S') saveAs();
    else if (ev.key === 'd' || ev.key === 'D') deleteImage();
    else if (ev.key === '+' || ev.key === '=') zoomAt(1.25, b.w / 2, b.h / 2);
    else if (ev.key === '-') zoomAt(0.8, b.w / 2, b.h / 2);
  });
}

document.addEventListener('copy', (ev) => { if (ev.target === $('textbox')) return; ev.preventDefault(); copySelection(); });
document.addEventListener('visibilitychange', () => Trace(`visibility ${document.visibilityState}`));
// macOS may still be hiding the app when the page starts (see App.startup): ask to be shown until the page is visible.
async function revealWhenHidden() {
  for (let i = 0; i < 40 && document.visibilityState === 'hidden'; i++) { await Reveal(); await new Promise((r) => setTimeout(r, 150)); }
}

// ---- Control mode ------------------------------------------------------------------------------------------
// With `mormiview --control PORT` another program can give commands (see control.go and docs/control.md). The ones that are not about the window or the
// dialogs come here as an event. They are done with the code of the mouse and the keys: the pointer commands send the same pointer events a mouse
// would, so every tool, every handle and every shortcut works as it does for a person. Picture positions are pixels of the picture, as the picture is
// shown: {x, y} names the pixel, and the pointer goes to its middle.
const ctl = {sleep: (ms) => new Promise((r) => setTimeout(r, ms))};

const ctlClient = (x, y) => { const r = $('stage').getBoundingClientRect(); return [r.left + view.x + view.ox + (x + 0.5) * view.sx, r.top + view.y + view.oy + (y + 0.5) * view.sy]; };

// Waits until what a command started is done (a step is in the history).
async function ctlSettle() {
  await ctl.sleep(40);
  for (let i = 0; i < 500 && (ink || fin || shape || history.busy); i++) await ctl.sleep(20);
  await ctl.sleep(120);
}

const CTL_MENU = {open: choose, save: saveAs, close: closeOrQuit, 'close-image': closeImage, create: () => createImage(), delete: deleteImage, discard: discardChanges, undo, redo, copy: () => copySelection(), paste: pasteImage};

function ctlState() {
  const sel = selectionRect();
  return {
    ok: true, name: view.name, dir: view.dir, index: view.index, count: view.count, size: pic.naturalWidth ? [dispW(), dispH()] : null,
    loaded: !!pic.naturalWidth && pic.complete, mode: view.mode, zoom: view.sx, zoomY: view.sy, pixelated: view.pixelated, rot: view.rot,
    edit: editOn(), tool: tool(), modified: !!(view.modified || floatChanges(view.fl)), selection: sel, polygon: poly.closed ? poly.pts : null, text: txt ? {x: txt.x, y: txt.y, value: $('textbox').value} : null,
    paint: {fg: paint.fg, bg: paint.bg, width: paint.width, style: paint.style, fill: paint.fill, tolerance: paint.tol, font: paint.font, size: paint.size, bold: paint.bold, italic: paint.italic},
    full: document.body.classList.contains('full'), dock: $('dock').classList.contains('open'), info: toldInfo, status: $('status').textContent, history: {at: history.at, length: history.list.length},
  };
}

async function ctlRun(m) {
  switch (m.cmd) {
    case 'state': return ctlState();
    case 'history': return {ok: true, checkpoints: [...checkpoints].map(([name, e]) => ({name, step: history.list.indexOf(e)})).filter((c) => c.step >= 0), at: history.at, steps: history.list.map((e, i) => ({n: i, label: e.label || (i ? 'change' : 'start'), by: e.by || 'user'}))}; // the steps undo goes through, oldest first
    case 'checkpoint': { // remember the step the window is at now under a name
      const e = history.list[history.at]; if (!e) throw new Error('there is no step yet');
      checkpoints.set(m.name, e); return {ok: true, step: history.at};
    }
    case 'goto': { // go to a step {step} of the history, or to a named checkpoint {checkpoint}: undo or redo as many times as it takes
      let to = m.step;
      if (m.checkpoint != null) { const e = checkpoints.get(m.checkpoint); to = e ? history.list.indexOf(e) : -1; if (to < 0) throw new Error(`there is no checkpoint "${m.checkpoint}" (any more)`); }
      if (!Number.isInteger(to) || to < 0 || to >= history.list.length) throw new Error(`there is no step ${to}: 0 to ${history.list.length - 1}`);
      await travel(to); await ctlSettle(); return {ok: true, at: history.at};
    }
    case 'show': { // make the part {x, y, w, h} fill the window (and, with mark, select it so that the user sees what is meant)
      const b = stageSize(), s = Math.min(b.w / m.w, b.h / m.h);
      view.free = true; view.sx = view.sy = s; view.ox = view.oy = 0; view.x = b.w / 2 - (m.x + m.w / 2) * s; view.y = b.h / 2 - (m.y + m.h / 2) * s;
      if (m.mark) view.sel = {x0: m.x, y0: m.y, x1: m.x + m.w, y1: m.y + m.h};
      draw(); await ctl.sleep(300); return ctlState();
    }
    case 'effect': { // an effect on a part: {op, …} (see effects.go); with selection: true the part is what the user selected
      const spec = {...m}; delete spec.cmd; delete spec.selection; delete spec.clear; delete spec.recent;
      if (m.selection) {
        const part = effectPart();
        if (!part.pts && part.x === undefined) throw new Error('nothing is selected in the window');
        Object.assign(spec, part);
      }
      if (m.clear) { spec.op = 'fill'; if (!spec.color) spec.color = paint.bg; } // the second color, as Delete does
      const info = await applyEffect(spec); if (m.recent) noteRecentEffect(spec.op); // (recent: true counts it as one used from the menu)
      return {...ctlState(), note: info.note || ''};
    }
    case 'transform': { // resize, rotate, flip or place (see transform.go)
      await commitText(); if (view.fl) await flushFloat({clear: true});
      const spec = {...m}; delete spec.cmd;
      const info = await Transform(view.rot, JSON.stringify(spec)); await showImage(info, {rot: 0}); commit(`${m.op}${m.op === 'place' ? 'd image' : ''}`); return ctlState();
    }
    case 'gif': { // the frames of an animation (1 based numbers are made 0 based by the tools): {op: start|add|duplicate|delete|move|delay, …}
      await commitText(); if (view.fl) await flushFloat({clear: true});
      const info = await gifDo(m);
      await showImage(info, {keepView: true, rot: 0}); commit(`animation: ${m.op}`); await ctlSettle(); return ctlState();
    }
    case 'ctxmenu': { // the right-click menu at the pixel {x, y} of the window (the middle by default); sub: the submenu to open, for example "Effects"
      const r = $('stage').getBoundingClientRect();
      showSelectionMenu(m.cx ?? r.left + r.width / 2, m.cy ?? r.top + r.height / 2);
      if (m.sub) [...document.querySelectorAll('#ctx .has-sub')].find((b) => b.firstChild.textContent === m.sub)?.onmouseenter?.();
      await ctl.sleep(300); return {ok: true};
    }
    case 'chat': { // the chat panel open (on: false shuts it), with an unsent draft {text} in its box
      toggleChat(m.on !== false);
      if (m.text !== undefined) { const t = $('chat-text'); t.value = m.text; t.dispatchEvent(new Event('input')); }
      await ctl.sleep(m.wait ?? 3500); return {ok: true};
    }
    case 'aidlg': await aiDialog(m); await ctl.sleep(m.wait ?? 600); return {ok: true}; // the AI provider dialog, filled in (close: true cancels it)
    case 'closemenu': closeSelectionMenu(); return {ok: true};
    case 'rotdlg': { // the Rotate by angle dialog, left open with the angle {angle} in it (close: true shuts it)
      if (m.close) { $('rd-cancel').click(); return {ok: true}; }
      askRotation(!!m.part).catch(() => {});
      $('rd-a').value = $('rd-r').value = String(m.angle ?? 0);
      await ctl.sleep(300); return {ok: true};
    }
    case 'tip': { // the tooltip of the element with the id {id}, as if the pointer had rested on it
      const el = document.getElementById(m.id); if (!el) throw new Error(`no element "${m.id}"`);
      el.dispatchEvent(new PointerEvent('pointerover', {bubbles: true}));
      await ctl.sleep(1400); return {ok: true};
    }
    case 'gifplay': { // the frame player: {on: true|false}, or {step: 1|-1} (stops the animation first)
      if (m.step) await gifStepAny(m.step); else await gifSetPlaying(m.on !== false);
      await ctl.sleep(300); return ctlState();
    }
    case 'wait': await ctl.sleep(m.ms ?? 200); return {ok: true};
    case 'menu': { const f = CTL_MENU[m.name]; if (!f) throw new Error(`no menu item "${m.name}": ${Object.keys(CTL_MENU).join(', ')}`); await f(); await ctlSettle(); return {ok: true}; }
    case 'edit': setEdit(m.on !== false); await ctl.sleep(250); return ctlState();
    case 'polygon': { // a closed polygon selection {pts: [[x, y], …]}, or none ({pts: []}) to let it go
      if (!m.pts?.length) { polyClear(); return ctlState(); }
      if (!editOn()) setEdit(true); setTool('poly'); poly.pts = m.pts.map((p) => [p[0], p[1]]); poly.closed = true; poly.down = false; poly.raw = []; renderPoly(); return ctlState();
    }
    case 'tool': if (!editOn()) setEdit(true); setTool(m.name); await ctl.sleep(250); return ctlState(); // select, poly, pen, line, rect, ellipse, fill, text, pick
    case 'mode': setMode(MODES[String(m.mode)] || m.mode); commit(); await ctl.sleep(300); return ctlState(); // 1 to 6
    case 'zoom': { // an exact zoom (1 is 100%) with the picture pixel {x, y} (the middle by default) in the middle of the window
      const b = stageSize(), s = m.scale, cx = m.x ?? dispW() / 2, cy = m.y ?? dispH() / 2;
      view.free = true; view.sx = view.sy = s; view.ox = view.oy = 0; view.x = b.w / 2 - (cx + 0.5) * s; view.y = b.h / 2 - (cy + 0.5) * s;
      draw(); await ctl.sleep(300); return ctlState();
    }
    case 'zoomby': { const r = $('stage').getBoundingClientRect(), [cx, cy] = m.x === undefined ? [r.left + r.width / 2, r.top + r.height / 2] : ctlClient(m.x, m.y); zoomAt(m.factor, cx - r.left, cy - r.top); await ctl.sleep(300); return ctlState(); }
    case 'pan': { const b = stageSize(); view.free = true; view.x = b.w / 2 - (m.x + 0.5) * view.sx; view.y = b.h / 2 - (m.y + 0.5) * view.sy; draw(); await ctl.sleep(250); return ctlState(); }
    case 'rotate': rotate(m.dir ?? 1); commit('turned'); await ctl.sleep(500); return ctlState();
    case 'fullscreen': { const on = document.body.classList.contains('full'); if (m.on === undefined || m.on !== on) { await ToggleFullscreen(); settleFullscreen(); await ctl.sleep(2200); } return ctlState(); }
    case 'color': if (m.fg) paint.fg = m.fg.toUpperCase(); if (m.bg) paint.bg = m.bg.toUpperCase(); showColors(); return {ok: true};
    case 'option': // width, style (solid, dashed, dotted), fill (outline, both, fill), tolerance (0 to 100), font, size, bold, italic
      for (const [k, key] of [['width', 'width'], ['style', 'style'], ['fill', 'fill'], ['tolerance', 'tol'], ['font', 'font'], ['size', 'size'], ['bold', 'bold'], ['italic', 'italic']]) if (m[k] !== undefined) paint[key] = m[k];
      showPenWidth(); showToolOptions(); $('pen-w').value = paint.width; renderText(); return {ok: true};
    case 'palette': if (m.open) openDock(m.open === 'bg' ? 'bg' : 'fg'); else closeDock(); await ctl.sleep(300); return {ok: true};
    case 'select': { // the frame of a selection {x0, y0, x1, y1}, both corners pixels of the picture
      if (!editOn()) setEdit(true); setTool('select');
      if (view.fl) await flushFloat({clear: true});
      view.sel = {x0: Math.min(m.x0, m.x1), y0: Math.min(m.y0, m.y1), x1: Math.max(m.x0, m.x1) + 1, y1: Math.max(m.y0, m.y1) + 1}; view.selDrag = null; draw(); commit(); return ctlState();
    }
    case 'shape': { // a line, rect, ellipse or path put into the picture: {kind, x0, y0, x1, y1, pts, color, bg, width, style, fill}, one step of the history
      await commitText(); if (view.fl) await flushFloat();
      const info = await DrawShape(view.rot, JSON.stringify(m)); await showImage(info, {keepView: true, rot: 0}); commit(`${m.kind} drawn`); return ctlState();
    }
    case 'resize': { // the canvas made w × h, the picture where the anchor says: {w, h, anchor, bg}, one step of the history
      await commitText(); if (view.fl) await flushFloat();
      const info = await ResizeCanvas(view.rot, m.w, m.h, m.anchor || '', m.bg || ''); await showImage(info, {rot: 0}); commit('canvas resized'); return ctlState();
    }
    case 'crop': { // only the part {x, y, w, h} is kept, one step of the history
      await commitText(); if (view.fl) await flushFloat();
      const info = await CropPicture(view.rot, m.x, m.y, m.w, m.h); await showImage(info, {rot: 0}); commit('cropped'); return ctlState();
    }
    case 'retouch': { // clone a part, blend its edges or fill a gap from its surroundings: {op: clone|blend|fill, …}, one step of the history
      await commitText(); if (view.fl) await flushFloat();
      const info = await Retouch(view.rot, JSON.stringify(m)); await showImage(info, {keepView: true, rot: 0}); commit(`retouch: ${m.op}`); return {...ctlState(), note: info.note || ''};
    }
    case 'fill': { // the paint bucket at the pixel {x, y}: {color, tolerance (0 to 100)}
      await commitText(); if (view.fl) await flushFloat({clear: true});
      const info = await FloodFill(view.rot, m.x, m.y, m.color, Math.round((m.tolerance ?? paint.tol) * 2.55));
      if (info.state !== view.state) { await showImage(info, {keepView: true, rot: 0}); commit('paint bucket fill'); }
      return ctlState();
    }
    case 'text': { // a text put into the picture with its top left corner at the pixel {x, y}: {value, color}, the font and size are the options
      await commitText(); if (view.fl) await flushFloat({clear: true});
      if (m.x < 0 || m.y < 0 || m.x >= dispW() || m.y >= dispH()) throw new Error('the text could not be placed: the pixel is outside the image');
      txt = {x: m.x, y: m.y, color: m.color || paint.fg, opts: {align: m.align, boxW: m.box_w, outline: m.outline, outlineW: m.outline_width, shadow: m.shadow}}; $('textbox').value = m.value;
      await commitText(); return ctlState();
    }
    case 'pixel': { if (pixelState === 'preparing') await preparePixels(); const p = await fetchPixel(m.x, m.y); if (m.show) showPixel(p); return {ok: true, pixel: p}; } // show: true puts it in the readout of the bar
    case 'cutsel': { // Cut: the selected part goes to the clipboard and is cleared
      await commitText(); if (!selectionRect()) throw new Error('nothing is selected in the window');
      await copySelection(); await clearSelectedPart(); await ctlSettle(); return ctlState();
    }
    case 'cropsel': { await commitText(); if (!selectionRect()) throw new Error('nothing is selected in the window'); await cropToSelection(); await ctlSettle(); return ctlState(); }
    case 'savesel': { // only the selected part is written to the file {path, overwrite, quality}
      await commitText(); if (view.fl) await flushFloat();
      const r = selectionRect(); if (!r) throw new Error('nothing is selected in the window');
      return {ok: true, note: await SaveRegionTo(await cutPng(r), m.path, !!m.overwrite, m.quality || 0)};
    }
    case 'printsel': { await commitText(); if (view.fl) await flushFloat(); const r = selectionRect() || {x: 0, y: 0, w: dispW(), h: dispH()}; return {ok: true, note: await PrintPNG(await cutPng(r))}; }
    case 'saveto': { // save the picture to a full path without the dialog: {path, overwrite}
      await commitText(); if (view.fl) await flushFloat();
      const note = await SaveToPathQ(m.path, view.rot, !!m.overwrite, m.quality || 0); await ctlSettle(); return {ok: true, note};
    }
    case 'pixels': { // set single pixels: {list: [[x, y, "#rrggbbaa"], …]}, one step of the history
      await commitText(); if (view.fl) await flushFloat();
      const info = await PaintPixels(view.rot, JSON.stringify(m.list)); await showImage(info, {keepView: true, rot: 0}); commit('pixels set'); await ctlSettle(); return ctlState();
    }
    case 'new': { // a new blank picture {w, h, color: "#rrggbbaa"}; edit mode starts with it, and a small one is shown big, in whole pixels
      editAfterImage = true;
      try { await NewBlankColor(m.w, m.h, m.color || '#FFFFFFFF'); } catch (e) { editAfterImage = false; throw e; }
      await ctl.sleep(600);
      if (Math.max(m.w, m.h) <= 256) {
        const b = stageSize(), s = Math.max(1, Math.floor(Math.min(b.w * 0.85 / m.w, b.h * 0.85 / m.h)));
        view.free = true; view.sx = view.sy = s; view.ox = view.oy = 0; view.x = (b.w - m.w * s) / 2; view.y = (b.h - m.h * s) / 2; draw();
      }
      await ctl.sleep(200); return ctlState();
    }
    case 'eval': throw new Error('"eval" does not exist: use the commands');
  }
  throw new Error(`unknown command "${m.cmd}"`);
}

EventsOn('control', async (id, m) => {
  let res;
  actor = 'AI';
  try { res = await ctlRun(m); } catch (e) { res = {error: errText(e)}; } finally { actor = 'user'; }
  try { await ControlReply(id, JSON.stringify(res)); } catch {}
});

(async () => {
  Trace(`js start, visibility ${document.visibilityState}`);
  const mode = await Mode();
  if (mode === 'about') { showAbout(); return; }
  revealWhenHidden();
  startViewer();
  showImage(await Image());
  commit('start'); // the way the window starts is where undo ends
})();
