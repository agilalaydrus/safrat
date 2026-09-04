"use client";

import { useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
import type { ManasikSession } from "@hajj-saas/proto-gen/hajj/v1/manasik_pb";
import type { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { manasikClient, pilgrimClient } from "@/lib/rpc";

type Props = { open: boolean; session?: ManasikSession; seasonId: string; onClose: () => void };

const STATUS_OPTIONS: [string, string][] = [["PRESENT", "Hadir"], ["ABSENT", "Tidak Hadir"], ["EXCUSED", "Izin"]];
const STATUS_COLOR: Record<string, string> = { PRESENT: "var(--color-emerald-800)", ABSENT: "var(--color-danger-600)", EXCUSED: "var(--color-gold-800)" };

export default function AttendanceDrawer({ open, session, seasonId, onClose }: Props) {
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [marks, setMarks] = useState<Record<string, string>>({});
  const [summary, setSummary] = useState({ present: 0, absent: 0, excused: 0 });
  const [saving, setSaving] = useState("");
  const [notice, setNotice] = useState("");
  const [term, setTerm] = useState("");

  const load = () => {
    if (!session) return;
    Promise.all([
      pilgrimClient.listPilgrims({ seasonId, limit: 1000, offset: 0 }),
      manasikClient.listManasikAttendance({ sessionId: session.id }),
    ]).then(([pilgrimsResponse, attendanceResponse]) => {
      setPilgrims(pilgrimsResponse.pilgrims.filter((p) => !p.isSubstituted));
      setMarks(Object.fromEntries(attendanceResponse.rows.map((r) => [r.pilgrimId, r.status])));
      setSummary({ present: attendanceResponse.presentCount, absent: attendanceResponse.absentCount, excused: attendanceResponse.excusedCount });
    }).catch(() => setNotice("Gagal memuat absensi."));
  };
  useEffect(() => { if (open) { setNotice(""); setTerm(""); load(); } }, [open, session]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!open || !session) return null;

  const mark = async (pilgrimId: string, status: string) => {
    setSaving(pilgrimId);
    setNotice("");
    try {
      await manasikClient.recordManasikAttendance({ sessionId: session.id, pilgrimId, status });
      load();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal mencatat absensi.");
    } finally {
      setSaving("");
    }
  };

  const filtered = term.trim()
    ? pilgrims.filter((p) => `${p.fullName} ${p.passportNumber}`.toLowerCase().includes(term.toLowerCase()))
    : pilgrims;

  return (
    <div style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <div>
            <p style={{ margin: "0 0 4px", fontSize: 11, color: "var(--color-gold-800)", fontWeight: 700 }}>ABSENSI</p>
            <h2 style={{ margin: 0, fontSize: 18 }}>{session.title}</h2>
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
              Hadir {summary.present} · Tidak Hadir {summary.absent} · Izin {summary.excused} — dari {pilgrims.length} jamaah
            </p>
          </div>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        {notice && <p style={{ margin: "0 24px", color: "var(--color-danger-600)", fontSize: 13 }}>{notice}</p>}
        <div style={{ padding: "12px 24px 0" }}>
          <input placeholder="Cari nama atau paspor..." value={term} onChange={(e) => setTerm(e.target.value)} style={searchInput} />
        </div>
        <div style={body}>
          {filtered.map((p) => (
            <div key={p.id} style={row}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{p.fullName}</strong>
                <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{p.passportNumber}</p>
              </div>
              <div style={{ display: "flex", gap: 4 }}>
                {STATUS_OPTIONS.map(([value, label]) => {
                  const active = marks[p.id] === value;
                  return (
                    <button key={value} type="button" disabled={saving === p.id} onClick={() => void mark(p.id, value)}
                      style={{ ...statusBtn, ...(active ? { background: STATUS_COLOR[value], color: "#fff", borderColor: STATUS_COLOR[value] } : {}) }}>
                      {label}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
          {!filtered.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Tidak ada jamaah.</p>}
        </div>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)" };
const sheet: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", background: "#fff", display: "flex", flexDirection: "column" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", padding: "20px 24px 12px", borderBottom: "1px solid var(--color-cream-300)" };
const closeBtn: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const searchInput: React.CSSProperties = { width: "100%", minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit" };
const body: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24, display: "grid", gap: 8, alignContent: "start" };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 13, flexWrap: "wrap" };
const statusBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "#fff", color: "var(--color-warm-600)", fontSize: 12, fontWeight: 600 };
