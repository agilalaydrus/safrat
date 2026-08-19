import PublicRegistrationForm from "@/components/registrations/PublicRegistrationForm";
export default async function Page({ params }: { params: Promise<{ operatorId: string; seasonId: string }> }) {
  const { operatorId, seasonId } = await params;
  return <PublicRegistrationForm operatorId={operatorId} seasonId={seasonId} />;
}
