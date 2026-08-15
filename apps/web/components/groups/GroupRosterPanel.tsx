"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { IconGenderFemale, IconGenderMale, IconWheelchair, IconX } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { groupClient } from "@/lib/rpc";

type P = { open: boolean; groupId: string; groupName: string; onClose: () => void };

export default function GroupRosterPanel({ open, groupId, groupName, onClose }: P) {
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    if (!open || !groupId) return;
    setLoading(true); setNotice("");
    groupClient.getGroupRoster({ groupId })
      .then((r) => setPilgrims(r.pilgrims))
      .catch(() => setNotice("Gagal memuat daftar jamaah rombongan ini."))
      .finally(() => setLoading(false));
  }, [open, groupId]);

  if (!open) return null;

  return (
    <div role="dialog" aria-modal="true" aria-label={`Anggota rombongan ${groupName}`} style={overlay}>
      <aside style={sheet}>
        <header style={header}>
          <div><p style={eyebrow}>ANGGOTA ROMBONGAN</p><h2 style={{ margin: 0 }}>{groupName}</h2></div>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </header>
        <div className="gold-divider" />
        {notice && <p style={{ color: "var(--color-danger-600)", padding: "0 4px" }}>{notice}</p>}
        <div style={list}>
          {loading && <p style={{ color: "var(--color-warm-400)", textAlign: "center", marginTop: 40 }}>Memuat...</p>}
          {!loading && !pilgrims.length && !notice && <p style={{ color: "var(--color-warm-400)", textAlign: "center", marginTop: 40 }}>Belum ada jamaah di rombongan ini.</p>}
          {pilgrims.map((pilgrim) => (
            <Link key={pilgrim.id} href={`/dashboard/pilgrims/${pilgrim.id}`} style={card}>
              <div>
                <strong>{pilgrim.fullName}</strong>
                <p style={meta}>{pilgrim.gender === Gender.FEMALE ? <IconGenderFemale size={15} /> : <IconGenderMale size={15} />}{pilgrim.passportNumber}</p>
              </div>
              {pilgrim.requiresWheelchair && <IconWheelchair size={20} color="var(--color-gold-800)" />}
            </Link>
          ))}
        </div>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.48)", backdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)", padding: 24, boxShadow: "-6px 0 32px rgba(26,20,16,.12)" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", flexShrink: 0 };
const list: React.CSSProperties = { flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: 10, padding: "12px 4px" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center", color: "inherit" };
const meta: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 12, fontFamily: "ui-monospace, monospace" };
