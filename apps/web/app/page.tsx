import type { Metadata } from "next";
import { ThemeProvider } from "@/components/landing/ThemeProvider";
import Navbar from "@/components/landing/Navbar";
import Hero from "@/components/landing/Hero";
import PlatformOverview from "@/components/landing/PlatformOverview";
import ProductShowcase from "@/components/landing/ProductShowcase";
import Workflow from "@/components/landing/Workflow";
import Pricing from "@/components/landing/Pricing";
import Faq from "@/components/landing/Faq";
import CtaAndFooter from "@/components/landing/CtaAndFooter";

export const metadata: Metadata = {
  title: "Platform Operasional Umrah untuk PPIU",
  description:
    "Kelola jamaah, kloter, operasional lapangan, keuangan, agen, dan storefront travel dalam satu platform TawafiqHub.",
  openGraph: {
    title: "TawafiqHub | Platform Operasional Umrah untuk PPIU",
    description:
      "Satu ruang kerja untuk tim travel, tour leader, agen, dan jamaah dari pendaftaran sampai kepulangan.",
    type: "website",
    locale: "id_ID",
  },
};

const structuredData = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "TawafiqHub",
  applicationCategory: "BusinessApplication",
  operatingSystem: "Web",
  description:
    "Platform operasional Umrah untuk PPIU yang menghubungkan pengelolaan jamaah, keberangkatan, layanan, dan keuangan.",
  offers: {
    "@type": "AggregateOffer",
    lowPrice: "589000",
    highPrice: "2489000",
    priceCurrency: "IDR",
  },
};

export default function LandingPage() {
  return (
    <ThemeProvider>
      <div className="landing-scope landing-v2">
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
        />
        <Navbar />
        <main>
          <Hero />
          <PlatformOverview />
          <ProductShowcase />
          <Workflow />
          <Pricing />
          <Faq />
        </main>
        <CtaAndFooter />
      </div>
    </ThemeProvider>
  );
}
