import type { NextConfig } from "next";

// The test pages are files named page.envtest.tsx. Next.js treats them as
// pages only in a build with FLEET_ENVTEST_BUILD=1, so a production build has
// no test page. That build goes to its own directory, so it never replaces
// the production build in .next.
const envtestBuild = process.env.FLEET_ENVTEST_BUILD === "1";

const nextConfig: NextConfig = {
  output: "standalone",
  // Next.js takes the directory of the highest lockfile it finds as the
  // tracing root. A lockfile in a parent directory moves server.js to a nested
  // path inside .next/standalone.
  outputFileTracingRoot: __dirname,
  turbopack: {
    root: __dirname,
    // The page scenarios run on envtest, which has no kubelet and no
    // metrics.k8s.io. In the envtest build the observe readers read the
    // observe.yaml of the scenario instead. A production build never
    // bundles readers.envtest.ts.
    ...(envtestBuild && {
      resolveAlias: {
        "@/lib/observe/readers": "./src/lib/observe/readers.envtest.ts",
      },
    }),
  },
  pageExtensions: envtestBuild
    ? ["envtest.tsx", "tsx", "ts", "jsx", "js"]
    : ["tsx", "ts", "jsx", "js"],
  distDir: envtestBuild ? ".next-envtest" : ".next",
  // The Kubernetes client loads optional modules and files at run time, so
  // the server loads it from node_modules instead of the bundle.
  serverExternalPackages: ["@kubernetes/client-node"],
};

export default nextConfig;
