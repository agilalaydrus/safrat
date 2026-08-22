"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { IconAward, IconBed, IconBuilding, IconBus, IconCircleCheck, IconChecklist, IconClockHour4, IconPlane, IconStar, IconUserCircle, IconUsersGroup, IconWheelchair, IconWifiOff, IconX } from "@tabler/icons-react";
import { PilgrimAppInfo } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch, toDateSafe } from "@/lib/offline";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { GoogleSignInButton } from "@/components/auth/GoogleSignInButton";
import { usePilgrimCode } from "@/lib/pilgrim-context";

const PAYMENT_LABEL: Record<string, { label: string; color: string; bg: string }> = {
  UNPAID: { label: "Belum Bayar", color: "var(--color-danger-600)", bg: "var(--color-danger-100)" },
  DP: { label: "DP (Uang Muka)", color: "var(--color-gold-800)", bg: "var(--color-gold-50)" },
  PAID: { label: "Lunas", color: "var(--color-emerald-900)", bg: "var(--color-emerald-50)" },
};

// "Terkonfirmasi" means both identity (KYC) is verified by admin AND
// payment has started (DP or Lunas) — neither alone is enough, and being
// merely "not cancelled" was never a meaningful confirmation signal.
function confirmationState(status: string, kycStatus: string, paymentStatus: string): "CANCELLED" | "CONFIRMED" | "PENDING" {
  if (status === "CANCELLED") return "CANCELLED";
  if (kycStatus === "VERIFIED" && paymentStatus !== "UNPAID") return "CONFIRMED";
  return "PENDING";
}

