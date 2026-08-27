# Prompt 9router: Onboarding Wizard + Operator Public Profile

> **Constraint:** Ikuti arsitektur yang sudah ada — Go 3-layer (handler → service → repository),
> sqlc untuk semua query, buf generate untuk proto changes, Better Auth session di middleware.
> Jangan break fitur yang sudah ada. Semua query harus scope by operatorID.

---

## Context

### Kondisi Existing

**Onboarding (`/onboarding/page.tsx`):**
- Sudah ada 2-step flow: buat organisasi/operator (step 1) + buat season pertama (step 2)
- UI minimal — plain HTML `<input>` dan `<button>`, tidak pakai design system/CSS vars
- Tidak ada progress indicator, logo, deskripsi, atau WhatsApp
- Setelah step 2 selesai → redirect ke `/dashboard`

**Operator proto (`proto/hajj/v1/operator.proto`):**
- `Operator` message: id, better_auth_org_id, name, country, email, license_number, created_at, slug
- `ResolveOperatorSlug` RPC sudah ada dan public — returns operatorId, name, activeSeasonId
- Tidak ada field: logo_url, description, whatsapp_number, website, address

**Operators table:**
- Kolom existing: id, better_auth_org_id, name, country, email, license_number, created_at, slug
- Tidak ada: logo_url, description, whatsapp_number, website, address

---

## Task 1 — Migration: Tambah field profil publik ke operators

Buat file `apps/api/db/migrations/073_operator_public_profile.sql`:

```sql
-- +goose Up
ALTER TABLE operators
  ADD COLUMN IF NOT EXISTS logo_url       TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS description    TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS whatsapp_number TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS website        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS address        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS city           TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS is_profile_complete BOOLEAN NOT NULL DEFAULT FALSE;
  -- TRUE setelah operator selesai onboarding wizard step terakhir

-- +goose Down
ALTER TABLE operators
  DROP COLUMN IF EXISTS logo_url,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS whatsapp_number,
  DROP COLUMN IF EXISTS website,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS city,
  DROP COLUMN IF EXISTS is_profile_complete;
```

---

## Task 2 — sqlc queries baru

Tambahkan query baru di `apps/api/db/query/operator.sql`:

```sql
-- name: UpdateOperatorProfile :one
UPDATE operators SET
  logo_url         = $2,
  description      = $3,
  whatsapp_number  = $4,
  website          = $5,
  address          = $6,
  city             = $7,
  is_profile_complete = TRUE
WHERE id = $1 AND operator_id = $1
RETURNING *;
-- Note: operators.id IS the operator_id (operators are their own scope)
-- Correct query scoping:
-- WHERE id = $1

-- name: GetOperatorPublicProfile :one
SELECT
  o.id,
  o.name,
  o.slug,
  o.logo_url,
  o.description,
  o.whatsapp_number,
  o.website,
  o.address,
  o.city,
  o.license_number,
  o.country,
  o.is_profile_complete
FROM operators o
WHERE o.slug = $1;

-- name: GetActiveSeasonsByOperator :many
SELECT
  s.id,
  s.name,
  s.type,
  s.start_date,
  s.end_date,
  s.is_registration_open,
  COUNT(p.id) AS pilgrim_count
FROM seasons s
LEFT JOIN pilgrims p ON p.season_id = s.id AND p.operator_id = s.operator_id
WHERE s.operator_id = $1
  AND s.end_date >= NOW()
  AND s.is_registration_open = TRUE
GROUP BY s.id, s.name, s.type, s.start_date, s.end_date, s.is_registration_open
ORDER BY s.start_date ASC;
```

Jalankan `sqlc generate` dari `apps/api/` setelah query ditambahkan.

---

## Task 3 — Proto update

Edit `proto/hajj/v1/operator.proto`:

**Tambah field ke `Operator` message:**
```protobuf
message Operator {
  string id = 1;
  string better_auth_org_id = 2;
  string name = 3;
  string country = 4;
  string email = 5;
  string license_number = 6;
  google.protobuf.Timestamp created_at = 7;
  string slug = 8;
  // New fields:
  string logo_url = 9;
  string description = 10;
  string whatsapp_number = 11;
  string website = 12;
  string address = 13;
  string city = 14;
  bool is_profile_complete = 15;
}
```

**Tambah messages + RPC baru:**
```protobuf
message UpdateOperatorProfileRequest {
  string logo_url        = 1;
  string description     = 2;
  string whatsapp_number = 3;
  string website         = 4;
  string address         = 5;
  string city            = 6;
}

message GetPublicProfileRequest {
  string slug = 1 [(buf.validate.field).string.min_len = 1];
}

message PublicSeasonSummary {
  string id                 = 1;
  string name               = 2;
  string type               = 3;
  google.protobuf.Timestamp start_date = 4;
  google.protobuf.Timestamp end_date   = 5;
  int32  pilgrim_count      = 6;
}

message GetPublicProfileResponse {
  string operator_id     = 1;
  string name            = 2;
  string slug            = 3;
  string logo_url        = 4;
  string description     = 5;
  string whatsapp_number = 6;
  string website         = 7;
  string address         = 8;
  string city            = 9;
  string license_number  = 10;
  string country         = 11;
  repeated PublicSeasonSummary active_seasons = 12;
}

// Tambah ke OperatorService:
rpc UpdateMyProfile(UpdateOperatorProfileRequest) returns (Operator);
rpc GetPublicProfile(GetPublicProfileRequest) returns (GetPublicProfileResponse);
// GetPublicProfile adalah PUBLIC (unauthenticated) — tambahkan ke publicProcedures
// di internal/middleware/auth.go dan rateLimitedProcedures di ratelimit.go
```

