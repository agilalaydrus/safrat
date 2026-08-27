# Tawafiq Hub — Phase 4 Technical Implementation Document
## 7 Modules: Cash Flow · Vendor SLA · Staff Scheduling · Insurance · Preparation Checklist · Find My Group · Digital Certificate

---

## Architectural Context & Invariants

```
Stack      : Turborepo monorepo, Next.js 15 (apps/web :3131), Go Connect RPC (apps/api :8131), PostgreSQL :5434
Auth       : Better Auth. Go API validates every RPC by DB session lookup — no JWT.
Layers     : handler/ → service/ → repository/ — never skip, never reach sideways.
Tenant     : Every query MUST be scoped by operatorID from middleware.OperatorIDFromCtx().
Migrations : goose, apps/api/db/migrations/, table names PLURAL. Last applied: 050.
sqlc       : runs from apps/api/. buf: runs from proto/.
Public RPCs: Must appear in BOTH publicProcedures AND rateLimitedProcedures.
Errors     : Never expose raw DB errors — always map through connectError().
CSS vars   : --color-cream-*, --color-emerald-*, --color-gold-*, --color-warm-*, --color-danger-*
```

**Next free migration: 051**

---

## Module A — Cash Flow Dashboard

### Business Rule
Operator needs to forecast: given current jamaah payment status vs upcoming vendor payment
deadlines, will they have enough funds? Dashboard shows: total collected vs total committed
to vendors, month-by-month projection, and a "danger zone" alert when gap is negative.

### A1 — Migration 051: vendor_payments

```sql
-- +goose Up

-- Tracks committed vendor payment obligations per season.
-- Each row = one scheduled payment to a vendor (hotel deposit, bus block, catering, etc.)
CREATE TABLE vendor_payments (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id   UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id     UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  vendor_name   TEXT        NOT NULL,
  category      TEXT        NOT NULL DEFAULT 'HOTEL'
                            CHECK (category IN ('HOTEL','TRANSPORT','CATERING','VISA','INSURANCE','OTHER')),
  description   TEXT        NOT NULL DEFAULT '',
  amount        NUMERIC(14,2) NOT NULL,
  due_date      DATE        NOT NULL,
  status        TEXT        NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING','PAID','OVERDUE','CANCELLED')),
  paid_at       TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_payments_operator_season_idx ON vendor_payments(operator_id, season_id);
CREATE INDEX vendor_payments_due_date_idx        ON vendor_payments(due_date, status);

CREATE TRIGGER vendor_payments_set_updated_at
  BEFORE UPDATE ON vendor_payments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS vendor_payments;
```

### A2 — sqlc Queries

**File: `apps/api/db/query/cashflow.sql`**

```sql
-- name: CreateVendorPayment :one
INSERT INTO vendor_payments (operator_id, season_id, vendor_name, category, description, amount, due_date)
VALUES (@operator_id, @season_id, @vendor_name, @category, @description, @amount, @due_date)
RETURNING *;

-- name: ListVendorPayments :many
SELECT * FROM vendor_payments
WHERE operator_id = @operator_id AND season_id = @season_id
ORDER BY due_date ASC;

-- name: UpdateVendorPaymentStatus :one
UPDATE vendor_payments
SET status = @status, paid_at = CASE WHEN @status = 'PAID' THEN NOW() ELSE NULL END
WHERE id = @id AND operator_id = @operator_id
RETURNING *;

-- name: DeleteVendorPayment :exec
DELETE FROM vendor_payments WHERE id = @id AND operator_id = @operator_id;

-- name: GetCashFlowSummary :one
-- Aggregates collected vs committed for a season.
SELECT
  -- Total collected from pilgrims (sum of total_paid on all active pilgrims)
  COALESCE(SUM(p.total_paid) FILTER (WHERE p.status = 'ACTIVE'), 0)        AS total_collected,
  -- Total committed to vendors (all non-cancelled)
  COALESCE(SUM(vp.amount) FILTER (WHERE vp.status != 'CANCELLED'), 0)      AS total_committed,
  -- Already paid to vendors
  COALESCE(SUM(vp.amount) FILTER (WHERE vp.status = 'PAID'), 0)            AS total_paid_out,
  -- Pending/overdue vendor obligations
  COALESCE(SUM(vp.amount) FILTER (WHERE vp.status IN ('PENDING','OVERDUE')), 0) AS total_outstanding,
  -- Overdue only
  COALESCE(SUM(vp.amount) FILTER (WHERE vp.status = 'OVERDUE'), 0)         AS total_overdue,
  -- Next 30 days obligations
  COALESCE(SUM(vp.amount) FILTER (
    WHERE vp.status = 'PENDING' AND vp.due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 30
  ), 0)                                                                     AS due_next_30_days,
  -- Count pilgrims not yet fully paid
  COUNT(p.id) FILTER (WHERE p.payment_status != 'PAID' AND p.status = 'ACTIVE') AS unpaid_pilgrim_count
FROM pilgrims p
FULL OUTER JOIN vendor_payments vp
  ON vp.season_id = p.season_id AND vp.operator_id = p.operator_id
WHERE COALESCE(p.operator_id, vp.operator_id) = @operator_id
  AND COALESCE(p.season_id,   vp.season_id)   = @season_id;

-- name: GetMonthlyProjection :many
-- Month-by-month breakdown of vendor obligations vs expected collection.
SELECT
  DATE_TRUNC('month', vp.due_date)::DATE AS month,
  SUM(vp.amount)                          AS vendor_obligations,
  COUNT(vp.id)                            AS payment_count
FROM vendor_payments vp
WHERE vp.operator_id = @operator_id
  AND vp.season_id   = @season_id
  AND vp.status != 'CANCELLED'
GROUP BY 1
ORDER BY 1;
```

### A3 — Proto: `proto/hajj/v1/cashflow.proto`

```protobuf
syntax = "proto3";
package hajj.v1;
import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";
option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service CashFlowService {
  rpc CreateVendorPayment(CreateVendorPaymentRequest)   returns (VendorPayment);
  rpc ListVendorPayments(ListVendorPaymentsRequest)     returns (ListVendorPaymentsResponse);
  rpc UpdateVendorPaymentStatus(UpdateVendorPaymentStatusRequest) returns (VendorPayment);
  rpc DeleteVendorPayment(DeleteVendorPaymentRequest)   returns (DeleteVendorPaymentResponse);
  rpc GetCashFlowSummary(GetCashFlowSummaryRequest)     returns (CashFlowSummary);
  rpc GetMonthlyProjection(GetMonthlyProjectionRequest) returns (GetMonthlyProjectionResponse);
}

message VendorPayment {
  string id          = 1;
  string season_id   = 2;
  string vendor_name = 3;
  string category    = 4;
  string description = 5;
  double amount      = 6;
  string due_date    = 7;   // ISO date string YYYY-MM-DD
  string status      = 8;   // PENDING | PAID | OVERDUE | CANCELLED
  google.protobuf.Timestamp paid_at    = 9;
  google.protobuf.Timestamp created_at = 10;
}

message CashFlowSummary {
  double total_collected      = 1;
  double total_committed      = 2;
  double total_paid_out       = 3;
  double total_outstanding    = 4;
  double total_overdue        = 5;
  double due_next_30_days     = 6;
  int64  unpaid_pilgrim_count = 7;
  double net_position         = 8;  // total_collected - total_outstanding (computed in service)
}

message MonthlyProjectionEntry {
  string month               = 1;  // YYYY-MM
  double vendor_obligations  = 2;
  int32  payment_count       = 3;
}

message CreateVendorPaymentRequest {
  string season_id   = 1 [(buf.validate.field).string.min_len = 1];
  string vendor_name = 2 [(buf.validate.field).string.min_len = 1];
  string category    = 3;
  string description = 4;
  double amount      = 5 [(buf.validate.field).double.gt = 0];
  string due_date    = 6 [(buf.validate.field).string.min_len = 1];
}

message ListVendorPaymentsRequest      { string season_id = 1; }
message ListVendorPaymentsResponse     { repeated VendorPayment payments = 1; }
message UpdateVendorPaymentStatusRequest { string id = 1; string status = 2; }
message DeleteVendorPaymentRequest     { string id = 1; }
message DeleteVendorPaymentResponse    {}
message GetCashFlowSummaryRequest      { string season_id = 1; }
message GetMonthlyProjectionRequest    { string season_id = 1; }
message GetMonthlyProjectionResponse   { repeated MonthlyProjectionEntry months = 1; }
```

