// The start of the web version: the files of earlier visits are read, the Go program (mormiview.wasm, the same Go code as the desktop
// app) is started, its methods are bound, and then the page of the desktop app (src/main.js) runs as it does there.

import './web.css';
import {ready, env} from './memfs.js';
import './wasm_exec.js';
import {bind, goReady, openFolder} from './host.js';
import {addMenuItem} from './menubar.js';

const loading = document.getElementById('mv-loading');
const bar = loading?.querySelector('.bar i');
const fail = (msg) => {
  if (!loading) return;
  loading.querySelector('small').textContent = msg;
  loading.classList.add('failed');
};

// fetchWasm loads the program and shows how far it is (it is some megabytes).
async function fetchWasm(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: ${res.status}`);
  const total = +res.headers.get('content-length') || 0;
  if (!res.body || !total || res.headers.get('content-encoding')) return res.arrayBuffer();
  const reader = res.body.getReader(), buf = new Uint8Array(total);
  for (let got = 0; ;) {
    const {done, value} = await reader.read();
    if (done) return buf.buffer.slice(0, got);
    if (got + value.length > buf.length) return null; // (the size was not the real one: read it again plainly)
    buf.set(value, got);
    got += value.length;
    if (bar) bar.style.width = `${Math.round(got / total * 100)}%`;
  }
}

async function start() {
  addMenuItem('File', 'Open File…', 'Open Folder…', openFolder);
  const url = new URL('mormiview.wasm', document.baseURI).href;
  const [, bytes] = await Promise.all([ready, fetchWasm(url).then((b) => b || fetch(url).then((r) => r.arrayBuffer()))]);
  const go = new globalThis.Go();
  go.argv = ['mormiview'];
  go.env = env;
  const {instance} = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance).then(() => fail('mormiView stopped. Reload the page to start it again.'));
  await goReady;
  bind();
  document.body.classList.add('web');
  await import('../src/main.js'); // (in the built site this also waits until the app's style is loaded)
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))); // its first frame is drawn under the loading screen
  document.body.classList.add('mv-ready');
  loading?.remove();
}

start().catch((e) => { console.error(e); fail(`mormiView could not start: ${e.message || e}`); });
