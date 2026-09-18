# Agent Notes

## Repository Shape

- The root application is a Bun + TypeScript ESM program; its entrypoint is `index.ts` and the root package is private.
- `--template/` is a separate Vite scaffold with its own `package.json`, lockfile, and `tsconfig.json`; do not change or run it for root-app work unless explicitly requested.
- `CLAUDE.md` contains the repository's Bun API and tooling preferences; follow it alongside this file.

## Commands

- Install root dependencies with `bun install`.
- Run the root app with `bun run index.ts`.
- The configured `bunx tsc --noEmit` currently also discovers the incomplete `--template/` scaffold and fails on its browser assets; for root-only verification use `bunx tsc --noEmit --ignoreConfig index.ts --module Preserve --moduleResolution bundler --allowImportingTsExtensions --verbatimModuleSyntax --types bun --lib ESNext --target ESNext`.
- Run tests with `bun test` when test files exist; there is currently no test suite or test script.

## Constraints

- Use Bun-native APIs and commands rather than Node/npm equivalents; do not introduce Vite or another build tool for the root app.
- Bun automatically loads `.env`; do not add `dotenv`.
