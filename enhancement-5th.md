# Tawafiq Hub — Agent (Tour Leader) Portal
## Technical Implementation Document

---

## Context & Terminology

**IMPORTANT TERMINOLOGY CHANGE — apply across all UI:**
- "Agen" → **"Tour Leader"** in all agent-facing pages and operator dashboard
- "Ketua Rombongan" / "Group Leader" → **"Muttawwif"** in leader app (`/leader`) and operator dashboard
- Backend field names (`agent`, `group_leader`, `leader_id`) stay unchanged — this is display-only
- These changes reflect the real roles in the Haji & Umrah industry:
  - **Tour Leader** = Indonesian trip organizer/seller who refers jamaah, sometimes travels with group
  - **Muttawwif** = Local Saudi guide assigned as group leader on the ground

---

## What Backend Already Has (DO NOT REBUILD)

The following are fully implemented — read them, understand them, do not recreate:

- `AgentService.GetMyWallet` — resolves caller's wallet via `linked_user_id`, returns balance + transactions
- `AgentService.RequestPayout` — agent requests withdrawal, validates against available balance
- `AgentService.ListPayoutRequests` — operator inbox of pending payout requests
- `AgentService.RejectPayoutRequest` — operator rejects with note
- `AgentService.RecordAgentPayout` — operator disburses payment, links to request_id
- `AgentService.ListAgentPayouts` — operator view: commission earned vs disbursed per agent
- sqlc: `GetAgentByLinkedUser`, `GetAgentByReferralCode`, `CreateAgentApplication`
- DB columns: `referral_code`, `tier`, `referred_by_agent_id`, `linked_user_id` on agents table

---

## Architecture Overview

```
Stack : Turborepo monorepo, Next.js 15 (apps/web :3131), Go Connect RPC :8131, PostgreSQL :5434
Auth  : Better Auth. Agent logs in at same /sign-in page as operator staff.
        Agent is invited to operator's Better Auth org (role: "member") by operator.
        agents.linked_user_id = Better Auth "user"(id) — the link between identity and agent record.
        resolveLandingPath detects linked_agent → redirects to /agent portal.
Layers: handler/ → service/ → repository/ — never skip.
Tenant: All queries scoped by operatorID from ctx. Agent RPCs resolve operatorID from org membership.
```

---

## Module A — Identity Service: Add linked_agent to MyAccess

### Why
`resolveLandingPath` needs to know "is this Better Auth user also an agent?" to redirect
correctly. Currently `MyAccess` has `linked_pilgrim` and `leader_groups` but nothing for agents.

### A1 — Proto update: `proto/hajj/v1/identity.proto`

Add to `MyAccess` message:

```protobuf
// Tour Leader (Agent) — set when this Better Auth identity is linked to
// an agent record via agents.linked_user_id. A user can be simultaneously
// an org member AND a linked agent (Tour Leaders are invited as org members).
AgentSummary linked_agent = 7;

message AgentSummary {
  string id             = 1;
  string name           = 2;
  string referral_code  = 3;
  bool   is_active      = 4;
}
```

Run: `pnpm buf:generate`

### A2 — Service update: `apps/api/internal/service/identity.go`

Inside `GetMyAccess`, after the existing leader_groups lookup, add:

```go
// Resolve linked agent — a Better Auth org member may simultaneously be
// a Tour Leader (agent). Look up by linked_user_id within this operator's
// agents. Non-fatal if not found — not every org member is an agent.
if access.OperatorID != "" {
    operatorUUID, err := uuid.Parse(access.OperatorID)
    if err == nil {
        agent, err := s.agentRepo.GetByLinkedUser(ctx, operatorUUID, userID)
        if err == nil && agent.IsActive {
            access.LinkedAgent = &hajjv1.AgentSummary{
                Id:           agent.ID.String(),
                Name:         agent.Name,
                ReferralCode: agent.ReferralCode,
                IsActive:     agent.IsActive,
            }
        }
        // pgx.ErrNoRows = not an agent, silently ignore.
    }
}
```

Add `agentRepo *repository.AgentRepository` to IdentityService struct and inject in `NewIdentityService`.
Wire in `main.go` — IdentityService already receives agentRepository, or add it.

### A3 — Frontend: `apps/web/lib/access-cache.ts`

The `MyAccess` TypeScript type (generated from proto) will now include `linkedAgent`.
Update any type assertions or usage in `access-cache.ts` to include the new field.
No logic change needed — proto-gen handles the mapping.

### A4 — `apps/web/lib/post-login.ts` — Update resolveLandingPath

