import CancellationPolicyPanel from "@/components/cancellation/CancellationPolicyPanel";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <CancellationPolicyPanel seasonId={id} />;
}
