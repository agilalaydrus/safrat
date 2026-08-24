"use client";

import { useState } from "react";
import { AlertTriangle, Bus, Hotel, Users } from "lucide-react";

type TabId = "kamar" | "bus" | "sos" | "agen";

const TABS: { id: TabId; label: string; icon: typeof Hotel }[] = [
  { id: "kamar", label: "Alokasi Kamar & Mahram", icon: Hotel },
  { id: "bus", label: "Armada Bus & Driver Saudi", icon: Bus },
  { id: "sos", label: "Gateway SOS 10 Menit", icon: AlertTriangle },
  { id: "agen", label: "Komisi Agen & Cabang", icon: Users },
];

const panelHeader = "flex flex-col gap-4 border-b border-slate-100 pb-4 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800";
const cardBase = "rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950";

export default function ModuleTabs() {
  const [activeTab, setActiveTab] = useState<TabId>("kamar");

  return (
    <section id="cara-kerja" className="bg-slate-50 py-20 dark:bg-slate-900">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto mb-12 max-w-3xl text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Interactive Dashboard
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-4xl dark:text-slate-100">
            Eksplorasi Modul Operasional Unggulan
          </h3>
          <p className="mt-3 text-sm text-slate-600 sm:text-base dark:text-slate-300">
            Klik tab di bawah untuk melihat bagaimana sistem menangani setiap tantangan lapangan.
          </p>
        </div>

        {/* Tab switcher */}
        <div className="mb-8 flex flex-wrap justify-center gap-2">
          {TABS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => setActiveTab(id)}
              className={`flex items-center gap-2 rounded-xl px-5 py-2.5 text-xs font-bold transition-all sm:text-sm ${
                activeTab === id
                  ? "bg-emerald-600 text-white shadow-md shadow-emerald-600/20"
                  : "border border-slate-200 bg-white text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-300 dark:hover:bg-slate-800"
              }`}
            >
              <Icon className="h-4 w-4" />
              <span>{label}</span>
            </button>
          ))}
        </div>

        {/* Panel */}
        <div className="mx-auto max-w-4xl rounded-3xl border border-slate-200 bg-white p-6 shadow-xl sm:p-10 dark:border-slate-800 dark:bg-slate-950">
          {activeTab === "kamar" && (
            <div className="space-y-6">
              <div className={panelHeader}>
                <div>
                  <h4 className="text-xl font-bold text-slate-900 dark:text-white">Hotel Safwah Royal Orchid (Makkah)</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">Okupansi: 96% • Algoritma Mahram Sesuai Syariah Aktif</p>
                </div>
                <span className="w-fit rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                  ✓ Validasi Mahram 100% Lolos
                </span>
              </div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className={`${cardBase} dark:bg-slate-900`}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-bold text-slate-900 dark:text-white">Kamar #1402 (Quad Pria)</span>
                    <span className="rounded bg-emerald-600 px-2 py-0.5 text-[10px] font-bold text-white">4/4 Full</span>
                  </div>
                  <p className="text-xs text-slate-600 dark:text-slate-400">H. Ridwan, H. Syamsul, Bpk. Fajar, Bpk. Subagio</p>
                  <span className="mt-2 block text-[11px] font-semibold text-emerald-700 dark:text-emerald-400">
                    ✓ Verifikasi Gender: Laki-laki
                  </span>
                </div>
                <div className={`${cardBase} dark:bg-slate-900`}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-bold text-slate-900 dark:text-white">Kamar #1405 (Double Pasutri)</span>
                    <span className="rounded bg-emerald-600 px-2 py-0.5 text-[10px] font-bold text-white">2/2 Full</span>
                  </div>
                  <p className="text-xs text-slate-600 dark:text-slate-400">H. Hendra (Suami) &amp; Hj. Farida (Istri Sah)</p>
                  <span className="mt-2 block text-[11px] font-semibold text-emerald-700 dark:text-emerald-400">
                    ✓ Surat Nikah / Mahram Terverifikasi
                  </span>
                </div>
              </div>
              <div className="rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-xs text-slate-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-slate-300">
                💡 <strong>Keunggulan:</strong> Jika Anda mencoba memasukkan jamaah pria dan wanita yang bukan mahram dalam
                satu kamar, tombol &quot;Simpan Rooming List&quot; otomatis terkunci dengan peringatan tegas.
              </div>
            </div>
          )}

          {activeTab === "bus" && (
            <div className="space-y-6">
              <div className={panelHeader}>
                <div>
                  <h4 className="text-xl font-bold text-slate-900 dark:text-white">Plotting Armada Bus VIP Saptco</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">Rute: Bandara Jeddah (JED) ➔ Hotel Madinah</p>
                </div>
                <span className="w-fit rounded-full bg-teal-100 px-3 py-1 text-xs font-bold text-teal-800 dark:bg-teal-950 dark:text-teal-300">
                  Link Mobile Supir Aktif
                </span>
              </div>
              <div className={`${cardBase} space-y-3 dark:bg-slate-900`}>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-bold text-slate-900 dark:text-white">Bus #01 VIP (KSA 7721 BXD)</span>
                  <span className="font-mono text-xs font-bold text-slate-700 dark:text-slate-300">45/45 Kursi</span>
                </div>
                <div className="grid grid-cols-1 gap-2 text-xs text-slate-600 sm:grid-cols-2 dark:text-slate-400">
                  <div>Supir: <strong className="text-slate-800 dark:text-slate-200">Syeikh Tariq (+966 50 123 4567)</strong></div>
                  <div>Muthowif: <strong className="text-slate-800 dark:text-slate-200">Ust. Syauqi (+62 812-9988)</strong></div>
                </div>
                <div className="flex items-center justify-between pt-2 text-xs">
                  <span className="text-slate-500 dark:text-slate-400">Status Manifest: Terkirim ke WhatsApp Supir</span>
                  <span className="font-bold text-emerald-600 dark:text-emerald-400">✓ Dibuka Supir 5 menit lalu</span>
                </div>
              </div>
              <div className="rounded-2xl border border-teal-200 bg-teal-50 p-4 text-xs text-slate-700 dark:border-teal-900/60 dark:bg-teal-950/30 dark:text-slate-300">
                📱 <strong>Tanpa Instal Aplikasi:</strong> Driver Saudi &amp; Muthowif cukup klik link tautan web ringan via
                WhatsApp untuk cek nomor kursi jamaah secara real-time.
              </div>
            </div>
          )}

          {activeTab === "sos" && (
            <div className="space-y-6">
              <div className={panelHeader}>
                <div>
                  <h4 className="text-xl font-bold text-slate-900 dark:text-white">Gateway Darurat Jamaah (10 Menit)</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">Protokol Penanganan Jamaah Terpisah di Masjidil Haram</p>
                </div>
                <span className="w-fit animate-pulse rounded-full bg-rose-100 px-3 py-1 text-xs font-bold text-rose-800 dark:bg-rose-950 dark:text-rose-300">
                  🚨 Peringatan Aktif
                </span>
              </div>
              <div className="space-y-2 rounded-2xl border border-rose-200 bg-rose-50 p-4 dark:border-rose-900/60 dark:bg-rose-950/30">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-bold text-rose-900 dark:text-rose-200">Hj. Aminah Sudirman (64 Thn) - Kloter 02</span>
                  <span className="rounded bg-rose-600 px-2 py-0.5 font-mono text-xs font-bold text-white">06:42 Sisa Waktu</span>
                </div>
                <p className="text-xs text-slate-700 dark:text-slate-300">📍 Terdeteksi di: Pintu 79 King Fahd Gate, Masjidil Haram</p>
                <p className="text-xs text-slate-600 dark:text-slate-400">
                  Muthowif yang Ditugaskan: <strong className="text-slate-800 dark:text-slate-200">Ust. Syauqi (Sedang Menuju Lokasi)</strong>
                </p>
              </div>
              <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-xs text-slate-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-slate-300">
                ⏱️ <strong>Auto-Escalation:</strong> Jika dalam 10 menit Muthowif belum mengonfirmasi penanganan, notifikasi
                darurat otomatis berdering ke ponsel Direktur Operasional di Jakarta.
              </div>
            </div>
          )}

          {activeTab === "agen" && (
            <div className="space-y-6">
              <div className={panelHeader}>
                <div>
                  <h4 className="text-xl font-bold text-slate-900 dark:text-white">Transparansi Komisi Agen &amp; Mitra Cabang</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">Rekapitulasi komisi otomatis per kloter dan per musim</p>
                </div>
                <span className="w-fit rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                  Rekap Otomatis
                </span>
              </div>
              <div className={`${cardBase} space-y-3 dark:bg-slate-900`}>
                <div className="flex items-center justify-between text-sm">
                  <span className="font-bold text-slate-900 dark:text-white">Mitra Cabang Surabaya (Ust. Fadli)</span>
                  <span className="font-mono font-bold text-emerald-600 dark:text-emerald-400">Rp 48.500.000</span>
                </div>
                <div className="flex justify-between text-xs text-slate-600 dark:text-slate-400">
                  <span>Total 24 Jamaah Terdaftar (Lunas)</span>
                  <span className="text-slate-500 dark:text-slate-500">Status Pembayaran: Siap Cair</span>
                </div>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-slate-100 p-4 text-xs text-slate-700 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300">
                🤝 <strong>Bebas Sengketa Komisi:</strong> Agen dapat melihat status jamaah rujukan mereka secara transparan
                tanpa perlu bolak-balik menanyakan rekapan ke kantor pusat.
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
