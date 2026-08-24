# Ariadne Frontend Aesthetic Audit

Date: 2026-07-22
Scope: `frontend/` visual system, component primitives, launcher, tool centers, Flow surfaces
Mode: advanced aesthetic review, not feature review

## 1. Executive Judgment

Ariadne's frontend is past "usable" and into "needs visual governance."

It already has real aesthetic ambition:

- brand mark with a thread/path metaphor
- token foundations (`--primary`, `--surface`, `--flow-*`)
- shared tool-center shell patterns
- a coherent Flow redesign direction in reference images

But the product currently speaks in multiple visual dialects at once:

1. Launcher / Tool Centers: teal utility shell
2. Flow redesign: slate/glass cognitive workbench
3. Todos latest pass: blue/white SaaS task app

The result is not low craft. The result is fragmented craft.

**Overall score: 6.4 / 10**

High ceiling. Main bottleneck is consistency and system authority, not raw visual talent.

## 2. What Already Works

### 2.1 Brand metaphor is correct

`AriadneMark` communicates path, memory, and return better than a generic productivity glyph. Keep the metaphor.

### 2.2 Surface architecture is product-correct

The product correctly separates:

- Launcher as a light entry surface
- Tool Centers as focused workspaces
- overlays / pinned windows as utility surfaces
- Flow as the cognitive workspace

This structure is already more mature than many desktop utility apps.

### 2.3 Tool centers share a real shell language

Shared patterns exist:

- `tool-header`
- `system-pill`
- `status-strip`
- list/detail workspace rhythm

This is a real foundation and should be elevated, not replaced.

### 2.4 Flow has a worldview

Flow is not just CRUD. It has:

- evidence
- inspector
- readiness
- task package
- local-first cognition

That is a strong aesthetic story. The problem is execution density and language drift, not concept emptiness.

## 3. Core Problems

### P0-1. Brand accent is split

Default theme accent is teal (`#0f766e` family).
Flow and especially Todos hardcode blue (`#2563eb` family).

Impact:

- theme switching cannot fully recolor the product
- brand memory weakens
- users feel "multiple products inside one shell"

Evidence:

- global tokens use teal
- many Flow/Todos styles hardcode blue hex/rgba
- Todos primary buttons, borders, and soft fills are blue-first

Required direction:

- one accent family only
- semantic colors remain separate
- no page-level second primary color

### P0-2. `style.css` is a visual debt center

`frontend/src/style.css` is roughly 305KB / 13k+ lines.

Observed scale signals:

- hundreds of hardcoded hex values
- hundreds of hardcoded rgba values
- many non-standard font weights (`820`, `840`, etc.)
- many one-off radii and shadows
- repeated selector families for the same surfaces

Impact:

- spacing rhythm drifts
- radius system drifts
- shadows become unpredictable
- local polish creates global inconsistency
- every new page invents private visual rules

Required direction:

- split styles by tokens / themes / base / components / surfaces
- stop treating one CSS file as the design system

### P0-3. Design primitives exist but are not authoritative

`Ari*` primitives exist:

- `AriButton`
- `AriCard`
- `AriInput`
- `AriSearchBox`
- `AriEmptyState`
- `AriBadge`
- `AriToolbar`
- `AriField`
- `AriTip`
- `AriProgress`

But usage is split almost evenly with raw controls:

- raw `<button>` usage is widespread
- `AriButton` is not yet the default path

Also:

- `AriCard` panel variant references `var(--shadow-soft)` which is not defined
- badge semantic tones partially hardcode Tailwind palette colors instead of theme tokens

Impact:

- same action looks different across surfaces
- design system becomes optional library instead of product law

Required direction:

- `Ari*` becomes the only normal control path
- raw controls only for true special-case interaction widgets

### P1-1. Copy language oscillates between product and design-doc

User-visible surfaces mix:

- Chinese product labels
- English architecture/design labels
- English tool subtitles that read like implementation notes

Examples of the wrong layer leaking into UI:

- tool center subtitles like `Local text timeline...`, `Semantic diff...`, `Command chains...`
- Flow kickers like `CHAT HISTORY`, `AGENT PACKAGE`, `INSIGHTS`, `RULES`, `DRAFTS`
- inspector labels like `Agent Inspector`, `Insight Inspector`, `Readiness`

Impact:

- bilingual cheapness
- design-spec residue on screen
- weaker product voice

