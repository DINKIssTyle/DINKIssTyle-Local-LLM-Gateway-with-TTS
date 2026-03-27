# LLM 채팅 히스토리 구현 문제 및 해결

## 증상

LLM 채팅 앱에서 대화 히스토리를 켜면 이전 답변 내용이 다음 답변에 **반복/요약되어 나타나는 문제**가 발생했습니다.

### 예시
```
User: 내 이름은 하니
Assistant: 안녕하세요, 하니님!

User: 사과를 영어로
Assistant: 당신의 이름은 하니입니다. 사과를 영어로 하면 "apple"입니다.

User: 바나나를 영어로
Assistant: 안녕하세요, 하니님! 당신의 이름은 하니예요. 
          사과를 영어로 하면 apple이에요. 
          바나나를 영어로 하면 banana예요. ← 이전 답변 전부 반복!
```

## 원인 분석

### 잘못된 구현 (기존 코드)

```javascript
// 문제: slice로 최근 N개만 잘라서 보냄
const historyLimit = (parseInt(config.historyCount) || 0) + 1;
const history = messages.slice(-historyLimit).map(m => {...});
```

**문제점:**
1. User 메시지 push 직후에 slice하면, 이전 assistant 응답이 누락될 수 있음
2. `historyCount + 1` 방식은 user/assistant 쌍을 고려하지 않음
3. 불완전한 컨텍스트가 LLM에 전달되어 모델이 혼란스러운 응답 생성

### 올바른 구현 (Chrome 확장 프로그램 참고)

```javascript
// 전체 히스토리를 유지하고, 한도 초과 시 가장 오래된 것만 제거
const maxMessages = settings.maxHistory * 2;  // user+assistant 쌍
if (conversationHistory.length > maxMessages) {
    conversationHistory = conversationHistory.slice(-maxMessages);
}

// 전체 히스토리를 그대로 LLM에 전송
const messages = [
    { role: 'system', content: systemContent },
    ...conversationHistory
];
```

## 해결 방법

### 수정된 코드 (`frontend/app.js`)

```javascript
// Trim old messages if history exceeds limit (Chrome extension approach)
// historyCount = number of conversation turns (user+assistant pairs)
const maxMessages = (parseInt(config.historyCount) || 10) * 2;
if (messages.length > maxMessages) {
    // Remove oldest messages, keeping recent ones
    messages = messages.slice(-maxMessages);
}

// Map ALL current messages to API format
const history = messages.map(m => {
    if (m.image) {
        return { role: m.role, content: [...] };
    } else {
        let content = m.content || '';
        if (m.role === 'assistant') {
            // Remove think tags from history
            content = content.replace(/<think>[\s\S]*?<\/think>/g, '').trim();
        }
        return { role: m.role, content: content };
    }
});

const payload = {
    model: config.model,
    messages: [systemMsg, ...history],
    temperature: config.temperature,
    max_tokens: config.maxTokens,
    stream: true
};
```

## 핵심 차이점

| 항목 | 잘못된 방식 | 올바른 방식 |
|------|-------------|-------------|
| 히스토리 관리 | 매번 최근 N개만 slice | 전체 유지, 초과 시 오래된 것 제거 |
| historyCount 의미 | 메시지 개수 | 대화 턴 수 (user+assistant 쌍) |
| 전송 내용 | slice된 일부 메시지 | 전체 messages 배열 |
| 컨텍스트 일관성 | 불완전 (누락 가능) | 완전 (순서 보장) |

## 추가 개선 사항

1. **stop 토큰 및 penalties 제거**: LM Studio 등 서버가 모델에 맞게 설정한 기본값 사용
2. **think 태그 정리**: 히스토리에 포함되는 assistant 응답에서 `<think>...</think>` 제거
3. **시스템 프롬프트 강화**: "이전 답변을 반복하지 말 것" 명시

## 추가 발견: 치명적 버그

### 증상
LLM에 전송되는 `messages` 배열에 **assistant 응답이 포함되지 않음**:
```json
{
  "messages": [
    { "role": "system", "content": "..." },
    { "role": "user", "content": "첫번째 질문" },
    { "role": "user", "content": "두번째 질문" },  // assistant 없이 user만 연속!
    { "role": "user", "content": "세번째 질문" }
  ]
}
```

### 원인
`processStream` 함수에서 스트림 데이터 `[DONE]`을 받으면 **`return`으로 함수를 즉시 종료**:
```javascript
if (dataStr === '[DONE]') return;  // ← 여기서 종료되면...

// ... 이 코드가 실행되지 않음!
messages.push({ role: 'assistant', content: fullText });
```

### 해결
`return`을 `break`로 변경:
```javascript
if (dataStr === '[DONE]') break; // 루프만 종료, 함수는 계속
```

---
*Updated: 2026-01-18*
