#!/bin/sh
# Builds the web version of mormiView: the Go program for WebAssembly (web_main.go and the rest of the package) and the page (frontend/,
# started by frontend/web/boot.js) into a static site in frontend/dist-web, which any static web server can serve (it needs no backend).
#   scripts/build_web.sh          build
#   scripts/build_web.sh serve    build and serve it on http://localhost:5180
set -e
cd "$(dirname "$0")/.."
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" frontend/web/wasm_exec.js
mkdir -p frontend/web/public
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o frontend/web/public/mormiview.wasm .
(cd frontend && npx vite build --config vite.web.config.js)
echo "built frontend/dist-web ($(du -sh frontend/dist-web | cut -f1))"
if [ "$1" = serve ]; then
  cd frontend && exec npx vite preview --config vite.web.config.js --port 5180 --strictPort
fi
