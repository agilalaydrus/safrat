import ChecklistPanel from "@/components/checklist/ChecklistPanel";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ChecklistPanel seasonId={id} />;
}