### A4 — Frontend: `/dashboard/cashflow/page.tsx`

Dashboard with 4 sections:
1. **KPI row** — Total Terkumpul (green), Total Komitmen Vendor (neutral), Posisi Bersih (green if positive, red if negative), Jatuh Tempo 30 Hari (orange warning).
2. **Danger Zone Alert** — if net_position < due_next_30_days, show red banner: "⚠ Dana tidak cukup untuk vendor payment bulan ini. Defisit: Rp X".
3. **Monthly timeline chart** — bar chart using inline SVG (no library needed) showing vendor obligations per month.
4. **Vendor payment table** — list all entries with status badge (PENDING=orange, PAID=green, OVERDUE=red, CANCELLED=gray), Mark as Paid button, Delete button.

Add "+ Tambah Pembayaran Vendor" button that opens inline form: vendor name, category dropdown, amount, due date, description.

Add to nav: `{ href: "/dashboard/cashflow", label: "Cash Flow", icon: IconCash }`.

---

## Module B — Vendor SLA & Contract Management

### Business Rule
Operator tracks contracts and SLA commitments with each vendor: room count committed,
actual rooms confirmed, deadline for confirmation, and notes. Provides digital paper trail
when disputes arise. No file upload required for v1 — structured data fields are sufficient.

### B1 — Migration 052: vendor_contracts

```sql
-- +goose Up
CREATE TABLE vendor_contracts (
  id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id         UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id           UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  vendor_name         TEXT        NOT NULL,
  vendor_type         TEXT        NOT NULL DEFAULT 'HOTEL'
                                  CHECK (vendor_type IN ('HOTEL','TRANSPORT','CATERING','VISA_AGENT','INSURANCE','OTHER')),
  contract_number     TEXT        NOT NULL DEFAULT '',
  -- SLA fields
  committed_units     INTEGER     NOT NULL DEFAULT 0,   -- rooms blocked / seats reserved
  confirmed_units     INTEGER     NOT NULL DEFAULT 0,   -- actually confirmed by vendor
  confirmation_deadline DATE,
  -- Financial
  rate_per_unit       NUMERIC(14,2) NOT NULL DEFAULT 0,
  total_value         NUMERIC(14,2) GENERATED ALWAYS AS (committed_units * rate_per_unit) STORED,
  deposit_amount      NUMERIC(14,2) NOT NULL DEFAULT 0,
  deposit_paid        BOOLEAN     NOT NULL DEFAULT false,
  -- Status
  status              TEXT        NOT NULL DEFAULT 'NEGOTIATING'
                                  CHECK (status IN ('NEGOTIATING','CONFIRMED','PARTIAL','CANCELLED')),
  notes               TEXT        NOT NULL DEFAULT '',
  contact_name        TEXT        NOT NULL DEFAULT '',
  contact_phone       TEXT        NOT NULL DEFAULT '',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_contracts_operator_season_idx ON vendor_contracts(operator_id, season_id);

CREATE TRIGGER vendor_contracts_set_updated_at
  BEFORE UPDATE ON vendor_contracts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Immutable SLA event log — every status change or note is recorded here.
CREATE TABLE vendor_contract_events (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id     UUID        NOT NULL REFERENCES vendor_contracts(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  event_type      TEXT        NOT NULL,  -- e.g. 'STATUS_CHANGED', 'UNITS_UPDATED', 'NOTE_ADDED'
  description     TEXT        NOT NULL,
  recorded_by     TEXT        NOT NULL,  -- user_id
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX vendor_contract_events_contract_idx ON vendor_contract_events(contract_id);

-- +goose Down
DROP TABLE IF EXISTS vendor_contract_events;
DROP TABLE IF EXISTS vendor_contracts;
```

### B2 — sqlc Queries

**File: `apps/api/db/query/vendor_contract.sql`**

```sql
-- name: CreateVendorContract :one
INSERT INTO vendor_contracts (
  operator_id, season_id, vendor_name, vendor_type, contract_number,
  committed_units, confirmed_units, confirmation_deadline,
  rate_per_unit, deposit_amount, status, notes, contact_name, contact_phone
) VALUES (
  @operator_id, @season_id, @vendor_name, @vendor_type, @contract_number,
  @committed_units, @confirmed_units, @confirmation_deadline,
  @rate_per_unit, @deposit_amount, @status, @notes, @contact_name, @contact_phone
) RETURNING *;

-- name: ListVendorContracts :many
SELECT * FROM vendor_contracts
WHERE operator_id = @operator_id AND season_id = @season_id
ORDER BY vendor_type, vendor_name;

-- name: UpdateVendorContract :one
UPDATE vendor_contracts
SET vendor_name = @vendor_name, confirmed_units = @confirmed_units,
    confirmation_deadline = @confirmation_deadline, status = @status,
    notes = @notes, deposit_paid = @deposit_paid, contact_name = @contact_name,
    contact_phone = @contact_phone
WHERE id = @id AND operator_id = @operator_id
RETURNING *;

-- name: DeleteVendorContract :exec
DELETE FROM vendor_contracts WHERE id = @id AND operator_id = @operator_id;

-- name: CreateContractEvent :one
INSERT INTO vendor_contract_events (contract_id, operator_id, event_type, description, recorded_by)
VALUES (@contract_id, @operator_id, @event_type, @description, @recorded_by)
RETURNING *;

-- name: ListContractEvents :many
SELECT * FROM vendor_contract_events
WHERE contract_id = @contract_id AND operator_id = @operator_id
ORDER BY created_at DESC;

-- name: GetVendorSLAStatus :many
-- Summary of all contracts with SLA health check.
SELECT
  vc.*,
  CASE
    WHEN vc.confirmed_units >= vc.committed_units THEN 'ON_TRACK'
    WHEN vc.confirmation_deadline < CURRENT_DATE AND vc.confirmed_units < vc.committed_units THEN 'OVERDUE'
    WHEN vc.confirmation_deadline BETWEEN CURRENT_DATE AND CURRENT_DATE + 7 THEN 'AT_RISK'
    ELSE 'PENDING'
  END AS sla_health
FROM vendor_contracts vc
WHERE vc.operator_id = @operator_id AND vc.season_id = @season_id
ORDER BY sla_health DESC, vc.confirmation_deadline ASC;
```

