# WIJOYO KUSUMO

**PEMECAH BATU adalah WAKAF — BUKAN UNTUK DIJUAL — BUKAN ASET BERSAMA**

**Mesin pengetahuan integrasi untuk OTRIS — Pemecah Batu HL7 QCMS**

> *"Jangan pindahkan pasien, tapi hadirkan spesialis."*
> Keahlian yang hadir, bukan pasien yang dipindahkan.

**Prinsip kerja: DO NOT FABRICATE.** Data yang tidak memiliki bukti dinyatakan *NOT VERIFIED*, bukan diisi dengan dugaan.

---

## Ikrar Wakaf Pemecah Batu

Saya, **Hanif Ahdi Fiddini**, mewakafkan **PEMECAH BATU** — OTRIS Pemecah Batu HL7 QCMS, beserta kode, protokol Baiat, Amanah WIJOYO KUSUMO, Verifikasi, Audit / Berita Acara, Guard, dengan tahapan PB 8-10-12-15, di mana PB 8 + Protokol Lengkap = SUDAH BISA / GOVERNED-8.

Wakaf ini untuk Rumah Sakit, Klinik, dan umat. **Bukan aset bersama, bukan milik pribadi, bukan untuk dijual.**

- **Wakif:** Hanif Ahdi Fiddini
- **Nadzir:** Hanif Ahdi Fiddini
- **Saksi 1:** Istri
- **Saksi 2:** Anak
- **Tanggal:** 14 November 2026
- **Tempat:** Pekanbaru

Dokumen asli Ikrar disimpan oleh Wakif dan Nadzir, ditandatangani basah. Yang di repository ini hanya teksnya.

---

## Tentang

WIJOYO KUSUMO adalah mesin pengetahuan integrasi (*integration-knowledge engine*) dalam ekosistem **OTRIS**. Tujuannya menghubungkan perangkat medis yang sudah ada di rumah sakit, dari merek apa pun, tanpa memaksa rumah sakit membeli peralatan baru atau beralih ke satu merek.

Nama sebelumnya adalah **PEMECAH BATU**. Sebagian folder, berkas, dan branch masih memakai nama lama tersebut (misalnya `OTRIS-PEMECAH-BATU-TANPA-JETSON`).

Proyek ini adalah **wakaf**, dipersembahkan untuk Ayahanda Wijoyo Kusumo.

## Struktur Repository

### Kode dan skrip

| Path | Isi |
|---|---|
| `OTRIS-PEMECAH-BATU-TANPA-JETSON/` | Kode sumber utama Pemecah Batu, versi tanpa Jetson |
| `OTRIS-PEMECAH-BATU-TANPA-JETSON.zip` | Arsip rilis final versi tanpa Jetson |
| `scripts/` | Skrip pendukung, termasuk orkestrator uji E2E artefak Edge |

### Spesifikasi dan arsitektur

| Dokumen | Isi |
|---|---|
| `WIJOYO_KUSUMO_FINAL_SPECIFICATION.md` / `.pdf` | Spesifikasi final WIJOYO KUSUMO |
| `PEMECAH-BATU-ARCHITECTURE-APPROACH-V1-0.md` | Pendekatan arsitektur Pemecah Batu v1.0 |

### Laporan fase pengembangan

| Dokumen | Topik |
|---|---|
| `PHASE_1_EXPLORATION_REPORT.md` | Fase 1: eksplorasi |
| `PHASE_3A_ORCHESTRATION_INSPECTION_REPORT.md` | Fase 3A: inspeksi orkestrasi |
| `PHASE_3B_REVISED_DESIGN_SPECIFICATION.md` | Fase 3B: spesifikasi desain revisi |
| `PHASE_4A_TEMPORAL_QC_INSPECTION_REPORT.md` | Fase 4A: inspeksi QC temporal |
| `PHASE_5A_SEMANTIC_STATISTICAL_AUTHORITY_INSPECTION.md` | Fase 5A: inspeksi otoritas semantik dan statistik |

### Laporan audit dan verifikasi

| Dokumen | Topik |
|---|---|
| `MASTER-AUDIT-POSITION-REPORT.md` | Posisi audit induk |
| `OTRIS_PASS_1_2_3_FULL_EXISTING_RESULTS.md` | Hasil lengkap PASS 1, 2, dan 3 terhadap kode yang sudah ada |
| `.D3K-1H-C-FINAL-REPORT.md` | Laporan final D3K-1H-C |
| `.D3K-1H-C-WIRE-LOCK-REPORT.md` | Laporan wire-lock D3K-1H-C |

> Catatan: dua laporan D3K diawali titik (`.`), sehingga tersembunyi di sebagian penjelajah berkas.

### Halaman dan buku

| Berkas | Isi |
|---|---|
| `Otris-Open-Source-Landing.html` | Halaman landing OTRIS Open Source |
| `Otris-Pemecah-Batu-Landing-Warna-Kehidupan.html` | Halaman landing Pemecah Batu, tema *Warna Kehidupan* |
| `Pemecah-Batu-Book-50.html` | Buku *Pemecah Batu* |

## Riwayat Kemampuan Utama

Berdasarkan riwayat commit:

- **Mapping:** pembangun klaim produksi (*production claim builder*) dengan akses CLI dan perilaku *fail-closed*
- **QC:** QC terkelola (*governed QC*) terintegrasi ke pipeline produksi
- **QC temporal:** kecerdasan QC temporal dan riwayat diagnostik
- **Governance:** digest bukti semantik terikat pada proses admisi
- **Pengujian:** WIJOYO ditambahkan ke orkestrator artefak Edge (E2E)

## Prinsip Rekayasa

Setiap perubahan mengikuti urutan berikut:

1. **Existing Code** — audit kode yang sudah ada terlebih dahulu
2. **Evidence** — kumpulkan bukti
3. **Gap Classification**
   - **A** — sudah ada, belum tersambung
   - **B** — sudah ada, belum lolos
   - **C** — terbukti belum ada
   - **D** — ketidakcocokan kontrak
4. **Minimal Delta** — perubahan sekecil mungkin
5. **Test**
6. **Neutral Review**
7. **Founder Approval**

Pekerjaan yang sudah tercatat **PASS** dikunci dan tidak dibuka ulang tanpa keputusan Founder.

## Bahasa

Go · HTML · Shell

## Lisensi

Lihat [`LICENSE.md`](LICENSE.md).

Wakaf ini di atas lisensi. Lisensi mengatur cara pakai, Wakaf mengatur kepemilikan. Kode boleh dipakai, tapi tidak boleh dijual, tidak boleh diakui sebagai aset bersama, dan tidak boleh dihapus amanahnya.

## Pendiri

**Hanif Ahdi Fiddini** — Founder OTRIS

---

*Wakaf Ayahanda Wijoyo Kusumo.*
*PEMECAH BATU adalah WAKAF — BUKAN UNTUK DIJUAL — BUKAN ASET BERSAMA.*