Jalankan `buf generate` dari `proto/` setelah edit.

---

## Task 4 — Go: Service + Handler

### Service (`apps/api/internal/service/operator.go`)

**Tambah method `UpdateMyProfile`:**
```go
func (s *OperatorService) UpdateMyProfile(ctx context.Context, request *hajjv1.UpdateOperatorProfileRequest) (*hajjv1.Operator, error) {
    operatorID := middleware.OperatorIDFromCtx(ctx)
    updated, err := s.repository.UpdateOperatorProfile(ctx, db.UpdateOperatorProfileParams{
        ID:             operatorID,
        LogoUrl:        request.LogoUrl,
        Description:    request.Description,
        WhatsappNumber: request.WhatsappNumber,
        Website:        request.Website,
        Address:        request.Address,
        City:           request.City,
    })
    if err != nil {
        return nil, serviceError("OperatorService.UpdateMyProfile", err)
    }
    return operatorMessage(updated), nil
}
```

**Tambah method `GetPublicProfile`:**
```go
func (s *OperatorService) GetPublicProfile(ctx context.Context, request *hajjv1.GetPublicProfileRequest) (*hajjv1.GetPublicProfileResponse, error) {
    if request == nil || request.Slug == "" {
        return nil, serviceError("OperatorService.GetPublicProfile", apperror.ErrValidation)
    }
    operator, err := s.repository.GetOperatorPublicProfile(ctx, request.Slug)
    if err != nil {
        return nil, serviceError("OperatorService.GetPublicProfile", err)
    }
    seasons, err := s.seasonRepository.GetActiveSeasonsByOperator(ctx, operator.ID)
    if err != nil && !errors.Is(err, apperror.ErrNotFound) {
        return nil, serviceError("OperatorService.GetPublicProfile", err)
    }
    var seasonSummaries []*hajjv1.PublicSeasonSummary
    for _, s := range seasons {
        seasonSummaries = append(seasonSummaries, &hajjv1.PublicSeasonSummary{
            Id:          s.ID,
            Name:        s.Name,
            Type:        string(s.Type),
            StartDate:   timestamppb.New(s.StartDate.Time),
            EndDate:     timestamppb.New(s.EndDate.Time),
            PilgrimCount: int32(s.PilgrimCount),
        })
    }
    return &hajjv1.GetPublicProfileResponse{
        OperatorId:     operator.ID,
        Name:           operator.Name,
        Slug:           operator.Slug.String,
        LogoUrl:        operator.LogoUrl,
        Description:    operator.Description,
        WhatsappNumber: operator.WhatsappNumber,
        Website:        operator.Website,
        Address:        operator.Address,
        City:           operator.City,
        LicenseNumber:  operator.LicenseNumber.String,
        Country:        operator.Country.String,
        ActiveSeasons:  seasonSummaries,
    }, nil
}
```

### Handler (`apps/api/internal/handler/operator.go`)

Tambah dua handler untuk `UpdateMyProfile` dan `GetPublicProfile` — ikuti pola handler yang sudah ada.

### Middleware (`apps/api/internal/middleware/auth.go`)

Tambah ke `publicProcedures`:
```go
"/hajj.v1.OperatorService/GetPublicProfile": true,
```

### Ratelimit (`apps/api/internal/middleware/ratelimit.go`)

Tambah ke `rateLimitedProcedures`:
```go
"/hajj.v1.OperatorService/GetPublicProfile": true,
```

---

## Task 5 — Frontend: Onboarding Wizard Redesign

Edit `apps/web/app/onboarding/page.tsx` — **ganti seluruh isi file** dengan wizard 3-step yang proper.

**Step 1 — Profil Operator** (sudah ada, redesign UI):
- Nama perusahaan (required)
- Nomor izin PPIU/PIHK (optional)
- Negara (ISO-2, default ID)

**Step 2 — Detail Publik** (baru):
- Deskripsi singkat (textarea, max 300 karakter) — "Ceritakan sedikit tentang travel Anda"
- Nomor WhatsApp CS (format +62...)
- Kota kantor
- Website (optional)

**Step 3 — Musim Pertama** (sudah ada, redesign UI):
- Nama musim
- Jenis musim (dropdown)
- Tanggal mulai & selesai

