import { cloudflareTest } from '@cloudflare/vitest-pool-workers';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [
    cloudflareTest({
      main: './src/worker.ts',
      wrangler: { configPath: './wrangler.jsonc' },
      miniflare: {
        bindings: {
          FVF_SIGNING_SECRET: 'test-signing-secret-at-least-32-chars!!',
        },
      },
    }),
  ],
  test: {
    include: ['test/**/*.test.ts'],
  },
});
