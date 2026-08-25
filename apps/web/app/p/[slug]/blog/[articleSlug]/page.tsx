/* eslint-disable @next/next/no-img-element */
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Link from "next/link";

type Article = { title: string; slug: string; body: string; excerpt?: string; coverImageUrl?: string; altText?: string; author?: string; seoTitle?: string; seoDescription?: string };
type Profile = { name: string; slug: string; content?: { displayName?: string; blogPosts?: Article[]; seoTitle?: string; seoDescription?: string } };

async function getProfile(slug: string): Promise<Profile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, { method: "POST", headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" }, body: JSON.stringify({ slug }), cache: "no-store" });
    return response.ok ? await response.json() as Profile : null;
  } catch { return null; }
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string; articleSlug: string }> }): Promise<Metadata> {
  const { slug, articleSlug } = await params;
  const profile = await getProfile(slug);
  const article = profile?.content?.blogPosts?.find((item) => item.slug === articleSlug);
  if (!profile || !article) return { title: "Artikel tidak ditemukan" };
  return { title: article.seoTitle || `${article.title} | ${profile.content?.displayName || profile.name}`, description: article.seoDescription || article.excerpt, openGraph: article.coverImageUrl ? { images: [{ url: article.coverImageUrl }] } : undefined };
}

export default async function BlogArticlePage({ params }: { params: Promise<{ slug: string; articleSlug: string }> }) {
  const { slug, articleSlug } = await params;
  const profile = await getProfile(slug);
  const article = profile?.content?.blogPosts?.find((item) => item.slug === articleSlug);
  if (!profile || !article) notFound();
  const name = profile.content?.displayName || profile.name;
  return <main className="tenant-scope min-h-[100dvh]"><article className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:py-24"><Link href="/" className="tenant-package-link">← Kembali ke {name}</Link>{article.coverImageUrl && <img src={article.coverImageUrl} alt={article.altText || article.title} className="mt-8 aspect-[16/8] w-full rounded-2xl object-cover" /> }<p className="mt-10 text-xs font-semibold uppercase tracking-[0.14em] text-emerald-700">Blog {name}</p><h1 className="mt-4 text-4xl font-black tracking-tight text-slate-950 sm:text-5xl dark:text-slate-100">{article.title}</h1>{article.excerpt && <p className="mt-6 text-lg leading-8 text-slate-600 dark:text-slate-300">{article.excerpt}</p>}<div className="mt-10 whitespace-pre-line text-base leading-8 text-slate-700 dark:text-slate-200">{article.body}</div></article></main>;
}
