# ticket-online

Demo alur pemesanan tiket konser menggunakan Svelte 5, Vite, TypeScript, dan Bun.

## Development

```bash
bun install
bun run dev
```

## Verification

```bash
bun test
bun run check
bun run build
```

Preview production build:

```bash
bun run preview
```

## Deployment

Deploy isi `dist/` sebagai static site. Hosting harus mengarahkan semua URL aplikasi yang tidak cocok dengan file statis ke `/index.html`, sehingga direct navigation dan refresh pada URL seperti `/konser/nusa-malam` tetap bekerja.

Checkout dan e-ticket dalam proyek ini masih berupa simulasi browser. Belum ada database, payment gateway, reservasi stok, atau email sungguhan.
