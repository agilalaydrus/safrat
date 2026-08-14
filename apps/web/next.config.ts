import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@hajj-saas/proto-gen", "@hajj-saas/validations", "@hajj-saas/ui"],
};

export default nextConfig;
