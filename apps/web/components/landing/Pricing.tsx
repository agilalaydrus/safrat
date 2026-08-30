import Link from "next/link";
import { IconCheck, IconMessageCircle } from "@tabler/icons-react";
import { PRICING_TIERS, WA_SALES_LINK } from "./content";

function PriceAction({ contactSales, label }: { contactSales?: boolean; label: string }) {
  return contactSales ? (
    <a href={WA_SALES_LINK} target="_blank" rel="noreferrer" className="landing-button landing-button-secondary">
      <IconMessageCircle size={18} /> {label}
    </a>
  ) : (
    <Link href="/sign-up" className="landing-button landing-button-primary">{label}</Link>
  );
}

export default function Pricing() {
  const [starter, growth, custom] = PRICING_TIERS;

  return (
    <section id="harga" className="landing-section landing-pricing">
      <div className="landing-container">
        <header className="landing-section-heading">
          <p className="landing-eyebrow">Harga yang jelas</p>
          <h2>Mulai sesuai skala travel Anda.</h2>
          <p>Setiap paket mencakup fondasi operasional. Pilih identitas domain dan lingkungan server yang dibutuhkan.</p>
        </header>

        <div className="landing-pricing-layout">
          <article className="landing-price-featured">
            <div className="landing-price-head">
              <div>
                <span>Rekomendasi</span>
                <h3>{growth.name}</h3>
                <p>{growth.blurb}</p>
              </div>
              <div className="landing-price-amount"><strong>{growth.price}</strong><span>/{growth.unit}</span></div>
            </div>
            <div className="landing-price-body">
              <ul>
                {growth.features.map((feature) => <li key={feature}><IconCheck size={18} />{feature}</li>)}
              </ul>
              <PriceAction label={growth.cta} />
            </div>
          </article>

          <div className="landing-price-side">
            {[starter, custom].map((tier) => (
              <article key={tier.name}>
                <div>
                  <h3>{tier.name}</h3>
                  <p>{tier.blurb}</p>
                </div>
                <div className="landing-price-side-bottom">
                  <div className="landing-price-amount"><strong>{tier.price}</strong><span>/{tier.unit}</span></div>
                  <PriceAction contactSales={tier.contactSales} label={tier.cta} />
                </div>
              </article>
            ))}
          </div>
        </div>
        <p className="landing-pricing-note">Harga belum termasuk biaya domain, layanan pihak ketiga, atau kebutuhan integrasi khusus.</p>
      </div>
    </section>
  );
}
