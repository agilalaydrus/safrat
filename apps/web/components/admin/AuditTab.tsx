"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { IconAlertTriangle, IconDownload, IconLock } from "@tabler/icons-react";
import type { AuditTrailEntry } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { platformClient } from "@/lib/rpc";

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url; a.download = filename;
  document.body.appendChild(a); a.click(); document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

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
  const [exporting, setExporting] = useState(false);

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

  // C4 (TUGAS-PANEL-SAAS.md): exports the same filtered trail as CSV, signed
  // so an auditor can prove the file they were handed is the exact one this
  // platform produced. The CSV bytes and the manifest arrive on the same
  // stream — accumulated here and written to disk untouched, never
  // re-encoded, so what gets hashed server-side and what lands on disk are
  // provably identical.
  const exportAuditor = async () => {
    setExporting(true);
    setFailure("");
    try {
      const csvParts: Uint8Array[] = [];
      let manifest: { sha256: string; signedAt: string; keyFingerprint: string; hmacSha256: string } | undefined;
      for await (const chunk of platformClient.exportAuditTrail({ category, days, actor: actor.trim() })) {
        if (chunk.payload.case === "csvChunk") csvParts.push(chunk.payload.value);
        else if (chunk.payload.case === "manifest") manifest = chunk.payload.value;
      }
      if (!manifest) throw new Error("Server tidak mengirim manifes ekspor.");
      const stamp = new Date().toISOString().replace(/[:.]/g, "-");
      downloadBlob(new Blob(csvParts as BlobPart[], { type: "text/csv;charset=utf-8" }), `audit-trail-${stamp}.csv`);
      downloadBlob(
        new Blob([JSON.stringify(manifest, null, 2)], { type: "application/json" }),
        `audit-trail-${stamp}.manifest.json`,
      );
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal mengekspor jejak audit.");
    } finally {
      setExporting(false);
    }
  };

  return (
    <section className="tw-screen">
      <PageHeader
        eyebrow="TAWAFIQHUB / AUDIT"
        title="Audit"
        subtitle={
          <>
            Jejak lintas seluruh travel. <strong>Hanya bisa dibaca</strong> — migrasi 125 sudah mencabut UPDATE dan
            DELETE dari peran aplikasi, jadi tombol hapus di sini akan menjadi tombol yang pasti gagal.
          </>
        }
        primaryAction={
          <button type="button" className="tw-btn tw-btn--emerald" onClick={() => void exportAuditor()} disabled={exporting}>
            <IconDownload size={16} />{exporting ? "Mengekspor…" : "Ekspor auditor"}
          </button>
        }
      />

      <div className="tw-filter-bar">
        <div className="tw-segmented" role="group" aria-label="Kategori">
          {CATEGORIES.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setCategory(value)} aria-pressed={category === value}
              className={category === value ? "tw-segmented__item is-active" : "tw-segmented__item"}>{label}</button>
          ))}
        </div>
        <div className="tw-segmented" role="group" aria-label="Rentang waktu">
          {PERIODS.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setDays(value)} aria-pressed={days === value}
              className={days === value ? "tw-segmented__item is-active" : "tw-segmented__item"}>{label}</button>
          ))}
        </div>
        <input
          value={actor}
          onChange={(event) => setActor(event.target.value)}
          placeholder="Cari pelaku (email atau id)"
          aria-label="Cari pelaku"
          className="tw-search"
        />
      </div>

      {failure && <p className="tw-inline-alert" data-tone="danger"><IconAlertTriangle size={16} />{failure}</p>}
      {loading && <p className="tw-note">Memuat…</p>}

      {!loading && entries.length === 0 && (
        <EmptyState
          title="Tidak ada kejadian yang cocok"
          cause="Saringan kategori, rentang waktu, dan pelaku sedang berlaku bersamaan."
          nextStep="Longgarkan salah satunya — daftar ini benar-benar kosong, bukan gagal memuat."
        />
      )}

      {entries.length > 0 && (
        <>
          {truncated && (
            <p className="tw-inline-alert" data-tone="warning">
              <IconAlertTriangle size={15} />
              Daftar terpotong di 200 baris. Persempit rentang waktu atau saringannya — ekor yang kosong bukan berarti
              tidak ada kejadian lain.
            </p>
          )}
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead>
                <tr>{["Waktu", "Pelaku", "Tindakan", "Travel", "Objek", "Alasan"].map((head) => (
                  <th key={head}>{head}</th>
                ))}</tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id}>
                    <td style={{ whiteSpace: "nowrap" }}>{timeOf(entry.at)}</td>
                    <td>{entry.actor}</td>
                    <td style={{ fontWeight: 600 }}>{ACTION_LABEL[entry.action] ?? entry.action}</td>
                    <td>
                      {entry.operatorId
                        ? <Link href={`/admin/tenant/${entry.operatorId}`} style={link}>{entry.operatorName || entry.operatorId}</Link>
                        : <span style={{ color: "var(--color-warm-400)" }}>platform</span>}
                    </td>
                    <td style={{ color: "var(--color-warm-400)" }}>{entry.entityType}</td>
                    <td style={{ maxWidth: 320 }}>{entry.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      <div className="tw-note">
        <p>
          <IconLock size={13} style={{ verticalAlign: "-2px", marginRight: 6 }} />
          <strong>Pembacaan data pribadi lewat sesi lihat-saja tidak ada di tabel ini.</strong> Ia dicatat terpisah
          sebagai hitungan harian — satu baris per orang, per layar, per hari — karena satu baris per permintaan akan
          jadi puluhan ribu baris yang mengubur segalanya. Ada di halaman detail tenant masing-masing.
        </p>
        <p>
          <IconDownload size={13} style={{ verticalAlign: "-2px", marginRight: 6 }} />
          <strong>Ekspor auditor</strong> mengunduh dua berkas: CSV berisi seluruh baris yang cocok dengan saringan di
          atas (tanpa batas 200 baris), dan <code>manifest.json</code> yang membuktikan keasliannya — hash dari CSV
          itu, ditandatangani dengan kunci milik platform. Auditor dapat menghitung ulang hash CSV dan mencocokkannya
          ke manifes untuk memastikan berkas itu tidak diubah.
        </p>
      </div>
    </section>
  );
}

const link: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 700, textDecoration: "none" };
