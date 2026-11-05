// The file system of the web version: Go's os package, built for WebAssembly, calls globalThis.fs with Node's callback API. This one keeps
// the files in memory and in the browser's IndexedDB, so that the pictures opened, the recent list and the settings are still there after a
// reload (and in the other tabs that are opened later). /tmp is not kept.
//
// A picture that Go writes outside the settings and /tmp (Save As, overwrite, exported frames) is also downloaded: that is how a page gives
// the user a file.

const HOME = '/home/user';
export const PICTURES = `${HOME}/Pictures`;
export const DOWNLOADS = `${HOME}/Downloads`;
const CONFIG = `${HOME}/.config`;
const DB = 'mormiview-files', STORE = 'files';

const S_IFDIR = 0o040000, S_IFREG = 0o100000;
const C = {O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536};

const nodes = new Map(); // path → {dir: true, mtime} | {data: Uint8Array, size, mtime}
const fds = new Map();   // fd → {path, pos, flags, wrote}
let nextFd = 10;
let ino = 1;
const inos = new Map();

function err(code, path) {
  const e = new Error(`${code}: ${path ?? ''}`);
  e.code = code;
  return e;
}

export function norm(p) {
  p = String(p);
  if (!p.startsWith('/')) p = '/' + p;
  const out = [];
  for (const part of p.split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') out.pop(); else out.push(part);
  }
  return '/' + out.join('/');
}
const parent = (p) => p === '/' ? '/' : (p.slice(0, p.lastIndexOf('/')) || '/');
export const base = (p) => p.slice(p.lastIndexOf('/') + 1);

function mkdirp(p) {
  p = norm(p);
  if (nodes.get(p)?.dir) return;
  if (p !== '/') mkdirp(parent(p));
  nodes.set(p, {dir: true, mtime: Date.now()});
  persist(p);
}

function stat(p) {
  const n = nodes.get(p);
  if (!n) throw err('ENOENT', p);
  if (!inos.has(p)) inos.set(p, ino++);
  const size = n.dir ? 4096 : n.size;
  return {
    dev: 1, ino: inos.get(p), mode: (n.dir ? S_IFDIR | 0o755 : S_IFREG | 0o644), nlink: 1, uid: 0, gid: 0, rdev: 0,
    size, blksize: 4096, blocks: Math.ceil(size / 512), atimeMs: n.mtime, mtimeMs: n.mtime, ctimeMs: n.mtime,
    isDirectory: () => !!n.dir,
  };
}

function grow(n, need) {
  if (n.data.length >= need) return;
  const d = new Uint8Array(Math.max(need, n.data.length * 2, 4096));
  d.set(n.data.subarray(0, n.size));
  n.data = d;
}

// ---- What happens to written files: kept, and downloaded --------------------------------------------------------------------

const kept = (p) => !p.startsWith('/tmp/') && p !== '/tmp';
const IMAGE = /\.(png|jpe?g|jpe|jfif|gif|bmp|webp|tiff?|ico|svg|avif)$/i;
const downloads = new Map();
let onDownload = null;

// A finished picture file outside the settings and /tmp: downloaded after a moment (a file that is renamed at once, the temporary file of a
// save, is not; nor a file that starts with a dot).
function written(p) {
  if (!kept(p) || p.startsWith(CONFIG + '/') || base(p).startsWith('.') || !IMAGE.test(p)) return;
  clearTimeout(downloads.get(p));
  downloads.set(p, setTimeout(() => {
    downloads.delete(p);
    const n = nodes.get(p);
    if (n && !n.dir && onDownload) onDownload(p, n.data.slice(0, n.size));
    if (n && p.startsWith(DOWNLOADS + '/')) { nodes.delete(p); persist(p); } // (the downloads folder only hands files to the browser)
  }, 120));
}

export function setDownloadHandler(fn) { onDownload = fn; }

// ---- IndexedDB ------------------------------------------------------------------------------------------------------------------

let db = null;
const dirty = new Set();
let flushTimer = 0;
let flushing = Promise.resolve();

function persist(p) {
  if (!kept(p)) return;
  dirty.add(p);
  clearTimeout(flushTimer);
  flushTimer = setTimeout(flush, 250);
}

// flush writes what changed to IndexedDB (a tab that is opened next reads it).
export function flush() {
  clearTimeout(flushTimer);
  if (!db || !dirty.size) return flushing;
  const paths = [...dirty];
  dirty.clear();
  flushing = flushing.then(() => new Promise((resolve) => {
    const tx = db.transaction(STORE, 'readwrite');
    const st = tx.objectStore(STORE);
    for (const p of paths) {
      const n = nodes.get(p);
      if (!n) st.delete(p);
      else if (n.dir) st.put({dir: true, mtime: n.mtime}, p);
      else st.put({data: n.data.slice(0, n.size), mtime: n.mtime}, p);
    }
    tx.oncomplete = tx.onerror = tx.onabort = () => resolve();
  }));
  return flushing;
}

