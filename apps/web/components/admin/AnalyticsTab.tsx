"use client";

import { useEffect, useMemo, useState } from "react";
import { IconAlertTriangle } from "@tabler/icons-react";
import type { GetPlatformAnalyticsResponse } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { MethodologyNote } from "@/components/ui/MethodologyNote";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;
const count = (n: number) => new Intl.NumberFormat("id-ID").format(n);
const PERIODS: [number, string][] = [[30, "30 hari"], [90, "90 hari"], [365, "1 tahun"]];

export default function AnalyticsTab() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<GetPlatformAnalyticsResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    setLoading(true);
    setFailure("");
    platformClient
      .getPlatformAnalytics({ days })
      .then(setData)
      .catch(() => setFailure("Gagal memuat analitik."))
      .finally(() => setLoading(false));
  }, [days]);

  const nrr = useMemo(() => {
    if (!data || data.mrrAtWindowStart <= 0n) return undefined;
    const start = Number(data.mrrAtWindowStart);
    const retained = start + Number(data.expansionMrrIdr) - Number(data.contractionMrrIdr) - Number(data.churnedMrrIdr);
    return (retained / start) * 100;
  }, [data]);

  if (loading) return <p className="admin-note">Memuat analitik…</p>;
  if (failure) return <p className="admin-inline-alert" data-tone="danger"><IconAlertTriangle size={16} />{failure}</p>;
  if (!data) return null;

  const trialRate = data.trialsStarted > 0 ? (data.trialsConverted / data.trialsStarted) * 100 : undefined;
  const net = Number(data.newMrrIdr) + Number(data.expansionMrrIdr) - Number(data.contractionMrrIdr) - Number(data.churnedMrrIdr);
  const scale = Math.max(Number(data.mrrIdr), Number(data.mrrAtWindowStart), 1);

  // The order is the story: where it started, what came in, what left, where it
  // ended. Colour follows the fixed categorical order from the design rules, so
  // a tone means the same thing on every screen.
  const flow: [string, number, string][] = [
    ["Awal periode", Number(data.mrrAtWindowStart), "neutral"],
    ["Travel baru", Number(data.newMrrIdr), "success"],
    ["Naik paket", Number(data.expansionMrrIdr), "info"],
    ["Turun paket", Number(data.contractionMrrIdr), "warning"],
    ["Berhenti", Number(data.churnedMrrIdr), "danger"],
    ["Sekarang", Number(data.mrrIdr), "brand"],
  ];

  return (
    <section className="admin-tab">
      <PageHeader
        eyebrow="TAWAFIQHUB / ANALITIK"
        title="Analitik"
        subtitle={`${count(data.payingTenants)} travel membayar · ${count(data.trialingTenants)} sedang trial · ${count(data.churnedTenants)} berhenti dalam ${data.days} hari`}
        controls={
          <div className="admin-segmented" role="group" aria-label="Rentang waktu">
            {PERIODS.map(([value, label]) => (
              <button key={value} type="button" onClick={() => setDays(value)} aria-pressed={days === value}
                className={days === value ? "admin-segmented__item is-active" : "admin-segmented__item"}>
                {label}
              </button>
            ))}
          </div>
        }
      />

      <div className="admin-stat-grid tw-stagger">
        <StatCard
          label="MRR" value={rupiah(data.mrrIdr)} unit="per bulan" tone="brand"
          delta={{
            value: `${net >= 0 ? "+" : "−"}${rupiah(Math.abs(net))}`,
            label: `pergerakan ${data.days} hari`,
            direction: net > 0 ? "up" : net < 0 ? "down" : "neutral",
          }}
        />
        <StatCard label="Travel membayar" value={count(data.payingTenants)} unit="travel" tone="success" />
        <StatCard
          label="NRR" value={nrr === undefined ? "—" : `${nrr.toFixed(1)}%`} unit=""
          tone={nrr === undefined ? "neutral" : nrr < 100 ? "warning" : "success"}
          delta={nrr === undefined
            ? { value: "belum ada dasar hitungan", direction: "neutral" }
            : { value: nrr < 100 ? "ekspansi belum menutup churn" : "ekspansi menutup churn", direction: nrr < 100 ? "down" : "up" }}
        />
        <StatCard
          label="Konversi trial" value={trialRate === undefined ? "—" : `${trialRate.toFixed(0)}%`} unit="" tone="info"
          delta={{ value: `${count(data.trialsConverted)} dari ${count(data.trialsStarted)}`, label: "yang mulai di periode ini", direction: "neutral" }}
        />
      </div>

      <section className="tw-card admin-panel tw-enter" aria-labelledby="analytics-flow-title">
        <h2 id="analytics-flow-title" className="admin-panel__title">Pergerakan MRR</h2>
        <p className="admin-panel__lede">
          Sumbu horizontal adalah rupiah per bulan, diskalakan terhadap MRR terbesar antara awal dan akhir periode.
          Panjang bilah bisa dibandingkan antarbaris.
        </p>
        <div className="admin-flow">
          {flow.map(([label, amount, tone]) => (
            <div key={label} className="admin-flow__row">
              <div className="admin-flow__head">
                <span>{label}</span>
                <span className="admin-flow__value">{rupiah(amount)}</span>
              </div>
              <div className="admin-flow__track">
                <div className="admin-flow__fill" data-tone={tone} style={{ width: `${Math.min((amount / scale) * 100, 100)}%` }} />
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="tw-card admin-panel tw-enter" aria-labelledby="analytics-plan-title">
        <h2 id="analytics-plan-title" className="admin-panel__title">Per paket</h2>
        <p className="admin-panel__lede">
          Trial dan yang masa aktifnya habis dipisahkan, karena keduanya belum membayar.
        </p>
        <div className="admin-table-wrap">
          <table className="admin-table">
            <thead>
              <tr>
                <th>Paket</th>
                <th className="is-numeric">Harga/bulan</th>
                <th className="is-numeric">Membayar</th>
                <th className="is-numeric">MRR</th>
                <th className="is-numeric">Trial</th>
                <th className="is-numeric">Habis masa</th>
              </tr>
            </thead>
            <tbody>
              {data.byPlan.map((plan) => (
                <tr key={plan.plan} data-state={plan.lapsedTenants > 0 ? "attention" : undefined}>
                  <td style={{ fontWeight: 700 }}>{plan.plan}</td>
                  <td className="is-numeric">{rupiah(plan.monthlyIdr)}</td>
                  <td className="is-numeric">{count(plan.payingTenants)}</td>
                  <td className="is-numeric" style={{ fontWeight: 700 }}>{rupiah(plan.mrrIdr)}</td>
                  <td className="is-numeric">{count(plan.trialTenants)}</td>
                  <td className="is-numeric">
                    {plan.lapsedTenants > 0
                      ? <strong style={{ color: "var(--color-warning-700)" }}>{count(plan.lapsedTenants)}</strong>
                      : "0"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <div className="admin-grid-3 tw-stagger">
        <StatCard label="Sedang trial" value={count(data.trialingTenants)} unit="belum menyumbang MRR" tone="info" />
        <StatCard label="Ditangguhkan" value={count(data.suspendedTenants)} unit="dihentikan sengaja"
          tone={data.suspendedTenants > 0 ? "warning" : "neutral"} />
        <StatCard label="Habis masa" value={count(data.lapsedTenants)} unit="tidak dibatalkan, tidak bisa masuk"
          tone={data.lapsedTenants > 0 ? "danger" : "neutral"} />
      </div>

      <MethodologyNote
        summary={<p style={{ margin: 0 }}>
          Angka di layar ini dihitung dari baris yang hidup, bukan dari potret bulanan. Lima batasannya disebut di sini
          supaya tidak ditemukan lewat keputusan yang terlanjur diambil.
        </p>}
        points={[
          <><strong>Komisi marketplace bukan MRR.</strong> Ia pendapatan lain dan sengaja tidak ada di sini —
            mencampurnya membuat pertumbuhan terlihat lebih cepat daripada sebenarnya.</>,
          <>MRR hanya menghitung travel yang <strong>benar-benar bisa memakai produknya</strong>: bukan trial, tidak
            dibatalkan, tidak ditangguhkan, masa bayarnya belum habis. Definisinya sama persis dengan pemeriksaan akses
            yang menentukan mereka bisa masuk atau tidak.</>,
          <><strong>NRR di bawah 100% berarti ekspansi tidak menutup churn.</strong> MRR awal periode direkonstruksi
            dari pergerakan, bukan diingat — akibatnya travel yang naik paket lalu berhenti di periode yang sama muncul
            di churn, bukan di ekspansi.</>,
          <>Konversi trial diukur pada <strong>rombongan yang mulai di periode ini</strong>, jadi angkanya baru
            mengendap setelah periodenya lewat.</>,
          <>Tidak ada skor risiko churn di sini. Kalau nanti ada, ia <strong>heuristik</strong> — penanda prioritas,
            bukan vonis tentang seorang pelanggan.</>,
        ]}
      />
    </section>
  );
}
