"use client";

import { useEffect, useState } from "react";
import {
  AlertTriangle,
  Bed,
  Bus,
  CheckCircle2,
  MapPin,
  MessageCircle,
  Plus,
  RefreshCcw,
  Siren,
} from "lucide-react";

type Room = {
  id: string;
  code: string;
  hotel: string;
  type: string;
  occupants: { name: string; gender: "L" | "P"; relation: string }[];
  capacity: number;
};

const INITIAL_ROOMS: Room[] = [
  {
    id: "r1",
    code: "MK-QUAD-104",
    hotel: "Pullman Zamzam, Makkah",
    type: "Quad",
    capacity: 4,
    occupants: [
      { name: "Ahmad Fauzi", gender: "L", relation: "Suami" },
      { name: "Siti Aminah", gender: "P", relation: "Istri" },
      { name: "Budi Santoso", gender: "L", relation: "Ayah" },
      { name: "Rina Wulandari", gender: "P", relation: "Anak" },
    ],
  },
  {
    id: "r2",
    code: "MD-DBL-207",
    hotel: "Movenpick Al Aziziyah, Madinah",
    type: "Double",
    capacity: 2,
    occupants: [{ name: "Hasan Basri", gender: "L", relation: "Sendiri" }],
  },
  {
    id: "r3",
    code: "MK-QUAD-118",
    hotel: "Pullman Zamzam, Makkah",
    type: "Quad",
    capacity: 4,
    occupants: [
      { name: "Dewi Lestari", gender: "P", relation: "Ibu" },
      { name: "Putri Handayani", gender: "P", relation: "Anak" },
    ],
  },
];

const BUSES = [
  {
    id: "b1",
    code: "SAPTCO VIP #01",
    route: "Jeddah menuju Makkah",
    driver: "Syeikh Tariq",
    plate: "KSA 7721",
    seats: 45,
    filled: 45,
  },
  {
    id: "b2",
    code: "SAPTCO VIP #02",
    route: "Makkah menuju Madinah",
    driver: "Syeikh Faisal",
    plate: "KSA 5540",
    seats: 45,
    filled: 42,
  },
];

const SOS_DEMO_SECONDS = 12;

function isMahramValid(occupants: Room["occupants"]) {
  const genders = new Set(occupants.map((o) => o.gender));
  if (genders.size <= 1) return true;
  return occupants.every((o) => o.relation !== "Sendiri" && o.relation !== "Non mahram");
}

function exportRoomingList(rooms: Room[]) {
  const win = window.open("", "_blank", "width=800,height=900");
  if (!win) return;
  const rows = rooms
    .map(
      (r) =>
        `<tr><td>${r.code}</td><td>${r.hotel}</td><td>${r.type}</td><td>${r.occupants.length}/${r.capacity}</td><td>${r.occupants
          .map((o) => `${o.name} (${o.gender})`)
          .join(", ")}</td></tr>`
    )
    .join("");
  win.document.write(`
    <html><head><title>Rooming List, Tawafiq Hub</title>
    <style>
      body{font-family:sans-serif;padding:24px;color:#0f172a}
      h1{font-size:18px;margin-bottom:4px}
      p{color:#64748b;font-size:12px;margin-top:0}
      table{width:100%;border-collapse:collapse;margin-top:16px;font-size:12px}
      th,td{border:1px solid #e2e8f0;padding:8px;text-align:left}
      th{background:#f1f5f9}
    </style></head>
    <body>
      <h1>Rooming List, Musim 1447H</h1>
      <p>Data contoh dari demo interaktif Tawafiq Hub. Gunakan dialog print browser untuk menyimpan sebagai PDF.</p>
      <table><thead><tr><th>Kode Kamar</th><th>Hotel</th><th>Tipe</th><th>Terisi</th><th>Penghuni</th></tr></thead><tbody>${rows}</tbody></table>
    </body></html>
  `);
  win.document.close();
  win.focus();
  win.print();
}

