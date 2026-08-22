"use client";

import { use, useEffect, useState } from "react";
import { CertificateData } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";
import { pilgrimAppClient } from "@/lib/rpc";

export default function CertificatePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = use(params);
  const [data, setData] = useState<CertificateData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    pilgrimAppClient.getMyCertificate({ appAccessCode: code })
      .then(setData)
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [code]);

  if (loading) return <div style={{ textAlign: "center", padding: 60, color: "var(--color-warm-500)" }}>Memuat sertifikat...</div>;
  if (error || !data) return <div style={{ textAlign: "center", padding: 60, color: "var(--color-warm-500)" }}>Sertifikat tidak ditemukan.</div>;

  const startStr = data.startDate?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) ?? "-";
  const endStr = data.endDate?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) ?? "-";
  const isHajj = data.seasonType === "HAJJ";

  const rows = [
    { label: "Tanggal Berangkat", value: startStr },
    { label: "Tanggal Kembali", value: endStr },
    { label: "Grup", value: data.groupName || "-" },
    { label: "Pembimbing", value: data.leaderName || "-" },
    { label: "Hotel Makkah", value: data.makkahHotels || "-" },
    { label: "Hotel Madinah", value: data.madinahHotels || "-" },
  ];

  return (
    <>
      <style>{`
        @media print {
          .no-print { display: none !important; }
          body { margin: 0; }
        }
      `}</style>

      <div style={{ maxWidth: 800, margin: "0 auto", padding: "40px 32px" }}>
        <div className="no-print" style={{ textAlign: "right", marginBottom: 24 }}>
          <button onClick={() => window.print()} style={printButton}>Cetak / Simpan PDF</button>
        </div>

        <div style={outerFrame}>
          <div style={innerFrame}>
            <p style={eyebrow}>SERTIFIKAT {isHajj ? "HAJI" : "UMRAH"}</p>
            <p style={subEyebrow}>Certificate of Completion</p>

            <p style={{ color: "var(--color-warm-600)", fontSize: 14, margin: "0 0 8px" }}>Diberikan kepada</p>
            <h1 style={pilgrimName}>{data.pilgrimName}</h1>
            <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "0 0 32px" }}>Paspor: {data.passportNumber} · {data.nationality}</p>

            <p style={{ color: "var(--color-warm-600)", fontSize: 14, margin: "0 0 4px" }}>
              telah melaksanakan ibadah <strong>{isHajj ? "Haji" : "Umrah"}</strong>
            </p>
            <p style={seasonName}>{data.seasonName}</p>

            <div style={grid}>
              {rows.map((r) => (
                <div key={r.label} style={cell}>
                  <p style={cellLabel}>{r.label}</p>
                  <p style={cellValue}>{r.value}</p>
                </div>
              ))}
            </div>

            <div style={footer}>
              <p style={{ color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 4px" }}>Diselenggarakan oleh</p>
              <p style={{ fontWeight: 700, fontSize: 16, color: "var(--color-emerald-900)", margin: "0 0 2px" }}>{data.operatorName}</p>
              {data.licenseNumber && <p style={{ color: "var(--color-warm-400)", fontSize: 12, margin: 0 }}>No. Izin: {data.licenseNumber}</p>}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

const printButton: React.CSSProperties = { padding: "10px 20px", background: "var(--color-emerald-900)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700 };
const outerFrame: React.CSSProperties = { border: "8px solid var(--color-gold-500)", borderRadius: 16, padding: "48px 56px", textAlign: "center", background: "var(--color-cream-100)" };
const innerFrame: React.CSSProperties = { border: "2px solid var(--color-gold-300)", borderRadius: 10, padding: "32px 40px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-700)", fontSize: 13, fontWeight: 700, letterSpacing: ".12em", margin: "0 0 4px" };
const subEyebrow: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 13, color: "var(--color-warm-500)", margin: "0 0 32px" };
const pilgrimName: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 36, fontWeight: 700, color: "var(--color-emerald-900)", margin: "0 0 4px" };
const seasonName: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 22, color: "var(--color-emerald-800)", margin: "0 0 32px", fontWeight: 700 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, textAlign: "left", marginBottom: 32 };
const cell: React.CSSProperties = { padding: "12px 16px", background: "#fff", borderRadius: 8, border: "1px solid var(--color-cream-400)" };
const cellLabel: React.CSSProperties = { margin: "0 0 2px", fontSize: 11, color: "var(--color-warm-400)", fontWeight: 600, textTransform: "uppercase", letterSpacing: ".06em" };
const cellValue: React.CSSProperties = { margin: 0, fontSize: 14, fontWeight: 700, color: "var(--color-warm-800)" };
const footer: React.CSSProperties = { borderTop: "1px solid var(--color-cream-400)", paddingTop: 24 };
