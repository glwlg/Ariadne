# Frontend style system

Visual constitution source: `docs/plans/2026-07-22-frontend-aesthetic-audit.md`

## Files

- `tokens.css` — base design tokens (accent, radius, shadow, semantic colors)
- `themes.css` — theme remaps only
- `base.css` — document/window base
- `surfaces-launcher-tools.css` — launcher + tool centers
- `surfaces-flow.css` — Flow workspace
- `surfaces-settings-api-network-overlays.css` — settings, API testing, network, overlays

## Rules

1. Accent is brand primary only (blue family by default theme tokens).
2. No page-level second primary color dialect.
3. Radius: `--radius-sm|md|lg` only.
4. Shadow: `--shadow-soft|panel|float` only.
5. Prefer `Ari*` components for ordinary controls.
6. User-visible copy stays product language, not design-doc English.
