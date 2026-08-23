"use client";

import { useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { FAQ_ITEMS } from "./content";

export default function Faq() {
  const [openFaq, setOpenFaq] = useState<number | null>(0);

  return (
    <section id="faq" className="border-t border-slate-200 bg-slate-50 py-20 dark:border-slate-800 dark:bg-slate-900">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <div className="mb-12 text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Frequently Asked Questions
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white">
            Pertanyaan yang Sering Diajukan PPIU
          </h3>
        </div>

        <div className="space-y-4">
          {FAQ_ITEMS.map((item, idx) => (
            <div
              key={item.q}
              className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-950"
            >
              <button
                type="button"
                onClick={() => setOpenFaq(openFaq === idx ? null : idx)}
                aria-expanded={openFaq === idx}
                className="flex w-full items-center justify-between p-5 text-left text-sm font-bold text-slate-900 transition-colors hover:text-emerald-700 sm:text-base dark:text-white dark:hover:text-emerald-400"
              >
                <span>{item.q}</span>
                {openFaq === idx ? (
                  <ChevronUp className="h-5 w-5 flex-shrink-0 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <ChevronDown className="h-5 w-5 flex-shrink-0 text-slate-400" />
                )}
              </button>
              {openFaq === idx && (
                <div className="border-t border-slate-100 px-5 pb-5 pt-3 text-xs leading-relaxed text-slate-600 sm:text-sm dark:border-slate-800 dark:text-slate-300">
                  {item.a}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
