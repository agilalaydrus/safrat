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
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import ProfileShareBanner from "@/components/settings/ProfileShareBanner";

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
      <PageHeader
        eyebrow="Operator Dashboard"
        title="Selamat datang"
        subtitle={seasonName ? `Musim: ${seasonName}` : "Memuat data..."}
        controls={
          <>
            <select className="dashboard-filter-select" aria-label="Filter musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)}>
              {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>)}
            </select>
            <select className="dashboard-filter-select" aria-label="Filter kloter keberangkatan" value={kloterId} onChange={(e) => setKloterId(e.target.value)}>
              <option value="">Semua Kloter</option>
              {kloters.map((k) => <option key={k.id} value={k.id}>{k.code}</option>)}
            </select>
          </>
        }
      />

      <div className="dashboard-home">
        <ProfileShareBanner />
        {error && <p role="alert" className="dashboard-error-banner">{error}</p>}

        <div className="dashboard-stats-grid tw-stagger">
          {stats ? (
            <>
              <StatCard label="Total Jamaah" value={stats.total} unit="jamaah" tone="brand" />
              <StatCard label="Tersubstitusi" value={stats.substituted} unit="jamaah" tone="danger" />
              <StatCard label="Butuh Kursi Roda" value={stats.requiresWheelchair} unit="jamaah" tone="info" />
              <StatCard label="Belum Ada Grup" value={stats.unassignedGroup} unit="jamaah" tone="warning" />
              <StatCard label="Belum Ada Kloter" value={stats.unassignedKloter} unit="jamaah" tone="warning" />
            </>
          ) : (
            Array.from({ length: 5 }, (_, index) => <div key={index} className="dashboard-skeleton-card" />)
          )}
        </div>

        <section className="dashboard-home-section">
          <p className="section-eyebrow">Aksi Cepat</p>
          <h2 className="dashboard-section-title">Kelola Operasional</h2>
          <p className="dashboard-section-subtitle">Tindakan umum untuk operasional harian</p>
          <div className="dashboard-quick-grid tw-stagger">
            {QUICK_ACTIONS.map(({ icon: Icon, title, desc, href }) => (
              <Link key={title} href={href} className="tw-card tw-card--large tw-enter dashboard-quick-card">
                <div className="dashboard-quick-icon"><Icon size={18} aria-hidden /></div>
                <p className="dashboard-quick-title">{title}</p>
                <p className="dashboard-quick-description">{desc}</p>
                <span className="dashboard-quick-arrow">Mulai <IconArrowRight size={12} aria-hidden /></span>
              </Link>
            ))}
          </div>
        </section>

        <section>
          <div className="tw-card tw-card--large tw-enter dashboard-activity">
            <div className="dashboard-activity-head">
              <p className="dashboard-activity-title">Aktivitas Terbaru</p>
            </div>
            <div className="tw-stagger">
              {activity.length ? activity.map((log) => {
                const Icon = ACTIVITY_ICON[log.entityType] ?? IconUser;
                return (
                  <div key={log.id} className="dashboard-activity-row tw-enter">
                    <div className="dashboard-activity-icon"><Icon size={14} aria-hidden /></div>
                    <p className="dashboard-activity-text">{log.description}{log.actorName ? <span className="dashboard-activity-actor"> · oleh {log.actorName}</span> : null}</p>
                    <p className="dashboard-activity-time">{log.createdAt ? relativeTime(log.createdAt) : ""}</p>
                  </div>
                );
              }) : (
                <p className="dashboard-activity-empty">Belum ada aktivitas tercatat.</p>
              )}
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
