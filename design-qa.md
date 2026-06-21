**Findings**
- [P1] Rendered screenshot comparison still blocked
  Location: Product Design image-to-code QA for the Todo page.
  Evidence: the source design is available at `C:\Users\luwei\AppData\Local\Temp\codex-clipboard-9c836478-d426-4c0f-853a-43a1ec5f9582.png`, but this turn did not expose Browser/Chrome capture tools for a same-viewport screenshot of the rebuilt app. I did not use Playwright because the repo instruction says to prefer the built-in browser tool for frontend debugging.
  Impact: I can verify build correctness and static implementation changes, but cannot honestly mark screenshot-level fidelity as passed.
  Fix: capture the packaged Todo page once Browser/Chrome tooling is available, then compare it against the source image at the same desktop viewport.

**Open Questions**
- Whether the latest package fully clears the visual overlap in the user's running desktop window.

**Implementation Checklist**
- Reworked the Todo page to match the selected reference structure: top title/search/actions, "下一件事" focus card, table-style "接下来" list, collapsed completed section, and fixed right reminder rail.
- Replaced the previous long-text layout with display helpers that clean literal `\n`, shorten notes, and place schedule, range, location, priority, status, and 留痕 in separate UI slots.
- Updated the Todo bottom dock to read as "待办总览" for the Todo route instead of a generic context dock.
- Removed the visible row action button group from the list because it squeezed into a vertical column in the user's screenshot; list rows now show data and a single edit/more entry, while processing actions live in the focus card.
- Added high-specificity layout corrections for the focus card so its header, icon, content, and actions occupy separate grid areas.
- Reduced Todo page internal padding and adjusted the main/right rail grid so the page does not sit as a compressed block inside the work area.
- After the latest user screenshot, enlarged the Todo canvas cap, widened the reminder rail, raised the focus card, increased list header and row heights, and centered the canvas so the screen no longer reads as a small compressed block in the upper-left work area.
- Verified no visible "证据" text remains under Flow components.

**Follow-up Polish**
- Use a real rendered screenshot to tune the exact 2K desktop spacing after the user runs the new package.

source visual truth path: `C:\Users\luwei\AppData\Local\Temp\codex-clipboard-9c836478-d426-4c0f-853a-43a1ec5f9582.png`
implementation screenshot path: unavailable; Browser/Chrome capture tool unavailable in this turn
viewport: intended desktop, same state as the supplied Todo design screenshot
state: Ariadne Flow Todo page with one active focus todo and reminder rail
full-view comparison evidence: source image opened locally; rendered implementation screenshot unavailable
focused region comparison evidence: unavailable because rendered screenshot capture was blocked
patches made since previous QA pass: removed squeezed row action buttons, corrected focus-card grid areas, reduced Todo page padding, adjusted main/right rail grid, then enlarged the Todo canvas cap to 1540px, widened the reminder rail to 360px, raised the focus card to 400px, increased list header and row heights, rebuilt and repackaged
final result: blocked
