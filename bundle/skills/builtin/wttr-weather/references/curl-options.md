# curl 옵션·명령 예시·응답 해석

위치는 SKILL.md의 규칙으로 먼저 결정한다.

## 3. curl 명령과 정확한 값

아래 좌표는 문법 예시다. 실행할 때는 1단계에서 얻은 위치로 바꾼다.

```bash
# 현재 날씨 또는 일별 최고/최저/천문 정보
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  --get 'https://wttr.in/37.5665,126.9780' \
  --data-urlencode 'format=j2' --data-urlencode 'lang=ko'

# 시간대별 예보, 하루 전반 요약, 강수확률
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  --get 'https://wttr.in/37.5665,126.9780' \
  --data-urlencode 'format=j1' --data-urlencode 'lang=ko'
```

| curl 인자 | 의미 |
|---|---|
| `--fail` | HTTP 400 이상을 실패로 처리 |
| `--silent --show-error` | 진행률은 숨기고 오류는 표시 |
| `--connect-timeout 5` | 연결 대기 최대 5초 |
| `--max-time 15` | 한 요청 전체 최대 15초 |
| `--get` | 뒤의 데이터를 GET 쿼리로 전송 |
| `--data-urlencode 'format=j1'` | 쿼리 값을 인코딩하여 전송; 요청에 맞게 `j2` 등으로 교체 |
| `--data-urlencode 'lang=ko'` | 한국어 요청; 영어는 `lang=en`, 일본어는 `lang=ja` |

URL 및 인자는 셸에서 안전하게 인용한다. 사용자 문장을 셸 코드로 끼워 넣지 않는다. 공백·`&`·`%`가 있는 쿼리는 `--data-urlencode`로 전달하고 중복 인코딩하지 않는다. Windows PowerShell에서는 별칭 충돌을 피하려고 `curl.exe`를 사용한다.

JSON은 단위가 명시된 필드를 선택한다. 기본은 `temp_C`, `FeelsLikeC`, `windspeedKmph`, `precipMM`이다. 화씨 요청은 `temp_F`, `FeelsLikeF`, 풍속 mph 요청은 `windspeedMiles`를 선택한다. m/s 요청은 `windspeedKmph / 3.6`으로 변환한다. JSON 키 자체가 단위를 정하므로 아래 `m`, `u`, `M` 옵션으로 JSON 키나 값의 단위가 바뀐다고 가정하지 않는다.

## 4. 표시용 옵션

일반 자연어/TTS 답변에는 JSON을 사용한다. 사용자에게 원문/표/이미지가 필요할 때만 아래를 선택한다.

| 옵션 | 정확한 의미 |
|---|---|
| `format=1` | 현재 상태 아이콘 + 기온 |
| `format=2` | 현재 상태 아이콘 + 기온 + 바람 등 |
| `format=3` | 위치명 + 현재 상태 아이콘 + 기온 |
| `format=4` | 위치명 + 현재 상태 아이콘 + 기온 + 바람 등 |
| 사용자 지정 `format` | `%l` 위치, `%C` 상태 텍스트, `%c` 아이콘, `%t` 기온, `%f` 체감, `%h` 습도, `%w` 바람, `%p` 강수량(mm/3시간), `%P` 기압(hPa), `%S` 일출, `%s` 일몰 |
| `m` / `u` / `M` | 미터법 / 미국식 / 미터법에 풍속 m/s. 대소문자 구분, 하나만 선택 |
| `0` / `1` / `2` (단독 쿼리 플래그) | 현재만 / 현재+오늘 예보 / 현재+오늘+내일 예보. 특정 날짜 선택자가 아님 |
| `T` | ANSI 색상·제어문자 제거 |
| `n` | 낮·밤 중심의 좁은 표 |
| `q` / `Q` | 제목 생략 / 제목과 도시명 생략 |
| `F` | Follow 안내 생략 |

```bash
# 위치를 포함한 간단한 한 줄, 섭씨·미터법
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  --get 'https://wttr.in/37.5665,126.9780' \
  --data-urlencode 'format=3' --data-urlencode 'lang=ko' --data-urlencode 'm'

# 아이콘 없는 사용자 지정 한 줄
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  --get 'https://wttr.in/37.5665,126.9780' \
  --data-urlencode 'format=%l: %C, %t, feels like %f, humidity %h, wind %w' \
  --data-urlencode 'lang=ko' --data-urlencode 'm'

# 현재+오늘+내일 표, 색상 제거, 미터법
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  'https://wttr.in/37.5665,126.9780?m2TF&lang=ko'

# 사용자가 PNG 파일을 요청한 경우에만 파일로 저장
curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
  'https://wttr.in/37.5665,126.9780_0_lang=ko.png' --output weather.png
```

짧은 플래그는 `?m2TF`처럼 결합하고 값이 있는 옵션은 `&lang=ko`로 구분한다. PNG는 옵션 구분에 `?` 대신 `_`를 사용한다. 저장 경로는 사용자 요청에 맞춘다. JSON 원문 요청에는 필요한 `j1`/`j2`를 선택한다. 여러 도시의 JSON은 각 위치를 따로 조회한다. 실험적인 `v2`/`v3` 화면은 일반 날씨 답변에 사용하지 않는다.

