# Flow uses a Self Model with conservative speaker attribution

Ariadne Flow should move toward becoming a "second me" by maintaining a local Self Model, but it must first use that model for internal understanding rather than unconfirmed external representation. Chat evidence must be attributed conservatively: explicit current evidence wins, confirmed Self Model data only assists interpretation, observed profile signals are weak, and model inference is lowest priority. This keeps Flow useful for user-centered summaries without letting another person's chat message become the user's action, commitment, or todo.

**Consequences**

The "Me" surface should be backed by stateful Self Assertions rather than only static profile fields. Flow may use low-risk confirmed identity and preference data in prompts, but sensitive or unrelated personal information stays local unless explicitly needed and allowed.
