# Ariadne

Ariadne is a desktop productivity assistant for quick launch, local context recall, capture, and focused utility tools. Its domain language centers on explicit user actions, local evidence, and separate surfaces for search, tools, and memory.

## Language

### Core Surfaces

**Launcher**:
The compact search-and-command surface used to find results and trigger actions. It is not the workspace for long-running tool flows.
_Avoid_: Dashboard, main window, home page

**Search Result**:
A ranked item returned by the launcher, such as a file, app, plugin trigger, workflow, clipboard item, capture, command, or setting.
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

**Legacy x-tools**:
The previous Python/PyQt runtime whose data can be imported and whose runtime behavior may coexist or conflict with Ariadne.
_Avoid_: x-tools mode, old app
