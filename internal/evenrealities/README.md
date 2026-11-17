# Even Realities task module

`AssembleRequest` is the gateway entry point for the plugin's data-only
`glasses_request` envelope. This package has no dependency on `core`, the device
SDK, a database, or an LLM/network client. It only validates task data and builds
model requests. Non-actionable clarification tasks fail before inference. Templates are embedded and decoded once at process startup.

- `request.go`: versioned task contract, input limits, prompt selection, language,
  ambient/chat boundaries, and judgment output schema.
- `prompts/templates.json`: English LLM instructions and input templates for chat,
  judgment, planning, hint writing, translation, suggested replies, and verified
  source selection. A stable hash identifies the exact catalog in hint events. Raw transcripts/source quotations remain unchanged.
- `selection.go`: rebuild the verified-source selection request after the common
  gateway's search/grounding stage, retaining the resolved model protocol. Context source selection uses one record
  and its local sentence numbers, with a JSON schema only on compatible APIs.
- `request_test.go`: task/language combinations, request isolation, bounds, and
  a local assembly benchmark.

The plugin sends `model`, `stream`, and:

```json
{
  "glasses_request": {
    "version": 1,
    "task": "chat",
    "text": "The current question",
    "context": "Previous explicit AI turns",
    "target": "ko",
    "chatSession": true
  }
}
```

Tasks: `chat`, `judge`, `plan`, `answer`, `translation`, `translation_only`,
`translation_replies`, `translation_stream`, `translation_preview`. Judgment
receives separate recent/latest transcript fields. Hint answers receive a plan
and the original observed/anchor data. No system instructions or schema are
accepted from the plugin. Task metadata is stripped before forwarding to an LLM.
Unsupported versions/tasks fail rather than invoking a client prompt fallback.

Integration points:

1. `core/server.go`: call assembly before common authentication/settings/model
   routing and normalize to the actual default/additional model protocol.
2. `core/glasses_hint_grounding.go`: retain source lookup/verification in core,
   call this module for the source-selection request and static proof policies.
3. `core/llm_connections.go`: use the module’s short Native judgment contracts
   while keeping protocol conversion generic.
4. `core/evenrealities_request_test.go`: exercise actual standard/native handler
   routes and additional-connection account credentials using synthetic data.

For removal, delete this folder and these core integration calls/tests. Existing
`core/glasses_*.go` still owns STT/hint services, auditing and source verification;
removing this package alone does **not** remove all Even Realities functionality.
The WebView still owns microphone/gesture/display control, transcription queues,
short local context and card lifecycle. This is not a server-side audio rewrite.

`plugins/even-realities/test-support/gateway-prompts.mjs` reads this same catalog
only for UI regression tests. It is never imported into the browser bundle.
No old-plugin settings migration or client-prompt fallback is provided.

`contracts.go` owns internal fact schemas, per-record source-selection schemas and
the structural unresolved-task guard. Native APIs retain prompt-based output
contracts and independent reference checks; schema enforcement is not assumed.