```ts
export async function resolveLandingPath(): Promise<string> {
  const access = await getMyAccessCached();

  if (access.isOrgMember) {
    const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
    if (!session.data?.session?.activeOrganizationId) {
      const organizations = await authClient.organization.list();
      const organization = organizations.data?.[0];
      if (organization) {
        await authClient.organization.setActive({ organizationId: organization.id });
      }
    }
  }

  const isAdmin = access.isOrgMember && (access.orgRole === "owner" || access.orgRole === "admin");
  if (isAdmin) return "/dashboard";
  if (access.leaderGroups.length > 0) return "/leader";

  // Tour Leader — org member with linked agent record but not owner/admin.
  // Must come after leader check: a leader is also an agent (per migration 037
  // comment "Leaders automatically become agents"), but their primary surface
  // is /leader, not /agent.
  if (access.isOrgMember && access.linkedAgent?.isActive) return "/agent";

  if (access.isOrgMember) return "/dashboard";
  if (access.linkedPilgrim) return "/pilgrim";
  return "/onboarding";
}
```

---

## Module B — Agent Portal: `/agent`

### B1 — Route guard: `apps/web/app/agent/layout.tsx`

```tsx
"use client";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getMyAccessCached } from "@/lib/access-cache";

export default function AgentLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  useEffect(() => {
    getMyAccessCached().then(access => {
      if (!access.linkedAgent?.isActive) {
        router.replace("/sign-in");
      }
    });
  }, [router]);
  return <>{children}</>;
}
```

### B2 — Main portal: `apps/web/app/agent/page.tsx`

Four-tab portal. Each tab is a separate component for clarity.

```tsx
"use client";
import { useState } from "react";
import { IconWallet, IconUsers, IconLink, IconClock } from "@tabler/icons-react";
import AgentWalletTab      from "@/components/agent/AgentWalletTab";
import AgentJamaahTab      from "@/components/agent/AgentJamaahTab";
import AgentReferralTab    from "@/components/agent/AgentReferralTab";
import AgentPayoutTab      from "@/components/agent/AgentPayoutTab";
import { authClient }      from "@/lib/auth-client";

const TABS = [
  { id: "wallet",   label: "Dompet Komisi", icon: IconWallet },
  { id: "jamaah",   label: "Jamaah Saya",   icon: IconUsers },
  { id: "referral", label: "Link Referral", icon: IconLink },
  { id: "payout",   label: "Pencairan",     icon: IconClock },
] as const;
type TabId = typeof TABS[number]["id"];

export default function AgentPortalPage() {
  const [tab, setTab] = useState<TabId>("wallet");
  const { data: session } = authClient.useSession();

  const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0 28px", flexWrap: "wrap" };
  const tabActive: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center", cursor: "pointer" };
  const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center", cursor: "pointer" };

  return (
    <main style={{ maxWidth: 860, margin: "0 auto", padding: "32px 24px" }}>
      <p style={{ color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" }}>TOUR LEADER PORTAL</p>
      <h1 style={{ fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 4px" }}>
        Selamat datang, {session?.user?.name?.split(" ")[0]}
      </h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Kelola komisi, jamaah referral, dan pencairan dana Anda.</p>
      <div className="gold-divider" />
      <div style={tabBar}>
        {TABS.map(t => (
          <button key={t.id} onClick={() => setTab(t.id)} style={tab === t.id ? tabActive : tabInactive}>
            <t.icon size={17} />{t.label}
          </button>
        ))}
      </div>
      {tab === "wallet"   && <AgentWalletTab />}
      {tab === "jamaah"   && <AgentJamaahTab />}
      {tab === "referral" && <AgentReferralTab />}
      {tab === "payout"   && <AgentPayoutTab />}
    </main>
  );
}
```

### B3 — `apps/web/components/agent/AgentWalletTab.tsx`

Calls `AgentService.GetMyWallet`. Shows:
- Three KPI cards: **Total Komisi** (total_earned_idr), **Tersedia** (available_idr, green), **Menunggu Pencairan** (pending_requested_idr, orange)
- Transaction history table with type badge:
  - CREDIT → green "Komisi Diterima" + product name + amount
  - DEBIT → red "Dicairkan" + method + amount
  - PENDING_REQUEST → orange "Menunggu Persetujuan" + amount

