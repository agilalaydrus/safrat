import { CheckCircle2, XCircle } from "lucide-react";
import { PROBLEM_SOLUTION } from "./content";

export default function ProblemSolution() {
  return (
    <section className="bg-slate-100 px-5 py-20 dark:bg-slate-950">
      <div className="mx-auto max-w-5xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Sebelum dan Sesudah
        </p>
        <h2 className="mb-12 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Yang Bikin Musim Kemarin Pusing
        </h2>

        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div className="rounded-2xl border border-rose-200 bg-rose-50/70 p-6 dark:border-rose-900 dark:bg-rose-950/30">
            <p className="mb-4 flex items-center gap-2 text-sm font-bold text-rose-700 dark:text-rose-300">
              <XCircle size={17} />
              Cara Lama, Spreadsheet dan Grup WA
            </p>
            <ul className="space-y-3">
              {PROBLEM_SOLUTION.map((row) => (
                <li key={row.problem} className="rounded-lg border border-rose-200 bg-white/70 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-slate-900/40 dark:text-rose-200">
                  {row.problem}
                </li>
              ))}
            </ul>
          </div>

          <div className="rounded-2xl border border-emerald-200 bg-emerald-50/70 p-6 dark:border-emerald-900 dark:bg-emerald-950/20">
            <p className="mb-4 flex items-center gap-2 text-sm font-bold text-emerald-700 dark:text-emerald-300">
              <CheckCircle2 size={17} />
              Dengan Tawafiq Hub
            </p>
            <ul className="space-y-3">
              {PROBLEM_SOLUTION.map((row) => (
                <li key={row.solution} className="rounded-lg border border-emerald-200 bg-white/70 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-slate-900/40 dark:text-emerald-200">
                  {row.solution}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
