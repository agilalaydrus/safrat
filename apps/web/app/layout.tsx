import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Tawafiq Hub | Operator Dashboard",
  description: "Hajj and Umrah operations management",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
