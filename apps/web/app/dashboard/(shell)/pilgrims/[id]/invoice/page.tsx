import PilgrimInvoice from "@/components/pilgrims/PilgrimInvoice";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PilgrimInvoice pilgrimId={id} />;
}
