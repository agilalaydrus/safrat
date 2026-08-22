"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  IconAlertTriangle,
  IconArrowRight,
  IconBed,
  IconBrandWhatsapp,
  IconBus,
  IconCheck,
  IconCircleCheck,
  IconCompass,
  IconDeviceMobile,
  IconLock,
  IconMinus,
  IconMoonStars,
  IconPlus,
  IconShoppingCart,
  IconSos,
  IconUserDollar,
  IconUsers,
  IconUsersGroup,
  IconX,
} from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";

/* ------------------------------------------------------------------ */
/* Content — grounded in features that actually exist in this repo.   */
/* Nothing here claims an integration, certification, or number that  */
/* isn't backed by real code. Estimates are labeled as estimates.     */
/* ------------------------------------------------------------------ */

const NAV_LINKS = [
  ["Fitur", "#fitur"],
  ["Simulasi Validasi", "#simulasi"],
  ["Kalkulator ROI", "#roi"],
  ["Testimoni", "#testimoni"],
  ["FAQ", "#faq"],
] as const;

const TRUST_BADGES = [
  { icon: IconUsersGroup, label: "Validasi Mahram & Gender Otomatis" },
  { icon: IconSos, label: "Eskalasi SOS 10 Menit, Bukan Janji" },
  { icon: IconLock, label: "Sesi Login Terenkripsi, Terisolasi per Operator" },
  { icon: IconDeviceMobile, label: "Aplikasi Muttawwif & Jamaah via Browser" },
] as const;

const PROBLEM_SOLUTION = [
  {
    problem: "Rekap Excel manual, lembur H-1 keberangkatan",
    solution: "Satu database live — staf, ketua grup, dan jamaah melihat data yang sama, kapan saja",
  },
  {
    problem: "Jamaah non-mahram bisa kesasar sekamar",
    solution: "Alokasi kamar tervalidasi otomatis: kapasitas, gender, dan aturan mahram",
  },
  {
    problem: "Drama kursi bus & manifest kertas yang mudah hilang",
    solution: "Kursi per jamaah tercatat, manifest bisa dibuka dari HP Muttawwif",
  },
  {
    problem: "Panik saat jamaah terpisah atau darurat di Tanah Suci",
    solution: "SOS satu tombol dari jamaah, eskalasi otomatis ke koordinator kalau tak direspons",
  },
] as const;

const FEATURE_MODULES = [
  { icon: IconUsers, title: "Manajemen Jamaah & Manifest", desc: "Daftar satu per satu atau impor CSV massal. Data paspor, dokumen, dan status per jamaah dalam satu tempat." },
  { icon: IconMoonStars, title: "Kloter & Kalender Hijriah", desc: "Kelola musim per periode Hijriah, dari Rajab sampai musim Haji itu sendiri." },
  { icon: IconBed, title: "Rooming List & Validasi Mahram", desc: "Alokasi kamar hotel dengan validasi kapasitas dan aturan gender/mahram otomatis." },
  { icon: IconBus, title: "Armada Bus & Manifest Lapangan", desc: "Jadwalkan pergerakan kendaraan, tetapkan kursi per jamaah, pantau manifest secara real-time." },
  { icon: IconSos, title: "Protokol SOS 10 Menit", desc: "Satu tombol untuk jamaah. Belum direspons dalam 10 menit, otomatis dieskalasi ke seluruh koordinator." },
  { icon: IconShoppingCart, title: "Paket Perjalanan & Produk Digital", desc: "Susun itinerary harian, hotel, dan kloter default per paket yang dijual, plus add-on seperti eSIM." },
  { icon: IconUserDollar, title: "Komisi Agen & Mitra", desc: "Lacak agen rujukan, hitung komisi otomatis per transaksi, cairkan lewat dompet digital mereka." },
  { icon: IconDeviceMobile, title: "Aplikasi Muttawwif & Jamaah", desc: "PWA ringan untuk chat, absen kendaraan/hotel, dan SOS — langsung dari browser tanpa unduh." },
] as const;

const FAQ_ITEMS = [
  {
    q: "Bisa impor data jamaah dari Excel yang sudah ada?",
    a: "Bisa. Modul Jamaah menerima impor CSV massal, jadi data yang sudah ada tidak perlu diketik ulang satu per satu.",
  },
  {
    q: "Apakah formatnya sesuai kebutuhan pelaporan Kemenag/Siskopatuh?",
    a: "Tawafiq Hub dibangun mengikuti istilah dan alur kerja operasional PPIU/PIHK sehari-hari — Grup, Kloter, Muttawwif — bukan istilah SaaS generik. Untuk kebutuhan pelaporan resmi, data jamaah dan kloter bisa diekspor dan disesuaikan formatnya. Kami tidak mengklaim integrasi otomatis ke Siskopatuh saat ini.",
  },
  {
    q: "Bagaimana kalau sinyal terbatas atau roaming saat di Saudi?",
    a: "Halaman yang sudah pernah dibuka tetap bisa diakses offline. Aksi penting seperti SOS, check-in, dan chat disimpan di perangkat dan otomatis terkirim ulang begitu sinyal kembali — bukan jaminan zero-network penuh, tapi cukup untuk kondisi roaming yang naik-turun.",
  },
  {
    q: "Seberapa aman data jamaah kami?",
    a: "Autentikasi lewat sesi terenkripsi (Better Auth), dan setiap operator terisolasi — data satu travel tidak bisa diakses travel lain lewat query yang sama. Seluruh trafik berjalan lewat koneksi terenkripsi (HTTPS/TLS).",
  },
] as const;

const MAHRAM_SCENARIOS = [
  { id: "suami-istri", label: "Suami – Istri", genderA: "L", genderB: "P", isMahram: true },
  { id: "non-mahram", label: "Pria – Wanita (Non-Mahram)", genderA: "L", genderB: "P", isMahram: false },
  { id: "wanita-wanita", label: "Wanita – Wanita", genderA: "P", genderB: "P", isMahram: true },
] as const;

/* ------------------------------------------------------------------ */

