import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './src',
  timeout: 30000,
  retries: 1,
  use: {
    headless: true,
    baseURL: 'http://localhost:8080',
  },
  webServer: {
    command: 'cd ../.. && make dev',
    port: 8080,
    reuseExistingServer: !process.env.CI,
    timeout: 60000,
  },
});