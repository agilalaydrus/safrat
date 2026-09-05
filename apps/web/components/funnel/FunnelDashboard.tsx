"use client";

import { useEffect, useMemo, useState } from "react";
import { IconClockHour4, IconInfoCircle, IconMapPin, IconTrendingUp, IconUsers } from "@tabler/icons-react";
import type { FunnelReport } from "@hajj-saas/proto-gen/hajj/v1/funnel_report_pb";
import { funnelReportClient } from "@/lib/rpc";

// The funnel in the order people walk it. Steps missing from the response are
// shown as zero rather than hidden: a step nobody reached is the most useful
// row on the screen, and dropping it would hide exactly where people stop.
//
// The labels say what is actually recorded, not what the step is called. There
// is no separate catalogue page — packages sit on the storefront itself — so
// KATALOG is recorded when somebody opens one journey's registration page.
// Naming it "melihat paket" would describe a page that does not exist.
const STEPS: [string, string][] = [
  ["LANDING", "Membuka halaman depan"],
  ["ARTIKEL", "Membaca artikel"],
  ["KATALOG", "Membuka halaman pendaftaran"],
  ["MULAI_ISI", "Mulai mengisi formulir"],
  ["KIRIM", "Menekan tombol kirim"],
  ["SELESAI", "Pendaftaran diterima"],
];

const PERIODS: [number, string][] = [[7, "7 hari"], [30, "30 hari"], [90, "90 hari"]];
const number = (n: number) => new Intl.NumberFormat("id-ID").format(n);

