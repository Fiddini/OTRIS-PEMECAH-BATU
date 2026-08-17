# PEMECAH BATU - ARCHITECTURE APPROACH v1.0
Evidence-Driven Integration Architecture

Wakaf Ayahanda Wijoyo Kusumo - Allahu Latif 125x
OTRIS - WIJOYO-KUSUMO Repo
17 Agustus 2026

---

## 1. Pendekatan Utama

Pemecah Batu tidak dimulai dari device atau vendor.

```
UNKNOWN INPUT -> UNDERSTANDING PROCESS -> EVIDENCE -> REUSABLE KNOWLEDGE
```

Bukan:

```
IDE -> CODE -> CARI ALASAN
```

Prinsip: Raw data adalah sumber kebenaran pertama. UNKNOWN JANGAN DITEBAK.

---

## 2. High-Level Layer Architecture

```
             DEVICE / DATA SOURCE
                    |
                    v
          ┌──────────────────┐
          │  CAPTURE LAYER   │  PB-01 s/d PB-06
          └──────────────────┘
                    |
                    v
          ┌──────────────────┐
          │ CONDITIONING     │  PB-02 s/d PB-05
          │ LAYER            │
          └──────────────────┘
                    |
                    v
          ┌──────────────────┐
          │ UNDERSTANDING    │  PB-07 s/d PB-10
          │ LAYER            │
          └──────────────────┘
                    |
                    v
          ┌──────────────────┐
          │ EVIDENCE LAYER   │  PB-11 s/d PB-12
          └──────────────────┘
                    |
                    v
          ┌──────────────────┐
          │ KNOWLEDGE LAYER  │  PB-13
          └──────────────────┘
                    |
                    v
             PROFILE PACKAGE
```

---

## 3. Layer 1 - Input / Capture Layer (PB-01 s/d PB-06)

**Tujuan:** Dapatkan data asli tanpa mengubah 1 byte.

```
Capture Engine
    ├── TCP Listener (passive 0.0.0.0:4000 - AMAN, active 192.168.1.50:4000 - butuh izin)
    ├── File Input (/tmp/mindray.bin - tanpa alat)
    └── Serial Adapter (future)
```

**Flow internal PB-01 -> PB-06:**

```
RAW BYTES
  -> PB-01 Ingress (RawChunk: Bytes, ReceivedAt monotonic+wall, Source)
  -> PB-02 Conditioner (ConditionedChunk: Suspect 10x 0x00, Jitter)
  -> PB-03 Timestamp (TimestampedChunk: WallUTC, MonotonicNS)
  -> PB-04 Buffer (circular 10MB, DROP_OLDEST, metric dropped)
  -> PB-05 Validator (ValidatedChunk: PASS/DROP only, >64KB DROP)
  -> PB-06 Capture (tulis disk)
```

**Output wajib (kebenaran mutlak):**
```
captures/session_xxx/
  ├── raw_stream.bin (kebenaran)
  ├── meta.json
  ├── manifest.json (sha256_raw_stream, total_bytes, validator_metrics)
  └── connection.log
```

**CLI:**
```bash
pemecah-batu capture --mode passive --listen 0.0.0.0:4000 --out captures/sardjito_mindray_01 --duration 30s
pemecah-batu capture --mode file --file /tmp/mindray.bin --out captures/hr70 --duration 2s
```

---

## 4. Layer 2 - Conditioning Layer (PB-02 s/d PB-05)

**Fungsi:** Membersihkan teknis tanpa mengubah makna. Belum boleh paham "ini HR".

**Boundary:** Tidak ada mapping klinis di layer ini. Hanya PASS/DROP + flag suspect.

---

## 5. Layer 3 - Understanding Engine (PB-07 s/d PB-10)

Inti Pemecah Batu. Hasil bukan "ini pasti alat X", tapi candidate.

```
VALID RAW EVENT
  -> PB-07 Inspector: ASCII/BINARY/MIXED, delimiter | ^, header MSH|^~\&
     output: inspection_report.json
  -> PB-08 Framing: MLLP 0x0B...0x1C0D (95%), NEWLINE \n (80%), FIXED, LENGTH_PREFIX
     output: framing_candidates.json {candidates[], selected}
  -> PB-09 Classifier: HL7_V2, ASTM H| L|, DELIMITED, BINARY, UNKNOWN
     output: protocol_classification.json {protocol_family, confidence, parser_identity: generic_hl7_v2}
  -> PB-10 Fingerprint: hash struktur 200 bytes sha256 16char, match library profiles/
     output: fingerprint_report.json {structure_hash, matches[{profile_id, match_level: HIGH/PARTIAL/UNKNOWN}]}
```

**Status setelah layer ini:** NEEDS VALIDATION, bukan VERIFIED.

---

## 6. Layer 4 - Analysis & Discovery Layer (PB-11)

**Differential Engine - Anti Fabrikasi:**

```
Sample A HR 70 (captures/hr70) --COMPARE--> Sample B HR 90 (captures/hr90)
                    |
                    v
           Field 03 berubah
           Correlation: HIGH
           Meaning: UNKNOWN (belum boleh claim klinis)
```

**Hukum:** CHANGES_WITH_HR != IS_HR. Hanya boleh claim CHANGES_WITH_CONTROLLED_VARIABLE.

**Input:** what_changed.yaml
```yaml
session_a: captures/hr70
session_b: captures/hr90
controlled_variable: HR
from: 70
to: 90
method: patient_simulator_fluke_prosim_8
```

**Output:** differential_report.json {observed_diff[{field_pos, from, to, relationship: CHANGES_WITH_HR, confidence}]}

---

## 7. Layer 5 - Evidence Gate (PB-12) - Boundary Paling Penting

