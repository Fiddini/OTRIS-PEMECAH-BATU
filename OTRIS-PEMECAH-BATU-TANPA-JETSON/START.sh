#!/bin/bash
set -e
echo "=== OTRIS PEMECAH BATU - CLEAN START ==="
echo "Wakaf Ayahanda Wijoyo Kusumo - Allahu Latif 125x"
cd "$(dirname "$0")"
go mod tidy
go build -o bin/pemecah-batu ./cmd/WIJOYO_KUSUMO
./bin/pemecah-batu --help
echo ""
echo "Test tanpa alat:"
echo -e "\x0BMSH|^~\&|LAB|ABC|001|70|bpm\x1C\x0D" > /tmp/mindray.bin
./bin/pemecah-batu capture --mode file --file /tmp/mindray.bin --out captures/hr70 --duration 2s
./bin/pemecah-batu pipeline captures/hr70
echo ""
echo "SELESAI - Bentuk jadi siap di bin/pemecah-batu"
