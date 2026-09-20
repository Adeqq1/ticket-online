# Project Guidance

## Tooling

- Use Bun for dependency installation and scripts.
- Use `bun run dev`, `bun run build`, and `bun run preview` for Vite.
- Use `bun test` for pure TypeScript tests.
- Use `bun run check` for Svelte and TypeScript diagnostics.
- Bun loads `.env` automatically; do not add `dotenv`.

## Frontend

- The root application uses Svelte 5 with Vite, not SvelteKit.
- Keep browser state local to the page unless a concrete cross-route requirement justifies a store.
- Keep pure filtering, pricing, validation, routing, and snapshot logic in `frontend/src/lib`.
- Preserve semantic HTML, focus handling, live regions, reduced motion, dark mode, and print behavior.
- Avoid `innerHTML`; render application data through Svelte templates.

## Routing And Deployment

- Supported paths are `/`, `/konser`, `/konser/:id`, `/checkout/:id`, and `/tiket/:id`.
- Ticket quantities are passed from detail to checkout through URL query parameters.
- The app is a static SPA; production hosting must rewrite non-file application requests to `/index.html`.

## Demo Boundaries

- Concert inventory and prices are static browser data.
- Payment and email behavior are simulated.
- E-tickets are stored in `sessionStorage` and the drawn code is not a real QR code.
