"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconLock, IconTrash } from "@tabler/icons-react";
import type { PlatformPlanLimit, PlatformPlanOverride, AffectedTenant } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

// The three the entitlement trigger actually reads. Listing them keeps the
// editor from inventing a flag no code consults.
const FLAGS = [
  { key: "branches", label: "Cabang" },
  { key: "installments", label: "Cicilan" },
  { key: "crm", label: "CRM Leads" },
];

/**
 * NULL means unlimited and zero is a real limit, so the two must never render
 * alike — an empty cell would read as both.
 */
function quotaLabel(quota?: { unlimited: boolean; value: number }): string {
  if (!quota || quota.unlimited) return "Tanpa batas";
  return String(quota.value);
}

type LimitDraft = { unlimited: boolean; value: string };

function draftOf(quota?: { unlimited: boolean; value: number }): LimitDraft {
  return quota && !quota.unlimited ? { unlimited: false, value: String(quota.value) } : { unlimited: true, value: "" };
}

function toQuota(draft: LimitDraft) {
  return draft.unlimited ? { unlimited: true, value: 0 } : { unlimited: false, value: Number(draft.value || 0) };
}

export default function PlanQuotaTab() {
  const [limits, setLimits] = useState<PlatformPlanLimit[]>([]);
  const [overrides, setOverrides] = useState<PlatformPlanOverride[]>([]);
  const [includeExpired, setIncludeExpired] = useState(false);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [editing, setEditing] = useState<string>("");
  const [pilgrims, setPilgrims] = useState<LimitDraft>({ unlimited: true, value: "" });
  const [branches, setBranches] = useState<LimitDraft>({ unlimited: true, value: "" });
  const [flags, setFlags] = useState<Record<string, boolean>>({});
  const [reason, setReason] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [preview, setPreview] = useState<AffectedTenant[]>();
  const [grandfather, setGrandfather] = useState(true);
  const [saving, setSaving] = useState(false);
  const [revoking, setRevoking] = useState("");
  const [revokeReason, setRevokeReason] = useState("");

  const refresh = useCallback(() => {
    setLoading(true);
    Promise.all([
      platformClient.listPlanLimits({}),
      platformClient.listPlanOverrides({ includeExpired }),
    ])
      .then(([limitResponse, overrideResponse]) => {
        setLimits(limitResponse.limits);
        setOverrides(overrideResponse.overrides);
      })
      .catch(() => setNotice("Gagal memuat batas paket."))
      .finally(() => setLoading(false));
  }, [includeExpired]);

  useEffect(refresh, [refresh]);

  function startEditing(limit: PlatformPlanLimit) {
    setEditing(limit.plan);
    setPilgrims(draftOf(limit.maxPilgrims));
    setBranches(draftOf(limit.maxBranches));
    setFlags({ ...limit.featureFlags });
    setReason("");
    setConfirmation("");
    setPreview(undefined);
    setGrandfather(true);
    setNotice("");
  }

  function cancelEditing() {
    setEditing("");
    setPreview(undefined);
  }

  // Nothing is written here. Seeing who breaks is the point of the step.
  async function runPreview() {
    setNotice("");
    try {
      const response = await platformClient.previewPlanLimitChange({
        plan: editing,
        maxPilgrims: toQuota(pilgrims),
        maxBranches: toQuota(branches),
        featureFlags: flags,
      });
      setPreview(response.affectedTenants);
    } catch {
      setNotice("Gagal menghitung dampak. Perubahan belum disimpan.");
    }
  }

  async function save() {
    setSaving(true);
    setNotice("");
    try {
      const response = await platformClient.setPlanLimit({
        plan: editing,
        maxPilgrims: toQuota(pilgrims),
        maxBranches: toQuota(branches),
        featureFlags: flags,
        reason: reason.trim(),
        confirmation: confirmation.trim(),
        idempotencyKey: crypto.randomUUID(),
        grandfatherAffected: grandfather,
      });
      const kept = response.grandfatheredTenants;
      setNotice(
        kept > 0
          ? `Batas ${editing} disimpan. ${kept} travel dikunci di angka lamanya.`
          : `Batas ${editing} disimpan.`,
      );
      cancelEditing();
      refresh();
    } catch {
      setNotice("Gagal menyimpan. Periksa alasan dan ketikan konfirmasi.");
    } finally {
      setSaving(false);
    }
  }

  async function removeOverride(operatorId: string, name: string) {
    try {
      await platformClient.deletePlanOverride({
        operatorId,
        reason: revokeReason.trim(),
        idempotencyKey: crypto.randomUUID(),
      });
      setRevoking("");
      setRevokeReason("");
      setNotice(`Kelonggaran ${name} dicabut. Travel ini kembali ke batas paketnya.`);
      refresh();
    } catch {
      setNotice("Gagal mencabut kelonggaran.");
    }
  }

  const activeOverrides = useMemo(
    () => overrides.filter((o) => !o.expiresAt || o.expiresAt.toDate() > new Date()).length,
    [overrides],
  );
  const confirmed = confirmation.trim().toUpperCase() === editing;
  const canSave = Boolean(preview) && confirmed && reason.trim().length > 0 && !saving;

  if (loading) return <p style={muted}>Memuat batas paket…</p>;

  return (
    <section style={{ display: "grid", gap: 20 }}>
      <div>
        <h2 style={heading}>Paket &amp; Kuota</h2>
        <p style={muted}>
          {limits.length} paket · {activeOverrides} kelonggaran aktif · perubahan batas berlaku ke seluruh travel pada paket itu
        </p>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      <table style={table}>
        <caption style={caption}>Batas per paket — lintas seluruh travel</caption>
        <thead>
          <tr>{["Paket", "Jamaah", "Cabang", "Fitur", "Diubah", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
        </thead>
        <tbody>
          {limits.map((limit) => (
            <tr key={limit.plan} style={tr}>
              <td style={{ ...td, fontWeight: 700 }}>{limit.plan}</td>
              <td style={td}>{quotaLabel(limit.maxPilgrims)}</td>
              <td style={td}>{quotaLabel(limit.maxBranches)}</td>
              <td style={td}>
                {FLAGS.filter((f) => limit.featureFlags[f.key]).map((f) => f.label).join(" · ") || "—"}
              </td>
              <td style={{ ...td, color: "var(--color-warm-400)" }}>
                {limit.updatedAt ? limit.updatedAt.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) : "—"}
              </td>
              <td style={td}>
                <button onClick={() => startEditing(limit)} style={ghost}>Ubah</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {editing && (
        <div style={card}>
          <h3 style={{ margin: 0, fontSize: 15 }}>Ubah batas {editing}</h3>
          <div style={grid}>
            <QuotaField label="Batas jamaah" draft={pilgrims} onChange={setPilgrims} />
            <QuotaField label="Batas cabang" draft={branches} onChange={setBranches} />
          </div>
          <fieldset style={fieldset}>
            <legend style={legend}>Fitur yang dibuka paket ini</legend>
            {FLAGS.map((flag) => (
              <label key={flag.key} style={checkRow}>
                <input
                  type="checkbox"
                  checked={Boolean(flags[flag.key])}
                  onChange={(e) => setFlags({ ...flags, [flag.key]: e.target.checked })}
                />
                {flag.label}
              </label>
            ))}
          </fieldset>

          <button onClick={runPreview} style={ghost}>Hitung dampak</button>

          {preview && preview.length === 0 && (
            <p style={okBox}>Tidak ada travel yang melampaui batas baru ini.</p>
          )}

          {preview && preview.length > 0 && (
            <div style={warnBox}>
              <p style={{ margin: 0, display: "flex", alignItems: "center", gap: 6, fontWeight: 700 }}>
                <IconAlertTriangle size={16} />
                {preview.length} travel akan seketika melampaui batas baru
              </p>
              <ul style={{ margin: "8px 0 0", paddingInlineStart: 20, display: "grid", gap: 4 }}>
                {preview.map((tenant) => (
                  <li key={tenant.operatorId}>
                    <strong>{tenant.name}</strong> — {tenant.pilgrimCount} jamaah, {tenant.activeBranchCount} cabang
                    {tenant.reasons.length > 0 && ` · ${tenant.reasons.join(", ")}`}
                  </li>
                ))}
              </ul>
              <p style={{ margin: "10px 0 0" }}>
                Mereka tidak kehilangan data, tetapi tidak bisa menambah jamaah atau cabang baru sampai
                turun di bawah batas — kecuali dikunci di angka lamanya.
              </p>
              <label style={{ ...checkRow, marginTop: 8 }}>
                <input type="checkbox" checked={grandfather} onChange={(e) => setGrandfather(e.target.checked)} />
                <IconLock size={14} /> Kunci travel ini di angka lamanya
              </label>
            </div>
          )}

          <label style={label}>
            Alasan perubahan
            <input value={reason} onChange={(e) => setReason(e.target.value)} style={input} placeholder="Mis. penyesuaian harga Oktober" />
          </label>
          <label style={label}>
            Ketik <strong>{editing}</strong> untuk mengonfirmasi
            <input value={confirmation} onChange={(e) => setConfirmation(e.target.value)} style={input} />
          </label>

          <div style={{ display: "flex", gap: 10 }}>
            <button onClick={save} disabled={!canSave} style={{ ...primary, opacity: canSave ? 1 : 0.5 }}>
              {saving ? "Menyimpan…" : "Simpan batas"}
            </button>
            <button onClick={cancelEditing} style={ghost}>Batal</button>
          </div>
          {!preview && <small style={hint}>Hitung dampak dulu — menyimpan tanpa melihat siapa yang terdampak adalah cara memblokir belasan travel sekaligus.</small>}
        </div>
      )}

      <div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: 12, flexWrap: "wrap" }}>
          <h3 style={{ margin: 0, fontSize: 15 }}>Kelonggaran per travel</h3>
          <label style={{ ...checkRow, fontSize: 13 }}>
            <input type="checkbox" checked={includeExpired} onChange={(e) => setIncludeExpired(e.target.checked)} />
            Tampilkan yang sudah kedaluwarsa
          </label>
        </div>
        <p style={muted}>
          Kelonggaran satu travel lebih aman daripada mengubah batas paket untuk semua.
        </p>

        {overrides.length === 0 ? (
          <div style={emptyBox}>
            <p style={{ margin: 0, fontWeight: 700 }}>Belum ada kelonggaran</p>
            <p style={{ ...muted, marginTop: 6 }}>
              Setiap travel memakai batas paketnya. Kelonggaran dibuat dari halaman travel yang bersangkutan.
            </p>
          </div>
        ) : (
          <table style={table}>
            <caption style={caption}>Lintas seluruh travel</caption>
            <thead>
              <tr>{["Travel", "Paket", "Jamaah", "Cabang", "Alasan", "Berlaku sampai", "Diubah oleh", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {overrides.map((override) => {
                const expired = Boolean(override.expiresAt && override.expiresAt.toDate() <= new Date());
                return (
                  <tr key={override.operatorId} style={tr}>
                    <td style={{ ...td, fontWeight: 700, opacity: expired ? 0.55 : 1 }}>{override.operatorName}</td>
                    <td style={td}>{override.plan}</td>
                    <td style={td}>{override.maxPilgrims ?? "—"}</td>
                    <td style={td}>{override.maxBranches ?? "—"}</td>
                    <td style={td}>{override.note || "—"}</td>
                    <td style={td}>
                      {override.expiresAt
                        ? <span style={expired ? expiredPill : undefined}>
                            {override.expiresAt.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
                            {expired ? " · kedaluwarsa" : ""}
                          </span>
                        : "Tanpa akhir"}
                    </td>
                    <td style={{ ...td, color: "var(--color-warm-400)" }}>{override.updatedBy || "—"}</td>
                    <td style={td}>
                      {revoking === override.operatorId ? (
                        <div style={{ display: "grid", gap: 6, minWidth: 220 }}>
                          <input
                            value={revokeReason}
                            onChange={(e) => setRevokeReason(e.target.value)}
                            style={{ ...input, minHeight: 38 }}
                            placeholder="Alasan mencabut"
                            aria-label={`Alasan mencabut kelonggaran ${override.operatorName}`}
                            autoFocus
                          />
                          <div style={{ display: "flex", gap: 6 }}>
                            <button
                              onClick={() => removeOverride(override.operatorId, override.operatorName)}
                              disabled={!revokeReason.trim()}
                              style={{ ...danger, opacity: revokeReason.trim() ? 1 : 0.5 }}
                            >
                              Cabut
                            </button>
                            <button onClick={() => { setRevoking(""); setRevokeReason(""); }} style={ghost}>Batal</button>
                          </div>
                        </div>
                      ) : (
                        <button
                          onClick={() => { setRevoking(override.operatorId); setRevokeReason(""); }}
                          style={ghost}
                          aria-label={`Cabut kelonggaran ${override.operatorName}`}
                        >
                          <IconTrash size={15} />Cabut
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

function QuotaField({ label: text, draft, onChange }: { label: string; draft: LimitDraft; onChange: (next: LimitDraft) => void }) {
  return (
    <div style={label}>
      <label style={{ display: "grid", gap: 6 }}>
        {text}
        <input
          type="number"
          min={0}
          value={draft.value}
          disabled={draft.unlimited}
          onChange={(e) => onChange({ unlimited: false, value: e.target.value })}
          style={{ ...input, opacity: draft.unlimited ? 0.5 : 1 }}
        />
      </label>
      <label style={checkRow}>
        <input
          type="checkbox"
          checked={draft.unlimited}
          onChange={(e) => onChange({ unlimited: e.target.checked, value: "" })}
        />
        Tanpa batas
      </label>
    </div>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const hint: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 12 };
const card: React.CSSProperties = { display: "grid", gap: 12, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7, justifySelf: "start" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, justifySelf: "start" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const caption: React.CSSProperties = { captionSide: "top", textAlign: "left", padding: "0 0 8px", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: "0.06em", textTransform: "uppercase" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const fieldset: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 14, border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 14px" };
const legend: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)", padding: "0 4px" };
const checkRow: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const noticeBox: React.CSSProperties = { margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13 };
const warnBox: React.CSSProperties = { padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13 };
const okBox: React.CSSProperties = { margin: 0, padding: "10px 14px", background: "var(--color-success-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 8, color: "var(--color-emerald-800)", fontSize: 13 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff", marginTop: 10 };
const danger: React.CSSProperties = { minHeight: 38, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 13 };
const expiredPill: React.CSSProperties = { color: "var(--color-warm-400)" };