export default function FunnelDashboard() {
  const [days, setDays] = useState(30);
  const [report, setReport] = useState<FunnelReport>();
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    setLoading(true);
    setNotice("");
    funnelReportClient
      .getFunnelReport({ days })
      .then(setReport)
      .catch(() => setNotice("Gagal memuat data corong pengunjung."))
      .finally(() => setLoading(false));
  }, [days]);

  const steps = useMemo(() => {
    const found = new Map(report?.steps.map((step) => [step.step, step.visitors]) ?? []);
    return STEPS.map(([id, label]) => ({ id, label, visitors: found.get(id) ?? 0 }));
  }, [report]);

  // Bars are measured against the front page rather than against the largest
  // number anywhere. An article is its own front door — somebody arriving from
  // a search engine straight onto an article never opens "/" — so ARTIKEL can
  // legitimately exceed LANDING. The bar is clamped at full width and the note
  // at the bottom says why, rather than rescaling everything to hide it.
  const entry = steps[0]?.visitors ?? 0;
  const finished = steps[steps.length - 1]?.visitors ?? 0;
  const conversion = entry > 0 ? (finished / entry) * 100 : 0;

  const peakHour = useMemo(() => {
    if (!report?.hours.length) return undefined;
    return report.hours.reduce((best, hour) => (hour.visitors > best.visitors ? hour : best));
  }, [report]);
  const hourMax = Math.max(1, ...(report?.hours.map((hour) => hour.visitors) ?? [1]));
  const placeMax = Math.max(1, ...(report?.places.map((place) => place.visitors) ?? [1]));
  const dailyMax = Math.max(1, ...(report?.daily.map((day) => day.visitors) ?? [1]));

  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>PEMASARAN / CORONG PENGUNJUNG</p>
          <h1 style={title}>Corong Pengunjung</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0 }}>
            {number(entry)} pembuka halaman depan · {number(finished)} pendaftaran diterima ·{" "}
            {conversion.toFixed(1)}% dari pembuka halaman depan
          </p>
        </div>
        <div style={periodBar} role="group" aria-label="Rentang waktu">
          {PERIODS.map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => setDays(value)}
              aria-pressed={days === value}
              style={days === value ? periodActive : periodInactive}
            >
              {label}
            </button>
          ))}
        </div>
      </header>
      <div className="gold-divider" />

      {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
      {loading && <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>}

      {!loading && report && (
        <>
          {entry === 0 && (
            <p style={emptyNotice}>
              Belum ada kunjungan tercatat pada rentang ini. Angka mulai terkumpul setelah halaman storefront Anda
              dibuka orang, dan ringkasan hariannya disusun tiap malam.
            </p>
          )}

          <section style={{ ...card, marginTop: 24 }}>
            <h3 style={cardTitle}>Perjalanan Pengunjung</h3>
            <p style={cardSubtitle}>
              Panjang bilah dihitung terhadap jumlah pembuka halaman depan. Persentase di kanan adalah bagian yang
              tersisa dari langkah pertama, bukan dari langkah sebelumnya. Artikel bisa melebihi 100% karena artikel
              adalah pintu masuk tersendiri: orang yang datang dari mesin pencari langsung ke artikel tidak pernah
              membuka halaman depan.
            </p>
            <div style={{ display: "grid", gap: 12 }}>
              {steps.map((step, index) => {
                const share = entry > 0 ? (step.visitors / entry) * 100 : 0;
                // Steps deepen in colour as they approach a registration, so the
                // eye follows the journey instead of reading six identical bars.
                const shade = ["var(--color-gold-400)", "var(--color-gold-500)", "var(--color-gold-600)",
                  "var(--color-emerald-800)", "var(--color-emerald-900)", "var(--color-emerald-950)"][index];
                return (
                  <div key={step.id}>
                    <div style={rowHead}>
                      <span>{step.label}</span>
                      <span style={{ fontWeight: 700 }}>
                        {number(step.visitors)} orang <span style={{ color: "var(--color-warm-400)", fontWeight: 500 }}>· {share.toFixed(1)}%</span>
                      </span>
                    </div>
                    <div style={track}>
                      <div style={{ width: `${Math.min(share, 100)}%`, height: "100%", background: shade, borderRadius: 5, transition: "width .3s ease" }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          <section style={{ ...card, marginTop: 20 }}>
            <h3 style={cardTitle}><IconTrendingUp size={16} style={icon} />Tren Harian</h3>
            <p style={cardSubtitle}>
              Pengunjung halaman depan per hari, dengan jumlah pendaftaran yang masuk pada hari yang sama. Hari
              berjalan belum muncul: ringkasan hariannya baru disusun setelah hari itu berakhir.
            </p>
            {report.daily.length === 0 ? (
              <p style={emptyRow}>Belum ada hari yang terkumpul pada rentang ini.</p>
            ) : (
              <>
                <div style={trendChart}>
                  {report.daily.map((day) => {
                    const height = (day.visitors / dailyMax) * 100;
                    return (
                      <div
                        key={day.day}
                        style={trendColumn}
                        title={`${day.day} · ${number(day.visitors)} pengunjung · ${number(day.registrations)} pendaftar`}
                      >
                        {day.registrations > 0 && <span style={trendDot} />}
                        <div
                          style={{
                            height: `${Math.max(height, day.visitors > 0 ? 3 : 1)}%`,
                            background: day.registrations > 0 ? "var(--color-emerald-800)" : "var(--color-cream-400)",
                            borderRadius: "3px 3px 0 0",
                            transition: "height .3s ease",
                          }}
                        />
                      </div>
                    );
                  })}
                </div>
                <div style={trendAxis}>
                  <span>{report.daily[0]?.day}</span>
                  <span>{report.daily[report.daily.length - 1]?.day}</span>
                </div>
                <div style={{ display: "flex", gap: 16, marginTop: 10, fontSize: 12, color: "var(--color-warm-500)" }}>
                  <Legend color="var(--color-emerald-800)" label="Hari dengan pendaftaran" />
                  <Legend color="var(--color-cream-400)" label="Hari tanpa pendaftaran" />
                </div>
              </>
            )}
          </section>

          <section style={{ ...card, marginTop: 20 }}>
            <h3 style={cardTitle}>Kanal Kedatangan</h3>
            <p style={cardSubtitle}>
              Diurutkan menurut jumlah pendaftar, bukan jumlah penonton. Kanal dengan seribu penonton dan nol
              pendaftar bukan kanal yang bagus, dan mengurutkannya menurut penonton akan menaruhnya di puncak.
            </p>
            {report.sources.length === 0 ? (
              <p style={emptyRow}>Belum ada kanal tercatat.</p>
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table style={table}>
                  <thead>
                    <tr>{["Kanal", "Penonton", "Pendaftar", "Rasio", "Usia rata-rata pendaftar"].map((head) => <th key={head} style={th}>{head}</th>)}</tr>
                  </thead>
                  <tbody>
                    {report.sources.map((source) => {
                      const rate = source.visitors > 0 ? (source.registrations / source.visitors) * 100 : 0;
                      const age = report.channelAges.find((entry) => entry.source === source.source);
                      return (
                        <tr key={source.source} style={{ borderTop: "1px solid var(--color-cream-300)" }}>
                          <td style={{ ...td, fontWeight: 600 }}>{source.source}</td>
                          <td style={td}>{number(source.visitors)}</td>
                          <td style={{ ...td, fontWeight: 700, color: source.registrations > 0 ? "var(--color-emerald-800)" : "var(--color-warm-400)" }}>
                            {number(source.registrations)}
                          </td>
                          <td style={td}>{source.visitors > 0 ? `${rate.toFixed(1)}%` : "—"}</td>
                          <td style={td}>
                            {age ? `${age.averageAge.toFixed(0)} tahun` : "—"}
                            {age && <span style={{ color: "var(--color-warm-400)" }}> ({number(age.sample)} pendaftar)</span>}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          <div style={twoCol}>
            <section style={card}>
              <h3 style={cardTitle}><IconClockHour4 size={16} style={icon} />Jam Aktif</h3>
              <p style={cardSubtitle}>
                Waktu Indonesia Barat. {peakHour
                  ? `Paling ramai pukul ${String(peakHour.hour).padStart(2, "0")}.00 — jam terbaik untuk membalas pesan dan menayangkan iklan.`
                  : "Belum ada kunjungan tercatat."}
              </p>
              <div style={hourGrid}>
                {Array.from({ length: 24 }, (_, hour) => {
                  const visitors = report.hours.find((row) => row.hour === hour)?.visitors ?? 0;
                  const intensity = visitors / hourMax;
                  return (
                    <div key={hour} style={hourColumn}>
                      <div
                        style={{
                          height: `${Math.max(intensity * 72, visitors > 0 ? 4 : 2)}px`,
                          background: visitors === 0 ? "var(--color-cream-300)" : `color-mix(in srgb, var(--color-emerald-800) ${25 + intensity * 75}%, var(--color-cream-200))`,
                          borderRadius: 3,
                          transition: "height .3s ease",
                        }}
                        title={`${String(hour).padStart(2, "0")}.00 WIB · ${number(visitors)} pengunjung`}
                      />
                      {hour % 6 === 0 && <span style={hourLabel}>{String(hour).padStart(2, "0")}</span>}
                    </div>
                  );
                })}
              </div>
            </section>

            <section style={card}>
              <h3 style={cardTitle}><IconMapPin size={16} style={icon} />Asal Daerah</h3>
              <p style={cardSubtitle}>Sampai tingkat kota, tidak lebih rinci dari itu.</p>
              {report.places.length === 0 ? (
                <p style={emptyRow}>
                  Belum ada data daerah pada periode ini. Hanya pengunjung baru sejak lokasi diaktifkan yang tercatat —
                  kunjungan sebelum itu tidak ditebak mundur.
                </p>
              ) : (
                <div style={{ display: "grid", gap: 10 }}>
                  {report.places.map((place) => (
                    <div key={`${place.province}-${place.city}`}>
                      <div style={rowHead}>
                        <span>{place.city || "Tidak diketahui"} <span style={{ color: "var(--color-warm-400)" }}>· {place.province}</span></span>
                        <span style={{ fontWeight: 700 }}>{number(place.visitors)}</span>
                      </div>
                      <div style={track}>
                        <div style={{ width: `${(place.visitors / placeMax) * 100}%`, height: "100%", background: "var(--color-gold-500)", borderRadius: 5 }} />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>

          <section style={{ ...card, marginTop: 20 }}>
            <h3 style={cardTitle}><IconUsers size={16} style={icon} />Kinerja Artikel</h3>
            <p style={cardSubtitle}>
              Pembaca adalah orang, bukan jumlah buka halaman. Kolom terakhir menghitung pembaca yang pada hari yang
              sama menyelesaikan pendaftaran.
            </p>
            {report.articles.length === 0 ? (
              <p style={emptyRow}>Belum ada artikel yang dibaca pada rentang ini.</p>
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table style={table}>
                  <thead><tr>{["Artikel", "Pembaca", "Mendaftar di hari yang sama"].map((head) => <th key={head} style={th}>{head}</th>)}</tr></thead>
                  <tbody>
                    {report.articles.map((article) => (
                      <tr key={article.slug} style={{ borderTop: "1px solid var(--color-cream-300)" }}>
                        <td style={{ ...td, fontWeight: 600 }}>{article.slug}</td>
                        <td style={td}>{number(article.readers)}</td>
                        <td style={{ ...td, fontWeight: 700, color: article.registrations > 0 ? "var(--color-emerald-800)" : "var(--color-warm-400)" }}>
                          {number(article.registrations)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          <section style={methodology}>
            <h3 style={{ ...cardTitle, marginBottom: 12, display: "flex", alignItems: "center", gap: 8 }}>
              <IconInfoCircle size={16} />Cara angka ini dihitung
            </h3>
            <ul style={methodList}>
              <li>
                Pengunjung dihitung <strong>sekali per hari</strong>. Satu orang yang membuka sepuluh halaman tetap
                satu pengunjung, dan orang yang sama kembali besok dihitung lagi sebagai pengunjung baru.
              </li>
              <li>
                Tidak ada cookie dan tidak ada pelacakan antarhari. Penanda pengunjung dibuat dari alamat jaringan dan
                perangkat, lalu diganti tiap tengah malam — jadi <strong>penautan lintas hari tidak akurat</strong>:
                orang yang membaca artikel hari Senin lalu mendaftar hari Kamis tidak akan tercatat sebagai pembaca
                yang mendaftar.
              </li>
              <li>
                Karena itu, kanal pada tabel pendaftar diambil dari kanal yang tercatat pada formulir pendaftaran itu
                sendiri, bukan dari kunjungan pertama.
              </li>
              <li>
                Penanda dengan lebih dari 60 kejadian dalam sehari dianggap mesin perayap dan <strong>dibuang
                seluruhnya</strong>, bukan dipangkas.
              </li>
              <li>
                Hari dan jam memakai Waktu Indonesia Barat, bukan UTC.
              </li>
              <li>
                Usia hanya diketahui dari orang yang <strong>sudah mendaftar</strong>. Usia pengunjung tidak ditebak:
                menebaknya berarti menyajikan dugaan sebagai hasil pengukuran.
              </li>
              <li>
                Jam aktif, asal daerah, dan kinerja artikel dibaca dari catatan mentah yang hanya disimpan{" "}
                <strong>{report.rawRetentionDays} hari</strong>. Ringkasan harian — langkah dan kanal — disimpan
                selamanya.
              </li>
            </ul>
          </section>
        </>
      )}
    </main>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{ width: 10, height: 10, borderRadius: 2, background: color, display: "inline-block" }} />
      {label}
    </span>
  );
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "8px 0 32px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,40px)", fontWeight: 500, margin: 0 };
const periodBar: React.CSSProperties = { display: "inline-flex", gap: 4, padding: 4, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const periodBase: React.CSSProperties = { minHeight: 40, padding: "0 16px", border: 0, borderRadius: 7, background: "transparent", font: "inherit", fontSize: 13, fontWeight: 700, color: "var(--color-warm-500)", cursor: "pointer", transition: "background .2s ease, color .2s ease" };
const periodActive: React.CSSProperties = { ...periodBase, background: "white", color: "var(--color-emerald-900)", boxShadow: "0 1px 2px rgba(0,0,0,.06)" };
const periodInactive: React.CSSProperties = periodBase;
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 };
const cardTitle: React.CSSProperties = { margin: "0 0 8px", fontSize: 16, fontWeight: 700 };
const cardSubtitle: React.CSSProperties = { margin: "0 0 20px", fontSize: 13, lineHeight: 1.6, color: "var(--color-warm-500)" };
const twoCol: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(340px,1fr))", gap: 16, marginTop: 20 };
const rowHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13, marginBottom: 5 };
const track: React.CSSProperties = { height: 10, background: "var(--color-cream-300)", borderRadius: 5, overflow: "hidden" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: "10px 12px", fontSize: 11, color: "var(--color-warm-400)", background: "var(--color-cream-100)" };
const td: React.CSSProperties = { padding: "10px 12px", color: "var(--color-warm-700)" };
const icon: React.CSSProperties = { verticalAlign: "-3px", marginRight: 6 };
const trendChart: React.CSSProperties = { display: "flex", alignItems: "flex-end", gap: 2, height: 132, padding: "12px 0 0" };
const trendColumn: React.CSSProperties = { flex: 1, minWidth: 3, height: "100%", display: "flex", flexDirection: "column", justifyContent: "flex-end", position: "relative" };
const trendDot: React.CSSProperties = { position: "absolute", top: -2, left: "50%", transform: "translateX(-50%)", width: 5, height: 5, borderRadius: 99, background: "var(--color-gold-500)" };
const trendAxis: React.CSSProperties = { display: "flex", justifyContent: "space-between", marginTop: 6, fontSize: 11, color: "var(--color-warm-400)" };
const hourGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(24,1fr)", gap: 3, alignItems: "end", minHeight: 96 };
const hourColumn: React.CSSProperties = { display: "flex", flexDirection: "column", justifyContent: "flex-end", gap: 4 };
const hourLabel: React.CSSProperties = { fontSize: 10, color: "var(--color-warm-400)", textAlign: "center" };
const emptyRow: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-warm-400)", lineHeight: 1.6 };
const emptyNotice: React.CSSProperties = { margin: "20px 0 0", padding: "14px 16px", borderRadius: 10, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", fontSize: 13, lineHeight: 1.6, color: "var(--color-warm-700)" };
const methodology: React.CSSProperties = { ...card, marginTop: 20, background: "var(--color-cream-100)" };
const methodList: React.CSSProperties = { margin: 0, paddingLeft: 20, display: "grid", gap: 10, fontSize: 13, lineHeight: 1.65, color: "var(--color-warm-700)" };
