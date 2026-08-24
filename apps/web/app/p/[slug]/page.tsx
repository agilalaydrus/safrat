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
  return {
    title: `${name} | Travel Umrah & Haji`,
    description: profile.content?.heroSubtitle || profile.heroSubtitle || profile.content?.description || profile.description || `Paket Umrah dan Haji dari ${name}.`,
    alternates: process.env.NEXT_PUBLIC_APP_URL ? { canonical: buildTenantLinkFromBase(profile.slug, "/", process.env.NEXT_PUBLIC_APP_URL) } : undefined,
  };
}

export default async function OperatorPublicProfile({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) notFound();
  return <TenantStorefront profile={profile} />;
}
