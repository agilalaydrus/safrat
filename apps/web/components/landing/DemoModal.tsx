"use client";

import { useMemo, useState } from "react";
import { CheckCircle2, MessageCircle, X } from "lucide-react";
import { FEATURE_MODULES } from "./content";

const SALES_WHATSAPP = process.env.NEXT_PUBLIC_SALES_WHATSAPP ?? "";

export default function DemoModal({ onClose }: { onClose: () => void }) {
  const [travelName, setTravelName] = useState("");
  const [picName, setPicName] = useState("");
  const [whatsapp, setWhatsapp] = useState("");
  const [pilgrimCount, setPilgrimCount] = useState("");
  const [modules, setModules] = useState<string[]>([]);
  const [sent, setSent] = useState(false);

  function toggleModule(title: string) {
    setModules((prev) => (prev.includes(title) ? prev.filter((m) => m !== title) : [...prev, title]));
  }

  const message = useMemo(() => {
    return [
      "Halo Tawafiq Hub, saya mau jadwalkan demo.",
      `Nama Travel: ${travelName || "-"}`,
      `PIC: ${picName || "-"}`,
      `WhatsApp: ${whatsapp || "-"}`,
      `Estimasi Jamaah: ${pilgrimCount || "-"}`,
      `Modul yang diminati: ${modules.length ? modules.join(", ") : "-"}`,
    ].join("\n");
  }, [travelName, picName, whatsapp, pilgrimCount, modules]);

  function submit() {
    const url = SALES_WHATSAPP
      ? `https://wa.me/${SALES_WHATSAPP}?text=${encodeURIComponent(message)}`
      : `https://wa.me/?text=${encodeURIComponent(message)}`;
    window.open(url, "_blank");
    setSent(true);
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4" role="dialog" aria-modal="true" aria-label="Jadwalkan demo">
      <div className="max-h-[90vh] w-full max-w-md overflow-y-auto rounded-2xl bg-white shadow-2xl dark:bg-slate-900">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4 dark:border-slate-700">
          <h3 className="text-lg font-extrabold text-slate-900 dark:text-white">Jadwalkan Demo</h3>
          <button type="button" onClick={onClose} aria-label="Tutup" className="grid h-9 w-9 place-items-center rounded-lg text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800">
            <X size={18} />
          </button>
        </div>

        {sent ? (
          <div className="flex flex-col items-center gap-3 px-6 py-10 text-center">
            <CheckCircle2 size={40} className="text-emerald-600" />
            <p className="text-sm font-bold text-slate-900 dark:text-white">Permintaan sudah disiapkan</p>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Jendela WhatsApp sudah kebuka di tab baru, tinggal kirim pesannya ke tim kami.
            </p>
            <button type="button" onClick={onClose} className="mt-2 rounded-lg bg-emerald-600 px-5 py-2.5 text-sm font-bold text-white">
              Tutup
            </button>
          </div>
        ) : (
          <div className="grid gap-4 px-6 py-5">
            <Field label="Nama Travel" value={travelName} onChange={setTravelName} />
            <Field label="Nama PIC" value={picName} onChange={setPicName} />
            <Field label="Nomor WhatsApp" value={whatsapp} onChange={setWhatsapp} placeholder="08xxxxxxxxxx" />
            <Field label="Estimasi Jumlah Jamaah" value={pilgrimCount} onChange={setPilgrimCount} />
            <fieldset>
              <legend className="mb-2 text-sm font-bold text-slate-700 dark:text-slate-200">Modul yang diminati</legend>
              <div className="flex flex-wrap gap-2">
                {FEATURE_MODULES.map((m) => (
                  <button
                    key={m.title}
                    type="button"
                    onClick={() => toggleModule(m.title)}
                    className={
                      modules.includes(m.title)
                        ? "rounded-full bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white"
                        : "rounded-full border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"
                    }
                  >
                    {m.title}
                  </button>
                ))}
              </div>
            </fieldset>
            <button
              type="button"
              onClick={submit}
              className="mt-2 inline-flex items-center justify-center gap-2 rounded-lg bg-emerald-600 px-4 py-3 text-sm font-bold text-white hover:bg-emerald-700"
            >
              <MessageCircle size={16} />
              Kirim lewat WhatsApp
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-bold text-slate-700 dark:text-slate-200">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded-lg border border-slate-300 bg-white px-3.5 py-2.5 text-sm text-slate-900 outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100 dark:border-slate-700 dark:bg-slate-950 dark:text-white"
      />
    </label>
  );
}
