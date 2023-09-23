import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import viteTsconfigPaths from "vite-tsconfig-paths";
import { Logout } from "react-admin";

const plugins = [react(), viteTsconfigPaths()];

export default defineConfig({
  plugins,
  build: {
    outDir: "build",
  },
  server: {
    port: 3000,
  },
});
