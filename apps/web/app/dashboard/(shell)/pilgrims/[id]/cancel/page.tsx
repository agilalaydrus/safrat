import PilgrimCancelFlow from "@/components/cancellation/PilgrimCancelFlow";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PilgrimCancelFlow pilgrimId={id} />;
}
