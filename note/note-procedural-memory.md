# 반복 요청 절차 기억(Procedural Memory) 설계안

## 목표

사용자의 장기 사실(memory)과 별개로, 자주 반복되는 요청을 **어떤 절차로 처리하면 가장 빠르고 안정적이었는지**를 저장한다.

예시:
- 날씨
- 미세먼지
- 시스템 메모리 상태
- 디스크 사용량
- 현재 시간

핵심은 "무슨 정보를 기억할까"가 아니라 아래를 기억하는 것이다.

- 이 요청은 어떤 `intent`였는가
- 어떤 tool chain으로 처리했는가
- 얼마나 빨랐는가
- 성공했는가
- 사용자가 바로 만족했는가
- 다음에도 같은 방식이 유효한가

이 계층은 사용자 사실 저장용 `memories` 테이블과 분리해야 한다.

---

## 왜 분리해야 하는가

현재 시스템의 `memories`는 아래 성격에 가깝다.

- 사용자 선호
- 장기 사실
- 프로젝트 맥락
- 대화 기반 회상 정보

반복 요청 최적화는 성격이 다르다.

- 결과 자체는 대부분 휘발성이다
- 대신 "처리 절차"는 재사용 가치가 있다
- 평가는 규칙 기반 점수와 latency로 가능하다
- 프롬프트 힌트나 라우팅 정책으로 재사용할 수 있다

즉, 이번 기능은 `semantic memory`가 아니라 `procedural memory`에 가깝다.

---

## 전체 구조

아래 4계층으로 나눈다.

1. 요청 정규화 계층
- 사용자 입력을 `intent_key`로 분류한다
- 같은 의미의 다양한 문장을 한 범주로 묶는다

2. 실행 기록 계층
- 실제 처리 중 사용한 tool chain과 성능을 기록한다

3. 집계/레시피 계층
- 자주 반복되는 요청의 대표 절차(recipe)를 만든다
- 평균 latency, 성공률, 품질 점수를 유지한다

4. 재사용 계층
- 다음 요청이 들어오면 상위 recipe를 시스템 프롬프트 또는 내부 라우터에 힌트로 준다

---

## 추천 DB 스키마

기존 `mcp/db.go`에 새 테이블을 추가하는 방향을 권장한다.

### 1. request_patterns

반복 요청의 대표 패턴을 저장한다.

```sql
CREATE TABLE IF NOT EXISTS request_patterns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    intent_key TEXT NOT NULL,
    sample_query TEXT NOT NULL,
    query_fingerprint TEXT NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 1,
    first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_request_patterns_user_intent
ON request_patterns(user_id, intent_key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_request_patterns_user_fingerprint
ON request_patterns(user_id, query_fingerprint);
```

설명:
- `intent_key`: `weather.current`, `air_quality.current`, `system.memory_status`
- `query_fingerprint`: 정규화된 질의의 해시
- `hit_count`: 같은 패턴이 반복된 횟수

### 2. request_executions

실제 처리 1회를 기록한다.

```sql
CREATE TABLE IF NOT EXISTS request_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    intent_key TEXT NOT NULL,
    request_pattern_id INTEGER,
    raw_query TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    tool_chain_json TEXT NOT NULL,
    tool_count INTEGER NOT NULL DEFAULT 0,
    total_latency_ms INTEGER NOT NULL DEFAULT 0,
    tool_latency_ms INTEGER NOT NULL DEFAULT 0,
    success INTEGER NOT NULL DEFAULT 0,
    fallback_used INTEGER NOT NULL DEFAULT 0,
    repeated_tool_blocked INTEGER NOT NULL DEFAULT 0,
    self_correction_used INTEGER NOT NULL DEFAULT 0,
    followup_within_2m INTEGER NOT NULL DEFAULT 0,
    user_feedback_score REAL NOT NULL DEFAULT 0,
    recipe_version TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(request_pattern_id) REFERENCES request_patterns(id)
);

CREATE INDEX IF NOT EXISTS idx_request_executions_user_intent
ON request_executions(user_id, intent_key, created_at DESC);
```

