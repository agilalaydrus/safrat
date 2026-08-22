"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconPlus } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { HealthReport } from "@hajj-saas/proto-gen/hajj/v1/health_report_pb";
import { groupLeaderClient, healthReportClient } from "@/lib/rpc";
import { useLeaderGroup } from "@/lib/leader-context";

const SEVERITY_LABEL: Record<string, string> = { RINGAN: "Ringan", SEDANG: "Sedang", BERAT: "Berat" };
const SEVERITY_COLOR: Record<string, string> = { RINGAN: "var(--color-warm-500)", SEDANG: "var(--color-gold-800)", BERAT: "var(--color-danger-600)" };

export default function LeaderHealthPage() {
  const { selectedGroupId: groupId, loaded } = useLeaderGroup();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [reports, setReports] = useState<HealthReport[]>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState({ pilgrimId: "", severity: "RINGAN", symptoms: "", actionTaken: "" });
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");

  const refresh = () => {
    if (!groupId) return;
    Promise.all([
      groupLeaderClient.getGroupRoster({ groupId }).then((r) => setPilgrims(r.pilgrims)),
      healthReportClient.listHealthReports({}).then((r) => setReports(r.reports.filter((x) => x.groupId === groupId))),
    ]).catch(() => setNotice("Gagal memuat data kesehatan."));
  };
  useEffect(refresh, [groupId]);

  async function submit() {
    if (!form.pilgrimId || !form.symptoms.trim()) { setNotice("Pilih jamaah dan isi gejala."); return; }
    setSaving(true);
    setNotice("");
    try {
      await healthReportClient.createHealthReport({ pilgrimId: form.pilgrimId, severity: form.severity, symptoms: form.symptoms, actionTaken: form.actionTaken });
      setForm({ pilgrimId: "", severity: "RINGAN", symptoms: "", actionTaken: "" });
      setFormOpen(false);
      refresh();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal membuat laporan kesehatan.");
    } finally {
      setSaving(false);
    }
  }

  if (!loaded) return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;

  return (
    <main style={page}>
      <div style={header}>
        <p style={eyebrow}>KESEHATAN GRUP</p>
        <button onClick={() => setFormOpen((v) => !v)} style={addBtn}><IconPlus size={16} />Laporkan</button>
      </div>
      {notice && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{notice}</p>}

      {formOpen && (
        <section style={form_}>
          <label style={fieldLabel}>Jamaah</label>
          <select value={form.pilgrimId} onChange={(e) => setForm((f) => ({ ...f, pilgrimId: e.target.value }))} style={select}>
            <option value="">— pilih jamaah —</option>
            {pilgrims.map((p) => <option key={p.id} value={p.id}>{p.fullName}</option>)}
          </select>
          <label style={fieldLabel}>Tingkat Keparahan</label>
          <select value={form.severity} onChange={(e) => setForm((f) => ({ ...f, severity: e.target.value }))} style={select}>
            <option value="RINGAN">Ringan</option>
            <option value="SEDANG">Sedang</option>
            <option value="BERAT">Berat — notifikasi langsung ke operator</option>
          </select>
          <label style={fieldLabel}>Gejala</label>
          <textarea value={form.symptoms} onChange={(e) => setForm((f) => ({ ...f, symptoms: e.target.value }))} style={textarea} rows={2} />
          <label style={fieldLabel}>Tindakan (opsional)</label>
          <input value={form.actionTaken} onChange={(e) => setForm((f) => ({ ...f, actionTaken: e.target.value }))} style={input} />
          <button disabled={saving} onClick={() => void submit()} style={primary}>{saving ? "Menyimpan..." : "Simpan Laporan"}</button>
        </section>
      )}

      <div style={list}>
        {reports.map((r) => (
          <article key={r.id} style={card}>
            <div style={row}>
              <strong>{r.pilgrimName}</strong>
              <span style={{ ...sevBadge, color: SEVERITY_COLOR[r.severity], borderColor: SEVERITY_COLOR[r.severity] }}>
                {r.severity === "BERAT" && <IconAlertTriangle size={12} />}{SEVERITY_LABEL[r.severity] ?? r.severity}
              </span>
            </div>
            <p style={{ margin: "4px 0 0", fontSize: 13, color: "var(--color-warm-600)" }}>{r.symptoms}</p>
            {r.actionTaken && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>Tindakan: {r.actionTaken}</p>}
            <p style={{ margin: "4px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>{r.resolved ? "Sudah ditangani" : "Belum ditangani"} · {r.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
          </article>
        ))}
        {!reports.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada laporan kesehatan untuk grup ini.</p>}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: 0 };
const addBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 12px", background: "var(--color-emerald-900)", color: "#fff", fontSize: 12, fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 4 };
const form_: React.CSSProperties = { display: "grid", gap: 8, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, margin: "12px 0" };
const fieldLabel: React.CSSProperties = { fontSize: 12, fontWeight: 600, color: "var(--color-warm-700)" };
const select: React.CSSProperties = { minHeight: 44, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", font: "inherit", background: "#fff" };
const input: React.CSSProperties = { minHeight: 44, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", font: "inherit" };
const textarea: React.CSSProperties = { border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "10px 12px", font: "inherit", resize: "vertical" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, marginTop: 4 };
const list: React.CSSProperties = { display: "grid", gap: 10, marginTop: 12 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const sevBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, fontWeight: 700, border: "1px solid", borderRadius: 99, padding: "2px 8px" };
