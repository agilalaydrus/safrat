"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Timestamp } from "@bufbuild/protobuf";
import {
  IconArrowRight,
  IconBuildingHospital,
  IconBus,
  IconFileImport,
  IconUserPlus,
} from "@tabler/icons-react";
import { PilgrimStats } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import {
  accommodationClient,
  pilgrimClient,
  seasonClient,
  transportClient,
} from "@/lib/rpc";
import { PageHero } from "@/components/ui/PageHero";
import { StatCard } from "@/components/ui/StatCard";

const QUICK_ACTIONS = [
  { icon: IconUserPlus, title: "Tambah Jamaah", desc: "Daftarkan jamaah baru ke dalam sistem secara manual", href: "/dashboard/pilgrims" },
  { icon: IconFileImport, title: "Import CSV", desc: "Upload data jamaah massal dari file spreadsheet", href: "/dashboard/pilgrims" },
  { icon: IconBuildingHospital, title: "Alokasi Kamar", desc: "Atur penempatan jamaah ke kamar hotel", href: "/dashboard/accommodation" },
  { icon: IconBus, title: "Buat Pergerakan", desc: "Jadwalkan transportasi dan penugasan kursi", href: "/dashboard/transport" },
];

function relativeTime(ts: Timestamp): string {
  const diff = Date.now() - ts.toDate().getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins} mnt lalu`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} jam lalu`;
  return `${Math.floor(hrs / 24)} hari lalu`;
}

