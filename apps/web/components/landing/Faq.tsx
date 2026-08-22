"use client";

import { useState } from "react";
import { Minus, Plus } from "lucide-react";
import { FAQ_ITEMS } from "./content";

export default function Faq() {
  const [open, setOpen] = useState<number | null>(0);

  return (
    <section id="faq" className="bg-white px-5 py-20 dark:bg-slate-900">
      <div className="mx-auto max-w-2xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">FAQ</p>
        <h2 className="mb-10 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Pertanyaan yang Sering Ditanyakan
        </h2>

        <div className="space-y-3">
          {FAQ_ITEMS.map((item, i) => {
            const isOpen = open === i;
            return (
              <div key={item.q} className="overflow-hidden rounded-xl border border-slate-200 dark:border-slate-700">
                <button
                  type="button"
                  onClick={() => setOpen(isOpen ? null : i)}
                  aria-expanded={isOpen}
                  className="flex w-full items-center justify-between gap-3 px-5 py-4 text-left text-sm font-bold text-slate-900 dark:text-white"
                >
                  {item.q}
                  {isOpen ? <Minus size={16} className="shrink-0 text-emerald-600" /> : <Plus size={16} className="shrink-0 text-slate-400" />}
                </button>
                {isOpen && <p className="px-5 pb-5 text-sm leading-relaxed text-slate-600 dark:text-slate-300">{item.a}</p>}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