function openDB() {
  return new Promise((resolve) => {
    let req;
    try { req = indexedDB.open(DB, 1); } catch { resolve(null); return; }
    req.onupgradeneeded = () => req.result.createObjectStore(STORE);
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => resolve(null);
  });
}

async function load() {
  db = await openDB();
  if (!db) return;
  await new Promise((resolve) => {
    const tx = db.transaction(STORE, 'readonly');
    const req = tx.objectStore(STORE).openCursor();
    req.onsuccess = () => {
      const c = req.result;
      if (!c) { resolve(); return; }
      const v = c.value, p = c.key;
      if (v.dir) nodes.set(p, {dir: true, mtime: v.mtime});
      else nodes.set(p, {data: v.data, size: v.data.length, mtime: v.mtime});
      c.continue();
    };
    req.onerror = () => resolve();
  });
  for (const p of [...nodes.keys()]) mkdirp(parent(p)); // (a folder record may be missing)
}

// ---- What the page uses -----------------------------------------------------------------------------------------------------------

// writeFile puts bytes at a path (making the folders), as an opened picture is copied in.
export function writeFile(p, bytes) {
  p = norm(p);
  mkdirp(parent(p));
  const data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  nodes.set(p, {data, size: data.length, mtime: Date.now()});
  persist(p);
  return p;
}

export function exists(p) { return nodes.has(norm(p)); }
export function readFile(p) { const n = nodes.get(norm(p)); return n && !n.dir ? n.data.slice(0, n.size) : null; }

// A name in dir that no file has yet: "name.png", else "name (2).png" …
export function freeName(dir, name) {
  if (!nodes.has(`${dir}/${name}`)) return name;
  const dot = name.lastIndexOf('.'), stem = dot > 0 ? name.slice(0, dot) : name, ext = dot > 0 ? name.slice(dot) : '';
  for (let i = 2; ; i++) if (!nodes.has(`${dir}/${stem} (${i})${ext}`)) return `${stem} (${i})${ext}`;
}

// ---- Node's fs, as syscall/fs_js.go calls it ------------------------------------------------------------------------------------

const later = (cb, e, v) => queueMicrotask(() => cb(e, v));
function wrap(fn) {
  return (...args) => {
    const cb = args.pop();
    let v;
    try { v = fn(...args); } catch (e) { later(cb, e.code ? e : err('EIO', e.message)); return; }
    later(cb, null, v);
  };
}

let outputBuf = '';
const decoder = new TextDecoder('utf-8');

