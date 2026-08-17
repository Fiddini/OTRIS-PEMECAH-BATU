#!/bin/bash
set -e
echo "=== BUILD WAKAF BENTUK JADI - TANPA JETSON ==="
mkdir -p bin/wakaf
go mod tidy
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_linux_amd64 ./cmd/pemecah-batu
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_linux_arm64 ./cmd/pemecah-batu
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/wakaf/pemecah-batu_windows_amd64.exe ./cmd/pemecah-batu
sha256sum bin/wakaf/* > bin/wakaf/SHA256SUMS.txt
ls -lh bin/wakaf/
