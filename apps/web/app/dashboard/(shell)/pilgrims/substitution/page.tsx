import SubstitutionHistory from "@/components/pilgrims/SubstitutionHistory";
export default async function Page({ searchParams }: { searchParams: Promise<{ seasonId?: string }> }) {
  const { seasonId } = await searchParams;
  return <SubstitutionHistory initialSeasonId={seasonId} />;
}
