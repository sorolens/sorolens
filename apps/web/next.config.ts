import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@sorolens/ui", "@sorolens/xdr"],
};

export default nextConfig;
