import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Vitest config for the Next.js web app. The key decisions:
//   - jsdom environment: React Testing Library needs a DOM
//   - @/ path alias: matches tsconfig's paths so imports like @/lib/format resolve
//   - react plugin: handles JSX transform for .tsx test files
//   - setupFiles: registers @testing-library/jest-dom matchers (toBeInTheDocument, etc.)
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./__tests__/setup.ts"],
    include: ["__tests__/**/*.test.{ts,tsx}"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
});
