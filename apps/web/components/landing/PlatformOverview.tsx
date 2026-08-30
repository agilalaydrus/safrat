import Image from "next/image";
import { IconArrowRight } from "@tabler/icons-react";
import { CAPABILITIES } from "./content";

export default function PlatformOverview() {
  return (
    <section id="platform" className="landing-section landing-platform">
      <div className="landing-container">
        <header className="landing-section-heading">
          <p className="landing-eyebrow">Satu sumber data</p>
          <h2>Operasional tidak lagi terpecah di banyak tempat.</h2>
          <p>
            Setiap perubahan mengikuti jamaah yang sama, dari formulir pendaftaran sampai laporan setelah kepulangan.
          </p>
        </header>

        <div className="landing-platform-layout">
          <figure className="landing-editorial-photo landing-editorial-photo-tall">
            <Image
              src="/images/tenant-editorial/about_pilgrim_editorial_1787645090421.webp"
              alt="Jamaah Umrah menggunakan layanan digital travel"
              fill
              sizes="(max-width: 767px) 100vw, 42vw"
            />
          </figure>

          <div className="landing-capability-list">
            {CAPABILITIES.map(({ title, description, icon: Icon }) => (
              <article key={title}>
                <span className="landing-capability-icon"><Icon size={22} stroke={1.7} /></span>
                <div>
                  <h3>{title}</h3>
                  <p>{description}</p>
                </div>
                <IconArrowRight className="landing-capability-arrow" size={19} aria-hidden />
              </article>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
