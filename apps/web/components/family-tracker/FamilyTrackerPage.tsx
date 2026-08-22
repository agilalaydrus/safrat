"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle } from "@tabler/icons-react";
import { FamilyStatus } from "@hajj-saas/proto-gen/hajj/v1/family_tracker_pb";
import { familyTrackerClient } from "@/lib/rpc";

const PAY_LABEL: Record<string, string> = { PAID: "Lunas", DP: "DP", UNPAID: "Belum Bayar" };
const PAY_COLOR: Record<string, string> = { PAID: "var(--color-emerald-800)", DP: "var(--color-gold-800)", UNPAID: "var(--color-danger-600)" };

export default function FamilyTrackerPage({ code }: { code: string }) {
  const [status, setStatus] = useState<FamilyStatus>();
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    familyTrackerClient.getFamilyStatus({ appAccessCode: code }).then(setStatus).catch(() => setNotFound(true)).finally(() => setLoading(false));
  }, [code]);

  if (loading) return <Centered><p style={{ color: "var(--color-warm-400)" }}>Memuat status...</p></Centered>;
  if (notFound || !status) return <Centered>
    <p style={brand}>Tawafiq Hub</p>
    <p style={{ color: "var(--color-warm-500)", marginTop: 8 }}>Kode pelacak tidak valid atau sudah tidak aktif.</p>
  </Centered>;

  const departureStr = status.departureDate ? status.departureDate.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }) : "-";
  const lastSeenStr = status.lastLocationAt ? status.lastLocationAt.toDate().toLocaleString("id-ID") : "Belum ada pembaruan";

  const rows: [string, string][] = [
    ["Musim", status.seasonName],
    ["Tanggal Berangkat", departureStr],
    ["Check-in Hotel", status.hotelCheckedIn ? "Sudah" : "Belum"],
    ["Grup", status.groupName || "-"],
    ["Koordinator", status.leaderName || "-"],
    ["Pembaruan Lokasi Terakhir", lastSeenStr],
  ];
  if (status.ritualsTotal > 0) rows.push(["Progress Ibadah", `${status.ritualsCompleted} dari ${status.ritualsTotal} ritual selesai`]);
  if (status.pilgrimStatus === "CANCELLED") rows.push(["Status Keberangkatan", "Dibatalkan"]);

  return <main style={page}>
    <div style={{ maxWidth: 480, margin: "0 auto" }}>
      <p style={brand}>Tawafiq Hub</p>
      <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "4px 0 32px" }}>Pelacak Status Jamaah</p>

      {status.hasActiveSos && <div style={sosBanner}>
        <IconAlertTriangle size={18} style={{ flexShrink: 0 }} />
        Jamaah ini saat ini membutuhkan bantuan. Tim koordinator sedang menangani.
      </div>}

      <div style={card}>
        <div style={cardHead}>
          <p style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, margin: "0 0 4px" }}>Nama Jamaah</p>
          <p style={{ color: "#fff", fontSize: 24, fontWeight: 700, margin: 0 }}>{status.firstName}</p>
          {status.journeyStatusLabel && <p style={{ color: "var(--color-gold-300)", fontSize: 14, fontWeight: 600, margin: 0 }}>{status.journeyStatusLabel}</p>}
          <span style={{ ...payBadge, background: PAY_COLOR[status.paymentStatus] ?? "var(--color-warm-500)" }}>{PAY_LABEL[status.paymentStatus] ?? status.paymentStatus}</span>
        </div>
        <div style={cardBody}>
          {rows.map(([label, value]) => <div key={label} style={dataRow}>
            <span style={{ fontSize: 13, color: "var(--color-warm-500)" }}>{label}</span>
            <span style={{ fontSize: 14, fontWeight: 600, color: "var(--color-warm-800)" }}>{value}</span>
          </div>)}
        </div>
        <div style={cardFoot}>
          <p style={{ fontSize: 11, color: "var(--color-warm-400)", margin: 0 }}>Halaman ini hanya menampilkan informasi ringkas untuk ketenangan keluarga.</p>
        </div>
      </div>
    </div>
  </main>;
}

function Centered({ children }: { children: React.ReactNode }) {
  return <main style={{ minHeight: "100vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", gap: 8 }}>{children}</main>;
}

const page: React.CSSProperties = { minHeight: "100vh", background: "var(--color-cream-100)", padding: "40px 24px" };
const brand: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontSize: 28, color: "var(--color-emerald-900)", margin: 0 };
const sosBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, background: "var(--color-danger-100)", border: "1px solid var(--color-danger-600)", borderRadius: 10, padding: "14px 18px", marginBottom: 20, color: "var(--color-danger-600)", fontWeight: 700, fontSize: 14 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, overflow: "hidden" };
const cardHead: React.CSSProperties = { background: "var(--color-emerald-900)", padding: "20px 24px", display: "grid", gap: 8, justifyItems: "start" };
const payBadge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "4px 12px" };
const cardBody: React.CSSProperties = { padding: "20px 24px", display: "grid", gap: 16 };
const dataRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--color-cream-300)", paddingBottom: 12 };
const cardFoot: React.CSSProperties = { padding: "12px 24px", background: "var(--color-cream-100)", borderTop: "1px solid var(--color-cream-300)" };
