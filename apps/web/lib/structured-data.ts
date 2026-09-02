/**
 * JSON-LD for storefronts and articles.
 *
 * Everything here is emitted only when the underlying value genuinely exists.
 * Structured data that describes something the page does not show is a policy
 * violation, and the penalty falls on the travel agency's domain rather than
 * on ours — so an absent field is left absent instead of being filled with a
 * plausible-looking default.
 */

type Json = Record<string, unknown>;

function compact(value: Json): Json {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) =>
      entry !== undefined && entry !== null && entry !== "" &&
      !(Array.isArray(entry) && entry.length === 0),
    ),
  );
}

export interface AgencyInput {
  name: string;
  url: string;
  description?: string;
  logoUrl?: string;
  phone?: string;
  address?: string;
  city?: string;
  country?: string;
  licenseNumber?: string;
  sameAs?: string[];
}

/** The agency itself — what puts a business panel beside a search result. */
export function travelAgencyJsonLd(input: AgencyInput): Json {
  const address = compact({
    "@type": "PostalAddress",
    streetAddress: input.address,
    addressLocality: input.city,
    addressCountry: input.country || "ID",
  });
  return compact({
    "@context": "https://schema.org",
    "@type": "TravelAgency",
    name: input.name,
    url: input.url,
    description: input.description,
    logo: input.logoUrl,
    image: input.logoUrl,
    telephone: input.phone,
    // Only when it carries more than the country we defaulted in.
    address: Object.keys(address).length > 2 ? address : undefined,
    // The Kemenag licence is what makes an umrah agency legitimate, and it is
    // the first thing a careful pilgrim looks for.
    identifier: input.licenseNumber,
    sameAs: input.sameAs?.filter(Boolean),
  });
}

export interface ArticleInput {
  title: string;
  url: string;
  description?: string;
  imageUrl?: string;
  author?: string;
  publishedAt?: string;
  publisherName: string;
  publisherLogoUrl?: string;
}

export function articleJsonLd(input: ArticleInput): Json {
  return compact({
    "@context": "https://schema.org",
    "@type": "Article",
    headline: input.title.slice(0, 110),
    url: input.url,
    mainEntityOfPage: input.url,
    description: input.description,
    image: input.imageUrl,
    author: input.author ? { "@type": "Person", name: input.author } : { "@type": "Organization", name: input.publisherName },
    publisher: compact({
      "@type": "Organization",
      name: input.publisherName,
      logo: input.publisherLogoUrl ? { "@type": "ImageObject", url: input.publisherLogoUrl } : undefined,
    }),
    datePublished: input.publishedAt,
    // Absent rather than copied from datePublished. Claiming a page was
    // modified when it was not is exactly the kind of small untruth that
    // structured data is policed for.
    dateModified: undefined,
  });
}

export interface PackageInput {
  name: string;
  url: string;
  description?: string;
  imageUrl?: string;
  category?: string;
  /** Only set when a real number was entered and is shown on the page. */
  priceIDR?: number;
  brandName: string;
}

/**
 * A package.
 *
 * The Offer is attached only when a numeric price exists, because that number
 * is also what the page displays. A price parsed out of free text — "Mulai Rp
 * 28 juta" — would be a guess presented as a fact, and the fastest way to lose
 * rich results for every customer at once.
 */
export function packageJsonLd(input: PackageInput): Json {
  return compact({
    "@context": "https://schema.org",
    "@type": "Product",
    name: input.name,
    url: input.url,
    description: input.description,
    image: input.imageUrl,
    category: input.category,
    brand: { "@type": "Brand", name: input.brandName },
    offers: input.priceIDR
      ? {
          "@type": "Offer",
          price: String(input.priceIDR),
          priceCurrency: "IDR",
          url: input.url,
          // The listed figure is a starting price, and the page says so too.
          availability: "https://schema.org/InStock",
        }
      : undefined,
  });
}

/** Serialised for a <script type="application/ld+json">. */
export function jsonLdScript(data: Json | Json[]): string {
  // </script> inside a JSON string would end the tag early; escaping the slash
  // is the standard defence and changes nothing about the parsed value.
  return JSON.stringify(data).replace(/</g, "\\u003c");
}
