"use client";

import { useCallback, useEffect, useState } from "react";
import { IconAlertTriangle, IconDownload, IconFileZip } from "@tabler/icons-react";
import type { DataExportRow } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { dataExportClient } from "@/lib/rpc";

const STATUS_LABEL: Record<string, { label: string; color: string }> = {
  PENDING: { label: "menunggu diproses", color: "var(--color-warm-500)" },
  PROCESSING: { label: "sedang disusun", color: "var(--color-gold-800)" },
  READY: { label: "siap diunduh", color: "var(--color-emerald-900)" },
  FAILED: { label: "gagal", color: "var(--color-danger-600)" },
};

const dateTime = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }) : "";

const size = (bytes: bigint | number) => {
  const mb = Number(bytes) / 1_048_576;
  return mb >= 1 ? `${mb.toFixed(1)} MB` : `${(Number(bytes) / 1024).toFixed(0)} KB`;
};

/**
 * The portability right — UU PDP — not a sales feature.
 *
 * An export is built in the background (it can take real time on a large
 * catalogue), so this screen polls its own history rather than blocking on a
 * request. Every download link is minted fresh and expires; none is stored or
 * shown until the operator asks for it again.
 */
export default function DataExportPanel() {
  const [exports, setExports] = useState<DataExportRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [requesting, setRequesting] = useState(false);
  const [notice, setNotice] = useState("");
  const [downloadingId, setDownloadingId] = useState("");

  const load = useCallback(() => {
    dataExportClient
      .listDataExports({ limit: 20 })
      .then((response) => setExports(response.exports))
      .catch(() => setNotice("Gagal memuat riwayat ekspor."))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  // Poll only while something is actually in flight — an export that has
  // settled one way or the other needs no more checking, and polling forever
  // would just be a tab quietly calling the server for nothing.
  useEffect(() => {
    const inFlight = exports.some((row) => row.status === "PENDING" || row.status === "PROCESSING");
    if (!inFlight) return;
    const timer = window.setInterval(load, 4000);
    return () => window.clearInterval(timer);
  }, [exports, load]);

  const request = async () => {
    setRequesting(true);
    setNotice("");
    try {
      await dataExportClient.requestDataExport({ idempotencyKey: `export-${crypto.randomUUID()}` });
      load();
    } catch (error: unknown) {
      setNotice(error instanceof Error ? error.message : "Gagal meminta ekspor.");
    } finally {
      setRequesting(false);
    }
  };

  const download = async (id: string) => {
    setDownloadingId(id);
    setNotice("");
    try {
      const result = await dataExportClient.getDataExportDownloadUrl({ exportId: id });
      window.location.href = result.url;
    } catch (error: unknown) {
      setNotice(error instanceof Error ? error.message : "Gagal membuat tautan unduhan.");
    } finally {
      setDownloadingId("");
    }
  };

  return (
    <div style={{ display: "grid", gap: 16 }}>
      <div style={card}>
        <h2 style={{ margin: "0 0 6px" }}>Ekspor Data Saya</h2>
        <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--color-warm-500)", lineHeight: 1.6 }}>
          Musim, paket, jamaah, dan transaksi yang tersimpan atas nama travel Anda — dalam satu berkas yang bisa
          disimpan. Ini hak portabilitas data menurut UU PDP, bukan sekadar fitur.
        </p>
        <button disabled={requesting} onClick={request} style={emerald}>
          {requesting ? "Meminta…" : "Minta Ekspor Baru"}
        </button>
        <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
          Disusun di latar belakang — bisa makan waktu beberapa menit untuk katalog besar. Tautan unduhan berlaku 7
          hari, dan dibuat baru setiap kali Anda memintanya, bukan disimpan di layar ini.
        </p>
      </div>

      {notice && <p style={warning}><IconAlertTriangle size={15} />{notice}</p>}

      <div style={card}>
        <h3 style={{ margin: "0 0 12px", fontSize: 15 }}>Riwayat</h3>
        {loading ? (
          <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>Memuat…</p>
        ) : exports.length === 0 ? (
          <p style={{ margin: 0, color: "var(--color-warm-400)", fontSize: 13 }}>Belum pernah meminta ekspor.</p>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {exports.map((row) => {
              const status = STATUS_LABEL[row.status] ?? STATUS_LABEL.PENDING!;
              return (
                <div key={row.id} style={rowStyle}>
                  <IconFileZip size={18} style={{ color: "var(--color-warm-400)", flexShrink: 0 }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: "flex", gap: 8, alignItems: "baseline", flexWrap: "wrap" }}>
                      <strong style={{ fontSize: 13 }}>{dateTime(row.requestedAt)}</strong>
                      <span style={{ fontSize: 12, fontWeight: 700, color: status.color }}>{status.label}</span>
                      {row.status === "READY" && <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>{size(row.sizeBytes)}</span>}
                    </div>
                    {row.status === "FAILED" && row.error && (
                      <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-danger-600)" }}>{row.error}</p>
                    )}
                    {row.status === "READY" && row.expiresAt && (
                      <p style={{ margin: "2px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>
                        Tautan sebelumnya kedaluwarsa {dateTime(row.expiresAt)} — minta lagi kalau sudah lewat.
                      </p>
                    )}
                  </div>
                  {row.status === "READY" && (
                    <button onClick={() => download(row.id)} disabled={downloadingId === row.id} style={ghostBtn}>
                      <IconDownload size={15} />{downloadingId === row.id ? "…" : "Unduh"}
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px" };
const ghostBtn: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, padding: "0 12px", display: "inline-flex", gap: 6, alignItems: "center", fontSize: 12 };
const rowStyle: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 12px" };
const warning: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 13 };
