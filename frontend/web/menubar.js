// The menu bar of the web version: the same menu Go builds for the desktop (about.go, effects_menu.go, recent.go…), drawn by the page.
// Go sends it whole each time something in it changes (an item turned on or off, the recent pictures, a check mark); a click goes back to
// Go (mvMenuClick), which runs the item's own callback. The shortcuts of the items work too, except the ones the page handles itself
// and the ones the browser keeps for itself (Ctrl+W, Ctrl+N, Ctrl+Q…).

const MAC = navigator.platform.startsWith('Mac');
const PAGE_KEYS = new Set(['c', 'v', 'z', 's', 'n', 'y', 'insert']); // main.js handles these keys itself
const extras = []; // {menu, after, label, run, accel}

// addMenuItem adds an item of the web version's own to a menu of Go's, after the item labelled after (at the end when there is none).
export function addMenuItem(menu, after, label, run) { extras.push({menu, after, label, run}); }

let bar = null, open = null, model = [], hoverOpen = false;

function accelText(a) {
  if (!a) return '';
  const parts = a.split('+'), key = parts.pop();
  const k = key.length === 1 ? key.toUpperCase() : key.charAt(0).toUpperCase() + key.slice(1);
  if (MAC) {
    const sym = {cmdorctrl: '⌘', shift: '⇧', optionoralt: '⌥', ctrl: '⌃'};
    return parts.map((p) => sym[p] || p).sort((x, y) => '⌃⌥⇧⌘'.indexOf(x) - '⌃⌥⇧⌘'.indexOf(y)).join('') + k;
  }
  const word = {cmdorctrl: 'Ctrl', shift: 'Shift', optionoralt: 'Alt', ctrl: 'Ctrl'};
  return [...parts.map((p) => word[p] || p), k].join('+');
}

function matches(a, ev) {
  if (!a) return false;
  const parts = a.toLowerCase().split('+'), key = parts.pop();
  const want = {cmd: parts.includes('cmdorctrl'), shift: parts.includes('shift'), alt: parts.includes('optionoralt'), ctrl: parts.includes('ctrl')};
  const cmd = MAC ? ev.metaKey : ev.ctrlKey;
  if (want.cmd !== cmd || want.shift !== ev.shiftKey || want.alt !== ev.altKey) return false;
  if (MAC && want.ctrl !== ev.ctrlKey) return false;
  return ev.key.toLowerCase() === key || ev.code?.toLowerCase() === `key${key}`;
}

function close() {
  if (!open) return;
  open.btn.classList.remove('open');
  open.panel.remove();
  open = null;
  hoverOpen = false;
}

function run(item) {
  close();
  if (item.run) item.run(); else window.mvMenuClick(item.id);
}

function panelFor(items, x, y, depth = 0) {
  const panel = document.createElement('div');
  panel.className = 'mv-menu';
  panel.setAttribute('role', 'menu');
  panel.style.left = `${x}px`; panel.style.top = `${y}px`;
  for (const it of items) {
    if (it.type === 'Separator') { const s = document.createElement('div'); s.className = 'sep'; panel.append(s); continue; }
    const b = document.createElement('button');
    b.type = 'button';
    b.setAttribute('role', it.type === 'Checkbox' ? 'menuitemcheckbox' : 'menuitem');
    if (it.type === 'Checkbox') b.setAttribute('aria-checked', String(!!it.checked));
    b.disabled = !!it.disabled;
    b.innerHTML = `<span class="chk">${it.checked ? '✓' : ''}</span><span class="lbl"></span><span class="acc">${accelText(it.accel)}</span>${it.items ? '<span class="arr">›</span>' : ''}`;
    b.querySelector('.lbl').textContent = it.label;
    if (it.items) {
      let sub = null;
      const show = () => {
        panel.querySelectorAll(':scope > .mv-menu').forEach((p) => p.remove());
        const r = b.getBoundingClientRect(), pr = panel.getBoundingClientRect();
        sub = panelFor(it.items, r.right - pr.left - 4, r.top - pr.top - 5, depth + 1);
        panel.append(sub);
        const sr = sub.getBoundingClientRect(); // (keeps it on the screen)
        if (sr.right > innerWidth) sub.style.left = `${-sr.width + 4}px`;
        if (sr.bottom > innerHeight) sub.style.top = `${Math.max(-pr.top, innerHeight - sr.height - pr.top - 4)}px`;
      };
      b.addEventListener('pointerenter', show);
      b.addEventListener('click', (ev) => { ev.stopPropagation(); show(); });
    } else {
      b.addEventListener('pointerenter', () => panel.querySelectorAll(':scope > .mv-menu').forEach((p) => p.remove()));
      b.addEventListener('click', (ev) => { ev.stopPropagation(); if (!it.disabled) run(it); });
    }
    panel.append(b);
  }
  return panel;
}

