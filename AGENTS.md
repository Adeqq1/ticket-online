# Agent Notes

## Repository Shape

- The application is a Svelte 5 + Vite + TypeScript SPA.
- Bun is the package manager and test runner.
- `frontend/src/main.ts` mounts `frontend/src/App.svelte`; pathname matching lives in `frontend/src/lib/route.ts`.
- Pure domain logic lives in `frontend/src/lib` and must remain testable without a browser.

## Frontend Structure

```text
frontend/
  public/                 static files copied unchanged to the build
  scripts/                frontend verification and tooling scripts
  src/
    App.svelte            application composition and route dispatch
    main.ts               browser entrypoint
    vite-env.d.ts         Vite environment declarations
    layouts/              page shells, site header, and site footer
    pages/
      public/             buyer-facing route pages
      admin/              admin and staff route pages
    components/           UI components, grouped by area when useful
      admin/
      detail/
      ticket/
    lib/                  TypeScript modules and their colocated unit tests
    styles/
      app.css             stylesheet entrypoint, imported by main.ts
      shared.css          shared tokens, resets, and global styles
      pages/              global page stylesheets
```

- Keep application code, CSS, and unit tests in `src/`. The frontend root contains configuration, manifests, lockfiles, `index.html`, and `Dockerfile`.
- Use PascalCase for Svelte components and kebab-case for TypeScript and CSS filenames. Name route pages `*Page.svelte`.
- Place unit tests beside the module they test as `<module>.test.ts`. Keep each module's scenarios together; preserve coverage when merging tests.
- Pages assemble layouts, components, and library modules. Components must not import route pages; layouts may compose pages. Keep domain logic separate from browser and API side effects.
- Import shared and page styles through `src/styles/app.css`. Use Svelte `<style>` for new component-specific styles; preserve existing global selectors and import order when moving stylesheets.
- Add subfolders only for an existing, coherent group of files. Do not create empty folders, duplicate modules, or catch-all folders such as `misc`.
- Keep browser state local to the page unless a concrete cross-route requirement justifies a store.
- Keep imports relative. Do not add path aliases, dependencies, or abstractions just to reorganize files.

## Commands

- Run commands from `frontend/`.
- Install dependencies with `bun install`.
- Run development with `bun run dev`.
- Run pure logic tests with `bun test`.
- Run Svelte and TypeScript checks with `bun run check`.
- Build production assets with `bun run build`.
- Preview the build with `bun run preview`.
- Bun loads `.env` automatically; do not add `dotenv`.

## Constraints

- Use Svelte and Vite for frontend work; do not restore the old `Bun.serve()` HTML-import server.
- Do not introduce SvelteKit unless explicitly requested.
- Preserve the clean URL and query contracts documented in `frontend/src/lib/route.ts` and used by checkout.
- Production hosting requires an SPA fallback to `/index.html`.
- Keep ticket snapshot validation and checkout trust-boundary validation intact.
- Preserve semantic HTML, focus handling, live regions, reduced motion, dark mode, and print behavior.
- Avoid `innerHTML`; render application data through Svelte templates.
- Deploy `frontend/dist/` as the static site output; `frontend/public/_redirects` must be copied into that output.
- After a structural change, run `bun test`, `bun run check`, and `bun run build`. Update all imports and file references without changing routes, API contracts, storage formats, or validation behavior.
