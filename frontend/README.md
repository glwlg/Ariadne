# Fancun Frontend

Vue 3 + Vite + TypeScript frontend for Fancun.

## Commands

```powershell
pnpm install
pnpm test
pnpm build
pnpm dev
```

## UI Rules

- Use design tokens from `src/styles/tokens.css` and `src/styles/themes.css` (imported via `src/style.css`) across the three active light themes: `cloud-blue`, `professional-pink`, and `light-graphite` (no `.dark` theme).
- Keep launcher actions explicit (`PreviewAction`); do not infer file behavior from `path`.
- Use `Fancun*` UI primitives (`FancunButton`, etc.) for standard controls.
- Use Reka UI primitives for headless menus and overlays.
- Use shadcn-vue conventions for local component ownership.
