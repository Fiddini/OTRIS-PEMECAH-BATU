#!/bin/zsh
set -e
# BUILD-FINAL-LOCKED.sh - Simpan di OTRIS-PEMECAH-BATU-TANPA-JETSON/
# Ini gabungan V6+V7+V8+V9 yang sudah BUILD exit=0

echo "=== BUILD FINAL LOCKED - TANPA JETSON ==="
cd "$(dirname "$0")"

echo "[1] Reset dari git..."
git checkout -- internal/capture/capture.go internal/profilebuilder/profilebuilder.go internal/inspector/inspector.go internal/differential/differential.go internal/fingerprint/fingerprint.go internal/evidence/evidence.go internal/classifier/classifier.go internal/replay/replay.go internal/framing/framing.go || true

echo "[2] Patch semua..."
python3 << 'PY'
import pathlib, re

# CAPTURE
p = pathlib.Path("internal/capture/capture.go")
txt = p.read_text(errors='ignore')
lines = txt.splitlines()
out=[]
skip=False
depth=0
for line in lines:
    if not skip and re.match(r'\s*type\s+osContext\b', line):
        skip=True
        depth=line.count('{')-line.count('}')
        if depth<=0: depth=1
        continue
    if not skip and re.match(r'\s*func\s+osBackground\s*\(\)', line):
        skip=True
        depth=line.count('{')-line.count('}')
        if depth<=0: depth=1
        continue
    if not skip and re.match(r'\s*func\s+context\.Background\(\)', line):
        skip=True
        depth=line.count('{')-line.count('}')
        if depth<=0: depth=1
        continue
    if skip:
        depth+=line.count('{')
        depth-=line.count('}')
        if depth<=0:
            skip=False
        continue
    out.append(line)
txt = "\n".join(out)
txt = txt.replace("osBackground()", "context.Background()")
txt = re.sub(r'\bosContext\b', 'context.Context', txt)
txt = txt.replace('json.NewEncoder(mf2).SetIndent("", "  ").Encode(meta)', 'enc2 := json.NewEncoder(mf2)\n\tenc2.SetIndent("", "  ")\n\tenc2.Encode(meta)')
if '"context"' not in txt and 'context.Background()' in txt:
    txt = txt.replace('import (', 'import (\n\t"context"', 1)
p.write_text(txt)

# PROFILEBUILDER
p2 = pathlib.Path("internal/profilebuilder/profilebuilder.go")
txt2 = p2.read_text(errors='ignore')
txt2 = txt2.replace('fmt.Sprintf("version: "%s"\\n"', 'fmt.Sprintf("version: \\"%s\\"\\n"')
txt2 = txt2.replace('fmt.Sprintf("firmware_tested: "%s"\\n"', 'fmt.Sprintf("firmware_tested: \\"%s\\"\\n"')
txt2 = txt2.replace('fmt.Sprintf("      source_code: "%s"\\n"', 'fmt.Sprintf("      source_code: \\"%s\\"\\n"')
txt2 = txt2.replace('fmt.Sprintf("validated_at: "%s"\\n"', 'fmt.Sprintf("validated_at: \\"%s\\"\\n"')
txt2 = txt2.replace('fmt.Sprintf("  - "%s"\\n"', 'fmt.Sprintf("  - \\"%s\\"\\n"')
def repl_enc(m):
    indent=m.group(1)
    enc_arg=m.group(2).strip()
    encode_arg=m.group(3).strip()
    return f'{indent}enc := json.NewEncoder({enc_arg})\n{indent}enc.SetIndent("", "  ")\n{indent}enc.Encode({encode_arg})'
txt2, _ = re.subn(r'^(\s*)json\.NewEncoder\(([^)]+)\)\.SetIndent\("", "  "\)\.Encode\(([^)]+)\)', repl_enc, txt2, flags=re.MULTILINE)
p2.write_text(txt2)

# INSPECTOR
p3 = pathlib.Path("internal/inspector/inspector.go")
txt3 = p3.read_text(errors='ignore')
txt3 = txt3.replace('^~\\&', '^~\\\\&')
txt3, _ = re.subn(r'^(\s*)json\.NewEncoder\(([^)]+)\)\.SetIndent\("", "  "\)\.Encode\(([^)]+)\)', repl_enc, txt3, flags=re.MULTILINE)
p3.write_text(txt3)

# LAINNYA
for p4 in pathlib.Path("internal").rglob("*.go"):
    if p4.name in ("capture.go","profilebuilder.go","inspector.go"):
        continue
    t = p4.read_text(errors='ignore')
    t_new, nn = re.subn(r'^(\s*)json\.NewEncoder\(([^)]+)\)\.SetIndent\("", "  "\)\.Encode\(([^)]+)\)', repl_enc, t, flags=re.MULTILINE)
    if nn>0:
        if t_new.count('enc := json.NewEncoder') > 1:
            parts = t_new.split('enc := json.NewEncoder')
            new_t = parts[0]
            for idx, part in enumerate(parts[1:], start=1):
                if idx==1:
                    new_t += 'enc := json.NewEncoder' + part
                else:
                    new_t += 'enc2 := json.NewEncoder' + part.replace('enc.SetIndent','enc2.SetIndent',1).replace('enc.Encode','enc2.Encode',1)
            t_new = new_t
        p4.write_text(t_new)

print("PATCH DONE")
PY

echo "[3] Build..."
rm -rf bin
mkdir -p bin
go mod tidy
go build -o bin/pemecah-batu ./cmd/WIJOYO_KUSUMO
echo "BUILD exit=$?"
ls -lh bin/pemecah-batu
./bin/pemecah-batu --help | head -n 15

if [ -f bin/pemecah-batu ]; then
  echo ""
  echo "=== ADO PIAK! FINAL LOCKED SUKSES ==="
fi