Required direction:

- user-facing UI is product copy only
- architecture language stays in docs/code comments
- Chinese-first voice for normal product surfaces

### P1-2. Visual density is too high

Many Flow pages over-explain themselves:

- too many kickers
- too many pills/chips/meta rows
- top actions plus bottom command dock competing
- empty states / progress / status / inspectors all speaking at once

Impact:

- product feels busy rather than authoritative
- hierarchy collapses
- advanced calm is lost

Required direction:

- one primary action per page context
- fewer meta ornaments
- quieter empty and secondary states

### P1-3. Theme system has breadth without deep consistency

Supported themes:

- `light`
- `dark`
- `professional-pink`
- `light-graphite`
- `cloud-blue`

Problems:

- page-level hardcodes ignore theme tokens
- some themes collapse success into primary
- dark mode can look partially skinned rather than fully redesigned

Impact:

- themes feel like filters, not systems

Required direction:

- themes only remap tokens
- surfaces never hardcode theme-sensitive colors
- semantic colors remain independently meaningful

### P1-4. Todos is becoming a separate visual product

Todos latest layout has good structure:

- focus card
- next list
- reminder rail

But its styling language is drifting into a blue SaaS task app, not Flow.

Impact:

- strongest recent polish is also the strongest consistency break
- Todos feels detached from Ariadne/Flow identity

Required direction:

- keep the structure
- re-home it into Flow tokens, radii, buttons, type, and accent rules

### P2-1. Radius / weight / shadow systems are over-fragmented

Current system effectively allows too many values:

- radii: many one-offs plus frequent `999px`
- weights: nonstandard values such as `760`, `820`, `840`
- shadows: many unique recipes

Impact:

- "designed" in isolation
- "incoherent" as a product family

Required direction:

- small closed sets only

### P2-2. Action hierarchy is duplicated

Examples:

- page top actions
- inline card actions
- bottom `FlowCommandDock`
- context menus

Especially in Flow, English dock verbs and Chinese page actions coexist.

Impact:

- users cannot tell which action is primary
- UI looks capable but unsettled

Required direction:

- one primary action locus per context
- dock either becomes the authority or becomes secondary/contextual only

## 4. Surface Scores

| Surface | Score | Judgment |
|---|---:|---|
| Launcher | 7.5/10 | Clear command-palette structure; needs tighter detail discipline |
| Tool centers | 6.8/10 | Shared shell is good; copy and density still engineering-heavy |
| Flow Home | 7.0/10 | Best direction; already feels like a cognitive canvas |
| Flow Timeline / Insights / Assets | 6.5/10 | Strong concepts, overloaded presentation |
| Todos | 5.8/10 | Recently polished, but most detached from global language |
| Design-system base | 5.0/10 | Tokens exist; governance is weak |

## 5. Recommended Aesthetic Constitution

Ariadne should feel like:

> a calm local workbench, not a noisy AI dashboard

### Identity

- ink + teal
- evidence-first
- quiet authority
- Chinese product voice
- almost no decorative motion

### Closed visual sets

#### Accent

- primary accent: teal family only
- no page-invented blue primary

#### Radius

Use only:

- `sm`
- `md`
- `lg`

Suggested target:

- `sm = 6`
- `md = 10`
- `lg = 14`

Avoid casual `999px` except true search/chip cases that are system-approved.

#### Shadow

Use only:

- `soft`
- `panel`
- `float`

#### Type weight

Use only:

- `500`
- `600`
- `700`
- optional display `800`

Remove pseudo-precision weights like `820` / `840`.

#### Control authority

Normal UI must go through:

- `AriButton`
- `AriInput`
- `AriSearchBox`
- `AriCard`
- `AriEmptyState`
- `AriBadge`
- `AriToolbar`
- `AriField`
- `AriTip`
- `AriProgress`

Raw controls only for special interaction surfaces such as capture overlay geometry or timeline scrubbing.

#### Copy authority

User-visible UI must be product copy:

- object
- state
- result
- risk
- next action

Never:

- design rationale
- architecture labels as decoration
- English kickers that only serve the design file

## 6. Repair Priority

### Phase 0 — Freeze the law

1. Write and adopt the visual constitution above as implementation law.
2. Freeze accent to teal family.
3. Freeze radius / shadow / weight closed sets.
4. Ban new page-level color dialects.

