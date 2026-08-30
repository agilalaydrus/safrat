"use client";

import { useState } from "react";
import { IconMinus, IconPlus } from "@tabler/icons-react";
import { FAQ_ITEMS } from "./content";

export default function Faq() {
  const [openFaq, setOpenFaq] = useState<number | null>(0);

  return (
    <section id="faq" className="landing-section landing-faq">
      <div className="landing-container landing-faq-layout">
        <header className="landing-section-heading">
          <h2>Pertanyaan yang sering diajukan.</h2>
          <p>Belum menemukan jawaban yang Anda cari? Tim kami dapat membantu melalui WhatsApp.</p>
        </header>

        <div className="landing-faq-list">
          {FAQ_ITEMS.map((item, index) => {
            const isOpen = openFaq === index;
            return (
              <article key={item.q} data-open={isOpen}>
                <button type="button" onClick={() => setOpenFaq(isOpen ? null : index)} aria-expanded={isOpen}>
                  <span>{item.q}</span>
                  {isOpen ? <IconMinus size={20} /> : <IconPlus size={20} />}
                </button>
                {isOpen && <p>{item.a}</p>}
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