```tsx
"use client";
import { useEffect, useState } from "react";
import { createAgentClient } from "@/lib/rpc";
import { AgentWallet, WalletTransactionType } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";

export default function AgentWalletTab() {
  const [wallet, setWallet] = useState<AgentWallet | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    createAgentClient().getMyWallet({}).then(setWallet).finally(() => setLoading(false));
  }, []);

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat dompet...</p>;
  if (!wallet) return <p style={{ color: "var(--color-danger-600)" }}>Gagal memuat data.</p>;

  const fmt = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

  const typeLabel: Record<number, { label: string; color: string }> = {
    [WalletTransactionType.CREDIT]:          { label: "Komisi Diterima", color: "var(--color-emerald-700)" },
    [WalletTransactionType.DEBIT]:           { label: "Dicairkan",       color: "var(--color-danger-600)" },
    [WalletTransactionType.PENDING_REQUEST]: { label: "Menunggu Persetujuan", color: "var(--color-gold-700)" },
  };

  return (
    <div>
      {/* KPI cards */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(200px,1fr))", gap: 16, marginBottom: 28 }}>
        {[
          { label: "Total Komisi",        value: fmt(wallet.totalEarnedIdr),    color: "var(--color-warm-800)" },
          { label: "Tersedia",            value: fmt(wallet.availableIdr),      color: "var(--color-emerald-700)" },
          { label: "Menunggu Pencairan",  value: fmt(wallet.pendingRequestedIdr), color: "var(--color-gold-700)" },
        ].map(c => (
          <div key={c.label} style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "20px 18px" }}>
            <p style={{ margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 }}>{c.label}</p>
            <p style={{ margin: 0, fontSize: 22, fontWeight: 700, color: c.color }}>{c.value}</p>
          </div>
        ))}
      </div>

      {/* Transaction history */}
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
        <h3 style={{ margin: "0 0 16px", fontSize: 15, fontWeight: 700 }}>Riwayat Transaksi</h3>
        {wallet.transactions.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 14 }}>Belum ada transaksi.</p>}
        <div style={{ display: "grid", gap: 10 }}>
          {wallet.transactions.map(tx => {
            const meta = typeLabel[tx.type] ?? { label: "Transaksi", color: "var(--color-warm-600)" };
            const isDebit = tx.type === WalletTransactionType.DEBIT;
            return (
              <div key={tx.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--color-cream-300)" }}>
                <div>
                  <p style={{ margin: 0, fontSize: 13, fontWeight: 600, color: meta.color }}>{meta.label}</p>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{tx.description} · {tx.createdAt?.toDate().toLocaleDateString("id-ID")}</p>
                </div>
                <p style={{ margin: 0, fontWeight: 700, fontSize: 14, color: isDebit ? "var(--color-danger-600)" : "var(--color-emerald-700)" }}>
                  {isDebit ? "-" : "+"}{fmt(tx.amountIdr)}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
```

### B4 — `apps/web/components/agent/AgentJamaahTab.tsx`

New sqlc query needed (see B4a below). Shows list of pilgrims referred by this agent with payment
and document status.

**B4a — New sqlc query: `apps/api/db/query/agent.sql` — append:**

```sql
-- name: ListMyPilgrims :many
-- Agent-facing: list pilgrims referred by this agent, across all seasons.
SELECT
  p.id, p.full_name, p.passport_number, p.gender, p.payment_status,
  p.documents_passport AND p.documents_photo AND p.documents_vaccine AS docs_complete,
  p.status AS pilgrim_status,
  s.name AS season_name,
  s.start_date AS departure_date
FROM pilgrims p
JOIN seasons s ON s.id = p.season_id
WHERE p.agent_id = @agent_id
  AND p.operator_id = @operator_id
ORDER BY s.start_date DESC, p.full_name ASC;
```

**B4b — Add to AgentService:**

```go
func (s *AgentService) ListMyPilgrims(ctx context.Context, orgID, userID string) ([]*hajjv1.AgentPilgrim, error) {
    op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
    if err != nil {
        return nil, serviceError("AgentService.ListMyPilgrims", err)
    }
    agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
    if err != nil {
        return nil, serviceError("AgentService.ListMyPilgrims", err)
    }
    rows, err := s.agentRepository.ListMyPilgrims(ctx, op.ID, agent.ID)
    if err != nil {
        return nil, serviceError("AgentService.ListMyPilgrims", err)
    }
    // map rows to proto...
    return out, nil
}
```

**B4c — Add to agent.proto:**

```protobuf
// Add to AgentService
rpc ListMyPilgrims(ListMyPilgrimsRequest) returns (ListMyPilgrimsResponse);

message AgentPilgrim {
  string id              = 1;
  string full_name       = 2;
  string passport_number = 3;
  string gender          = 4;
  string payment_status  = 5;
  bool   docs_complete   = 6;
  string pilgrim_status  = 7;
  string season_name     = 8;
  google.protobuf.Timestamp departure_date = 9;
}
message ListMyPilgrimsRequest  {}
message ListMyPilgrimsResponse { repeated AgentPilgrim pilgrims = 1; }
```

**B4d — Frontend tab:**

Table showing all referred pilgrims with:
- Nama, Season, Status Bayar (badge: PAID=green, DP=orange, UNPAID=red), Dokumen (✅/⚠), Status (ACTIVE/CANCELLED)
- Total row count at top: "X jamaah referral Anda"
- Filter by season (dropdown)

