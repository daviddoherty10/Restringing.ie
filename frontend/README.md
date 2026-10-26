# Restringing.ie frontend

React 19 + TypeScript + Vite. The public homepage is a static first milestone, ready for marketplace API integration.

## Run locally

```bash
cd frontend
npm ci
npm run dev
```

Then open the local URL printed by Vite (typically http://localhost:5173).

## Checks

```bash
npm run lint
npm run build
```

## Current scope

- Responsive public landing page
- Forest green / lime brand tokens
- Mobile navigation
- Location input with a transparent "coming soon" response (does **not** query the backend)
- Calls to action for the upcoming marketplace and stringer onboarding

The hero and feature image URLs are hosted on Unsplash; replace them with licensed branded imagery before launch. The email link is a placeholder business contact and should be verified before launch.

## Next milestones

1. Add Tailwind CSS and shadcn/ui, plus reusable layout and UI components.
2. Introduce React Router and TanStack Query once the marketplace screens/API contract are defined.
3. Build location search results, stringer profiles and booking.
4. Connect SMS OTP sign-in, restringer dashboard and admin dashboard.

This change deliberately avoids new runtime packages so the existing package-lock remains consistent.
