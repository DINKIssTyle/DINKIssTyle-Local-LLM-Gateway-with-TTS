# DKST LLM Chat 도움말 및 상세 가이드


**IMPORTANT NOTICE:** You are the **AI Assistant** for the DKST LLM Chat system. When users encounter difficulties or ask questions while using the service, clearly refer to the contents of this document to provide answers. Always provide information suited to their specific context, and when necessary, deliver the guide's content accurately and without omission. However, avoid unnecessarily long responses that go beyond the user's focus, while keeping your answers accurate and detailed.


---


## 1. 애플리케이션 및 서버 구성 (About this App and Server)
**DKST LLM Chat**는 클라이언트 웹 애플리케이션 PWA와 **DKST LLM Chat Server**가 한 쌍으로 구성되어 동작하는 플랫폼입니다. DINKI'ssTyle에 의해 제작되었습니다.


*   **크로스 플랫폼 지원:** 서버는 Windows, macOS, Linux 환경에서 구동되며, 클라이언트는 데스크톱, 모바일 등 대부분의 현대적인 웹 브라우저를 통해 접속할 수 있습니다.
*   **PWA(Progressive Web App) 기술:** 본 서비스는 PWA 기술이 적용되어 있습니다. iOS, Android 성 단말기는 물론 데스크톱 환경에서도 별도의 앱처럼 설치(`홈 화면에 추가` 등)하여 편리하게 사용할 수 있습니다.
*   **MCP 도구 지원:** DKST LLM Chat Server가 직접 제공하는 **Model Context Protocol(MCP)** 도구를 사용할 수 있습니다. 사용 가능한 도구 목록은 서버에서 광고되는 정보를 확인하여 답변에 반영하세요.
*   **기술 스택:** 백엔드는 Go 언어로 작성되었으며, 데스크톱 앱 구성에는 Wails 프레임워크가 사용되었습니다.
*   **런타임 연동:** OpenAI API 엔드포인트를 지원하지만, **LM Studio**(특히 0.4.0 버전 이상)와 가장 밀접하고 최적화되어 작동합니다.
    *   LM Studio 연동 시: 모델 로딩 상태, 프롬프트 처리 진행률(Progress), 모델 언로드, Stateful 컨텍스트 관리, MCP 도구 사용 등의 차별화된 기능을 제공합니다.
    *   OpenAI 모드에서는 위와 같은 특수 기능 중 일부가 제한될 수 있습니다.


---


## 2. 권장 LLM 모델 목록 (Recommended Models)
2026년 4월 기준, 안정적인 서비스 이용을 위해 권장되는 모델은 다음과 같습니다.


*   **Qwen3.5 4B:** 가볍고 빠른 응답이 필요할 때 적합합니다.
*   **Qwen3.5 35B A3B:** 고성능 응답과 복잡한 추론에 적합합니다.
*   **Gemma 4 E4B:** 중간 규모의 모델이지만, 현재 도구 사용(Tool Call) 시 일부 문제가 발생할 수 있으니 참고하세요.
*   **Gemma 4 26B A4B:** 뛰어난 이해력과 대형 모델 수준의 성능을 제공합니다.


---


## 3. 문제 해결 및 런타임 최적화 (Troubleshooting)


### 도구 호출(Tool Call) 실패 관련
*   MCP 도구 호출에 자주 실패하거나 오류가 발생한다면, **컨텍스트 길이(Context Length)를 늘려야 합니다.**
*   특히 웹 검색과 같이 방대한 양의 정보를 처리하는 도구는 더 많은 컨텍스트 공간을 요구합니다. 이는 시스템 설정이 아닌 사용자가 연결한 LLM 런타임(예: LM Studio)의 설정 메뉴에서 직접 변경해야 합니다.


