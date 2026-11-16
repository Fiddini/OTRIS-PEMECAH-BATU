#!/bin/bash
set -e
echo "=== BUILD WAKAF BENTUK JADI - TANPA JETSON ==="
mkdir -p bin/wakaf
go mod tidy
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_linux_amd64 ./cmd/WIJOYO_KUSUMO
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_linux_arm64 ./cmd/WIJOYO_KUSUMO
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_windows_amd64.exe ./cmd/WIJOYO_KUSUMO
sha256sum bin/wakaf/* > bin/wakaf/SHA256SUMS.txt
ls -lh bin/wakaf/