### B3 — Proto: `proto/hajj/v1/vendor.proto`

```protobuf
syntax = "proto3";
package hajj.v1;
import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";
option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service VendorService {
  rpc CreateVendorContract(CreateVendorContractRequest) returns (VendorContract);
  rpc ListVendorContracts(ListVendorContractsRequest)   returns (ListVendorContractsResponse);
  rpc UpdateVendorContract(UpdateVendorContractRequest) returns (VendorContract);
  rpc DeleteVendorContract(DeleteVendorContractRequest) returns (DeleteVendorContractResponse);
  rpc AddContractEvent(AddContractEventRequest)         returns (ContractEvent);
  rpc ListContractEvents(ListContractEventsRequest)     returns (ListContractEventsResponse);
  rpc GetVendorSLAStatus(GetVendorSLAStatusRequest)    returns (GetVendorSLAStatusResponse);
}

message VendorContract {
  string  id                    = 1;
  string  vendor_name           = 2;
  string  vendor_type           = 3;
  string  contract_number       = 4;
  int32   committed_units       = 5;
  int32   confirmed_units       = 6;
  string  confirmation_deadline = 7;
  double  rate_per_unit         = 8;
  double  total_value           = 9;
  double  deposit_amount        = 10;
  bool    deposit_paid          = 11;
  string  status                = 12;
  string  sla_health            = 13;  // ON_TRACK | AT_RISK | OVERDUE | PENDING
  string  notes                 = 14;
  string  contact_name          = 15;
  string  contact_phone         = 16;
  google.protobuf.Timestamp created_at = 17;
}

message ContractEvent {
  string id          = 1;
  string contract_id = 2;
  string event_type  = 3;
  string description = 4;
  string recorded_by = 5;
  google.protobuf.Timestamp created_at = 6;
}

message CreateVendorContractRequest {
  string season_id              = 1 [(buf.validate.field).string.min_len = 1];
  string vendor_name            = 2 [(buf.validate.field).string.min_len = 1];
  string vendor_type            = 3;
  string contract_number        = 4;
  int32  committed_units        = 5;
  string confirmation_deadline  = 6;
  double rate_per_unit          = 7;
  double deposit_amount         = 8;
  string notes                  = 9;
  string contact_name           = 10;
  string contact_phone          = 11;
}

message ListVendorContractsRequest     { string season_id = 1; }
message ListVendorContractsResponse    { repeated VendorContract contracts = 1; }
message UpdateVendorContractRequest    {
  string id = 1; string vendor_name = 2; int32 confirmed_units = 3;
  string confirmation_deadline = 4; string status = 5; string notes = 6;
  bool deposit_paid = 7; string contact_name = 8; string contact_phone = 9;
}
message DeleteVendorContractRequest    { string id = 1; }
message DeleteVendorContractResponse   {}
message AddContractEventRequest        { string contract_id = 1; string event_type = 2; string description = 3; }
message ListContractEventsRequest      { string contract_id = 1; }
message ListContractEventsResponse     { repeated ContractEvent events = 1; }
message GetVendorSLAStatusRequest      { string season_id = 1; }
message GetVendorSLAStatusResponse     { repeated VendorContract contracts = 1; }
```

### B4 — Frontend: `/dashboard/vendors/page.tsx`

Three-tab page:
1. **SLA Overview** — card per contract with health badge: ON_TRACK=green, AT_RISK=orange, OVERDUE=red. Shows: vendor name, type, confirmed/committed units (e.g. "120/150 kamar"), deadline, deposit status.
2. **Contract List** — full table with all fields, inline edit, delete.
3. **Event Log** — per-contract timeline of all events (who changed what, when).

Add to nav: `{ href: "/dashboard/vendors", label: "Vendor", icon: IconBuildingStore }`.

---

## Module C — Staff & Coordinator Scheduling

### Business Rule
Each kloter has one or more coordinators assigned from the operator's staff (Better Auth org members).
Coordinator sees their assignment, duty checklist, and contact list for their kloter.
Operator sees all assignments across all kloters.

### C1 — Migration 053: kloter_staff

```sql
-- +goose Up

-- Staff assignment per kloter. A staff member can be assigned to multiple kloters,
-- a kloter can have multiple staff with different roles.
CREATE TABLE kloter_staff (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  kloter_id   UUID        NOT NULL REFERENCES kloters(id)  ON DELETE CASCADE,
  staff_id    TEXT        NOT NULL,   -- Better Auth "user"(id), TEXT not UUID
  staff_name  TEXT        NOT NULL,   -- denormalized for display without join
  staff_email TEXT        NOT NULL,
  role        TEXT        NOT NULL DEFAULT 'COORDINATOR'
              CHECK (role IN ('COORDINATOR','MEDICAL','GUIDE','ADMIN_SUPPORT')),
  duties      TEXT        NOT NULL DEFAULT '',   -- free-text duty description
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (kloter_id, staff_id)
);

CREATE INDEX kloter_staff_operator_idx ON kloter_staff(operator_id);
CREATE INDEX kloter_staff_kloter_idx   ON kloter_staff(kloter_id);
CREATE INDEX kloter_staff_user_idx     ON kloter_staff(staff_id);

-- +goose Down
DROP TABLE IF EXISTS kloter_staff;
```

### C2 — sqlc Queries

**File: `apps/api/db/query/staff_schedule.sql`**

```sql
-- name: AssignStaffToKloter :one
INSERT INTO kloter_staff (operator_id, kloter_id, staff_id, staff_name, staff_email, role, duties)
VALUES (@operator_id, @kloter_id, @staff_id, @staff_name, @staff_email, @role, @duties)
ON CONFLICT (kloter_id, staff_id) DO UPDATE
  SET role = EXCLUDED.role, duties = EXCLUDED.duties
RETURNING *;

-- name: ListKloterStaff :many
SELECT ks.*, k.name AS kloter_name, k.departure_date
FROM kloter_staff ks
JOIN kloters k ON k.id = ks.kloter_id
WHERE ks.operator_id = @operator_id AND ks.kloter_id = @kloter_id;

-- name: ListMyAssignments :many
-- Staff-facing: show all kloters this staff member is assigned to.
SELECT ks.*, k.name AS kloter_name, k.departure_date, s.name AS season_name
FROM kloter_staff ks
JOIN kloters k ON k.id = ks.kloter_id
JOIN seasons s ON s.id = k.season_id
WHERE ks.staff_id = @staff_id AND ks.operator_id = @operator_id
ORDER BY k.departure_date ASC;

-- name: ListAllStaffSchedule :many
-- Operator overview: all kloters with their assigned staff.
SELECT
  k.id AS kloter_id, k.name AS kloter_name, k.departure_date,
  s.name AS season_name,
  COUNT(ks.id) AS staff_count,
  STRING_AGG(ks.staff_name, ', ' ORDER BY ks.role) AS staff_names
FROM kloters k
JOIN seasons s ON s.id = k.season_id
LEFT JOIN kloter_staff ks ON ks.kloter_id = k.id
WHERE k.operator_id = @operator_id AND s.id = @season_id
GROUP BY k.id, k.name, k.departure_date, s.name
ORDER BY k.departure_date ASC;

-- name: RemoveStaffFromKloter :exec
DELETE FROM kloter_staff
WHERE kloter_id = @kloter_id AND staff_id = @staff_id AND operator_id = @operator_id;
```

