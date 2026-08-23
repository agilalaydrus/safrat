"use client";

import { useState, type FormEvent } from "react";
import { ArrowRight, CheckCircle2, Compass, X } from "lucide-react";

export default function DemoModal({ onClose }: { onClose: () => void }) {
  const [submitted, setSubmitted] = useState(false);
  const [form, setForm] = useState({
    namaTravel: "",
    namaPIC: "",
    whatsapp: "",
    estimasiJamaah: "200 - 500 Jamaah",
  });

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitted(true);
  }

  const inputClass =
    "w-full rounded-xl border border-slate-300 bg-slate-50 px-3.5 py-2.5 text-xs text-slate-900 placeholder-slate-400 focus:border-emerald-600 focus:bg-white focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:focus:bg-slate-800";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4 backdrop-blur-sm">
      <div className="relative w-full max-w-lg overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl dark:border-slate-800 dark:bg-slate-950">
        <button
          type="button"
          onClick={onClose}
          aria-label="Tutup"
          className="absolute right-4 top-4 rounded-full bg-slate-100 p-2 text-slate-500 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
        >
          <X className="h-5 w-5" />
        </button>

        <div className="border-b border-slate-200 bg-slate-50 p-6 sm:p-8 dark:border-slate-800 dark:bg-slate-900">
          <div className="mb-1 flex items-center gap-2">
            <Compass className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
            <span className="font-mono text-xs font-bold uppercase text-emerald-700 dark:text-emerald-400">
              Tawafiq Hub Enterprise
            </span>
          </div>
          <h3 className="text-xl font-bold text-slate-900 dark:text-white">Jadwalkan Demo &amp; Konsultasi Sistem</h3>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Dapatkan akses langsung ke dashboard dan panduan setup musim 1447H.
          </p>
        </div>

        <div className="p-6 sm:p-8">
          {submitted ? (
            <div className="space-y-4 py-6 text-center">
              <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-950 dark:text-emerald-400">
                <CheckCircle2 className="h-8 w-8" />
              </div>
              <h4 className="text-lg font-bold text-slate-900 dark:text-white">Permintaan Demo Terkirim!</h4>
              <p className="text-xs leading-relaxed text-slate-600 dark:text-slate-400">
                Terima kasih, <strong>{form.namaPIC || "Bapak/Ibu"}</strong>. Tim konsultan operasional kami akan
                mengirimkan link akses demo ke WhatsApp <strong>{form.whatsapp}</strong> dalam 15 menit.
              </p>
              <button
                type="button"
                onClick={onClose}
                className="rounded-xl bg-emerald-600 px-6 py-2.5 text-xs font-bold text-white hover:bg-emerald-700"
              >
                Tutup
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="mb-1 block text-xs font-bold text-slate-700 dark:text-slate-300">
                  Nama Travel Umrah / PIHK *
                </label>
                <input
                  type="text"
                  required
                  placeholder="Contoh: PT. Barokah Tour & Travel"
                  value={form.namaTravel}
                  onChange={(e) => setForm({ ...form, namaTravel: e.target.value })}
                  className={inputClass}
                />
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-bold text-slate-700 dark:text-slate-300">
                    Nama PIC / Pimpinan *
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="Nama Lengkap"
                    value={form.namaPIC}
                    onChange={(e) => setForm({ ...form, namaPIC: e.target.value })}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-bold text-slate-700 dark:text-slate-300">Nomor WhatsApp *</label>
                  <input
                    type="tel"
                    required
                    placeholder="0812-xxxx-xxxx"
                    value={form.whatsapp}
                    onChange={(e) => setForm({ ...form, whatsapp: e.target.value })}
                    className={inputClass}
                  />
                </div>
              </div>

              <div>
                <label className="mb-1 block text-xs font-bold text-slate-700 dark:text-slate-300">
                  Estimasi Jamaah per Musim
                </label>
                <select
                  value={form.estimasiJamaah}
                  onChange={(e) => setForm({ ...form, estimasiJamaah: e.target.value })}
                  className={inputClass}
                >
                  <option value="50 - 200 Jamaah">50 - 200 Jamaah (1-3 Kloter)</option>
                  <option value="200 - 500 Jamaah">200 - 500 Jamaah</option>
                  <option value="500 - 1.500 Jamaah">500 - 1.500 Jamaah</option>
                  <option value="> 1.500 Jamaah">&gt; 1.500 Jamaah (Enterprise)</option>
                </select>
              </div>

              <button
                type="submit"
                className="mt-2 flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-600 py-3 text-xs font-bold text-white shadow-md shadow-emerald-600/30 transition-all hover:bg-emerald-700 sm:text-sm"
              >
                <span>Kirim Permintaan Demo</span>
                <ArrowRight className="h-4 w-4" />
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