**Design requirements:**
- Pakai CSS variables yang sudah ada: `--color-cream-*`, `--color-emerald-*`, `--color-gold-*`
- Step indicator di atas: tiga lingkaran bernomor dengan garis penghubung, step aktif highlight gold
- Card putih dengan shadow tipis, centered, max-width 560px
- Input styling konsisten dengan dashboard yang sudah ada
- Tombol "Lanjutkan" (step 1-2) dan "Selesai & Buka Dashboard" (step 3)
- Pada step 1: setelah `createOperator`, langsung call `UpdateMyProfile` untuk simpan description/whatsapp/city di step 2
- Atau simpan state lokal dan call semua sekaligus saat step 3 selesai — pilih yang lebih clean

**API calls:**
```ts
// Step 1 selesai:
authClient.organization.create(...)
authClient.organization.setActive(...)
operatorClient.createOperator(...)

// Step 2 selesai (simpan ke state, call di akhir):
// state: { description, whatsappNumber, city, website }

// Step 3 selesai:
seasonClient.createSeason(...)
operatorClient.updateMyProfile({ description, whatsappNumber, city, website })
router.push("/dashboard")
```

---

## Task 6 — Frontend: Dashboard Settings — Profile Editor

Tambah section baru di halaman settings operator (cek path existing, kemungkinan `/dashboard/settings` atau `/dashboard/operator`).

Buat komponen `OperatorProfileForm` dengan field:
- Logo URL (text input untuk URL, atau placeholder untuk upload nanti)
- Deskripsi (textarea)
- Nomor WhatsApp CS
- Website
- Alamat kantor
- Kota

Submit → call `operatorClient.updateMyProfile(...)` → toast sukses.

Tampilkan juga "Bagikan profil publik Anda:" dengan link `https://app.tawafiqhub.com/p/[slug]` + tombol copy.

---

## Task 7 — Frontend: Halaman Profil Publik

Buat file `apps/web/app/p/[slug]/page.tsx` — **Server Component** (Next.js App Router, bukan client component).

```
Route: /p/[slug]
Auth: tidak diperlukan — unauthenticated public page
Data: GetPublicProfile RPC (public procedure)
```

**Layout halaman:**

```
┌─────────────────────────────────────────┐
│  [Logo]  Nama Operator                  │
│          ★ Terpercaya · Kota · Negara   │
│          Nomor Izin: PPIU-XXX           │
├─────────────────────────────────────────┤
│  Tentang Kami                           │
│  [deskripsi]                            │
├─────────────────────────────────────────┤
│  Paket Tersedia                         │
│  ┌──────────────────┐  ┌─────────────┐  │
│  │ Umrah Ramadhan   │  │ Haji Plus   │  │
│  │ Mar 2027         │  │ Jun 2027    │  │
│  │ [pilgrim] jamaah │  │             │  │
│  │ [Daftar Sekarang]│  │             │  │
│  └──────────────────┘  └─────────────┘  │
├─────────────────────────────────────────┤
│  📱 Hubungi Kami                        │
│  [WhatsApp Button]  [Website]           │
└─────────────────────────────────────────┘
```

**Implementation notes:**
- Fetch data server-side menggunakan Connect client dengan transport `http://localhost:9100` (internal)
- Jika operator tidak ditemukan (slug tidak valid) → Next.js `notFound()`
- Jika `active_seasons` kosong → tampilkan "Belum ada paket tersedia saat ini"
- Tombol "Daftar Sekarang" → `/register/[operatorId]?season=[seasonId]`
- WhatsApp button → `https://wa.me/[whatsapp_number]` (strip non-digit, tambah `62` jika awalan `0`)
- Meta tags untuk SEO: title = nama operator, description = deskripsi operator
- Design: cream background, emerald accent, konsisten dengan landing page yang sudah ada

**Tambah ke `apps/web/middleware.ts` PUBLIC_PATHS:**
```ts
"/p/:slug*"
```

---

## Task 8 — Navigasi post-onboarding

Setelah operator selesai onboarding (redirect ke `/dashboard`), tampilkan banner sekali:

```
"Profil publik Anda sudah aktif di /p/[slug] — bagikan ke calon jamaah!"
[Lihat Profil] [Salin Link]
```

Kondisi tampil: `is_profile_complete === true` tapi belum pernah dismiss. Simpan dismiss state di `localStorage` key `profile_banner_dismissed`.

---

## Verifikasi

Setelah semua task selesai:

1. `buf generate` berhasil — tidak ada error proto
2. `sqlc generate` berhasil — tidak ada error query
3. `go build ./...` dari `apps/api/` berhasil
4. `pnpm --filter @hajj-saas/web typecheck` berhasil
5. Flow manual test:
   - Buat akun baru → masuk wizard onboarding → isi 3 step → redirect dashboard
   - Buka `/p/[slug-operator]` tanpa login → halaman profil tampil
   - Klik "Daftar Sekarang" → redirect ke `/register/[operatorId]`
   - Dashboard settings → edit profil → simpan → refresh `/p/[slug]` → perubahan tampil
