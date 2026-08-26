import type { Metadata } from "next";
import PilgrimShellLayout from "./shell";

// A server component purely so this segment can carry its own metadata: the
// shell below is a client component, and without this every pilgrim page
// inherited the root title and greeted a jamaah with "Operator Dashboard".
export const metadata: Metadata = {
  title: "Jamaah",
  description: "Jadwal keberangkatan, panduan ibadah, dan kabar terbaru perjalanan Anda.",
};

export default function PilgrimLayout({ children }: { children: React.ReactNode }) {
  return <PilgrimShellLayout>{children}</PilgrimShellLayout>;
}
