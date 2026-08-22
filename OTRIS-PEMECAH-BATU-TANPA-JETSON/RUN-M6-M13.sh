#!/bin/zsh
set -e
# RUN-M6-M13.sh - Jalanin PB-06 s/d PB-13
# Simpan di OTRIS-PEMECAH-BATU-TANPA-JETSON/

OUT=/tmp/out-final
SESSION=REAL01

echo "=== M6-M13 OTRIS PEMECAH BATU TANPA JETSON ==="
echo "OUT=$OUT SESSION=$SESSION"
rm -rf $OUT
mkdir -p $OUT docs/frozen
echo ok > docs/frozen/spec.md

# Buat dummy raw HL7 biar pipeline bisa jalan tanpa device
# HL7 MSH|^~\& sample
cat > $OUT/dummy.hl7 << 'HL7'
MSH|^~\&|BeneVision|N12|OTRIS|PEMECAH|202508230426||ORU^R01|12345|P|2.4
PID|||123456||TEST^PATIENT||19800101|M
OBX|1|NM|HR^Heart Rate||80|bpm||||F
OBX|2|NM|SpO2^Oxygen||98|%||||F
OBX|3|NM|NIBP_S^Systolic||120|mmHg||||F
HL7

# Buat raw_stream.bin dummy (binary)
python3 - << 'PY'
import pathlib
out = pathlib.Path("/tmp/out-final")
raw = out / "raw_stream.bin"
hl7 = b"MSH|^~\\&|BeneVision|N12|OTRIS|PEMECAH|202508230426||ORU^R01|12345|P|2.4\r"
hl7 += b"PID|||123456||TEST^PATIENT||19800101|M\r"
hl7 += b"OBX|1|NM|HR^Heart Rate||80|bpm||||F\r"
hl7 += b"OBX|2|NM|SpO2^Oxygen||98|%||||F\r"
hl7 += b"OBX|3|NM|NIBP_S^Systolic||120|mmHg||||F\r"
# tulis 10x biar ada sample
raw.write_bytes(hl7*10)
print(f"written {raw} {len(hl7*10)} bytes")
# manifest dummy
import json, time, os
manifest = {
 "session_id": "REAL01",
 "byte_count": len(hl7*10),
 "started_at": time.time(),
 "transport": "file",
 "note": "dummy for M6-M13"
}
(json_path := out / "manifest.json").write_text(json.dumps(manifest, indent=2))
PY

echo ""
echo "=== M6 PB-06 CAPTURE (file mode) ==="
./bin/pemecah-batu capture --mode file --file $OUT/raw_stream.bin --out $OUT --duration 1s || true
ls -lh $OUT | head -n 20

echo ""
echo "=== M7 PB-07 INSPECTOR ==="
./bin/pemecah-batu inspect $OUT || ./bin/pemecah-batu pipeline $OUT 2>&1 | head -n 50

echo ""
echo "=== M7-M13 PIPELINE PB-07 s/d PB-13 ==="
./bin/pemecah-batu pipeline $OUT || true

echo ""
echo "=== HASIL ==="
ls -lh $OUT
echo ""
echo "--- $OUT/profile.yaml (jika ada di $OUT) ---"
cat $OUT/profile.yaml 2>/dev/null | head -n 100 || echo "belum di $OUT, cek di profiles/experimental/..."

echo ""
echo "--- Cari profile.yaml di profiles/ ---"
find profiles -name "profile.yaml" 2>/dev/null | head -n 5 | xargs -I{} sh -c 'echo "== {} =="; cat {} | head -n 40'

echo ""
echo "=== SELESAI M6-M13 ==="