export default function LandingPage() {
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);
  const dashboardHref = "/dashboard";
  const [modalOpen, setModalOpen] = useState(false);

  return (
    <div style={page}>
      <nav style={nav}>
        <Link href="/" aria-label="Tawafiq Hub home" style={navLogo}>
          <span style={navLogoIcon}><IconCompass size={20} stroke={2} /></span>
          Tawafiq Hub
        </Link>
        <div style={navCenter}>
          {NAV_LINKS.map(([label, href]) => (
            <a key={href} href={href} style={navLink}>{label}</a>
          ))}
        </div>
        <div style={navLinks}>
          <span style={quickStatPill}>Musim 1447H Ready</span>
          {isAuthenticated ? (
            <Link href={dashboardHref} style={navCta}>Dashboard</Link>
          ) : !isPending && (
            <>
              <button type="button" onClick={() => setModalOpen(true)} style={navGhostBtn}>Jadwalkan Konsultasi</button>
              <Link href="/sign-up" style={navCta}>Coba Live Demo</Link>
            </>
          )}
        </div>
      </nav>

      <section style={hero}>
        <div style={heroInner}>
          <p style={eyebrow}>SISTEM OPERASI PPIU &amp; HAJI KHUSUS</p>
          <h1 style={heroTitle}>Tinggalkan Kerepotan Spreadsheet.<br />Kelola Operasional dengan Tenang.</h1>
          <p style={heroSub}>
            Dari pendaftaran jamaah, rooming list hotel bebas sengketa mahram, manajemen armada bus,
            sampai gateway SOS darurat — satu dashboard yang dipakai tim Anda tiap hari, bukan cuma waktu demo.
          </p>
          <div style={ctaRow}>
            {isAuthenticated ? (
              <Link href={dashboardHref} style={heroCta}>Buka dashboard<IconArrowRight size={18} /></Link>
            ) : !isPending && (
              <>
                <Link href="/sign-up" style={heroCta}>Eksplorasi Demo Dashboard<IconArrowRight size={18} /></Link>
                <a href="#roi" style={heroOutline}>Hitung Penghematan Musim</a>
              </>
            )}
          </div>
          <div style={badgeRow}>
            {TRUST_BADGES.map(({ icon: Icon, label }) => (
              <span key={label} style={badge}><Icon size={14} />{label}</span>
            ))}
          </div>
        </div>
      </section>

      <section id="fitur-preview" style={section}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Fitur Unggulan</p>
          <h2 style={sectionTitle}>Coba Langsung, Bukan Sekadar Screenshot</h2>
          <p style={sectionSub}>Empat alur kerja yang paling sering dipakai operator tiap musim. Klik tab untuk melihat gambarannya.</p>
          <DashboardPreview />
        </div>
      </section>

      <section style={sectionAlt}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Sebelum vs Sesudah</p>
          <h2 style={sectionTitle}>Yang Bikin Musim Kemarin Pusing</h2>
          <div style={compareGrid}>
            <div style={compareCol}>
              <p style={compareHead}><IconAlertTriangle size={16} />Cara Lama</p>
              {PROBLEM_SOLUTION.map((row) => (
                <div key={row.problem} style={compareCardProblem}>{row.problem}</div>
              ))}
            </div>
            <div style={compareCol}>
              <p style={{ ...compareHead, color: COLOR.emerald700 }}><IconCircleCheck size={16} />Dengan Tawafiq Hub</p>
              {PROBLEM_SOLUTION.map((row) => (
                <div key={row.solution} style={compareCardSolution}>{row.solution}</div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section id="fitur" style={section}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Modul Fitur Terpadu</p>
          <h2 style={sectionTitle}>Semua Modul yang Operator Beneran Butuh</h2>
          <p style={sectionSub}>Bukan daftar fitur generik. Ini modul yang dipakai dari pendaftaran jamaah pertama sampai jamaah pulang.</p>
          <div style={grid}>
            {FEATURE_MODULES.map(({ icon: Icon, title, desc }) => (
              <div key={title} style={featureCard}>
                <div style={featureIcon}><Icon size={20} color="#fff" /></div>
                <h3 style={featureTitle}>{title}</h3>
                <p style={featureDesc}>{desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="simulasi" style={sectionAlt}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Live Simulator</p>
          <h2 style={sectionTitle}>Coba Logikanya Sendiri</h2>
          <p style={sectionSub}>Dua aturan yang paling sering jadi sumber masalah operasional — coba langsung di sini.</p>
          <div style={simGrid}>
            <MahramSimulator />
            <SosSimulator />
          </div>
        </div>
      </section>

      <section id="roi" style={section}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Kalkulator ROI</p>
          <h2 style={sectionTitle}>Estimasi Penghematan Musim Anda</h2>
          <p style={sectionSub}>Angka di bawah ini estimasi berdasarkan asumsi yang bisa Anda lihat sendiri, bukan janji pasti.</p>
          <RoiCalculator />
        </div>
      </section>

      <section id="testimoni" style={sectionAlt}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>Testimoni</p>
          <h2 style={sectionTitle}>Contoh Skenario Penggunaan</h2>
          <p style={sectionSub}>
            Tawafiq Hub baru memasuki musim pertamanya. Tiga kartu di bawah ini adalah <strong>contoh ilustratif</strong> — bukan
            kutipan klien nyata — untuk menggambarkan siapa yang biasanya memakai tiap modul. Akan diganti dengan testimoni asli
            setelah musim pertama berjalan.
          </p>
          <div style={grid}>
            {ILLUSTRATIVE_TESTIMONIALS.map((t) => (
              <div key={t.name} style={testimonialCard}>
                <span style={illustrativeTag}>Contoh Ilustrasi</span>
                <p style={testimonialQuote}>&ldquo;{t.quote}&rdquo;</p>
                <p style={testimonialName}>{t.name}</p>
                <p style={testimonialRole}>{t.role}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="faq" style={section}>
        <div style={sectionInner}>
          <p style={sectionEyebrow}>FAQ</p>
          <h2 style={sectionTitle}>Pertanyaan yang Sering Muncul</h2>
          <FaqAccordion />
        </div>
      </section>

      <section style={banner}>
        <div style={bannerInner}>
          <p style={{ ...sectionEyebrow, color: "#A7F3D0" }}>
            {isAuthenticated ? "Selamat datang kembali" : "Siap Musim 1447H / 1448H Berikutnya?"}
          </p>
          <h2 style={bannerTitle}>Musim Berikutnya,<br />Kelola Lebih Rapi.</h2>
          <p style={bannerText}>Buat musim pertama Anda dalam hitungan menit, gratis untuk mulai.</p>
          <div style={{ display: "flex", gap: 12, justifyContent: "center", flexWrap: "wrap" }}>
            <Link href={isAuthenticated ? dashboardHref : "/sign-up"} style={heroCta}>
              {isAuthenticated ? "Buka dashboard" : "Buat Musim Pertama Anda"}<IconArrowRight size={18} />
            </Link>
            {!isAuthenticated && (
              <button type="button" onClick={() => setModalOpen(true)} style={bannerOutline}>Jadwalkan Demo &amp; Setup</button>
            )}
          </div>
        </div>
      </section>

      <footer style={footer}>
        <div style={footerInner}>
          <div>
            <Link href="/" aria-label="Tawafiq Hub home" style={navLogo}>
              <span style={navLogoIcon}><IconCompass size={20} stroke={2} /></span>
              Tawafiq Hub
            </Link>
            <p style={footerTag}>Platform operator Haji &amp; Umrah terpadu.</p>
          </div>
          <div style={footerCol}>
            <p style={footerColTitle}>Modul</p>
            {FEATURE_MODULES.slice(0, 5).map((f) => <span key={f.title} style={footerLink}>{f.title}</span>)}
          </div>
          <div style={footerCol}>
            <p style={footerColTitle}>Kontak</p>
            <span style={footerLink}>Jadwalkan konsultasi lewat tombol di atas</span>
            <span style={footerLink}>Kantor &amp; layanan lapangan: segera diumumkan</span>
          </div>
        </div>
        <p style={footerCopyright}>© 2026 Tawafiq Hub. Hak cipta dilindungi.</p>
      </footer>

      {modalOpen && <EnterpriseModal onClose={() => setModalOpen(false)} />}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Interactive dashboard preview                                      */
/* ------------------------------------------------------------------ */

const DASHBOARD_TABS = ["rooming", "armada", "sos", "substitusi"] as const;
type DashboardTabId = (typeof DASHBOARD_TABS)[number];

const DASHBOARD_TAB_LABEL: Record<DashboardTabId, string> = {
  rooming: "Alokasi Kamar & Mahram",
  armada: "Armada Bus & Muthawwif",
  sos: "Gateway Darurat 10-Menit",
  substitusi: "Substitusi Jamaah Instan",
};

function DashboardPreview() {
  const [tab, setTab] = useState<DashboardTabId>("rooming");

  return (
    <div style={previewCard}>
      <div style={previewTabBar}>
        {DASHBOARD_TABS.map((id) => (
          <button key={id} type="button" onClick={() => setTab(id)} style={tab === id ? previewTabActive : previewTabInactive}>
            {DASHBOARD_TAB_LABEL[id]}
          </button>
        ))}
      </div>
      <div style={previewBody}>
        {tab === "rooming" && <RoomingPreview />}
        {tab === "armada" && <ArmadaPreview />}
        {tab === "sos" && <SosPreview />}
        {tab === "substitusi" && <SubstitusiPreview />}
      </div>
    </div>
  );
}

function RoomingPreview() {
  const rooms = [
    { code: "MK-QUAD-04", type: "Quad · Makkah", occ: "4/4", names: "Suami & Istri, 2 Mahram", status: "ok" as const },
    { code: "MD-TRIP-11", type: "Triple · Madinah", occ: "2/3", names: "Kosong 1 slot", status: "warn" as const },
    { code: "MK-DBL-02", type: "Double · Makkah", occ: "1/2", names: "Percobaan pasang non-mahram", status: "blocked" as const },
  ];
  return (
    <div>
      <div style={previewRowsGrid}>
        {rooms.map((r) => (
          <div key={r.code} style={previewRoomCard}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={monoBadge}>{r.code}</span>
              <StatusPill status={r.status} okLabel="Tervalidasi" warnLabel="Perlu isi" blockLabel="Diblokir" />
            </div>
            <p style={previewRoomType}>{r.type}</p>
            <p style={previewRoomOcc}>{r.occ} terisi — {r.names}</p>
          </div>
        ))}
      </div>
      <p style={previewFootnote}>Kapasitas dan aturan gender/mahram divalidasi otomatis saat kamar diisi — bukan dicek manual belakangan.</p>
    </div>
  );
}

function ArmadaPreview() {
  const buses = [
    { code: "BUS-JED-01", route: "Jeddah → Madinah", driver: "Abdullah Al-Harbi", phone: "+966 5X XXX XXXX", seats: "42/45" },
    { code: "BUS-MK-07", route: "Madinah → Makkah", driver: "Faisal Al-Otaibi", phone: "+966 5X XXX XXXX", seats: "45/45" },
  ];
  return (
    <div>
      <div style={previewRowsGrid}>
        {buses.map((b) => (
          <div key={b.code} style={previewRoomCard}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={monoBadge}>{b.code}</span>
              <span style={monoBadge}>{b.seats} kursi</span>
            </div>
            <p style={previewRoomType}>{b.route}</p>
            <p style={previewRoomOcc}>Driver: {b.driver} · {b.phone}</p>
          </div>
        ))}
      </div>
      <p style={previewFootnote}>Manifest bisa dibuka Muttawwif langsung dari HP untuk absen naik/turun kendaraan tiap keberangkatan.</p>
    </div>
  );
}

function SosPreview() {
  return (
    <div style={sosPreviewWrap}>
      <div style={sosPreviewLeft}>
        <p style={{ ...previewRoomType, marginBottom: 4 }}>Jamaah menekan tombol SOS</p>
        <div style={escalationTrack}>
          <div style={escalationStep}>
            <span style={{ ...escalationDot, background: COLOR.amber300 }} />
            <div>
              <p style={escalationLevel}>Level 1 — Muttawwif</p>
              <p style={escalationSub}>Notifikasi langsung ke ketua grup</p>
            </div>
          </div>
          <div style={escalationLine} />
          <div style={escalationStep}>
            <span style={{ ...escalationDot, background: COLOR.rose300 }} />
            <div>
              <p style={escalationLevel}>Level 2 — Direksi PPIU</p>
              <p style={escalationSub}>Otomatis, kalau 10 menit tidak direspons</p>
            </div>
          </div>
        </div>
      </div>
      <p style={previewFootnote}>Lihat simulasi lengkap dengan hitung mundur di bagian <a href="#simulasi" style={inlineLink}>Live Simulator</a> di bawah.</p>
    </div>
  );
}

function SubstitusiPreview() {
  return (
    <div style={previewRoomCard}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <span style={monoBadge}>MK-QUAD-04</span>
        <StatusPill status="ok" okLabel="Susunan tetap utuh" warnLabel="" blockLabel="" />
      </div>
      <p style={previewRoomType}>Jamaah A. Rahman (batal) → digantikan Jamaah S. Fitri</p>
      <p style={previewRoomOcc}>Kamar, kursi bus, dan riwayat penempatan tidak berubah — hanya kepesertaan yang diganti, tercatat di log audit.</p>
    </div>
  );
}

function StatusPill({ status, okLabel, warnLabel, blockLabel }: { status: "ok" | "warn" | "blocked"; okLabel: string; warnLabel: string; blockLabel: string }) {
  const styleMap = { ok: pillOk, warn: pillWarn, blocked: pillDanger };
  const labelMap = { ok: okLabel, warn: warnLabel, blocked: blockLabel };
  return <span style={styleMap[status]}>{labelMap[status]}</span>;
}

/* ------------------------------------------------------------------ */
/* Mahram simulator                                                   */
/* ------------------------------------------------------------------ */

function MahramSimulator() {
  const [selected, setSelected] = useState<(typeof MAHRAM_SCENARIOS)[number] | null>(null);

  return (
    <div style={simCard}>
      <h3 style={simTitle}><IconUsersGroup size={18} />Simulator Validasi Mahram</h3>
      <p style={simDesc}>Pilih pasangan jamaah, sistem menilai apakah boleh sekamar sesuai aturan gender &amp; mahram.</p>
      <div style={simButtonRow}>
        {MAHRAM_SCENARIOS.map((s) => (
          <button key={s.id} type="button" onClick={() => setSelected(s)} style={selected?.id === s.id ? simBtnActive : simBtn}>
            {s.label}
          </button>
        ))}
      </div>
      {selected && (
        <div style={selected.isMahram ? simResultOk : simResultBlocked}>
          {selected.isMahram ? <IconCircleCheck size={20} /> : <IconX size={20} />}
          <div>
            <p style={simResultTitle}>{selected.isMahram ? "Valid — Boleh sekamar" : "Diblokir — Bukan mahram"}</p>
            <p style={simResultSub}>
              {selected.isMahram
                ? "Kombinasi gender dan hubungan keluarga memenuhi aturan mahram."
                : "Sistem menolak penempatan sekamar lintas gender tanpa hubungan mahram yang tercatat."}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* SOS countdown simulator                                            */
/* ------------------------------------------------------------------ */

const SOS_DEMO_SECONDS = 10;

function SosSimulator() {
  const [phase, setPhase] = useState<"idle" | "counting" | "escalated">("idle");
  const [secondsLeft, setSecondsLeft] = useState(SOS_DEMO_SECONDS);

  function activate() {
    setPhase("counting");
    setSecondsLeft(SOS_DEMO_SECONDS);
    const timer = window.setInterval(() => {
      setSecondsLeft((prev) => {
        if (prev <= 1) {
          window.clearInterval(timer);
          setPhase("escalated");
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }

  function reset() {
    setPhase("idle");
    setSecondsLeft(SOS_DEMO_SECONDS);
  }

  const pct = Math.round(((SOS_DEMO_SECONDS - secondsLeft) / SOS_DEMO_SECONDS) * 100);

  return (
    <div style={simCard}>
      <h3 style={simTitle}><IconSos size={18} />Simulator Countdown SOS</h3>
      <p style={simDesc}>
        Simulasi dipercepat: {SOS_DEMO_SECONDS} detik di sini mewakili 10 menit asli di sistem — aturan eskalasinya sama persis.
      </p>
      {phase === "idle" && (
        <button type="button" onClick={activate} style={sosActivateBtn}><IconAlertTriangle size={16} />Aktifkan SOS (Simulasi)</button>
      )}
      {phase === "counting" && (
        <div>
          <div style={sosProgressTrack}><div style={{ ...sosProgressFill, width: `${pct}%` }} /></div>
          <p style={sosCountdownText}>Menunggu respons Muttawwif — {secondsLeft} detik lagi sebelum eskalasi</p>
          <button type="button" onClick={reset} style={simBtn}>Reset</button>
        </div>
      )}
      {phase === "escalated" && (
        <div style={simResultBlocked}>
          <IconAlertTriangle size={20} />
          <div>
            <p style={simResultTitle}>Dieskalasi ke Level 2 — Direksi PPIU</p>
            <p style={simResultSub}>10 menit terlewati tanpa respons Muttawwif. Semua koordinator operator kini menerima notifikasi.</p>
            <button type="button" onClick={reset} style={{ ...simBtn, marginTop: 10 }}>Ulangi Simulasi</button>
          </div>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* ROI calculator                                                     */
/* ------------------------------------------------------------------ */

const ADMIN_MINUTES_PER_PILGRIM = 9;
const ADMIN_HOURS_PER_KLOTER_COORD = 2;
const ASSUMED_HOURLY_ADMIN_COST_IDR = 50000;

function RoiCalculator() {
  const [pilgrims, setPilgrims] = useState(500);
  const [kloters, setKloters] = useState(6);

  const { hoursSaved, costSaved } = useMemo(() => {
    const hours = (pilgrims * ADMIN_MINUTES_PER_PILGRIM) / 60 + kloters * ADMIN_HOURS_PER_KLOTER_COORD;
    return { hoursSaved: Math.round(hours), costSaved: Math.round(hours * ASSUMED_HOURLY_ADMIN_COST_IDR) };
  }, [pilgrims, kloters]);

  return (
    <div style={roiCard}>
      <div style={roiSliders}>
        <label style={roiLabel}>
          Jumlah Jamaah per Musim
          <span style={roiSliderValue}>{pilgrims.toLocaleString("id-ID")} jamaah</span>
          <input type="range" min={100} max={5000} step={50} value={pilgrims} onChange={(e) => setPilgrims(Number(e.target.value))} style={roiRange} />
        </label>
        <label style={roiLabel}>
          Jumlah Keberangkatan / Kloter
          <span style={roiSliderValue}>{kloters} kloter</span>
          <input type="range" min={2} max={50} step={1} value={kloters} onChange={(e) => setKloters(Number(e.target.value))} style={roiRange} />
        </label>
      </div>
      <div style={roiResultGrid}>
        <div style={roiResultCard}>
          <p style={roiResultValue}>~{hoursSaved.toLocaleString("id-ID")} jam</p>
          <p style={roiResultLabel}>Estimasi jam kerja admin dihemat / musim</p>
        </div>
        <div style={roiResultCard}>
          <p style={{ ...roiResultValue, color: COLOR.emerald700 }}>Tervalidasi otomatis</p>
          <p style={roiResultLabel}>Sengketa kamar mahram — dicegah oleh sistem, bukan diperkirakan</p>
        </div>
        <div style={roiResultCard}>
          <p style={roiResultValue}>~Rp {costSaved.toLocaleString("id-ID")}</p>
          <p style={roiResultLabel}>Estimasi penghematan biaya admin / musim</p>
        </div>
      </div>
      <p style={previewFootnote}>
        Asumsi: {ADMIN_MINUTES_PER_PILGRIM} menit rekap manual per jamaah, {ADMIN_HOURS_PER_KLOTER_COORD} jam koordinasi per kloter,
        dan biaya waktu admin Rp{ASSUMED_HOURLY_ADMIN_COST_IDR.toLocaleString("id-ID")}/jam. Sesuaikan sendiri untuk operasional Anda —
        ini estimasi, bukan jaminan angka pasti.
      </p>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* FAQ accordion                                                      */
/* ------------------------------------------------------------------ */

function FaqAccordion() {
  const [openIndex, setOpenIndex] = useState<number | null>(0);
  return (
    <div style={faqList}>
      {FAQ_ITEMS.map((item, i) => {
        const open = openIndex === i;
        return (
          <div key={item.q} style={faqItem}>
            <button type="button" onClick={() => setOpenIndex(open ? null : i)} style={faqQuestion} aria-expanded={open}>
              {item.q}
              {open ? <IconMinus size={16} /> : <IconPlus size={16} />}
            </button>
            {open && <p style={faqAnswer}>{item.a}</p>}
          </div>
        );
      })}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Enterprise demo/setup modal                                        */
/* ------------------------------------------------------------------ */

const SALES_WHATSAPP = process.env.NEXT_PUBLIC_SALES_WHATSAPP ?? "";

function EnterpriseModal({ onClose }: { onClose: () => void }) {
  const [travelName, setTravelName] = useState("");
  const [picName, setPicName] = useState("");
  const [whatsapp, setWhatsapp] = useState("");
  const [pilgrimCount, setPilgrimCount] = useState("");
  const [modules, setModules] = useState<string[]>([]);
  const [copied, setCopied] = useState(false);

  function toggleModule(title: string) {
    setModules((prev) => (prev.includes(title) ? prev.filter((m) => m !== title) : [...prev, title]));
  }

  const message = useMemo(() => {
    const lines = [
      "Halo Tawafiq Hub, saya ingin jadwalkan demo & setup.",
      `Nama Travel: ${travelName || "-"}`,
      `PIC: ${picName || "-"}`,
      `WhatsApp: ${whatsapp || "-"}`,
      `Estimasi Jumlah Jamaah: ${pilgrimCount || "-"}`,
      `Modul yang diminati: ${modules.length ? modules.join(", ") : "-"}`,
    ];
    return lines.join("\n");
  }, [travelName, picName, whatsapp, pilgrimCount, modules]);

  async function copyMessage() {
    try {
      await navigator.clipboard.writeText(message);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard permission denied — the textarea below still shows the message to copy manually */
    }
  }

  return (
    <div style={modalOverlay} role="dialog" aria-modal="true" aria-label="Jadwalkan demo dan setup">
      <div style={modalCard}>
        <div style={modalHead}>
          <h3 style={modalTitle}>Jadwalkan Demo &amp; Setup</h3>
          <button type="button" onClick={onClose} style={modalCloseBtn} aria-label="Tutup"><IconX size={20} /></button>
        </div>
        <div style={modalBody}>
          <label style={modalLabel}>Nama Travel<input value={travelName} onChange={(e) => setTravelName(e.target.value)} style={modalInput} /></label>
          <label style={modalLabel}>Nama PIC<input value={picName} onChange={(e) => setPicName(e.target.value)} style={modalInput} /></label>
          <label style={modalLabel}>WhatsApp<input value={whatsapp} onChange={(e) => setWhatsapp(e.target.value)} style={modalInput} placeholder="08xxxxxxxxxx" /></label>
          <label style={modalLabel}>Estimasi Jumlah Jamaah<input value={pilgrimCount} onChange={(e) => setPilgrimCount(e.target.value)} style={modalInput} /></label>
          <fieldset style={modalFieldset}>
            <legend style={modalLabel}>Modul yang diminati</legend>
            <div style={modalChipRow}>
              {FEATURE_MODULES.map((m) => (
                <button
                  key={m.title}
                  type="button"
                  onClick={() => toggleModule(m.title)}
                  style={modules.includes(m.title) ? modalChipActive : modalChip}
                >
                  {modules.includes(m.title) && <IconCheck size={12} />}
                  {m.title}
                </button>
              ))}
            </div>
          </fieldset>
          {SALES_WHATSAPP ? (
            <a
              href={`https://wa.me/${SALES_WHATSAPP}?text=${encodeURIComponent(message)}`}
              target="_blank"
              rel="noopener noreferrer"
              style={modalSubmitBtn}
            >
              <IconBrandWhatsapp size={18} />Kirim via WhatsApp
            </a>
          ) : (
            <button type="button" onClick={copyMessage} style={modalSubmitBtn}>
              <IconBrandWhatsapp size={18} />{copied ? "Tersalin! Kirim ke tim kami" : "Salin Ringkasan Permintaan"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Illustrative testimonial content — explicitly labeled as examples  */
/* ------------------------------------------------------------------ */

const ILLUSTRATIVE_TESTIMONIALS = [
  {
    name: "Direktur Operasional (Contoh)",
    role: "PPIU Ilustrasi — musim ~800 jamaah",
    quote: "Yang paling terasa itu tim di lapangan dan admin di kantor akhirnya lihat data yang sama, tidak perlu tunggu rekap WhatsApp lagi.",
  },
  {
    name: "Koordinator Rooming (Contoh)",
    role: "PIHK Ilustrasi — musim ~1.200 jamaah",
    quote: "Susun kamar biasanya paling rawan salah pasang mahram. Sekarang sistem yang tolak duluan sebelum jadi masalah di hotel.",
  },
  {
    name: "Muttawwif Lapangan (Contoh)",
    role: "Grup Ilustrasi — 45 jamaah",
    quote: "Manifest bus dan absen kendaraan bisa saya buka dari HP sendiri, tidak perlu bawa kertas lagi tiap keberangkatan.",
  },
] as const;

/* ------------------------------------------------------------------ */
/* Style tokens — scoped to this landing page only.                   */
/* Slate/Emerald light palette, deliberately separate from the        */
/* cream/gold app-chrome palette used inside /dashboard, /leader, and */
/* /pilgrim so this redesign carries zero risk to the rest of the app.*/
/* ------------------------------------------------------------------ */

const COLOR = {
  bg: "#F8FAFC",
  surface: "#FFFFFF",
  border: "#E2E8F0",
  emerald50: "#ECFDF5",
  emerald100: "#D1FAE5",
  emerald300: "#6EE7B7",
  emerald600: "#059669",
  emerald700: "#047857",
  emerald800: "#065F46",
  emerald900: "#064E3B",
  teal600: "#0D9488",
  cyan600: "#0891B2",
  amber100: "#FEF3C7",
  amber300: "#FCD34D",
  amber800: "#92400E",
  rose100: "#FFE4E6",
  rose300: "#FDA4AF",
  rose700: "#BE123C",
  slate900: "#0F172A",
  slate700: "#334155",
  slate600: "#475569",
  slate500: "#64748B",
  slate400: "#94A3B8",
  slate200: "#E2E8F0",
  slate100: "#F1F5F9",
} as const;

const MONO = "ui-monospace, 'SF Mono', Menlo, Consolas, monospace";

const page: React.CSSProperties = { fontFamily: "'Plus Jakarta Sans', sans-serif", background: COLOR.bg, color: COLOR.slate700, overflowX: "hidden" };

const nav: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 16, padding: "14px 32px", position: "sticky", top: 0, background: "rgba(248,250,252,.9)", backdropFilter: "blur(8px)", zIndex: 20, borderBottom: `1px solid ${COLOR.border}`, flexWrap: "wrap" };
const navLogo: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, fontSize: 19, fontWeight: 700, color: COLOR.slate900 };
const navLogoIcon: React.CSSProperties = { width: 30, height: 30, borderRadius: 9, display: "grid", placeItems: "center", background: COLOR.emerald600, color: "#fff" };
const navCenter: React.CSSProperties = { display: "flex", gap: 22, alignItems: "center" };
const navLink: React.CSSProperties = { fontSize: 13.5, fontWeight: 600, color: COLOR.slate600 };
const navLinks: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, minHeight: 40 };
const quickStatPill: React.CSSProperties = { fontSize: 11.5, fontWeight: 700, color: COLOR.emerald800, background: COLOR.emerald100, border: `1px solid ${COLOR.emerald300}`, borderRadius: 999, padding: "6px 12px" };
const navGhostBtn: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: COLOR.emerald700, background: "transparent", border: `1.5px solid ${COLOR.emerald600}`, borderRadius: 8, padding: "9px 16px", minHeight: 40 };
const navCta: React.CSSProperties = { fontSize: 13, fontWeight: 700, background: COLOR.emerald600, color: "#fff", padding: "10px 18px", borderRadius: 8, minHeight: 40, display: "inline-flex", alignItems: "center" };

const hero: React.CSSProperties = { position: "relative", textAlign: "center", padding: "88px 32px 64px", background: `linear-gradient(180deg, ${COLOR.bg} 0%, #fff 100%)` };
const heroInner: React.CSSProperties = { maxWidth: 760, margin: "0 auto" };
const eyebrow: React.CSSProperties = { fontSize: 12, fontWeight: 700, letterSpacing: ".12em", color: COLOR.emerald700, marginBottom: 18, textTransform: "uppercase" };
const heroTitle: React.CSSProperties = { fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: "clamp(32px,6vw,54px)", fontWeight: 800, color: COLOR.slate900, lineHeight: 1.15, marginBottom: 22, letterSpacing: "-.01em" };
const heroSub: React.CSSProperties = { fontSize: 16.5, color: COLOR.slate600, maxWidth: 580, margin: "0 auto 34px", lineHeight: 1.7 };
const ctaRow: React.CSSProperties = { display: "flex", gap: 12, justifyContent: "center", flexWrap: "wrap", minHeight: 48 };
const heroCta: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, background: `linear-gradient(135deg, ${COLOR.emerald600}, ${COLOR.emerald700})`, color: "#fff", fontWeight: 700, fontSize: 15, padding: "14px 26px", borderRadius: 10, boxShadow: "0 8px 20px -6px rgba(5,150,105,.4)" };
const heroOutline: React.CSSProperties = { display: "inline-flex", alignItems: "center", fontWeight: 700, fontSize: 15, padding: "14px 26px", borderRadius: 10, background: "#fff", color: COLOR.slate900, border: `1.5px solid ${COLOR.slate200}` };
const badgeRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", justifyContent: "center", gap: 8, marginTop: 40 };
const badge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12.5, fontWeight: 600, color: COLOR.slate700, background: "#fff", border: `1px solid ${COLOR.border}`, borderRadius: 999, padding: "7px 14px", boxShadow: "0 1px 2px rgba(15,23,42,.04)" };

const section: React.CSSProperties = { padding: "80px 32px" };
const sectionAlt: React.CSSProperties = { padding: "80px 32px", background: COLOR.slate100, borderTop: `1px solid ${COLOR.border}`, borderBottom: `1px solid ${COLOR.border}` };
const sectionInner: React.CSSProperties = { maxWidth: 1080, margin: "0 auto" };
const sectionEyebrow: React.CSSProperties = { textAlign: "center", fontSize: 11.5, fontWeight: 700, letterSpacing: ".1em", textTransform: "uppercase", color: COLOR.emerald700, marginBottom: 8 };
const sectionTitle: React.CSSProperties = { textAlign: "center", fontSize: "clamp(26px,4vw,34px)", fontWeight: 800, color: COLOR.slate900, marginBottom: 10 };
const sectionSub: React.CSSProperties = { textAlign: "center", fontSize: 15, color: COLOR.slate500, maxWidth: 560, margin: "0 auto 40px", lineHeight: 1.6 };

const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(240px,1fr))", gap: 18 };
const featureCard: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 14, padding: "24px 22px", boxShadow: "0 1px 2px rgba(15,23,42,.04)" };
const featureIcon: React.CSSProperties = { width: 42, height: 42, borderRadius: 11, display: "grid", placeItems: "center", background: `linear-gradient(135deg, ${COLOR.emerald600}, ${COLOR.teal600})`, marginBottom: 14 };
const featureTitle: React.CSSProperties = { fontSize: 16, fontWeight: 700, color: COLOR.slate900, marginBottom: 6 };
const featureDesc: React.CSSProperties = { fontSize: 13.5, color: COLOR.slate500, lineHeight: 1.6 };

/* problem/solution comparison */
const compareGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 24 };
const compareCol: React.CSSProperties = { display: "grid", gap: 10 };
const compareHead: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 700, color: COLOR.rose700, marginBottom: 4 };
const compareCardProblem: React.CSSProperties = { background: COLOR.rose100, border: `1px solid ${COLOR.rose300}`, color: "#9F1239", borderRadius: 10, padding: "14px 16px", fontSize: 13.5, lineHeight: 1.5 };
const compareCardSolution: React.CSSProperties = { background: COLOR.emerald50, border: `1px solid ${COLOR.emerald300}`, color: COLOR.emerald800, borderRadius: 10, padding: "14px 16px", fontSize: 13.5, lineHeight: 1.5 };

/* dashboard preview */
const previewCard: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 16, overflow: "hidden", boxShadow: "0 4px 16px -8px rgba(15,23,42,.1)" };
const previewTabBar: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 4, padding: 10, background: COLOR.slate100, borderBottom: `1px solid ${COLOR.border}` };
const previewTabBase: React.CSSProperties = { minHeight: 40, fontSize: 13, fontWeight: 700, padding: "0 14px", borderRadius: 8, border: "1px solid transparent", background: "transparent", color: COLOR.slate500 };
const previewTabActive: React.CSSProperties = { ...previewTabBase, background: "#fff", color: COLOR.emerald700, borderColor: COLOR.border, boxShadow: "0 1px 2px rgba(15,23,42,.06)" };
const previewTabInactive: React.CSSProperties = previewTabBase;
const previewBody: React.CSSProperties = { padding: 22 };
const previewRowsGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(220px,1fr))", gap: 12, marginBottom: 14 };
const previewRoomCard: React.CSSProperties = { border: `1px solid ${COLOR.border}`, borderRadius: 12, padding: 14, background: "#fff" };
const previewRoomType: React.CSSProperties = { fontSize: 14, fontWeight: 700, color: COLOR.slate900, margin: "10px 0 2px" };
const previewRoomOcc: React.CSSProperties = { fontSize: 12.5, color: COLOR.slate500 };
const previewFootnote: React.CSSProperties = { fontSize: 12.5, color: COLOR.slate400, lineHeight: 1.6 };
const monoBadge: React.CSSProperties = { fontFamily: MONO, fontSize: 12, fontWeight: 600, color: COLOR.slate700, background: COLOR.slate100, borderRadius: 6, padding: "3px 8px" };
const inlineLink: React.CSSProperties = { color: COLOR.emerald700, fontWeight: 700 };

const pillBase: React.CSSProperties = { fontSize: 11, fontWeight: 700, borderRadius: 999, padding: "3px 9px", border: "1px solid" };
const pillOk: React.CSSProperties = { ...pillBase, background: COLOR.emerald100, color: COLOR.emerald800, borderColor: COLOR.emerald300 };
const pillWarn: React.CSSProperties = { ...pillBase, background: COLOR.amber100, color: COLOR.amber800, borderColor: COLOR.amber300 };
const pillDanger: React.CSSProperties = { ...pillBase, background: COLOR.rose100, color: "#9F1239", borderColor: COLOR.rose300 };

const sosPreviewWrap: React.CSSProperties = { display: "grid", gap: 16 };
const sosPreviewLeft: React.CSSProperties = { border: `1px solid ${COLOR.border}`, borderRadius: 12, padding: 16, background: COLOR.slate100 };
const escalationTrack: React.CSSProperties = { display: "grid", gap: 10, marginTop: 10 };
const escalationStep: React.CSSProperties = { display: "flex", gap: 10, alignItems: "flex-start" };
const escalationDot: React.CSSProperties = { width: 10, height: 10, borderRadius: "50%", marginTop: 5, flexShrink: 0 };
const escalationLine: React.CSSProperties = { width: 2, height: 16, background: COLOR.border, marginInlineStart: 4 };
const escalationLevel: React.CSSProperties = { fontSize: 13.5, fontWeight: 700, color: COLOR.slate900 };
const escalationSub: React.CSSProperties = { fontSize: 12, color: COLOR.slate500 };

/* simulators */
const simGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(320px,1fr))", gap: 20 };
const simCard: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 16, padding: 24, boxShadow: "0 1px 2px rgba(15,23,42,.04)" };
const simTitle: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, fontSize: 16, fontWeight: 700, color: COLOR.slate900, marginBottom: 6 };
const simDesc: React.CSSProperties = { fontSize: 13, color: COLOR.slate500, lineHeight: 1.6, marginBottom: 16 };
const simButtonRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 14 };
const simBtnBase: React.CSSProperties = { minHeight: 40, fontSize: 12.5, fontWeight: 700, padding: "0 14px", borderRadius: 8, border: `1px solid ${COLOR.border}`, background: "#fff", color: COLOR.slate700 };
const simBtn: React.CSSProperties = simBtnBase;
const simBtnActive: React.CSSProperties = { ...simBtnBase, background: COLOR.emerald600, color: "#fff", borderColor: COLOR.emerald600 };
const simResultBase: React.CSSProperties = { display: "flex", gap: 12, alignItems: "flex-start", borderRadius: 12, padding: 16, border: "1px solid" };
const simResultOk: React.CSSProperties = { ...simResultBase, background: COLOR.emerald50, borderColor: COLOR.emerald300, color: COLOR.emerald800 };
const simResultBlocked: React.CSSProperties = { ...simResultBase, background: COLOR.rose100, borderColor: COLOR.rose300, color: "#9F1239" };
const simResultTitle: React.CSSProperties = { fontSize: 14, fontWeight: 700, marginBottom: 3 };
const simResultSub: React.CSSProperties = { fontSize: 12.5, lineHeight: 1.6, opacity: .9 };

const sosActivateBtn: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, fontSize: 13.5, fontWeight: 700, padding: "0 18px", borderRadius: 10, border: 0, background: COLOR.rose700, color: "#fff" };
const sosProgressTrack: React.CSSProperties = { height: 8, borderRadius: 999, background: COLOR.slate200, overflow: "hidden", marginBottom: 10 };
const sosProgressFill: React.CSSProperties = { height: "100%", background: `linear-gradient(90deg, ${COLOR.amber300}, ${COLOR.rose700})`, transition: "width 1s linear" };
const sosCountdownText: React.CSSProperties = { fontSize: 13, color: COLOR.slate600, marginBottom: 12 };

/* ROI calculator */
const roiCard: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 16, padding: 28, boxShadow: "0 1px 2px rgba(15,23,42,.04)" };
const roiSliders: React.CSSProperties = { display: "grid", gap: 24, marginBottom: 28, gridTemplateColumns: "repeat(auto-fit,minmax(280px,1fr))" };
const roiLabel: React.CSSProperties = { display: "grid", gap: 8, fontSize: 13, fontWeight: 700, color: COLOR.slate700 };
const roiSliderValue: React.CSSProperties = { fontFamily: MONO, fontSize: 15, fontWeight: 700, color: COLOR.emerald700 };
const roiRange: React.CSSProperties = { width: "100%", accentColor: COLOR.emerald600 };
const roiResultGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 14, marginBottom: 16 };
const roiResultCard: React.CSSProperties = { background: COLOR.emerald50, border: `1px solid ${COLOR.emerald300}`, borderRadius: 12, padding: "18px 16px", textAlign: "center" };
const roiResultValue: React.CSSProperties = { fontSize: 22, fontWeight: 800, color: COLOR.slate900, marginBottom: 4 };
const roiResultLabel: React.CSSProperties = { fontSize: 12, color: COLOR.slate600, lineHeight: 1.5 };

/* testimonials */
const testimonialCard: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 14, padding: 22, position: "relative" };
const illustrativeTag: React.CSSProperties = { display: "inline-block", fontSize: 10.5, fontWeight: 700, letterSpacing: ".04em", color: COLOR.amber800, background: COLOR.amber100, border: `1px solid ${COLOR.amber300}`, borderRadius: 999, padding: "3px 9px", marginBottom: 14 };
const testimonialQuote: React.CSSProperties = { fontSize: 14, color: COLOR.slate700, lineHeight: 1.65, marginBottom: 16 };
const testimonialName: React.CSSProperties = { fontSize: 13.5, fontWeight: 700, color: COLOR.slate900 };
const testimonialRole: React.CSSProperties = { fontSize: 12, color: COLOR.slate400 };

/* FAQ */
const faqList: React.CSSProperties = { display: "grid", gap: 10, maxWidth: 760, margin: "0 auto" };
const faqItem: React.CSSProperties = { background: COLOR.surface, border: `1px solid ${COLOR.border}`, borderRadius: 12, overflow: "hidden" };
const faqQuestion: React.CSSProperties = { width: "100%", display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, minHeight: 52, padding: "14px 18px", background: "transparent", border: 0, fontSize: 14, fontWeight: 700, color: COLOR.slate900, textAlign: "start" };
const faqAnswer: React.CSSProperties = { padding: "0 18px 18px", fontSize: 13.5, color: COLOR.slate600, lineHeight: 1.7 };

/* CTA banner */
const banner: React.CSSProperties = { position: "relative", background: `linear-gradient(135deg, ${COLOR.emerald800} 0%, ${COLOR.emerald900} 100%)`, textAlign: "center", padding: "80px 32px" };
const bannerInner: React.CSSProperties = {};
const bannerTitle: React.CSSProperties = { color: "#fff", fontSize: "clamp(28px,5vw,38px)", fontWeight: 800, margin: "10px 0 14px", lineHeight: 1.2 };
const bannerText: React.CSSProperties = { fontSize: 15, color: "rgba(255,255,255,.75)", maxWidth: 460, margin: "0 auto 30px" };
const bannerOutline: React.CSSProperties = { display: "inline-flex", alignItems: "center", fontWeight: 700, fontSize: 15, padding: "14px 26px", borderRadius: 10, background: "transparent", color: "#fff", border: "1.5px solid rgba(255,255,255,.4)" };

/* footer */
const footer: React.CSSProperties = { padding: "48px 32px 28px", borderTop: `1px solid ${COLOR.border}` };
const footerInner: React.CSSProperties = { maxWidth: 1080, margin: "0 auto", display: "grid", gridTemplateColumns: "1.4fr 1fr 1fr", gap: 32 };
const footerTag: React.CSSProperties = { fontSize: 12.5, color: COLOR.slate400, marginTop: 10 };
const footerCol: React.CSSProperties = { display: "grid", gap: 8, alignContent: "start" };
const footerColTitle: React.CSSProperties = { fontSize: 11.5, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: COLOR.slate400, marginBottom: 4 };
const footerLink: React.CSSProperties = { fontSize: 13, color: COLOR.slate600 };
const footerCopyright: React.CSSProperties = { maxWidth: 1080, margin: "32px auto 0", fontSize: 12, color: COLOR.slate400 };

/* modal */
const modalOverlay: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(15,23,42,.5)", display: "grid", placeItems: "center", zIndex: 50, padding: 16 };
const modalCard: React.CSSProperties = { width: "100%", maxWidth: 480, maxHeight: "90vh", overflowY: "auto", background: "#fff", borderRadius: 16, boxShadow: "0 24px 48px -12px rgba(15,23,42,.35)" };
const modalHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "18px 22px", borderBottom: `1px solid ${COLOR.border}` };
const modalTitle: React.CSSProperties = { fontSize: 17, fontWeight: 800, color: COLOR.slate900 };
const modalCloseBtn: React.CSSProperties = { width: 36, height: 36, borderRadius: 9, border: 0, background: COLOR.slate100, color: COLOR.slate500, display: "grid", placeItems: "center" };
const modalBody: React.CSSProperties = { display: "grid", gap: 14, padding: 22 };
const modalLabel: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, fontWeight: 700, color: COLOR.slate700 };
const modalInput: React.CSSProperties = { minHeight: 44, border: `1px solid ${COLOR.border}`, borderRadius: 9, padding: "0 12px", fontSize: 14, font: "inherit", color: COLOR.slate900 };
const modalFieldset: React.CSSProperties = { border: 0, padding: 0, margin: 0, display: "grid", gap: 8 };
const modalChipRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8 };
const modalChipBase: React.CSSProperties = { minHeight: 36, display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12, fontWeight: 600, padding: "0 12px", borderRadius: 999, border: `1px solid ${COLOR.border}`, background: "#fff", color: COLOR.slate600 };
const modalChip: React.CSSProperties = modalChipBase;
const modalChipActive: React.CSSProperties = { ...modalChipBase, background: COLOR.emerald600, borderColor: COLOR.emerald600, color: "#fff" };
const modalSubmitBtn: React.CSSProperties = { display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, minHeight: 48, borderRadius: 10, border: 0, background: COLOR.emerald600, color: "#fff", fontWeight: 700, fontSize: 14, marginTop: 4 };