```tsx
"use client";
import { useEffect, useState } from "react";
import { createAgentClient } from "@/lib/rpc";

export default function AgentJamaahTab() {
  const [pilgrims, setPilgrims] = useState<any[]>([]);
  const [seasonFilter, setSeasonFilter] = useState("ALL");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    createAgentClient().listMyPilgrims({}).then(r => setPilgrims(r.pilgrims)).finally(() => setLoading(false));
  }, []);

  const seasons = ["ALL", ...Array.from(new Set(pilgrims.map(p => p.seasonName)))];
  const filtered = seasonFilter === "ALL" ? pilgrims : pilgrims.filter(p => p.seasonName === seasonFilter);

  const payBadge: Record<string, { label: string; color: string; bg: string }> = {
    PAID:   { label: "Lunas",       color: "#065f46", bg: "#d1fae5" },
    DP:     { label: "DP",          color: "#92400e", bg: "#fef3c7" },
    UNPAID: { label: "Belum Bayar", color: "#991b1b", bg: "#fee2e2" },
  };

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>;

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <p style={{ margin: 0, fontWeight: 600 }}>{filtered.length} jamaah</p>
        <select value={seasonFilter} onChange={e => setSeasonFilter(e.target.value)}
          style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 13 }}>
          {seasons.map(s => <option key={s} value={s}>{s === "ALL" ? "Semua Musim" : s}</option>)}
        </select>
      </div>
      {filtered.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 14 }}>Belum ada jamaah referral.</p>}
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, overflow: "hidden" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-cream-400)", background: "var(--color-cream-100)" }}>
              {["Nama", "Musim", "Pembayaran", "Dokumen", "Status"].map(h => (
                <th key={h} style={{ textAlign: "left", padding: "10px 14px", fontWeight: 700, color: "var(--color-warm-600)" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map(p => {
              const badge = payBadge[p.paymentStatus] ?? { label: p.paymentStatus, color: "#374151", bg: "#f3f4f6" };
              return (
                <tr key={p.id} style={{ borderBottom: "1px solid var(--color-cream-300)" }}>
                  <td style={{ padding: "12px 14px", fontWeight: 600 }}>{p.fullName}</td>
                  <td style={{ padding: "12px 14px", color: "var(--color-warm-500)" }}>{p.seasonName}</td>
                  <td style={{ padding: "12px 14px" }}>
                    <span style={{ padding: "3px 10px", borderRadius: 20, fontSize: 12, fontWeight: 700, color: badge.color, background: badge.bg }}>{badge.label}</span>
                  </td>
                  <td style={{ padding: "12px 14px" }}>{p.docsComplete ? "✅ Lengkap" : "⚠ Belum"}</td>
                  <td style={{ padding: "12px 14px", color: p.pilgrimStatus === "CANCELLED" ? "var(--color-danger-600)" : "var(--color-emerald-700)", fontWeight: 600 }}>
                    {p.pilgrimStatus === "CANCELLED" ? "Dibatalkan" : "Aktif"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

### B5 — `apps/web/components/agent/AgentReferralTab.tsx`

Shows referral code + shareable registration link. Agent copies link → calon jamaah buka link
→ isi form → agent_id otomatis terhubung.

```tsx
"use client";
import { useEffect, useState } from "react";
import { IconCopy, IconCheck } from "@tabler/icons-react";
import { getMyAccessCached } from "@/lib/access-cache";

