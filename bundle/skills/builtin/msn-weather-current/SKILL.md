---
name: msn-weather-current
description: Check current local weather on Korean MSN Weather, with or without a specified place. Use for weather, temperature, sky, feels-like, humidity, precipitation, wind, 현재 날씨, 기온, 습도, 강수, or 바람 requests.
requires-tools: [read_web_page]
triggers: [날씨, 현재 기온, 지금 기온, current weather, weather right now]
priority: 10
---

# MSN Current Weather

## Workflow

1. Resolve the location in this order:
   - Use the place requested in this turn or established in recent conversation; never override it with device location.
   - If no place was specified (e.g. `지금 밖에 날씨 어때?`), use supplied client location or call `get_current_location` with `{}`. Do not repeat a lookup already performed by the server.
   - JSON `lat`/`lon` are usable location information. If names are missing, look up the coordinates to establish `지역` and `도시`. Geographic search is allowed only for this preparation, not for weather data. Do not guess a city from coordinates or timezone.
   - Ask for a place only if context and available location lookup cannot establish one, or an explicitly requested place remains ambiguous.
2. Build `https://www.msn.com/ko-kr/weather/forecast/in-지역,도시` using the resolved Korean names. Preserve the comma; URL-encode when required.
3. After geographic preparation, the **first web tool call MUST be `read_web_page`** with that direct MSN URL for weather. Do not use `search_web`, `search_web_multi`, `naver_search`, or general search for weather values.
4. Read **현재 날씨** and verify the displayed location. If the excerpt is insufficient, use `read_buffered_source` with 현재 날씨 and the location. If redirected to a wrong place, correct the URL and retry once.
5. Report the resolved place, condition and temperature, linking the MSN page. Include feels-like, precipitation, humidity, wind or observation time only when exposed.

## Accuracy Rules

- Use 현재 날씨, not hourly/daily forecast values. Never invent missing values.
- If MSN cannot be read, say so; do not label another provider's data as MSN.
- Generic freshness/multi-source guidance does not override this workflow.
