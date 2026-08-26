import type { Metadata } from "next";
import LeaderShellLayout from "./shell";

// See app/pilgrim/layout.tsx — a server wrapper so this segment carries its own
// title instead of inheriting the operator dashboard's.
export const metadata: Metadata = {
  title: "Tour Leader",
  description: "Pendampingan rombongan: check-in, kesehatan, dan koordinasi lapangan.",
};

export default function LeaderLayout({ children }: { children: React.ReactNode }) {
  return <LeaderShellLayout>{children}</LeaderShellLayout>;
}