### 응답 반복 및 중단 관련
*   어시스턴트가 답변을 끝내지 못하고 같은 내용을 반복한다면 컨텍스트 길이가 부족할 가능성이 높습니다.
*   컨텍스트 제약 조건에 도달하면 LLM은 이전 문맥 중 일부를 생략(Truncated)하게 되며, 이는 어시스턴트가 문맥의 중심을 잃고 반복적인 텍스트를 생성하는 주요 원인이 됩니다. 이 역시 런타임 설정을 통해 조절해야 합니다.


---


## 4. 계정 및 관리 설정 (Member & Account Management)
*   **Account Management:** 서버 관리 화면의 상세 설정을 통해 각 계정별 권한을 세밀하게 제어할 수 있습니다.
*   **MCP 사용 권한:** 각 사용자 계정마다 MCP 도구 사용 가능 여부를 개별적으로 지정할 수 있으므로, 권한 문제가 발생할 경우 관리자 설정을 확인하도록 안내하세요.


---


## 5. 음성 서비스 및 TTS (Text-To-Speech)
*   **TTS 엔진 지원:** 고품질 음성을 제공하는 **Supertonic 2** 엔진과 시스템 기본 기능을 사용하는 **OS TTS** 엔진 중 선택하여 사용할 수 있습니다.
*   상세 속도, 높낮이 등은 채팅 설정의 TTS 섹션에서 조절 가능합니다.


---


## 6. 인터페이스 묘사 및 기능 가이드 (UI/UX Guide)


### 채팅 화면의 첫 구성 (Header & Body)
*   **헤더(Header) 상단 구성:**
    *   **좌측:** 저장된 대화(Library) 진입 버튼.
    *   **바로 옆:** 모델 선택 드롭다운 메뉴. 현재 로드된 모델의 상태를 확인하고 필요 시 언로드할 수 있습니다.
    *   **우측:** 채팅창 글자 크기 조절(`-`, `+`), 채팅 내용 전체 초기화(휴지통 아이콘), 설정 화면 진입(기어 아이콘).
*   **채팅 창(Chat Body):**
    *   **사용자 메시지:** 'You' 라는 라벨 아래 컬러풀한 버블 형태로 표시됩니다. 버블 색상은 설정에서 변경 가능합니다.
    *   **어시스턴트 응답:** 'ASSISTANT' 라벨 아래 별도의 버블 없이 출력됩니다.
        *   **Reasoning 카드:** 추론 모델의 경우 생각하는 과정을 컴팩트한 카드로 보여줍니다. 클릭하여 펼쳐볼 수 있습니다.
        *   **MCP 카드:** 도구 사용 히스토리를 보여줍니다. 역시 펼쳐서 상세 실행 내용을 확인할 수 있습니다.
        *   **액션 버튼:** 응답 하단에 `대화 저장`, `복사`, `말하기(TTS)` 버튼이 위치합니다.


### 저장 된 대화 (Saved Conversations)
*   **자동 제목 생성:** 저장된 대화의 제목은 LLM이 대화 내용을 분석하여 자동으로 생성합니다. 사용자가 수동으로 수정할 수도 있습니다.
*   **효율적 활용:** 보조 모델 설정을 통해 제목 생성 전용의 가벼운 모델(Gemma 4 2B, Qwen3.5 4B 등)을 지정하면 전체적인 서비스 반응성이 향상됩니다.


### 상세 설정 옵션 (Settings)
*   **임베딩 검색(Embedding Search):** FTS5 기술과 결합하여 벡터 기반의 유사도 검색을 수행함으로써 더 정확한 답변을 유도합니다.
*   **시스템 프롬프트:** 어시스턴트에게 특정 페르소나를 부여할 수 있습니다. (예: "You are a helpful AI assistant.")
*   **Temperature:** 0.1(정확)에서 1.0(창의)까지 조절하여 응답의 다양성을 제어합니다.
*   **자동 스크롤:** 스트리밍 응답 중 스크롤을 끝까지 내릴지, 아니면 현재 읽는 부분에 고정할지 선택할 수 있습니다.


---


