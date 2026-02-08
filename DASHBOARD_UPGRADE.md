# Dashboard Upgrade - SvelteKit 5 Implementation

## Summary

Successfully upgraded the Sandkasten dashboard from embedded HTML templates to a full SvelteKit 5 application with shadcn-svelte components. The dashboard is now a modern, responsive web application with dark mode, real-time updates, and a comprehensive UI.

## What Was Implemented

### Frontend (SvelteKit 5)

#### Configuration
- ✅ Configured `@sveltejs/adapter-static` to output to `internal/web/dist/`
- ✅ Added Vite proxy for `/api`, `/v1`, and `/healthz` routes
- ✅ Set up path aliases and TypeScript configuration

#### Core Infrastructure
- ✅ **API Client** (`web/src/lib/api.ts`) - Typed fetch wrapper for all endpoints
- ✅ **TypeScript Types** (`web/src/lib/types.ts`) - Complete type definitions
- ✅ **Utilities** - Time formatting, clipboard operations

#### Layout & Navigation
- ✅ Root layout with collapsible sidebar navigation
- ✅ Dark/light mode toggle with persistence (mode-watcher)
- ✅ Lucide icons throughout
- ✅ Responsive design

#### Pages

**Overview Dashboard (`/`)**
- Stats cards: Total Sessions, Active, Expired, Uptime
- Pool status with progress bars (current/target per image)
- Recent sessions table (last 5)
- Real-time auto-refresh every 5 seconds

**Sessions Page (`/sessions`)**
- Full sessions table with all fields
- Search by ID or image name
- Filter by status (all/running/expired/destroyed)
- Create session dialog with image/TTL/workspace options
- Destroy confirmation dialog
- Copy session IDs to clipboard
- Relative timestamps with full date tooltips

**Workspaces Page (`/workspaces`)**
- Workspaces table with ID, created date, size, labels
- Delete workspace with confirmation
- Copy workspace IDs to clipboard
- Size formatting (MB/GB)

**Settings Page (`/settings`)**
- YAML configuration editor with monospace font
- Save/Reload/Validate buttons
- Warning banner about restart requirement
- Collapsible configuration reference
- Toast notifications for all operations

#### UX Features
- ✅ Toast notifications (Sonner) - replaced all `alert()` calls
- ✅ Relative timestamps ("2m ago") with tooltips
- ✅ Copy-to-clipboard for all IDs
- ✅ Loading skeletons while fetching data
- ✅ Auto-refresh with countdown timers
- ✅ Form validation
- ✅ Error handling with user-friendly messages

### Backend (Go)

#### Pool Status API
- ✅ Added `Status()` method to `internal/pool/pool.go`
- ✅ Returns `PoolStatus` with enabled state and per-image current/target counts
- ✅ Included pool data in `/api/status` response

#### Web Handlers
- ✅ Replaced HTML template embeds with `embed.FS` for `dist/` folder
- ✅ Added `ServeSPA()` handler with SPA fallback routing
- ✅ Pass pool manager reference to web handler
- ✅ Include pool status in `/api/status` JSON response

#### Router & Middleware
- ✅ Updated `NewServer()` to accept pool manager parameter
- ✅ Replaced individual page routes with catch-all SPA handler
- ✅ Updated auth middleware to skip auth for:
  - Static assets (`/_app/*`, `.js`, `.css`, etc.)
  - SPA routes (`/sessions`, `/workspaces`, `/settings`)
  - Dashboard UI (`/`, `/api/status`)
- ✅ Updated `cmd/sandkasten/main.go` to pass pool to API server

#### Build System
- ✅ Added `web` target to Makefile
- ✅ Made `daemon` target depend on `web` (builds frontend first)
- ✅ Updated `clean` target to remove `internal/web/dist/`

#### Cleanup
- ✅ Removed old HTML templates (`status.html`, `settings.html`)
- ✅ Updated `internal/web/README.md` with new architecture
- ✅ Created `web/README.md` with development guide

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Development Mode                        │
├─────────────────────────────────────────────────────────────┤
│  SvelteKit Dev Server (:5173)                               │
│         ↓ proxy /api, /v1                                   │
│  Go Daemon (:8080) ← API requests                           │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                     Production Build                        │
├─────────────────────────────────────────────────────────────┤
│  1. pnpm build → internal/web/dist/ (static files)          │
│  2. go build → embeds dist/ via embed.FS                    │
│  3. Single binary serves SPA + API                          │
└─────────────────────────────────────────────────────────────┘
```

## How to Use

### Development Workflow

```bash
# Terminal 1: Start Go daemon
make run

# Terminal 2: Start SvelteKit dev server
cd web
pnpm dev
```

Visit `http://localhost:5173` for hot-reload development.

### Production Build

```bash
make daemon  # Builds web + daemon
./bin/sandkasten --config sandkasten.yaml
```

Visit `http://localhost:8080` (or configured listen address).

## File Changes

### Added
- `web/src/lib/api.ts` - API client
- `web/src/lib/types.ts` - TypeScript types
- `web/src/lib/utils/time.ts` - Time formatting
- `web/src/lib/utils/clipboard.ts` - Clipboard utilities
- `web/src/lib/components/mode-toggle.svelte` - Dark mode toggle
- `web/src/routes/+layout.svelte` - Root layout with sidebar
- `web/src/routes/+page.svelte` - Overview page
- `web/src/routes/sessions/+page.svelte` - Sessions page
- `web/src/routes/workspaces/+page.svelte` - Workspaces page
- `web/src/routes/settings/+page.svelte` - Settings page
- `web/README.md` - Frontend documentation
- `web/.gitignore` - Git ignore for frontend
- `internal/web/dist/.gitkeep` - Placeholder for build output

### Modified
- `web/svelte.config.js` - Configured adapter-static output
- `web/vite.config.ts` - Added API proxy
- `internal/web/handlers.go` - Replaced HTML with SPA serving
- `internal/web/README.md` - Updated documentation
- `internal/api/router.go` - Updated routes and dependencies
- `internal/api/middleware.go` - Updated auth skip logic
- `internal/pool/pool.go` - Added Status() method
- `cmd/sandkasten/main.go` - Pass pool to API server
- `Makefile` - Added web build target

### Removed
- `internal/web/templates/status.html`
- `internal/web/templates/settings.html`

## Dependencies

All frontend dependencies were already installed:
- SvelteKit 5.49.2
- Svelte 5.49.2
- Tailwind CSS v4.1.18
- shadcn-svelte (bits-ui, formsnap, etc.)
- @lucide/svelte 0.561.0
- svelte-sonner 1.0.7
- mode-watcher 1.1.0

No additional packages needed!

## Notes

- The dashboard is fully functional but requires building before the Go daemon can embed it
- Run `make daemon` to build both frontend and backend in one command
- The `dist/` folder is git-ignored; build it as part of your deployment process
- Auth logic unchanged: dashboard UI pages have no auth, API operations require API key if configured
