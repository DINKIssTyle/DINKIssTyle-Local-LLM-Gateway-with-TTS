# Tool + LLM evaluation testbed

This testbed calls the configured OpenAI-compatible LLM endpoint and can execute the app's real read-only tool registry. It uses the same bundled-skill loader and prompt injection path as the app, and covers skill-backed flows, tool-backed web/retrieval flows, and ordinary conversations such as stable knowledge, arithmetic, translation, rewriting, creative writing, history follow-ups, and passive memory context.

Every report records per-round request bytes, system-prompt characters, tool-schema bytes, latency, finish reason, token usage, cached/reasoning tokens when provided, protocol corrections, tool traces, retrieved URLs, and the final answer. Deterministic checks cover tool/no-tool behavior, LLM-round and answer-length budgets, required/forbidden content, distinct search angles, citations, duplicates, unsupported future dates, and authoritative-source coverage.

Secrets are read from `.env.eval.local` or process environment variables and are never written to reports.

```dotenv
DKST_EVAL_ENDPOINT=http://127.0.0.1:8094
DKST_EVAL_MODEL=qwen3.6-35b-a3b
DKST_EVAL_API_KEY=replace_me
# Optional overrides; defaults to bundle/skills/builtin and no user-skill directory.
DKST_EVAL_BUILTIN_SKILLS_DIR=bundle/skills/builtin
DKST_EVAL_USER_SKILLS_DIR=
```

List scenarios without using the API:

```bash
go run ./cmd/tool-eval -list
```

Isolate the real search provider and parser path without calling the LLM:

```bash
go run ./cmd/tool-eval -search '현재 미국 뉴스'
```

Run the primary regression scenario:

```bash
go run ./cmd/tool-eval -scenario latest_migration_news
```

Run all scenarios:

```bash
go run ./cmd/tool-eval -all
```

Run only the ordinary-conversation and control scenarios:

```bash
go run ./cmd/tool-eval -scenario-prefix general_
```

Scenario controls include:

- `enable_tools`: disable app tools for an ordinary-conversation control.
- `tool_names`: expose only a named subset of the safe read-only tools.
- `history`: provide visible prior user/assistant turns.
- `context_strategy` plus memory fields: evaluate injected retrieval context.
- `reasoning_effort`: compare provider reasoning levels such as `none` and the automatic default.
- `expectations.required_skills`: require exact selected namespaces such as `builtin:msn-weather-current`.
- `expectations.max_llm_rounds`, `max_answer_chars`, and `forbid_tool_call`: enforce efficiency and instruction-following budgets.

Reports are written under `.eval-results/` with mode `0600`. A report never contains the API key.