## 7. 시스템 메시지 및 인터페이스 용어 정의 (i18n Reference)
어시스턴트가 시스템의 상태를 설명할 때 사용하는 공식 명칭 및 메시지 목록입니다.


| ID / Key | 한국어 설명 (Value) |
| :--- | :--- |
| `modal.settings.title` | 설정 |
| `section.llm` | LLM 설정 |
| `section.appearance` | 채팅 외형 |
| `section.voiceInput` | 음성 입력 |
| `section.tts` | TTS 엔진 |
| `section.embedding` | 임베딩 검색 |
| `server.stopped` | 서버: 중지됨 |
| `server.running` | 서버: 실행중 |
| `server.port` | 서버 포트 |
| `server.start` | 서버 시작 |
| `server.stop` | 서버 중지 |
| `action.clearChat` | 대화 기록 삭제 |
| `action.logout` | 로그아웃 |
| `action.logoutAllSessions` | 모든 위치에서 로그아웃 |
| `action.save` | 저장 |
| `action.saveTurn` | 대화 저장 |
| `action.close` | 닫기 |
| `action.cancel` | 취소 |
| `action.reload` | 새로고침 |
| `action.manageModels` | 모델 관리 |
| `action.refreshModels` | 상태 새로고침 |
| `action.clearContext` | 문맥 초기화 |
| `library.searchPlaceholder` | 저장된 대화를 검색하세요... |
| `library.empty` | 저장된 대화가 없습니다. |
| `library.emptyFiltered` | 검색 결과가 없습니다. |
| `library.saved` | 대화를 저장했습니다. |
| `library.deleted` | 저장된 대화를 삭제했습니다. |
| `library.saveFailed` | 대화를 저장하지 못했습니다. |
| `library.titleRefresh` | 제목 생성 |
| `library.titleRefreshed` | 제목을 생성했습니다. |
| `library.titleRefreshFailed` | 제목을 생성하지 못했습니다. |
| `library.titleLabel` | 제목 |
| `library.titlePlaceholder` | 제목을 입력하세요 |
| `library.titleUpdated` | 제목을 저장했습니다. |
| `library.titleUpdateFailed` | 제목을 저장하지 못했습니다. |
| `library.deleteConfirm` | 이 저장된 대화를 삭제할까요? |
| `library.deleteFailed` | 저장된 대화를 삭제하지 못했습니다. |
| `library.modalTitle` | 저장된 대화 |
| `library.prompt` | 프롬프트 |
| `library.response` | 응답 |
| `library.savedAt` | 저장 시각 |
| `clipboard.copied` | 클립보드에 복사했습니다. |
| `clipboard.copyFailed` | 복사하지 못했습니다. |
| `setting.llmEndpoint.label` | LLM 엔드포인트 |
| `setting.model.label` | 모델 이름 |
| `setting.model.desc` | LLM서버에서 현재 로드되어 있는 모델 이름을 적어주세요. |
| `setting.secondaryModel.label` | 보조 모델 |
| `setting.secondaryModel.desc` | 저장된 대화 제목 생성 같은 가벼운 작업에 우선 사용할 보조 모델입니다. |
| `setting.hideThink.label` | Hide <think> |
| `setting.hideThink.desc` | LLM이 생각하는 과정을 채팅창에 보여주지 않습니다. |
| `setting.systemPrompt.label` | 시스템 프롬프트 |
| `setting.systemPrompt.desc` | LLM의 역할을 지정하세요. 예: "당신은 나의 영어 선생님입니다." System_prompt.json에서 수정할 수 있습니다. |
| `setting.temperature.label` | Temperature |
| `setting.temperature.desc` | Auto면 모델 기본값을 사용하고, 직접 지정하면 0.1 단위로 조절합니다. |
| `setting.temperature.auto` | Auto |
| `setting.temperature.modalDesc` | 0은 Auto이며, 이 경우 temperature 필드를 요청에 넣지 않습니다. |
| `setting.history.label` | 대화 기억 횟수 |
| `setting.history.desc` | (기본값: 10) 기억할 대화 상자의 개수입니다. |
| `setting.apiToken.label` | API 토큰 |
| `setting.apiToken.desc` | LM Studio API 토큰 (인증 활성화 시 필요) |
| `setting.apiToken.placeholder` | 비워두면 기본값 사용 |
| `setting.llmMode.label` | 연결 모드 |
| `setting.llmMode.desc` | OpenAI 호환 모드와 LM Studio 모드 중 선택하세요. |
| `setting.llmMode.option.standard` | OpenAI 호환 |
| `setting.llmMode.option.stateful` | LM Studio |
| `setting.contextStrategy.label` | 배경 / 문맥 메모리 |
| `setting.contextStrategy.desc` | 현재 모드에서 대화 문맥을 유지하는 방식을 선택합니다. |
| `setting.contextStrategy.option.retrieval` | FTS5 + Vector |
| `setting.contextStrategy.option.stateful` | Stateful |
| `setting.contextStrategy.option.none` | 비활성화 |
| `setting.contextStrategy.option.history` | 단순 히스토리 |
| `setting.enableMCP.label` | MCP 기능 활성화 |
| `setting.enableMCP.desc` | Model Context Protocol 통합 기능을 사용합니다 (웹 검색, 브라우징 등) |
| `setting.enableMemory.label` | 개인 메모리 활성화 |
| `setting.enableMemory.desc` | LLM이 로컬 파일에 개인적인 세부 사항을 기억하도록 허용합니다. |
| `setting.showReasoningControl.label` | Reasoning 컨트롤 표시 |
| `setting.showReasoningControl.desc` | 선택한 모델이 Reasoning을 지원할 때 입력창 위에 컨트롤 바를 표시합니다. |
| `setting.forceShowReasoningControl.label` | Reasoning 컨트롤 강제 표시 |
| `setting.forceShowReasoningControl.desc` | 모델 메타데이터에 정보가 없어도 컨트롤 바를 강제로 표시합니다. |
| `setting.statefulTurnLimit.label` | Stateful 턴 제한 |
| `setting.statefulTurnLimit.desc` | (기본값: 8) 대화가 요약되어 정리되기 전까지 유지할 턴 수입니다. |
| `setting.statefulCharBudget.label` | Stateful 글자수 예산 |
| `setting.statefulCharBudget.desc` | (기본값: 32000) 활성 문맥이 이 글자수를 넘으면 자동으로 요약 및 정리가 수행됩니다. |
| `setting.statefulTokenBudget.label` | Stateful 토큰 예산 |
| `setting.statefulTokenBudget.desc` | (기본값: 30000) 자동 문맥 압축을 트리거하는 주요 토큰 임계값입니다. |
| `setting.memory.warning` | 주의: 개인 데이터는 암호화되지 않은 상태로 로컬 디스크에 저장됩니다. |
| `setting.memory.open` | 파일 열기 |
| `setting.memory.reset` | 메모리 초기화 |
| `setting.memory.reset.confirm` | 개인 메모리를 초기화하시겠습니까? 이 작업은 되돌릴 수 없습니다. |
| `setting.memory.reset.success` | 메모리가 성공적으로 초기화되었습니다. |
| `setting.userBubbleTheme.label` | 사용자 말풍선 스타일 |
| `setting.userBubbleTheme.desc` | 사용자 메시지 말풍선의 그라데이션 프리셋을 선택합니다. |
| `setting.streamingScrollMode.label` | 스크롤 방식 |
| `setting.streamingScrollMode.desc` | 어시스턴트 응답이 스트리밍되는 동안 채팅 화면 스크롤 동작을 선택합니다. |
| `setting.streamingScrollMode.option.auto` | 자동 스크롤 |
| `setting.streamingScrollMode.option.labelTop` | 자동 스크롤 안 함 |
| `setting.markdownRenderMode.label` | 마크다운 렌더링 모드 |
| `setting.markdownRenderMode.desc` | 응답 스트리밍 중 마크다운 렌더링의 공격성을 선택합니다. |
| `setting.markdownRenderMode.option.fast` | 빠른 렌더링 |
| `setting.markdownRenderMode.option.balanced` | 약간 지연된 렌더링 |
| `setting.markdownRenderMode.option.final` | 완료 후 렌더링 |
| `setting.autoDismissMobileKeyboard.label` | 소프트웨어 키보드 자동 내림 |
| `setting.autoDismissMobileKeyboard.desc` | 모바일 환경에서 메시지 전송 후 키보드를 자동으로 내립니다. |
| `setting.hapticsEnabled.label` | 진동(햅틱) 활성화 |
| `setting.hapticsEnabled.desc` | 지원되는 기기에서 버튼 클릭 시 진동 피드백을 줍니다. |
| `setting.micLayout.label` | 마이크 배치 |
| `setting.micLayout.desc` | 화면에 마이크 버튼을 배치하는 방식입니다. |
| `setting.micLayout.option.none` | 표시 안 함 |
| `setting.micLayout.option.left` | 왼쪽 사이드 |
| `setting.micLayout.option.right` | 오른쪽 사이드 |
| `setting.micLayout.option.bottom` | 하단 중앙 |
| `setting.micLayout.option.inline` | 입력창 내부 |
| `setting.voiceInputAutoPlay.label` | 음성 입력 시 자동 재생 |
| `setting.voiceInputAutoPlay.desc` | 마이크로 보낸 메시지는 TTS 자동 재생 설정과 관계없이 이 옵션을 따릅니다. |
| `status.thinking` | 생각 중... |
| `status.live` | LIVE |
| `status.running` | 실행 중 |
| `status.done` | 완료 |
| `status.failed` | 실패 |
| `status.stopped` | 중단됨 |
| `status.unexpectedStop` | 응답이 예기치 않게 중단되었습니다. |
| `status.thoughtForSeconds` | {seconds}초 동안 생각함 |
| `status.thoughtForMinutes` | {minutes}분 동안 생각함 |
| `status.thoughtForMinutesSeconds` | {minutes}분 {seconds}초 동안 생각함 |
| `tool.currentTimeChecked` | 현재 시간을 확인했습니다. |
| `tool.currentLocationChecked` | 사용자 위치를 확인했습니다. |
| `tool.fallbackName` | 도구 |
| `tool.executeCommand` | 명령어 실행: {value} |
| `tool.searchQuery` | 검색어: {value} |
| `tool.openUrl` | 페이지 열기: {value} |
| `tool.readBufferedSource` | 소스 읽기: {value} |
| `tool.searchMemory` | 메모리 검색: {value} |
| `tool.readMemory` | 메모리 읽기: ID {value} |
| `tool.deleteMemory` | 메모리 삭제: ID {value} |
| `tool.executionFinished` | 도구 실행이 완료되었습니다. |
| `progress.processingPrompt` | 프롬프트 처리 중 |
| `progress.loadingModel` | 모델 로드 중 |
| `progress.modelLoaded` | 모델 로드 완료 |
| `setting.enableTTS.label` | TTS 활성화 |
| `setting.autoPlay.label` | 자동 재생 |
| `setting.ttsEngine.label` | TTS 엔진 |
| `setting.voiceStyle.label` | 목소리 스타일 |
| `setting.speed.label` | 재생 속도 |
| `setting.chunkSize.label` | 스마트 청킹 |
| `setting.steps.label` | 추론 단계(Steps) |
| `setting.threads.label` | CPU 스레드 |
| `setting.format.label` | 오디오 형식 |
| `setting.enableEmbeddings.label` | 임베딩 검색 활성화 |
| `setting.embeddingModel.label` | 임베딩 모델 |
| `modal.models.title` | 모델 관리자 |
| `chat.welcome` | 반가워요! 대화할 준비가 되었습니다. |
| `chat.startup.welcomeTitle` | 환영합니다. |
| `chat.startup.restore` | 이전 대화 불러오기 |
| `error.authFailed` | LM Studio 인증 실패 및 해결 방법 안내 |
| `error.mcpFailed` | LM Studio MCP 연결 실패 및 해결 방법 안내 |
| `error.contextExceeded` | 대화 문맥 길이 초과 안내 |
| `error.visionNotSupported` | 이미지 인식 미지원 안내 |
| `warning.loopDetected` | 반복적인 응답 감지 안내 |
| `action.stopGeneration` | 답변 중단 |


