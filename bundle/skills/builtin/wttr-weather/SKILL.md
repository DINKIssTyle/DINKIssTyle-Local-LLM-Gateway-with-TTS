---
name: wttr-weather
description: Fetch current weather, daily or time-slot forecasts, rain chances, and sunrise/sunset from wttr.in using curl. Use for 날씨, 기온, 체감온도, 강수확률, 우산, 시간대별 예보, 일출/일몰, or explicit wttr.in requests. For an explicit MSN request, use the MSN weather skill instead.
requires-tools: [execute_command]
---

# wttr.in 날씨

`execute_command`에서 `curl`로 조회한다. 질문에 맞는 형식을 먼저 선택하고 실제 응답에 있는 값만 답한다. 명령 실행 도구가 없으면 실행한 것처럼 말하지 말고 사용 불가를 알린다.

**이 스킬은 도구가 아니다. `builtin:wttr-weather`, `wttr-weather`, `curl`을 도구 이름으로 호출하지 않는다.** 실제 도구 이름은 `get_current_location`과 `execute_command`다. 스킬을 선택했다는 것은 아래 절차를 적용하라는 뜻이다. 잘못된 도구 이름 오류가 나면 같은 응답 안에서 실제 도구로 수정 호출하고 결과까지 확인한다. “다시 시도하겠습니다/잠시만 기다려 주세요”로 끝내지 않는다.

## 1. 위치 결정 — 이름이 없으면 반드시 좌표

- **이번 사용자 요청에 위치명이 있으면** 그 위치를 사용한다. 명시한 도시·국가·공항·랜드마크를 기기 위치로 덮어쓰지 않는다. 동명이면 국가/지역을 확인한다.
- **이번 요청에 위치명이 없으면 `get_current_location`을 `{}`로 호출하고 반환된 `lat`, `lon`을 사용한다.** 이번 요청을 위해 서버가 이미 조회한 위치 결과가 제공되었다면 재호출 없이 그 좌표를 사용한다.
- 이전 대화의 도시명, 서버 IP, 시간대, 예제의 서울 좌표를 현재 위치 대신 사용하지 않는다. 좌표를 도시명으로 역변환할 필요도 없다.
- URL은 `https://wttr.in/<lat>,<lon>`이다. **위도 먼저, 경도 다음**, 쉼표를 보존한다. 위도 -90~90, 경도 -180~180의 유한한 수를 사용한다. 0도도 유효하다.
- 위치 도구가 없거나 실패하거나 좌표가 없으면 사용자에게 위치명을 요청한다. 위치를 생략한 `curl wttr.in`으로 우회하지 않는다. 이는 게이트웨이 서버의 IP 위치를 조회한다.
- 위치명은 URL 경로 한 세그먼트로 UTF-8 인코딩한다. 응답의 `request`, `nearest_area`로 요청 지역이 맞는지 확인한다.

## 2. 사용자 요청 → 조회 형식

- `format=j2`: 현재 기온·체감·습도·바람, 일별 최고/최저, 일출/일몰. `current_condition[0]` 또는 대상 날짜의 `weather[]`를 읽는다.
- `format=j1`: 시간대별·아침/오후·강수확률·우산·하루 전반 요약. 대상 날짜의 `weather[].hourly[]`까지 읽는다.
- 한 줄·상태바·표·PNG·도시 비교의 옵션은 상세 문서를 참고한다. `format=1` 또는 `?1`로 날짜를 선택하지 않는다.

## 3. 실행과 해석

추가 옵션은 [curl 옵션·명령 예시·응답 해석](references/curl-options.md)을 참고한다. 일반 조회는 다음 실제 도구 호출 방식으로 수행한다.

예: “내일 아침”처럼 위치명이 없는 요청 → `get_current_location`에 `{}` 전달 → 반환 좌표로 **`execute_command`** 호출. 아래 좌표는 예시이며 실제 반환값으로 바꾼다.

```json
{"command":"curl --fail --silent --show-error --connect-timeout 5 --max-time 15 --get 'https://wttr.in/37.5665,126.9780' --data-urlencode 'format=j1' --data-urlencode 'lang=ko'"}
```

이 JSON은 `execute_command`의 인자다. `command` 안의 curl 문자열이 별도 도구 이름이 아니다. 이미 이번 요청의 위치 좌표가 공급되었으면 바로 `execute_command`를 호출한다.


현재값은 `current_condition[0]`, 예보는 `weather[].date`로 고른 날짜에서 읽는다. 오늘/내일은 요청 위치의 현지 날짜를 기준으로 하고 반환된 날짜 범위 밖의 날씨는 생성하지 않는다. 강수량과 강수확률을 구분한다. 기본 단위는 섭씨·km/h·mm이며 요청한 단위를 명시한다.

내일 아침은 내일 날짜의 `hourly[]` 중 06:00 이상 12:00 미만을 선택한다. `time`은 `600`=06:00, `900`=09:00인 HHmm 문자열이다. `tempC`, `FeelsLikeC`, `chanceofrain`(%), `precipMM`, `windspeedKmph`, `lang_ko[0].value` 또는 `weatherDesc[0].value`를 읽는다. 실제 제공된 시각의 예보임을 표시하고 번역되지 않은 상태 설명은 한국어로 번역한다.

## 4. 실패 처리와 답변

종료 코드와 JSON을 검증한다. HTML·오류 문구·필수 데이터 누락을 성공으로 취급하지 않는다.

연결 실패·타임아웃·429·5xx·잘못된 응답이면 동일 위치·쿼리로 `https://wttr.is/`에 한 번 재시도한다. 같은 백엔드이므로 독립 출처로 세지 않는다. 계속 실패하면 실패를 알리고 값을 생성하지 않는다. 다른 제공자 사용 시 출처를 표시한다.

요청 지역·대상 날짜/시간대·상태·기온을 중심으로 짧게 답한다. 요청한 세부 값과 단위를 포함하고 현재와 예보를 구분한다. 채팅에는 실제 조회 위치의 wttr.in 링크를 제공한다. TTS 문장에는 URL·ANSI 표·이모지를 읽어 넣지 않는다.

