"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { IconArrowLeft, IconPlus, IconTrash } from "@tabler/icons-react";
import { CancellationPolicy } from "@hajj-saas/proto-gen/hajj/v1/cancellation_pb";
import { cancellationClient, seasonClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

export default function CancellationPolicyPanel({ seasonId }: { seasonId: string }) {
  const [seasonName, setSeasonName] = useState("");
  const [policies, setPolicies] = useState<CancellationPolicy[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [form, setForm] = useState({ name: "", minDays: "90", refundPct: "100", sortOrder: "0" });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => setSeasonName(response.seasons.find((s) => s.id === seasonId)?.name ?? "")).catch(() => {});
  }, [seasonId]);

  const refresh = () => {
    setLoading(true);
    cancellationClient.listCancellationPolicies({ seasonId }).then((response) => setPolicies(response.policies)).catch(() => setNotice("Gagal memuat kebijakan pembatalan.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const addPolicy = async () => {
    if (!form.name.trim()) { setNotice("Nama ketentuan wajib diisi."); return; }
    setSaving(true);
    setNotice("");
    try {
      await cancellationClient.setCancellationPolicy({
        seasonId, name: form.name.trim(), minDays: Number(form.minDays) || 0,
        refundPct: Math.min(100, Math.max(0, Number(form.refundPct) || 0)), sortOrder: Number(form.sortOrder) || 0,
      });
      setForm({ name: "", minDays: "90", refundPct: "100", sortOrder: String(policies.length) });
      refresh();
    } catch (error) {
      setNotice(`Gagal menyimpan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  const removePolicy = async (policy: CancellationPolicy) => {
    if (!window.confirm(`Hapus ketentuan "${policy.name}"?`)) return;
    try {
      await cancellationClient.deleteCancellationPolicy({ id: policy.id });
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <Link href="/dashboard/seasons" style={{ color: "var(--color-gold-800)", display: "inline-flex", alignItems: "center", gap: 6 }}><IconArrowLeft size={16} />Kembali ke daftar musim</Link>
    <header style={header}>
      <p style={eyebrow}>MUSIM {seasonName ? `· ${seasonName}` : ""}</p>
      <h1 style={title}>Kebijakan Pembatalan</h1>
      <p style={{ color: "var(--color-warm-500)", margin: 0 }}>Atur ketentuan pengembalian dana berdasarkan jarak hari sebelum keberangkatan. Sistem memakai ketentuan pertama yang cocok, jadi urutan menentukan mana yang dipakai duluan.</p>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    <RoleGate require={["owner", "admin"]} fallback={<p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Hanya pemilik atau admin yang dapat mengatur kebijakan pembatalan.</p>}>
      <section style={card}>
        <h2 style={{ margin: 0 }}>Tambah Ketentuan</h2>
        <div style={formGrid}>
          <label style={field}>Nama Ketentuan<input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} placeholder="Lebih dari 90 hari" style={input} /></label>
          <label style={field}>Min. Hari Sebelum Keberangkatan<input type="number" min={0} value={form.minDays} onChange={(e) => setForm((f) => ({ ...f, minDays: e.target.value }))} style={input} /></label>
          <label style={field}>Persentase Refund (%)<input type="number" min={0} max={100} value={form.refundPct} onChange={(e) => setForm((f) => ({ ...f, refundPct: e.target.value }))} style={input} /></label>
          <label style={field}>Urutan Prioritas<input type="number" min={0} value={form.sortOrder} onChange={(e) => setForm((f) => ({ ...f, sortOrder: e.target.value }))} style={input} /></label>
        </div>
        <button disabled={saving} onClick={addPolicy} style={emerald}><IconPlus size={18} />Tambah Ketentuan</button>
      </section>
    </RoleGate>

    <section style={{ marginTop: 20 }}>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : policies.length ? <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead><tr>{["Urutan", "Nama Ketentuan", "Min. Hari", "Refund", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
          <tbody>
            {policies.map((policy) => <tr key={policy.id} style={tr}>
              <td style={td}>{policy.sortOrder}</td>
              <td style={td}><strong>{policy.name}</strong></td>
              <td style={td}>≥ {policy.minDays} hari</td>
              <td style={td}><span style={{ fontWeight: 700, color: "var(--color-emerald-900)" }}>{policy.refundPct}%</span></td>
              <td style={td}><RoleGate require={["owner", "admin"]}><button onClick={() => removePolicy(policy)} aria-label={`Hapus ${policy.name}`} style={deleteBtn}><IconTrash size={15} /></button></RoleGate></td>
            </tr>)}
          </tbody>
        </table>
      </div> : <p style={{ color: "var(--color-warm-500)" }}>Belum ada kebijakan pembatalan untuk musim ini, jadi pembatalan akan selalu 0% refund.</p>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 900, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { marginTop: 20 };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 8px" };
const card: React.CSSProperties = { display: "grid", gap: 14, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginTop: 20 };
const formGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px", display: "inline-flex", gap: 8, alignItems: "center", justifySelf: "start" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 600, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, overflow: "hidden" };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "12px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "12px 16px", fontSize: 14 };
const deleteBtn: React.CSSProperties = { minHeight: 32, minWidth: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
