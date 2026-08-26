import type { Metadata, Viewport } from "next";
import "./globals.css";

export const metadata: Metadata = {
  // A template rather than one fixed string: every page under this layout used
  // to inherit "Operator Dashboard", so a jamaah opening the app saw a browser
  // tab addressed to someone else entirely.
  title: {
    default: "TawafiqHub",
    template: "%s | TawafiqHub",
  },
  description: "Platform operasional Umrah dan Haji: jadwal, jamaah, dan pendampingan perjalanan.",
  applicationName: "TawafiqHub",
  manifest: "/manifest.json",
  icons: {
    icon: [
      { url: "/icons/icon.svg", type: "image/svg+xml" },
      { url: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
    ],
    apple: "/icons/apple-touch-icon.png",
    shortcut: "/favicon.ico",
  },
  appleWebApp: {
    capable: true,
    title: "TawafiqHub",
    statusBarStyle: "default",
  },
};

export const viewport: Viewport = {
  themeColor: "#065f46",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  // lang="id": the interface is Indonesian throughout. Declaring English made
  // screen readers pronounce it as English and prompted browsers to offer a
  // translation of text already in the reader's language.
  return <html lang="id"><body>{children}</body></html>;
}