### C3 — Proto: `proto/hajj/v1/staff_schedule.proto`

```protobuf
syntax = "proto3";
package hajj.v1;
import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";
option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service StaffScheduleService {
  rpc AssignStaffToKloter(AssignStaffToKloterRequest)   returns (KloterStaff);
  rpc ListKloterStaff(ListKloterStaffRequest)           returns (ListKloterStaffResponse);
  rpc RemoveStaffFromKloter(RemoveStaffFromKloterRequest) returns (RemoveStaffFromKloterResponse);
  rpc ListAllStaffSchedule(ListAllStaffScheduleRequest) returns (ListAllStaffScheduleResponse);
  rpc ListMyAssignments(ListMyAssignmentsRequest)       returns (ListMyAssignmentsResponse);
}

message KloterStaff {
  string id          = 1;
  string kloter_id   = 2;
  string kloter_name = 3;
  string staff_id    = 4;
  string staff_name  = 5;
  string staff_email = 6;
  string role        = 7;
  string duties      = 8;
  google.protobuf.Timestamp departure_date = 9;
}

message KloterScheduleSummary {
  string kloter_id    = 1;
  string kloter_name  = 2;
  string season_name  = 3;
  int32  staff_count  = 4;
  string staff_names  = 5;
  google.protobuf.Timestamp departure_date = 6;
}

message AssignStaffToKloterRequest {
  string kloter_id   = 1 [(buf.validate.field).string.min_len = 1];
  string staff_id    = 2 [(buf.validate.field).string.min_len = 1];
  string staff_name  = 3 [(buf.validate.field).string.min_len = 1];
  string staff_email = 4;
  string role        = 5;
  string duties      = 6;
}
message ListKloterStaffRequest           { string kloter_id = 1; }
message ListKloterStaffResponse          { repeated KloterStaff staff = 1; }
message RemoveStaffFromKloterRequest     { string kloter_id = 1; string staff_id = 2; }
message RemoveStaffFromKloterResponse    {}
message ListAllStaffScheduleRequest      { string season_id = 1; }
message ListAllStaffScheduleResponse     { repeated KloterScheduleSummary kloters = 1; }
message ListMyAssignmentsRequest         {}
message ListMyAssignmentsResponse        { repeated KloterStaff assignments = 1; }
```

### C4 — Frontend

**`/dashboard/schedule/page.tsx`** — Operator view: grid of all kloters per season with assigned staff names, color-coded by how many staff (red=0, orange=1, green=2+). Click kloter → drawer with staff list + assign form (select from Better Auth members via `authClient.organization.listMembers`).

**`/dashboard/my-schedule/page.tsx`** — Staff-facing: "Jadwal Saya" page listing all kloters they're assigned to, with duty description and pilgrim count. Visible to all org members.

Add to nav: `{ href: "/dashboard/schedule", label: "Jadwal Tim", icon: IconCalendarEvent }`.

---

## Module D — Insurance Claim Support

### Business Rule
When a jamaah needs an insurance claim (medical, death, flight disruption), the operator
needs to quickly export a structured data sheet with all required fields. This module adds:
1. Insurance info fields on each pilgrim
2. One-click export of all required claim fields as PDF or structured data

No new external insurance integration needed for v1 — purely data collection + export.

### D1 — Migration 054: pilgrim insurance fields

```sql
-- +goose Up
ALTER TABLE pilgrims
  ADD COLUMN IF NOT EXISTS insurance_provider  TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS insurance_policy_no TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS insurance_class     TEXT NOT NULL DEFAULT 'STANDARD'
                           CHECK (insurance_class IN ('STANDARD','PLUS','PREMIUM')),
  ADD COLUMN IF NOT EXISTS blood_type          TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS chronic_conditions  TEXT NOT NULL DEFAULT '',   -- comma-separated
  ADD COLUMN IF NOT EXISTS current_medications TEXT NOT NULL DEFAULT '';

-- Tracks insurance claim events — immutable log.
CREATE TABLE insurance_claims (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  claim_type      TEXT        NOT NULL CHECK (claim_type IN ('MEDICAL','DEATH','FLIGHT','BAGGAGE','OTHER')),
  incident_date   DATE        NOT NULL,
  description     TEXT        NOT NULL,
  status          TEXT        NOT NULL DEFAULT 'FILED'
                              CHECK (status IN ('FILED','SUBMITTED','PROCESSING','SETTLED','REJECTED')),
  claim_amount    NUMERIC(14,2),
  settled_amount  NUMERIC(14,2),
  filed_by        TEXT        NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX insurance_claims_operator_idx ON insurance_claims(operator_id);
CREATE INDEX insurance_claims_pilgrim_idx  ON insurance_claims(pilgrim_id);

CREATE TRIGGER insurance_claims_set_updated_at
  BEFORE UPDATE ON insurance_claims
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS insurance_claims;
ALTER TABLE pilgrims
  DROP COLUMN IF EXISTS insurance_provider,
  DROP COLUMN IF EXISTS insurance_policy_no,
  DROP COLUMN IF EXISTS insurance_class,
  DROP COLUMN IF EXISTS blood_type,
  DROP COLUMN IF EXISTS chronic_conditions,
  DROP COLUMN IF EXISTS current_medications;
```

### D2 — sqlc Queries

**File: `apps/api/db/query/insurance.sql`**

```sql
-- name: CreateInsuranceClaim :one
INSERT INTO insurance_claims (pilgrim_id, operator_id, claim_type, incident_date, description, status, claim_amount, filed_by)
VALUES (@pilgrim_id, @operator_id, @claim_type, @incident_date, @description, @status, @claim_amount, @filed_by)
RETURNING *;

-- name: ListInsuranceClaims :many
SELECT ic.*, p.full_name, p.passport_number, p.insurance_provider, p.insurance_policy_no
FROM insurance_claims ic
JOIN pilgrims p ON p.id = ic.pilgrim_id
WHERE ic.operator_id = @operator_id
ORDER BY ic.created_at DESC;

-- name: UpdateInsuranceClaimStatus :one
UPDATE insurance_claims
SET status = @status, settled_amount = @settled_amount
WHERE id = @id AND operator_id = @operator_id
RETURNING *;

-- name: GetInsuranceClaimExportData :one
-- All fields needed for a complete insurance claim document.
SELECT
  p.full_name, p.passport_number, p.date_of_birth, p.gender, p.nationality,
  p.phone, p.emergency_contact_name, p.emergency_contact_phone,
  p.blood_type, p.chronic_conditions, p.current_medications,
  p.insurance_provider, p.insurance_policy_no, p.insurance_class,
  p.medical_notes,
  s.name AS season_name, s.start_date, s.end_date,
  o.name AS operator_name, o.license_number, o.phone AS operator_phone,
  ic.*
FROM insurance_claims ic
JOIN pilgrims p  ON p.id  = ic.pilgrim_id
JOIN seasons  s  ON s.id  = p.season_id
JOIN operators o ON o.id  = ic.operator_id
WHERE ic.id = @id AND ic.operator_id = @operator_id;
```

### D3 — Proto additions to pilgrim.proto

Add fields to `UpdatePilgrimRequest` and `Pilgrim` message:
```protobuf
// Insurance & medical fields — added to existing Pilgrim message
string insurance_provider  = 30;
string insurance_policy_no = 31;
string insurance_class     = 32;
string blood_type          = 33;
string chronic_conditions  = 34;
string current_medications = 35;
```

