# Chrome STT·번역 활용 검토 — 2026-11-13

이번 작업은 기능 검토와 가용성 검사이며 앱의 STT 엔진을 교체하지 않았다. 마이크 사용, 음성 업로드, 언어팩 설치, 기존 Chrome 프로필 변경은 하지 않았다.

## 직접 확인한 내용

별도 임시 프로필과 localhost 테스트 페이지로 검사했다.

| 실행 파일 | API 노출 | 한국어 로컬 STT | 한국어→영어 Translator |
| --- | --- | --- | --- |
| 설치된 Google Chrome 157.0.8103.52의 `--headless` | SpeechRecognition, processLocally, Translator, LanguageDetector | `downloadable` 응답 | `downloadable` 응답 |
| 게이트웨이에 설치된 chrome-headless-shell 157.0.8103.78 | 위 API 이름은 노출 | 완료 응답 확인 못함 | 제한 시간 내 완료하지 않음 |

`downloadable`은 설치 후 사용 가능성을 의미하며 인식·번역 성공이나 처리 속도를 측정한 결과가 아니다. Headless Shell의 API 이름 노출만으로 구현된 백엔드까지 사용 가능하다고 판단할 수 없다. 셸 실패의 구체적인 원인이나 모든 버전에서의 미지원 여부까지 확정하지 않았다.

## 기능 구분

- Chrome의 Live Caption/Live Translate는 사용자가 보는 브라우저 자막 기능이다. 자막 버블의 결과를 앱에 직접 반환하는 안정적인 공개 JS API는 이번 조사에서 확인하지 못했다. 공식 도움말상 Live Translate는 자막을 Google에 보내 번역하는 기능이다. 자동 자막 UI를 OCR/접근성으로 추출하는 경로는 취약하므로 권하지 않는다.
- Web Speech API는 결과 이벤트와 임시/최종 전사를 받을 수 있는 앱 인터페이스다. `processLocally=true`와 언어팩 상태를 확인해야 로컬 처리를 요구할 수 있다. 기본값에서는 원격 처리 가능성이 있다. 최신 명세에는 `start(audioTrack)`이 있어 안경 PCM을 브라우저 오디오 트랙으로 공급할 설계가 가능하다. 실제 설치 버전에서 트랙 입력·장시간 지속·인식 품질은 추가 검증해야 한다.
- Translator API는 음성이 아니라 텍스트를 번역하는 로컬 전문 모델 인터페이스다. Chrome 공식 문서에 한국어가 포함되고 초기 모델/언어팩 다운로드와 생성 시 사용자 활성화 조건을 고려해야 한다. 문서상 데스크톱 대상이며 iPhone Even WebView에서 직접 쓰는 구조로 가정하지 않는다.
- 일반 Chrome의 `--headless`는 현재 일반 Chrome과 구현을 공유한다. `chrome-headless-shell`은 별도 구형 headless 구현 계열이므로 서로 동일한 기능 지원을 가정하지 않는다.

## 권장 연결안

1. Mac 게이트웨이에 선택형 Chrome worker를 추가한다. 검색용 Headless Shell과 별도로, 정식 Chrome을 전용 영속 프로필로 실행한다. 최초 언어팩 준비와 사용자 활성화는 작은 설정 창에서 처리하고 이후 headless 운용 가능성을 검증한다.
2. 안경 PCM → Mac → localhost/WebSocket → AudioWorklet/MediaStreamDestination → `SpeechRecognition.start(audioTrack)` → 임시/최종 전사 순으로 연결한다. 스피커 재생이나 가상 마이크를 기본 설계로 요구하지 않는다. 초기에는 고정 입력 언어를 사용한다. 텍스트 LanguageDetector를 음성 언어 판별기로 오해하지 않는다.
3. 전사 → `Translator.translate()` 또는 `translateStreaming()` → 안경의 번역 박스로 연결한다. 임시 전사는 짧은 주기로 병합하고 revision을 확인하여 늦은 번역이 최신 자막을 덮지 않게 한다. 확정 전사는 SQLite에 저장한다.
4. 긍정·부정 답변 추천은 번역 모델이 생성하는 기능이 아니다. 번역문 표시와 분리해 확정 전사에 한해서 기존 LLM을 호출한다. JEV 힌트 선별 구조는 유지한다.
5. Chrome worker 종료·언어팩 미지원·서비스 오류 때 기존 MLX Whisper와 번역 경로로 복귀한다. 먼저 Whisper STT + Chrome Translator 조합으로 번역 LLM 부하를 분리하고, 그 다음 Chrome STT와 실제 안경 오디오 입력을 비교하는 순서를 권한다.

