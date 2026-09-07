# Frontend (Next.js App Router)

- TypeScript strict mode. No `any`.
- Server Components by default; add `"use client"` only when you need state,
  effects, or browser APIs. Say why when you add it.
- Data fetching from server components where possible; API calls go through
  `lib/api.ts`, never raw `fetch` scattered in components.
- Auth token lives in an httpOnly cookie set by the backend. Never localStorage.
- Prices come from the API as int64 cents — format at render time only.
- Tailwind for styling. No component library unless I approve it.
- Keep components small; extract when a file passes ~150 lines.