const fs = {
  constants: C,
  writeSync(fd, buf) {
    if (fd === 1 || fd === 2) {
      outputBuf += decoder.decode(buf);
      const nl = outputBuf.lastIndexOf('\n');
      if (nl !== -1) { console.log(outputBuf.substring(0, nl)); outputBuf = outputBuf.substring(nl + 1); }
      return buf.length;
    }
    return fdWrite(fd, buf, 0, buf.length, null);
  },
  write: wrap((fd, buf, off, len, pos) => (fd === 1 || fd === 2) ? fs.writeSync(fd, buf.subarray(off, off + len)) : fdWrite(fd, buf, off, len, pos)),
  read: wrap((fd, buf, off, len, pos) => {
    const f = fds.get(fd);
    if (!f) throw err('EBADF');
    const n = nodes.get(f.path);
    if (!n || n.dir) throw err('EISDIR', f.path);
    const at = pos ?? f.pos;
    const cnt = Math.max(0, Math.min(len, n.size - at));
    buf.set(n.data.subarray(at, at + cnt), off);
    if (pos == null) f.pos += cnt;
    return cnt;
  }),
  open: wrap((path, flags, mode) => {
    const p = norm(path);
    let n = nodes.get(p);
    if (n && (flags & C.O_CREAT) && (flags & C.O_EXCL)) throw err('EEXIST', p);
    if (!n) {
      if (!(flags & C.O_CREAT)) throw err('ENOENT', p);
      const d = nodes.get(parent(p));
      if (!d) throw err('ENOENT', p);
      if (!d.dir) throw err('ENOTDIR', p);
      n = {data: new Uint8Array(0), size: 0, mtime: Date.now()};
      nodes.set(p, n);
    } else if (n.dir && (flags & 3)) throw err('EISDIR', p);
    if ((flags & C.O_DIRECTORY) && !n.dir) throw err('ENOTDIR', p);
    if (!n.dir && (flags & C.O_TRUNC) && (flags & 3)) { n.size = 0; n.mtime = Date.now(); }
    const fd = nextFd++;
    fds.set(fd, {path: p, pos: (flags & C.O_APPEND) ? n.size : 0, flags, wrote: false});
    return fd;
  }),
  close: wrap((fd) => {
    const f = fds.get(fd);
    if (!f) throw err('EBADF');
    fds.delete(fd);
    if (f.wrote) { persist(f.path); written(f.path); }
  }),
  fstat: wrap((fd) => { const f = fds.get(fd); if (!f) throw err('EBADF'); return stat(f.path); }),
  stat: wrap((p) => stat(norm(p))),
  lstat: wrap((p) => stat(norm(p))),
  mkdir: wrap((p) => {
    p = norm(p);
    if (nodes.has(p)) throw err('EEXIST', p);
    if (!nodes.get(parent(p))?.dir) throw err('ENOENT', p);
    nodes.set(p, {dir: true, mtime: Date.now()});
    persist(p);
  }),
  readdir: wrap((p) => {
    p = norm(p);
    const n = nodes.get(p);
    if (!n) throw err('ENOENT', p);
    if (!n.dir) throw err('ENOTDIR', p);
    const pre = p === '/' ? '/' : p + '/';
    const out = [];
    for (const k of nodes.keys()) if (k !== p && k.startsWith(pre) && !k.slice(pre.length).includes('/')) out.push(k.slice(pre.length));
    return out;
  }),
  unlink: wrap((p) => {
    p = norm(p);
    const n = nodes.get(p);
    if (!n) throw err('ENOENT', p);
    if (n.dir) throw err('EISDIR', p);
    nodes.delete(p); persist(p);
  }),
  rmdir: wrap((p) => {
    p = norm(p);
    const n = nodes.get(p);
    if (!n) throw err('ENOENT', p);
    if (!n.dir) throw err('ENOTDIR', p);
    const pre = p + '/';
    for (const k of nodes.keys()) if (k.startsWith(pre)) throw err('ENOTEMPTY', p);
    nodes.delete(p); persist(p);
  }),
  rename: wrap((from, to) => {
    from = norm(from); to = norm(to);
    const n = nodes.get(from);
    if (!n) throw err('ENOENT', from);
    if (!nodes.get(parent(to))?.dir) throw err('ENOENT', to);
    const moved = [[from, to]];
    if (n.dir) for (const k of nodes.keys()) if (k.startsWith(from + '/')) moved.push([k, to + k.slice(from.length)]);
    for (const [a, b] of moved) { const v = nodes.get(a); nodes.delete(a); nodes.set(b, v); persist(a); persist(b); }
    if (!n.dir) written(to);
  }),
  truncate: wrap((p, len) => { const n = nodes.get(norm(p)); if (!n || n.dir) throw err('ENOENT', p); setSize(n, len); persist(norm(p)); }),
  ftruncate: wrap((fd, len) => { const f = fds.get(fd); if (!f) throw err('EBADF'); setSize(nodes.get(f.path), len); f.wrote = true; }),
  fsync: wrap(() => {}),
  chmod: wrap(() => {}), fchmod: wrap(() => {}), chown: wrap(() => {}), fchown: wrap(() => {}), lchown: wrap(() => {}),
  utimes: wrap((p, at, mt) => { const n = nodes.get(norm(p)); if (!n) throw err('ENOENT', p); n.mtime = mt * 1000; }),
  link: wrap(() => { throw err('ENOSYS'); }), symlink: wrap(() => { throw err('ENOSYS'); }),
  readlink: wrap((p) => { throw err('EINVAL', p); }),
};

function setSize(n, len) {
  grow(n, len);
  if (len > n.size) n.data.fill(0, n.size, len);
  n.size = len; n.mtime = Date.now();
}

function fdWrite(fd, buf, off, len, pos) {
  const f = fds.get(fd);
  if (!f) throw err('EBADF');
  const n = nodes.get(f.path);
  if (!n || n.dir) throw err('EISDIR', f.path);
  let at = pos ?? f.pos;
  if (f.flags & C.O_APPEND) at = n.size;
  grow(n, at + len);
  if (at > n.size) n.data.fill(0, n.size, at);
  n.data.set(buf.subarray(off, off + len), at);
  n.size = Math.max(n.size, at + len);
  n.mtime = Date.now();
  if (pos == null) f.pos = at + len;
  f.wrote = true;
  return len;
}

let cwd = PICTURES;
const enosys = () => { const e = new Error('not implemented'); e.code = 'ENOSYS'; throw e; };
globalThis.fs = fs;
globalThis.process = {
  getuid: () => 1000, getgid: () => 1000, geteuid: () => 1000, getegid: () => 1000, getgroups: enosys,
  pid: 1, ppid: 0, umask: () => 0o022, cwd: () => cwd, chdir: (p) => { cwd = norm(p); },
};
globalThis.path = {resolve: (...ps) => { let r = cwd; for (const p of ps) r = String(p).startsWith('/') ? p : `${r}/${p}`; return norm(r); }};

// ready: the files of the earlier visits are read, and the usual folders are there.
export const ready = (async () => {
  await load();
  for (const d of ['/tmp', PICTURES, DOWNLOADS, CONFIG]) mkdirp(d);
})();

export const env = {HOME, TMPDIR: '/tmp', USER: 'user', XDG_CONFIG_HOME: CONFIG};
