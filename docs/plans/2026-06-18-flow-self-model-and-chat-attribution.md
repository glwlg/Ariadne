# Flow Self Model And Chat Attribution

## Context

Flow currently answers questions about the user's day from local Work Memory evidence. A chat screenshot showed a failure mode where a message said by someone else was rewritten as if it was the user's own statement. This breaks the core trust contract for Flow: user-centered summaries must not turn other people's words into the user's actions, commitments, or todos.

The broader product goal is for Flow to become a "second me": a local assistant that understands the user's identity, work context, preferences, relationships, and boundaries well enough to summarize and reason from the user's point of view.

## Decision

Introduce a "Me" module backed by a Self Model. The Self Model is not just a flat profile form. It is a collection of Self Assertions with source, confirmation state, confidence, scope, privacy level, and model-context permission.

Flow should use this Self Model for internal interpretation first. It should not directly represent the user externally, reply to others, or make commitments without a confirmation path.

## Highest Principle

Flow should prefer saying less, or marking attribution as uncertain, over confidently assigning a message, action, commitment, or todo to the user when the evidence does not support it.

For chat evidence, this means:

- A quoted "you" or "I" belongs to the speaker's message context, not automatically to the current user.
- A message can become "the user said X" only when current evidence identifies it as the user's own message.
- A message can become "someone asked the user to X" only when current evidence supports that the user was addressed.
- If neither is clear, Flow should write "someone in the group mentioned X" or "the addressee is unclear."

## Product Scope

This is not limited to a single question type. The attribution rule applies anywhere Flow converts evidence into a user-centered answer, especially:

- Daily summaries.
- Contact and chat summaries.
- Todos and follow-ups.
- Risks, blockers, and commitments.
- Retrospectives, skills, checklists, and task packages derived from Work Memory.

Screenshot description can be looser, but it still must not invert speaker attribution.

## "Me" Module Shape

The product entry can be named "我". Internally, it should be organized into these areas:

- Identity: name, nicknames, account display names, role, team, current projects.
- Preferences: language, tone, answer length, work style, risk tolerance, decision preferences.
- Relationships: confirmed contacts, group chats, project members, and observed relationship candidates.
- Boundaries: what Flow may summarize automatically, what needs confirmation, and what must never be sent to an external model automatically.

## Assertion States

Every durable item in the Self Model should be represented as a Self Assertion:

- Confirmed assertion: manually entered or explicitly accepted by the user.
- Observed assertion: inferred from Work Memory, shown as a candidate with evidence and confidence.
- Rejected assertion: explicitly denied by the user, used to prevent the same bad inference.
- Ephemeral assertion: true only for one event or conversation and not part of the long-term Self Model.

Examples:

- "My name is luwei" is a confirmed, long-term assertion.
- "I work on DMS v2" may be confirmed or observed with project scope.
- "I often collaborate with Zhang Xiaoteng" may start as observed and require confirmation.
- "The 'look this afternoon' message is my todo" is an event-level assertion and must not become durable without evidence or confirmation.

## Privacy Tiers

Self Assertions should carry model-context permission:

- Always usable in prompts: low-risk confirmed name, preferred address, role, active projects, answer preferences, language and tone.
- Use only when relevant: age, city, company, team relationships, contact relationships, working hours, routine preferences.
- Never automatically sent: phone number, government ID, address, account secrets, credentials, private finance, medical information, family-sensitive information.

Sensitive fields may be stored locally, but they must not become part of the default prompt just because they are in the "Me" module.

## Answer Voice

Flow should internally reason from the user's point of view, but normal Flow answers should still address the user as "you".

Use first person only in explicit drafting modes, such as drafting a daily report, status update, reply, or self-description for the user to review.

## Chat Attribution Rules

The Self Model helps speaker attribution but cannot override evidence.

Evidence priority:

1. Explicit current evidence: message structure, active conversation title, adjacent speaker labels, message bubble side, account marker, OCR text, window title.
2. Confirmed Self Model data: user name, nicknames, account display names, known groups or contacts.
3. Observed Self Model data: recurring contacts, projects, collaboration patterns.
4. Model inference: lowest priority, never enough on its own.

Required distinctions:

- "I sent this" means the evidence identifies the user as the sender.
- "Someone addressed me" means the evidence identifies the user as the addressee.
- "Someone mentioned me" means the user appears in text, but assignment is unclear.
- "Someone said I said X" is a report or quote, not proof that the user said X.

For the screenshot failure mode, the safe answer shape is:

> In the group chat, people discussed pushing data to the real-time database and changing the data table. Zhang Xiaoteng said it was fixed. Someone also mentioned "look at it this afternoon", but the current evidence does not confirm who said it or whether it is the user's todo.

## Correction Loop

Flow should let the user correct attribution and turn corrections into Self Assertions:

- "This was not me."
- "This is my todo."
- "This person is a contact."
- "Do not treat this group as work-related."

Corrections should cite the underlying evidence and be reusable, so the same bad inference does not recur.

## MVP

First version should focus on the minimum feature set that directly improves Flow trust:

1. Add a "Me" surface with identity, preference, relationship, and boundary sections.
2. Store fields as Self Assertions with confirmation state and privacy tier.
3. Build a compact Self Model summary for Flow prompts using only allowed assertions.
4. Update chat attribution prompts and evidence guidance to distinguish user-authored messages, messages addressed to the user, mentions of the user, and quoted speech.
5. Use conservative wording when attribution is uncertain.
6. Add user correction actions for common attribution mistakes.

Out of scope for the MVP:

- Full contact CRM.
- Automatic external replies.
- Silent long-term learning of identity facts.
- Treating all second-person chat text as user-addressed or user-authored.

## Acceptance Criteria

- Flow does not turn another person's chat message into the user's action, commitment, or todo without evidence.
- Flow can say "unclear" for speaker attribution and still provide useful context.
- Manually confirmed Self Assertions outrank observed assertions.
- Observed assertions remain candidates until confirmed.
- Sensitive Self Assertions are not included in default model prompts.
- First-person output appears only in explicit drafting modes.
