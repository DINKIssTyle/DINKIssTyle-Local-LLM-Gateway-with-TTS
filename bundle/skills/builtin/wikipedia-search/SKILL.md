---
name: wikipedia-search
description: Search Wikipedia and summarize retrieved articles. Use for 위키백과, 위키피디아, Wikipedia, or explicit Wikimedia Commons media searches.
---

# Wikipedia Search

1. Choose the requested Wikipedia language (Korean: `ko.wikipedia.org`; English: `en.wikipedia.org`). URL-encode query values.
2. The first web tool call MUST be `read_web_page` on that Wikipedia search API, instead of generic search routing:
   `https://ko.wikipedia.org/w/api.php?action=query&list=search&srsearch=<QUERY>&srnamespace=0&srlimit=5&utf8=1&format=jsonfm`
   Change the host for other languages. Keep `format=jsonfm`: the page reader supports HTML-formatted JSON.
3. Search results identify titles, NOT article body evidence. Select the correct subject (e.g. actor versus novelist), then call `read_web_page` on:
   `https://ko.wikipedia.org/wiki/<TITLE>`
   Use the exact returned title; replace spaces with underscores and URL-encode it. Read this rendered article, not raw `prop=revisions` wikitext.
4. If buffered evidence is insufficient, call `read_buffered_source` with its `source_id` and a focused `query`. If article reading fails, try the matched article once with `?action=render`. If that also fails, stop and report that its content could not be verified.
5. Summarize retrieved facts in the user's language and link to the article. A request only to list search matches can be answered from search results, labeled as matches.

## Accuracy

Never replace missing article evidence with model memory, presumed encyclopedia structure, or a guessed biography. Empty results or an unrelated/disambiguation page do not support a factual summary.

## Commons Media

Use Commons only for explicit media searches, not encyclopedia topics:
`https://commons.wikimedia.org/w/api.php?action=query&list=search&srsearch=<QUERY>&srnamespace=6&srlimit=5&utf8=1&format=jsonfm`
Read the matched Commons file page for details and attribution.
