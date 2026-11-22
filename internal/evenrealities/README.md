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

Hint research and source quality use the bundled
`internal/evenrealities/skills/trusted-information/SKILL.md`. The planner receives
only its active planning reference, without skill-discovery metadata or reference
routing text; both source-selection protocols receive its selection reference. These are explicitly embedded into the isolated hint requests rather
than depending on ordinary chat skill discovery or consuming its prompt budget. The skill and both references
participate in `PromptRevision` so hint events identify policy changes too.

The planner returns `search_query` separately from the information `task`.
The client carries it as `plan.searchQuery`; request assembly forwards it as
`hint_research.search_query`. Web lookup uses this source-focused query when
present, otherwise the original task; memory lookup always uses the task.
The task remains unchanged for answer selection. Source URLs accompany titles
and immutable sentences so the model can assess provenance without treating a
URL or an official-looking title as proof. No domain or advertising keyword
blacklist is added.

The context-task planner and source selector output budgets are 1024 tokens. Hint requests no longer
force reasoning off; the model's supported/default reasoning mode applies.
Translation retains its existing reasoning controls. The model deliberates
privately and returns the existing compact selection JSON; reference validation
still checks the selected immutable text independently. This can increase
inference latency, and instructions alone do not guarantee source quality.

Unit and integration tests verify assembly, packaging, query routing and output
contracts. For an opt-in behavioral evaluation against a configured real model:

```sh
DKST_HINT_SELECTION_LIVE_URL=http://127.0.0.1:1234/v1/chat/completions \
DKST_HINT_SELECTION_LIVE_MODEL='<loaded-model-id>' \
go test ./internal/evenrealities -run '^TestHintSourceQualityLive$' -v -count=1
```

`DKST_HINT_SELECTION_LIVE_TOKEN` is optional. The evaluation tests both selection
contracts with advertising/reflection-only results, vendor facts beside sales
copy, useful technical blogs, independent comparisons, and personal experience
that is appropriate only when requested. It uses synthetic source excerpts and
no live search, so it evaluates selection quality rather than search recall or
real-world source authenticity. Without an explicit endpoint and model it skips.


The judge has a 15s gateway deadline per attempt. The client has 34s for its
initial request and one schema repair, and the hint gate allows 38s so it cannot
cancel the client first. Deliberation is included in these budgets. A timed-out
or interrupted upstream stream returns a typed judge error instead of a normal
`request.complete` carrying empty/partial JSON. The client preserves transport
error codes and distinguishes a missing final answer from invalid grounding;
it does not retry these as schema repairs. The regression suite includes a valid
judgment arriving after the former 6s deadline.

To check the actual configured judge connection with synthetic speech:

```sh
DKST_HINT_JUDGE_LIVE_CONFIG='<gateway-config.json>' \
DKST_HINT_JUDGE_LIVE_MODEL_ROUTE='<configured-hint-model-route>' \
go test ./internal/core -run '^TestGlassesJudgeConfiguredModelLive$' -v -count=1
```

This checks completed, transcript-grounded research plans through the real
handler. It does not run web lookup, retrieve personal memory, or display a card
on the device.
