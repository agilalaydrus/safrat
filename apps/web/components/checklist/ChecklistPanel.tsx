"use client";

import { useCallback, useEffect, useState } from "react";
import { IconTrash } from "@tabler/icons-react";
import { ChecklistStat, ChecklistTemplate } from "@hajj-saas/proto-gen/hajj/v1/checklist_pb";
import { checklistClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const QUICK_ADD = [
  { title: "Paspor", category: "DOKUMEN" },
  { title: "Foto 4x6", category: "DOKUMEN" },
  { title: "Vaksin Meningitis", category: "KESEHATAN" },
  { title: "Pelunasan Biaya", category: "PEMBAYARAN" },
];

export default function ChecklistPanel({ seasonId }: { seasonId: string }) {
  const [tab, setTab] = useState<"templates" | "progress">("templates");
  const [templates, setTemplates] = useState<ChecklistTemplate[]>([]);
  const [stats, setStats] = useState<ChecklistStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  const [itemTitle, setItemTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("DOKUMEN");
  const [isRequired, setIsRequired] = useState(true);
  const [adding, setAdding] = useState(false);

  const refreshTemplates = useCallback(() => {
    setLoading(true);
    checklistClient.listChecklistTemplates({ seasonId }).then((response) => setTemplates(response.templates)).catch(() => setNotice("Gagal memuat template.")).finally(() => setLoading(false));
  }, [seasonId]);
  const refreshStats = useCallback(() => {
    setLoading(true);
    checklistClient.getChecklistStats({ seasonId }).then((response) => setStats(response.stats)).catch(() => setNotice("Gagal memuat progres.")).finally(() => setLoading(false));
  }, [seasonId]);

  useEffect(() => { if (tab === "templates") refreshTemplates(); else refreshStats(); }, [refreshStats, refreshTemplates, tab]);

  const addTemplate = async (t: { title: string; category: string; description?: string; isRequired?: boolean }) => {
    setAdding(true);
    try {
      await checklistClient.createChecklistTemplate({
        seasonId, title: t.title, description: t.description ?? "", category: t.category, isRequired: t.isRequired ?? true, sortOrder: templates.length,
      });
      refreshTemplates();
    } catch (error) {
      setNotice(`Gagal menambah: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setAdding(false);
    }
  };

  const addCustom = async () => {
    if (!itemTitle.trim()) return;
    await addTemplate({ title: itemTitle, description, category, isRequired });
    setItemTitle("");
    setDescription("");
  };

  const remove = async (id: string) => {
    try {
      await checklistClient.deleteChecklistTemplate({ id });
      setTemplates((current) => current.filter((t) => t.id !== id));
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <p style={eyebrow}>PERSIAPAN KEBERANGKATAN</p>
    <h1 style={title}>Checklist Persiapan</h1>
    <div className="gold-divider" />
    <div style={tabRow}>
      <button onClick={() => setTab("templates")} style={tab === "templates" ? tabActive : tabInactive}>Template</button>
      <button onClick={() => setTab("progress")} style={tab === "progress" ? tabActive : tabInactive}>Progres</button>
    </div>
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    {tab === "templates" && <div style={{ marginTop: 16 }}>
      <RoleGate require={["owner", "admin"]}>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 16 }}>
          {QUICK_ADD.map((q) => <button key={q.title} disabled={adding} onClick={() => addTemplate(q)} style={ghost}>+ {q.title}</button>)}
        </div>
        <section style={{ ...card, marginBottom: 20 }}>
          <h3 style={{ margin: 0 }}>Item Kustom</h3>
          <label style={field}>Judul
            <input value={itemTitle} onChange={(e) => setItemTitle(e.target.value)} style={input} />
          </label>
          <label style={field}>Deskripsi
            <input value={description} onChange={(e) => setDescription(e.target.value)} style={input} />
          </label>
          <label style={field}>Kategori
            <select value={category} onChange={(e) => setCategory(e.target.value)} style={input}>
              <option value="DOKUMEN">Dokumen</option>
              <option value="KESEHATAN">Kesehatan</option>
              <option value="PEMBAYARAN">Pembayaran</option>
              <option value="PERLENGKAPAN">Perlengkapan</option>
            </select>
          </label>
          <label style={checkboxRow}><input type="checkbox" checked={isRequired} onChange={(e) => setIsRequired(e.target.checked)} /> Wajib</label>
          <button disabled={adding} onClick={addCustom} style={emerald}>{adding ? "Menambah..." : "Tambah Item"}</button>
        </section>
      </RoleGate>

      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={{ display: "grid", gap: 8 }}>
        {templates.map((t) => <div key={t.id} style={row}>
          <div>
            <strong>{t.title}</strong>{t.isRequired && <span style={reqBadge}>Wajib</span>}
            <span style={{ display: "block", fontSize: 12, color: "var(--color-warm-400)" }}>{t.category}{t.description ? ` · ${t.description}` : ""}</span>
          </div>
          <RoleGate require={["owner", "admin"]}><button onClick={() => remove(t.id)} aria-label={`Hapus ${t.title}`} style={deleteBtn}><IconTrash size={14} /></button></RoleGate>
        </div>)}
        {!templates.length && <p style={{ color: "var(--color-warm-500)" }}>Belum ada item checklist untuk musim ini.</p>}
      </div>}
    </div>}

    {tab === "progress" && <div style={{ marginTop: 16 }}>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={{ display: "grid", gap: 12 }}>
        {stats.map((s) => {
          const pct = s.totalPilgrims > 0 ? Math.round((s.completedCount / s.totalPilgrims) * 100) : 0;
          return <div key={s.templateId} style={card}>
            <div style={{ display: "flex", justifyContent: "space-between" }}>
              <strong>{s.title}{s.isRequired && <span style={reqBadge}>Wajib</span>}</strong>
              <span style={{ color: "var(--color-warm-500)", fontSize: 13 }}>{s.completedCount}/{s.totalPilgrims} ({pct}%)</span>
            </div>
            <div style={barTrack}><div style={{ ...barFill, width: `${pct}%` }} /></div>
          </div>;
        })}
        {!stats.length && <p style={{ color: "var(--color-warm-500)" }}>Belum ada data progres.</p>}
      </div>}
    </div>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 900, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,40px)", fontWeight: 500, margin: 0 };
const tabRow: React.CSSProperties = { display: "flex", gap: 8 };
const tabActive: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-gold-100)", borderRadius: 12, background: "var(--color-gold-50)", color: "var(--color-gold-600)", boxShadow: "0 0 0 4px color-mix(in srgb, var(--color-gold-500) 12%, transparent)", fontWeight: 700, padding: "0 16px" };
const tabInactive: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-500)", padding: "0 16px" };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 18, display: "grid", gap: 10 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-gold-800)", borderRadius: 8, background: "transparent", color: "var(--color-gold-800)", fontWeight: 600, padding: "0 14px" };
const checkboxRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, color: "var(--color-warm-700)", fontSize: 14 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 14px", background: "white" };
const reqBadge: React.CSSProperties = { marginLeft: 8, fontSize: 11, color: "var(--color-danger-600)", fontWeight: 700 };
const deleteBtn: React.CSSProperties = { minHeight: 32, minWidth: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center", flexShrink: 0 };
const barTrack: React.CSSProperties = { height: 10, borderRadius: 999, background: "var(--color-cream-300)", overflow: "hidden" };
const barFill: React.CSSProperties = { height: "100%", background: "var(--color-emerald-800)" };
