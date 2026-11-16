#!/bin/zsh
set -e
cd /Users/lacita/Desktop/WIJOYO-KUSUMO
echo "=== RESET inspector ==="
git checkout -- OTRIS-PEMECAH-BATU-TANPA-JETSON/internal/inspector/inspector.go || true
cd OTRIS-PEMECAH-BATU-TANPA-JETSON

echo "=== FIX V9 - INSPECTOR unknown escape ==="
python3 << 'PY'
import pathlib
p = pathlib.Path("internal/inspector/inspector.go")
txt = p.read_text(errors='ignore')
print("BEFORE line 16:")
for line in txt.splitlines()[15:17]:
    print(line)

# Original buggy: "MSH|^~\&"  -> single backslash + & = invalid \& escape
# Fix to: "MSH|^~\\&" -> double backslash = valid, represents single backslash in string
# In file content, we need to replace \ & with \\ &
# The file has: ^~\&  (caret tilde backslash ampersand)
# Want: ^~\\& (caret tilde backslash backslash ampersand)
txt = txt.replace('^~\\&', '^~\\\\&')
# Also just in case there is ^~\\& already doubled? No

# Also fix json chain if present
import re
def repl_enc(m):
    indent=m.group(1)
    enc_arg=m.group(2).strip()
    encode_arg=m.group(3).strip()
    return f'{indent}enc := json.NewEncoder({enc_arg})\n{indent}enc.SetIndent("", "  ")\n{indent}enc.Encode({encode_arg})'
txt, n = re.subn(r'^(\s*)json\.NewEncoder\(([^)]+)\)\.SetIndent\("", "  "\)\.Encode\(([^)]+)\)', repl_enc, txt, flags=re.MULTILINE)
print(f"json fix n={n}")

p.write_text(txt)
print("\nAFTER line 16:")
for line in txt.splitlines()[15:17]:
    print(line)
print("DONE V9")
PY

echo ""
echo "--- inspector.go 15-20 ---"
sed -n '15,20p' internal/inspector/inspector.go

echo ""
echo "=== BUILD V9 ==="
rm -rf bin
mkdir -p bin
go mod tidy
go build -o bin/pemecah-batu ./cmd/WIJOYO_KUSUMO
echo "BUILD exit=$?"
ls -lh bin/pemecah-batu
./bin/pemecah-batu --help | head -n 20

if [ -f bin/pemecah-batu ]; then
  echo ""
  echo "=== ADO PIAK! BUILD SUKSES V9 ==="
fi
