# Writing Specs — Aturan & Panduan

Dokumen ini adalah **panduan naratif** untuk penulis spec. Untuk referensi lengkap setiap field (auto-generated), lihat [`SPEC.md`](../SPEC.md). Untuk kontrak mesin, lihat [`schemas/spec.schema.json`](../schemas/spec.schema.json).

---

## Filosofi

1. **YAML adalah source of truth.** Tidak ada database, tidak ada build step. Edit file, reload browser.
2. **Narasi dulu, mesin kemudian.** Spec ini dioptimalkan untuk halaman dokumentasi yang dibaca manusia — bukan kontrak wire format seperti OpenAPI. (OpenAPI bisa di-import & di-export, lihat bagian [Interop](#interop-dengan-openapi).)
3. **Additive-first.** Spec dipecah ke banyak file dan di-merge secara kumulatif — setiap file menambah, jarang ada yang menghapus.

---

## Tiga mode penggunaan

| Mode | Kapan dipakai | `-spec` pointing ke |
|------|----------------|----------------------|
| **Single-file** | Prototipe, API kecil | `./api.yaml` (satu file lengkap) |
| **Multi-file (directory)** | API menengah, banyak kontributor | `./spec/index.yaml` atau `./spec/` (dir) |
| **Multi-project** | Internal dev portal, banyak service terpisah | `./projects/` (dir berisi subdir per project) |

### Single-file

```yaml
# api.yaml
info:
  title: My API
  version: "1.0"
sections:
  - id: users
    title: Users
    endpoints: [...]
```

Jalankan: `docs-gen -spec ./api.yaml`.

### Multi-file

```
spec/
├── index.yaml            # info, authentication, flow_overview, theme, api_tester_defaults
├── sections/
│   ├── users.yaml        # sections: [...]
│   └── orders.yaml       # sections: [...]
├── guides/
│   └── checkout.yaml     # guides: [...]
└── screens/
    └── dashboard.yaml    # screens: [...]
```

Jalankan: `docs-gen -spec ./spec/index.yaml` (atau `-spec ./spec`). Semua `.yaml`/`.yml` di bawah direktori akan di-merge.

### Multi-project

```
projects/
├── index.yaml                  # project default
├── account/
│   ├── index.yaml              # project "account"
│   └── sections/*.yaml
└── storage/
    ├── index.yaml              # project "storage"
    └── sections/*.yaml
```

Jalankan: `docs-gen -spec ./projects/`. Akses: `/docs` (default), `/docs?p=account`, `/docs?p=storage`. Subdirektori tanpa `index.yaml` **bukan** project — diabaikan dari daftar.

---

## Aturan merge (multi-file)

Tiga aturan yang berlaku rekursif:

1. **Slice di-append.** Setiap file menambah ke daftar yang sama. Tidak ada deduplikasi — hindari mendefinisikan ID yang sama di dua file.
2. **Object di-merge per-field.** Overlay non-zero mengoverride base; zero value **tidak** pernah menghapus base.
3. **Scalar overlay menang saat non-zero.** String kosong, 0, false, nil — semuanya tidak menggeser base.

### Konsekuensi praktis

```yaml
# index.yaml
info:
  title: Platform API
  version: "2.0"

# sections/users.yaml  (overlay)
info:
  description: Tambahan deskripsi   # description ditambahkan
sections:
  - id: users
    title: Users                     # section baru di-append
```

Hasil merge: `title=Platform API`, `version=2.0`, `description="Tambahan deskripsi"`, `sections=[users]`.

**Jebakan:** kalau overlay sengaja ingin me-`null`-kan field — tidak bisa. Zero value = "diam". Hapus field di base kalau mau benar-benar kosong.

---

## Anatomi spec

### `info` — identitas & environment

```yaml
info:
  title: Platform API
  version: "2.0.0"
  description: API gateway untuk seluruh layanan platform.
  base_url: https://api.example.com         # fallback URL input tester
  base_urls:                                 # dropdown environment global
    - label: Production
      url: https://api.example.com
      default: true
    - label: Staging
      url: https://staging.example.com
    - label: Local
      url: http://localhost:8080
  overview_cards:                            # kartu di halaman overview
    - icon: "🚀"
      title: REST API
      description: Ringkasan dalam satu kalimat
      content: |
        Markdown **full** — paragraph, `code`, list, dll.
```

- `base_url` adalah string tunggal. Dipakai untuk mengisi URL input di API tester saat environment selector tidak ada.
- `base_urls` adalah array (label + URL + default). Muncul sebagai dropdown environment.
- Setidaknya salah satu dari `base_url` / `base_urls` perlu di-set kalau kamu ingin API tester bisa kirim request ke default yang masuk akal.

### `authentication` — metode auth

```yaml
authentication:
  methods:
    - type: Bearer JWT
      header: Authorization
      format: "Bearer <token>"
      description: Login via auth service untuk mendapat JWT
      source: auth-service
      token_contains: [user_id, permissions]

    - type: API Key
      header: X-API-Key
      format: "<api_key>"
      description: Untuk service-to-service dan CI jobs
      note: Otomatis di-exchange ke JWT di backend (cached 1 jam)
```

Tiap method di-render sebagai kartu di halaman Authentication. `note` muncul sebagai catatan tambahan. `token_contains` bikin list badge.

### `sections` — grup endpoint

```yaml
sections:
  - id: users
    title: Users
    description: Manajemen user dan profil

    # (opsional) override base URL khusus section ini
    base_url: https://users.example.com
    base_urls:
      - label: Users Prod
        url: https://users.example.com
        default: true
      - label: Users Staging
        url: https://staging.users.example.com

    endpoints:
      - name: List users
        method: GET
        path: /v1/users
        auth: JWT                    # bebas: none, JWT, API Key, ... (string label saja)
        permission: users:read       # (opsional) badge permission
        description: Ambil daftar user dengan pagination
        query_params:
          - name: limit
            type: integer
            required: false
            default: "20"
            description: Jumlah item per page
          - name: cursor
            type: string
            required: false
            description: Cursor untuk halaman berikutnya
        body: []                     # untuk GET kosongkan
        example_response: |
          {
            "users": [...],
            "next_cursor": "abc"
          }

      - name: Create user
        method: POST
        path: /v1/users
        auth: JWT
        permission: users:write
        description: Register user baru
        body:
          - name: email
            type: string
            required: true
            description: Email (harus unik)
            example: alice@example.com
          - name: name
            type: string
            required: true
        example_body: |
          {
            "email": "alice@example.com",
            "name": "Alice"
          }
        example_response: |
          {
            "id": "usr_123",
            "email": "alice@example.com"
          }
```

**Konvensi `auth`:** string bebas — apa yang kamu tulis akan muncul apa adanya di badge endpoint. Gunakan label yang konsisten di seluruh spec (`JWT`, `API Key`, `none`, atau gabung `JWT | API Key`).

**`id` section:** harus URL-friendly (lowercase, no space). Digunakan sebagai anchor.

### `guides` — alur multi-endpoint

Guide menjelaskan alur lintas endpoint, misalnya "upload file → lampirkan ke artifact":

```yaml
guides:
  - id: file_upload
    icon: "📤"
    title: File Upload
    description: Dua langkah — upload ke Media Service, lalu simpan URL
    flow:
      - step: 1
        title: Upload file ke Media Service
        description: Multipart request ke /media/upload
        endpoint:
          method: POST
          path: /upload
          service: media-service
          content_type: multipart/form-data
          auth: JWT
          permission: media:upload
          fields:
            - name: file
              type: binary
              required: true
              description: Binary file (max 10MB)
            - name: folder
              type: string
              required: false
              description: Target folder
        curl_example: |
          curl -X POST https://media.example.com/upload \
            -H "Authorization: Bearer $TOKEN" \
            -F "file=@photo.jpg"
        response_example: |
          {
            "url": "https://media.example.com/u/abc123.jpg",
            "media_id": "med_abc123"
          }

      - step: 2
        title: Simpan URL ke resource
        description: Update artifact dengan media URL
        endpoint:
          method: PATCH
          path: /artifacts/:id
          auth: JWT
          permission: artifact:write
          fields:
            - name: image_url
              type: string
              required: true
        actions:                       # tautan ke endpoint existing
          - type: link
            description: Lihat detail endpoint
            endpoint: "#update-artifact"
```

### `screens` — dokumentasi halaman frontend/mobile

Untuk tim yang ingin mendokumentasikan halaman UI dan API call yang diperlukan halaman itu:

```yaml
screens:
  - id: dashboard
    icon: "📊"
    title: Dashboard
    description: Halaman utama setelah login
    image: /screenshots/dashboard.png   # opsional
    platform: [Web, Mobile]
    calls:
      - method: GET
        path: /v1/users/me
        purpose: Ambil info user saat ini
        trigger: On mount
        auth: required
        notes: Cache di client untuk 5 menit
      - method: GET
        path: /v1/notifications
        purpose: Load notifikasi terbaru
        trigger: On mount
        auth: required
```

### `events` — channel async

Untuk mendokumentasikan Kafka, AMQP, MQTT, webhook, pub/sub, dll:

```yaml
events:
  - id: user-signup
    title: User Signup
    description: Fired ketika user menyelesaikan registrasi
    protocol: kafka
    address: user.signup.v1           # nama topic/queue/URL
    operations:
      - type: publish                  # atau: subscribe
        summary: Notifikasi user baru
        description: Di-publish oleh auth-service setelah verifikasi email
        payload:
          - name: user_id
            type: string
            required: true
            description: ID user yang baru register
          - name: email
            type: string
            required: true
          - name: signup_source
            type: string
            required: false
            description: web | mobile | api
        example: |
          {
            "user_id": "usr_abc",
            "email": "alice@example.com",
            "signup_source": "web"
          }

  - id: payment-webhook
    title: Payment Status Webhook
    protocol: webhook
    address: POST https://your-app.example.com/webhooks/payment
    operations:
      - type: publish
        summary: Status update transaksi
        payload:
          - name: transaction_id
            type: string
            required: true
          - name: status
            type: string
            required: true
            description: pending | success | failed
```

`protocol` adalah string bebas — `kafka`, `amqp`, `mqtt`, `nats`, `webhook`, `sse`, `websocket`, dst. `type` sebaiknya `publish` atau `subscribe` (dari perspektif service yang didokumentasikan).

### `flow_diagram_nodes` + `flow_diagram_edges` — diagram arsitektur

```yaml
flow_diagram_nodes:
  - id: auth
    label: "🔐 Auth Service"
    type: service
    color: "#4f46e5"
    position: { x: 100, y: 50 }        # opsional — kosongi untuk auto-layout
  - id: api-gateway
    label: "🚪 API Gateway"
    type: service
    color: "#06b6d4"
    position: { x: 300, y: 50 }

flow_diagram_edges:
  - source: api-gateway
    target: auth
    label: "verify"
    animated: true
    color: "#4f46e5"
    style: dashed                       # atau kosong untuk solid
```

**Auto-layout:** kalau **semua** node punya `x: 0, y: 0` (atau `position` dihapus), dagre akan menghitung layout otomatis. Kalau **ada satu pun** node dengan koordinat ≠ 0, semua node di-render apa adanya (layout manual menang). Cocok untuk MVP: hapus semua `position` dulu, tuning manual setelah diagram mulai besar.

### `api_tester_defaults` — default tester

```yaml
api_tester_defaults:
  methods: [GET, POST, PATCH, DELETE, PUT]
  auth_modes:
    - name: JWT Bearer
      header: Authorization
      prefix: "Bearer "
      placeholder: YOUR_JWT_TOKEN_HERE
    - name: API Key
      header: X-API-Key
      prefix: ""
      placeholder: YOUR_API_KEY_HERE
```

`auth_modes` muncul sebagai dropdown di tester dan sebagai input di halaman Credentials.

### `theme` — branding

```yaml
theme:
  title: My Company Docs
  logo_icon: "🏢"                       # emoji atau string pendek
  logo_image: /assets/logo.svg          # URL gambar (prioritas di atas logo_icon)
  primary_color: "#ff6600"              # override CSS --primary
  favicon: /favicon.ico
```

Semua field opsional. Kalau `theme.title` kosong, fallback ke `info.title`. Kalau `logo_icon` dan `logo_image` dua-duanya kosong, pakai `📖` default.

### `permissions` — katalog permission

```yaml
permissions:
  - name: users:read
    description: Baca data user
  - name: users:write
    description: Buat/update user
  - name: media:upload
    description: Upload ke media storage
```

Dipakai sebagai "kamus" — ditampilkan di halaman overview. Tidak ada validasi silang otomatis antara ini dan `endpoints[].permission`, jadi pastikan kamu konsisten sendiri.

### `constraints` — rules / invariants

```yaml
constraints:
  - Satu organization hanya boleh punya satu project
  - File upload harus via Media Service, tidak langsung ke object storage
  - Token expiry 1 jam, refresh via /auth/refresh
```

Muncul sebagai list di halaman overview.

---

## Pattern: per-section base URL (monolith ⟷ microservice)

### Monolith

Semua endpoint di satu backend. Cukup set `info.base_urls` — semua section pakai yang sama.

```yaml
info:
  base_urls:
    - label: Prod
      url: https://api.example.com
      default: true
sections:
  - id: users
    title: Users
    endpoints: [...]
  - id: orders
    title: Orders
    endpoints: [...]
```

### Microservice (multiple services, satu halaman docs)

Tiap section menjelaskan satu service dengan backend-nya sendiri:

```yaml
info:
  title: Platform API
  # (opsional) global default kalau ada endpoint yang shared
  base_urls:
    - label: Gateway
      url: https://api.example.com
      default: true

sections:
  - id: account
    title: Account Service
    base_url: https://account.example.com
    base_urls:
      - label: Prod
        url: https://account.example.com
        default: true
      - label: Staging
        url: https://staging.account.example.com
    endpoints: [...]

  - id: storage
    title: Storage Service
    base_url: https://storage.example.com
    endpoints: [...]
```

**Perilaku:** dropdown environment global di header **tidak** mempengaruhi section yang punya `base_url*` sendiri (attribute `data-uses-global="false"`). Section yang tidak override tetap ikut global selector.

### Microservice (tiap service dokumentasi terpisah)

Kalau tiap service punya tim dan schedule release sendiri, gunakan **multi-project**:

```
dev-portal/
├── account/index.yaml + files...
├── storage/index.yaml + files...
└── payments/index.yaml + files...
```

Pakai `docs-gen -spec ./dev-portal/` — pengunjung pilih project dari switcher.

---

## Interop dengan OpenAPI

### Import — pakai swagger.yaml existing

```bash
docs-gen -spec ./swagger.yaml
```

Auto-detect via key `openapi:` di top-level. Mapping:

| OpenAPI | → | Internal |
|---------|---|----------|
| `info.title/version/description` | | `info.*` |
| `servers[]` | | `info.base_urls[]` |
| `tags[]` | | satu `section` per tag |
| `paths.*` dengan `tags[0]` | | `endpoint` di section |
| `components.securitySchemes.*` | | `authentication.methods[]` |
| `parameters` (query) | | `endpoint.query_params` |
| `requestBody` (JSON) | | `endpoint.body` |
| `security` per op | | `endpoint.auth` |

Operation tanpa tag → masuk ke section "default".

### Export — jadikan OpenAPI untuk tooling lain

Server expose `/docs/openapi` yang mengembalikan OpenAPI 3.0 JSON. Berguna untuk Postman, Insomnia, Redocly.

```bash
curl https://your-docs.example.com/docs/openapi > openapi.json
```

Mapping reverse — beberapa info yang unik di internal model (guides, screens, events, theme) tidak punya representasi di OpenAPI dan akan hilang di output.

---

## Konvensi & best practices

### ID naming

- Gunakan **lowercase**, **kebab-case** atau **snake_case** konsisten.
- ID harus unik per tipe (section id, guide id, screen id, event id). Tidak ada validasi otomatis — audit manual.
- ID dipakai sebagai HTML anchor — hindari karakter spesial.

### Field `description`

- Single-line description: kalimat lengkap, ada titik.
- Multi-line content (`overview_cards[].content`, `ep.example_body`, dll): gunakan YAML block literal `|` supaya newline dipertahankan.
- `description` endpoint mendukung markdown minimal: **bold**, *italic*, `code`, bullet `-`, heading `#`.

### Optional vs required

Semua top-level field di `APISpec` sekarang **optional** (lihat `omitempty` di struct). Yang "required" adalah yang kamu butuhkan untuk halaman yang bermakna — tidak ada artinya menampilkan halaman dengan `info.title: ""`.

Per-endpoint: `name`, `method`, `path`, `description` praktis wajib; sisanya sesuai kebutuhan.

### Hindari

- ❌ Mendefinisikan section dengan `id` yang sama di dua file overlay — hasilnya section duplikat.
- ❌ Menggunakan `auth: required` padahal di tempat lain kamu pakai `auth: JWT`. Pilih satu konvensi.
- ❌ Mengandalkan overlay zero value untuk "menghapus" field dari base — tidak akan terjadi.
- ❌ Hardcode URL production di `example_body` atau `curl_example` kalau kamu sudah pakai `base_urls` — pakai placeholder saja.

### Recommended

- ✅ Set `# yaml-language-server: $schema=../schemas/spec.schema.json` di baris pertama tiap file — IDE auto-complete + lint otomatis.
- ✅ Commit `SPEC.md` dan `schemas/spec.schema.json` ke repo — CI workflow kami cek kalau stale.
- ✅ Validasi setiap PR via `docs-gen validate ./spec/index.yaml`.
- ✅ Mulai dengan single-file, split jadi multi-file saat >300 baris atau >5 sections.

---

## Validasi

### Sebelum commit

```bash
docs-gen validate ./spec/index.yaml
# ok: ./spec/index.yaml     (rc=0)
# atau
# invalid: <error message>  (rc=1)
```

### Di CI (GitHub Actions)

```yaml
- name: Validate spec
  run: go run ./cmd/server validate ./spec/index.yaml
```

`validate` melakukan: parse YAML, merge multi-file, cek struct types. Belum melakukan semantic check (ID unik, reference antar section, dll) — PR welcome.

### Regenerate SPEC.md saat ubah struct Go

```bash
make generate
# atau
go run ./cmd/gendocs
```

CI menolak merge kalau `SPEC.md` / `schemas/spec.schema.json` tidak ter-commit setelah perubahan struct.

---

## FAQ

**Q: Bagaimana cara meng-hide section dari sidebar tanpa menghapus?**
A: Belum ada field `hidden:`. Workaround: pindahkan `endpoints:` keluar dari section itu (section kosong tetap muncul dengan 0 endpoint). PR welcome untuk field `hidden: bool`.

**Q: Bisa pakai variable / include?**
A: Belum. YAML 1.2 anchor/alias (`&`, `*`) works di dalam satu file. Untuk cross-file, pecah section logis ke file tersendiri.

**Q: Bagaimana menyamakan API tester credentials lintas environment?**
A: Credentials disimpan per `base_url` di `localStorage`. Ganti environment = ganti credentials. Ini by design — production token tidak seharusnya pernah sampai di staging dropdown secara tidak sengaja.

**Q: Apakah perubahan ke spec require restart server?**
A: Tidak dalam `-dev` mode — ada file watcher (2s polling) yang reload otomatis. Production mode perlu restart.

**Q: Spec saya 3000 baris di satu file — apakah ada batasan?**
A: Tidak ada. Tapi pemecahan ke multi-file memudahkan PR review dan mengurangi konflik merge antar tim.

---

## Lihat juga

- [`SPEC.md`](../SPEC.md) — referensi field lengkap (auto-generated)
- [`schemas/spec.schema.json`](../schemas/spec.schema.json) — JSON Schema Draft 2020-12
- [`examples/museum/`](../examples/museum/) — spec lengkap sebagai referensi
- [`ROADMAP.md`](../ROADMAP.md) — rencana pengembangan
- [`CHANGELOG.md`](../CHANGELOG.md) — history perubahan & migration notes
