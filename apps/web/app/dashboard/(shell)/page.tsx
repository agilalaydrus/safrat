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
  IconUser,
  IconUsersGroup,
  IconHome2,
  IconSos,
} from "@tabler/icons-react";
import { PilgrimStats } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { AuditLog } from "@hajj-saas/proto-gen/hajj/v1/operator_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import {
  kloterClient,
  operatorClient,
  pilgrimClient,
  seasonClient,
} from "@/lib/rpc";
import { PageHero } from "@/components/ui/PageHero";
import { StatCard } from "@/components/ui/StatCard";

const QUICK_ACTIONS = [
  { icon: IconUserPlus, title: "Tambah Jamaah", desc: "Daftarkan jamaah baru ke dalam sistem secara manual", href: "/dashboard/pilgrims" },
  { icon: IconFileImport, title: "Import CSV", desc: "Upload data jamaah massal dari file spreadsheet", href: "/dashboard/pilgrims" },
  { icon: IconBuildingHospital, title: "Alokasi Kamar", desc: "Atur penempatan jamaah ke kamar hotel", href: "/dashboard/accommodation" },
  { icon: IconBus, title: "Buat Pergerakan", desc: "Jadwalkan transportasi dan penugasan kursi", href: "/dashboard/transport" },
];

const ACTIVITY_ICON: Record<string, typeof IconUser> = {
  pilgrim: IconUser,
  group: IconUsersGroup,
  accommodation: IconHome2,
  movement: IconBus,
  sos_alert: IconSos,
};

function relativeTime(ts: Timestamp): string {
  const diff = Date.now() - ts.toDate().getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "Baru saja";
  if (mins < 60) return `${mins} mnt lalu`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs} jam lalu`;
  return `${Math.floor(hrs / 24)} hari lalu`;
}

export default function DashboardPage() {
  const router = useRouter();
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [kloterId, setKloterId] = useState("");
  const [stats, setStats] = useState<PilgrimStats | null>(null);
  const [activity, setActivity] = useState<AuditLog[]>([]);
  const [error, setError] = useState("");

  const seasonName = seasons.find((s) => s.id === seasonId)?.name ?? "";

  useEffect(() => {
    let cancelled = false;

    async function loadSeasons() {
      try {
        const response = await seasonClient.listSeasons({});
        const season = response.seasons.find((item) => item.isActive) ?? response.seasons[0];
        if (!cancelled) {
          setSeasons(response.seasons);
          if (season) setSeasonId(season.id);
        }
      } catch (caught) {
        if (cancelled) return;
        if ((caught as { code?: string }).code === "unauthenticated") {
          router.push("/sign-in");
          return;
        }
        setError("Tidak dapat memuat data musim.");
      }
    }

    void loadSeasons();
    return () => { cancelled = true; };
  }, [router]);

  // Switching seasons invalidates whichever kloter was selected — kloter are
  // scoped to one season, so a stale kloterId from the old season would
  // either 404 or silently filter against the wrong season's data.
  useEffect(() => {
    setKloterId("");
    if (!seasonId) { setKloters([]); return; }
    kloterClient.listKloters({ seasonId }).then((response) => setKloters(response.kloters)).catch(() => setKloters([]));
  }, [seasonId]);

  useEffect(() => {
    if (!seasonId) return;
    let cancelled = false;

    async function loadDashboard() {
      setStats(null);
      setError("");
      try {
        const [statsResponse, auditResponse] = await Promise.all([
          pilgrimClient.getPilgrimStats({ seasonId, kloterId }),
          operatorClient.listAuditLogs({ limit: 8 }),
        ]);
        if (!cancelled) {
          setStats(statsResponse);
          setActivity(auditResponse.logs);
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
  }, [router, seasonId, kloterId]);

  return (
    <div>
      <PageHero
        eyebrow="Operator Dashboard"
        title="Selamat datang"
        subtitle={seasonName ? `Musim: ${seasonName}` : "Memuat data..."}
        actions={
          <>
            <select aria-label="Filter musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={filterSelect}>
              {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>)}
            </select>
            <select aria-label="Filter kloter keberangkatan" value={kloterId} onChange={(e) => setKloterId(e.target.value)} style={filterSelect}>
              <option value="">Semua Kloter</option>
              {kloters.map((k) => <option key={k.id} value={k.id}>{k.code}</option>)}
            </select>
          </>
        }
      />

      <div style={body}>
        {error && <p role="alert" style={errorBanner}>{error}</p>}

        <div style={statsGrid}>
          {stats ? (
            <>
              <StatCard label="Total Jamaah" value={stats.total} accent="gold" />
              <StatCard label="Tersubstitusi" value={stats.substituted} accent="danger" />
              <StatCard label="Butuh Kursi Roda" value={stats.requiresWheelchair} accent="emerald" />
              <StatCard label="Belum Ada Rombongan" value={stats.unassignedGroup} accent="gold" />
              <StatCard label="Belum Ada Kloter" value={stats.unassignedKloter} accent="gold" />
            </>
          ) : (
            Array.from({ length: 5 }, (_, index) => <div key={index} style={skeletonCard} />)
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
            </div>
            {activity.length ? activity.map((log) => {
              const Icon = ACTIVITY_ICON[log.entityType] ?? IconUser;
              return (
                <div key={log.id} style={actRow}>
                  <div style={actIcon}><Icon size={14} aria-hidden /></div>
                  <p style={actText}>{log.description}{log.actorName ? <span style={actActor}> · oleh {log.actorName}</span> : null}</p>
                  <p style={actTime}>{log.createdAt ? relativeTime(log.createdAt) : ""}</p>
                </div>
              );
            }) : (
              <p style={emptyActivity}>Belum ada aktivitas tercatat.</p>
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
const filterSelect: React.CSSProperties = { minHeight: 44, maxWidth: 200, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "#fff", font: "inherit" };
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
const actActor: React.CSSProperties = { color: "var(--color-warm-400)" };
const actRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, padding: "10px 18px", borderBottom: "1px solid rgba(237,229,212,.5)" };
const actIcon: React.CSSProperties = { width: 28, height: 28, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)" };
const actText: React.CSSProperties = { flex: 1, fontSize: 12, color: "var(--color-warm-700)", lineHeight: 1.4 };
const actTime: React.CSSProperties = { fontSize: 10, color: "var(--color-warm-400)", flexShrink: 0 };
const emptyActivity: React.CSSProperties = { padding: "24px 18px", fontSize: 13, color: "var(--color-warm-400)", textAlign: "center" };
