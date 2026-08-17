# OTRIS Pemecah Batu — Universal Integration Workbench (Wakaf)

> `Teknologi harus mengurangi beban manusia, bukan menambah ketergantungan.`

## Apa itu Pemecah Batu?

Bukan cracker. Bukan pembobol.
Pemecah Batu adalah **Open Integration Workbench** untuk ubah `data/interface yang belum dikenal` jadi `integration knowledge yang bisa diuji, dibuktikan, dan dipakai ulang`.

Wakaf untuk RS Indonesia.

## 5 Hukum Konstitusi

1. PATIENT FIRST
2. UNKNOWN JANGAN DITEBAK
3. DISCOVERY != VALIDASI
4. SETIAP KLAIM BUTUH KETERTELUSURAN
5. PENGETAHUAN KEMBALI KE MANUSIA

## Status Implementasi (Wakaf)

| Engine | Status | Keterangan |
|--------|--------|------------|
| PB-01 Protocol Ingress | ✅ DONE | passive/active/file |
| PB-02 Signal Conditioner | ✅ DONE | suspect detection |
| PB-03 Timestamp Normalizer | ✅ DONE | UTC + monotonic |
| PB-04 Stream Buffer | ✅ DONE | circular + DROP_OLDEST |
| PB-05 Data Validator | ✅ DONE | PASS/DROP only |
| PB-06 Capture | ✅ DONE | raw_stream.bin + manifest |
| PB-07 Inspector | 📝 SPEC | ASCII/BINARY |
| PB-08 Framing | 📝 SPEC | MLLP/NEWLINE/FIXED |
| PB-09 Classifier | 📝 SPEC | HL7/ASTM/UNKNOWN |
| PB-10 Fingerprint | 📝 SPEC | HIGH/PARTIAL/UNKNOWN |
| PB-11 Differential | 📝 SPEC | CHANGES_WITH_HR |
| PB-12 Evidence+Replay | 📝 SPEC | 1000x false_data=0 |
| PB-13 Profile Builder | 📝 SPEC | yaml final |

## Cara Pakai - Hari Ini Bisa Go Run

```bash
# install deps
go mod tidy

# lihat bantuan
go run ./cmd/pemecah-batu --help

# 1. Capture dari file (test tanpa alat)
echo -e "MSH|^~\\&|ABC|001|70|bpm" > /tmp/sample.txt
go run ./cmd/pemecah-batu capture --mode file --file /tmp/sample.txt --out captures/test01 --duration 2s

# 2. Capture passive - tunggu Mindray connect ke kita (AMAN, tidak ganggu alat)
go run ./cmd/pemecah-batu capture --mode passive --transport tcp --listen 0.0.0.0:4000 --out captures/sardjito_mindray_01 --duration 30s
# lalu di Mindray setting IP kita: 192.168.1.10 port 4000

# 3. Capture active - kita connect ke alat (butuh izin RS)
go run ./cmd/pemecah-batu capture --mode active --target 192.168.1.50:4000 --out captures/sesi02 --duration 30s

# cek hasil
ls -lh captures/test01/
cat captures/test01/manifest.json
hexdump -C captures/test01/raw_stream.bin | head
```

## Struktur Output PB-06

```
captures/sardjito_mindray_n12_01/
├── raw_stream.bin   <- kebenaran, jangan diubah
├── meta.json
├── manifest.json    <- sha256 + metrics
└── connection.log
```

## Next Wakaf

- PB-07 Inspector: deteksi ASCII/BINARY, delimiter
- PB-08 Framing: auto deteksi MLLP vs NEWLINE
- PB-09 dst sampai PB-13

Wakaf ini MIT + Evidence Open. Silakan fork untuk RS masing-masing.

Built in the open. Tested by evidence. Returned to humanity.