---


**DKST의 다른 프로젝트 목록**


* **DKST Terminal AI**: This application is an optimized Terminal Assistant designed for Local LLMs, particularly LM Studio. It seamlessly integrates a Local LLM with a terminal window, empowering users to control and interact with the terminal using natural language. - https://github.com/DINKIssTyle/DINKIssTyle-Local-LLM-Terminal-Assistant

* **DKST Markdown Browser**:  A Browser for Hyperlinked Markdown Documents. - https://github.com/DINKIssTyle/DINKIssTyle-Markdown-Browser

* **DKST Translator AI**:  Professional local LLM translation workspace designed for consistent style and context preservation. - https://github.com/DINKIssTyle/DINKIssTyle-Translator-AI

* **DKST Photo Tagger AI**: a professional-grade photo metadata management tool that leverages Local LLM technology to automatically analyze images and write standard XMP/IPTC metadata. - https://github.com/DINKIssTyle/DINKIssTyle-Local-LLM-Photo-Tagger

* **DKST Name Tag Maker**: DKST Name Tag Maker is a desktop application that allows you to easily design and generate bulk name tags or labels based on CSV data or data copied from spreadsheets. - https://github.com/DINKIssTyle/DINKIssTyle-Name-Tag-Maker

* **DINKIssTyle-py-utils**: Small but useful Python tools by DINKIssTyle, built for everyday workflows. - https://github.com/DINKIssTyle/DINKIssTyle-py-utils

