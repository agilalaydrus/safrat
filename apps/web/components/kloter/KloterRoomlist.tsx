"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconBed, IconDownload } from "@tabler/icons-react";
import type { GetKloterRoomlistResponse } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { kloterClient } from "@/lib/rpc";

const GENDER_LABEL: Record<string, string> = { male: "Laki-laki", female: "Perempuan", family: "Keluarga" };
const dateOnly = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) : "";

export default function KloterRoomlist({ kloterId }: { kloterId: string }) {
  const [list, setList] = useState<GetKloterRoomlistResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    setLoading(true);
    kloterClient
      .getKloterRoomlist({ kloterId })
      .then(setList)
      .catch(() => setFailure("Gagal memuat roomlist."))
      .finally(() => setLoading(false));
  }, [kloterId]);

  // The file the hotel receives. Built from the same rows the screen shows so
  // the two can never disagree, with a BOM so Excel on Windows does not mangle
  // Indonesian names.
  const download = () => {
    if (!list) return;
    const escape = (value: string) => `"${String(value ?? "").replaceAll('"', '""')}"`;
    const lines = [["Kota", "Hotel", "Kamar", "Tipe", "Peruntukan", "Kapasitas", "Penghuni"].map(escape).join(",")];
    for (const hotel of list.hotels) {
      for (const room of hotel.rooms) {
        lines.push([
          hotel.city, hotel.name, room.roomNumber, room.roomType,
          GENDER_LABEL[room.designatedGender] ?? room.designatedGender,
          String(room.capacity),
          room.occupants.map((occupant) => occupant.fullName).join(" · "),
        ].map(escape).join(","));
      }
    }
    for (const occupant of list.unassigned) {
      lines.push(["", "BELUM DAPAT KAMAR", "", "", "", "", occupant.fullName].map(escape).join(","));
    }
    const blob = new Blob(["﻿" + lines.join("\r\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `roomlist-${list.kloterCode || kloterId}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (loading) return <p style={muted}>Memuat roomlist…</p>;
  if (failure) return <p style={errorBox}><IconAlertTriangle size={15} />{failure}</p>;
  if (!list) return null;

  const flagged = list.hotels.flatMap((hotel) => hotel.rooms.filter((room) => room.mixedWithoutMahram)).length;

  return (
    <section style={card}>
      <div style={headerRow}>
        <div>
          <h2 style={sectionTitle}><IconBed size={18} color="var(--color-emerald-800)" />Roomlist Hotel</h2>
          <p style={muted}>
            {list.totalPilgrims} jamaah · {list.unassigned.length} belum dapat kamar · {list.bedsFree} tempat tidur kosong
          </p>
        </div>
        <button type="button" onClick={download} style={downloadButton} disabled={list.hotels.length === 0}>
          <IconDownload size={15} />Unduh CSV
        </button>
      </div>

      {list.unassigned.length > 0 && (
        <div style={urgentBox}>
          <strong style={{ fontSize: 13 }}>{list.unassigned.length} jamaah belum punya kamar</strong>
          <p style={{ ...muted, fontSize: 12, marginTop: 6 }}>
            {list.unassigned.map((occupant) => occupant.fullName).join(" · ")}
          </p>
        </div>
      )}

      {flagged > 0 && (
        <div style={reviewBox}>
          <strong style={{ fontSize: 13 }}>{flagged} kamar keluarga perlu dilihat lagi</strong>
          <p style={{ ...muted, fontSize: 12, marginTop: 6 }}>
            Berisi laki-laki dan perempuan yang mahramnya tidak ada di kamar itu. Aturan alokasi tidak bisa menangkap
            ini — kamar keluarga memang menerima siapa saja — dan tidak selalu salah, tapi selalu layak diperiksa
            sebelum daftarnya dikirim ke hotel.
          </p>
        </div>
      )}

      {list.hotels.length === 0 ? (
        <p style={muted}>Belum ada kamar yang dialokasikan untuk kloter ini.</p>
      ) : (
        list.hotels.map((hotel) => (
          <div key={hotel.hotelId} style={hotelBlock}>
            <div style={hotelHead}>
              <strong>{hotel.name}</strong>
              <span style={{ color: "var(--color-warm-400)" }}>
                {hotel.city}
                {hotel.checkInDate && ` · ${dateOnly(hotel.checkInDate)} – ${dateOnly(hotel.checkOutDate)}`}
              </span>
            </div>
            <div style={roomGrid}>
              {hotel.rooms.map((room) => (
                <div key={room.roomId} style={room.mixedWithoutMahram ? { ...roomCard, borderColor: "var(--color-warning-200)", background: "var(--color-warning-50)" } : roomCard}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                    <strong>Kamar {room.roomNumber}</strong>
                    <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>
                      {room.occupants.length}/{room.capacity} · {GENDER_LABEL[room.designatedGender] ?? room.designatedGender}
                    </span>
                  </div>
                  <ul style={occupantList}>
                    {room.occupants.map((occupant) => (
                      <li key={occupant.pilgrimId}>
                        {occupant.fullName}
                        {occupant.hasMahram && !occupant.mahramInRoom && (
                          <span style={{ color: "var(--color-warning-700)", fontSize: 11 }}> · mahram di kamar lain</span>
                        )}
                      </li>
                    ))}
                  </ul>
                  {room.bedsFree > 0 && (
                    <small style={{ color: "var(--color-warm-400)" }}>{room.bedsFree} tempat tidur kosong</small>
                  )}
                  {room.bedsFree < 0 && (
                    <small style={{ color: "var(--color-danger-600)", fontWeight: 700 }}>
                      kelebihan {Math.abs(room.bedsFree)} orang
                    </small>
                  )}
                </div>
              ))}
            </div>
          </div>
        ))
      )}
    </section>
  );
}

const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 12, padding: 22, marginTop: 16 };
const headerRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", alignItems: "flex-start", marginBottom: 14 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16, fontWeight: 700, display: "flex", alignItems: "center", gap: 8 };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: "6px 0 0", lineHeight: 1.6 };
const hotelBlock: React.CSSProperties = { marginTop: 16 };
const hotelHead: React.CSSProperties = { display: "flex", gap: 10, alignItems: "baseline", flexWrap: "wrap", fontSize: 14, marginBottom: 10 };
const roomGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(220px,1fr))", gap: 10 };
const roomCard: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: 12, background: "var(--color-cream-100)", fontSize: 13 };
const occupantList: React.CSSProperties = { margin: "8px 0 6px", paddingLeft: 18, display: "grid", gap: 3, color: "var(--color-warm-700)" };
const urgentBox: React.CSSProperties = { padding: "12px 16px", borderRadius: 10, background: "var(--color-danger-100)", color: "var(--color-danger-600)", marginBottom: 12 };
const reviewBox: React.CSSProperties = { padding: "12px 16px", borderRadius: 10, background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", color: "var(--color-warning-700)", marginBottom: 12 };
const downloadButton: React.CSSProperties = { minHeight: 40, padding: "0 16px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-emerald-900)", font: "inherit", fontWeight: 700, fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" };
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "12px 16px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
