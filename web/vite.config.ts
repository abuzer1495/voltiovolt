// SPDX-License-Identifier: AGPL-3.0-or-later
import { defineConfig } from 'vitest/config';

// The built front end goes into cmd/vlt-gui/frontend/dist; the VLT GUI
// embeds it (SEC-095: no remote content). No inline script, no eval.
export default defineConfig({
  base: '/',
  build: {
    outDir: '../cmd/vlt-gui/frontend/dist',
    emptyOutDir: true,
    assetsInlineLimit: 0,
    modulePreload: { polyfill: false },
    sourcemap: false,
    target: 'es2022',
  },
  test: {
    environment: 'happy-dom',
    environmentOptions: {
      happyDOM: {
        settings: { disableCSSFileLoading: true, disableJavaScriptFileLoading: true, disableJavaScriptEvaluation: true },
      },
    },
    include: ['test/**/*.test.{ts,tsx}'],
  },
});
