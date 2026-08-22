# Spesifikasi Lengkap: Group, Kloter, Muttawwif & Cascade Update System
## Tawafiq Hub — Hajj & Umrah Operator SaaS

> **Dokumen ini mencakup:** Business requirements, data model, API surface, UI/UX spec,
> lifecycle jamaah end-to-end, dan event-driven cascade update system.

---

## 1. Business Context

### Siapa yang terlibat?

| Peran | Definisi | Platform |
|---|---|---|
| **Operator** | Travel Hajj/Umrah yang mengelola seluruh perjalanan | Dashboard web |
| **Tour Leader (Agent)** | Pendamping dari Indonesia, ikut berangkat bersama jamaah | `/agent` portal |
| **Muttawwif** | Pemandu lokal di Saudi, menerima rombongan di sana | `/leader` app |
| **Jamaah** | Peserta ibadah | `/pilgrim` app |
| **Keluarga Jamaah** | Ingin tahu kondisi jamaah dari Indonesia | Family Tracker (public) |

### Hierarki Operasional

```
Season (Musim Haji/Umrah)
  └── Kloter (batch penerbangan, ~450 jamaah)
        ├── Tour Leader (Agent) — 1 per kloter, dari Indonesia
        ├── Kepala Kloter — dari operator
        └── Group (~45 jamaah per group, 10 group per kloter)
              ├── Muttawwif — 1 per group, pemandu lokal Saudi
              └── Jamaah (45 orang)
```

### Masalah yang Diselesaikan

- Operator tidak tahu real-time posisi rombongan mereka di Saudi
- Muttawwif tidak punya tools digital — masih pakai WhatsApp manual
- Tidak ada tracking ritual → jamaah tidak tahu progress manasiknya
- Kepulangan kacau → jamaah ketinggalan bus, salah gate, tidak ada di manifest
- Keluarga di Indonesia panik karena tidak dapat kabar
- Satu update harus diketik ulang ke banyak channel — tidak ada sumber tunggal kebenaran

---

## 2. Lifecycle Jamaah (End-to-End)

### Status Journey

```
[1]  REGISTERED           Jamaah terdaftar, dokumen lengkap
[2]  DOCUMENT_VERIFIED    Paspor + visa + vaksin terverifikasi
[3]  PRE_DEPARTURE        Briefing, manasik, packing
[4]  DEPARTED_INDONESIA   Sudah boarding di bandara Indonesia
[5]  IN_TRANSIT           Transit (Dubai, Doha, dll) — opsional
[6]  ARRIVED_SAUDI        Tiba di Saudi (Madinah atau Makkah)
[7]  IN_MADINAH           Ziarah Madinah
[8]  IN_MAKKAH            Umrah Qudum, persiapan Haji
[9]  IN_ARAFAH            Wuquf — puncak ibadah Haji
[10] IN_MUZDALIFAH        Mabit Muzdalifah
[11] IN_MINA              Mabit Mina, Lempar Jumrah
[12] BACK_IN_MAKKAH       Tawaf Ifadah, Sa'i, Tawaf Wada
[13] PRE_DEPARTURE_SAUDI  Persiapan kepulangan
[14] DEPARTED_SAUDI       Boarding di bandara Saudi
[15] IN_TRANSIT_RETURN    Transit kepulangan — opsional
[16] ARRIVED_INDONESIA    Tiba di Indonesia → sertifikat otomatis UNLOCK
[17] COMPLETED            Selesai
```

### Lifecycle Umrah (lebih pendek)

```
REGISTERED → DOCUMENT_VERIFIED → PRE_DEPARTURE → DEPARTED_INDONESIA
→ ARRIVED_SAUDI → IN_MAKKAH → [IN_MADINAH] → PRE_DEPARTURE_SAUDI
→ DEPARTED_SAUDI → ARRIVED_INDONESIA → COMPLETED
```

### Siapa yang Update Status

