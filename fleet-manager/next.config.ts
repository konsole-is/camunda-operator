import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // Next.js takes the directory of the highest lockfile it finds as the
  // tracing root. A lockfile in a parent directory moves server.js to a nested
  // path inside .next/standalone.
  outputFileTracingRoot: __dirname,
  turbopack: { root: __dirname },
};

export default nextConfig;
