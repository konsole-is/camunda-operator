import type { NextConfig } from "next";

// The gallery routes are files named page.gallery.tsx. Next.js treats them as
// pages only in a build with FLEET_DEV_GALLERY=1, so a production build has no
// gallery route and no story code. That build goes to its own directory, so it
// never replaces the production build in .next.
const galleryBuild = process.env.FLEET_DEV_GALLERY === "1";

const nextConfig: NextConfig = {
  output: "standalone",
  // Next.js takes the directory of the highest lockfile it finds as the
  // tracing root. A lockfile in a parent directory moves server.js to a nested
  // path inside .next/standalone.
  outputFileTracingRoot: __dirname,
  turbopack: { root: __dirname },
  pageExtensions: galleryBuild
    ? ["gallery.tsx", "tsx", "ts", "jsx", "js"]
    : ["tsx", "ts", "jsx", "js"],
  distDir: galleryBuild ? ".next-gallery" : ".next",
  // The Kubernetes client loads optional modules and files at run time, so
  // the server loads it from node_modules instead of the bundle.
  serverExternalPackages: ["@kubernetes/client-node"],
};

export default nextConfig;