Add new `InsuranceService` in `proto/hajj/v1/insurance.proto`:
```protobuf
service InsuranceService {
  rpc CreateInsuranceClaim(CreateInsuranceClaimRequest) returns (InsuranceClaim);
  rpc ListInsuranceClaims(ListInsuranceClaimsRequest)   returns (ListInsuranceClaimsResponse);
  rpc UpdateInsuranceClaimStatus(UpdateInsuranceClaimStatusRequest) returns (InsuranceClaim);
}
```

### D4 — Frontend

**`/dashboard/insurance/page.tsx`** — Two tabs:
1. **Klaim Aktif** — table of all claims with status badges, Update Status action.
2. **Ekspor Data Klaim** — search pilgrim, show all insurance + medical fields, "Cetak / Export PDF" button using `window.print()` with print-optimized CSS.

Add insurance fields to pilgrim form (collapsible "Asuransi & Medis" section).

---

## Module E — Preparation Checklist

### Business Rule
Operator creates a preparation checklist template per season. Each jamaah gets their own
copy. Items can be: document submissions, medical requirements, payment milestones, physical
preparations. Operator can see completion rate per item across all jamaah.

### E1 — Migration 055: checklist_templates + pilgrim_checklists

```sql
-- +goose Up

-- Operator-defined template items per season.
CREATE TABLE checklist_templates (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  season_id   UUID        NOT NULL REFERENCES seasons(id)  ON DELETE CASCADE,
  title       TEXT        NOT NULL,
  description TEXT        NOT NULL DEFAULT '',
  category    TEXT        NOT NULL DEFAULT 'DOCUMENT'
              CHECK (category IN ('DOCUMENT','MEDICAL','PAYMENT','PREPARATION','OTHER')),
  is_required BOOLEAN     NOT NULL DEFAULT true,
  sort_order  INTEGER     NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX checklist_templates_season_idx ON checklist_templates(season_id, sort_order);

-- Per-pilgrim checklist state.
CREATE TABLE pilgrim_checklist_items (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id     UUID        NOT NULL REFERENCES checklist_templates(id) ON DELETE CASCADE,
  pilgrim_id      UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id     UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  is_completed    BOOLEAN     NOT NULL DEFAULT false,
  completed_at    TIMESTAMPTZ,
  completed_by    TEXT        NOT NULL DEFAULT '',  -- 'pilgrim' | 'operator' | user_id
  notes           TEXT        NOT NULL DEFAULT '',
  UNIQUE (template_id, pilgrim_id)
);

CREATE INDEX pilgrim_checklist_items_pilgrim_idx ON pilgrim_checklist_items(pilgrim_id);
CREATE INDEX pilgrim_checklist_items_operator_idx ON pilgrim_checklist_items(operator_id, template_id);

-- +goose Down
DROP TABLE IF EXISTS pilgrim_checklist_items;
DROP TABLE IF EXISTS checklist_templates;
```

### E2 — sqlc Queries

**File: `apps/api/db/query/checklist.sql`**

```sql
-- name: CreateChecklistTemplate :one
INSERT INTO checklist_templates (operator_id, season_id, title, description, category, is_required, sort_order)
VALUES (@operator_id, @season_id, @title, @description, @category, @is_required, @sort_order)
RETURNING *;

-- name: ListChecklistTemplates :many
SELECT * FROM checklist_templates
WHERE operator_id = @operator_id AND season_id = @season_id
ORDER BY sort_order ASC;

-- name: DeleteChecklistTemplate :exec
DELETE FROM checklist_templates WHERE id = @id AND operator_id = @operator_id;

-- name: UpsertPilgrimChecklistItem :one
INSERT INTO pilgrim_checklist_items (template_id, pilgrim_id, operator_id, is_completed, completed_at, completed_by, notes)
VALUES (@template_id, @pilgrim_id, @operator_id, @is_completed,
        CASE WHEN @is_completed THEN NOW() ELSE NULL END, @completed_by, @notes)
ON CONFLICT (template_id, pilgrim_id) DO UPDATE
  SET is_completed = EXCLUDED.is_completed,
      completed_at = CASE WHEN EXCLUDED.is_completed THEN NOW() ELSE NULL END,
      completed_by = EXCLUDED.completed_by,
      notes        = EXCLUDED.notes
RETURNING *;

-- name: GetPilgrimChecklist :many
-- Returns all template items for a pilgrim with completion state.
SELECT
  ct.id AS template_id, ct.title, ct.description, ct.category, ct.is_required, ct.sort_order,
  COALESCE(pci.is_completed, false) AS is_completed,
  pci.completed_at, pci.completed_by, pci.notes
FROM checklist_templates ct
LEFT JOIN pilgrim_checklist_items pci
  ON pci.template_id = ct.id AND pci.pilgrim_id = @pilgrim_id
WHERE ct.operator_id = @operator_id AND ct.season_id = @season_id
ORDER BY ct.sort_order ASC;

-- name: GetChecklistCompletionStats :many
-- Operator dashboard: completion rate per template item across all pilgrims.
SELECT
  ct.id, ct.title, ct.category, ct.is_required,
  COUNT(pci.id) FILTER (WHERE pci.is_completed)  AS completed_count,
  COUNT(p.id)                                     AS total_pilgrims
FROM checklist_templates ct
CROSS JOIN pilgrims p
LEFT JOIN pilgrim_checklist_items pci
  ON pci.template_id = ct.id AND pci.pilgrim_id = p.id
WHERE ct.operator_id = @operator_id
  AND ct.season_id   = @season_id
  AND p.operator_id  = @operator_id
  AND p.season_id    = @season_id
  AND p.status       = 'ACTIVE'
GROUP BY ct.id, ct.title, ct.category, ct.is_required
ORDER BY ct.sort_order;
```

### E3 — Proto: `proto/hajj/v1/checklist.proto`

