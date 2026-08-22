import { ILLUSTRATIVE_TESTIMONIALS } from "./content";

export default function Testimonials() {
  return (
    <section className="bg-slate-100 px-5 py-20 dark:bg-slate-950">
      <div className="mx-auto max-w-6xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Testimoni
        </p>
        <h2 className="mb-3 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Contoh Skenario Penggunaan
        </h2>
        <p className="mx-auto mb-10 max-w-2xl text-center text-sm text-slate-500 dark:text-slate-400">
          Tawafiq Hub baru memasuki musim pertamanya. Tiga kartu di bawah ini adalah{" "}
          <strong className="text-slate-700 dark:text-slate-200">contoh ilustratif</strong>, bukan kutipan klien nyata, untuk
          menggambarkan siapa yang biasanya memakai tiap modul. Akan diganti dengan testimoni asli begitu musim pertama
          selesai berjalan.
        </p>

        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {ILLUSTRATIVE_TESTIMONIALS.map((t) => (
            <div key={t.name} className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-700 dark:bg-slate-900">
              <span className="mb-4 inline-block rounded-full border border-amber-300 bg-amber-100 px-3 py-1 text-[10px] font-bold text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-300">
                Contoh Ilustrasi
              </span>
              <p className="mb-5 text-sm leading-relaxed text-slate-700 dark:text-slate-200">{t.quote}</p>
              <p className="text-sm font-bold text-slate-900 dark:text-white">{t.name}</p>
              <p className="text-xs text-slate-400">{t.role}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