function openMenu(btn, top) {
  close();
  const r = btn.getBoundingClientRect();
  const panel = panelFor(top.items || [], r.left, r.bottom);
  document.body.append(panel);
  const pr = panel.getBoundingClientRect();
  if (pr.right > innerWidth - 4) panel.style.left = `${Math.max(4, innerWidth - pr.width - 4)}px`;
  btn.classList.add('open');
  open = {btn, panel, top};
  hoverOpen = true;
}

const DESKTOP_ONLY = new Set(['Exit']); // (a page does not end the browser; Ctrl+W closes the tab)

function withExtras(items) {
  const out = structuredClone(items);
  for (const m of out) {
    if (!m.items) continue;
    m.items = m.items.filter((x) => !DESKTOP_ONLY.has(x.label));
    while (m.items.at(-1)?.type === 'Separator') m.items.pop();
  }
  for (const e of extras) {
    const m = out.find((t) => t.label === e.menu);
    if (!m) continue;
    m.items = m.items || [];
    const item = {id: -1, label: e.label, type: 'Text', run: e.run};
    const i = m.items.findIndex((x) => x.label === e.after);
    if (i >= 0) m.items.splice(i + 1, 0, item); else m.items.push(item);
  }
  return out;
}

export function renderMenu(items) {
  model = withExtras(items);
  window.mvMenuModel = model; // (for checks from the console)
  if (!bar) {
    bar = document.createElement('nav');
    bar.id = 'menubar';
    bar.setAttribute('role', 'menubar');
    document.body.prepend(bar);
    document.body.classList.add('has-menubar');
    window.addEventListener('pointerdown', (ev) => { if (open && !ev.target.closest('.mv-menu, #menubar')) close(); }, true);
    window.addEventListener('blur', close);
    window.addEventListener('resize', close);
    window.addEventListener('keydown', onKey, true);
  }
  const wasOpen = open?.top?.label;
  close();
  bar.replaceChildren();
  const brand = document.createElement('span');
  brand.className = 'brand';
  brand.innerHTML = '<span class="wm-mormi">mormi</span><span class="wm-product">View</span>';
  bar.append(brand);
  for (const top of model) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = top.label;
    btn.setAttribute('role', 'menuitem');
    btn.addEventListener('pointerdown', (ev) => { ev.preventDefault(); if (open?.btn === btn) close(); else openMenu(btn, top); });
    btn.addEventListener('pointerenter', () => { if (hoverOpen && open?.btn !== btn) openMenu(btn, top); });
    btn.addEventListener('keydown', (ev) => { if (ev.key === 'Enter' || ev.key === ' ' || ev.key === 'ArrowDown') { ev.preventDefault(); openMenu(btn, top); open.panel.querySelector('button:not(:disabled)')?.focus(); } });
    bar.append(btn);
    if (top.label === wasOpen) queueMicrotask(() => openMenu(btn, top));
  }
}

function findAccel(items, ev) {
  for (const it of items) {
    if (it.items) { const f = findAccel(it.items, ev); if (f) return f; }
    else if (it.accel && matches(it.accel, ev)) return it;
  }
  return null;
}

function onKey(ev) {
  if (open && ev.key === 'Escape') { ev.preventDefault(); ev.stopImmediatePropagation(); close(); return; }
  if (open && (ev.key === 'ArrowDown' || ev.key === 'ArrowUp')) { // up and down in the open menu
    ev.preventDefault(); ev.stopImmediatePropagation();
    const btns = [...open.panel.querySelectorAll(':scope > button:not(:disabled)')];
    const i = btns.indexOf(document.activeElement);
    btns[(i + (ev.key === 'ArrowDown' ? 1 : -1) + btns.length) % btns.length]?.focus();
    return;
  }
  if (!(ev.metaKey || ev.ctrlKey || ev.altKey || /^F\d+$/.test(ev.key))) return;
  const t = ev.target;
  if (t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t instanceof HTMLSelectElement || t?.isContentEditable) return;
  if (document.querySelector('.mv-modal') || !document.getElementById('newdlg').hidden || !document.getElementById('rotdlg').hidden) return;
  if (PAGE_KEYS.has(ev.key.toLowerCase())) return;
  const it = findAccel(model, ev);
  if (!it || it.disabled) return;
  ev.preventDefault(); ev.stopImmediatePropagation();
  run(it);
}