설명:
- `tool_chain_json`: 실행된 tool 이름, 인자 요약, 성공 여부, 개별 latency
- `followup_within_2m`: 사용자가 바로 다시 질문했는지
- `recipe_version`: 어떤 추천 recipe를 사용했는지 기록

### 3. procedure_recipes

반복 요청에 대해 재사용 가능한 "좋은 절차"를 집계 저장한다.

```sql
CREATE TABLE IF NOT EXISTS procedure_recipes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    intent_key TEXT NOT NULL,
    recipe_name TEXT NOT NULL,
    trigger_hint TEXT NOT NULL DEFAULT '',
    tool_chain_template_json TEXT NOT NULL,
    preconditions_json TEXT NOT NULL DEFAULT '{}',
    avg_latency_ms REAL NOT NULL DEFAULT 0,
    success_rate REAL NOT NULL DEFAULT 0,
    quality_score REAL NOT NULL DEFAULT 0,
    final_score REAL NOT NULL DEFAULT 0,
    usage_count INTEGER NOT NULL DEFAULT 0,
    last_used_at DATETIME,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_procedure_recipes_user_intent
ON procedure_recipes(user_id, intent_key, active, final_score DESC);
```

설명:
- `tool_chain_template_json`: 재사용용 절차 템플릿
- `final_score`: 실제 선택에 쓰는 종합 점수
- user별로 둘 수도 있고, 나중에는 전역 recipe도 별도 테이블로 분리 가능

### 4. optional: request_intent_stats

빠른 집계가 필요하면 intent 단위 통계를 둔다.

```sql
CREATE TABLE IF NOT EXISTS request_intent_stats (
    user_id TEXT NOT NULL,
    intent_key TEXT NOT NULL,
    total_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    avg_latency_ms REAL NOT NULL DEFAULT 0,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, intent_key)
);
```

이 테이블은 없어도 되지만 UI 통계나 빠른 랭킹에 유용하다.

---

## intent_key 설계

처음에는 LLM 분류보다 규칙 기반 분류를 추천한다.
초기 목적은 높은 정확도의 거친 묶음이지, 완벽한 NLU가 아니다.

### 초기 intent 후보

- `weather.current`
- `air_quality.current`
- `time.current`
- `system.memory_status`
- `system.cpu_status`
- `system.disk_status`
- `system.network_status`
- `system.process_status`
- `search.general`

### 분류 방식

1. 입력 소문자/trim
2. 숫자, 날짜, 지역 표현 일부 정규화
3. 한국어/영어 키워드 사전 매칭
4. 없으면 `search.general` 또는 `unknown`

예시 매핑:

- "오늘 서울 날씨" -> `weather.current`
- "지금 미세먼지" -> `air_quality.current`
- "메모리 얼마나 남았어" -> `system.memory_status`
- "현재 시간" -> `time.current`

### 중요한 원칙

- 결과는 휘발성이므로 저장하지 않는다
- intent와 절차만 저장한다
- 지역, 시간대, 장치 OS 같은 조건은 `normalized_query` 또는 `preconditions_json`으로 분리한다

---

## tool_chain_json 구조 제안

`request_executions.tool_chain_json`은 아래 형식을 권장한다.

```json
[
  {
    "tool": "search_web",
    "args_summary": "weather in Seoul",
    "latency_ms": 821,
    "success": true
  },
  {
    "tool": "read_buffered_source",
    "args_summary": "latest source for weather question",
    "latency_ms": 154,
    "success": true
  }
]
```

시스템 상태 확인처럼 내부 명령 기반 기능이 생기면 아래처럼 기록할 수 있다.

```json
[
  {
    "tool": "system_memory_status",
    "args_summary": "local machine",
    "latency_ms": 21,
    "success": true
  }
]
```

---

## 점수 계산 방식

처음부터 LLM 평가를 메인으로 두지 말고, 규칙 기반 점수 + 선택적 LLM 보조 평가를 권장한다.

### 1차 점수식

```text
score =
  success_bonus
  + latency_bonus
  + no_followup_bonus
  - fallback_penalty
  - repeated_tool_penalty
  - self_correction_penalty
```