```protobuf
syntax = "proto3";
package hajj.v1;
import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";
option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service ChecklistService {
  // Template management (operator)
  rpc CreateChecklistTemplate(CreateChecklistTemplateRequest) returns (ChecklistTemplate);
  rpc ListChecklistTemplates(ListChecklistTemplatesRequest)   returns (ListChecklistTemplatesResponse);
  rpc DeleteChecklistTemplate(DeleteChecklistTemplateRequest) returns (DeleteChecklistTemplateResponse);

  // Per-pilgrim checklist (operator + pilgrim app)
  rpc GetPilgrimChecklist(GetPilgrimChecklistRequest)     returns (GetPilgrimChecklistResponse);
  rpc UpdateChecklistItem(UpdateChecklistItemRequest)     returns (ChecklistItem);
  rpc GetChecklistStats(GetChecklistStatsRequest)         returns (GetChecklistStatsResponse);

  // Pilgrim-app facing — public
  rpc GetMyChecklist(GetMyChecklistRequest)               returns (GetMyChecklistResponse);
  rpc CompleteMyChecklistItem(CompleteMyChecklistItemRequest) returns (ChecklistItem);
}

message ChecklistTemplate {
  string id          = 1;
  string title       = 2;
  string description = 3;
  string category    = 4;
  bool   is_required = 5;
  int32  sort_order  = 6;
}

message ChecklistItem {
  string  template_id   = 1;
  string  title         = 2;
  string  description   = 3;
  string  category      = 4;
  bool    is_required   = 5;
  bool    is_completed  = 6;
  string  completed_by  = 7;
  string  notes         = 8;
  google.protobuf.Timestamp completed_at = 9;
}

message ChecklistStat {
  string template_id      = 1;
  string title            = 2;
  string category         = 3;
  bool   is_required      = 4;
  int32  completed_count  = 5;
  int32  total_pilgrims   = 6;
}

message CreateChecklistTemplateRequest {
  string season_id   = 1 [(buf.validate.field).string.min_len = 1];
  string title       = 2 [(buf.validate.field).string.min_len = 1];
  string description = 3;
  string category    = 4;
  bool   is_required = 5;
  int32  sort_order  = 6;
}
message ListChecklistTemplatesRequest   { string season_id = 1; }
message ListChecklistTemplatesResponse  { repeated ChecklistTemplate templates = 1; }
message DeleteChecklistTemplateRequest  { string id = 1; }
message DeleteChecklistTemplateResponse {}
message GetPilgrimChecklistRequest      { string pilgrim_id = 1; string season_id = 2; }
message GetPilgrimChecklistResponse     { repeated ChecklistItem items = 1; }
message UpdateChecklistItemRequest      { string template_id = 1; string pilgrim_id = 2; bool is_completed = 3; string notes = 4; }
message GetChecklistStatsRequest        { string season_id = 1; }
message GetChecklistStatsResponse       { repeated ChecklistStat stats = 1; }
// Pilgrim app — authenticated by app_access_code
message GetMyChecklistRequest           { string app_access_code = 1; }
message GetMyChecklistResponse          { repeated ChecklistItem items = 1; }
message CompleteMyChecklistItemRequest  { string app_access_code = 1; string template_id = 2; string notes = 3; }
```

Add to publicProcedures + rateLimitedProcedures:
```go
"/hajj.v1.ChecklistService/GetMyChecklist":          true,
"/hajj.v1.ChecklistService/CompleteMyChecklistItem": true,
// Rate: same as PilgrimAppService (4 per minute per IP)
```

### E4 — Frontend

**`/dashboard/seasons/[id]/checklist/page.tsx`** — Two tabs:
1. **Template** — add/delete items, drag to reorder (use sort_order). Predefined quick-add buttons: "Paspor", "Foto 4x6", "Vaksin Meningitis", "Pelunasan Biaya".
2. **Progress** — table of items with completion bar per item (e.g. "45/60 jamaah sudah selesai").

**Pilgrim App `/pilgrim` page** — Add "Persiapan" tab showing checklist with checkboxes. Tapping item calls `CompleteMyChecklistItem`. Required items shown first, with red badge if incomplete.

---

## Module F — Find My Group (Jamaah Terpisah)

### Business Rule
When a jamaah gets separated from their group in Masjidil Haram, they can tap "Saya Tersesat"
in the pilgrim app. This sends their current GPS location to their group leader and triggers
a push notification. Leader can see the location and call the jamaah back to the group.
Different from SOS (emergency) — this is a non-emergency "find me" request.

This reuses existing tables: `pilgrim_location` (already exists from migration 031),
`sos_alerts` pattern for notification delivery.

### F1 — Migration 056: lost_reports

```sql
-- +goose Up
CREATE TABLE lost_reports (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  pilgrim_id  UUID        NOT NULL REFERENCES pilgrims(id) ON DELETE CASCADE,
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  group_id    UUID        REFERENCES groups(id),
  latitude    FLOAT8      NOT NULL,
  longitude   FLOAT8      NOT NULL,
  last_known_location TEXT NOT NULL DEFAULT '',
  status      TEXT        NOT NULL DEFAULT 'LOST'
              CHECK (status IN ('LOST','FOUND','RESOLVED')),
  resolved_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX lost_reports_group_idx    ON lost_reports(group_id, status);
CREATE INDEX lost_reports_operator_idx ON lost_reports(operator_id, status);

-- +goose Down
DROP TABLE IF EXISTS lost_reports;
```

### F2 — sqlc Queries

**File: `apps/api/db/query/lost_report.sql`**

```sql
-- name: CreateLostReport :one
INSERT INTO lost_reports (pilgrim_id, operator_id, group_id, latitude, longitude, last_known_location)
SELECT
  p.id, p.operator_id, p.group_id, @latitude, @longitude, @last_known_location
FROM pilgrims p
WHERE p.app_access_code = @app_access_code
RETURNING *;

-- name: ResolveLostReport :exec
UPDATE lost_reports
SET status = 'FOUND', resolved_at = NOW()
WHERE id = @id AND operator_id = @operator_id;

-- name: ListActiveLostReports :many
SELECT lr.*, p.full_name, p.phone, g.name AS group_name
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
LEFT JOIN groups g ON g.id = lr.group_id
WHERE lr.operator_id = @operator_id AND lr.status = 'LOST'
ORDER BY lr.created_at DESC;

-- name: ListGroupLostReports :many
SELECT lr.*, p.full_name, p.phone
FROM lost_reports lr
JOIN pilgrims p ON p.id = lr.pilgrim_id
WHERE lr.group_id = @group_id AND lr.status = 'LOST'
ORDER BY lr.created_at DESC;
```

### F3 — Proto: add to existing sos.proto or new lost_report.proto

```protobuf
syntax = "proto3";
package hajj.v1;
import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";
option go_package = "github.com/hajj-saas/api/internal/gen/hajj/v1;hajjv1";

service LostReportService {
  // Public — pilgrim-facing, authenticated by app_access_code
  rpc ReportLost(ReportLostRequest)              returns (LostReport);

  // Authenticated — operator + leader facing
  rpc ListActiveLostReports(ListActiveLostReportsRequest) returns (ListActiveLostReportsResponse);
  rpc ResolveLostReport(ResolveLostReportRequest)         returns (ResolveLostReportResponse);
  rpc ListGroupLostReports(ListGroupLostReportsRequest)   returns (ListGroupLostReportsResponse);
}

message LostReport {
  string id                    = 1;
  string pilgrim_id            = 2;
  string pilgrim_name          = 3;
  string pilgrim_phone         = 4;
  string group_name            = 5;
  double latitude              = 6;
  double longitude             = 7;
  string last_known_location   = 8;
  string status                = 9;
  google.protobuf.Timestamp created_at  = 10;
  google.protobuf.Timestamp resolved_at = 11;
}

message ReportLostRequest {
  string app_access_code      = 1 [(buf.validate.field).string.min_len = 1];
  double latitude              = 2;
  double longitude             = 3;
  string last_known_location  = 4;
}

message ListActiveLostReportsRequest  {}
message ListActiveLostReportsResponse { repeated LostReport reports = 1; }
message ResolveLostReportRequest      { string id = 1; }
message ResolveLostReportResponse     {}
message ListGroupLostReportsRequest   { string group_id = 1; }
message ListGroupLostReportsResponse  { repeated LostReport reports = 1; }
```

Add to publicProcedures + rateLimitedProcedures:
```go
"/hajj.v1.LostReportService/ReportLost": true,
// Rate: 3 per hour per IP — once lost, you don't spam this. But allow retries.
"/hajj.v1.LostReportService/ReportLost": rate.Every(time.Hour / 3),
```

After `CreateLostReport`, service calls `notification.NewFirebasePusher` to push
"🟡 [Nama Jamaah] melaporkan diri terpisah dari rombongan" to group leader's FCM token.

