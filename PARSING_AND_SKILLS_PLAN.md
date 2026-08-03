# Parsing and skills architecture

## Parsing: one canonical boundary

The UI must not guess which upstream provider produced an event. The target
pipeline is:

1. Provider adapters decode OpenAI Chat Completions, LM Studio stateful events,
   and raw-JSON compatibility streams.
2. A server-side normalizer emits only the app event contract:
   `message.delta`, `reasoning.*`, `tool_call.*`, `chat.end`, and `error`.
3. Tool arguments are accumulated and JSON-decoded once. Textual/XML tool-call
   recovery remains a quarantined compatibility adapter, never a second normal
   execution path.
4. The browser SSE decoder handles transport details only. It never contains
   provider-specific tool or reasoning rules.
5. Markdown repair runs outside code and math, followed by one final CommonMark
   render and HTML sanitization.

The first step is implemented in the browser decoder: split CRLF boundaries,
multiline `data:` fields, EOF flushing, typed parse failures, and named events
are now covered by fixtures. The quoted-emphasis form frequently produced by
local models (for example `**'Model'**를`) is repaired without touching code
fences.

Next server refactor:

- Move provider event decoding out of `internal/core/server.go` into adapter
  files with table-driven fixtures captured from each provider.
- Give every event a request ID, turn ID, sequence number, and schema version.
- Reject duplicate terminal events and tool execution by call ID/signature.
- Fuzz SSE block decoding, tool-argument accumulation, and relaxed textual
  compatibility parsing.
- Record parse diagnostics as structured debug-trace entries without exposing
  hidden reasoning or credentials.

## Skill directory model

Bundled and user skills must never share a writable directory.

| Platform | Bundled, read-only | User, writable |
| --- | --- | --- |
| macOS | `App.app/Contents/Resources/skills/builtin` | `~/Library/Application Support/DKST LLM Chat Server/skills/user` |
| Windows | `<app>/skills/builtin` | `<app>/skills/user` |
| Linux | `<app>/skills/builtin` | `<app>/skills/user` |

Each skill is a directory containing `SKILL.md`; optional `references/`,
`scripts/`, and `assets/` directories are local to that skill. The server window
opens the writable user directory and creates a short guide on first use.

## Loader plan

1. **Discovery:** scan only one directory level, reject symlink escapes, hidden
   directories, duplicate IDs, oversized files, and missing `SKILL.md`.
2. **Validation:** parse a small frontmatter schema (`id`, `name`, `description`,
   `version`, `permissions`, `platforms`) and report errors per skill without
   failing the entire server.
3. **Namespacing:** use `builtin:<id>` and `user:<id>`. A user skill cannot
   silently replace a bundled skill.
4. **Compilation:** select skills per request, enforce a prompt-size budget, and
   attach their instructions once at the prompt assembly boundary.
5. **Permissions:** skills declare capabilities; actual tool permissions remain
   controlled by the server and user account policy. Instructions alone never
   grant access.
6. **Lifecycle:** cache by content hash, reload between requests, expose status
   and validation errors in the server window, and keep the previous valid
   version when an edit is temporarily invalid.

Recommended initial bundled skills are conservative instruction packs:

- `web-research`: source selection, evidence limits, and citation discipline.
- `memory-curation`: what belongs in durable, working, or ephemeral memory.
- `local-diagnostics`: collect focused diagnostics before proposing a fix.
- `help-guide`: answer product questions from bundled help before web search.

Command execution, filesystem mutation, and network automation should not ship
as always-on default skills. Those belong in explicit, permission-gated user
skills after the loader and audit UI are complete.
