<!--lang:ko-->
# 추가 자산

추가 자산은 기본 채팅 서버에 로컬 음성 합성, 의미 기반 검색, 앱 웹 도구, 사용자 스킬을 더하기 위한 구성 요소입니다. 필요한 기능만 설치해 사용할 수 있으며 다운로드한 파일은 이 기기에 저장됩니다.

## 자산 관리자

**자산 관리자**에서는 다음 작업을 할 수 있습니다.

- TTS 및 벡터 DB 관련 로컬 기능을 한 번에 켜거나 끕니다.
- 로컬 모델과 전용 Chromium의 설치 상태를 확인합니다.
- 누락된 모델을 다운로드하거나 기존 모델을 다시 다운로드합니다.
- 다운로드 진행률과 현재 처리 중인 파일을 확인합니다.
- **새로고침**으로 디스크의 실제 설치 상태를 다시 검사합니다.

### TTS 및 벡터 DB 활성화

이 스위치는 로컬 TTS 런타임과 임베딩 런타임의 공통 활성화 설정입니다.

- 켜면 설치된 Supertonic TTS와 활성화된 임베딩 모델을 사용할 수 있도록 준비합니다.
- 끄면 실행 중인 로컬 런타임을 해제하고 관련 기능을 중지합니다.
- 끄더라도 이미 다운로드한 모델 파일이나 저장된 데이터는 삭제되지 않습니다.

설정 변경 후 서버가 실행 중이면 일부 기능은 다음 요청 또는 서버 재시작 이후 완전히 반영될 수 있습니다.

### 자산 저장 폴더

모달에 표시되는 **자산 저장 폴더**는 모델과 런타임 파일이 실제로 저장되는 위치입니다. 운영체제와 설치 방식에 따라 위치가 달라지므로 화면에 표시된 경로를 기준으로 확인하세요.

다운로드 중에는 해당 폴더의 파일을 이동하거나 수정하지 않는 것이 좋습니다.

## 모델 자산

### Supertonic 3

Supertonic 3는 응답을 기기에서 음성으로 합성할 때 사용하는 로컬 TTS 모델입니다. 외부 음성 합성 서비스로 텍스트를 보내지 않고 로컬에서 음성을 생성할 수 있습니다. 실제 사용 여부는 채팅의 TTS 설정과 선택한 음성 엔진에 따라 결정됩니다.

### 임베딩 모델과 벡터 DB

임베딩 모델은 문장의 의미를 숫자 벡터로 변환합니다. 이 벡터는 메모리 검색이나 의미가 비슷한 내용을 찾는 작업에 사용됩니다.

- 단순 키워드 일치보다 의미가 가까운 기록을 찾는 데 도움을 줍니다.
- 벡터 DB에는 검색을 위한 벡터와 관련 메타데이터가 저장됩니다.
- 임베딩 모델이 없거나 비활성화되어 있으면 관련 기능은 비벡터 방식으로 동작하거나 사용할 수 없습니다.

## 앱 도구 전용 Chromium

전용 Chromium은 웹 페이지를 읽거나 브라우저 자동화가 필요한 앱 도구에서 사용합니다. 시스템 Chrome 설치에 의존하지 않도록 앱이 호환되는 브라우저 버전을 별도로 관리합니다.

- 일반 채팅이나 단순 API 요청에는 필요하지 않습니다.
- 웹 탐색 도구를 사용할 때만 실행됩니다.
- 상태가 **설치 필요**라면 자산 관리자에서 준비할 수 있습니다.
- 업데이트가 필요한 경우 자산 관리자에서 새 버전으로 교체할 수 있습니다.

## 상태 표시

- **준비됨:** 필요한 파일이 있고 현재 설정에서 사용할 수 있습니다.
- **설치 필요:** 필수 파일이 없으므로 다운로드가 필요합니다.
- **다운로드 중:** 파일을 내려받거나 설치하는 중입니다.
- **실패:** 다운로드 또는 파일 검증에 문제가 발생했습니다. 네트워크와 저장 공간을 확인한 뒤 다시 시도하세요.

## 스킬 폴더

**스킬 폴더 열기**는 현재 운영체제의 사용자 스킬 폴더를 엽니다. 스킬은 모델에 특정 작업 절차, 참고 자료, 도구 사용 지침을 제공하기 위한 확장 단위입니다.

