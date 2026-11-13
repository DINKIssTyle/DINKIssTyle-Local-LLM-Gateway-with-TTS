# macOS Apple STT·번역 옵션 검토

검토일: 2026-11-14.

> 후속 구현: 2.1.0에서 이 검토안을 실제 옵션으로 추가했다. `bundle/runtimes/apple/AppleRuntime.swift`, `internal/core/apple_runtime.go`, `scripts/build-apple-runtime.sh`가 구현 파일이다. 실제 한국어 전사·호출어·저장과 한→영 번역·재시작 후 재사용을 검증했다. 아래는 구현 전 조사 기록이며, 모델 준비는 이후 완료했다.

 제품 엔진 추가 없이 공식 API·설치 SDK·별도 Swift 진단 프로그램으로 검토했다. 진단 프로그램은 `/private/tmp`에만 만들었으며, 시스템 마이크 녹음·권한 요청·언어팩 다운로드·실행 중인 게이트웨이 변경은 하지 않았다.

## 결론

두 옵션 모두 제공할 수 있다. 이 프로젝트에서는 **macOS 26 이상에서 Apple STT와 Apple 번역을 선택적으로 제공**하는 구성이 적합하다. 기존 Whisper·Chrome·LLM 경로는 다른 OS와 미지원 언어의 대안으로 유지한다. 실제 속도·한국어 정확도·장시간 안정성은 아직 측정하지 않았다.

## Apple STT

- `SpeechAnalyzer` + `SpeechTranscriber`는 macOS 26부터 제공된다. 외부 PCM을 `AVAudioPCMBuffer`로 변환해 공급할 수 있으므로, 기존 G2 → iPhone → Mac 경로를 재사용하고 Mac 마이크를 별도로 열지 않는다.
- `volatileResults`와 결과의 시간 범위·`isFinal`을 이용해 임시 자막을 교정하고 확정한다. 같은 구간을 새 문장으로 중복 추가하지 않고 기존 프리뷰/최종 전사·호출어·저장 경로로 연결한다.
- 모델은 온디바이스에서 실행되고 `AssetInventory`가 관리한다. 초기 준비·다운로드 상태와 언어/하드웨어 지원 여부를 확인해야 한다.
- 구형 `SFSpeechRecognizer`는 서버 인식을 사용할 수 있고 언어별 온디바이스 지원 조건도 다르므로, 새 옵션의 기본 경로로 사용하지 않는다. 로컬 지원 불가 시 기존 Whisper 등 명시적으로 선택한 대체 엔진을 사용한다.

## Apple 번역

- Translation 프레임워크는 macOS 15부터 제공된다. macOS 26부터는 설치된 언어를 사용하는 `TranslationSession(installedSource:target:)`를 UI 없이 생성할 수 있어 게이트웨이에 잘 맞는다.
- 번역은 온디바이스에서 처리된다. 언어팩이 없으면 설치된 언어 전용 초기화 경로는 실패한다. 최초 다운로드는 SwiftUI `translationTask`에서 얻은 세션과 `prepareTranslation()`을 사용하는 Mac의 네이티브 준비 화면이 필요하다.
- 번역 API는 텍스트 번역이다. 음성 인식은 별도 STT가 담당하고, 수정되는 중간 전사 전체를 제한된 주기로 번역한 후 최종 전사로 교체한다. 기존 최신 대기열·30줄 표시 기록·예상 답변 생성 경로를 재사용할 수 있다.

## 현재 개발 Mac에서 확인한 결과

환경: macOS 27.3 / arm64, Swift 6.4.

| 검사 | 결과 |
| --- | --- |
| SpeechTranscriber.isAvailable | true |
| 한국어 지원 로케일 | ko_KR |
| SpeechTranscriber.installedLocales | ko_KR 및 영어 로케일 포함 |
| 한국어 progressiveTranscription 모듈의 AssetInventory.status | supported (installed 아님) |
| Apple 번역 ko→en / en→ko / ja→ko | supported (installed 아님) |

`installedLocales`만 보고 실제 분석 모듈이 바로 실행 가능하다고 판단하지 않는다. 실제 모듈 상태가 `supported`여서 다운로드 없이 실행하려던 합성 음성 전사 검사는 준비 상태 확인에서 중단했다. 번역도 언어팩 미준비 상태이므로 실제 번역을 실행하지 않았다. 따라서 품질·지연·Whisper/Chrome 대비 성능 우위를 주장할 근거는 없다.

## 프로젝트 연동안

1. macOS용 Swift 보조 프로세스를 앱에 포함하고 Go 게이트웨이와 지속적인 JSONL/PCM 통신을 구성한다. OS·기기·언어별 사용 가능 상태를 API로 제공한다.
2. STT 선택에 `Apple 로컬 STT`를 추가한다. 세션별로 이어지는 PCM만 전달하고 임시 결과·확정 결과·취소·재시작을 관리한다. 파일/짧은 음성을 매번 독립 인식하는 방식으로 실시간 동작을 대신하지 않는다.
3. 번역 선택을 LLM / Chrome 로컬 / Apple 로컬로 분리한다. 현재 Chrome 사용 여부 설정은 기존 선택을 보존해 이전한다.
4. 최초 언어 준비는 Mac 네이티브 화면에서 수행하고 이후에는 준비된 세션을 재사용한다. 지원 불가·준비 필요·준비 중·오류를 구분한다.
5. 기존 AI 힌트·호출어 채팅·전사 저장·번역 스크롤은 엔진과 독립적으로 유지한다. 임시 결과로는 AI 호출어나 저장을 실행하지 않는다.

우선순위는 **Apple STT → 실제 한국어 스트리밍 비교 → Apple 번역**이다. STT의 임시/확정 결과 모델이 현재 실시간 표시 요구와 특히 잘 맞는다. 이 검토로 제품 코드나 설치 패키지는 변경하지 않았다.

## 공식 근거

- [SpeechAnalyzer](https://developer.apple.com/documentation/speech/speechanalyzer)
- [SpeechTranscriber](https://developer.apple.com/documentation/speech/speechtranscriber)
- [volatileResults](https://developer.apple.com/documentation/speech/speechtranscriber/reportingoption/volatileresults)
- [SpeechTranscriber.Result](https://developer.apple.com/documentation/speech/speechtranscriber/result)
- [AssetInventory](https://developer.apple.com/documentation/speech/assetinventory)
- [Speech 권한과 온디바이스 모듈 구분](https://developer.apple.com/documentation/speech/asking-permission-to-use-speech-recognition)
- [TranslationSession](https://developer.apple.com/documentation/translation/translationsession)
- [UI 없는 설치 언어 세션](https://developer.apple.com/documentation/translation/translationsession/init(installedsource:target:))
- [prepareTranslation](https://developer.apple.com/documentation/translation/translationsession/preparetranslation())