### F4 — Frontend

**Pilgrim App `/pilgrim`** — Add "Saya Tersesat" floating button (bottom center, yellow/orange, large tap target — accessible for elderly). Tap → confirm dialog → if user confirms, get GPS via `navigator.geolocation.getCurrentPosition()` → call `ReportLost` → show "Laporan terkirim ke koordinator Anda".

**Leader App `/leader`** — Add red badge + alert banner when active lost reports exist for their group. Show map link (opens Google Maps at latitude/longitude coordinates).

**Operator Dashboard `/dashboard/sos` or new `/dashboard/lost`** — Table of active lost reports with Resolve button.

---

## Module G — Digital Certificate & Trip Summary

### Business Rule
After a season ends, each jamaah gets a digital certificate of completion + a structured
trip summary (dates, hotel names, group, guide). Operator can customize certificate template.
Certificate is generated client-side as a printable HTML page — no server-side PDF needed.
Jamaah access it from the pilgrim app post-trip. Operator can share link from dashboard.

No new migration needed — all data already exists in pilgrims, seasons, hotels, groups tables.

### G1 — sqlc Query

**File: `apps/api/db/query/certificate.sql`**

```sql
-- name: GetCertificateData :one
-- All fields needed to render a complete trip certificate.
SELECT
  p.id, p.full_name, p.passport_number, p.nationality,
  p.app_access_code,
  s.name AS season_name, s.type AS season_type,
  s.start_date, s.end_date,
  o.name AS operator_name, o.license_number,
  g.name AS group_name,
  COALESCE(u.name, '') AS leader_name,
  -- Hotels visited (comma-separated via aggregation)
  STRING_AGG(DISTINCT h.name, ', ') AS hotels_visited,
  -- Makkah hotels
  STRING_AGG(DISTINCT h.name FILTER (WHERE h.city = 'Makkah'), ', ') AS makkah_hotels,
  -- Madinah hotels
  STRING_AGG(DISTINCT h.name FILTER (WHERE h.city = 'Madinah'), ', ') AS madinah_hotels
FROM pilgrims p
JOIN seasons   s  ON s.id  = p.season_id
JOIN operators o  ON o.id  = p.operator_id
LEFT JOIN groups g ON g.id = p.group_id
LEFT JOIN "user" u ON u.id = g.leader_id
LEFT JOIN room_allocations ra ON ra.pilgrim_id = p.id
LEFT JOIN rooms r  ON r.id  = ra.room_id
LEFT JOIN hotels h ON h.id  = r.hotel_id
WHERE p.app_access_code = @app_access_code
GROUP BY p.id, p.full_name, p.passport_number, p.nationality, p.app_access_code,
         s.name, s.type, s.start_date, s.end_date, o.name, o.license_number,
         g.name, u.name;
```

### G2 — Proto: add to pilgrim_app.proto

```protobuf
// Add to PilgrimAppService
rpc GetMyCertificate(GetMyCertificateRequest) returns (CertificateData);

message CertificateData {
  string pilgrim_name    = 1;
  string passport_number = 2;
  string nationality     = 3;
  string season_name     = 4;
  string season_type     = 5;   // HAJJ | UMRAH
  google.protobuf.Timestamp start_date     = 6;
  google.protobuf.Timestamp end_date       = 7;
  string operator_name   = 8;
  string license_number  = 9;
  string group_name      = 10;
  string leader_name     = 11;
  string hotels_visited  = 12;
  string makkah_hotels   = 13;
  string madinah_hotels  = 14;
}

message GetMyCertificateRequest {
  string app_access_code = 1 [(buf.validate.field).string.min_len = 1];
}
```

Add to publicProcedures + rateLimitedProcedures:
```go
"/hajj.v1.PilgrimAppService/GetMyCertificate": true,
// Rate: loose — a jamaah might share and view multiple times
"/hajj.v1.PilgrimAppService/GetMyCertificate": rate.Every(time.Minute / 4),
```

### G3 — Frontend: Certificate Page

**`apps/web/app/certificate/[code]/page.tsx`** — Public page, no auth.

```tsx
"use client";
import { use, useEffect, useState } from "react";
import { createPilgrimAppClient } from "@/lib/rpc";

export default function CertificatePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = use(params);
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    createPilgrimAppClient().getMyCertificate({ appAccessCode: code })
      .then(setData).finally(() => setLoading(false));
  }, [code]);

  if (loading) return <div style={{ textAlign: "center", padding: 60 }}>Memuat sertifikat...</div>;
  if (!data) return <div style={{ textAlign: "center", padding: 60 }}>Sertifikat tidak ditemukan.</div>;

  const startStr = data.startDate?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) ?? "-";
  const endStr   = data.endDate?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) ?? "-";
  const isHajj   = data.seasonType === "HAJJ";

  return (
    <>
      {/* Print-optimized layout */}
      <style>{`
        @media print {
          .no-print { display: none !important; }
          body { margin: 0; }
        }
        @import url('https://fonts.googleapis.com/css2?family=Playfair+Display:wght@400;700&family=Plus+Jakarta+Sans:wght@400;600;700&display=swap');
      `}</style>

      <div style={{ maxWidth: 800, margin: "0 auto", padding: "40px 32px", fontFamily: "'Plus Jakarta Sans', sans-serif" }}>
        {/* Print button */}
        <div className="no-print" style={{ textAlign: "right", marginBottom: 24 }}>
          <button onClick={() => window.print()} style={{ padding: "10px 20px", background: "var(--color-emerald-900)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, cursor: "pointer" }}>
            🖨 Cetak / Simpan PDF
          </button>
        </div>

        {/* Certificate */}
        <div style={{
          border: "8px solid var(--color-gold-500)",
          borderRadius: 16,
          padding: "48px 56px",
          textAlign: "center",
          background: "var(--color-cream-100)",
          position: "relative",
        }}>
          {/* Inner border decoration */}
          <div style={{ border: "2px solid var(--color-gold-300)", borderRadius: 10, padding: "32px 40px" }}>

            <p style={{ color: "var(--color-gold-700)", fontSize: 13, fontWeight: 700, letterSpacing: ".12em", margin: "0 0 4px" }}>
              SERTIFIKAT {isHajj ? "HAJI" : "UMRAH"}
            </p>

            <p style={{ fontFamily: "'Playfair Display', serif", fontSize: 13, color: "var(--color-warm-500)", margin: "0 0 32px" }}>
              Certificate of Completion
            </p>

            <p style={{ color: "var(--color-warm-600)", fontSize: 14, margin: "0 0 8px" }}>Diberikan kepada</p>
            <h1 style={{ fontFamily: "'Playfair Display', serif", fontSize: 36, fontWeight: 700, color: "var(--color-emerald-900)", margin: "0 0 4px" }}>
              {data.pilgrimName}
            </h1>
            <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "0 0 32px" }}>
              Paspor: {data.passportNumber} · {data.nationality}
            </p>

            <p style={{ color: "var(--color-warm-600)", fontSize: 14, margin: "0 0 4px" }}>
              telah melaksanakan ibadah <strong>{isHajj ? "Haji" : "Umrah"}</strong>
            </p>
            <p style={{ fontFamily: "'Playfair Display', serif", fontSize: 22, color: "var(--color-emerald-800)", margin: "0 0 32px", fontWeight: 700 }}>
              {data.seasonName}
            </p>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, textAlign: "left", marginBottom: 32 }}>
              {[
                { label: "Tanggal Berangkat", value: startStr },
                { label: "Tanggal Kembali",   value: endStr },
                { label: "Rombongan",          value: data.groupName || "-" },
                { label: "Pembimbing",         value: data.leaderName || "-" },
                { label: "Hotel Makkah",       value: data.makkahHotels || "-" },
                { label: "Hotel Madinah",      value: data.madinahHotels || "-" },
              ].map(r => (
                <div key={r.label} style={{ padding: "12px 16px", background: "#fff", borderRadius: 8, border: "1px solid var(--color-cream-400)" }}>
                  <p style={{ margin: "0 0 2px", fontSize: 11, color: "var(--color-warm-400)", fontWeight: 600, textTransform: "uppercase", letterSpacing: ".06em" }}>{r.label}</p>
                  <p style={{ margin: 0, fontSize: 14, fontWeight: 700, color: "var(--color-warm-800)" }}>{r.value}</p>
                </div>
              ))}
            </div>

            <div style={{ borderTop: "1px solid var(--color-cream-400)", paddingTop: 24 }}>
              <p style={{ color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 4px" }}>Diselenggarakan oleh</p>
              <p style={{ fontWeight: 700, fontSize: 16, color: "var(--color-emerald-900)", margin: "0 0 2px" }}>{data.operatorName}</p>
              {data.licenseNumber && <p style={{ color: "var(--color-warm-400)", fontSize: 12, margin: 0 }}>No. Izin: {data.licenseNumber}</p>}
            </div>

          </div>
        </div>
      </div>
    </>
  );
}
```

