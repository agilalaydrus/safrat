import PilgrimDetail from "@/components/pilgrims/PilgrimDetail";

export default async function PilgrimDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PilgrimDetail id={id} />;
}