function RoomingTab() {
  const [rooms, setRooms] = useState(INITIAL_ROOMS);

  function addRoom() {
    const n = rooms.length + 1;
    setRooms((prev) => [
      ...prev,
      { id: `r${Date.now()}`, code: `MK-DBL-${100 + n}`, hotel: "Pullman Zamzam, Makkah", type: "Double", capacity: 2, occupants: [] },
    ]);
  }

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Kapasitas dan aturan mahram divalidasi otomatis begitu kamar diisi.
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => exportRoomingList(rooms)}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3.5 py-2 text-xs font-bold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
          >
            Ekspor Rooming List PDF
          </button>
          <button
            type="button"
            onClick={addRoom}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700"
          >
            <Plus size={14} />
            Tambah Kamar Baru
          </button>
        </div>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {rooms.map((room) => {
          const valid = isMahramValid(room.occupants);
          return (
            <div key={room.id} className="rounded-xl border border-slate-200 bg-white p-4 dark:border-slate-700 dark:bg-slate-900">
              <div className="mb-2 flex items-center justify-between">
                <span className="rounded bg-slate-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                  {room.code}
                </span>
                <span
                  className={
                    room.occupants.length === 0
                      ? "rounded-full border border-slate-300 bg-slate-100 px-2 py-0.5 text-[10px] font-bold text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400"
                      : valid
                        ? "rounded-full border border-emerald-300 bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                        : "rounded-full border border-red-300 bg-red-100 px-2 py-0.5 text-[10px] font-bold text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-300"
                  }
                >
                  {room.occupants.length === 0 ? "Kosong" : `${room.occupants.length}/${room.capacity} ${valid ? "Valid" : "Bermasalah"}`}
                </span>
              </div>
              <p className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200">
                <Bed size={13} />
                {room.type}, {room.hotel}
              </p>
              <div className="space-y-1.5">
                {room.occupants.length === 0 && <p className="text-xs italic text-slate-400">Belum ada jamaah ditempatkan.</p>}
                {room.occupants.map((o) => (
                  <div key={o.name} className="flex items-center justify-between text-xs text-slate-600 dark:text-slate-300">
                    <span>{o.name}</span>
                    <span className="flex items-center gap-1">
                      <span className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] dark:bg-slate-800">{o.gender}</span>
                      <span className="text-slate-400">{o.relation}</span>
                    </span>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ArmadaTab() {
  const [sent, setSent] = useState<string | null>(null);

  function sendManifest(bus: (typeof BUSES)[number]) {
    const text = `Manifes ${bus.code}\nRute: ${bus.route}\nSupir: ${bus.driver} (${bus.plate})\nKursi terisi: ${bus.filled}/${bus.seats}`;
    window.open(`https://wa.me/?text=${encodeURIComponent(text)}`, "_blank");
    setSent(bus.id);
    window.setTimeout(() => setSent(null), 3000);
  }

  return (
    <div>
      <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
        Supir dan Muthowif buka manifesnya langsung dari HP masing masing, tidak perlu aplikasi khusus.
      </p>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {BUSES.map((bus) => (
          <div key={bus.id} className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-700 dark:bg-slate-900">
            <div className="mb-3 flex items-center justify-between">
              <span className="flex items-center gap-2 font-mono text-sm font-bold text-emerald-700 dark:text-emerald-400">
                <Bus size={16} />
                {bus.code}
              </span>
              <span className="rounded bg-slate-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                {bus.filled}/{bus.seats} kursi
              </span>
            </div>
            <p className="mb-1 text-sm text-slate-700 dark:text-slate-200">{bus.route}</p>
            <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
              Supir: {bus.driver}, plat {bus.plate}
            </p>
            <button
              type="button"
              onClick={() => sendManifest(bus)}
              className="inline-flex w-full items-center justify-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2.5 text-xs font-bold text-white hover:bg-emerald-700"
            >
              <MessageCircle size={14} />
              {sent === bus.id ? "Terkirim ke WhatsApp" : "Kirim Link Manifes ke WhatsApp"}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

function SosTab() {
  const [phase, setPhase] = useState<"active" | "escalated" | "resolved">("active");
  const [secondsLeft, setSecondsLeft] = useState(SOS_DEMO_SECONDS);

  function startCountdown() {
    setPhase("active");
    setSecondsLeft(SOS_DEMO_SECONDS);
  }

  useEffect(() => {
    if (phase !== "active") return;
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
    return () => window.clearInterval(timer);
  }, [phase]);

  function resolve() {
    setPhase("resolved");
  }

  return (
    <div className="grid grid-cols-1 gap-5 lg:grid-cols-[1.1fr_1fr]">
      <div className="rounded-xl border border-red-200 bg-red-50 p-5 dark:border-red-900 dark:bg-red-950/40">
        <div className="mb-3 flex items-center gap-2 text-red-800 dark:text-red-300">
          <Siren size={18} />
          <p className="text-sm font-bold">Jamaah Terpisah, Pintu 79, King Fahd Gate</p>
        </div>
        {phase !== "resolved" ? (
          <>
            <p className="mb-4 text-xs text-red-700 dark:text-red-300">
              {phase === "active"
                ? `Menunggu respons Muthowif, eskalasi otomatis dalam ${secondsLeft} detik lagi`
                : "Sudah 10 menit tidak direspons, otomatis dieskalasi ke Direksi PPIU"}
            </p>
            <div className="mb-4 h-2 overflow-hidden rounded-full bg-red-200 dark:bg-red-900">
              <div
                className="h-full bg-red-600 transition-all duration-1000"
                style={{ width: `${((SOS_DEMO_SECONDS - secondsLeft) / SOS_DEMO_SECONDS) * 100}%` }}
              />
            </div>
            <div className="mb-4 space-y-2">
              <StatusRow active={phase === "active"} label="Level 1, Muthowif Lapangan" />
              <StatusRow active={phase === "escalated"} label="Level 2, Direksi PPIU" danger />
            </div>
            <button
              type="button"
              onClick={resolve}
              className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700"
            >
              <CheckCircle2 size={14} />
              Tandai Selesai, Jamaah Ditemukan
            </button>
          </>
        ) : (
          <div className="flex items-center gap-2 text-emerald-700 dark:text-emerald-400">
            <CheckCircle2 size={16} />
            <p className="text-sm font-semibold">Kasus selesai, jamaah sudah ditemukan dan aman.</p>
          </div>
        )}
        {phase === "resolved" && (
          <button
            type="button"
            onClick={startCountdown}
            className="mt-3 inline-flex items-center gap-1.5 text-xs font-bold text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
          >
            <RefreshCcw size={12} />
            Ulangi simulasi
          </button>
        )}
      </div>

      <div className="relative overflow-hidden rounded-xl border border-slate-200 bg-slate-100 dark:border-slate-700 dark:bg-slate-900">
        <div
          className="absolute inset-0 opacity-40"
          style={{
            backgroundImage:
              "linear-gradient(to right, #cbd5e1 1px, transparent 1px), linear-gradient(to bottom, #cbd5e1 1px, transparent 1px)",
            backgroundSize: "24px 24px",
          }}
        />
        <div className="relative flex h-full min-h-[220px] flex-col items-center justify-center gap-2 p-6">
          <span className="relative flex h-4 w-4">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-500 opacity-60" />
            <span className="relative inline-flex h-4 w-4 rounded-full bg-red-600" />
          </span>
          <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-600 dark:text-slate-300">
            <MapPin size={13} />
            Titik koordinat terakhir jamaah
          </div>
          <p className="max-w-[220px] text-center text-[11px] text-slate-400">
            Ilustrasi peta, bukan data GPS langsung. Di sistem asli titik ini muncul dari lokasi HP jamaah.
          </p>
        </div>
      </div>
    </div>
  );
}

function StatusRow({ active, label, danger }: { active: boolean; label: string; danger?: boolean }) {
  return (
    <div
      className={
        active
          ? danger
            ? "flex items-center gap-2 rounded-lg border border-red-300 bg-red-100 px-3 py-2 text-xs font-bold text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-300"
            : "flex items-center gap-2 rounded-lg border border-amber-300 bg-amber-100 px-3 py-2 text-xs font-bold text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-300"
          : "flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-400 dark:border-slate-700 dark:bg-slate-900"
      }
    >
      {active && <AlertTriangle size={13} />}
      {label}
    </div>
  );
}

function SubstitusiTab() {
  const [replaced, setReplaced] = useState(false);

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-700 dark:bg-slate-900">
      <div className="mb-3 flex items-center justify-between">
        <span className="rounded bg-slate-100 px-2 py-0.5 font-mono text-[11px] font-semibold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          MK-QUAD-104
        </span>
        <span className="rounded-full border border-emerald-300 bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
          Susunan kamar tetap utuh
        </span>
      </div>
      <p className="mb-4 text-sm text-slate-700 dark:text-slate-200">
        {replaced ? (
          <>
            <span className="text-red-500 line-through">Ahmad Fauzi</span> digantikan{" "}
            <span className="font-semibold text-emerald-700 dark:text-emerald-400">Rudi Hartono</span>, batal berangkat H-3 karena sakit
          </>
        ) : (
          "Ahmad Fauzi dijadwalkan berangkat, kondisinya sehat"
        )}
      </p>
      <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
        Kamar, kursi bus, dan riwayat penempatan tidak ikut berubah, hanya nama pesertanya yang diganti dan tercatat di log.
      </p>
      <button
        type="button"
        onClick={() => setReplaced((v) => !v)}
        className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700"
      >
        {replaced ? "Batalkan simulasi" : "Simulasikan Jamaah Batal H-3"}
      </button>
    </div>
  );
}

const TABS = ["rooming", "armada", "sos", "substitusi"] as const;
type TabId = (typeof TABS)[number];
const TAB_LABEL: Record<TabId, string> = {
  rooming: "Alokasi Kamar & Mahram",
  armada: "Armada Bus & Supir Saudi",
  sos: "Gateway Darurat SOS",
  substitusi: "Substitusi Jamaah Instan",
};

export default function DashboardPreview() {
  const [tab, setTab] = useState<TabId>("rooming");

  return (
    <section id="preview" className="bg-white px-5 py-20 dark:bg-slate-900">
      <div className="mx-auto max-w-6xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Interactive Preview
        </p>
        <h2 className="mb-3 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Coba Langsung, Bukan Cuma Lihat Screenshot
        </h2>
        <p className="mx-auto mb-10 max-w-xl text-center text-sm text-slate-500 dark:text-slate-400">
          Empat alur kerja yang paling sering dipakai operator tiap musim. Klik tabnya, semua yang ada di sini bisa
          diinteraksi langsung.
        </p>

        <div className="overflow-hidden rounded-3xl border border-slate-200 shadow-xl dark:border-slate-700">
          <div className="flex flex-wrap gap-1 border-b border-slate-200 bg-slate-100 p-2 dark:border-slate-700 dark:bg-slate-800">
            {TABS.map((id) => (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={
                  tab === id
                    ? "rounded-lg bg-white px-3.5 py-2 text-xs font-bold text-emerald-700 shadow-sm dark:bg-slate-900 dark:text-emerald-400"
                    : "rounded-lg px-3.5 py-2 text-xs font-semibold text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
                }
              >
                {TAB_LABEL[id]}
              </button>
            ))}
          </div>
          <div className="bg-white p-5 dark:bg-slate-900 sm:p-7">
            {tab === "rooming" && <RoomingTab />}
            {tab === "armada" && <ArmadaTab />}
            {tab === "sos" && <SosTab />}
            {tab === "substitusi" && <SubstitusiTab />}
          </div>
        </div>
      </div>
    </section>
  );
}