## 5. 응답 해석과 날짜

- 현재: `current_condition[0]`의 `temp_C`/`temp_F`, `FeelsLikeC`/`FeelsLikeF`, `humidity`(%), `windspeedKmph`/`windspeedMiles`, `winddir16Point`, `precipMM`, `pressure`(hPa), `uvIndex`를 사용한다. 현재 강수량을 강수확률로 읽지 않는다.
- 일별: **`weather[].date` 문자열로 대상 날짜를 선택**한다. 최고/최저는 `maxtempC`/`mintempC` 또는 F 필드다. 배열 순서만 보고 오늘/내일이라고 단정하지 않는다.
- 시간대별: `hourly[].time`은 `0`=00:00, `300`=03:00, `1200`=12:00 같은 HHmm 숫자 문자열이다. 실제 반환된 간격을 사용한다. `tempC`/`tempF`는 현재 항목의 `temp_C`/`temp_F`와 키가 다르다.
- 아침은 기본 06:00~12:00 미만, 오후는 12:00~18:00 미만, 저녁은 18:00~24:00 미만으로 읽되 사용자 지정 시간이 우선이다. 정확한 시각 데이터가 없으면 가장 가까운 제공 시각임을 밝힌다. 내일 아침 요청에 오늘 현재값으로 답하지 않는다.
- `chanceofrain`/`chanceofsnow`는 해당 예보 시간대의 %다. 하루/구간 요약은 범위 또는 최대값임을 명시한다. 임의 평균을 공식 일일 강수확률처럼 말하지 않는다. 우산 조언은 해당 구간의 확률·상태·강수량에 근거한다.
- 오늘/내일/모레는 요청 위치의 현지 날짜 기준이다. 클라이언트 시각을 참고하되 해외 위치에 한국 날짜를 무조건 적용하지 않는다. 시간대를 확인할 수 없고 날짜 경계가 중요하면 확인 후 답한다.
- 제공 가능한 날짜는 **반환된 `weather[].date` 목록**이 기준이다. 주간·다음 주·과거가 범위 밖이면 제공 범위를 알린다. `num_of_days`, `date`, `hour` 같은 확인되지 않은 쿼리로 기간을 늘릴 수 있다고 만들지 않는다.
- 상태 설명은 `lang_ko[0].value` 등 요청 언어 필드를 우선하고 없거나 번역되지 않았으면 `weatherDesc[0].value`의 의미를 번역한다. `lang=ko`만으로 모든 상태가 한국어라고 가정하지 않는다.
- 관측 시각은 응답이 명확히 제공할 때만 표시한다. `observation_time`의 날짜·시간대를 추정하지 않는다. 날짜/시간대 없는 시각만으로 최신성이나 몇 분 전 관측인지 단정하지 않는다.


공식 옵션: https://wttr.in/:help
JSON: https://github.com/chubin/wttr.in#different-output-formats

## 위치명 인코딩과 반환 위치 확인

- 위치명은 URL 경로의 한 세그먼트로 UTF-8 퍼센트 인코딩한다. 예: `서울` → `%EC%84%9C%EC%9A%B8`, `New York` → `New%20York`. 좌표는 숫자와 쉼표를 사용한다. 위치 입력으로 호스트나 경로 형식을 바꾸지 않는다.
- 응답의 `request`, `nearest_area`로 국가·좌표가 요청 지역에 부합하는지 확인한다. 최근접 지점 이름은 요청한 행정구역명과 다를 수 있다. 맞지 않으면 이름에 국가를 추가하거나 올바른 좌표로 한 번 수정한다.


## 요청별 형식 상세표

| 사용자 요청 | `format` 값 | 사용할 데이터 |
|---|---|---|
| 지금 날씨, 현재 기온·체감·습도·바람 | `j2` | `current_condition[0]` |
| 오늘/내일/모레 최고·최저기온 | `j2` | 대상 날짜의 `weather[]` |
| 일출·일몰·달 위상 | `j2` | 대상 날짜의 `weather[].astronomy[0]` |
| 시간대별, 내일 아침/오후, 특정 시각 | `j1` | 대상 날짜의 `weather[].hourly[]` |
| 오늘/내일 비 와? 강수확률? 우산 필요해? | `j1` | 대상 구간의 `chanceofrain`, `precipMM` 등 |
| 오늘 날씨 / 내일 날씨처럼 하루 전반 요약 | `j1` | 최고·최저와 시간대별 상태·강수확률 |
| 여러 도시 비교 | 필요에 따라 `j1` 또는 `j2` | 도시마다 같은 형식으로 따로 조회 |
| 터미널 한 줄 / 상태바 / 표 / PNG | 아래 표시용 옵션 | 사용자가 해당 출력을 원할 때만 |

`j2`는 시간대별 데이터를 제외한 작은 JSON, `j1`은 시간대별 데이터를 포함한 JSON이다. `format=1`과 `?1`은 날짜 선택자가 아니다.

