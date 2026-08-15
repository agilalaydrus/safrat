import type { NextConfig } from "next";
import { withSentryConfig } from "@sentry/nextjs";

const nextConfig: NextConfig = {
  transpilePackages: ["@hajj-saas/proto-gen", "@hajj-saas/validations", "@hajj-saas/ui"],
};

// withSentryConfig no-ops most of its build-time behavior (source map upload,
// release creation) without SENTRY_AUTH_TOKEN/SENTRY_ORG/SENTRY_PROJECT set —
// safe to wrap unconditionally in dev.
export default withSentryConfig(nextConfig, { silent: true });
