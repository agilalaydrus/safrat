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
              data-open={openFaq === idx}
              className="landing-faq-card overflow-hidden rounded-2xl border shadow-sm transition-colors"
            >
              <button
                type="button"
                onClick={() => setOpenFaq(openFaq === idx ? null : idx)}
                aria-expanded={openFaq === idx}
                className="landing-faq-question flex w-full items-center justify-between gap-4 p-5 text-left text-sm font-bold transition-colors sm:text-base"
              >
                <span>{item.q}</span>
                {openFaq === idx ? (
                  <ChevronUp className="landing-faq-chevron landing-faq-chevron-open h-5 w-5 flex-shrink-0" />
                ) : (
                  <ChevronDown className="landing-faq-chevron h-5 w-5 flex-shrink-0" />
                )}
              </button>
              {openFaq === idx && (
                <div className="landing-faq-answer border-t px-5 pb-5 pt-3 text-xs leading-relaxed sm:text-sm">
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
