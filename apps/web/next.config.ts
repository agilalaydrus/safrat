import type { NextConfig } from "next";
import { withSentryConfig } from "@sentry/nextjs";
import withSerwistInit from "@serwist/next";
import { randomUUID } from "node:crypto";

const buildRevision = process.env.APP_BUILD_REVISION || randomUUID();

const pwaRoutes = [
  "/pilgrim",
  "/pilgrim/announcements",
  "/pilgrim/chat",
  "/pilgrim/checklist",
  "/pilgrim/products",
  "/pilgrim/profile",
  "/pilgrim/rituals",
  "/pilgrim/schedule",
  "/pilgrim/sos",
  "/pilgrim/survey",
  "/leader",
  "/leader/chat",
  "/leader/check-in",
  "/leader/health",
  "/leader/hotel",
  "/leader/location",
  "/leader/profile",
  "/leader/rituals",
  "/leader/sos",
  "/leader/wallet",
] as const;

const withSerwist = withSerwistInit({
  swSrc: "app/sw.ts",
  swDest: "public/sw.js",
  register: false,
  disable: process.env.NODE_ENV !== "production",
  additionalPrecacheEntries: pwaRoutes.map((url) => ({ url, revision: buildRevision })),
});

const nextConfig: NextConfig = {
  output: "standalone",
  // The offline PWA specs need a production build served alongside the dev
  // server, and both default to `.next` — a shared directory they would
  // clobber. NEXT_DIST_DIR lets that build live somewhere else.
  distDir: process.env.NEXT_DIST_DIR || ".next",
  generateBuildId: async () => buildRevision,
  transpilePackages: ["@hajj-saas/proto-gen", "@hajj-saas/validations", "@hajj-saas/ui"],
};

// withSentryConfig no-ops most of its build-time behavior (source map upload,
// release creation) without SENTRY_AUTH_TOKEN/SENTRY_ORG/SENTRY_PROJECT set —
// safe to wrap unconditionally in dev.
export default withSentryConfig(withSerwist(nextConfig), { silent: true });
