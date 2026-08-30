import Image from "next/image";
import Link from "next/link";
import { IconArrowUpRight } from "@tabler/icons-react";

export default function Hero() {
  return (
    <section className="landing-hero" aria-labelledby="landing-hero-title">
      <Image
        src="/images/tawafiqhub-operations-hero.webp"
        alt="Tim operasional Umrah mendampingi jamaah di bandara"
        fill
        priority
        sizes="100vw"
        className="landing-hero-image"
      />
      <div className="landing-hero-shade" />
      <div className="landing-container landing-hero-inner">
        <div className="landing-hero-copy">
          <p className="landing-eyebrow">Platform operasional Umrah</p>
          <h1 id="landing-hero-title">Operasional Umrah. Satu kendali.</h1>
          <p className="landing-hero-lead">
            Hubungkan jamaah, tim, keberangkatan, layanan, dan keuangan dalam satu ruang kerja.
          </p>
          <div className="landing-hero-actions">
            <Link href="/sign-up" className="landing-button landing-button-light">
              Mulai gratis <IconArrowUpRight size={18} />
            </Link>
            <a href="#platform" className="landing-button landing-button-ghost">Lihat platform</a>
          </div>
        </div>
      </div>
    </section>
  );
}
