# Web Dashboard

The web dashboard is a SvelteKit 5 application with shadcn-svelte components.

## Development

The frontend source code lives in `../../web/`. During development:

```bash
# Terminal 1: Run the Go daemon
task run

# Terminal 2: Run the SvelteKit dev server
cd web
pnpm dev
```

The Vite dev server proxies API requests to the Go daemon on `localhost:8080`.

## Production Build

In production, the SvelteKit app is built as static files and embedded into the Go binary:

```bash
task daemon  # Automatically builds web first, then compiles Go binary
```

The static files are output to `internal/web/dist/` and embedded using Go's `embed.FS`.

## Architecture

- **Frontend:** SvelteKit 5 + Svelte 5 + Tailwind CSS v4 + shadcn-svelte
- **Build:** `@sveltejs/adapter-static` outputs to `internal/web/dist/`
- **Embedding:** Go `embed.FS` packages the dist folder into the binary
- **Serving:** `handlers.go` serves static files with SPA fallback for client-side routing

## Pages

- `/` - Overview dashboard (stats, pool status, recent sessions)
- `/sessions` - Session management (create, destroy, search, filter)
- `/workspaces` - Workspace management (list, delete)
- `/settings` - Configuration editor (YAML editing with validation)

## Features

- Dark/light mode toggle
- Real-time auto-refresh (configurable intervals)
- Toast notifications (via Sonner)
- Relative timestamps with full date tooltips
- Copy-to-clipboard for IDs
- Loading skeletons
- Search and filter
- Form validation
- Responsive design
