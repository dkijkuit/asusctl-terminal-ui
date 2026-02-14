#!/usr/bin/env bash
set -euo pipefail

APP="asusctl-gui"

echo "Building ${APP}..."

if ! command -v go &>/dev/null; then
    echo "Go not found. Install with:  sudo apt install golang-go"
    exit 1
fi

go build -ldflags="-s -w" -o "${APP}" .
echo "Built ./${APP} ($(du -h ${APP} | cut -f1))"
echo ""
echo "Run:      ./${APP}"
echo "Install:  sudo install -m 755 ${APP} /usr/local/bin/"
