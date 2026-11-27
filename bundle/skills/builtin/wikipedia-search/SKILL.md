---
name: wikipedia-search
description: Search Wikipedia and summarize retrieved articles. Use for 위키백과, 위키피디아, Wikipedia, or explicit Wikimedia Commons media searches.
requires-tools: [read_web_page]
triggers: [위키, wikipedia]
---

# Wikipedia Search

1. Prefer `en.wikipedia.org` for deeper coverage unless the user asks for another language. URL-encode query values and keep `format=jsonfm`.
2. The first web tool call MUST be `read_web_page` on the search API, not generic search routing:
   - Korean/non-English query (resolves exact English titles in one call):
     `https://ko.wikipedia.org/w/api.php?action=query&generator=search&gsrsearch=<QUERY>&gsrnamespace=0&gsrlimit=5&prop=langlinks&lllang=en&utf8=1&format=jsonfm`
   - English query (or translated fallback if `ko` has no good match):
     `https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=<EN_QUERY>&srnamespace=0&srlimit=5&utf8=1&format=jsonfm`
3. Search results give titles, NOT article evidence. Pick the right subject, then call `read_web_page` on `https://en.wikipedia.org/wiki/<EN_TITLE>` using the `en` langlink (or `en` search title); fall back to `https://ko.wikipedia.org/wiki/<KO_TITLE>` only when no English edition exists or Korean was requested. Replace spaces with underscores and URL-encode; read the rendered article, not `prop=revisions`.
4. If buffered evidence is insufficient, call `read_buffered_source` with `source_id` and a focused `query`. On read failure, retry once with `?action=render`; if that fails, report that content could not be verified.
5. Summarize retrieved facts in the user's language and link the article read. List-only requests may cite search matches as matches.

## Accuracy

Never replace missing article evidence with model memory, presumed encyclopedia structure, or a guessed biography. Empty results or an unrelated/disambiguation page do not support a factual summary.

## Commons Media

Use Commons only for explicit media searches, not encyclopedia topics:
`https://commons.wikimedia.org/w/api.php?action=query&list=search&srsearch=<QUERY>&srnamespace=6&srlimit=5&utf8=1&format=jsonfm`
Read the matched Commons file page for details and attribution.
