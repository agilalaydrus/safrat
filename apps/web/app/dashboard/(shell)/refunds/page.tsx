import { RoleGate } from "@/components/auth/RoleGate";
import RefundPayoutDashboard from "@/components/refunds/RefundPayoutDashboard";

export default function RefundsPage() {
  return <RoleGate require={["owner", "admin"]} fallback={<main style={{ padding: 32 }}><h1>Akses dibatasi</h1><p>Hanya pemilik atau admin yang dapat mengelola pencairan refund.</p></main>}><RefundPayoutDashboard /></RoleGate>;
}
