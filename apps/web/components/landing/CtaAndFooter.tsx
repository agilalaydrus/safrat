import Link from "next/link";
import {
  IconArrowUpRight,
  IconBrandWhatsapp,
  IconCompass,
  IconMail,
  IconMapPin,
} from "@tabler/icons-react";
import { NAV_LINKS, WA_NUMBER_DISPLAY, WA_SALES_LINK } from "./content";

export default function CtaAndFooter() {
  return (
    <>
      <section className="landing-final-cta" aria-labelledby="landing-final-title">
        <div className="landing-container landing-final-inner">
          <div>
            <h2 id="landing-final-title">Bangun satu ruang kerja untuk seluruh tim travel.</h2>
          </div>
          <div className="landing-final-actions">
            <Link href="/sign-up" className="landing-button landing-button-light">Mulai gratis <IconArrowUpRight size={18} /></Link>
            <a href={WA_SALES_LINK} target="_blank" rel="noreferrer" className="landing-button landing-button-ghost"><IconBrandWhatsapp size={18} /> Bicara dengan tim</a>
          </div>
        </div>
      </section>

      <footer className="landing-footer">
        <div className="landing-container">
          <div className="landing-footer-main">
            <div className="landing-footer-brand">
              <span className="landing-brand-mark"><IconCompass size={22} /></span>
              <h2>TawafiqHub</h2>
              <p>Platform operasional Umrah untuk travel, tim lapangan, agen, dan jamaah.</p>
            </div>

            <nav aria-label="Tautan footer">
              <h3>Jelajahi</h3>
              {NAV_LINKS.map(([label, href]) => <a key={href} href={href}>{label}</a>)}
            </nav>

            <div className="landing-footer-contact">
              <h3>Hubungi kami</h3>
              <a href="mailto:business@tawafiqhub.id"><IconMail size={17} /> business@tawafiqhub.id</a>
              <a href={WA_SALES_LINK} target="_blank" rel="noreferrer"><IconBrandWhatsapp size={17} /> {WA_NUMBER_DISPLAY}</a>
              <span><IconMapPin size={17} /> DKI Jakarta dan Kota Bekasi</span>
            </div>
          </div>

          <div className="landing-footer-bottom">
            <p>© {new Date().getFullYear()} TawafiqHub. Hak cipta dilindungi.</p>
            <div><span>Kebijakan Privasi</span><span>Syarat dan Ketentuan</span></div>
          </div>
        </div>
      </footer>

      <a href={WA_SALES_LINK} target="_blank" rel="noreferrer" aria-label="Hubungi TawafiqHub melalui WhatsApp" className="landing-whatsapp">
        <IconBrandWhatsapp size={23} />
      </a>
    </>
  );
}