예시:

- 성공: `+1.0`
- 1500ms 이하: `+0.5`
- 3000ms 이하: `+0.2`
- 2분 내 재질문 없음: `+0.4`
- fallback 사용: `-0.2`
- 동일 tool 반복 차단 발생: `-0.2`
- self-correction 발생: `-0.3`
- 실패: `-1.0`

### recipe 최종 점수

```text
final_score =
  success_rate * 0.45
  + normalized_quality * 0.35
  + normalized_speed * 0.20
```

또는 더 단순하게:

```text
final_score = average(execution_score)
```

초기에는 단순 평균으로 시작해도 충분하다.

---

## 평가 신호 정의

### 자동 수집 가능한 신호

- 요청 전체 latency
- tool별 latency
- 성공/실패
- fallback 발생 여부
- self-correction 발생 여부
- 같은 요청에서 중복 tool 차단 여부
- 직후 follow-up 질문 여부

### 나중에 추가 가능한 신호

- 사용자의 thumbs up/down
- 답변 길이 대비 만족도
- 재시도 없이 종료된 비율
- 같은 intent에서 사용자가 특정 recipe를 선호하는 경향

---

## 현재 코드 기준 삽입 포인트

### 1. 요청 진입 직후

파일:
- `server.go`

위치:
- `handleChat()`에서 `reqMap` 파싱 후

여기서 할 일:
- 마지막 user message 추출
- `intent_key` 계산
- `normalized_query` 계산
- `request_pattern` upsert
- 실행 컨텍스트 생성

추천 구조:

```go
type RequestExecutionContext struct {
    UserID            string
    IntentKey         string
    RawQuery          string
    NormalizedQuery   string
    PatternID         int64
    TurnStart         time.Time
    ToolEvents        []ToolExecutionEvent
    Success           bool
    FallbackUsed      bool
    SelfCorrection    bool
    RepeatedBlocked   bool
    FollowupObserved  bool
}
```

### 2. tool 실행 시점

파일:
- `server.go`

위치:
- `toolExecutedThisTurn` 처리 블록 내부

여기서 할 일:
- tool 이름
- 인자 요약
- start/end 시간
- 성공/실패

를 `ToolExecutionEvent`로 누적한다.

### 3. self-correction 지점

파일:
- `server.go`

위치:
- `needsCorrection` 처리 블록

여기서 할 일:
- `SelfCorrection = true`

### 4. 요청 종료 직전

파일:
- `server.go`

위치:
- `request.complete` 디버그 트레이스 직전 또는 직후

여기서 할 일:
- `request_executions` insert
- intent 통계 집계
- recipe 점수 갱신 트리거

### 5. 다음 요청 프롬프트 주입 직전

파일:
- `server.go`
- `mcp/prompts.go`

여기서 할 일:
- 해당 `intent_key`의 상위 recipe 1개 조회
- 짧은 지시문으로 시스템 프롬프트에 삽입

예시:

```text
PROCEDURAL HINT:
For system.memory_status, the fastest successful strategy for this user was:
1. use local system status tool directly
2. avoid web search
Prefer the shortest direct path if the user is asking for current machine state.
```

중요:
- 힌트는 짧고 보수적으로
- 강제 규칙이 아니라 preference 수준으로

---

## 구현 단계 제안

### Phase 1. 로깅만 추가

목표:
- 반복 요청을 분류하고 실행 과정을 저장한다
- 아직 자동 재사용은 하지 않는다

구현:
- DB 테이블 추가
- `intent_key` 규칙 기반 분류기 추가
- request execution context 추가
- tool chain 기록 추가

장점:
- 위험이 가장 낮다
- 실제 데이터 분포를 먼저 볼 수 있다

### Phase 2. 집계와 recipe 생성

목표:
- 특정 intent의 반복 처리 절차를 요약한다

구현:
- background aggregator 또는 요청 종료 시 경량 집계
- 상위 tool chain을 recipe로 저장

예시:
- `weather.current`: `search_web -> read_buffered_source`
- `system.memory_status`: `system_memory_status`

