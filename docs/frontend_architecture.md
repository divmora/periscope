# Frontend Architecture

This document outlines the core architectural decisions and patterns used in the React frontend (`/ui`).

## 1. Theming & Styling
- **Engine**: Tailwind CSS + custom variables in `src/index.css`.
- **UI Components**: Shadcn UI.
- **Theme Standard**: The application defaults to the system preference but is heavily optimized for the "FinGrid Deep Charcoal" dark mode.
- **CSS Variables**: `index.css` overrides the default Shadcn slate variables to use specific `#09090b` (zinc-950) backgrounds and indigo accents.

## 2. API Communication
To ensure standardization and type safety, the UI never uses raw `fetch` calls directly inside components.
- **API Client Wrapper**: `src/api/client.ts` intercepts all API requests, appends `/api`, sets headers, and automatically parses the `success`, `data`, `error`, and `meta` JSON envelope defined in the backend API Standards.
- **API Services**: Each domain has its own service file (e.g., `src/api/telescope.ts`) containing typed functions that make the network calls.

## 3. Data Fetching & State
- **Library**: `@tanstack/react-query`
- **Pattern**: We use `useQuery` and `useMutation` inside pages/components to handle data fetching, caching, loading states (`isLoading`, `isFetching`), and automatic refetching. 
- Example: `const { data, isLoading } = useQuery({ queryKey: ['requests'], queryFn: TelescopeAPI.getRequests })`

## 4. Routing & Layouts
- **Library**: `react-router-dom`
- **Structure**: All project-specific views are scoped under a dynamic URL parameter: `/:projectId/telescope/...`
- **Layout**: `src/layouts/ProjectLayout.tsx` handles the sidebar and navigation.
- **Role-Based Access Control (RBAC)**: The `navItems` array in the layout contains an `allowedRoles` array. Items are dynamically filtered based on the current user's role (`admin`, `member`, `viewer`).
- **Protected Routes**: A `ProtectedRoute` wrapper enforces authentication before rendering any application layout.

## 5. Single Binary Integration
The frontend is designed to be shipped entirely inside the compiled Go backend binary using `//go:embed`.
- In production, running `pnpm build` generates the `ui/dist` directory.
- The `ui/embed.go` file embeds this `dist` folder into the `ui.DistFS` virtual filesystem.
- The Go HTTP router (in `serve.go`) handles all `/api/*` routes. Any other request is routed to the embedded React `index.html`.
- This architecture enables us to serve both the API and the complete SPA UI over a single port (`8006`).
