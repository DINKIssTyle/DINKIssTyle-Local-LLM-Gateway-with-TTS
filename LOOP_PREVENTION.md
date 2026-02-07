# 무한 반복 응답 차단 (Infinite Loop Prevention)

이 문서는 LLM이 동일한 텍스트를 무한히 반복 생성하는 것을 방지하기 위한 코드 로직과 위치를 설명합니다.

## 코드 위치 (Code Location)
- **파일 경로**: `frontend/app.js`
- **함수**: `streamResponse` 내부 (SSE 이벤트 처리 루프)
- **라인**: 약 1828 ~ 1844 라인 (버전에 따라 다를 수 있음)

## 구현 로직 (Implementation)
스트리밍되는 텍스트(`fullText`)를 실시간으로 분석하여, 특정 패턴이 과도하게 반복될 경우 생성을 강제로 중단(`stopGeneration`)하고 경고 메시지를 표시합니다.

### 감지 전략 (Dual Strategy)
1. **짧은 패턴 반복**: 5글자 이상의 짧은 문구가 5회 이상 연속으로 반복될 때 감지합니다.
   - 정규식: `/([\s\S]{5,}?)\1{4,}/`
2. **긴 패턴 반복**: 50글자 이상의 긴 문단이 3회 이상 연속으로 반복될 때 감지합니다.
   - 정규식: `/([\s\S]{50,}?)\1{2,}/`

## 코드 (Code Snippet)

```javascript
// --- LOOP DETECTION (Regex-based) ---
if (!loopDetected && fullText.length >= 50) {
    // Pattern: 5+ chars repeated 5+ times consecutively
    // Dual strategy: Short (5x) or Long (3x)
    const shortLoopMatch = fullText.match(/([\s\S]{5,}?)\1{4,}/);
    const longLoopMatch = fullText.match(/([\s\S]{50,}?)\1{2,}/);
    const loopMatch = shortLoopMatch || longLoopMatch;
    
    if (loopMatch && loopMatch[1].length >= LOOP_MIN_PATTERN_LENGTH) {
        console.warn(`[Loop Detection] Pattern detected: "${loopMatch[1].substring(0, 30)}..." repeated ${Math.floor(loopMatch[0].length / loopMatch[1].length)}+ times`);
        loopDetected = true;
        stopGeneration();

        // Add warning message to the display
        fullText += "\n\n" + t('warning.loopDetected');
    }
}
// --- END LOOP DETECTION ---
```
