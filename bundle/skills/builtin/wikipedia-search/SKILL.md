---
name: wikipedia-search
description: Search Wikipedia and Wikimedia entries, summaries, and media via https://commons.wikimedia.org/w/api.php. Use for 위키백과, 위키피디아, 위키미디어, Wikimedia, or Wikipedia search requests.
---

# Wikimedia / Wikipedia Search

## Workflow

1. Extract the search keyword and URL-encode it (spaces as `%20`, Korean/non-ASCII percent-encoded).
2. **The first web tool call MUST be `read_web_page` on `https://commons.wikimedia.org/w/api.php` with `format=jsonfm`** (returns HTML-formatted JSON for fast HTTP reading). Do not call `search_web`, `search_web_multi`, or `naver_search` first.
   - Topic search (`ns=0`):
     `https://commons.wikimedia.org/w/api.php?action=query&list=search&srsearch=<QUERY>&srnamespace=0&srlimit=5&utf8=1&format=jsonfm`
   - Title/URL lookup:
     `https://commons.wikimedia.org/w/api.php?action=opensearch&search=<QUERY>&limit=5&format=jsonfm`
   - Media/file search (`ns=6`):
     `https://commons.wikimedia.org/w/api.php?action=query&list=search&srsearch=<QUERY>&srnamespace=6&srlimit=5&utf8=1&format=jsonfm`
3. For full page content and canonical URL of a matched title, call `read_web_page`:
   `https://commons.wikimedia.org/w/api.php?action=query&titles=<TITLE>&prop=info|revisions&rvprop=content&rvslots=main&inprop=url&utf8=1&format=jsonfm`
4. If buffered, use `read_buffered_source` with `source_id` and the user's question.
5. Answer in the user's language and link to the Wikimedia page (`fullurl` / `https://commons.wikimedia.org/wiki/...`).

## Accuracy Rules

- Report only facts, titles, and URLs present in the API response.
- If results are empty or no matching page exists, state that clearly without guessing.
