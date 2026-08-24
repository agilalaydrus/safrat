import { TESTIMONIALS } from "./content";

export default function Testimonials() {
  return (
    <section className="border-t border-slate-200 bg-white py-20 dark:border-slate-800 dark:bg-slate-950">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto mb-12 max-w-3xl text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Kisah Sukses Operator
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-4xl dark:text-slate-100">
            Dipercaya Direksi &amp; Tim Operasional PPIU
          </h3>
        </div>

        <div className="mx-auto grid max-w-6xl grid-cols-1 gap-6 md:grid-cols-3">
          {TESTIMONIALS.map((t) => (
            <div
              key={t.name}
              className="flex flex-col justify-between rounded-3xl border border-slate-200 bg-slate-50 p-7 dark:border-slate-800 dark:bg-slate-900"
            >
              <p className="mb-6 text-sm italic leading-relaxed text-slate-600 dark:text-slate-300">&ldquo;{t.quote}&rdquo;</p>
              <div className="border-t border-slate-200 pt-4 dark:border-slate-800">
                <h5 className="text-sm font-bold text-slate-900 dark:text-white">{t.name}</h5>
                <p className="text-xs text-slate-500 dark:text-slate-400">{t.role}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
