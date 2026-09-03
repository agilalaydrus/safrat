import PlatformGate from "@/components/admin/PlatformGate";
import TenantDetail from "@/components/admin/TenantDetail";

export default async function TenantDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <PlatformGate>
      <TenantDetail operatorId={id} />
    </PlatformGate>
  );
}
