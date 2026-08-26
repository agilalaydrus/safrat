import type { Metadata } from "next";
import { notFound } from "next/navigation";
import TenantStorefront, { type StorefrontProfile } from "@/components/storefront/TenantStorefront";
import { buildTenantLinkFromBase } from "@/lib/tenant-link";

async function getProfile(slug: string): Promise<StorefrontProfile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ slug }),
      cache: "no-store",
    });
    if (!response.ok) return null;
    return (await response.json()) as StorefrontProfile;
  } catch {
    return null;
  }
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
    alternates: process.env.NEXT_PUBLIC_APP_URL ? { canonical: buildTenantLinkFromBase(profile.slug, "/", process.env.NEXT_PUBLIC_APP_URL) } : undefined,
  };
}

export default async function OperatorPublicProfile({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) notFound();
  return <TenantStorefront profile={profile} />;
}