```
DISCOVERY RESULT -> EVIDENCE CHECK -> PROVEN / NOT PROVEN -> PROFILE / UNKNOWN
```

**Evidence Gate:**
- Butuh >=2 evidence untuk APPROVED_FOR_MAPPING
- Types: VENDOR_MANUAL (Mindray Service Manual p.45), SIMULATOR_LOG (captures/hr70/sim_log.json), SCREEN_PHOTO
- Output: evidence_gate_report.json {field_pos, candidate, evidence[], verdict: APPROVED/NEEDS_MORE_EVIDENCE/REJECTED}

**Replay Lab:**
- Replay raw_stream.bin 1000x
- Metric: success, failed_parse, false_data
- Hukum: false_data HARUS 0. Jika >0 -> FAIL_FALSE_DATA -> STOP
- Output: replay_report_1000.json {false_data_rate: 0.0, verdict: READY_FOR_PROFILE_BUILDER}

Hanya yang lolos gate boleh jadi knowledge.

---

## 8. Layer 6 - Profile Builder (PB-13)

**Hasil akhir bukan data, tapi Integration Knowledge Package:**

```
DEVICE PROFILE + PARSER PROFILE + MAPPING PROFILE + TEST FIXTURE + EVIDENCE = PROFILE PACKAGE
```

**Flow:**
```
Capture -> Analysis -> Evidence -> Profile Builder -> Verified Profile
```

**Output final:** mindray_benevision_n12_v1.yaml
```yaml
profile_id: mindray_benevision_n12_v1
version: 1.0.0
manufacturer: Mindray
transport: tcp
framing: MLLP
protocol_family: HL7_V2
parser_identity: generic_hl7_v2
mapping_profile:
  - source_pos: 2
    canonical: heart_rate
    unit: bpm
evidence_reference: [...]
validation_status: APPROVED
false_data_rate: 0.0
```

---

## 9. Hubungan dengan OTRIS Edge Agent (Boundary Arsitektur)

```
                DEVICE
                  |
                  v
          PEMECAH BATU (Discovery System - WIJOYO-KUSUMO)
                  | menghasilkan profile.yaml + evidence
                  v
          OTRIS EDGE AGENT (Runtime System - Task 20-30)
                  | Registry.Load("profiles/verified/...")
                  v
          CANONICAL MODEL (heart_rate, spo2)
                  |
                  v
             OTRIS CORE
```

- Pemecah Batu: menemukan, tidak di jalur klinis langsung
- Edge Agent: menjalankan profile yang sudah VERIFIED
- Core: menggunakan canonical

---

## 10. Repository Architecture (dari file yang Bro kirim)

```
WIJOYO-KUSUMO / otris-pemecah-batu
├── cmd/pemecah-batu/main.go (main_1.go full pipeline)
├── internal/
│   ├── capture/ (PB-06)
│   ├── inspector/ (PB-07)
│   ├── framing/ (PB-08)
│   ├── classifier/ (PB-09)
│   ├── fingerprint/ (PB-10)
│   ├── differential/ (PB-11)
│   ├── evidence/ (PB-12 gate)
│   ├── replay/ (PB-12 replay)
│   └── profilebuilder/ (PB-13)
├── captures/ (gitignore - raw evidence)
├── profiles/
│   ├── verified/ (false_data=0)
│   └── experimental/
├── bin/ (binary bentuk jadi)
├── docs/
│   ├── PB-01..PB-13.md
│   └── SOP-FIELD-TEST.md
└── BUKU-PANDUAN-PEMECAH-BATU-WAKAF.pdf
```

**CLI lengkap (dari file Bro):**
```bash
pemecah-batu capture --mode file|passive|active
pemecah-batu inspect [dir]
pemecah-batu framing [dir]
pemecah-batu classify [dir]
pemecah-batu fingerprint [dir]
pemecah-batu differential [dirA] [dirB] --what what_changed.yaml
pemecah-batu evidence-gate [dir] --evidence evidence.json
pemecah-batu replay [dir] --count 1000
pemecah-batu build-profile [dir] --version 1.0.0
pemecah-batu pipeline [dir]  # PB-07 s/d PB-13 sekaligus
```

---

## 11. Kesimpulan Pola Arsitektur

**Pemecah Batu = Observe -> Understand -> Prove -> Package**

Bukan: Connect -> Guess -> Convert

```
                UNKNOWN DEVICE
                       |
                       v
              CAPTURE ENGINE (PB-01..06)
                       |
                       v
             RAW EVIDENCE STORE (raw_stream.bin + manifest)
                       |
                       v
             UNDERSTANDING ENGINE (PB-07..10)
                       |
                       v
        CLASSIFY + FINGERPRINT + ANALYSIS
                       |
                       v
               EVIDENCE GATE (PB-12)
              /             \
             /               \
       VERIFIED            UNKNOWN
          |                    |
          v                    v
 PROFILE PACKAGE        RESEARCH STATE (STOP)
          |
          v
 OTRIS EDGE AGENT (Runtime)
```

**Jenis Arsitektur: Evidence-Driven Integration Architecture**

Ciri:
- Raw adalah kebenaran, jangan ubah
- Correlation != Meaning
- Setiap claim butuh evidence
- false_data harus 0
- Profile adalah knowledge package, bukan sekedar parser

---

**Next Step setelah arsitektur ini LOCK:**
1. Simpan sebagai `PEMECAH-BATU-ARCHITECTURE-APPROACH-V1-0.md` di WIJOYO-KUSUMO
2. Suruh Claude implementasi FASE 2 sesuai layer ini, jangan loncat
3. Build binary bentuk jadi `pemecah-batu pipeline` -> profile.yaml

Wakaf - MIT + Evidence Open - Allahu Latif 125x