| Transisi | Siapa |
|---|---|
| REGISTERED → DOCUMENT_VERIFIED | Operator |
| PRE_DEPARTURE → DEPARTED_INDONESIA | Tour Leader / Operator (bulk per kloter) |
| ARRIVED_SAUDI → IN_MADINAH/IN_MAKKAH | Muttawwif (update lokasi group → cascade) |
| Perpindahan antar kota Saudi | Muttawwif (update lokasi group → cascade) |
| PRE_DEPARTURE_SAUDI → DEPARTED_SAUDI | Tour Leader (konfirmasi boarding) |
| DEPARTED_SAUDI → ARRIVED_INDONESIA | Operator / Tour Leader |
| ARRIVED_INDONESIA → COMPLETED | Otomatis (sistem) |

---

## 3. Data Model

### 3a. Tabel `kloters` (update)

```sql
-- Migration: 057_kloter_enhancement.sql
ALTER TABLE kloters
  ADD COLUMN IF NOT EXISTS season_id        UUID NOT NULL REFERENCES seasons(id),
  ADD COLUMN IF NOT EXISTS embarkasi        TEXT NOT NULL DEFAULT '',
  -- Kode embarkasi: SOC / JKG / SUB / BPN / UPG / dll
  ADD COLUMN IF NOT EXISTS flight_out       TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS flight_out_date  DATE,
  ADD COLUMN IF NOT EXISTS flight_in        TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS flight_in_date   DATE,
  ADD COLUMN IF NOT EXISTS kepala_kloter_id TEXT REFERENCES "user"(id),
  ADD COLUMN IF NOT EXISTS tour_leader_id   UUID REFERENCES agents(id),
  ADD COLUMN IF NOT EXISTS status           TEXT NOT NULL DEFAULT 'DRAFT',
  ADD COLUMN IF NOT EXISTS capacity         INT NOT NULL DEFAULT 450,
  ADD COLUMN IF NOT EXISTS notes            TEXT NOT NULL DEFAULT '';

ALTER TABLE kloters ADD CONSTRAINT kloters_status_check
  CHECK (status IN ('DRAFT','CONFIRMED','DEPARTED','IN_SAUDI','DEPARTED_SAUDI','COMPLETED'));
```

### 3b. Tabel `groups` (update)

```sql
-- Migration: 058_group_enhancement.sql
ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS kloter_id    UUID REFERENCES kloters(id),
  ADD COLUMN IF NOT EXISTS current_city TEXT NOT NULL DEFAULT 'INDONESIA',
  ADD COLUMN IF NOT EXISTS status       TEXT NOT NULL DEFAULT 'ACTIVE',
  ADD COLUMN IF NOT EXISTS capacity     INT NOT NULL DEFAULT 45,
  ADD COLUMN IF NOT EXISTS last_update  TIMESTAMPTZ;

ALTER TABLE groups ADD CONSTRAINT groups_city_check
  CHECK (current_city IN (
    'INDONESIA','MADINAH','MAKKAH','ARAFAH','MUZDALIFAH','MINA','TRANSIT','DEPARTED'
  ));

ALTER TABLE groups ADD CONSTRAINT groups_status_check
  CHECK (status IN ('ACTIVE','IN_IBADAH','EMERGENCY','COMPLETED'));
```

### 3c. Tabel `pilgrim_journey_status` (baru)

```sql
-- Migration: 059_pilgrim_journey.sql
CREATE TABLE pilgrim_journey_status (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id),
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id),
  status       TEXT NOT NULL DEFAULT 'REGISTERED',
  updated_by   TEXT REFERENCES "user"(id),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  notes        TEXT NOT NULL DEFAULT '',
  UNIQUE(pilgrim_id)
);

-- Audit log — immutable, tidak pernah di-delete
CREATE TABLE pilgrim_journey_log (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id),
  pilgrim_id  UUID NOT NULL REFERENCES pilgrims(id),
  from_status TEXT NOT NULL,
  to_status   TEXT NOT NULL,
  updated_by  TEXT REFERENCES "user"(id),
  notes       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3d. Tabel `ritual_templates` + `pilgrim_rituals` (baru)

```sql
-- Migration: 060_ritual_checklist.sql
CREATE TABLE ritual_templates (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id),
  season_type TEXT NOT NULL,  -- HAJJ | UMRAH
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  order_num   INT NOT NULL DEFAULT 0,
  is_required BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE pilgrim_rituals (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id),
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id),
  ritual_id    UUID NOT NULL REFERENCES ritual_templates(id),
  completed    BOOLEAN NOT NULL DEFAULT FALSE,
  completed_at TIMESTAMPTZ,
  completed_by TEXT REFERENCES "user"(id),
  notes        TEXT NOT NULL DEFAULT '',
  UNIQUE(pilgrim_id, ritual_id)
);

