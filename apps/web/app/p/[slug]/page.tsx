import type { Metadata } from "next";
import { notFound } from "next/navigation";
import TenantStorefront, { packagePriceAmount } from "@/components/storefront/TenantStorefront";
import { jsonLdScript, packageJsonLd, travelAgencyJsonLd } from "@/lib/structured-data";
import { buildTenantLinkFromBase } from "@/lib/tenant-link";
import { getPublicProfile as getProfile } from "@/lib/public-profile";

/**
 * The address search engines should treat as authoritative: the operator's own
 * verified domain when they have one, otherwise their platform subdomain.
 */
function canonicalUrl(canonicalHost: string | undefined, slug: string, path: string): string | undefined {
  if (canonicalHost) return `https://${canonicalHost}${path}`;
  const base = process.env.NEXT_PUBLIC_APP_URL;
  return base ? buildTenantLinkFromBase(slug, path, base) : undefined;
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) return { title: "Travel tidak ditemukan" };
  const name = profile.content?.displayName || profile.name;
  const seoTitle = profile.content?.seoTitle || `${name} | Travel Umrah & Haji`;
  const seoDescription = profile.content?.seoDescription || profile.content?.heroSubtitle || profile.heroSubtitle || profile.content?.description || profile.description || `Paket Umrah dan Haji dari ${name}.`;
  return {
    // absolute: the root layout appends "| TawafiqHub" to every title, which
    // would put the platform's brand in a client's search results.
    title: { absolute: seoTitle },
    description: seoDescription,
    openGraph: { title: seoTitle, description: seoDescription, images: profile.content?.ogImageUrl ? [{ url: profile.content.ogImageUrl }] : undefined },
    // Point search engines at the operator's own domain once they have one.
    // Leaving the canonical on the platform subdomain would mean a client who
    // bought a domain still gets indexed under ours — the opposite of what
    // they paid for — and the two addresses would compete as duplicates.
    alternates: { canonical: canonicalUrl(profile.canonicalHost, profile.slug, "/") },
  };
}

export default async function OperatorPublicProfile({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) notFound();

  // Anchored on the canonical address, so the markup names the same page the
  // canonical tag points at rather than whichever host served this request.
  const base = canonicalUrl(profile.canonicalHost, profile.slug, "") ?? "";
  const content = profile.content;
  const graph: object[] = [
    travelAgencyJsonLd({
      name: content?.displayName || profile.name,
      url: base || "/",
      description: content?.seoDescription || content?.description || profile.description,
      logoUrl: content?.logoUrl || profile.logoUrl,
      phone: content?.whatsappNumber || profile.whatsappNumber,
      address: content?.address || profile.address,
      city: content?.city || profile.city,
      country: profile.country,
      licenseNumber: profile.licenseNumber,
      sameAs: content?.socialLinks?.map((link) => link.url).filter(Boolean),
    }),
  ];
  for (const item of content?.publicPackages ?? []) {
    graph.push(
      packageJsonLd({
        name: item.title,
        url: base ? `${base}/` : "/",
        description: item.summary,
        imageUrl: item.imageUrl,
        category: item.category,
        // Only when a real number exists — never parsed out of the label.
        priceIDR: packagePriceAmount(item),
        brandName: content?.displayName || profile.name,
      }),
    );
  }

  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLdScript(graph as never) }} />
      <TenantStorefront profile={profile} />
    </>
  );
}
