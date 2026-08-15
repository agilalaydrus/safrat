import ApplyAsAgentForm from "@/components/agents/ApplyAsAgentForm";
export default async function Page({ params, searchParams }: { params: Promise<{ operatorId: string }>; searchParams: Promise<{ ref?: string }> }) {
  const { operatorId } = await params;
  const { ref } = await searchParams;
  return <ApplyAsAgentForm operatorId={operatorId} referredByCode={ref ?? ""} />;
}
