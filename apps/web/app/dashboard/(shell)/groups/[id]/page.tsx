import GroupDetail from "@/components/groups/GroupDetail";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <GroupDetail id={id} />;
}
