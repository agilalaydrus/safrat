/* eslint-disable @next/next/no-img-element */
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Link from "next/link";
import { articleJsonLd, jsonLdScript } from "@/lib/structured-data";

type Article = { title: string; slug: string; body: string; excerpt?: string; coverImageUrl?: string; altText?: string; author?: string; publishedAt?: string; seoTitle?: string; seoDescription?: string };
type Profile = { name: string; slug: string; logoUrl?: string; canonicalHost?: string; content?: { displayName?: string; logoUrl?: string; blogPosts?: Article[]; seoTitle?: string; seoDescription?: string } };

async function getProfile(slug: string): Promise<Profile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, { method: "POST", headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" }, body: JSON.stringify({ slug }), cache: "no-store" });
    return response.ok ? await response.json() as Profile : null;
  } catch { return null; }
}

/**
 * The address search engines should treat as authoritative: the operator's own
 * verified domain when they have one, otherwise their platform subdomain.
 *
 * Without this an article is reachable at two hostnames and counted as
 * duplicated, which splits whatever ranking it earns between them.
 */
function canonicalUrl(canonicalHost: string | undefined, slug: string, path: string): string | undefined {
  if (canonicalHost) return `https://${canonicalHost}${path}`;
  const base = process.env.NEXT_PUBLIC_TENANT_BASE_HOST;
  return base ? `https://${slug}.${base}${path}` : undefined;
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string; articleSlug: string }> }): Promise<Metadata> {
  const { slug, articleSlug } = await params;
  const profile = await getProfile(slug);
  const article = profile?.content?.blogPosts?.find((item) => item.slug === articleSlug);
  if (!profile || !article) return { title: "Artikel tidak ditemukan" };
  return { title: { absolute: article.seoTitle || `${article.title} | ${profile.content?.displayName || profile.name}` }, description: article.seoDescription || article.excerpt, openGraph: article.coverImageUrl ? { images: [{ url: article.coverImageUrl }] } : undefined, alternates: { canonical: canonicalUrl(profile.canonicalHost, profile.slug, `/blog/${article.slug}`) } };
}

export default async function BlogArticlePage({ params }: { params: Promise<{ slug: string; articleSlug: string }> }) {
  const { slug, articleSlug } = await params;
  const profile = await getProfile(slug);
  const article = profile?.content?.blogPosts?.find((item) => item.slug === articleSlug);
  if (!profile || !article) notFound();
  const name = profile.content?.displayName || profile.name;
  const published = formatPublishedDate(article.publishedAt);
  const base = canonicalUrl(profile.canonicalHost, profile.slug, `/blog/${article.slug}`);
  const jsonLd = articleJsonLd({
    title: article.title,
    // Anchored on the canonical address so the markup names the same page the
    // canonical tag does, not whichever host served this request.
    url: base ?? "",
    description: article.seoDescription || article.excerpt,
    imageUrl: article.coverImageUrl,
    author: article.author,
    publishedAt: article.publishedAt,
    publisherName: name,
    publisherLogoUrl: profile.content?.logoUrl || profile.logoUrl,
  });
  return <main className="tenant-scope min-h-[100dvh]"><script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLdScript(jsonLd) }} /><article className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:py-24"><Link href="/" className="tenant-package-link">← Kembali ke {name}</Link>{article.coverImageUrl && <img src={article.coverImageUrl} alt={article.altText || article.title} className="mt-8 aspect-[16/8] w-full rounded-2xl object-cover" /> }<p className="mt-10 text-xs font-semibold uppercase tracking-[0.14em] text-emerald-700">Blog {name}</p><h1 className="mt-4 text-4xl font-black tracking-tight text-slate-950 sm:text-5xl dark:text-slate-100">{article.title}</h1><p className="mt-4 text-sm text-slate-500 dark:text-slate-400">{[article.author || name, published].filter(Boolean).join(" · ")}</p>{article.excerpt && <p className="mt-6 text-lg leading-8 text-slate-600 dark:text-slate-300">{article.excerpt}</p>}<div className="mt-10 whitespace-pre-line text-base leading-8 text-slate-700 dark:text-slate-200">{article.body}</div></article></main>;
}

function formatPublishedDate(value?: string) { if (!value) return ""; const date = new Date(value); return Number.isNaN(date.getTime()) ? "" : date.toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" }); }