export default function AgentReferralTab() {
  const [referralCode, setReferralCode] = useState("");
  const [operatorId, setOperatorId]     = useState("");
  const [copied, setCopied]             = useState(false);

  useEffect(() => {
    getMyAccessCached().then(access => {
      setReferralCode(access.linkedAgent?.referralCode ?? "");
      setOperatorId(access.operatorId ?? "");
    });
  }, []);

  const baseUrl   = typeof window !== "undefined" ? window.location.origin : "";
  // Registration link with ?ref= so the form auto-links jamaah to this agent.
  // season_id is left as a placeholder — agent should share per-season link
  // or the registration page shows a season picker when season_id is absent.
  const refLink   = `${baseUrl}/register/${operatorId}?ref=${referralCode}`;

  const copy = async () => {
    await navigator.clipboard.writeText(refLink);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ maxWidth: 560 }}>
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 28, marginBottom: 20 }}>
        <h3 style={{ margin: "0 0 4px", fontSize: 15, fontWeight: 700 }}>Kode Referral Anda</h3>
        <p style={{ color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 16px" }}>
          Bagikan link di bawah ke calon jamaah. Setiap pendaftaran melalui link ini otomatis tercatat sebagai referral Anda.
        </p>
        <div style={{ background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "12px 16px", marginBottom: 16, fontFamily: "monospace", fontSize: 18, fontWeight: 700, color: "var(--color-emerald-900)", letterSpacing: ".1em" }}>
          {referralCode || "—"}
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <div style={{ flex: 1, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 14px", fontSize: 12, color: "var(--color-warm-600)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {refLink}
          </div>
          <button onClick={copy} style={{ height: 44, padding: "0 16px", background: copied ? "var(--color-emerald-700)" : "var(--color-emerald-900)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, cursor: "pointer", display: "flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}>
            {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
            {copied ? "Tersalin!" : "Salin Link"}
          </button>
        </div>
      </div>

      <div style={{ background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: "16px 20px", fontSize: 13, color: "var(--color-warm-600)" }}>
        <p style={{ margin: "0 0 8px", fontWeight: 700 }}>Cara kerja referral:</p>
        <ol style={{ margin: 0, paddingLeft: 20, lineHeight: 1.8 }}>
          <li>Calon jamaah buka link referral Anda</li>
          <li>Isi formulir pendaftaran (nama, paspor, kontak)</li>
          <li>Operator menerima pendaftaran → approve → jamaah menjadi referral Anda</li>
          <li>Komisi dihitung otomatis saat jamaah melakukan pembayaran</li>
        </ol>
      </div>
    </div>
  );
}
```

### B6 — `apps/web/components/agent/AgentPayoutTab.tsx`

Shows pending payout requests + form to request new payout.

```tsx
"use client";
import { useEffect, useState } from "react";
import { createAgentClient } from "@/lib/rpc";

export default function AgentPayoutTab() {
  const [wallet, setWallet]       = useState<any>(null);
  const [requests, setRequests]   = useState<any[]>([]);
  const [amount, setAmount]       = useState("");
  const [note, setNote]           = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice]       = useState("");
  const [loading, setLoading]     = useState(true);

  const refresh = () => {
    Promise.all([
      createAgentClient().getMyWallet({}),
      // ListPayoutRequests with empty agent_id returns this agent's own requests
      // since GetMyWallet resolves from session, not a request param.
      // Use GetMyWallet transactions (PENDING_REQUEST type) for display instead.
    ]).then(([w]) => {
      setWallet(w);
      // Extract pending requests from transactions
      setRequests(w.transactions.filter((t: any) => t.type === 3 /* PENDING_REQUEST */));
    }).finally(() => setLoading(false));
  };

  useEffect(refresh, []);

  const fmt = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

  const submit = async () => {
    const amountIDR = parseInt(amount.replace(/\D/g, ""), 10);
    if (!amountIDR || amountIDR <= 0) { setNotice("Masukkan jumlah yang valid."); return; }
    if (!note.trim()) { setNotice("Isi keterangan rekening tujuan transfer."); return; }
    setSubmitting(true); setNotice("");
    try {
      await createAgentClient().requestAgentPayout({ amountIdr: BigInt(amountIDR), note: note.trim() });
      setNotice("Permintaan pencairan berhasil dikirim. Operator akan memproses dalam 1-3 hari kerja.");
      setAmount(""); setNote("");
      refresh();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "Gagal mengirim permintaan.");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>;

  const inp: React.CSSProperties = { display: "block", width: "100%", marginTop: 6, padding: "10px 12px", fontSize: 14, borderRadius: 8, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-500)", fontFamily: "'Plus Jakarta Sans',sans-serif", outline: "none", boxSizing: "border-box" };
  const lbl: React.CSSProperties = { display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };

  return (
    <div style={{ display: "grid", gap: 20, maxWidth: 600 }}>
      {/* Available balance */}
      <div style={{ background: "var(--color-emerald-900)", borderRadius: 12, padding: "20px 24px", color: "#fff" }}>
        <p style={{ margin: "0 0 4px", fontSize: 12, opacity: .7 }}>Dana Tersedia untuk Dicairkan</p>
        <p style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>{wallet ? fmt(wallet.availableIdr) : "—"}</p>
      </div>

      {/* Request form */}
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
        <h3 style={{ margin: "0 0 16px", fontSize: 15, fontWeight: 700 }}>Ajukan Pencairan Dana</h3>
        <div style={{ display: "grid", gap: 14 }}>
          <label style={lbl}>
            Jumlah (Rp)
            <input value={amount} onChange={e => setAmount(e.target.value)} placeholder="Contoh: 500000" style={inp} />
          </label>
          <label style={lbl}>
            Keterangan (nomor rekening / e-wallet)
            <textarea value={note} onChange={e => setNote(e.target.value)} rows={3} placeholder="BCA 1234567890 a.n. Nama Anda" style={{ ...inp, resize: "vertical" }} />
          </label>
          {notice && <p style={{ fontSize: 13, color: notice.includes("berhasil") ? "var(--color-emerald-700)" : "var(--color-danger-600)" }}>{notice}</p>}
          <button onClick={submit} disabled={submitting} style={{ height: 44, background: "var(--color-gold-500)", color: "var(--color-warm-900)", border: "none", borderRadius: 8, fontWeight: 700, fontSize: 14, cursor: "pointer" }}>
            {submitting ? "Mengirim..." : "Ajukan Pencairan"}
          </button>
        </div>
      </div>

      {/* Pending requests */}
      {requests.length > 0 && (
        <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
          <h3 style={{ margin: "0 0 12px", fontSize: 15, fontWeight: 700 }}>Permintaan Menunggu</h3>
          {requests.map((r: any) => (
            <div key={r.id} style={{ display: "flex", justifyContent: "space-between", padding: "10px 0", borderBottom: "1px solid var(--color-cream-300)", fontSize: 13 }}>
              <span style={{ color: "var(--color-warm-500)" }}>{r.createdAt?.toDate().toLocaleDateString("id-ID")}</span>
              <span style={{ fontWeight: 700, color: "var(--color-gold-700)" }}>{fmt(r.amountIdr)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

---

## Module C — Registration Page: `?ref=` Integration

### Goal
When a calon jamaah opens `/register/[operatorId]?ref=[referralCode]`, the referral code is
stored in state and passed to `SubmitRegistration`. Backend looks up the agent by referral_code
and sets `agent_id` on the created registration row. When operator approves → pilgrim is created
with correct `agent_id`.

### C1 — Update `/register/[operatorId]/[seasonId]/page.tsx` and `/register/[operatorId]/page.tsx`

Both registration pages need to read `?ref=` from URL and pass it to the RPC.

```tsx
// At top of registration page component:
import { useSearchParams } from "next/navigation";

const searchParams = useSearchParams();
const refCode = searchParams.get("ref") ?? "";

// Pass to SubmitRegistration RPC:
await createRegistrationClient().submitRegistration({
  operatorId,
  seasonId,
  fullName,
  email,
  phone,
  // ... other fields
  referralCode: refCode,  // new field — see C2
});
```

If `/register/[operatorId]` (without season) exists as a landing page, it should show
a season picker, then proceed to the form with `?ref=` preserved in the URL.

### C2 — Update proto: `proto/hajj/v1/registration.proto`

Add to `SubmitRegistrationRequest`:
```protobuf
string referral_code = 20;  // optional; if set, agent is looked up and linked
```

### C3 — Update `RegistrationService.SubmitRegistration` in Go

```go
// After validating operator_id + season_id, if referral_code is non-empty:
if req.ReferralCode != "" {
    agent, err := s.agentRepo.GetByReferralCode(ctx, operatorID, req.ReferralCode)
    if err == nil && agent.IsActive {
        params.AgentID = pgtype.UUID{Bytes: agent.ID, Valid: true}
    }
    // If referral_code invalid or agent inactive — silently ignore, not an error.
    // Do NOT return error here: a typo in referral code should not block registration.
}
```

Add `agentRepo *repository.AgentRepository` to RegistrationService and inject in main.go.

---

## Module D — Operator Dashboard: Payout Requests Inbox

### Goal
Operators currently have no UI to see and approve/reject agent payout requests.
`AgentService.ListPayoutRequests` + `RejectPayoutRequest` + `RecordAgentPayout` already exist
in the backend — just need the UI.

### D1 — Update `apps/web/components/agents/AgentsDashboard.tsx`

Add third tab: **"Permintaan Pencairan"** (with badge count of pending requests).

```tsx
// Add to existing tab bar:
{ id: "requests", label: `Permintaan Pencairan${pendingCount > 0 ? ` (${pendingCount})` : ""}`, icon: IconCash }
```

### D2 — `apps/web/components/agents/PayoutRequestsPanel.tsx`

```tsx
"use client";
import { useEffect, useState } from "react";
import { createAgentClient } from "@/lib/rpc";

type Request = {
  id: string; agentName: string; amountIdr: bigint; note: string;
  requestedAt: { toDate: () => Date };
};

export default function PayoutRequestsPanel({ onCountChange }: { onCountChange?: (n: number) => void }) {
  const [requests, setRequests]   = useState<Request[]>([]);
  const [loading, setLoading]     = useState(true);
  const [working, setWorking]     = useState("");
  const [rejectId, setRejectId]   = useState("");
  const [rejectNote, setRejectNote] = useState("");
  const [notice, setNotice]       = useState("");

  const refresh = () => {
    createAgentClient().listPayoutRequests({ agentId: "" }).then(r => {
      // Filter PENDING only
      const pending = r.requests.filter((x: any) => x.status === 1 /* PENDING */);
      setRequests(pending);
      onCountChange?.(pending.length);
    }).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  const fmt = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

  const approve = async (req: Request) => {
    setWorking(req.id); setNotice("");
    try {
      await createAgentClient().recordAgentPayout({
        agentId: "", // resolved from request_id server-side
        amountIdr: req.amountIdr,
        note: "Disetujui dari permintaan",
        method: 1, // TRANSFER
        requestId: req.id,
      });
      setNotice(`Pencairan ${fmt(req.amountIdr)} untuk ${req.agentName} berhasil diproses.`);
      refresh();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "Gagal memproses.");
    } finally { setWorking(""); }
  };

  const reject = async () => {
    if (!rejectNote.trim()) return;
    setWorking(rejectId); setNotice("");
    try {
      await createAgentClient().rejectPayoutRequest({ requestId: rejectId, note: rejectNote });
      setRejectId(""); setRejectNote("");
      setNotice("Permintaan ditolak.");
      refresh();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "Gagal menolak.");
    } finally { setWorking(""); }
  };

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat permintaan...</p>;

  return (
    <div>
      {notice && <p style={{ color: "var(--color-emerald-700)", fontWeight: 600, marginBottom: 12, fontSize: 13 }}>{notice}</p>}
      {requests.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 14 }}>Tidak ada permintaan pencairan yang menunggu.</p>}
      <div style={{ display: "grid", gap: 12 }}>
        {requests.map(req => (
          <div key={req.id} style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "18px 20px" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
              <div>
                <p style={{ margin: 0, fontWeight: 700, fontSize: 15 }}>{req.agentName}</p>
                <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
                  {req.requestedAt.toDate().toLocaleDateString("id-ID")}
                </p>
              </div>
              <p style={{ margin: 0, fontWeight: 700, fontSize: 18, color: "var(--color-emerald-800)" }}>{fmt(req.amountIdr)}</p>
            </div>
            <p style={{ margin: "0 0 16px", fontSize: 13, color: "var(--color-warm-600)", background: "var(--color-cream-100)", padding: "8px 12px", borderRadius: 6 }}>{req.note}</p>
            {rejectId === req.id ? (
              <div style={{ display: "grid", gap: 8 }}>
                <textarea value={rejectNote} onChange={e => setRejectNote(e.target.value)} placeholder="Alasan penolakan (wajib)" rows={2}
                  style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", fontSize: 13, resize: "vertical" }} />
                <div style={{ display: "flex", gap: 8 }}>
                  <button onClick={reject} disabled={!!working || !rejectNote.trim()} style={{ flex: 1, height: 40, background: "var(--color-danger-600, #dc2626)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, cursor: "pointer" }}>
                    {working === req.id ? "Memproses..." : "Konfirmasi Tolak"}
                  </button>
                  <button onClick={() => { setRejectId(""); setRejectNote(""); }} style={{ height: 40, padding: "0 16px", border: "1px solid var(--color-cream-500)", borderRadius: 8, background: "none", cursor: "pointer" }}>Batal</button>
                </div>
              </div>
            ) : (
              <div style={{ display: "flex", gap: 8 }}>
                <button onClick={() => approve(req)} disabled={!!working} style={{ flex: 1, height: 40, background: "var(--color-emerald-900)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, cursor: "pointer" }}>
                  {working === req.id ? "Memproses..." : "✓ Setujui & Bayar"}
                </button>
                <button onClick={() => setRejectId(req.id)} style={{ height: 40, padding: "0 16px", border: "1px solid var(--color-danger-400, #f87171)", color: "var(--color-danger-600, #dc2626)", borderRadius: 8, background: "none", fontWeight: 600, cursor: "pointer" }}>
                  Tolak
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
```

---

## Module E — Terminology Update (Display Only)

Update all visible labels in UI. Backend field names unchanged.

### E1 — Files to update:

**`apps/web/app/dashboard/(shell)/agents/` (all files):**
- Page title: "Tour Leader" (bukan "Agen")
- Table column headers: "Nama Tour Leader", "Komisi Rate", "Jamaah Referral"
- Button labels: "Tambah Tour Leader", "Edit Tour Leader"
- Empty state: "Belum ada tour leader"

**`apps/web/app/leader/` (all files):**
- Page title / header: "Portal Muttawwif"
- "Rombongan Saya" stays the same (still correct)
- Any reference to "Group Leader" in text → "Muttawwif"

**`apps/web/app/dashboard/(shell)/groups/` (all files):**
- Column header: "Muttawwif" (bukan "Ketua Rombongan" atau "Leader")
- Picker label: "Pilih Muttawwif"

**`apps/web/components/settings/TeamPanel.tsx`:**
- Role display: "owner" → "Pemilik", "admin" → "Admin", "member" → "Staf"
- No change needed for Tour Leader / Muttawwif here (this is staff management)

### E2 — Nav label in layout.tsx

```tsx
{ href: "/dashboard/agents", label: "Tour Leader", icon: IconUserCheck }
```

---

## Module F — Agent Trip Participation (When Tour Leader Travels)

### Goal
When a Tour Leader (agent) is also traveling with the group, they need the same operational
view as a Muttawwif during the trip: their jamaah's hotel status, movement schedule, SOS alerts.
Solution: allow assigning an agent to a kloter (optional), then their `/agent` portal shows
a "Trip View" tab when they have an active kloter assignment.

This reuses existing `kloter_staff` from Phase 4 Module C. A Tour Leader traveling is simply
assigned as staff to a kloter with role `COORDINATOR`.

### F1 — No new migration needed
Use `kloter_staff` table (migration 053 from Phase 4). Assign the agent's `linked_user_id`
as a staff member of the relevant kloter.

### F2 — Agent portal: Trip View tab (conditional)

In `/agent/page.tsx`, after loading access data, check if the agent is also in `kloter_staff`:

```tsx
// Add to AgentPortalPage:
const [hasTrip, setHasTrip] = useState(false);

useEffect(() => {
  createStaffScheduleClient().listMyAssignments({}).then(r => {
    setHasTrip(r.assignments.length > 0);
  });
}, []);

// Add conditional tab:
...(hasTrip ? [{ id: "trip", label: "Perjalanan Saya", icon: IconMapPin }] : []),
```

When "Perjalanan Saya" tab is shown, render a simplified version of the leader view:
- Their assigned kloter's jamaah list
- Hotel check-in status per jamaah
- Active SOS alerts from their jamaah
- Movement schedule

This tab reuses components from `/leader` portal — extract shared components to
`apps/web/components/trip/` so both leader app and agent portal can use them.

---

## Execution Order

```
Step  1  →  Update identity.proto: add AgentSummary + linked_agent to MyAccess
Step  2  →  pnpm buf:generate
Step  3  →  Update identity.go service: add linked_agent resolution (inject agentRepo)
Step  4  →  Update main.go: pass agentRepo to NewIdentityService
Step  5  →  Update registration.proto: add referral_code field
Step  6  →  pnpm buf:generate (again after registration.proto change)
Step  7  →  Update agent.sql: add ListMyPilgrims query
Step  8  →  sqlc generate (from apps/api/)
Step  9  →  Update agent.go service: add ListMyPilgrims method
Step 10  →  Update agent.proto: add AgentPilgrim + ListMyPilgrims RPC
Step 11  →  pnpm buf:generate
Step 12  →  Update registration service: inject agentRepo, resolve referral_code
Step 13  →  go build ./... — zero errors before any frontend work
Step 14  →  Update apps/web/lib/post-login.ts: add agent check (step A4 above)
Step 15  →  Create apps/web/app/agent/layout.tsx (route guard)
Step 16  →  Create apps/web/app/agent/page.tsx
Step 17  →  Create all 4 tab components in apps/web/components/agent/
Step 18  →  Create apps/web/components/agents/PayoutRequestsPanel.tsx
Step 19  →  Update AgentsDashboard.tsx: add Permintaan Pencairan tab
Step 20  →  Update registration pages to read ?ref= from URL
Step 21  →  Apply E1/E2 terminology updates (display labels only)
Step 22  →  pnpm --filter web dev
Step 23  →  Run all verification checks below
```

---

## Verification Checklist

### Identity & Routing
- [ ] Agent (org member with linked_user_id) logs in → redirected to /agent (not /dashboard)
- [ ] Muttawwif (org member with group assignment) logs in → redirected to /leader (not /agent)
- [ ] Owner/Admin logs in → redirected to /dashboard (unchanged)
- [ ] linked_agent = null for owner/admin → they still go to /dashboard

### Agent Portal
- [ ] Wallet tab: total_earned, available, pending all match DB values
- [ ] Wallet tab: transaction history sorted by date DESC
- [ ] Jamaah tab: only shows pilgrims where agent_id = this agent (NOT all pilgrims)
- [ ] Jamaah tab: season filter works correctly
- [ ] Referral tab: copies correct URL with ?ref=[referral_code]
- [ ] Payout tab: RequestPayout fails if amount > available_idr (backend validation)
- [ ] Payout tab: RequestPayout with amount = 0 returns validation error

### Registration Referral
- [ ] `/register/[operatorId]?ref=[validCode]` → registration submitted → pilgrim_registrations.agent_id set
- [ ] `/register/[operatorId]?ref=[invalidCode]` → registration succeeds (silently ignores bad code)
- [ ] `/register/[operatorId]` (no ref) → registration succeeds with agent_id = NULL

### Operator Payout Inbox
- [ ] Approve payout: agent_payouts row inserted, payout_request status = APPROVED
- [ ] Reject payout: payout_request status = REJECTED, resolution_note saved
- [ ] Pending count badge shows correct number
- [ ] After approve/reject, request disappears from inbox

### Terminology
- [ ] /dashboard/agents page title shows "Tour Leader"
- [ ] /leader portal shows "Muttawwif" in header
- [ ] /dashboard/groups shows "Muttawwif" in leader column

### General
- [ ] `go build ./...` — zero errors
- [ ] `pnpm typecheck` — zero errors
- [ ] Agent cannot access /dashboard/* (no admin/owner role)
- [ ] Agent cannot call ListAgents, CreateAgent (authenticated but not org admin — handle gracefully)
```
