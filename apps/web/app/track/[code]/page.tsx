import FamilyTrackerPage from "@/components/family-tracker/FamilyTrackerPage";
export default async function Page({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  return <FamilyTrackerPage code={code} />;
}
