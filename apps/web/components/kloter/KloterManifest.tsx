"use client";

import { useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconDownload, IconFileText } from "@tabler/icons-react";
import type { GetKloterManifestResponse } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { kloterClient } from "@/lib/rpc";

// The six documents an Indonesian umrah departure needs, in the order the
// manifest lists them. Marriage or kinship proof is only asked of somebody
// travelling under a mahram — the server decides that per pilgrim, and this
// screen only renders what it was told.
const DOCUMENT_LABEL: Record<string, string> = {
  PASPOR: "Paspor",
  VAKSIN: "Vaksin meningitis",
  FOTO: "Foto biometrik",
  KTP: "KTP",
  KK: "Kartu Keluarga",
  BUKU_NIKAH: "Buku nikah / bukti mahram",
};

const dateOnly = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) : "";

export default function KloterManifest({ kloterId }: { kloterId: string }) {
  const [manifest, setManifest] = useState<GetKloterManifestResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");
  const [onlyIncomplete, setOnlyIncomplete] = useState(false);

  useEffect(() => {
    setLoading(true);
    kloterClient
      .getKloterManifest({ kloterId })
      .then(setManifest)
      .catch(() => setFailure("Gagal memuat manifes."))
      .finally(() => setLoading(false));
  }, [kloterId]);

  const rows = useMemo(() => {
    if (!manifest) return [];
    return onlyIncomplete ? manifest.rows.filter((row) => !row.documentsComplete) : manifest.rows;
  }, [manifest, onlyIncomplete]);

  // Built from the same rows the table shows, so the file and the screen can
  // never disagree. Generated in the browser rather than fetched again: a
  // second endpoint would be a second definition of what a manifest contains.
  const download = () => {
    if (!manifest) return;
    const header = ["Nama", "Paspor", "Kelamin", "Lahir", "Rombongan", "Kamar", "Telepon", "Kekurangan dokumen"];
    const escape = (value: string) => `"${value.replaceAll('"', '""')}"`;
    const lines = [header.map(escape).join(",")];
    for (const row of manifest.rows) {
      lines.push([
        row.fullName, row.passportNumber, row.gender, dateOnly(row.dateOfBirth),
        row.groupName, row.roomLabel, row.phone,
        row.missingDocuments.map((code) => DOCUMENT_LABEL[code] ?? code).join(" · "),
      ].map((value) => escape(String(value ?? ""))).join(","));
    }
    // BOM first: without it Excel on Windows reads the file as Latin-1 and
    // every Indonesian name with an accent arrives mangled.
    const blob = new Blob(["﻿" + lines.join("\r\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `manifes-${manifest.kloterCode || kloterId}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (loading) return <p style={muted}>Memuat manifes…</p>;
  if (failure) return <p style={errorBox}><IconAlertTriangle size={15} />{failure}</p>;
  if (!manifest) return null;

  const total = manifest.rows.length;
  const ready = manifest.readyCount;
  const blocking = Object.entries(manifest.missingByDocument).sort((a, b) => b[1] - a[1]);

  return (
    <section style={card}>
      <div style={headerRow}>
        <div>
          <h2 style={sectionTitle}><IconFileText size={18} color="var(--color-emerald-800)" />Manifes</h2>
          <p style={muted}>
            {total} jamaah · {ready} dokumennya lengkap · {total - ready} belum
            {manifest.capacity > 0 && ` · kapasitas ${manifest.capacity}`}
          </p>
        </div>
        <button type="button" onClick={download} style={downloadButton} disabled={total === 0}>
          <IconDownload size={15} />Unduh CSV
        </button>
      </div>

      {total === 0 ? (
        <p style={muted}>Belum ada jamaah di kloter ini.</p>
      ) : (
        <>
          {blocking.length > 0 && (
            <div style={blockingBox}>
              <strong style={{ fontSize: 13 }}>Yang paling menahan keberangkatan</strong>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 8 }}>
                {blocking.map(([code, count]) => (
                  <span key={code} style={chip}>
                    {DOCUMENT_LABEL[code] ?? code}: <strong>{count}</strong>
                  </span>
                ))}
              </div>
              <p style={{ ...muted, fontSize: 12, marginTop: 8 }}>
                Buku nikah hanya dihitung untuk jamaah yang berangkat dengan mahram. Menuntutnya dari semua orang akan
                membuat sebagian besar manifes terlihat kurang untuk dokumen yang memang tidak diminta dari mereka.
              </p>
            </div>
          )}

          <label style={filterRow}>
            <input type="checkbox" checked={onlyIncomplete} onChange={(event) => setOnlyIncomplete(event.target.checked)} />
            Tampilkan hanya yang dokumennya belum lengkap
          </label>

          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead>
                <tr>{["Nama", "Paspor", "Rombongan", "Kamar", "Kekurangan"].map((head) => (
                  <th key={head} style={th}>{head}</th>
                ))}</tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.pilgrimId} style={row.documentsComplete ? tr : { ...tr, background: "var(--color-warning-50)" }}>
                    <td style={td}>
                      <strong>{row.fullName}</strong>
                      <small style={{ display: "block", color: "var(--color-warm-400)" }}>
                        {row.gender === "FEMALE" ? "Perempuan" : "Laki-laki"}
                        {row.dateOfBirth && ` · ${dateOnly(row.dateOfBirth)}`}
                      </small>
                    </td>
                    <td style={td}>{row.passportNumber || <span style={warn}>belum ada</span>}</td>
                    <td style={td}>{row.groupName || "—"}</td>
                    <td style={td}>{row.roomLabel || "—"}</td>
                    <td style={td}>
                      {row.documentsComplete
                        ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 700 }}>lengkap</span>
                        : row.missingDocuments.map((code) => (
                            <span key={code} style={missingChip}>{DOCUMENT_LABEL[code] ?? code}</span>
                          ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {rows.length === 0 && <p style={muted}>Semua jamaah dokumennya sudah lengkap.</p>}
        </>
      )}
    </section>
  );
}

const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 12, padding: 22, marginTop: 16 };
const headerRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", alignItems: "flex-start", marginBottom: 14 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16, fontWeight: 700, display: "flex", alignItems: "center", gap: 8 };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: "6px 0 0", lineHeight: 1.6 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: 10, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 10, color: "var(--color-warm-700)", verticalAlign: "top" };
const warn: React.CSSProperties = { color: "var(--color-warning-700)", fontWeight: 700 };
const missingChip: React.CSSProperties = { display: "inline-block", margin: "0 4px 4px 0", padding: "2px 8px", borderRadius: 99, background: "var(--color-warning-200)", color: "var(--color-warning-700)", fontSize: 11, fontWeight: 700 };
const chip: React.CSSProperties = { display: "inline-block", padding: "4px 10px", borderRadius: 99, background: "#fff", border: "1px solid var(--color-cream-400)", fontSize: 12 };
const blockingBox: React.CSSProperties = { padding: "14px 16px", borderRadius: 10, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", marginBottom: 14 };
const filterRow: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--color-warm-700)", marginBottom: 12 };
const downloadButton: React.CSSProperties = { minHeight: 40, padding: "0 16px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-emerald-900)", font: "inherit", fontWeight: 700, fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" };
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "12px 16px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
