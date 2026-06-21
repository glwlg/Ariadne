# Flow Todos And Skill

## Context

Flow can already infer unfinished items from Work Memory answers, but that is only prose inside a chat response. The user needs a durable Todo module that keeps follow-ups visible, editable, and callable by the Flow agent.

This module must also respect the speaker-attribution work from the Self Model change: a chat sentence that mentions "you" or "I" must not become the user's todo unless evidence shows the user is the sender, addressee, or explicit owner.

## Decision

Add "待办" as a first-class Flow surface and Work Memory domain object. A todo is not a memory entry and not a draft. It is an actionable item with status, priority, scope, evidence, optional due/reminder time, and local lifecycle metadata.

The Flow Memory skill must expose todo operations through the same controlled Work Memory CLI used by the agent:

- List todos before answering questions about unfinished work, reminders, follow-ups, pending approvals, or next actions.
- Add a todo only when the task and owner are clear from the user request or evidence.
- Update a todo by id for status changes such as doing, waiting, done, or canceled.
- Keep unclear chat attribution in the answer instead of silently writing a todo.

## Data Model

Todo fields:

- `id`
- `title`
- `note`
- `status`: `open`, `doing`, `waiting`, `done`, `canceled`
- `priority`: `low`, `normal`, `high`, `urgent`
- `scope`: project, app, group, or work area
- `source`: manual, agent, fallback, import, or other origin
- `evidence`: memory ids or source references
- `dueAt`
- `remindAt`
- `completedAt`
- `createdAt`
- `updatedAt`

Todo status semantics:

- `open`: the item is known but not started.
- `doing`: the user is actively working on it.
- `waiting`: blocked by another person, approval, environment, or external condition.
- `done`: completed and kept for history.
- `canceled`: no longer relevant.

## Storage

Persist todos in the same Work Memory SQLite store:

- `work_memory_todos`
- `work_memory_todo_evidence`

Todos are saved with the Work Memory state so they survive app restart and can be loaded by Wails, CLI, and agent tools.

## CLI And Agent Skill

Supported CLI actions:

- `ariadne.exe workmemory todos --status open --limit 20`
- `ariadne.exe workmemory todo-add --title "<todo>" --text "<note>" --priority normal --scope "<project>" --evidence "<memory-id>"`
- `ariadne.exe workmemory todo-update --id "<todo-id>" --status doing|waiting|done|canceled`
- `ariadne.exe workmemory todo-delete --id "<todo-id>"`

Supported Chat Tools:

- `list_flow_todos`
- `add_flow_todo`
- `update_flow_todo`

Native Responses shell skill uses the same CLI whitelist. OpenAI-compatible Chat Completions uses the same tool schemas through the custom compatibility loop.

Guardrails added after the false-save bug:

- A chat answer is not allowed to claim that a todo was saved unless `add_flow_todo` or `update_flow_todo` actually ran and returned `ok=true`.
- For user requests such as "保存待办", the compatible Chat Tools loop must force `add_flow_todo` on the first model turn instead of leaving `tool_choice=auto`.
- If the model responds with prose such as "已保存" without the required todo tool call, Flow must return an agent error instead of displaying a fake success.
- Because Agent tools run in a Python child process and write through the `ariadne.exe workmemory` CLI, the running desktop service must resync todos from SQLite after an agent answer before the UI reads the todo list.

## Conversation Context

The OpenAI Agents SDK only manages tool execution inside one run. Ariadne owns the durable Flow conversation and must pass recent messages into each new agent run.

Flow sends the latest user/assistant messages as `Conversation Context` so the agent can resolve follow-up references such as "刚才", "再加一次", "上面那个", or "继续". This context is only a reference-resolution aid:

- It must not replace Work Memory or Todo tool queries.
- It must not treat a previous assistant claim such as "已保存" as proof that a todo exists.
- If the current question says "你刚才没加成功，再加一次" and recent context contains the earlier concrete request "端午值班保存待办", the compatible Chat Tools loop should force `add_flow_todo` again.
- If recent context is insufficient to identify the target item, the agent must ask for the missing todo content instead of guessing.

## UI

The "待办" page is a standalone Flow route:

- Left: create or edit a todo.
- Center: status counters, filtering, and grouped todo list.
- Right: active follow-up summary and attribution boundaries.

The page supports manual add/edit/delete and fast status updates without requiring a chat turn.

## Acceptance Criteria

- Todos survive app restart.
- The Flow agent can list, add, and update todos through controlled tools.
- The Flow UI has a dedicated "待办" entry and independent page file.
- Questions such as "今天还有什么没办" can use the todo list and Work Memory evidence together.
- Chat attribution remains conservative: unclear quoted messages are not automatically written as the user's todo.
- "Saved" state is based on successful local tool output, not model wording.
- Todos written by the Agent CLI path are visible in the running app after refresh without requiring an app restart.
