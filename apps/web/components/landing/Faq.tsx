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
          <span className="mb-3 inline-block rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-bold uppercase tracking-wider text-amber-800 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
            Frequently Asked Questions
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 dark:text-slate-100">
            Pertanyaan yang Sering Diajukan <span className="landing-gold-text">PPIU</span>
          </h3>
        </div>

        <div className="space-y-4">
          {FAQ_ITEMS.map((item, idx) => (
            <div
              key={item.q}
              className={`overflow-hidden rounded-2xl border bg-white shadow-sm transition-colors dark:bg-slate-950 ${
                openFaq === idx
                  ? "border-amber-200 dark:border-amber-800/60"
                  : "border-slate-200 dark:border-slate-800"
              }`}
            >
              <button
                type="button"
                onClick={() => setOpenFaq(openFaq === idx ? null : idx)}
                aria-expanded={openFaq === idx}
                className={`flex w-full items-center justify-between gap-4 p-5 text-left text-sm font-bold transition-colors sm:text-base ${
                  openFaq === idx
                    ? "bg-amber-50/60 text-amber-900 dark:bg-amber-950/20 dark:text-amber-200"
                    : "text-slate-900 hover:text-amber-800 dark:text-slate-200 dark:hover:text-amber-300"
                }`}
              >
                <span>{item.q}</span>
                {openFaq === idx ? (
                  <ChevronUp className="h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-300" />
                ) : (
                  <ChevronDown className="h-5 w-5 flex-shrink-0 text-slate-400 dark:text-slate-500" />
                )}
              </button>
              {openFaq === idx && (
                <div className="border-t border-amber-100 px-5 pb-5 pt-3 text-xs leading-relaxed text-slate-600 sm:text-sm dark:border-amber-900/40 dark:text-slate-400">
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
