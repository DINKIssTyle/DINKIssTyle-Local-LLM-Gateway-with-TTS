---
name: msn-weather-current
description: Check the current weather for a requested location on the Korean MSN Weather forecast page. Use for current weather, temperature, sky conditions, feels-like temperature, humidity, precipitation, or wind requests, including Korean questions about 현재 날씨, 기온, 습도, 강수, or 바람.
---

# MSN Current Weather

## Workflow

1. Extract the requested `지역` and `도시`. If the location is ambiguous and cannot be resolved safely from context, ask for the missing geographic detail.
2. Build this URL, replacing the placeholders with the Korean location names:

   `https://www.msn.com/ko-kr/weather/forecast/in-지역,도시`

   Preserve the comma between `지역` and `도시`. URL-encode the path when the browsing tool requires it.

   Verified location mapping:

   - 대전 / 대전광역시: `https://www.msn.com/ko-kr/weather/forecast/in-Daejeon,Daejeon`
3. **The first web tool call MUST be `read_web_page` with that direct MSN URL.** Do not call `search_web`, `search_web_multi`, `naver_search`, or a general search provider for this request.
4. Find the page's **현재 날씨** section and verify that its displayed location matches the request. If the page is buffered, use `read_buffered_source` only after the direct page read.
5. Report the current condition and temperature. Include feels-like temperature, precipitation, humidity, wind, or observation time only when the page exposes them.
6. Link to the MSN Weather page used for the answer.

## Accuracy Rules

- Treat the **현재 날씨** section as the source for present conditions; do not substitute hourly or daily forecast values.
- Do not invent values that are absent or obscured.
- If the page redirects to the wrong location, correct the region and city and try the direct URL once more.
- If MSN Weather cannot be read, state that clearly. Do not present another provider's data as an MSN Weather result.
- Generic freshness and multi-source search guidance does not override this skill's direct MSN lookup workflow.