export default function DashboardPage() {
  const router = useRouter();
  const [seasonId, setSeasonId] = useState("");
  const [seasonName, setSeasonName] = useState("");
  const [stats, setStats] = useState<PilgrimStats | null>(null);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function loadSeason() {
      try {
        const response = await seasonClient.listSeasons({});
        const season = response.seasons.find((item) => item.isActive) ?? response.seasons[0];
        if (!cancelled && season) {
          setSeasonId(season.id);
          setSeasonName(season.name);
        }
      } catch (caught) {
        if (cancelled) return;
        if ((caught as { code?: string }).code === "unauthenticated") {
          router.push("/sign-in");
          return;
        }
        setError("Tidak dapat memuat data musim aktif.");
      }
    }

    void loadSeason();
    return () => { cancelled = true; };
  }, [router]);

  useEffect(() => {
    if (!seasonId) return;
    let cancelled = false;

    async function loadDashboard() {
      setStats(null);
      setError("");
      try {
        const [statsResponse, hotelsResponse, movementsResponse] = await Promise.all([
          pilgrimClient.getPilgrimStats({ seasonId }),
          accommodationClient.listHotels({ seasonId }),
          transportClient.listMovements({ seasonId }),
        ]);
        // Fetch hotels with the other dashboard resources so this view stays in sync.
        void hotelsResponse;
        if (!cancelled) {
          setStats(statsResponse);
          setMovements(movementsResponse.movements);
        }
      } catch (caught) {
        if (cancelled) return;
        if ((caught as { code?: string }).code === "unauthenticated") {
          router.push("/sign-in");
          return;
        }
        setError("Tidak dapat memuat ringkasan dashboard. Silakan coba lagi.");
      }
    }

    void loadDashboard();
    return () => { cancelled = true; };
  }, [router, seasonId]);

  const recentMovements = [...movements]
    .sort((a, b) => (b.scheduledAt?.toDate().getTime() ?? 0) - (a.scheduledAt?.toDate().getTime() ?? 0))
    .slice(0, 3);

  return (
    <div>
      <PageHero
        eyebrow="Operator Dashboard"
        title="Selamat datang"
        subtitle={seasonName ? `Musim aktif: ${seasonName}` : "Memuat data..."}
      />

      <div style={body}>
        {error && <p role="alert" style={errorBanner}>{error}</p>}

        <div style={statsGrid}>
          {stats ? (
            <>
              <StatCard label="Total Jamaah" value={stats.total} accent="gold" />
              <StatCard label="Tersubstitusi" value={stats.substituted} accent="danger" />
              <StatCard label="Butuh Kursi Roda" value={stats.requiresWheelchair} accent="emerald" />
              <StatCard label="Belum Ada Grup" value={stats.unassignedGroup} accent="gold" />
            </>
          ) : (
            Array.from({ length: 4 }, (_, index) => <div key={index} style={skeletonCard} />)
          )}
        </div>

        <section style={{ marginBottom: 32 }}>
          <p className="section-eyebrow">Aksi Cepat</p>
          <h2 style={sectionTitle}>Kelola Operasional</h2>
          <p style={sectionSub}>Tindakan umum untuk operasional harian</p>
          <div style={cardsGrid}>
            {QUICK_ACTIONS.map(({ icon: Icon, title, desc, href }) => (
              <Link key={title} href={href} style={featureCard}>
                <div style={iconWrap}><Icon size={18} color="var(--color-emerald-800)" aria-hidden /></div>
                <p style={cardTitle}>{title}</p>
                <p style={cardDesc}>{desc}</p>
                <span style={cardArrow}>Mulai <IconArrowRight size={12} aria-hidden /></span>
              </Link>
            ))}
          </div>
        </section>

        <section>
          <div style={actCard}>
            <div style={actHead}>
              <p style={actTitle}>Aktivitas Terbaru</p>
              <Link href="/dashboard/transport" style={actLink}>Lihat semua →</Link>
            </div>
            {recentMovements.length ? recentMovements.map((movement) => (
              <div key={movement.id} style={actRow}>
                <div style={actIcon}><IconBus size={14} aria-hidden /></div>
                <p style={actText}>{movement.name} · {movement.origin} → {movement.destination}</p>
                <p style={actTime}>{movement.scheduledAt ? relativeTime(movement.scheduledAt) : "Belum dijadwalkan"}</p>
              </div>
            )) : (
              <p style={emptyActivity}>Belum ada pergerakan yang dijadwalkan.</p>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

const body: React.CSSProperties = { padding: "28px 32px" };
const statsGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(160px,1fr))", gap: 12, marginBottom: 32 };
const skeletonCard: React.CSSProperties = { background: "var(--color-cream-200)", borderRadius: 10, height: 88, animation: "pulse 1.5s ease-in-out infinite" };
const errorBanner: React.CSSProperties = { color: "var(--color-danger-600)", fontSize: 13, marginBottom: 16 };
const sectionTitle: React.CSSProperties = { fontSize: 20, fontWeight: 500, margin: "4px 0 2px" };
const sectionSub: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)", marginBottom: 16 };
const cardsGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12 };
const featureCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "18px 18px 16px", display: "block" };
const iconWrap: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-100)", display: "flex", alignItems: "center", justifyContent: "center", marginBottom: 12 };
const cardTitle: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-900)", marginBottom: 4 };
const cardDesc: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", lineHeight: 1.5 };
const cardArrow: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, marginTop: 10, fontSize: 11, fontWeight: 600, color: "var(--color-gold-600)" };
const actCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10, overflow: "hidden" };
const actHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 18px", borderBottom: "1px solid var(--color-cream-300)" };
const actTitle: React.CSSProperties = { fontSize: 13, fontWeight: 600 };
const actLink: React.CSSProperties = { fontSize: 11, fontWeight: 600, color: "var(--color-gold-600)" };
const actRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, padding: "10px 18px", borderBottom: "1px solid rgba(237,229,212,.5)" };
const actIcon: React.CSSProperties = { width: 28, height: 28, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)" };
const actText: React.CSSProperties = { flex: 1, fontSize: 12, color: "var(--color-warm-700)", lineHeight: 1.4 };
const actTime: React.CSSProperties = { fontSize: 10, color: "var(--color-warm-400)", flexShrink: 0 };
const emptyActivity: React.CSSProperties = { padding: "24px 18px", fontSize: 13, color: "var(--color-warm-400)", textAlign: "center" };
