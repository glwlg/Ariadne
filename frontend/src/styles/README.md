# Frontend style system

Visual constitution source: `docs/plans/2026-07-22-frontend-aesthetic-audit.md`

## Files

- `tokens.css` — base design tokens (accent, radius, shadow, semantic colors)
- `themes.css` — light theme remaps (`cloud-blue`, `professional-pink`, `light-graphite`)
- `base.css` — document/window base
- `surfaces-launcher-tools.css` — launcher + tool centers (clipboard, capture, hosts, workflows, json compare)
- `surfaces-settings-api-network-overlays.css` — settings, API testing, network, capture overlay, pinned image

## Rules

1. Accent is brand primary only (scoped by active light theme in `themes.css`: `cloud-blue`, `professional-pink`, `light-graphite`).
2. No page-level second primary color dialect.
3. Radius: `--radius-sm|md|lg` only.
4. Shadow: `--shadow-soft|panel|float` only.
5. Prefer `Fancun*` components for ordinary controls.
6. User-visible copy stays product language, not design-doc English.