### Phase 3. 프롬프트 힌트 재사용

목표:
- 다음 요청에서 더 빠르게 동일한 절차를 유도한다

구현:
- 상위 recipe 1개를 system prompt에 짧게 주입

주의:
- 아직 자동 강제 라우팅은 하지 않는다
- 잘못 학습되었을 때 부작용을 줄이기 위해 힌트 수준으로만 사용

### Phase 4. 내부 라우팅 최적화

목표:
- 특정 intent는 LLM이 돌기 전에 직접 빠른 tool path를 선택할 수 있게 한다

예시:
- `time.current` -> 직접 처리
- `system.memory_status` -> 로컬 시스템 tool 직접 처리

이 단계는 가장 공격적이므로 마지막에 두는 것이 안전하다.

---

## follow-up 판정 규칙

만족도 대체 신호로 follow-up을 사용할 수 있다.

### 단순 규칙

- 같은 user가 2분 이내에 같은 intent로 다시 질문하면 `followup_within_2m = 1`
- 다른 intent면 별도 처리
- "다시", "아닌데", "틀렸어", "업데이트" 같은 표현은 부정 신호로 가중 가능

### 주의

- follow-up은 항상 실패 의미가 아니다
- 대화 확장일 수도 있다

그래서 초기에는 약한 패널티만 주는 것이 좋다.

---

## LLM을 어디까지 쓸 것인가

LLM은 아래에만 제한적으로 쓰는 것을 추천한다.

- 드물게 `intent_key` 후보를 보조 추론
- 여러 실행 로그를 보고 사람이 읽을 수 있는 recipe 설명 생성
- 향후 고급 집계 분석

LLM에 맡기지 않는 것:

- 핵심 점수 계산
- 기본 intent 분류
- 실행 성공 판정
- DB 저장 여부 판단

즉, "운영 판단"은 규칙 기반, "설명"은 LLM 기반이 안정적이다.

---

## 추천 초기 범위

첫 구현은 아래 범위만 권장한다.

- `weather.current`
- `air_quality.current`
- `time.current`
- `system.memory_status`
- `system.disk_status`

그리고 아래까지만 구현한다.

- intent 분류
- request execution 저장
- 간단한 점수 계산
- 집계 조회 API 또는 내부 함수

프롬프트 재사용은 그 다음 단계에서 붙인다.

---

## 예상 장점

- 자주 쓰는 요청의 응답 시간이 점점 안정화됨
- 불필요한 web search 반복 감소
- 내부 시스템 상태 요청은 점차 직접 처리 경로로 수렴 가능
- 사용자별 습관을 반영한 맞춤 최적화 가능

---

## 예상 리스크

### 1. 잘못된 intent 분류

대응:
- 초기 카테고리를 적게 유지
- `unknown` 허용

### 2. 나쁜 절차가 우연히 높은 점수를 받음

대응:
- 최소 표본 수 조건 도입
- 예: 3회 이상 성공한 recipe만 재사용

### 3. 프롬프트 힌트가 과적용됨

대응:
- 힌트는 1개만
- 강제하지 말고 "prefer" 수준 유지

### 4. DB가 빠르게 커짐

대응:
- `request_executions`는 최근 N일 보관 또는 샘플링
- 집계값은 별도 유지

---

## 다음 구현 우선순위

1. `mcp/db.go`
- 새 테이블 생성
- upsert/search helper 추가

2. 새 파일 추가 권장
- `procedural_memory.go`
- `request_intent.go`

3. `server.go`
- request execution context 생성
- tool event 누적
- 종료 시 DB 저장

4. 이후
- 상위 recipe 조회
- system prompt 힌트 주입

---

## 결론

이 기능은 기존 장기 메모리의 확장이 아니라, 별도의 "절차 기억 레이어"로 설계하는 것이 맞다.

초기 구현은 반드시 아래 순서가 좋다.

1. 기록
2. 집계
3. 힌트 재사용
4. 자동 라우팅

이 순서를 지키면 과적응이나 잘못된 최적화 없이, 실제 사용 데이터를 기반으로 안정적으로 발전시킬 수 있다.