확인할 지표: 동일 한국어/영어 PCM에서 첫 임시 자막 지연, 최종 전사 지연·오류, 번역 지연·품질, 전체 RAM/CPU 사용량, Chrome 재시작 후 언어팩 유지, 30분 이상 연속 인식·복구. 브라우저 상주 비용을 포함해 측정하기 전에는 Whisper보다 가볍거나 빠르다고 보장하지 않는다.

## 근거

- [Chrome Headless와 Headless Shell의 차이](https://developer.chrome.com/docs/automation-and-testing/headless)
- [Chrome Translator API 및 지원 언어](https://developer.chrome.com/docs/ai/translator-api)
- [Chrome Live Caption/Translate 도움말](https://support.google.com/chrome/answer/10538231?hl=en)
- [Web Speech API 명세 — 오디오 트랙 입력 및 로컬 처리](https://webaudio.github.io/web-speech-api/)
- [온디바이스 음성 인식 제안 및 언어 지원 예시](https://github.com/WebAudio/web-speech-api/blob/main/explainers/on-device-speech-recognition.md)


## 1.5.0 구현 결과

Whisper 전사를 유지한 Chrome Translator 경로를 구현했다. `POST /api/translate/chrome`과 `/api/translate/chrome/prepare`는 기존 계정 인증을 요구한다. 정식 Chrome을 전용 영속 프로필로 실행하고 prepare에서 언어팩을 생성하며, 준비 이력을 저장해 브라우저 재시작 후 복원한다. 실제 한국어↔영어 및 재시작 복원 테스트를 통과했다. 준비 시 availability를 먼저 await하면 사용자 활성화가 소실될 수 있어 create를 직접 실행한다. 종료 시 Chrome을 정상 종료해 모델 상태를 보존하고, 시작 직후의 일시적 NotSupportedError만 제한적으로 재시도한다.

안경 앱은 전사 텍스트의 Chrome 번역을 답변 LLM과 별도 대기열로 처리하고 즉시 표시한다. LLM은 답변 예시만 생성한다. 늦은 이전 결과는 revision으로 차단하며 Chrome 실패는 기존 번역으로 복귀한다. 브라우저 STT와 임시 전사의 Chrome 번역은 이번 단계에 포함하지 않았다. 일반 Chrome 프로필이나 사용자 음성은 통합 테스트에 사용하지 않았다.


## 1.6.0 STT 구현 결과

정식 Chrome의 온라인 Web Speech 경로를 구현했다. MediaStreamTrackGenerator와 AudioData로 안경 PCM을 전달하므로 Mac 마이크·가상 마이크·스피커 재생은 필요하지 않다. 시작 시 짧은 무음 프레임으로 세션을 먼저 열어 첫 발화 누락을 방지한다. 발화의 증가하는 스냅샷에서는 새 바이트만 공급하고 Chrome 내부 큐로 중복 전송·늦은 프리뷰를 방어한다. 약 250ms 프리뷰, 수동/연속 녹음, 기존 전사 저장/호출어/AI 연결과 선택형 Whisper 복귀를 추가했다.

실제 합성 한국어 온라인 테스트에서 중간·최종 전사가 동작했다. 동일 입력으로 로컬 processLocally 경로는 시작 전에 aborted로 종료되어 배포 UI에서 제외했다. 이 결과를 모든 Chrome 로컬 인식 미지원으로 일반화하지 않는다. 온라인 선택 시 음성이 Google에 전송되며 UI에 이를 명시한다. 클라우드 모드를 로컬 실패의 자동 대체재로 사용하지 않는다.


## 1.7.0 중간 번역 연결

Chrome API의 중간 인식은 이미 수신했지만 번역 경로가 final 전사에만 연결돼 있었다. 이를 프리뷰에도 연결하고 Chrome의 stableText/draftText를 전달했다. 별도 최신 대기열이 임시 결과를 표시하며 최종 결과가 같은 발화의 임시 번역을 교체한다. 중간 결과는 LLM·저장·호출어 평가에 전달하지 않는다. 실제 온라인 STT→로컬 Translator 연속 테스트와 앱의 최종 STT 이전 번역 표시 테스트를 통과했다. 브라우저 UI인 Live Caption을 직접 추출하는 기능은 아니다.