export default function PilgrimHomePage() {
  const code = usePilgrimCode();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [info, setInfo] = useState<PilgrimAppInfo>();
  const [fromCache, setFromCache] = useState(false);
  const [error, setError] = useState("");
  const [requesting, setRequesting] = useState(false);
  const [notice, setNotice] = useState("");
  const [linkNotice, setLinkNotice] = useState("");

  useEffect(() => {
    if (!code) return;
    cachedFetch(`pilgrim-info:${code}`, () => pilgrimAppClient.getMyInfo({ appAccessCode: code }))
      .then((result) => {
        if (result.data) setInfo(result.data);
        setFromCache(result.fromCache);
      })
      .catch(() => setError("Data jamaah tidak ditemukan. Silakan hubungi petugas Anda."));
  }, [code]);

  // Landed back here right after Google's redirect (see GoogleSignInButton's
  // callbackURL below) — a Better Auth session now exists in this browser,
  // so complete the link once, then drop the marker from the URL.
  useEffect(() => {
    if (searchParams.get("google") !== "1" || !code) return;
    router.replace("/pilgrim");
    pilgrimAppClient.linkGoogleAccount({ appAccessCode: code })
      .then((result) => {
        invalidateMyAccessCache();
        setLinkNotice(`Akun Google ${result.linkedGoogleEmail} berhasil dihubungkan.`);
        setInfo((current) => (current ? new PilgrimAppInfo({ ...current, linkedGoogleEmail: result.linkedGoogleEmail }) : current));
      })
      .catch(() => setLinkNotice("Gagal menghubungkan akun Google. Silakan coba lagi."));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams, code]);

  async function toggleWheelchair() {
    if (!info) return;
    const next = !info.requiresWheelchair;
    setRequesting(true);
    setNotice("");
    try {
      await pilgrimAppClient.requestWheelchair({ appAccessCode: code, requiresWheelchair: next });
      setInfo(new PilgrimAppInfo({ ...info, requiresWheelchair: next }));
      setNotice(next ? "Permintaan kursi roda terkirim ke petugas." : "Permintaan kursi roda dibatalkan.");
    } catch {
      setNotice("Gagal mengirim permintaan. Periksa koneksi Anda dan coba lagi.");
    } finally {
      setRequesting(false);
    }
  }

  if (error) {
    return <main style={page}><p style={errorText}>{error}</p></main>;
  }
  if (!info) {
    return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;
  }

  const movement = info.nextMovement;
  return (
    <main style={page}>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan data tersimpan — Anda sedang offline</p>}
      <p style={eyebrow}>ASSALAMUALAIKUM</p>
      <h1 style={title}>{info.fullName}</h1>

      <div style={statusBadgeRow}>
        {(() => {
          const state = confirmationState(info.status, info.kycStatus, info.paymentStatus);
          if (state === "CANCELLED") return <span style={statusBadgeCancelled}><IconX size={13} />Pendaftaran Dibatalkan</span>;
          if (state === "CONFIRMED") return <span style={statusBadgeConfirmed}><IconCircleCheck size={13} />Pendaftaran Terkonfirmasi</span>;
          return <span style={statusBadgePending}><IconClockHour4 size={13} />Menunggu Konfirmasi</span>;
        })()}
        <span style={{ ...statusBadgeBase, color: (PAYMENT_LABEL[info.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).color, background: (PAYMENT_LABEL[info.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).bg }}>
          {(PAYMENT_LABEL[info.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).label}
        </span>
      </div>
      {confirmationState(info.status, info.kycStatus, info.paymentStatus) === "PENDING" && (
        <p style={{ margin: "-10px 0 16px", fontSize: 12, color: "var(--color-warm-500)" }}>
          {info.kycStatus !== "VERIFIED" ? "Lengkapi & tunggu verifikasi data KYC Anda di halaman Profil." : "Menunggu pembayaran DP atau pelunasan."}
        </p>
      )}

      <section style={wheelchairCard}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <IconWheelchair size={20} color="var(--color-emerald-800)" />
          <div>
            <p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>Bantuan Kursi Roda</p>
            <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{info.requiresWheelchair ? "Petugas telah diberi tahu kebutuhan Anda." : "Ajukan jika Anda membutuhkan kursi roda."}</p>
          </div>
        </div>
        <button onClick={toggleWheelchair} disabled={requesting || fromCache} style={info.requiresWheelchair ? wheelchairActiveButton : wheelchairButton}>
          {requesting ? "Mengirim..." : info.requiresWheelchair ? "Batalkan Permintaan" : "Minta Kursi Roda"}
        </button>
        {notice && <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-gold-800)" }}>{notice}</p>}
      </section>

      <section style={wheelchairCard}>
        {info.linkedGoogleEmail ? (
          <p style={{ margin: 0, display: "flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--color-emerald-900)" }}>
            <IconCircleCheck size={18} color="var(--color-emerald-800)" />
            Terhubung sebagai <strong>{info.linkedGoogleEmail}</strong>
          </p>
        ) : (
          <>
            <p style={{ margin: "0 0 4px", fontWeight: 700, fontSize: 14 }}>Amankan Akun Anda</p>
            <p style={{ margin: "0 0 12px", fontSize: 12, color: "var(--color-warm-500)" }}>Hubungkan akun Google untuk mengamankan akses Anda — diperlukan sebelum melakukan transaksi apa pun di kemudian hari.</p>
            <GoogleSignInButton callbackURL="/pilgrim?google=1" label="Hubungkan dengan Google" />
          </>
        )}
        {linkNotice && <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-gold-800)" }}>{linkNotice}</p>}
      </section>

      <div style={grid}>
        <div style={card}>
          <IconPlane size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Kloter</p>
          <p style={cardValue}>{info.kloterCode || "Belum ditentukan"}</p>
        </div>
        <div style={card}>
          <IconUsersGroup size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Grup</p>
          <p style={cardValue}>{info.groupName || "Belum ditentukan"}</p>
        </div>
        <div style={card}>
          <IconBuilding size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Hotel</p>
          <p style={cardValue}>{info.hotelName || "Belum ditentukan"}</p>
        </div>
        <div style={card}>
          <IconBed size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Kamar</p>
          <p style={cardValue}>{info.roomNumber || "Belum ditentukan"}</p>
        </div>
      </div>

      {info.hotelStays.length > 1 && (
        <section style={kloterCard}>
          <p style={eyebrow}><IconBuilding size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />HOTEL &amp; KAMAR ANDA</p>
          {info.hotelStays.map((stay, i) => (
            <p key={i} style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>{stay.hotelName}{stay.roomNumber ? ` · Kamar ${stay.roomNumber}` : ""}</p>
          ))}
        </section>
      )}

      {(info.kloterFlightNumber || info.kloterDepartureDate) && (
        <section style={kloterCard}>
          <p style={eyebrow}><IconPlane size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />DETAIL KLOTER {info.kloterCode}</p>
          {info.kloterEmbarkation && <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Embarkasi: {info.kloterEmbarkation}</p>}
          {info.kloterFlightNumber && <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Penerbangan: {info.kloterFlightNumber}</p>}
          {info.kloterDepartureDate && <p style={{ margin: "6px 0 0", fontWeight: 700, color: "var(--color-gold-800)" }}>{toDateSafe(info.kloterDepartureDate)?.toLocaleDateString("id-ID", { day: "2-digit", month: "long", year: "numeric" })}</p>}
        </section>
      )}

      <Link href="/pilgrim/checklist" style={surveyCard}>
        <IconChecklist size={20} color="var(--color-emerald-800)" />
        <div><p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>Checklist Persiapan</p><p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>Pantau kelengkapan dokumen dan persiapan keberangkatan Anda.</p></div>
      </Link>

      <a href={`/certificate/${code}`} target="_blank" rel="noreferrer" style={surveyCard}>
        <IconAward size={20} color="var(--color-gold-800)" />
        <div><p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>Unduh Sertifikat</p><p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>Lihat dan cetak sertifikat perjalanan Anda.</p></div>
      </a>

      <Link href="/pilgrim/survey" style={surveyCard}>
        <IconStar size={20} color="var(--color-gold-800)" />
        <div><p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>Beri Ulasan Perjalanan</p><p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>Bagikan pengalaman perjalanan Anda kepada operator.</p></div>
      </Link>

      <Link href="/pilgrim/profile" style={surveyCard}>
        <IconUserCircle size={20} color="var(--color-emerald-800)" />
        <div><p style={{ margin: 0, fontWeight: 700, fontSize: 14 }}>Profil Saya</p><p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>Info kontak, data perjalanan, dan ganti kata sandi.</p></div>
      </Link>

      {movement && (
        <section style={nextCard}>
          <p style={eyebrow}><IconBus size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />JADWAL BERIKUTNYA</p>
          <h2 style={{ margin: "4px 0 6px", fontSize: 22 }}>{movement.name}</h2>
          <p style={{ margin: 0, color: "var(--color-warm-500)" }}>{movement.origin} → {movement.destination}</p>
          {movement.mode === "FLIGHT" && (movement.airline || movement.flightNumber) && <p style={{ margin: "2px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{[movement.airline, movement.flightNumber].filter(Boolean).join(" · ")}</p>}
          <p style={{ margin: "6px 0 0", fontWeight: 700, color: "var(--color-gold-800)" }}>{toDateSafe(movement.scheduledAt)?.toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
        </section>
      )}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, margin: "0 0 16px" };
const wheelchairCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, margin: "0 0 16px" };
const wheelchairButton: React.CSSProperties = { width: "100%", minHeight: 44, marginTop: 12, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 700 };
const wheelchairActiveButton: React.CSSProperties = { ...wheelchairButton, background: "var(--color-gold-50)", border: "1px solid var(--color-gold-500)", color: "var(--color-gold-800)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 10 };
const kloterCard: React.CSSProperties = { marginTop: 12, background: "var(--color-gold-50)", border: "1px solid var(--color-gold-200)", borderRadius: 14, padding: 16 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "grid", gap: 4 };
const cardLabel: React.CSSProperties = { margin: 0, fontSize: 11, color: "var(--color-warm-400)" };
const cardValue: React.CSSProperties = { margin: 0, fontSize: 13, fontWeight: 700, color: "var(--color-warm-900)" };
const nextCard: React.CSSProperties = { marginTop: 20, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 14, padding: 18 };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 16 };
const statusBadgeRow: React.CSSProperties = { display: "flex", gap: 8, flexWrap: "wrap", margin: "10px 0 16px" };
const statusBadgeBase: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 5, padding: "5px 10px", borderRadius: 99, fontSize: 12, fontWeight: 700 };
const statusBadgeConfirmed: React.CSSProperties = { ...statusBadgeBase, color: "var(--color-emerald-900)", background: "var(--color-emerald-50)" };
const statusBadgeCancelled: React.CSSProperties = { ...statusBadgeBase, color: "var(--color-danger-600)", background: "var(--color-danger-100)" };
const statusBadgePending: React.CSSProperties = { ...statusBadgeBase, color: "var(--color-gold-800)", background: "var(--color-gold-50)" };
const errorText: React.CSSProperties = { color: "var(--color-danger-600)" };
const surveyCard: React.CSSProperties = { marginTop: 16, display: "flex", alignItems: "center", gap: 10, background: "var(--color-gold-50)", border: "1px solid var(--color-gold-200)", borderRadius: 14, padding: 14, textDecoration: "none" };