### Phase 1 — Make the system real

1. Split `frontend/src/style.css` into tokens, themes, base, components, surfaces.
2. Define missing tokens such as `--shadow-soft`.
3. Route semantic badge/tip colors through tokens.
4. Make `Ari*` the default control path.

### Phase 2 — Reunify product surfaces

1. Re-home Todos into Flow visual language while preserving layout structure.
2. Normalize Flow page headers, inspectors, and command dock hierarchy.
3. Replace English design-doc residue with Chinese product copy.
4. Reduce meta ornament density.

### Phase 3 — Theme depth

1. Ensure all theme-sensitive colors come from tokens only.
2. Validate light / dark / pink / graphite / cloud-blue against launcher, tools, Flow, Todos.
3. Keep semantic colors independent from brand accent.

## 7. Acceptance Criteria

A repair effort is successful only if all of the following are true:

1. No major surface introduces a second primary accent.
2. Todos no longer reads as a separate blue SaaS product.
3. Shared controls look the same across launcher, tool centers, and Flow.
4. Theme switching recolors the product without large hardcoded leftovers.
5. User-visible copy is Chinese product language, not bilingual design annotation.
6. `style.css` is no longer the single ungoverned source of all visual decisions.
7. Each main page has a clear primary action hierarchy instead of competing action bars.

## 8. Non-Goals

This audit does not require:

- a total visual redesign from zero
- abandoning Flow reference structure
- inventing a new brand metaphor
- turning the app into a marketing site
- adding more decorative animation

The goal is convergence, not reinvention.

## 9. Suggested First Implementation Slice

If only one repair slice starts immediately:

1. lock accent to teal tokens
2. remove Todos blue dialect
3. fix `--shadow-soft` and badge semantic tokens
4. replace the most visible English design-doc UI strings
5. stop adding new raw buttons where `AriButton` should exist

This slice creates the highest visible coherence gain with the least conceptual churn.

## 10. Final Position

Ariadne does not need "more style."

It needs:

- one accent story
- one control system
- one copy voice
- one spacing/radius/shadow grammar
- one product face across launcher, tools, and Flow

That is the difference between a collection of well-intended screens and a high-end desktop product.


## 11. Implementation Progress

Date: 2026-07-22

### Done

- Split `frontend/src/style.css` into modular styles:
  - `frontend/src/styles/tokens.css`
  - `frontend/src/styles/themes.css`
  - `frontend/src/styles/base.css`
  - `frontend/src/styles/surfaces-launcher-tools.css`
  - `frontend/src/styles/surfaces-flow.css`
  - `frontend/src/styles/surfaces-settings-api-network-overlays.css`
- Locked constitution tokens:
  - accent primary teal family
  - radius sm/md/lg = 6/10/14
  - shadow soft/panel/float
  - independent semantic success/warning/danger/info tokens including soft/border/foreground
- Theme remaps no longer collapse success into primary for graphite/cloud/pink; graphite/cloud keep teal brand accent.
- Removed blue SaaS primary dialect (`#2563eb` family) from styles.
- Converted hard-coded teal accent rgba values to `var(--primary)` mixes.
- Normalized pseudo font-weights (620/760/820/840...) to 600/700/800.
- Fixed `AriCard` / `AriBadge` / `AriTip` to use theme tokens (`--shadow-soft/panel`, semantic soft colors).
- Replaced major English design-doc UI strings with Chinese product copy across Flow and tool centers.
- Re-homed Todos styles into Flow surface tokens; Todos no longer uses blue as a private primary.
- Command dock reduced to timeline-only batch actions, removing competing bottom action bars on Todos/Insights/Drafts/Assets/Rules.
- Migrated additional ordinary actions to `AriButton` on Todos / Insights / Assets / Home / Timeline tool actions.
- `pnpm --dir frontend build` passes.

### Still open

- Full ordinary-control migration to `Ari*` is incomplete; remaining raw buttons are mostly list rows, nav items, tree toggles, icon-only closes, context menus, and special interaction widgets.
- Deeper density reduction across Timeline / Insights / Assets meta ornaments.
- Exhaustive neutral hardcode purge and visual screenshot QA across all five themes in the running desktop app.

### Verify

```powershell
pnpm --dir frontend build
```

Then visually check launcher, clipboard/hosts, Flow home, Todos, timeline under light/dark and at least one tinted theme.
