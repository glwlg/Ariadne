# Ariadne

Ariadne is a desktop productivity assistant for quick launch, local context recall, capture, and focused utility tools. Its domain language centers on explicit user actions, local evidence, and separate surfaces for search, tools, and memory.

## Language

### Core Surfaces

**Launcher**:
The compact search-and-command surface used to find results and trigger actions. It is not the workspace for long-running tool flows.
_Avoid_: Dashboard, main window, home page

**Search Result**:
A ranked item returned by the launcher, such as a file, app, plugin trigger, workflow, clipboard item, capture, memory, command, or setting.
_Avoid_: Row, item, card

**Preview**:
The contextual summary shown for the selected search result, including its metadata, evidence, and available actions.
_Avoid_: Details panel, inspector

**Action**:
An explicit operation attached to a search result, such as opening, copying, pinning, running, remembering, or requiring confirmation.
_Avoid_: Button behavior, inferred command

**Tool Window**:
An independent window that hosts a focused tool experience outside the launcher.
_Avoid_: Launcher page, embedded dashboard

**Tool Center**:
The full workspace for a focused tool such as settings, Hosts, workflow, clipboard, screenshot history, or JSON compare.
_Avoid_: Page, modal

### Flow And Work Memory

**Flow (心流)**:
The conversation-first surface for asking questions about recent local work context and reviewing the evidence behind answers.
_Avoid_: Work memory center, chatbot, activity dashboard

**Work Memory**:
The local body of captured, imported, or manually added evidence that Ariadne can search, summarize, and use to answer Flow questions.
_Avoid_: Flow memory, activity log, telemetry

**Memory Entry**:
A single piece of work memory evidence, such as a note, screenshot-derived record, clipboard record, or captured window context.
_Avoid_: Event, log line, record

**Evidence**:
A memory entry or source reference used to justify an answer, draft, insight, workflow, checklist, or task package.
_Avoid_: Citation, proof, attachment

**Time Machine**:
The automatic capture source that records stable foreground work context over time.
_Avoid_: Screen recorder, background stream

**Pending Capture**:
A newly captured memory entry that has not yet passed quality review and should not drive answers, drafts, insights, or agent evidence.
_Avoid_: Fresh evidence, unchecked screenshot

**Sensitive Entry**:
A memory entry that may contain credentials or private material and is excluded from normal answers, exports, and external processing unless explicitly allowed.
_Avoid_: Secret, blocked item

**Privacy Mode**:
The user-controlled state that prevents Ariadne from using local work memory in ways that could expose private context.
_Avoid_: Incognito mode, disabled memory

### Capture And Recall

**Screenshot History**:
The user-visible collection of saved screenshot captures and selections.
_Avoid_: Capture log, screenshot cache

**Clipboard History**:
The user-visible collection of recent text and image clipboard entries.
_Avoid_: Clipboard log, paste cache

**Pinned Image**:
An always-on-top image window opened from a screenshot, selection, clipboard image, or QR result.
_Avoid_: Screenshot preview, floating toolbar

**Image OCR Index**:
Searchable text extracted from recent screenshot and clipboard images after sensitive content is handled.
_Avoid_: Image search cache, OCR dump

### Tools And Automation

**Plugin**:
A built-in or local command provider that exposes trigger results, plugin results, and plugin-owned actions through the launcher.
_Avoid_: Extension, add-on, bridge

**Workflow Macro**:
A saved chain of commands that can pass user input, clipboard content, or prior step output between steps.
_Avoid_: Script, automation, command list

**Custom Launcher**:
A user-defined shortcut for opening an app, file, folder, URL, or command from the launcher.
_Avoid_: Bookmark, custom command

**Hosts Profile**:
A named local or remote hosts-file profile that can be previewed, conflict-checked, and applied after confirmation.
_Avoid_: Hosts template, hosts snippet

**Network Mini**:
The compact network monitor surface for quick traffic visibility.
_Avoid_: Network widget, traffic badge

**Settings Center**:
The tool center for configuring Ariadne behavior, credentials, diagnostics, migration, and rollback state.
_Avoid_: Preferences page, config screen

**Skill Asset**:
A confirmed local reusable instruction package distilled from work memory evidence or drafts.
_Avoid_: Workflow draft, note, installed skill

**Task Package**:
A reviewable bundle of goal, context, evidence, boundaries, and acceptance criteria for an agent to act on.
_Avoid_: Prompt, issue, checklist

**Legacy x-tools**:
The previous Python/PyQt runtime whose data can be imported and whose runtime behavior may coexist or conflict with Ariadne.
_Avoid_: x-tools mode, old app
