# Ariadne Frontend

Vue 3 + Vite + TypeScript frontend for the Ariadne rewrite.

## Commands

```powershell
pnpm install
pnpm build
pnpm dev
```

## UI Rules

- Use Graphite Teal tokens from `src/style.css`; default UI must stay light, with black/dark surfaces only under `.dark`.
- Keep launcher actions explicit; do not infer file behavior from `path`.
- Use Reka UI primitives for headless menus and overlays.
- Use shadcn-vue conventions for local component ownership.
