# Agent Notes

## Repository Shape

- The application is a Svelte 5 + Vite + TypeScript SPA.
- Bun is the package manager and test runner.
- `frontend/src/main.ts` mounts `frontend/src/App.svelte`; pathname matching lives in `frontend/src/lib/route.ts`.
- Pure domain logic lives in `frontend/src/lib` and must remain testable without a browser.
- Global and page CSS remain in `frontend/` and are imported by `frontend/src/app.css`.

## Commands

- Run commands from `frontend/`.
- Install dependencies with `bun install`.
- Run development with `bun run dev`.
- Run pure logic tests with `bun test`.
- Run Svelte and TypeScript checks with `bun run check`.
- Build production assets with `bun run build`.
- Preview the build with `bun run preview`.

## Constraints

- Use Svelte and Vite for frontend work; do not restore the old `Bun.serve()` HTML-import server.
- Do not introduce SvelteKit unless explicitly requested.
- Preserve the clean URL and query contracts documented in `frontend/src/lib/route.ts` and used by checkout.
- Production hosting requires an SPA fallback to `/index.html`.
- Keep ticket snapshot validation and checkout trust-boundary validation intact.
