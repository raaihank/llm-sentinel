import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'export',
  trailingSlash: true,
  distDir: '../dist/dashboard',
  basePath: '',
  assetPrefix: '',
  images: {
    unoptimized: true
  }
};

export default nextConfig;