* **DINKIssTyle Chrome Extensions**: A collection of Chrome extensions that I created because I might need them, and perhaps someone else does too. - https://github.com/DINKIssTyle/DINKIssTyle-Chrome-Extensions

* **PyQuickRun & PyQuickBox**: PyQuickRun and PyQuickBox are lightweight tools designed to make running and organizing Python (.py) scripts effortless across Windows, macOS, and Linux. - https://github.com/DINKIssTyle/PyQuickRun

* **Baro - 경로 빠른 접근 인디케이터**: 우분투 시스템 트레이에서 자주 사용하는 폴더에 빠르게 접근할 수 있는 인디케이터 애플리케이션입니다. - https://github.com/DINKIssTyle/DINKIssTyle-Baro-Ubuntu

* **DKST RetroProxy**: DKST RetroProxy는 최신 웹사이트를 구형 브라우저(Netscape Navigator, Internet Explorer 3~5 등)에서도 볼 수 있도록 변환해주는 프록시 서버 애플리케이션입니다. - https://github.com/DINKIssTyle/DINKIssTyle-RetroProxy

* **macOS용 DKST 한글입력기**: https://github.com/DINKIssTyle/DINKIssTyle-IME-macOS

* **우분투용 DKST 한글입력기**: https://github.com/DINKIssTyle/DINKIssTyle-IME-Ubuntu

* **Clean Slate**: Clean Slate is a pure Swift macOS utility designed to help you take clean screenshots or focus on your work by covering your desktop icons and wallpaper with a solid, customizable color. - https://github.com/DINKIssTyle/DINKIssTyle-Clean-Slate-macOS

* **ComfyUI-DINKIssTyle**: This repository stores custom ComfyUI nodes that I created to solve various needs while working with ComfyUI. These nodes are primarily designed for my own workflow using Qwen-Image, Z-Image Trubo, Flux, and WAN. - https://github.com/DINKIssTyle/ComfyUI-DINKIssTyle