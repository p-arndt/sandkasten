# Sandkasten Dashboard

Modern web dashboard for the Sandkasten sandbox runtime, built with SvelteKit 5 and shadcn-svelte.

## Features

- **Overview Dashboard** - Real-time stats, pool status, and recent sessions
- **Session Management** - Create, destroy, search, and filter sessions
- **Workspace Management** - List and delete persistent workspaces
- **Settings Editor** - YAML configuration editor with validation
- **Dark/Light Mode** - Persistent theme toggle
- **Real-time Updates** - Auto-refresh with configurable intervals
- **Modern UX** - Toast notifications, loading states, relative timestamps, copy-to-clipboard

## Tech Stack

- **SvelteKit 5** - Full-stack framework with SSG
- **Svelte 5** - Reactive UI components with runes
- **Tailwind CSS v4** - Utility-first styling
- **shadcn-svelte** - Beautiful UI component library
- **Lucide Icons** - Icon library
- **Sonner** - Toast notifications
- **mode-watcher** - Dark mode management

## Development

Start the Go daemon and SvelteKit dev server in parallel:

```bash
# Terminal 1: Run the Go daemon
task run

# Terminal 2: Run the dev server
cd web
pnpm dev
```

The dev server runs on `http://localhost:5173` and proxies API requests to the Go daemon on `localhost:8080`.

## Production Build

The dashboard is built as static files and embedded into the Go binary:

```bash
task daemon  # Builds web first, then compiles Go binary with embedded assets
```

Static files are output to `../internal/web/dist/` and embedded using Go's `embed.FS`.

## Project Structure

```
web/
├── src/
│   ├── lib/
│   │   ├── api.ts              # API client
│   │   ├── types.ts            # TypeScript types
│   │   ├── utils/              # Utilities (time, clipboard)
│   │   └── components/         # shadcn-svelte components
│   └── routes/
│       ├── +layout.svelte      # Root layout with sidebar
│       ├── +page.svelte        # Overview page
│       ├── sessions/           # Sessions management
│       ├── workspaces/         # Workspaces management
│       └── settings/           # Settings editor
├── svelte.config.js            # SvelteKit config (adapter-static)
├── vite.config.ts              # Vite config (API proxy)
└── package.json
```

## Available Scripts

- `pnpm dev` - Start development server
- `pnpm build` - Build for production
- `pnpm preview` - Preview production build
- `pnpm check` - Type-check the project
