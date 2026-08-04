# Repository Agent Instructions

- Do not hardcode key bindings in update, view, modal, selector, palette, or component logic.
- All keyboard shortcuts must be declared in `internal/keys/` and dispatched/displayed through the `internal/command` registry abstraction.
- If UI copy mentions a key, derive that text from a registry help view or palette entry rather than duplicating the key string.
- When adding a new interactive mode, add a mode-specific registry and wire dispatch, short help, full help, and palette entries through that registry.
- Do not leak new information across screens. The existing root model and current screen abstractions are designed to insulate hard-coded states. Lean on the root factory for actions specific to screen transitions
