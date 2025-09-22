import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // For production builds, export static files
  output: process.env.NODE_ENV === 'production' ? 'export' : undefined,
  trailingSlash: true,
  // Use custom distDir for production builds
  distDir: process.env.NODE_ENV === 'production' ? '../dist/dashboard' : '.next',
  basePath: '',
  assetPrefix: '',
  // Fix workspace root issue
  outputFileTracingRoot: process.cwd(),
  images: {
    unoptimized: true
  }
};

export default nextConfig;
