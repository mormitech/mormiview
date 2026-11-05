// The web version (see web_main.go and web/): the same page as the desktop app, started by web/boot.js, which first starts the Go program
// (built for WebAssembly into web/public/mormiview.wasm by scripts/build_web.sh). The result is a static site in dist-web/.
import {defineConfig} from 'vite';

const webPage = {
  name: 'mormiview-web-page',
  transformIndexHtml: {order: 'pre', handler: (html) => html
      .replace('<script src="./src/main.js" type="module"></script>', '<script src="./web/boot.js" type="module"></script>')
      .replace('<title>mormiView</title>', `<title>mormiView</title>
    <meta name="description" content="mormiView in the browser: view, edit, retouch and animate pictures. Free, open source, no account; your pictures stay on your computer.">
    <link rel="icon" href="./src/assets/mormitech-mark.svg" type="image/svg+xml">`)
      .replace('<main id="viewer">', `<div id="mv-loading" role="status"><img src="./src/assets/mormitech-mark.svg" alt="" width="72"><p><span class="wm-mormi">mormi</span><span class="wm-product">View</span></p><div class="bar"><i></i></div><small>Loading…</small></div>
<main id="viewer">`)},
};

export default defineConfig({
  base: './',
  appType: 'mpa',
  publicDir: 'web/public',
  plugins: [webPage],
  build: {outDir: 'dist-web', emptyOutDir: true, target: 'es2022', chunkSizeWarningLimit: 1200},
  server: {port: 5174},
});
