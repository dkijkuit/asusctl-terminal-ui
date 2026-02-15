#!/usr/bin/env bash
set -euo pipefail

APP="asusctl-gui"

echo "Building ${APP}..."

if ! command -v go &>/dev/null; then
    echo "Go not found. Install with:  sudo apt install golang-go"
    exit 1
fi

BUILD_VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
go build -ldflags="-s -w -X main.BuildVersion=${BUILD_VERSION}" -o "${APP}" .
echo "Built ./${APP} ($(du -h ${APP} | cut -f1))"
echo ""
echo "Run:      ./${APP}"
echo "Install:  sudo install -m 755 ${APP} /usr/local/bin/"