**Pilgrim App** — After season end date, show "Unduh Sertifikat" button in pilgrim app that opens `/certificate/[app_access_code]` in a new tab.

**Operator Dashboard** — In pilgrim detail page, add "Bagikan Sertifikat" button that copies `${origin}/certificate/${pilgrim.appAccessCode}` to clipboard.

---

## Execution Order

```
Step  1  →  Apply all migrations in order:
            goose -dir apps/api/db/migrations postgres "$DATABASE_URL" up
            (applies 051 through 056)

Step  2  →  sqlc generate (from apps/api/)
            Must complete before any Go implementation.

Step  3  →  pnpm buf:generate (from root)
            Generates Go + TS code from all new .proto files.

Step  4  →  Implement Go — strictly in layer order (repository first, then service, then handler):
            Module A: repository/cashflow.go → service/cashflow.go → handler/cashflow.go
            Module B: repository/vendor.go → service/vendor.go → handler/vendor.go
            Module C: repository/staff_schedule.go → service/staff_schedule.go → handler/staff_schedule.go
            Module D: repository/insurance.go → service/insurance.go → handler/insurance.go
            Module E: repository/checklist.go → service/checklist.go → handler/checklist.go
            Module F: repository/lost_report.go → service/lost_report.go → handler/lost_report.go
            Module G: Add GetMyCertificateData to repository/pilgrim_app.go and service/pilgrim_app.go

Step  5  →  Update auth.go: add all new public RPCs to publicProcedures
Step  6  →  Update ratelimit.go: add all new public RPCs to rateLimitedProcedures
Step  7  →  Update main.go: wire all 7 new handlers + any new public endpoints

Step  8  →  go build ./... — MUST be zero errors before touching frontend

Step  9  →  Implement all frontend pages:
            /dashboard/cashflow/page.tsx
            /dashboard/vendors/page.tsx
            /dashboard/schedule/page.tsx
            /dashboard/insurance/page.tsx
            /dashboard/seasons/[id]/checklist/page.tsx
            /certificate/[code]/page.tsx
            /app/track (add "Saya Tersesat" button to pilgrim app)
            /app/pilgrim (add "Persiapan" checklist tab)
            /app/pilgrim (add "Sertifikat" tab when season ended)

Step 10  →  Add nav entries for all new operator dashboard pages
Step 11  →  Add createXxxClient functions to apps/web/lib/rpc.ts for all new services
Step 12  →  pnpm --filter web dev
Step 13  →  Run all verification checks below
```

---

## Verification Checklist

### Module A — Cash Flow
- [ ] `GetCashFlowSummary` net_position = total_collected − total_outstanding (computed in service, not DB)
- [ ] Adding vendor payment with past due_date shows OVERDUE status warning in UI
- [ ] Danger zone banner appears when net_position < due_next_30_days
- [ ] Mark as Paid updates status + paid_at in DB, disappears from outstanding total
- [ ] All amounts computed server-side — no client-supplied calculation accepted

### Module B — Vendor SLA
- [ ] SLA health = OVERDUE when confirmation_deadline < today AND confirmed_units < committed_units
- [ ] SLA health = AT_RISK when deadline within 7 days
- [ ] Contract event log is immutable — no UPDATE on vendor_contract_events
- [ ] total_value is GENERATED ALWAYS (DB computed) — never writable from application

### Module C — Staff Schedule
- [ ] Staff assigned to same kloter twice → ON CONFLICT DO UPDATE (no duplicate)
- [ ] `ListMyAssignments` scoped to current user's Better Auth user_id
- [ ] Removing staff from kloter does not affect other kloters (correct WHERE clause)
- [ ] Kloter with 0 staff shows red indicator in schedule grid

### Module D — Insurance
- [ ] `GetInsuranceClaimExportData` includes all fields needed for a real insurance form
- [ ] insurance_claims rows: no UPDATE to claim amount after SETTLED (status only)
- [ ] Insurance fields visible in pilgrim detail form (collapsible section)
- [ ] Export/print renders correctly with window.print()

### Module E — Checklist
- [ ] `GetPilgrimChecklist` returns all template items even if no pilgrim_checklist_items row exists (LEFT JOIN)
- [ ] `CompleteMyChecklistItem` — app_access_code validated against DB, pilgrim_id never trusted from client
- [ ] Stats: completed_count / total_pilgrims shows correct percentage
- [ ] Deleting template cascades to delete pilgrim_checklist_items (ON DELETE CASCADE)
- [ ] Public RPCs in both publicProcedures AND rateLimitedProcedures

### Module F — Find My Group
- [ ] `ReportLost` derives pilgrim_id and group_id from app_access_code server-side — not from request
- [ ] Firebase push notification sent to leader on new lost report
- [ ] "Saya Tersesat" button in pilgrim app only visible (or prominent) when GPS available
- [ ] Resolving report sets resolved_at = NOW() in DB
- [ ] Rate limit: more than 3 ReportLost per hour per IP returns 429

### Module G — Digital Certificate
- [ ] `/certificate/[code]` loads without auth cookie
- [ ] Invalid app_access_code returns 404-equivalent, no data leak
- [ ] window.print() renders certificate without nav buttons (no-print class applied)
- [ ] Hotels list only shows hotels the pilgrim actually had room_allocation records for
- [ ] Certificate page does NOT expose passport number in URL — only app_access_code

### General
- [ ] `go vet ./...` — zero warnings
- [ ] `pnpm typecheck` — zero errors
- [ ] All 7 new services wired in main.go
- [ ] All public RPCs present in BOTH publicProcedures AND rateLimitedProcedures
- [ ] No raw pgx error messages reachable from any public endpoint
- [ ] `go test ./internal/service/... -count=1` — zero failures
```
