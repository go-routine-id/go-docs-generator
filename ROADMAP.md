# Roadmap: Docs Generator → Generator Dokumentasi API Universal

Status: **disepakati untuk dikerjakan bertahap dengan review gate per phase**.

Dokumen ini adalah sumber kebenaran arah pengembangan. Diubah hanya bila user menyetujui perubahan scope.

## Visi

Menjadikan `docs-generator` tool dokumentasi API yang bisa dipakai siapa saja — baik proyek monolithic maupun microservice — tanpa mengunci mereka pada konvensi Museum Digital Indonesia.

**Kasus motivasi utama:** satu repository dokumentasi bisa memuat banyak service dengan base URL berbeda (mis. `account-service`, `storage-service`), baik digabung dalam satu halaman maupun dipisah per project.

## Prinsip

1. **Satu sumber kebenaran**: JSON Schema mendefinisikan spec → dari situ diturunkan struct Go, validasi, dan dokumentasi.
2. **Extensibility via registry**, bukan via edit banyak file. Menambah doc-type baru = menambah 1 modul, bukan mengubah 4 tempat.
3. **Backward compatibility lewat migrator**, bukan dengan mempertahankan kode lama terus-menerus.
4. **Test dulu, refactor kemudian**. Phase 0 wajib selesai sebelum Phase 1 dimulai.
5. **Interop**: adopsi dipermudah dengan menerima dan menghasilkan format standar (OpenAPI, AsyncAPI).

## Phase 0 — Foundation

Tujuan: membuat refactor besar di phase berikutnya aman dan terukur.

- Pecah `pkg/docs/template.go` (2091 baris) menjadi file `web/templates/*.html`, `web/assets/*.css`, `web/assets/*.js` yang di-embed via `embed.FS`.
- Golden test untuk render HTML (fixtures `testdata/specs/*.yaml`).
- Unit test untuk `loader.go` (merge, discover, auto-include).
- Migrasi logging ke `log/slog` (human di dev, JSON di production).
- **Review gate** sebelum Phase 1.

Estimasi: 3–5 hari kerja fokus.

## Phase 1 — Spec sebagai Kontrak Pertama

Tujuan: membuat penambahan fitur baru murah dan aman.

- Definisikan `schemas/spec.schema.json` sebagai sumber kebenaran.
- Validasi saat load dengan pesan error line-aware (`file:line: field X required`).
- Pola **registry** untuk doc-type: tiap jenis (`Section`, `Guide`, `Screen`, `Permission`, …) adalah modul yang mendaftarkan struct, strategi merge, template partial, dan sidebar entry.
- **Per-section / per-endpoint `base_url`** (fitur konkret yang diminta user).
- Auto-generate `SPEC.md` dari schema.
- **Review gate** sebelum Phase 2.

Estimasi: 5–7 hari.

## Phase 2 — Interop

Tujuan: menghilangkan barrier adopsi bagi tim yang sudah punya dokumentasi.

- **Importer OpenAPI 3.1**: `docs-gen -openapi ./swagger.yaml`.
- **Exporter OpenAPI 3.1**: `/api/docs/openapi` (JSON/YAML).
- Type baru `Events` / `Webhooks` untuk async/event-driven (inspirasi AsyncAPI), mendaftar via registry Phase 1.
- **Review gate** sebelum Phase 3.

Estimasi: 5–7 hari.

## Phase 3 — Developer Experience

Tujuan: mempermudah penulis spec dan kustomisasi visual.

- Subcommand `docs-gen validate <path>` (CI-friendly).
- Subcommand `docs-gen init` (scaffold spec minimal).
- Auto-layout diagram (dagre/elk) saat `x/y` kosong. Manual tetap menang bila di-set.
- `theme.yaml` untuk branding (title, logo, warna, favicon) tanpa menyentuh template.
- **Review gate** sebelum Phase 4.

Estimasi: 3–5 hari.

## Phase 4 — Distribusi & Polish

Tujuan: siap rilis publik v2.0.0.

- Pindahkan `spec/` Museum ke `examples/museum/`; `spec/` jadi template generic (petstore-style).
- Rewrite README tanpa path/domain Museum-specific. Tambah section "Use cases" (mono vs micro).
- Docker image di ghcr, binary release lewat GitHub Releases.
- CHANGELOG + tag `v2.0.0`.

Estimasi: 2–3 hari.

## Catatan implementasi lintas-fase

- **Satu PR per fase** (bisa dipecah sub-PR bila perlu). Tidak mencampur perubahan lintas fase.
- Commit kecil, pesan fokus pada **why**.
- Setiap fase ditutup dengan ringkasan untuk user sebelum lanjut.
- Museum spec yang hidup di `spec/` selama Phase 0–3 tetap jadi acuan golden test; baru dipindah di Phase 4.

## Risiko diakui

- Phase 0 split template berpotensi konflik kalau ada kerja paralel di `template.go`.
- Registry pattern menambah abstraksi. Ongkos kecil untuk fitur pertama, hemat besar dari fitur ketiga ke atas.
- OpenAPI mapping tidak akan 100% lossless; perlu keputusan eksplisit soal field mana yang diturunkan.

## Yang *tidak* kami lakukan (non-goals)

- Membangun CMS untuk spec (tetap file-based).
- Menggantikan Swagger UI / Redoc secara head-to-head — fokus kami adalah narasi + interactive tester + arsitektur visual.
- Auth/authz terhadap halaman docs itu sendiri (tetap menjadi urusan reverse proxy).
