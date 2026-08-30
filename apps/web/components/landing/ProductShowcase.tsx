import Image from "next/image";
import { PRODUCT_AREAS } from "./content";

export default function ProductShowcase() {
  const [operator, storefront, pilgrim, leader, agent, security] = PRODUCT_AREAS;
  const OperatorIcon = operator.icon;
  const StorefrontIcon = storefront.icon;
  const PilgrimIcon = pilgrim.icon;
  const LeaderIcon = leader.icon;
  const AgentIcon = agent.icon;
  const SecurityIcon = security.icon;

  return (
    <section id="solusi" className="landing-section landing-solutions">
      <div className="landing-container">
        <header className="landing-section-heading">
          <h2>Satu platform. Pengalaman berbeda untuk setiap peran.</h2>
          <p>Operator mendapat kendali penuh. Tim lapangan bergerak cepat. Jamaah tetap mendapat informasi yang dibutuhkan.</p>
        </header>

        <div className="landing-product-grid">
          <article className="landing-product-cell landing-product-operator">
            <OperatorIcon size={26} stroke={1.7} />
            <div><h3>{operator.title}</h3><p>{operator.description}</p></div>
          </article>

          <figure className="landing-product-photo landing-product-photo-main">
            <Image
              src="/images/tenant-editorial/muthowif_team_natural_1787650228839.webp"
              alt="Tim muthowif dan operasional Umrah berdiskusi"
              fill
              sizes="(max-width: 767px) 100vw, 48vw"
            />
          </figure>

          <article className="landing-product-cell landing-product-storefront">
            <StorefrontIcon size={25} stroke={1.7} />
            <div><h3>{storefront.title}</h3><p>{storefront.description}</p></div>
          </article>

          <article className="landing-product-cell landing-product-pilgrim">
            <PilgrimIcon size={25} stroke={1.7} />
            <div><h3>{pilgrim.title}</h3><p>{pilgrim.description}</p></div>
          </article>

          <article className="landing-product-cell landing-product-leader">
            <LeaderIcon size={25} stroke={1.7} />
            <div><h3>{leader.title}</h3><p>{leader.description}</p></div>
          </article>

          <figure className="landing-product-photo landing-product-photo-secondary">
            <Image
              src="/images/tenant-editorial/hotel_view_haram_1787650244269.webp"
              alt="Pemandangan area Masjidil Haram dari hotel jamaah"
              fill
              sizes="(max-width: 767px) 100vw, 32vw"
            />
          </figure>

          <article className="landing-product-cell landing-product-agent">
            <AgentIcon size={25} stroke={1.7} />
            <div><h3>{agent.title}</h3><p>{agent.description}</p></div>
          </article>

          <article className="landing-product-cell landing-product-security">
            <SecurityIcon size={25} stroke={1.7} />
            <div><h3>{security.title}</h3><p>{security.description}</p></div>
          </article>
        </div>
      </div>
    </section>
  );
}
