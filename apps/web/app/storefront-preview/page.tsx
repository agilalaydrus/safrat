"use client";

import { useEffect, useState } from "react";
import TenantStorefront, { type StorefrontProfile } from "@/components/storefront/TenantStorefront";
import { operatorClient } from "@/lib/rpc";

export default function StorefrontPreviewPage() {
  const [profile, setProfile] = useState<StorefrontProfile | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    operatorClient.getMyStorefront({}).then((editor) => {
      setProfile({
        operatorId: "preview",
        name: editor.operatorName,
        slug: editor.operatorSlug,
        licenseNumber: editor.licenseNumber,
        country: editor.country,
        content: editor.content as unknown as StorefrontProfile["content"],
        activeSeasons: editor.activeSeasons.map((season) => ({
          id: season.id,
          name: season.name,
          slug: season.slug,
          type: season.type,
          startDate: season.startDate?.toDate().toISOString(),
          endDate: season.endDate?.toDate().toISOString(),
          pilgrimCount: season.pilgrimCount,
        })),
      });
    }).catch((cause) => setError(cause instanceof Error ? cause.message : "Preview gagal dimuat."));
  }, []);

  if (error) return <main style={{ minHeight: "100dvh", padding: 32 }}><h1>Preview tidak tersedia</h1><p>{error}</p></main>;
  if (!profile) return <main style={{ minHeight: "100dvh", padding: 32 }}>Memuat preview draft...</main>;
  return <TenantStorefront profile={profile} preview />;
}
