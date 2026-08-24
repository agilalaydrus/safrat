import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { buildTenantLinkFromBase } from "@/lib/tenant-link";

// Connect JSON response (camelCase proto JSON names). Timestamps are RFC3339
// strings. Fetched server-side from the public GetPublicProfile RPC.
type PublicSeason = {
  id: string;
  name: string;
  slug: string;
  type: string;
  startDate?: string;
  endDate?: string;
  pilgrimCount?: number;
};

type PublicProfile = {
  operatorId: string;
  name: string;
  slug: string;
  logoUrl?: string;
  description?: string;
  whatsappNumber?: string;
  website?: string;
  address?: string;
  city?: string;
  licenseNumber?: string;
  country?: string;
  activeSeasons?: PublicSeason[];
};

async function getProfile(slug: string): Promise<PublicProfile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ slug }),
      // Public marketing page — data changes rarely; revalidate keeps it
      // fast and lets Next dedupe the generateMetadata + page fetches.
      next: { revalidate: 120 },
    });
    if (!response.ok) return null;
    return (await response.json()) as PublicProfile;
  } catch {
    return null;
  }
}

function waLink(raw: string): string {
  const digits = raw.replace(/\D/g, "");
  const normalized = digits.startsWith("0") ? `62${digits.slice(1)}` : digits;
  return `https://wa.me/${normalized}`;
}

function formatMonthYear(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("id-ID", { month: "short", year: "numeric" });
}

const SEASON_LABEL: Record<string, string> = {
  HAJJ: "Haji",
  UMRAH_REGULER: "Umrah Reguler",
  UMRAH_RAJAB: "Umrah Rajab",
  UMRAH_RAMADHAN: "Umrah Ramadhan",
  UMRAH_SYAWAL: "Umrah Syawal",
  UMRAH_DZULQAIDAH: "Umrah Dzulqaidah",
};

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) return { title: "Operator tidak ditemukan" };
  return {
    title: `${profile.name} — Travel Umrah & Haji`,
    description: profile.description || `Paket Umrah & Haji dari ${profile.name}.`,
    alternates: process.env.NEXT_PUBLIC_APP_URL
      ? { canonical: buildTenantLinkFromBase(profile.slug, "/", process.env.NEXT_PUBLIC_APP_URL) }
      : undefined,
  };
}

export default async function OperatorPublicProfile({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) notFound();

  const seasons = profile.activeSeasons ?? [];
  const initials = profile.name.slice(0, 2).toUpperCase();

  return (
    <main className="min-h-screen bg-slate-50 pb-16 font-sans text-slate-900">
      {/* Header */}
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-4xl items-center gap-5 px-6 py-10">
          {profile.logoUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={profile.logoUrl} alt={profile.name} className="h-20 w-20 rounded-2xl border border-slate-200 object-cover" />
          ) : (
            <div className="flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-tr from-emerald-600 to-teal-500 text-2xl font-black text-white">
              {initials}
            </div>
          )}
          <div>
            <h1 className="text-2xl font-extrabold tracking-tight text-slate-900">{profile.name}</h1>
            <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-slate-500">
              <span className="font-semibold text-emerald-700">★ Terpercaya</span>
              {profile.city && <span>· {profile.city}</span>}
              {profile.country && <span>· {profile.country}</span>}
            </p>
            {profile.licenseNumber && (
              <p className="mt-1 text-xs text-slate-400">Nomor Izin: {profile.licenseNumber}</p>
            )}
          </div>
        </div>
      </header>

      <div className="mx-auto max-w-4xl space-y-8 px-6 pt-8">
        {/* About */}
        {profile.description && (
          <section className="rounded-2xl border border-slate-200 bg-white p-6">
            <h2 className="mb-2 text-sm font-bold uppercase tracking-wider text-emerald-700">Tentang Kami</h2>
            <p className="text-sm leading-relaxed text-slate-600">{profile.description}</p>
          </section>
        )}

        {/* Packages */}
        <section>
          <h2 className="mb-4 text-lg font-extrabold text-slate-900">Paket Tersedia</h2>
          {seasons.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
              Belum ada paket tersedia saat ini.
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {seasons.map((season) => (
                <div key={season.id} className="flex flex-col justify-between rounded-2xl border border-slate-200 bg-white p-5">
                  <div>
                    <span className="inline-block rounded-full bg-emerald-100 px-2.5 py-0.5 text-[11px] font-bold text-emerald-800">
                      {SEASON_LABEL[season.type] ?? season.type}
                    </span>
                    <h3 className="mt-2 text-base font-bold text-slate-900">{season.name}</h3>
                    <p className="mt-1 text-xs text-slate-500">
                      {formatMonthYear(season.startDate)}
                      {season.endDate ? ` – ${formatMonthYear(season.endDate)}` : ""}
                    </p>
                    {typeof season.pilgrimCount === "number" && season.pilgrimCount > 0 && (
                      <p className="mt-1 text-xs text-slate-400">{season.pilgrimCount} jamaah terdaftar</p>
                    )}
                  </div>
                  <Link
                    href={`/register/${season.slug}`}
                    className="mt-4 block rounded-xl bg-emerald-600 py-2.5 text-center text-xs font-bold text-white transition-colors hover:bg-emerald-700"
                  >
                    Daftar Sekarang
                  </Link>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Contact */}
        {(profile.whatsappNumber || profile.website) && (
          <section className="rounded-2xl border border-slate-200 bg-white p-6">
            <h2 className="mb-4 text-sm font-bold uppercase tracking-wider text-emerald-700">📱 Hubungi Kami</h2>
            <div className="flex flex-wrap gap-3">
              {profile.whatsappNumber && (
                <a
                  href={waLink(profile.whatsappNumber)}
                  target="_blank"
                  rel="noreferrer"
                  className="rounded-xl bg-emerald-600 px-5 py-2.5 text-sm font-bold text-white transition-colors hover:bg-emerald-700"
                >
                  WhatsApp
                </a>
              )}
              {profile.website && (
                <a
                  href={profile.website}
                  target="_blank"
                  rel="noreferrer"
                  className="rounded-xl border border-slate-300 bg-white px-5 py-2.5 text-sm font-bold text-slate-700 transition-colors hover:bg-slate-50"
                >
                  Website
                </a>
              )}
            </div>
            {profile.address && <p className="mt-4 text-xs text-slate-500">{profile.address}</p>}
          </section>
        )}

        <p className="pt-4 text-center text-xs text-slate-400">
          Ditenagai oleh <span className="font-bold text-emerald-700">TawafiqHub</span>
        </p>
      </div>
    </main>
  );
}
