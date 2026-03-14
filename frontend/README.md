# MediaShare — Frontend

Web client for the MediaShare platform. Built with **Next.js 14** (App Router), **TypeScript**, **Tailwind CSS**, and **shadcn/ui**.

---

## Table of Contents

- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Environment Variables](#environment-variables)
  - [Development](#development)
  - [Production Build](#production-build)
  - [Docker](#docker)
- [Pages & Routes](#pages--routes)
- [Component Guide](#component-guide)
  - [shadcn/ui Setup](#shadcnui-setup)
  - [Key Components](#key-components)
- [State Management](#state-management)
- [API Layer](#api-layer)
  - [Axios Instance](#axios-instance)
  - [Auto Token Refresh](#auto-token-refresh)
  - [API Modules](#api-modules)
- [Auth Flow](#auth-flow)
- [Upload Flow](#upload-flow)
- [Styling](#styling)

---

## Tech Stack

| Layer | Technology |
|---|---|
| Framework | Next.js 14 (App Router) |
| Language | TypeScript 5 |
| Styling | Tailwind CSS v3 |
| UI Components | shadcn/ui (Radix UI primitives) |
| Forms | React Hook Form + Zod |
| HTTP Client | Axios with interceptors |
| Global State | Zustand |
| Data Fetching | SWR |
| Notifications | Sonner (toast) |
| File Upload | react-dropzone + XMLHttpRequest |
| Date Formatting | date-fns |
| Icons | Lucide React |

---

## Project Structure

```
frontend/
├── app/                            # Next.js App Router pages
│   ├── layout.tsx                  # Root layout — wraps AuthProvider + Toaster
│   ├── page.tsx                    # Redirects to /explore
│   ├── globals.css                 # Tailwind + CSS variables (shadcn theme)
│   │
│   ├── (auth)/                     # Route group — no shared layout
│   │   ├── login/page.tsx          # Login form
│   │   └── register/page.tsx       # Registration form
│   │
│   ├── (public)/                   # Public-facing pages (no auth required)
│   │   ├── explore/page.tsx        # Infinite scroll media feed + search
│   │   └── i/[short_code]/
│   │       ├── page.tsx            # SSR share page (OG meta tags)
│   │       └── SharePageClient.tsx # Client-side share page interactions
│   │
│   ├── upload/
│   │   └── page.tsx                # Upload page (auth-gated client component)
│   │
│   ├── (dashboard)/
│   │   └── dashboard/page.tsx      # My Files — list, edit, delete, copy link
│   │
│   └── (admin)/
│       └── admin/
│           ├── page.tsx            # Admin stats dashboard
│           ├── media/page.tsx      # Admin media management table
│           ├── users/page.tsx      # Admin user management table
│           └── reports/page.tsx    # Admin content report queue
│
├── components/
│   ├── ui/                         # shadcn/ui component copies
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── card.tsx
│   │   ├── badge.tsx
│   │   ├── dialog.tsx
│   │   ├── label.tsx
│   │   └── progress.tsx
│   ├── layout/
│   │   ├── AuthProvider.tsx        # Bootstraps auth state on app load
│   │   └── Navbar.tsx              # Top navigation bar
│   ├── media/
│   │   └── MediaCard.tsx           # Media thumbnail card for grids
│   └── upload/
│       └── Uploader.tsx            # Full upload widget with progress
│
└── lib/
    ├── api.ts                      # Axios instance + all API call functions
    ├── auth.ts                     # In-memory token store helpers
    ├── store.ts                    # Zustand auth store
    ├── types.ts                    # Shared TypeScript types
    ├── utils.ts                    # cn(), formatBytes(), formatNumber()
    └── hooks.ts                    # useDebounce
```

---

## Getting Started

### Prerequisites

- Node.js 20+
- npm or pnpm
- The backend API running (see `backend/README.md`)

### Environment Variables

Create a `.env.local` file in the `frontend/` directory:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_APP_URL=http://localhost:3000
```

| Variable | Description |
|---|---|
| `NEXT_PUBLIC_API_URL` | URL of the Go backend API |
| `NEXT_PUBLIC_APP_URL` | Public URL of this frontend (used for OG tags and share links) |

### Development

```bash
cd frontend

# Install dependencies
npm install

# Add remaining shadcn components (first time only)
npx shadcn@latest add select separator tabs avatar toast dropdown-menu

# Start dev server
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

> The `next.config.ts` proxies `/api/*` to the backend, so you don't need to configure CORS for local development if both are on localhost.

### Production Build

```bash
npm run build
npm start
```

### Docker

```bash
# From the repo root
docker compose up frontend --build
```

The Dockerfile uses Next.js **standalone output** for a minimal production image.

To enable standalone output, add to `next.config.ts`:

```ts
const nextConfig: NextConfig = {
  output: 'standalone',
  // ...
};
```

---

## Pages & Routes

| Route | Auth | Description |
|---|---|---|
| `/` | — | Redirects to `/explore` |
| `/explore` | Public | Infinite-scroll feed, type filter, full-text search |
| `/i/:short_code` | Public | Share page — media preview, download, copy link, embed code, report |
| `/login` | Public | Login form |
| `/register` | Public | Registration form |
| `/upload` | Required | File upload with progress bar |
| `/dashboard` | Required | Manage your own files (edit, delete, copy link) |
| `/admin` | Admin only | Platform statistics dashboard |
| `/admin/media` | Admin only | Browse and delete all media |
| `/admin/users` | Admin only | Browse and suspend/activate users |
| `/admin/reports` | Admin only | Review and resolve content reports |

### Route Groups

Next.js route groups (folder names in parentheses) are used to organize pages without affecting URLs:

- `(auth)` — login, register
- `(public)` — explore, share pages
- `(dashboard)` — user dashboard
- `(admin)` — admin panel pages

---

## Component Guide

### shadcn/ui Setup

shadcn/ui copies component source code into your project (it is not an npm package). The `components.json` file configures it:

```json
{
  "style": "default",
  "tailwind": { "baseColor": "slate", "cssVariables": true },
  "aliases": { "components": "@/components", "utils": "@/lib/utils" }
}
```

**Adding more components:**

```bash
npx shadcn@latest add <component-name>
# Examples:
npx shadcn@latest add select
npx shadcn@latest add dropdown-menu
npx shadcn@latest add table
npx shadcn@latest add skeleton
npx shadcn@latest add separator
```

Components are added to `components/ui/` and can be edited freely.

**Currently included:**

| Component | File | Used in |
|---|---|---|
| `Button` | `ui/button.tsx` | Everywhere |
| `Input` | `ui/input.tsx` | Forms, search bars |
| `Card` | `ui/card.tsx` | Auth pages, stats |
| `Badge` | `ui/badge.tsx` | Media type labels, status indicators |
| `Label` | `ui/label.tsx` | Form field labels |
| `Dialog` | `ui/dialog.tsx` | Edit modal, report modal |
| `Progress` | `ui/progress.tsx` | Upload progress bar |

---

### Key Components

#### `AuthProvider` (`components/layout/AuthProvider.tsx`)

Runs once on app mount. Silently calls `POST /api/auth/refresh` using the `HttpOnly` cookie to restore session state after a page reload. Sets the in-memory access token and Zustand user state.

```tsx
// app/layout.tsx
<AuthProvider>
  {children}
</AuthProvider>
```

---

#### `Navbar` (`components/layout/Navbar.tsx`)

Sticky top navigation. Renders different links based on auth state and user role. Shows the Upload button and admin link when appropriate.

---

#### `Uploader` (`components/upload/Uploader.tsx`)

The core upload widget. Handles the full 4-step upload flow:

1. **Drag & drop** — react-dropzone, client-side type/size validation
2. **Sign** — `POST /api/upload/sign` → get presigned S3 URL
3. **XHR upload** — `PUT` directly to S3 using `XMLHttpRequest` (not `fetch`) to get `upload.onprogress` events
4. **Confirm + SSE** — `POST /api/upload/confirm` then opens an `EventSource` for real-time processing updates

```
idle → uploading (XHR progress %) → confirming → processing (SSE progress %) → done
                                                                              → error
```

> Uses `XMLHttpRequest` instead of `fetch` because the Fetch API does not expose upload progress events.

---

#### `MediaCard` (`components/media/MediaCard.tsx`)

Reusable thumbnail card for the explore feed and dashboard grid. Shows thumbnail (with play overlay for videos), title, type badge, tags, view/download counts, and file size.

---

#### `SharePageClient` (`app/(public)/i/[short_code]/SharePageClient.tsx`)

Client component for the share page. Records a view on mount, handles download (calls API for presigned URL, opens in new tab), copy link, and the report dialog. The parent `page.tsx` is a Server Component that generates OG meta tags for social media crawlers.

---

## State Management

### Zustand Auth Store (`lib/store.ts`)

```ts
interface AuthState {
  user: User | null;
  accessToken: string | null;
  isLoading: boolean;
  setAuth: (user: User, token: string) => void;
  clearAuth: () => void;
  setLoading: (loading: boolean) => void;
}
```

`isLoading` is `true` until `AuthProvider` finishes the initial token refresh attempt. Use it to prevent auth-gated pages from flashing before redirecting:

```tsx
const { user, isLoading } = useAuthStore();

useEffect(() => {
  if (!isLoading && !user) router.push('/login');
}, [user, isLoading, router]);

if (isLoading) return null;
```

### In-Memory Token Store (`lib/auth.ts`)

The access token is stored in a **module-level variable**, not in `localStorage` or `sessionStorage`. This prevents XSS attacks from stealing the token. The token is lost on page reload, which is why `AuthProvider` re-fetches it from the `HttpOnly` cookie on every mount.

```
Access token  →  module variable (lib/auth.ts)  →  lost on reload, restored by AuthProvider
Refresh token →  HttpOnly cookie               →  survives reload, JS cannot read it
```

---

## API Layer

### Axios Instance (`lib/api.ts`)

A single Axios instance is configured with:
- `baseURL` from `NEXT_PUBLIC_API_URL`
- `withCredentials: true` (sends the `refresh_token` cookie on cross-origin requests)
- 30-second timeout

### Auto Token Refresh

The response interceptor handles `401 Unauthorized` automatically:

1. Pauses the failing request
2. Calls `POST /api/auth/refresh` using the `HttpOnly` cookie
3. Updates the in-memory access token
4. Retries all queued requests with the new token
5. If refresh fails → clears auth state → redirects to `/login`

Concurrent requests that all get a `401` are queued and replayed after a single refresh — no duplicate refresh calls.

### API Modules

All API calls are organized as typed functions exported from `lib/api.ts`:

```ts
// Auth
authApi.register(data)
authApi.login(data)
authApi.logout()
authApi.refresh()
authApi.me()

// Upload
uploadApi.sign({ filename, content_type, size_bytes })
uploadApi.confirm(mediaId)
// SSE progress: new EventSource(`${API_URL}/api/upload/progress/${mediaId}`)

// User media
mediaApi.list({ page, page_size })
mediaApi.get(id)
mediaApi.update(id, { title, description, tags })
mediaApi.delete(id)

// Public
publicApi.explore({ cursor, page_size, type })
publicApi.search({ q, page, page_size })
publicApi.getByShortCode(shortCode)
publicApi.recordView(shortCode)
publicApi.download(shortCode)
publicApi.report(shortCode, reason)

// Admin
adminApi.listMedia(params)
adminApi.deleteMedia(id)
adminApi.listUsers(params)
adminApi.updateUser(id, data)
adminApi.listReports(params)
adminApi.updateReport(id, action)
adminApi.getStats()
```

---

## Auth Flow

```
Page Load
  │
  ▼
AuthProvider mounts
  │
  ├─ POST /api/auth/refresh (HttpOnly cookie sent automatically)
  │   ├─ success → setAccessToken() + setAuth(user, token) → isLoading = false
  │   └─ fail    → clearAuth()                             → isLoading = false
  │
  ▼
Protected page checks `user` and `isLoading`:
  - isLoading = true  → render nothing (avoid flash)
  - isLoading = false, user = null  → redirect to /login
  - isLoading = false, user = {...} → render page
```

---

## Upload Flow

```
1. User drops file onto Uploader
   └─ Client validates type (allowlist) and size (10MB image / 500MB video)

2. POST /api/upload/sign
   └─ Server: checks quota, generates short_code, creates DB record, returns presigned URL

3. XMLHttpRequest PUT → S3 (direct, backend not involved)
   └─ xhr.upload.onprogress → update progress bar state (0–100%)

4. POST /api/upload/confirm
   └─ Server: verifies S3 object exists, enqueues processor job

5. EventSource /api/upload/progress/:media_id (SSE)
   └─ Server: streams { status, progress } from Redis every 1s
   └─ Client: updates processing progress bar
   └─ On status = "ready" → show share URL + copy button
   └─ On status = "failed" → show error state
```

---

## Styling

Tailwind CSS with CSS variables for theming. The `globals.css` defines light and dark mode color tokens following the shadcn/ui convention:

```css
:root {
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  --primary: 222.2 47.4% 11.2%;
  /* ... */
}

.dark {
  --background: 222.2 84% 4.9%;
  /* ... */
}
```

All shadcn components use `hsl(var(--token))` so the entire theme can be changed by editing the CSS variables in `globals.css`.

### Utility helpers (`lib/utils.ts`)

```ts
cn(...classes)               // Merge Tailwind classes (clsx + tailwind-merge)
formatBytes(bytes)           // "2.3 MB"
formatNumber(n)              // "1.2K", "3.4M"
formatDuration(seconds)      // "1:23", "1:02:34"
```
