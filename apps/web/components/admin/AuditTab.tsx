"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { IconAlertTriangle, IconLock } from "@tabler/icons-react";
import type { AuditTrailEntry } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

// Named sets rather than a search box. During an incident nobody scrolls, and
// nobody recalls the exact spelling of an action under pressure.
const CATEGORIES: [string, string][] = [
  ["ALL", "Semua"],
  ["PRIVILEGED", "Tindakan istimewa"],
  ["IMPERSONATION", "Sesi lihat-saja"],
  ["PERSONAL_DATA", "Pembacaan data pribadi"],
];
const PERIODS: [number, string][] = [[7, "7 hari"], [30, "30 hari"], [90, "90 hari"], [0, "Semua"]];

const ACTION_LABEL: Record<string, string> = {
  tenant_suspended: "Travel ditangguhkan",
  tenant_reinstated: "Travel dibuka kembali",
  plan_limit_changed: "Batas paket diubah",
  trial_days_changed: "Lama trial diubah",
  grace_period_changed: "Masa tenggang diubah",
  impersonation_started: "Sesi lihat-saja dimulai",
  impersonation_ended: "Sesi lihat-saja ditutup",
  kyc_record_read: "Nomor identitas dibuka",
  kyc_status_set: "Status identitas diubah",
  sessions_revoked: "Sesi akun diakhiri paksa",
};

const timeOf = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "";

export default function AuditTab() {
  const [entries, setEntries] = useState<AuditTrailEntry[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [category, setCategory] = useState("ALL");
  const [days, setDays] = useState(30);
  const [actor, setActor] = useState("");
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setFailure("");
    platformClient
      .listAuditTrail({ category, days, actor: actor.trim(), limit: 200 })
      .then((response) => { setEntries(response.entries); setTruncated(response.truncated); })
      .catch(() => setFailure("Gagal memuat jejak audit."))
      .finally(() => setLoading(false));
  }, [category, days, actor]);

  useEffect(() => {
    const timer = window.setTimeout(load, actor ? 350 : 0);
    return () => window.clearTimeout(timer);
  }, [load, actor]);

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div>
        <h2 style={heading}>Audit</h2>
        <p style={muted}>
          Jejak lintas seluruh travel. <strong>Hanya bisa dibaca</strong> — migrasi 125 sudah mencabut UPDATE dan
          DELETE dari peran aplikasi, jadi tombol hapus di sini akan menjadi tombol yang pasti gagal.
        </p>
      </div>

      <div style={filterBar}>
        <div style={pillGroup} role="group" aria-label="Kategori">
          {CATEGORIES.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setCategory(value)} aria-pressed={category === value}
              style={category === value ? pillActive : pillInactive}>{label}</button>
          ))}
        </div>
        <div style={pillGroup} role="group" aria-label="Rentang waktu">
          {PERIODS.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setDays(value)} aria-pressed={days === value}
              style={days === value ? pillActive : pillInactive}>{label}</button>
          ))}
        </div>
        <input
          value={actor}
          onChange={(event) => setActor(event.target.value)}
          placeholder="Cari pelaku (email atau id)"
          aria-label="Cari pelaku"
          style={search}
        />
      </div>

      {failure && <p style={errorBox}><IconAlertTriangle size={16} />{failure}</p>}
      {loading && <p style={muted}>Memuat…</p>}

      {!loading && entries.length === 0 && (
        <div style={emptyBox}>
          <p style={{ margin: 0, fontWeight: 700 }}>Tidak ada kejadian yang cocok</p>
          <p style={{ ...muted, marginTop: 6 }}>
            Saringannya berlaku — ini benar-benar kosong, bukan gagal memuat.
          </p>
        </div>
      )}

      {entries.length > 0 && (
        <>
          {truncated && (
            <p style={warnLine}>
              <IconAlertTriangle size={15} />
              Daftar terpotong di 200 baris. Persempit rentang waktu atau saringannya — ekor yang kosong bukan berarti
              tidak ada kejadian lain.
            </p>
          )}
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead>
                <tr>{["Waktu", "Pelaku", "Tindakan", "Travel", "Objek", "Alasan"].map((head) => (
                  <th key={head} style={th}>{head}</th>
                ))}</tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id} style={tr}>
                    <td style={{ ...td, whiteSpace: "nowrap" }}>{timeOf(entry.at)}</td>
                    <td style={td}>{entry.actor}</td>
                    <td style={{ ...td, fontWeight: 600 }}>{ACTION_LABEL[entry.action] ?? entry.action}</td>
                    <td style={td}>
                      {entry.operatorId
                        ? <Link href={`/admin/tenant/${entry.operatorId}`} style={link}>{entry.operatorName || entry.operatorId}</Link>
                        : <span style={{ color: "var(--color-warm-400)" }}>platform</span>}
                    </td>
                    <td style={{ ...td, color: "var(--color-warm-400)" }}>{entry.entityType}</td>
                    <td style={{ ...td, maxWidth: 320 }}>{entry.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      <div style={noteBox}>
        <p style={{ ...muted, fontSize: 12, margin: 0, lineHeight: 1.7 }}>
          <IconLock size={13} style={{ verticalAlign: "-2px", marginRight: 6 }} />
          <strong>Pembacaan data pribadi lewat sesi lihat-saja tidak ada di tabel ini.</strong> Ia dicatat terpisah
          sebagai hitungan harian — satu baris per orang, per layar, per hari — karena satu baris per permintaan akan
          jadi puluhan ribu baris yang mengubur segalanya. Ada di halaman detail tenant masing-masing.
        </p>
      </div>
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: "4px 0 0" };
const heading: React.CSSProperties = { margin: 0, fontSize: 18 };
const filterBar: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap", alignItems: "center" };
const pillGroup: React.CSSProperties = { display: "inline-flex", gap: 4, padding: 4, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const pillBase: React.CSSProperties = { minHeight: 36, padding: "0 12px", border: 0, borderRadius: 7, background: "transparent", font: "inherit", fontSize: 12, fontWeight: 700, color: "var(--color-warm-500)", cursor: "pointer" };
const pillActive: React.CSSProperties = { ...pillBase, background: "#fff", color: "var(--color-emerald-900)" };
const pillInactive: React.CSSProperties = pillBase;
const search: React.CSSProperties = { minHeight: 40, minWidth: 220, padding: "0 12px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: 10, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 10, color: "var(--color-warm-700)", verticalAlign: "top" };
const link: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 700, textDecoration: "none" };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff" };
const noteBox: React.CSSProperties = { padding: "14px 16px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)" };
const warnLine: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-warning-50)", color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "12px 16px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
