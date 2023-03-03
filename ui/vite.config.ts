import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import viteTsconfigPaths from "vite-tsconfig-paths";
import viteFaviconsPlugin from "vite-plugin-favicon";
import { Logout } from "react-admin";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), viteTsconfigPaths(), viteFaviconsPlugin("./src/logo.png")],
  build: {
    outDir: "build",
  },
  server: {
    port: 3000,
  },
});