스킬마다 별도의 폴더를 만들고 그 안에 `SKILL.md`를 넣습니다.

```text
skills/user/my-skill/SKILL.md
```

필요하면 같은 스킬 폴더 안에 `references`, `scripts`, `assets` 폴더를 추가할 수 있습니다.

### 운영체제별 사용자 스킬 위치

- macOS: `~/Library/Application Support/DKST LLM Chat Server/skills/user`
- Windows: 앱 실행 파일 옆의 `skills\user`
- Linux: 앱 실행 파일 옆의 `skills/user`

앱에 포함되는 기본 스킬은 `skills/builtin`에서 읽기 전용으로 관리합니다. 사용자 스킬은 `skills/user`에 보관하며 기본 스킬을 직접 덮어쓰지 않습니다.

앱은 각 요청 전에 스킬 형식을 검증하고, 이름과 설명이 현재 요청에 관련된 스킬만 선택해 모델 지침에 추가합니다. 새로 만들거나 수정한 스킬은 다음 채팅 요청부터 반영됩니다. 스킬 지침 자체는 도구 권한이나 파일·네트워크 접근 권한을 부여하지 않습니다.
<!--/lang-->

<!--lang:en-->
# Additional Assets

Additional assets extend the base chat server with local speech synthesis, semantic retrieval, app web tools, and user skills. Install only the capabilities you need; downloaded files remain on this device.

## Asset Manager

The **Asset Manager** lets you:

- Enable or disable local TTS and vector-database features together.
- Check the installation state of local models and the dedicated Chromium runtime.
- Download missing models or download an installed model again.
- Follow download progress and the file currently being processed.
- Use **Refresh** to inspect the actual files on disk again.

### Enable TTS & Vector DB

This is the shared enable switch for the local TTS and embedding runtimes.

- When enabled, installed Supertonic assets and the configured embedding model can be loaded.
- When disabled, active local runtimes are released and the related features stop.
- Disabling the switch does not delete downloaded model files or stored data.

If the server is already running, some changes may take full effect on the next request or after a server restart.

### Asset Storage Folder

The **Asset Storage Folder** shown in the modal is the actual location used for model and runtime files. Its path depends on the operating system and installation type, so use the path displayed in the app.

Avoid moving or editing files in this directory while a download is active.

## Model Assets

### Supertonic 3

Supertonic 3 is the local model used to synthesize spoken audio from responses. It can generate audio on the device without sending the text to an external speech service. Whether it is used depends on the chat TTS settings and selected speech engine.

### Embedding Model and Vector DB

An embedding model converts text meaning into numeric vectors. Those vectors support memory retrieval and searches for semantically similar content.

- It can find related records beyond exact keyword matches.
- The vector database stores retrieval vectors and associated metadata.
- If the embedding model is missing or disabled, related features either use a non-vector fallback or remain unavailable.

## Dedicated Chromium for App Tools

The dedicated Chromium runtime is used by app tools that need to read web pages or automate a browser. The app manages a compatible browser version so these tools do not depend on a system Chrome installation.

- Normal chat and simple API requests do not need it.
- It runs only when a web-navigation tool requires it.
- If its status is **Missing**, prepare it from the Asset Manager.
- When an update is needed, the Asset Manager can replace it with a newer version.

## Status Labels

- **Ready:** Required files are present and usable with the current configuration.
- **Missing:** Required files are absent and need to be downloaded.
- **Downloading:** Files are being downloaded or installed.
- **Failed:** Downloading or validation failed. Check the network connection and free disk space, then retry.

## Skills Folder

**Open Skills Folder** opens the writable user-skills directory for the current operating system. A skill is an extension unit that can provide a model with task procedures, reference material, and tool-use instructions.

Create one directory per skill and place `SKILL.md` inside it.

```text
skills/user/my-skill/SKILL.md
```

A skill may also contain `references`, `scripts`, and `assets` directories.

### User-skill locations

- macOS: `~/Library/Application Support/DKST LLM Chat Server/skills/user`
- Windows: `skills\user` beside the application executable
- Linux: `skills/user` beside the application executable

Bundled skills are maintained as read-only content in `skills/builtin`. User skills are stored in `skills/user` and cannot silently replace bundled skills.

Before each request, the app validates skills and injects only those whose names and descriptions are relevant to the current request. New or edited skills take effect on the next chat request. Skill instructions do not grant tool, filesystem, or network permissions.
<!--/lang-->