-- Default HAJJ: Ihram, Tawaf Qudum, Sa'i, Wuquf Arafah, Mabit Muzdalifah,
--   Mabit Mina, Lempar Jumrah, Tawaf Ifadah, Tawaf Wada
-- Default UMRAH: Ihram, Tawaf Umrah, Sa'i, Tahallul, Tawaf Wada
```

### 3e. Tabel `group_location_log` (baru)

```sql
-- Migration: 061_group_location_log.sql
CREATE TABLE group_location_log (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id),
  group_id    UUID NOT NULL REFERENCES groups(id),
  city        TEXT NOT NULL,
  location    TEXT NOT NULL DEFAULT '',  -- detail: "Hotel Dar Al Hijra lt 5"
  updated_by  TEXT REFERENCES "user"(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3f. Tabel `pilgrim_health_reports` (baru)

```sql
-- Migration: 062_health_reports.sql
CREATE TYPE health_severity AS ENUM ('RINGAN', 'SEDANG', 'BERAT');

CREATE TABLE pilgrim_health_reports (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id  UUID NOT NULL REFERENCES operators(id),
  pilgrim_id   UUID NOT NULL REFERENCES pilgrims(id),
  group_id     UUID NOT NULL REFERENCES groups(id),
  reported_by  TEXT REFERENCES "user"(id),
  severity     health_severity NOT NULL DEFAULT 'RINGAN',
  symptoms     TEXT NOT NULL,
  action_taken TEXT NOT NULL DEFAULT '',
  resolved     BOOLEAN NOT NULL DEFAULT FALSE,
  resolved_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3g. Update tabel `pilgrims`

```sql
-- Migration: 063_pilgrim_departure.sql
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS kloter_id   UUID REFERENCES kloters(id),
  ADD COLUMN IF NOT EXISTS seat_number TEXT,
  ADD COLUMN IF NOT EXISTS tent_number TEXT,
  ADD COLUMN IF NOT EXISTS bus_number  TEXT;
```

### 3h. Tabel `cascade_events` (baru — audit event bus)

```sql
-- Migration: 064_cascade_events.sql
CREATE TABLE cascade_events (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id),
  event_type  TEXT NOT NULL,
  trigger_by  TEXT REFERENCES "user"(id),
  payload     JSONB NOT NULL DEFAULT '{}',
  processed   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cascade_events_unprocessed
  ON cascade_events(processed, created_at)
  WHERE processed = FALSE;
```

---

## 4. Cascade Update System

### Konsep

Satu aksi oleh siapapun (Muttawwif, Tour Leader, atau Operator) memicu update berantai
ke seluruh data terkait secara otomatis — tanpa user lain perlu melakukan tindakan apapun.

### Trigger → Cascade Map

**Muttawwif update lokasi group → "Tiba Hotel Makkah":**
```
groups.current_city = MAKKAH
groups.last_update = NOW()
group_location_log ← insert baru
  ↓ cascade
pilgrim_journey_status (semua jamaah di group) = IN_MAKKAH + log
Firebase push → tiap jamaah: "Rombongan Anda kini di Makkah"
SSE → operator dashboard: city count update real-time
Family tracker → "Sedang di Makkah" untuk tiap jamaah group ini
```

**Muttawwif bulk-complete ritual (misal: Tawaf Qudum):**
```
pilgrim_rituals (seluruh group) = COMPLETED
  ↓ cascade
group ritual_completion_pct dihitung ulang
kloter ritual_completion_pct dihitung ulang
Firebase push → tiap jamaah: "Tawaf Qudum telah selesai ✓"
SSE → operator monitoring: progress bar update
Pilgrim app → tab Ibadah refresh otomatis
Family tracker → "7 dari 9 ritual selesai"
```

**Tour Leader konfirmasi boarding (Departed Indonesia):**
```
kloters.status = DEPARTED
  ↓ cascade
pilgrim_journey_status (semua jamaah kloter) = DEPARTED_INDONESIA + log
Firebase push → tiap jamaah: "Perjalanan ibadah Anda dimulai"
SSE → operator dashboard: kloter card update
Family tracker → "Dalam perjalanan menuju Saudi"
```

**Tour Leader konfirmasi landed Saudi:**
```
kloters.status = IN_SAUDI
  ↓ cascade
pilgrim_journey_status (semua jamaah kloter) = ARRIVED_SAUDI + log
Firebase push → tiap jamaah: "Selamat tiba di Tanah Suci"
SSE → operator dashboard
Family tracker → "Tiba di Arab Saudi"
```

**Tour Leader konfirmasi boarding pulang:**
```
kloters.status = DEPARTED_SAUDI
  ↓ cascade
pilgrim_journey_status (semua jamaah kloter) = DEPARTED_SAUDI + log
Firebase push → tiap jamaah: "Dalam perjalanan pulang ke Indonesia"
SSE → operator dashboard
Family tracker → "Dalam perjalanan kembali ke Indonesia"
```

**Operator konfirmasi landed Indonesia:**
```
kloters.status = COMPLETED
  ↓ cascade
pilgrim_journey_status (semua jamaah kloter) = ARRIVED_INDONESIA + log
sertifikat digital → UNLOCKED otomatis
Firebase push → tiap jamaah: "Selamat tiba! Sertifikat Anda siap diunduh"
SSE → operator dashboard
Family tracker → "Telah tiba di Indonesia ✓"
pilgrim_journey_status → COMPLETED (auto, 24 jam kemudian via worker)
```

**Health report severity BERAT dibuat Muttawwif:**
```
pilgrim_health_reports ← insert
  ↓ cascade
Firebase push → operator: "⚠ Laporan kesehatan BERAT: [nama jamaah]"
Firebase push → Tour Leader: "⚠ Laporan kesehatan BERAT: [nama jamaah]"
SSE → operator monitoring: alert muncul real-time
```

### Arsitektur Teknis

**Layer 1 — Event Publication (Go API Service Layer)**

Setiap service write yang memicu cascade mempublish event ke Redis pub/sub.
Event dipublish setelah DB transaction commit berhasil — non-blocking via goroutine.
Gagalnya publish tidak rollback transaksi utama.

```go
// internal/events/bus.go
type EventType string

const (
  EventGroupCityUpdated     EventType = "group.city_updated"
  EventRitualBulkCompleted  EventType = "ritual.bulk_completed"
  EventKloterStatusChanged  EventType = "kloter.status_changed"
  EventHealthReportCreated  EventType = "health.report_created"
  EventJourneyStatusChanged EventType = "journey.status_changed"
)

type Event struct {
  Type       EventType       `json:"type"`
  OperatorID string          `json:"operator_id"`
  Payload    json.RawMessage `json:"payload"`
  TriggeredBy string         `json:"triggered_by"`
  CreatedAt  time.Time       `json:"created_at"`
}
```

**Layer 2 — Cascade Handler (Go Worker)**

Worker yang sudah ada (`cmd/worker`) menambah subscriber untuk setiap event type.
Cascade handler melakukan bulk DB writes + Firebase push notifications.
Setiap cascade write juga insert ke `cascade_events` sebagai audit trail.

```go
// internal/worker/cascade.go
func (w *CascadeWorker) handleGroupCityUpdated(event Event) error {
  // 1. Bulk update pilgrim_journey_status untuk semua jamaah di group
  // 2. Insert pilgrim_journey_log per jamaah
  // 3. Send Firebase push ke tiap jamaah
  // 4. Publish SSE event ke Redis channel operator
  // 5. Mark cascade_event sebagai processed
}
```

**Layer 3 — Real-time ke Browser (Server-Sent Events)**

Next.js route handler `/api/events` — operator dashboard subscribe via SSE.
Redis subscribe → kirim ke browser saat ada event baru.
Tidak perlu polling — browser mendapat update instan.

```ts
// apps/web/app/api/events/route.ts
export async function GET(request: Request) {
  // Verify Better Auth session
  // Subscribe ke Redis channel: `events:${operatorId}`
  // Stream SSE ke browser
  // Cleanup saat client disconnect
}
```

**Layer 4 — Push Notification ke PWA (Firebase)**

Firebase Cloud Messaging yang sudah ada dipakai untuk:
- Jamaah → ritual selesai, update lokasi group, jadwal kepulangan, sertifikat siap
- Muttawwif → instruksi dari operator, alert eskalasi SOS
- Tour Leader → health report BERAT, SOS group, update dari operator

**Prinsip cascade:**
- Non-blocking — gagal cascade tidak rollback DB write utama
- Idempotent — handle duplikasi event dengan graceful
- Audit — semua cascade tercatat di `cascade_events` + `pilgrim_journey_log`
- Tidak rekursif — cascade tidak trigger cascade lagi

---

## 5. API Surface (Proto)

### 5a. KloterService

```protobuf
service KloterService {
  rpc CreateKloter(CreateKloterRequest) returns (Kloter);
  rpc UpdateKloter(UpdateKloterRequest) returns (Kloter);
  rpc ListKloters(ListKlotersRequest) returns (ListKlotersResponse);
  rpc GetKloter(GetKloterRequest) returns (Kloter);
  rpc DeleteKloter(DeleteKloterRequest) returns (google.protobuf.Empty);
  // Soft cascade: hanya bisa delete saat status DRAFT

  rpc UpdateKloterStatus(UpdateKloterStatusRequest) returns (Kloter);
  // Trigger cascade ke semua jamaah dalam kloter

  rpc GetKloterSummary(GetKloterSummaryRequest) returns (KloterSummary);
  rpc AssignTourLeader(AssignTourLeaderRequest) returns (Kloter);
}

message KloterSummary {
  string kloter_id = 1;
  int32  total_pilgrims = 2;
  int32  in_indonesia = 3;
  int32  in_madinah = 4;
  int32  in_makkah = 5;
  int32  in_arafah = 6;
  int32  in_mina = 7;
  int32  returned = 8;
  int32  active_sos = 9;
  double ritual_completion_pct = 10;
  int32  health_reports_open = 11;
}
```

### 5b. GroupService (update)

```protobuf
// Tambahan ke GroupService yang sudah ada:

rpc UpdateGroupCity(UpdateGroupCityRequest) returns (Group);
// Trigger cascade: bulk update pilgrim status + SSE + push notif

rpc GetGroupMonitoring(GetGroupMonitoringRequest) returns (GroupMonitoring);
rpc ListGroupsByKloter(ListGroupsByKloterRequest) returns (ListGroupsResponse);

message UpdateGroupCityRequest {
  string group_id = 1;
  string city = 2;
  string location = 3;  // detail: "Hotel Dar Al Hijra lt 5, Makkah"
  string notes = 4;
}

message GroupMonitoring {
  string group_id = 1;
  string group_name = 2;
  string leader_name = 3;
  string current_city = 4;
  string last_update = 5;
  int32  total_pilgrims = 6;
  int32  checked_in = 7;
  int32  health_issues = 8;
  int32  active_sos = 9;
  double ritual_completion_pct = 10;
  repeated string pending_rituals = 11;
}
```

### 5c. RitualService (baru)

```protobuf
service RitualService {
  rpc ListRitualTemplates(ListRitualTemplatesRequest) returns (ListRitualTemplatesResponse);
  rpc CreateRitualTemplate(CreateRitualTemplateRequest) returns (RitualTemplate);
  rpc SeedDefaultTemplates(SeedDefaultTemplatesRequest) returns (google.protobuf.Empty);

  rpc CompleteRitual(CompleteRitualRequest) returns (PilgrimRitual);
  // Single jamaah

  rpc BulkCompleteRitual(BulkCompleteRitualRequest) returns (BulkCompleteRitualResponse);
  // Seluruh group — backend resolve pilgrim list dari group_id
  // Trigger cascade: SSE + push notif ke tiap jamaah

  rpc GetGroupRitualProgress(GetGroupRitualProgressRequest) returns (GroupRitualProgress);
  rpc GetPilgrimRituals(GetPilgrimRitualsRequest) returns (GetPilgrimRitualsResponse);
  // Public — auth by app_access_code, untuk pilgrim app
}

message BulkCompleteRitualRequest {
  string group_id = 1;
  string ritual_id = 2;
  string notes = 3;
  repeated string excluded_pilgrim_ids = 4;
  // Jamaah yang tidak ikut (sakit, terpisah)
}
```

### 5d. JourneyService (baru)

```protobuf
service JourneyService {
  rpc UpdatePilgrimStatus(UpdatePilgrimStatusRequest) returns (PilgrimJourneyStatus);
  rpc BulkUpdateStatus(BulkUpdateStatusRequest) returns (BulkUpdateStatusResponse);
  // Bulk per kloter — trigger cascade

  rpc GetPilgrimStatus(GetPilgrimStatusRequest) returns (PilgrimJourneyStatus);
  rpc GetKloterJourneyOverview(GetKloterJourneyOverviewRequest) returns (KloterJourneyOverview);

  rpc GetMyJourneyStatus(GetMyJourneyStatusRequest) returns (PilgrimJourneyStatus);
  // Public — auth by app_access_code
}

message KloterJourneyOverview {
  string kloter_id = 1;
  map<string, int32> status_counts = 2;
  repeated PilgrimStatusAlert alerts = 3;
  // Jamaah yang statusnya tidak sinkron dengan kloternya
}
```

### 5e. HealthReportService (baru)

```protobuf
service HealthReportService {
  rpc CreateHealthReport(CreateHealthReportRequest) returns (HealthReport);
  // Trigger cascade: push notif ke operator + Tour Leader jika severity BERAT

  rpc ListHealthReports(ListHealthReportsRequest) returns (ListHealthReportsResponse);
  rpc ResolveHealthReport(ResolveHealthReportRequest) returns (HealthReport);
}
```

---

## 6. UI/UX Specification

### 6a. Operator Dashboard

#### `/dashboard/kloter` — Kloter Management

**List view:**
- Card per kloter: nama, embarkasi badge, tanggal berangkat, status stepper, jamaah/kapasitas
- Filter by season, status
- Summary strip: total aktif di Saudi, akan berangkat minggu ini, akan pulang minggu ini

**Detail `/dashboard/kloter/[id]`:**

Tab **Info:**
- Form: nama, embarkasi, flight out/in, tanggal, kepala kloter (picker member), Tour Leader (picker agent)
- Status stepper visual dengan tombol transisi: "Konfirmasi" / "Catat Keberangkatan" / "Tiba di Saudi" / dll
- Setiap klik tombol transisi → cascade update ke semua jamaah secara otomatis

Tab **Rombongan:**
- List semua group: nama, Muttawwif, jumlah jamaah, lokasi sekarang, last update
- Color coding last update: hijau (<2 jam), kuning (2-6 jam), merah (>6 jam)
- Alert badge jika ada SOS atau health report aktif

Tab **Monitoring:**
- City breakdown cards: berapa group/jamaah di tiap lokasi
- Ritual completion progress per group
- Health reports aktif — severity badge
- SOS aktif

Tab **Jamaah:**
- Tabel semua jamaah: nama, group, status journey, ritual %, kesehatan flag
- Search, filter, export CSV/manifest PDF

Tab **Kepulangan:**
- Muncul saat kloter status IN_SAUDI
- Checklist pre-departure: berapa jamaah sudah konfirmasi siap, berapa belum
- Tombol "Semua Siap — Catat Keberangkatan Pulang" → cascade update

---

#### `/dashboard/monitoring` — Real-time Overview

**Auto-refresh via SSE** — tidak perlu reload halaman.

**Header alert bar:**
- SOS aktif (merah) — group tidak update >6 jam (kuning) — health report BERAT (oranye)

**City grid:**
- Section per kota: Makkah / Madinah / Arafah / Mina / dll
- Mini card per group: nama, Muttawwif, last update, ritual progress bar
- Klik → drawer detail group tanpa pindah halaman

**Kepulangan timeline:**
- Kloter yang pulang dalam 7 hari
- Per kloter: % jamaah sudah konfirmasi siap boarding

---

#### `/dashboard/groups` — Update

- Tambah kolom: Kloter, Lokasi Sekarang, Last Update, Ritual (x/y)
- Filter by kloter
- Row color: normal/kuning/merah berdasarkan last_update

---

### 6b. Muttawwif App (`/leader`) — Full Revamp

**5 Tab:**

**Tab 1 — Rombongan:**
- List jamaah: foto, nama, status kesehatan icon, ritual x/y
- Tap jamaah → detail lengkap: info, ritual personal, health history
- Bulk action: "Check-in semua ke hotel ini"

**Tab 2 — Ibadah:**
- List ritual sesuai template season
- Per ritual: nama, jumlah selesai/total, progress bar
- Tap → detail: siapa sudah, siapa belum
- Tombol "Semua Selesai" → BulkCompleteRitual → cascade otomatis ke seluruh jamaah
- Tombol "Pilih Jamaah" → partial complete (ada yang tidak ikut)

**Tab 3 — Lokasi:**
- Lokasi group sekarang + timestamp
- Tombol "Update Lokasi" → dropdown city + field detail
- Satu tap → cascade update ke seluruh jamaah + notif ke operator + family tracker
- Riwayat perpindahan lokasi

**Tab 4 — Kesehatan:**
- List health reports yang sudah dibuat
- Tombol "+ Laporkan" → form: pilih jamaah, severity, gejala, tindakan
- BERAT → otomatis push notif ke operator & Tour Leader

**Tab 5 — Kepulangan:**
- Aktif saat kloter status PRE_DEPARTURE_SAUDI
- Checklist per jamaah: paspor ✓, koper ✓, di bus ✓
- Konfirmasi "Semua anggota group siap boarding"
- Serah terima digital ke Tour Leader

---

### 6c. Pilgrim App (`/pilgrim`) — Update

**Tab "Perjalanan Saya":**
- Timeline visual status journey dengan checkpoints
- Setiap checkpoint: tanggal, lokasi, siapa yang update
- Update otomatis saat Muttawwif update lokasi group — tidak perlu refresh
- "Estimasi tiba Indonesia: [flight_in_date]"

**Tab "Ibadah Saya" (baru):**
- Progress ring besar: X dari Y ritual selesai
- List ritual — yang belum di atas, yang selesai di bawah dengan timestamp
- Keterangan: "Diselesaikan oleh Muttawwif [nama], [waktu]"
- Update otomatis saat Muttawwif bulk-complete ritual

**Push Notifications:**
- "Rombongan Anda kini di [lokasi]"
- "[Nama ritual] telah selesai ✓"
- "Bus kepulangan berangkat 2 jam lagi"
- "Selamat tiba di Indonesia! Sertifikat Anda siap diunduh"

---

### 6d. Tour Leader App (`/agent`) — Tambahan Tab

**Tab "Perjalanan" (muncul saat ditugaskan ke kloter):**

Sub-tab **Kloter:**
- Info kloter: flight, embarkasi, jadwal
- Summary: total jamaah, per-city breakdown, ritual completion
- Download manifest PDF

Sub-tab **Koordinasi:**
- Kontak semua Muttawwif dalam kloter (WhatsApp link)
- Health reports aktif dari semua group
- SOS aktif

Sub-tab **Kepulangan:**
- Checklist konfirmasi per group dari Muttawwif
- Tombol "Catat Boarding" → kloter status DEPARTED_SAUDI → cascade seluruh jamaah
- Status boarding per jamaah

---

### 6e. Family Tracker (Public) — Update

Tambahan (tetap tidak expose data sensitif):
- Status human-readable: "Sedang di Makkah", "Dalam perjalanan pulang", "Tiba di Indonesia"
- Progress ritual: "7 dari 9 ritual Haji telah selesai"
- Estimasi kepulangan: "Dijadwalkan tiba [tanggal]"
- Auto-update saat cascade terjadi — tanpa reload

Tidak pernah ditampilkan: nomor paspor, nomor kamar, nomor tenda, nomor telepon.

---

## 7. Business Rules

### Kloter
- Hanya bisa dihapus saat status `DRAFT`
- Status hanya bisa maju, tidak bisa mundur (kecuali CONFIRMED → DRAFT oleh owner)
- Kapasitas tidak bisa kurang dari jumlah jamaah terdaftar
- Satu agent/Tour Leader hanya bisa di-assign ke satu kloter aktif per season

### Group
- Harus punya `kloter_id` sebelum kloter berstatus `CONFIRMED`
- Muttawwif tidak bisa di-assign ke lebih dari satu group aktif
- Alert dikirim ke operator jika group tidak update lokasi >6 jam saat kloter `IN_SAUDI`

### Ritual
- Ritual yang sudah complete tidak bisa di-unmark — immutable
- `BulkCompleteRitual` hanya bisa dipanggil Muttawwif yang memimpin group tersebut
- Backend resolve pilgrim list dari group_id — client tidak kirim list jamaah
- Jamaah di `excluded_pilgrim_ids` tetap bisa di-complete manual kemudian

### Journey Status
- Status hanya bisa maju sesuai urutan — tidak bisa skip lebih dari 1 step
- `BulkUpdateStatus` per kloter hanya bisa dilakukan oleh operator atau Tour Leader
- Status `ARRIVED_INDONESIA` otomatis unlock sertifikat digital
- Semua perubahan status immutable di `pilgrim_journey_log`

### Health Reports
- Severity `BERAT` otomatis trigger push notif ke operator + Tour Leader
- Tidak bisa dihapus — hanya bisa di-resolve dengan catatan
- Jamaah dengan health report BERAT aktif tidak bisa di-bulk status update

### Cascade
- Non-blocking — gagal cascade tidak rollback DB write utama
- Idempotent — duplicate event dihandle gracefully
- Audit — semua cascade tercatat di `cascade_events` + log relevan
- Tidak rekursif — cascade tidak trigger cascade lagi

---

## 8. Migrasi Data Existing

- Groups yang ada → assign ke kloter via UI (batch update)
- Pilgrims yang ada → `pilgrim_journey_status` di-seed dengan status `REGISTERED`
- Ritual templates → seed via `SeedDefaultTemplates` RPC saat season dibuat
- `kloter_id` di pilgrims → nullable, diisi saat assign ke kloter
- Tidak ada breaking change — semua kolom baru nullable atau punya DEFAULT

---

## 9. Prioritas Implementasi

### Phase 1 — Fondasi (wajib sebelum musim berikutnya)
1. Migration 057-064
2. KloterService CRUD + status management
3. GroupService: kloter_id, current_city, UpdateGroupCity
4. JourneyService: BulkUpdateStatus per kloter
5. Event bus (Redis pub/sub) — skeleton
6. Dashboard: `/dashboard/kloter` (info + rombongan tab)
7. Muttawwif app: Tab Lokasi

### Phase 2 — Operasional Real-time
8. RitualService + seed default templates
9. Cascade handler di worker
10. SSE endpoint di Next.js
11. Dashboard: `/dashboard/monitoring` dengan SSE
12. Muttawwif app: Tab Ibadah (ritual tracker)
13. Pilgrim app: Tab Ibadah + status journey (SSE refresh)

### Phase 3 — Kepulangan & Post-trip
14. HealthReportService
15. Muttawwif app: Tab Kesehatan + Tab Kepulangan
16. Tour Leader app: Tab Perjalanan
17. Cascade: auto-unlock sertifikat saat ARRIVED_INDONESIA
18. Family tracker: status journey + ritual progress
19. Push notifications semua milestone

### Phase 4 — Enhancement
20. Manifest PDF export
21. Analytics per kloter/season
22. Survey kepuasan post-trip
23. Foto dokumentasi perjalanan

---

*Dokumen ini adalah referensi teknis + business untuk enhancement Group, Kloter, Muttawwif,*
*dan Cascade Update System. Siap dieksekusi sebagai prompt 9router per phase.*
*Last updated: Agustus 2026*
