// Dialogs drawn by the page: in the web version the ones the system shows on the desktop (Wails' message dialog and the Save dialog),
// and in both versions the form of AI → AI Provider and Key… (formDialog, from chat-api.js).

import './dialogs.css';

const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));

// modal shows a box over the page; build fills it and gets done(value) to close it with an answer. Esc is cancel.
function modal(build, cancelValue) {
  return new Promise((resolve) => {
    const prevFocus = document.activeElement;
    const back = document.createElement('div');
    back.className = 'mv-modal';
    back.setAttribute('role', 'dialog');
    back.setAttribute('aria-modal', 'true');
    const box = document.createElement('form');
    box.className = 'mv-box';
    box.autocomplete = 'off';
    back.append(box);
    let closed = false;
    const done = (v) => {
      if (closed) return;
      closed = true;
      back.remove();
      window.removeEventListener('keydown', key, true);
      prevFocus?.focus?.();
      resolve(v);
    };
    const key = (ev) => {
      if (ev.key === 'Escape') { ev.preventDefault(); ev.stopImmediatePropagation(); done(cancelValue); return; }
      ev.stopImmediatePropagation(); // the page's keys (S, D, the arrows…) are not for it while a dialog is open
    };
    window.addEventListener('keydown', key, true);
    build(box, done);
    document.body.append(back);
    (box.querySelector('[autofocus]') || box.querySelector('.primary') || box.querySelector('button'))?.focus();
  });
}

// messageDialog is Wails' MessageDialog: a title, a message and buttons; the answer is the label of the button.
export function messageDialog(o) {
  const buttons = o.Buttons?.length ? o.Buttons : o.Type === 'question' ? ['Yes', 'No'] : ['OK']; // (the system's question without buttons has Yes and No)
  const cancel = o.CancelButton || (buttons.includes('Cancel') ? 'Cancel' : buttons.includes('No') ? 'No' : buttons.at(-1));
  return modal((box, done) => {
    box.innerHTML = `<h2>${esc(o.Title)}</h2><p class="mv-msg">${esc(o.Message).replace(/\n/g, '<br>')}</p><div class="mv-btns"></div>`;
    const row = box.querySelector('.mv-btns');
    for (const b of buttons) {
      const el = document.createElement('button');
      el.type = 'button';
      el.textContent = b;
      if (b === (o.DefaultButton || buttons[0])) { el.className = 'primary'; }
      el.onclick = () => done(b);
      row.append(el);
    }
    box.onsubmit = (ev) => { ev.preventDefault(); done(o.DefaultButton || buttons[0]); };
  }, cancel);
}

const FORMATS = [['png', 'PNG'], ['jpg', 'JPEG'], ['gif', 'GIF'], ['bmp', 'BMP'], ['tiff', 'TIFF']];

// saveDialog asks for the name of the file to save; what is saved is downloaded (memfs.js). The answer is the name, or '' for cancel.
export function saveDialog(o) {
  const pattern = o.Filters?.[0]?.Pattern || '';
  const allowed = FORMATS.filter(([e]) => !pattern || pattern.includes(`*.${e}`) || (e === 'tiff' && pattern.includes('*.tif')));
  const name = o.DefaultFilename || 'picture.png';
  return modal((box, done) => {
    box.innerHTML = `<h2>${esc(o.Title || 'Save as')}</h2>
      <div class="mv-row"><label for="mv-name">Name</label><input id="mv-name" type="text" spellcheck="false" value="${esc(name)}" autofocus></div>
      <div class="mv-row"><label for="mv-type">Type</label><select id="mv-type">${allowed.map(([e, n]) => `<option value="${e}">${n}</option>`).join('')}</select></div>
      <p class="mv-note">Your browser downloads the file (to your Downloads folder, or where it asks).</p>
      <p class="mv-err" hidden></p>
      <div class="mv-btns"><button type="button" class="cancel">Cancel</button><button type="submit" class="primary">Save</button></div>`;
    const input = box.querySelector('#mv-name'), type = box.querySelector('#mv-type'), err = box.querySelector('.mv-err');
    const extOf = (n) => { const i = n.lastIndexOf('.'); return i > 0 ? n.slice(i + 1).toLowerCase() : ''; };
    const syncType = () => { let e = extOf(input.value); if (e === 'jpeg') e = 'jpg'; if (e === 'tif') e = 'tiff'; if (allowed.some(([x]) => x === e)) type.value = e; };
    syncType();
    input.addEventListener('input', syncType);
    type.addEventListener('change', () => { // the name follows the type
      const v = input.value, i = v.lastIndexOf('.');
      input.value = `${i > 0 ? v.slice(0, i) : v}.${type.value}`;
    });
    setTimeout(() => { const v = input.value, i = v.lastIndexOf('.'); input.setSelectionRange(0, i > 0 ? i : v.length); }, 0);
    box.querySelector('.cancel').onclick = () => done('');
    box.onsubmit = (ev) => {
      ev.preventDefault();
      let v = input.value.trim().replace(/[/\\]/g, '_');
      if (!v || v.startsWith('.')) { err.hidden = false; err.textContent = 'Give the file a name.'; return; }
      if (!extOf(v)) v += `.${type.value}`;
      done(v);
    };
  }, '');
}

// pickerDialog: the browser opens a file chooser only right after a click or a key; when that moment is over, this asks for the click.
export function pickerDialog(title, label) {
  return modal((box, done) => {
    box.innerHTML = `<h2>${esc(title)}</h2><div class="mv-btns"><button type="button" class="cancel">Cancel</button><button type="submit" class="primary">${esc(label)}</button></div>`;
    box.querySelector('.cancel').onclick = () => done(false);
    box.onsubmit = (ev) => { ev.preventDefault(); done(true); };
  }, false);
}

// textDialog asks for one line of text (the API key of the chat).
// (setup, when given, runs once the box is filled: for parts that change with what is chosen)
export function formDialog(title, html, read, setup) {
  return modal((box, done) => {
    box.innerHTML = `<h2>${esc(title)}</h2>${html}<p class="mv-err" hidden></p><div class="mv-btns"><button type="button" class="cancel">Cancel</button><button type="submit" class="primary">Save</button></div>`;
    box.querySelector('.cancel').onclick = () => done(null);
    box.onsubmit = (ev) => {
      ev.preventDefault();
      const v = read(box);
      if (typeof v === 'string') { const e = box.querySelector('.mv-err'); e.hidden = false; e.textContent = v; return; }
      done(v);
    };
    setup?.(box);
  }, null);
}
