"use client";

import { useEffect, useState } from "react";
import { IconCircle, IconCircleCheckFilled } from "@tabler/icons-react";
import { ChecklistItem } from "@hajj-saas/proto-gen/hajj/v1/checklist_pb";
import { checklistClient } from "@/lib/rpc";
import { usePilgrimCode } from "@/lib/pilgrim-context";

export default function PilgrimChecklistPage() {
  const code = usePilgrimCode();
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<string>();

  const refresh = () => {
    if (!code) return;
    checklistClient.getMyChecklist({ appAccessCode: code })
      .then((response) => setItems(response.items))
      .catch(() => setError("Gagal memuat checklist. Periksa koneksi Anda."))
      .finally(() => setLoading(false));
  };
  useEffect(refresh, [code]);

  const toggle = async (item: ChecklistItem) => {
    if (!code) return;
    setPending(item.templateId);
    try {
      await checklistClient.completeMyChecklistItem({ appAccessCode: code, templateId: item.templateId, isCompleted: !item.isCompleted, notes: item.notes });
      refresh();
    } catch {
      setError("Gagal memperbarui item. Coba lagi.");
    } finally {
      setPending(undefined);
    }
  };

  const completedCount = items.filter((i) => i.isCompleted).length;
  const pct = items.length ? Math.round((completedCount / items.length) * 100) : 0;

  return <main style={page}>
    <p style={eyebrow}>PERSIAPAN KEBERANGKATAN</p>
    <h1 style={title}>Checklist Saya</h1>
    {error && <p style={{ color: "var(--color-danger-600)" }}>{error}</p>}
    {!error && <>
      <div style={progressCard}>
        <div style={barTrack}><div style={{ ...barFill, width: `${pct}%` }} /></div>
        <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>{completedCount} dari {items.length} selesai ({pct}%)</p>
      </div>
      {loading ? <p style={{ color: "var(--color-warm-400)" }}>Memuat...</p> : <div style={{ display: "grid", gap: 10, marginTop: 16 }}>
        {items.map((item) => <button key={item.templateId} disabled={pending === item.templateId} onClick={() => toggle(item)} style={itemRow}>
          {item.isCompleted ? <IconCircleCheckFilled size={26} color="var(--color-emerald-800)" /> : <IconCircle size={26} color="var(--color-cream-500)" />}
          <div style={{ textAlign: "start" }}>
            <p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>{item.title}{item.isRequired && <span style={{ color: "var(--color-danger-600)", fontSize: 11, marginLeft: 6 }}>Wajib</span>}</p>
            {item.description && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{item.description}</p>}
          </div>
        </button>)}
        {!items.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada item checklist untuk musim Anda.</p>}
      </div>}
    </>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 26, margin: "0 0 16px" };
const progressCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14 };
const barTrack: React.CSSProperties = { height: 10, borderRadius: 999, background: "var(--color-cream-300)", overflow: "hidden" };
const barFill: React.CSSProperties = { height: "100%", background: "var(--color-emerald-800)" };
const itemRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, width: "100%", minHeight: 56, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "10px 14px" };
