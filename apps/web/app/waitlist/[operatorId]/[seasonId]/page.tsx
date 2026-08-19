import PublicWaitlistForm from "@/components/waitlist/PublicWaitlistForm";
export default async function Page({ params }: { params: Promise<{ operatorId: string; seasonId: string }> }) {
  const { operatorId, seasonId } = await params;
  return <PublicWaitlistForm operatorId={operatorId} seasonId={seasonId} />;
}
