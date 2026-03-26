/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

// Configuration State
// [NOTICE] 웹페이지(web.html)의 초기 설정값은 아래 객체에서 정의됩니다. HTML 파일의 value 속성은 무시됩니다.
// 브라우저 캐시(LocalStorage)에 저장된 값이 있다면 그것이 가장 우선됩니다.
let config = {
    apiEndpoint: 'http://127.0.0.1:1234',
    model: 'qwen/qwen3-vl-30b',
    hideThink: true,       // Default: True
    temperature: 0.7,      // Default: 0.7
    maxTokens: 4096,       // Default: 4096
    historyCount: 10,
    enableTTS: true,       // Default: True
    enableTTS: true,       // Default: True
    enableMCP: true,       // Default: True
    enableMemory: false,   // Default: False
    ttsLang: 'ko',
    chunkSize: 200,        // Default: 200 (Smart Chunking)
    systemPrompt: 'You are a helpful AI assistant.',
    ttsVoice: 'F1',        // Default: F1
    ttsSpeed: 1.1,         // Default: 1.1
    autoTTS: true,         // Default: True (Auto-play)
    ttsFormat: 'wav',      // Default: wav
    ttsSteps: 5,           // Default: 5
    ttsThreads: 2,         // Default: 2
    language: 'ko', // UI language
    apiToken: '',
    llmMode: 'standard', // 'standard' or 'stateful'
    disableStateful: false, // LM Studio specific
    statefulTurnLimit: 8,
    statefulCharBudget: 12000,
    statefulTokenBudget: 10000,
    micLayout: 'none', // 'none', 'left', 'right', 'bottom', 'inline'
    chatFontSize: 16
};

// ============================================================================
// i18n Translation System
// ============================================================================

const translations = {
    ko: {
        // Modal
        'modal.settings.title': '설정',
        // Sections
        'section.llm': 'LLM 설정',
        'section.voiceInput': '음성 입력',
        'section.tts': 'TTS 엔진',
        // Server
        'server.stopped': '서버: 중지됨',
        'server.running': '서버: 실행중',
        'server.port': '서버 포트',
        'server.start': '서버 시작',
        'server.stop': '서버 중지',
        // Actions
        'action.clearChat': '대화 기록 삭제',
        'action.logout': '로그아웃',
        'action.save': '저장',
        'action.cancel': '취소',
        'action.reload': '새로고침',
        'action.clearContext': '문맥 초기화',
        // Settings - LLM
        'setting.llmEndpoint.label': 'LLM 엔드포인트',
        'setting.model.label': '모델 이름',
        'setting.model.desc': 'LLM서버에서 현재 로드되어 있는 모델 이름을 적어주세요.',
        'setting.hideThink.label': 'Hide <think>',
        'setting.hideThink.desc': 'LLM이 생각하는 과정을 채팅창에 보여주지 않습니다.',
        'setting.systemPrompt.label': '시스템 프롬프트',
        'setting.systemPrompt.desc': 'LLM의 역할을 지정하세요. 예: "당신은 나의 영어 선생님입니다." System_prompt.json에서 수정할 수 있습니다.',
        'setting.temperature.label': 'Temperature',
        'setting.temperature.desc': '(기본값: 0.7) 값이 낮을수록 평범한 대답, 높을수록 창의적인 대답',
        'setting.maxTokens.label': 'Max Tokens',
        'setting.maxTokens.desc': '(기본값: 4096) LLM이 생성할 최대 토큰 수',
        'setting.history.label': 'History Count',
        'setting.history.desc': '(기본값: 10) 대화 기억 횟수',
        'setting.apiToken.label': 'API Token',
        'setting.apiToken.desc': 'LM Studio API Token (인증 활성화 시 필요, 빈칸이면 무시)',
        'setting.apiToken.placeholder': '빈칸이면 기본값 사용',
        'setting.llmMode.label': '연결 모드',
        'setting.llmMode.desc': 'OpenAI 호환 모드 또는 LM Studio 모드를 선택하세요.',
        'setting.llmMode.option.standard': 'OpenAI 호환',
        'setting.llmMode.option.stateful': 'LM Studio',
        'setting.disableStateful.label': '서버 저장 비활성화 (Stateful)',
        'setting.disableStateful.desc': '대화 내용을 서버에 저장하지 않습니다 (LM Studio).',
        'setting.enableMCP.label': 'MCP 기능 활성화',
        'setting.enableMCP.desc': 'Model Context Protocol 기능(웹 검색, 브라우징)을 활성화합니다.',
        'setting.enableMemory.label': '개인 메모리 활성화',
        'setting.enableMemory.desc': 'LLM이 사용자 정보를 파일에 기록하고 기억할 수 있게 합니다.',
        'setting.statefulTurnLimit.label': 'Stateful Turn Limit',
        'setting.statefulTurnLimit.desc': '(기본값: 8) LM Studio 모드에서 몇 턴까지 유지한 뒤 대화 문맥을 요약하고 새 체인으로 이어갈지 지정합니다.',
        'setting.statefulCharBudget.label': 'Stateful Character Budget',
        'setting.statefulCharBudget.desc': '(기본값: 12000) 활성 문맥의 예상 총 글자 수가 이 값을 넘기면 자동 compact가 실행됩니다.',
        'setting.statefulTokenBudget.label': 'Stateful Token Budget',
        'setting.statefulTokenBudget.desc': '(기본값: 10000) 자동 compact 판단에서 가장 우선으로 사용하는 예상 토큰 예산입니다.',
        'setting.memory.warning': '주의: 개인 정보가 PC에 평문으로 저장됩니다.',
        'setting.memory.open': '파일 열기',
        'setting.memory.reset': '메모리 초기화',
        'setting.memory.reset.confirm': '개인 메모리를 초기화하시겠습니까? 이 작업은 되돌릴 수 없습니다.',
        'setting.memory.reset.success': '메모리가 초기화되었습니다.',
        'setting.micLayout.label': '마이크 레이아웃',
        'setting.micLayout.desc': '화면에 마이크를 배치합니다.',
        'setting.micLayout.option.none': '사용 안 함',
        'setting.micLayout.option.left': '왼쪽',
        'setting.micLayout.option.right': '오른쪽',
        'setting.micLayout.option.bottom': '하단',
        'setting.micLayout.option.inline': '메시지 창 내부',
        'status.thinking': '생각 중...',
        'status.live': '진행 중',
        'status.running': '실행 중',
        'status.done': '완료',
        'status.failed': '실패',
        'status.stopped': '중단됨',
        'status.unexpectedStop': '응답이 예상치 못하게 중단되었습니다.',
        'status.thoughtForSeconds': '{seconds}초 동안 생각함',
        'status.thoughtForMinutes': '{minutes}분 동안 생각함',
        'status.thoughtForMinutesSeconds': '{minutes}분 {seconds}초 동안 생각함',
        'tool.currentTimeChecked': '현재 시간을 확인했습니다.',
        'tool.executionFinished': '도구 실행이 완료되었습니다.',
        'tool.noQueryDetails': '세부 질의 정보 없음',
        'tool.unknownError': '알 수 없는 오류',
        'progress.processingPrompt': '프롬프트 처리 중',
        'progress.loadingModel': '모델 로딩 중',
        'progress.modelLoaded': '모델 로딩 완료',
        // Settings - TTS
        'setting.enableTTS.label': 'TTS 활성화',
        'setting.enableTTS.desc': '응답을 음성으로 재생합니다.',
        'setting.autoPlay.label': '자동 재생',
        'setting.autoPlay.desc': '응답을 자동으로 음성 재생합니다.',
        'setting.voiceStyle.label': '음성 스타일',
        'setting.voiceStyle.desc': 'TTS 음성 스타일을 선택합니다.',
        'setting.speed.label': '속도',
        'setting.speed.desc': '음성 재생 속도입니다.',
        'setting.ttsLang.label': 'TTS 언어',
        'setting.ttsLang.desc': '선호하는 언어를 선택하세요.',
        'setting.chunkSize.label': 'Smart Chunking',
        'setting.chunkSize.desc': '(추천값: 150~300) TTS가 몇 글자씩 잘라 생성할지 지정',
        'setting.steps.label': '추론 단계',
        'setting.steps.desc': '(추천값: 2~8, 기본값: 5) 높을수록 자연스러운 음성',
        'setting.threads.label': 'CPU 사용',
        'setting.threads.desc': '(기본값: 2) TTS 생성에 할당하는 CPU 스레드',
        'setting.format.label': '재생 형식',
        'setting.format.desc': 'MP3는 WAV를 변환하여 재생합니다.',
        // Advanced
        'section.advanced': '고급 설정',
        'setting.cert.label': 'HTTPS 인증서',
        'setting.cert.desc': '이 기기에서 서버를 신뢰하기 위해 인증서를 다운로드합니다.',
        'action.downloadCert': '인증서(cert.pem) 다운로드',
        // Chat
        'chat.welcome': '안녕하세요! 채팅할 준비가 되었습니다. 우측 상단 기어 아이콘에서 설정하세요.',
        'chat.instruction': '우측 상단 설정(⚙️)에서 설정을 변경하실 수 있습니다.',
        'chat.startup.welcomeTitle': '환영합니다.',
        'chat.startup.welcomeBody': '바로 대화를 시작하실 수 있습니다.',
        'chat.startup.issueBody': '아래 항목을 확인해 주세요.',
        'chat.startup.restore': '마지막 대화 불러오기',
        'chat.startup.restoreLoaded': '마지막 대화를 불러왔습니다.',
        'chat.startup.restoreMissing': '불러올 마지막 대화가 없습니다.',
        'chat.startup.restoreFailed': '마지막 대화를 불러오지 못했습니다.',
        'input.placeholder': '메시지를 입력하세요...',
        'input.placeholder.sttA': '지금 말하세요...',
        'input.placeholder.sttB': '듣는 중...',
        // Health Check
        'health.systemReady': '시스템 준비 완료',
        'health.checkRequired': '시스템 점검 필요',
        'health.checkFailed': '시스템 점검 실패',
        'health.backendError': '백엔드와 통신할 수 없습니다 (Wails 및 API 응답 없음).',
        'health.llm': 'LLM',
        'health.tts': 'TTS',
        'health.status.connected': '연결됨',
        'health.status.ready': '준비됨',
        'health.status.disabled': '비활성화됨',
        'health.status.unreachable': '연결 불가',
        'health.mode': '모드',
        'health.checkToken': ' -> **API Token**을 확인해주세요.',
        'health.checkServer': ' -> **LM Studio 서버**가 실행 중인지 확인해주세요.',
        'health.checkServer': ' -> **LM Studio 서버**가 실행 중인지 확인해주세요.',
        // Errors
        'error.authFailed': 'LM Studio 인증 실패.\n\n해결 방법:\n1. LM Studio -> Developer(사이드바) -> Server Settings\n2. \'Require Authentication\' 끄기\n3. 또는 \'Manage Tokens\'에서 \'Create new token\' API Key를 생성해서 우측 상단 설정(⚙️)에 입력하세요.\n\n원본 오류: ',
        'error.mcpFailed': 'LM Studio MCP 연결 실패.\n\n해결 방법:\n1. LM Studio -> Developer(사이드바) -> Server Settings\n2. \'Allow calling servers from mcp.json\' 옵션 켜기\n3. 또는 우측 상단 설정(⚙️)에서 \'MCP 기능 활성화\' 옵션을 꺼주세요.\n\n원본 오류: ',
        'error.contextExceeded': '대화 문맥 길이가 초과되었습니다. 사이드바 하단의 [문맥 초기화] 버튼을 눌러주세요.',
        'error.visionNotSupported': '선택한 모델은 이미지를 인식할 수 없습니다. 비전(Vision) 모델을 선택해주세요.',
        'warning.loopDetected': '[⚠️ 반복 응답 감지로 인해 응답 처리를 중단했습니다.]'
    },
    en: {
        // Modal
        'modal.settings.title': 'Settings',
        // Sections
        'section.llm': 'LLM Settings',
        'section.voiceInput': 'Voice Input',
        'section.tts': 'TTS Engine',
        // Server
        'server.stopped': 'Server: Stopped',
        'server.running': 'Server: Running',
        'server.port': 'Server Port',
        'server.start': 'Start Server',
        'server.stop': 'Stop Server',
        'error.contextExceeded': 'Context size has been exceeded. Please click [Reset Context] in the sidebar.',
        'error.visionNotSupported': 'The selected model does not support images. Please choose a vision-capable model.',
        // Actions
        'action.clearChat': 'Clear Chat History',
        'action.logout': 'Logout',
        'action.save': 'Save Settings',
        'action.cancel': 'Cancel',
        'action.reload': 'Reload',
        'action.clearContext': 'Reset Context',
        // Settings - LLM
        'setting.llmEndpoint.label': 'LLM Endpoint',
        'setting.model.label': 'Model Name',
        'setting.model.desc': 'Enter the model name loaded on your LLM server.',
        'setting.hideThink.label': 'Hide <think>',
        'setting.hideThink.desc': 'Hides the thinking process from the chat.',
        'setting.systemPrompt.label': 'System Prompt',
        'setting.systemPrompt.desc': 'Define the LLM\'s role. Example: "You are my English teacher." It can be modified in System_prompt.json.',
        'setting.temperature.label': 'Temperature',
        'setting.temperature.desc': '(Default: 0.7) Lower = predictable, Higher = creative',
        'setting.maxTokens.label': 'Max Tokens',
        'setting.maxTokens.desc': '(Default: 4096) Maximum tokens to generate',
        'setting.history.label': 'History Count',
        'setting.history.desc': '(Default: 10) Number of messages to remember',
        'setting.apiToken.label': 'API Token',
        'setting.apiToken.desc': 'LM Studio API Token (Required if Auth is enabled)',
        'setting.apiToken.placeholder': 'Leaving empty uses default',
        'setting.llmMode.label': 'Connection Mode',
        'setting.llmMode.desc': 'Select between OpenAI Compatible or LM Studio',
        'setting.llmMode.option.standard': 'OpenAI Compatible',
        'setting.llmMode.option.stateful': 'LM Studio',
        'setting.disableStateful.label': 'Disable Stateful Storage',
        'setting.disableStateful.desc': 'Do not store conversation on server (LM Studio).',
        'setting.enableMCP.label': 'Enable MCP Features',
        'setting.enableMCP.desc': 'Enable integration with Model Context Protocol (web search, browsing)',
        'setting.enableMemory.label': 'Enable Personal Memory',
        'setting.enableMemory.desc': 'Allow LLM to remember personal details in a local file.',
        'setting.statefulTurnLimit.label': 'Stateful Turn Limit',
        'setting.statefulTurnLimit.desc': '(Default: 8) Choose how many turns LM Studio keeps before compacting the conversation into a summary and starting a fresh chain.',
        'setting.statefulCharBudget.label': 'Stateful Character Budget',
        'setting.statefulCharBudget.desc': '(Default: 12000) If the estimated active context grows past this many characters, automatic compaction runs.',
        'setting.statefulTokenBudget.label': 'Stateful Token Budget',
        'setting.statefulTokenBudget.desc': '(Default: 10000) Primary token safety threshold used to decide when automatic context compaction should trigger.',
        'setting.memory.warning': 'Warning: Personal data is stored unencrypted on local disk.',
        'setting.memory.open': 'Open File',
        'setting.memory.reset': 'Reset Memory',
        'setting.memory.reset.confirm': 'Are you sure you want to reset your personal memory? This cannot be undone.',
        'setting.memory.reset.success': 'Memory reset successfully.',
        'setting.micLayout.label': 'Mic Layout',
        'setting.micLayout.desc': 'Place a microphone on the screen.',
        'setting.micLayout.option.none': 'None',
        'setting.micLayout.option.left': 'Left Side',
        'setting.micLayout.option.right': 'Right Side',
        'setting.micLayout.option.bottom': 'Bottom',
        'setting.micLayout.option.inline': 'In Message Input',
        'status.thinking': 'Thinking...',
        'status.live': 'Live',
        'status.running': 'Running',
        'status.done': 'Done',
        'status.failed': 'Failed',
        'status.stopped': 'Stopped',
        'status.unexpectedStop': 'The response stopped unexpectedly.',
        'status.thoughtForSeconds': 'Thought for {seconds}s',
        'status.thoughtForMinutes': 'Thought for {minutes}m',
        'status.thoughtForMinutesSeconds': 'Thought for {minutes}m {seconds}s',
        'tool.currentTimeChecked': 'Checked the current time.',
        'tool.executionFinished': 'Tool execution finished.',
        'tool.noQueryDetails': 'No query details',
        'tool.unknownError': 'Unknown error',
        'progress.processingPrompt': 'Processing Prompt',
        'progress.loadingModel': 'Loading Model',
        'progress.modelLoaded': 'Model Loaded',
        // Settings - TTS
        'setting.enableTTS.label': 'Enable TTS',
        'setting.enableTTS.desc': 'Play responses as audio.',
        'setting.autoPlay.label': 'Auto-play',
        'setting.autoPlay.desc': 'Automatically play audio responses.',
        'setting.voiceStyle.label': 'Voice Style',
        'setting.voiceStyle.desc': 'Select the TTS voice style.',
        'setting.speed.label': 'Speed',
        'setting.speed.desc': 'Audio playback speed.',
        'setting.ttsLang.label': 'TTS Language',
        'setting.ttsLang.desc': 'Select your preferred language.',
        'setting.chunkSize.label': 'Smart Chunking',
        'setting.chunkSize.desc': '(Recommended: 150~300) Characters per TTS chunk',
        'setting.steps.label': 'Inference Steps',
        'setting.steps.desc': '(Recommended: 2~8, Default: 5) Higher = more natural voice',
        'setting.threads.label': 'CPU Threads',
        'setting.threads.desc': '(Default: 2) CPU threads for TTS generation',
        'setting.format.label': 'Audio Format',
        'setting.format.desc': 'MP3 is converted from WAV.',
        // Advanced
        'section.advanced': 'Advanced Settings',
        'setting.cert.label': 'HTTPS Certificate',
        'setting.cert.desc': 'Download the certificate to trust this server on this device.',
        'action.downloadCert': 'Download cert.pem',
        // Chat
        'chat.welcome': 'Hello! I am ready to chat. Configure settings using the gear icon.',
        'chat.instruction': 'You can configure settings in the top right menu.',
        'chat.startup.welcomeTitle': 'Welcome.',
        'chat.startup.welcomeBody': 'You can start chatting right away.',
        'chat.startup.issueBody': 'Please check the items below.',
        'chat.startup.restore': 'Load last conversation',
        'chat.startup.restoreLoaded': 'Last conversation loaded.',
        'chat.startup.restoreMissing': 'No saved conversation to load.',
        'chat.startup.restoreFailed': 'Could not load the last conversation.',
        'input.placeholder': 'Type a message...',
        'input.placeholder.sttA': 'Speak now...',
        'input.placeholder.sttB': 'Listening...',
        // Health Check
        'health.systemReady': 'System Ready',
        'health.checkRequired': 'System Check Required',
        'health.checkFailed': 'System Check Failed',
        'health.backendError': 'Could not communicate with backend (neither Wails nor API).',
        'health.llm': 'LLM',
        'health.tts': 'TTS',
        'health.status.connected': 'Connected',
        'health.status.ready': 'Ready',
        'health.status.disabled': 'Disabled',
        'health.status.unreachable': 'Unreachable',
        'health.mode': 'Mode',
        'health.checkToken': ' -> Check **API Token**.',
        'health.checkServer': ' -> Check if **LM Studio Server** is running.',
        'health.checkServer': ' -> Check if **LM Studio Server** is running.',
        // Errors
        'error.authFailed': 'LM Studio Authentication Failed.\n\nSolution:\n1. Open LM Studio -> Developer (sidebar) -> Server Settings\n2. Turn OFF \'Require Authentication\'\n3. Or go to \'Manage Tokens\' -> \'Create new token\' and enter it in Settings.\n\nOriginal Error: ',
        'error.mcpFailed': 'LM Studio MCP Connection Failed.\n\nSolution:\n1. Open LM Studio -> Developer (sidebar) -> Server Settings\n2. Turn ON \'Allow calling servers from mcp.json\'\n3. Or turn OFF \'Enable MCP Features\' in the top right settings (⚙️).\n\nOriginal Error: ',
        'warning.loopDetected': '[⚠️ Repeated responses were detected, and the response was stopped.]'
    }
};

function t(key) {
    const lang = config.language || 'ko';
    return translations[lang]?.[key] || translations['en']?.[key] || key;
}

function normalizeInlineMarkdownSpacing(segment) {
    return segment.replace(/\*\*\s+([^*\n](?:[^*\n]*?[^*\s\n])?)\s+\*\*/g, '**$1**');
}

function normalizeMarkdownOutsideCode(text, transform) {
    const parts = String(text).split(/(```[\s\S]*?```|`[^`\n]+`)/g);
    return parts.map((part, index) => {
        if (index % 2 === 1) return part;
        return transform(part);
    }).join('');
}

function normalizeMarkdownForRender(text) {
    if (!text) return '';

    let normalized = String(text);

    // Remove invisible characters that can break markdown emphasis or list parsing.
    normalized = normalized.replace(/[\u200B-\u200D\u2060\uFEFF]/g, '');

    // Convert common unicode bullets into markdown list markers so streaming content
    // renders as proper nested lists instead of raw text lines.
    normalized = normalized
        .replace(/(^|\n)([ \t]*)[•●▪■▸▹▻▶▷►]\s+/g, '$1$2- ')
        .replace(/(^|\n)([ \t]*)[◦○◇◆]\s+/g, '$1$2  - ');

    // Normalize stray spaces inside strong markers without touching code spans/blocks.
    normalized = normalizeMarkdownOutsideCode(normalized, (segment) =>
        normalizeInlineMarkdownSpacing(segment)
            .replace(/(^|\n)([ \t]*[-*+]\s+)\*\*\s+([^*\n]+?)\s+\*\*/g, '$1$2**$3**')
    );

    // Normalize markdown headings/lists/tables when streamed without enough spacing.
    normalized = normalized
        .replace(/\r\n/g, '\n')
        .replace(/\r/g, '\n')
        .replace(/([^\n])\n(#{1,6}\s)/g, '$1\n\n$2')
        .replace(/([^\n])\n((?:[-*+]\s|\d+\.\s))/g, '$1\n\n$2')
        .replace(/([^\n])\n(\|.+\|)/g, '$1\n\n$2')
        .replace(/(\|[^\n]+\|)\n(?=\|[-:\s|]+\|)/g, '$1\n')
        // Streaming sometimes inserts blank lines between markdown table rows.
        // Collapse those gaps so GFM parsers can recognize the table again.
        .replace(/(\|[^\n]+\|)\n\s*\n(?=\|[-:\s|]+\|)/g, '$1\n')
        .replace(/(\|[-:\s|]+\|)\n\s*\n(?=\|)/g, '$1\n')
        .replace(/(\|[^\n]+\|)\n\s*\n(?=\|[^\n]+\|)/g, '$1\n')
        .replace(/\n{3,}/g, '\n\n');

    return normalized;
}

function applyTranslations() {
    const lang = config.language || 'ko';
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (translations[lang]?.[key]) {
            el.textContent = translations[lang][key];
        }
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        if (translations[lang]?.[key]) {
            el.placeholder = translations[lang][key];
        }
    });
    // Update language selector
    const langSelect = document.getElementById('cfg-lang');
    if (langSelect) langSelect.value = lang;
    updateMessageInputPlaceholder();
}

function setLanguage(lang) {
    config.language = lang;
    localStorage.setItem('appConfig', JSON.stringify(config));
    applyTranslations();
}

// ============================================================================
// Screen Wake Lock API
// ============================================================================
let wakeLock = null;

// Audio Context Recovery for iOS/PWA
document.addEventListener('visibilitychange', async () => {
    if (document.visibilityState === 'visible') {
        console.log('[Audio] App foregrounded, checking audio context...');

        // Re-acquire Wake Lock if it was active
        if (isPlayingQueue || isGenerating) {
            await requestWakeLock();
        }
    }
});

// Unlock audio context on first user interaction (critical for iOS)
const unlockAudio = () => {
    // We don't use Web Audio API directly for playback (using Audio element), 
    // but this pattern helps browser policies favorable to audio
    const audio = new Audio();
    audio.play().catch(() => { });
    document.removeEventListener('touchstart', unlockAudio);
    document.removeEventListener('click', unlockAudio);
    console.log('[Audio] Audio unlocked by user interaction');
};
document.addEventListener('touchstart', unlockAudio);
document.addEventListener('click', unlockAudio);

async function requestWakeLock() {
    if ('wakeLock' in navigator) {
        try {
            wakeLock = await navigator.wakeLock.request('screen');
            console.log('[WakeLock] Screen Wake Lock active');
            wakeLock.addEventListener('release', () => {
                console.log('[WakeLock] Screen Wake Lock released');
            });
        } catch (err) {
            console.error(`[WakeLock] Failed to request Wake Lock: ${err.name}, ${err.message}`);
        }
    }
}

async function releaseWakeLock() {
    if (wakeLock) {
        try {
            await wakeLock.release();
            wakeLock = null;
        } catch (err) {
            console.error(`[WakeLock] Failed to release Wake Lock: ${err.name}, ${err.message}`);
        }
    }
}

// ============================================================================
// Settings Modal Control
// ============================================================================

/**
 * Fetch available models from LLM server and populate dropdown
 */
async function fetchModels() {
    const select = document.getElementById('cfg-model');
    if (!select) return;

    try {
        const response = await fetch('/api/models', { credentials: 'include' });
        if (!response.ok) {
            const errText = await response.text();
            throw new Error(errText || `HTTP ${response.status}`);
        }

        const data = await response.json();
        console.log('[Models] Raw response:', data);

        let models = [];
        if (Array.isArray(data)) {
            models = data;
        } else if (data.data && Array.isArray(data.data)) {
            models = data.data;
        } else if (data.object === 'list' && Array.isArray(data.data)) {
            models = data.data;
        } else if (data.models && Array.isArray(data.models)) {
            // LM Studio /api/v1/models format
            models = data.models.map(m => ({
                id: m.key, // Map key to id
                ...m
            }));
        } else {
            console.warn('[Models] Unexpected format:', data);
        }

        // Clear existing options
        select.innerHTML = '';

        if (models.length === 0) {
            select.innerHTML = '<option value="">No models available</option>';
            return;
        }

        // Populate with models
        models.forEach(model => {
            const option = document.createElement('option');
            option.value = model.id;
            option.textContent = model.id;
            select.appendChild(option);
        });

        // Select current config value if it exists
        if (config.model && Array.from(select.options).some(opt => opt.value === config.model)) {
            select.value = config.model;
        } else if (models.length > 0) {
            select.value = models[0].id;
            config.model = models[0].id;
        }
    } catch (err) {
        console.error('[Models] Failed to fetch:', err);
        // Show specific error in dropdown
        select.innerHTML = `<option value="">Error: ${err.message}</option>`;

        // Also add a manual input option
        const manualOption = document.createElement('option');
        manualOption.value = config.model || '';
        manualOption.textContent = config.model || 'Enter model manually';
        select.appendChild(manualOption);
    }
}

function openSettingsModal() {
    document.getElementById('settings-modal').classList.add('active');
    fetchModels(); // Populate model dropdown when modal opens
}

function closeSettingsModal() {
    document.getElementById('settings-modal').classList.remove('active');
}

/**
 * Handle certificate download
 */
async function downloadCertificate() {
    try {
        const response = await fetch('/api/cert/download', { credentials: 'include' });
        if (!response.ok) throw new Error('Failed to download certificate');

        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'cert.pem';
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
    } catch (err) {
        console.error('[Cert] Download failed:', err);
        alert('인증서 다운로드 실패: ' + err.message);
    }
}

// Chat State
let messages = [];
let pendingImage = null;
let isGenerating = false;
let abortController = null;
let lastResponseId = null; // For Stateful Chat
let statefulTurnCount = 0;
let statefulEstimatedChars = 0;
let statefulSummary = '';
let statefulResetCount = 0;
let pendingStatefulResetReason = null;
let statefulLastInputTokens = 0;
let statefulLastOutputTokens = 0;
let statefulPeakInputTokens = 0;

// Audio State
let currentAudio = null;
let currentAudioBtn = null;
let audioWarmup = null; // Used to bypass auto-play blocks
let ttsQueue = [];
let activeTTSSessionLabel = "";

let isPlayingQueue = false;

// Streaming TTS State
let streamingTTSActive = false;
let streamingTTSCommittedIndex = 0; // How much of the display text has been sent to TTS
let streamingTTSBuffer = ""; // Uncommitted text buffer
let streamingTTSProcessor = null; // Reference to the active processor loop
let ttsSessionId = 0;


// DOM Elements
const chatMessages = document.getElementById('chat-messages');
const AUTO_SCROLL_THRESHOLD_PX = 80;
let shouldAutoScroll = true;
let autoScrollHoldTimeout = null;
let autoScrollResizeObserver = null;
let lockScrollToLatest = false;
let suppressNextScrollEvent = false;
let activeStreamingMessageId = null;
let pendingScrollToBottom = false;
let progressDockHideTimer = null;
let composerProgressLabel = '';
let composerProgressActive = false;
let composerProgressPercent = null;

if (chatMessages) {
    chatMessages.addEventListener('scroll', () => {
        if (suppressNextScrollEvent) {
            suppressNextScrollEvent = false;
            return;
        }

        shouldAutoScroll = isChatNearBottom();
        if (isGenerating) {
            if (shouldAutoScroll) {
                lockScrollToLatest = true;
            } else {
                const distanceFromBottom = chatMessages.scrollHeight - chatMessages.clientHeight - chatMessages.scrollTop;
                if (distanceFromBottom > AUTO_SCROLL_THRESHOLD_PX * 2) {
                    lockScrollToLatest = false;
                }
            }
        }
    }, { passive: true });
}
const messageInput = document.getElementById('message-input');
const sendBtn = document.getElementById('send-btn');
const imagePreviewVal = document.getElementById('image-preview');
const previewContainer = document.getElementById('preview-container');
const chatProgressDock = document.getElementById('chat-progress-dock');
const inputArea = document.getElementById('input-area');
const inputContainer = document.querySelector('#input-area .input-container');
const inlineMicBtn = document.getElementById('inline-mic-btn');

function updateViewportMetrics() {
    const root = document.documentElement;
    const vv = window.visualViewport;
    const visibleHeight = vv ? vv.height : window.innerHeight;
    const offsetTop = vv ? vv.offsetTop : 0;
    const occupiedBottom = vv ? Math.max(0, window.innerHeight - (vv.height + vv.offsetTop)) : 0;

    root.style.setProperty('--app-height', `${Math.round(visibleHeight + offsetTop)}px`);
    root.style.setProperty('--viewport-bottom-offset', `${Math.round(occupiedBottom)}px`);
}

if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', updateViewportMetrics);
    window.visualViewport.addEventListener('scroll', updateViewportMetrics);
}
window.addEventListener('resize', updateViewportMetrics, { passive: true });
window.addEventListener('orientationchange', updateViewportMetrics);
updateViewportMetrics();

// Audio Context for Auto-play
let audioContextUnlocked = false;
let audioCtx = null;

async function unlockAudioContext() {
    if (!audioCtx) {
        const AudioContext = window.AudioContext || window.webkitAudioContext;
        if (AudioContext) {
            audioCtx = new AudioContext();
        }
    }

    if (audioCtx && audioCtx.state === 'suspended') {
        try {
            await audioCtx.resume();
            // Play silent buffer to unlock state
            const buffer = audioCtx.createBuffer(1, 1, 22050);
            const source = audioCtx.createBufferSource();
            source.buffer = buffer;
            source.connect(audioCtx.destination);
            source.start(0);
            audioContextUnlocked = true;
            console.log("AudioContext unlocked/resumed");
        } catch (e) {
            console.error("Failed to resume AudioContext", e);
        }
    }

    // Initialize/Unlock HTML5 Audio element for iOS
    if (!currentAudio) {
        currentAudio = new Audio();
        currentAudio.playsInline = true;
        // iOS requires a play() call during user interaction to unlock background audio
        currentAudio.src = 'data:audio/wav;base64,UklGRigAAABXQVZFRm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQQAAAAAAA=='; // 1 sample silent wav
        currentAudio.play().catch(() => { });
    }
}

/**
 * Update Media Session Metadata
 */
function updateMediaSessionMetadata(text) {
    if ('mediaSession' in navigator) {
        navigator.mediaSession.metadata = new MediaMetadata({
            title: text || "TTS Message",
            artist: "DKST Chat",
            album: "Local LLM Gateway",
            artwork: [
                { src: 'apple-touch-icon.png', sizes: '180x180', type: 'image/png' }
            ]
        });

        // Set action handlers
        navigator.mediaSession.setActionHandler('pause', () => {
            stopAllAudio();
        });
        navigator.mediaSession.setActionHandler('stop', () => {
            stopAllAudio();
        });
    }
}

function clearMediaSessionMetadata() {
    if ('mediaSession' in navigator) {
        navigator.mediaSession.metadata = null;
    }
}

function readWavHeader(view) {
    if (view.byteLength < 44) return null;

    const readTag = (offset) => String.fromCharCode(
        view.getUint8(offset),
        view.getUint8(offset + 1),
        view.getUint8(offset + 2),
        view.getUint8(offset + 3)
    );

    if (readTag(0) !== 'RIFF' || readTag(8) !== 'WAVE') return null;

    let offset = 12;
    let fmt = null;
    let dataOffset = -1;
    let dataSize = 0;

    while (offset + 8 <= view.byteLength) {
        const chunkId = readTag(offset);
        const chunkSize = view.getUint32(offset + 4, true);
        const chunkDataStart = offset + 8;

        if (chunkId === 'fmt ') {
            if (chunkSize < 16 || chunkDataStart + chunkSize > view.byteLength) return null;
            fmt = {
                audioFormat: view.getUint16(chunkDataStart, true),
                channelCount: view.getUint16(chunkDataStart + 2, true),
                sampleRate: view.getUint32(chunkDataStart + 4, true),
                bitsPerSample: view.getUint16(chunkDataStart + 14, true),
                raw: new Uint8Array(view.buffer.slice(chunkDataStart, chunkDataStart + chunkSize))
            };
        } else if (chunkId === 'data') {
            dataOffset = chunkDataStart;
            dataSize = Math.min(chunkSize, view.byteLength - chunkDataStart);
            break;
        }

        offset = chunkDataStart + chunkSize + (chunkSize % 2);
    }

    if (!fmt || dataOffset < 0) return null;

    return { fmt, dataOffset, dataSize };
}

function concatenateWavArrayBuffers(buffers) {
    if (!buffers || buffers.length === 0) return null;
    if (buffers.length === 1) return buffers[0];

    const parts = [];
    let totalDataLength = 0;

    for (const buffer of buffers) {
        const header = readWavHeader(new DataView(buffer));
        if (!header || header.fmt.audioFormat !== 1) return null;

        if (parts.length > 0) {
            const baseFmt = parts[0].fmt;
            if (
                header.fmt.channelCount !== baseFmt.channelCount ||
                header.fmt.sampleRate !== baseFmt.sampleRate ||
                header.fmt.bitsPerSample !== baseFmt.bitsPerSample
            ) {
                return null;
            }
        }

        parts.push({
            fmt: header.fmt,
            data: new Uint8Array(buffer, header.dataOffset, header.dataSize)
        });
        totalDataLength += header.dataSize;
    }

    const result = new Uint8Array(44 + totalDataLength);
    const view = new DataView(result.buffer);
    const fmtChunk = parts[0].fmt.raw;

    const writeTag = (offset, tag) => {
        for (let i = 0; i < tag.length; i++) {
            view.setUint8(offset + i, tag.charCodeAt(i));
        }
    };

    writeTag(0, 'RIFF');
    view.setUint32(4, 36 + totalDataLength, true);
    writeTag(8, 'WAVE');
    writeTag(12, 'fmt ');
    view.setUint32(16, 16, true);
    result.set(fmtChunk.slice(0, 16), 20);
    writeTag(36, 'data');
    view.setUint32(40, totalDataLength, true);

    let offset = 44;
    for (const part of parts) {
        result.set(part.data, offset);
        offset += part.data.length;
    }

    return result.buffer;
}

async function promiseWithTimeout(promise, timeoutMs) {
    let timeoutId = null;
    try {
        return await Promise.race([
            promise,
            new Promise((resolve) => {
                timeoutId = setTimeout(() => resolve(null), timeoutMs);
            })
        ]);
    } finally {
        if (timeoutId) clearTimeout(timeoutId);
    }
}

async function combinePlayableChunks(primaryUrl, queuedTexts) {
    if (!primaryUrl || !queuedTexts || queuedTexts.length === 0) {
        return { url: primaryUrl, revokeInputs: null };
    }

    if ((config.ttsFormat || 'wav') !== 'wav') {
        return { url: primaryUrl, revokeInputs: null };
    }

    const urls = [primaryUrl];
    const consumedTexts = [];

    for (let i = 0; i < Math.min(2, queuedTexts.length); i++) {
        const nextText = queuedTexts[i];
        const cachedPromise = ttsAudioCache.get(nextText);
        if (!cachedPromise) break;

        const nextUrl = await promiseWithTimeout(cachedPromise, 120);
        if (!nextUrl) break;

        urls.push(nextUrl);
        consumedTexts.push(nextText);
    }

    if (urls.length === 1) {
        return { url: primaryUrl, revokeInputs: null };
    }

    try {
        const buffers = await Promise.all(urls.map(async (url) => {
            const response = await fetch(url);
            return await response.arrayBuffer();
        }));
        const mergedBuffer = concatenateWavArrayBuffers(buffers);
        if (!mergedBuffer) {
            return { url: primaryUrl, revokeInputs: null };
        }

        for (const text of consumedTexts) {
            if (ttsQueue[0] === text) {
                ttsQueue.shift();
            } else {
                const idx = ttsQueue.indexOf(text);
                if (idx >= 0) ttsQueue.splice(idx, 1);
            }
            ttsAudioCache.delete(text);
        }

        return {
            url: URL.createObjectURL(new Blob([mergedBuffer], { type: 'audio/wav' })),
            revokeInputs: urls
        };
    } catch (e) {
        console.error('[TTS] Failed to combine WAV chunks:', e);
        return { url: primaryUrl, revokeInputs: null };
    }
}


// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    updateViewportMetrics();
    // Check authentication first
    await checkAuth();

    await checkAuth();

    // Initial Config Load
    try {
        loadConfig();
    } catch (e) {
        console.error("Config load failed, using defaults:", e);
    }

    fetchModels().catch(console.warn); // Fetch models in background

    await loadVoiceStyles(); // Fetch voice styles
    await syncServerConfig(); // Sync with server
    setupEventListeners();
    initServerControl();

    // Initial System Check
    setTimeout(checkSystemHealth, 500);

    // Start Location Tracking
    updateUserLocation();
    setInterval(updateUserLocation, 300000); // Update every 5 mins


    // Setup Markdown
    marked.setOptions({
        gfm: true,
        breaks: true,
        highlight: function (code, lang) {
            const language = highlight.getLanguage(lang) ? lang : 'plaintext';
            return highlight.highlight(code, { language }).value;
        },
        langPrefix: 'hljs language-'
    });

    // Register Service Worker for PWA
    if ('serviceWorker' in navigator) {
        window.addEventListener('load', () => {
            navigator.serviceWorker.register('/sw.js?v=3')
                .then(reg => console.log('[PWA] Service Worker registered:', reg.scope))
                .catch(err => console.warn('[PWA] Service Worker failed:', err));
        });
    }
});

// Current user state
let currentUser = null;
let currentUserLocation = null; // Store location: {lat, lon, accuracy}
let lastSessionCache = null;

// Location Tracking
function updateUserLocation() {
    if (!navigator.geolocation) {
        console.warn("[Location] Geolocation not supported");
        return;
    }
    navigator.geolocation.getCurrentPosition(
        (position) => {
            const { latitude, longitude, accuracy } = position.coords;
            // Format loosely: "Lat: 37.5, Lon: 127.0 (Acc: 10m)"
            // Or JSON:
            currentUserLocation = JSON.stringify({
                lat: latitude,
                lon: longitude,
                acc: accuracy
            });
            console.log("[Location] Updated:", currentUserLocation);
        },
        (err) => {
            console.warn("[Location] Error:", err.message);
            currentUserLocation = null;
        },
        { enableHighAccuracy: false, timeout: 10000, maximumAge: 600000 } // 10 min cache
    );
}

// Check authentication status
async function checkAuth() {
    try {
        const response = await fetch('/api/auth/check', { credentials: 'include' });
        const data = await response.json();

        if (!data.authenticated) {
            window.location.href = '/login.html';
            return;
        }

        currentUser = {
            id: data.user_id,
            role: data.role
        };

        // Show admin features if admin
        if (currentUser.role === 'admin') {
            const adminSection = document.getElementById('admin-section');
            if (adminSection) adminSection.style.display = 'block';
            loadUserList();
        }
    } catch (e) {
        console.error('Auth check failed:', e);
        // Don't redirect on network error (might be running in Wails)
    }
}

// Load user list for admin
async function loadUserList() {
    try {
        const response = await fetch('/api/users');
        if (!response.ok) return;

        const users = await response.json();
        const listEl = document.getElementById('user-list');
        if (!listEl) return;

        listEl.innerHTML = users.map(u => `
            <div class="user-item">
                <span>${u.id} (${u.role})</span>
                ${u.id !== currentUser.id ? `<button class="icon-btn" onclick="deleteUser('${u.id}')" title="Delete"><span class="material-icons-round">delete</span></button>` : ''}
            </div>
        `).join('');
    } catch (e) {
        console.error('Failed to load users:', e);
    }
}

// Add user
async function addUser() {
    const id = prompt('Enter username:');
    if (!id) return;

    const password = prompt('Enter password:');
    if (!password) return;

    const role = confirm('Grant admin privileges?') ? 'admin' : 'user';

    try {
        const response = await fetch('/api/users/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, password, role })
        });

        if (response.ok) {
            loadUserList();
            alert('User added successfully');
        } else {
            alert('Failed to add user');
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Delete user
async function deleteUser(id) {
    if (!confirm(`Delete user "${id}"?`)) return;

    try {
        const response = await fetch('/api/users/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id })
        });

        if (response.ok) {
            loadUserList();
        } else {
            alert('Failed to delete user');
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

// Logout
async function logout() {
    try {
        await fetch('/api/logout', { method: 'POST' });
        window.location.href = '/login.html';
    } catch (e) {
        console.error('Logout failed:', e);
    }
}

// Server state
let serverRunning = false;

// Initialize server control and check status
async function initServerControl() {
    // Check if Wails runtime is available
    if (typeof window.go === 'undefined') {
        console.log('Wails runtime not detected. Running in web mode.');
        document.querySelector('.server-control').style.display = 'none';
        // Web mode: do not add is-desktop class
        return;
    }

    // Desktop mode: add class to show desktop-only elements
    document.body.classList.add('is-desktop');

    // Get initial server status
    try {
        const status = await window.go.main.App.GetServerStatus();
        updateServerUI(status.running, status.port);
    } catch (e) {
        console.error('Failed to get server status:', e);
    }
}

// Toggle server start/stop
async function toggleServer() {
    if (typeof window.go === 'undefined') {
        alert('Wails runtime not available.');
        return;
    }

    const port = document.getElementById('server-port').value;
    const btn = document.getElementById('server-toggle-btn');
    btn.disabled = true;

    try {
        if (serverRunning) {
            await window.go.main.App.StopServer();
            updateServerUI(false, port);
        } else {
            // Also update LLM endpoint
            // const llmEndpoint = document.getElementById('cfg-api').value; // UI Element removed
            await window.go.main.App.SetLLMEndpoint(config.apiEndpoint);
            await window.go.main.App.StartServer(port);
            updateServerUI(true, port);
        }
    } catch (e) {
        alert('Server error: ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

// Update server status UI
function updateServerUI(running, port) {
    serverRunning = running;
    const statusEl = document.getElementById('server-status');
    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('span:last-child');
    const btn = document.getElementById('server-toggle-btn');

    if (running) {
        dot.className = 'status-dot running';
        text.textContent = `Server: Running on :${port}`;
        btn.innerHTML = '<span class="material-icons-round">stop</span> Stop Server';
    } else {
        dot.className = 'status-dot stopped';
        text.textContent = 'Server: Stopped';
        btn.innerHTML = '<span class="material-icons-round">play_arrow</span> Start Server';
    }
}

function loadConfig() {
    const saved = localStorage.getItem('appConfig');
    if (saved) {
        try {
            config = { ...config, ...JSON.parse(saved) };
        } catch (e) {
            console.error('Failed to parse saved config:', e);
            // Optional: localStorage.removeItem('appConfig');
        }
    }

    // Update UI
    const cfgApi = document.getElementById('cfg-api');
    if (cfgApi) cfgApi.value = config.apiEndpoint;
    document.getElementById('cfg-model').value = config.model;
    document.getElementById('cfg-hide-think').checked = config.hideThink;
    document.getElementById('cfg-temp').value = config.temperature;
    document.getElementById('cfg-max-tokens').value = config.maxTokens;
    document.getElementById('cfg-history').value = config.historyCount;
    const apiTokenEl = document.getElementById('cfg-api-token');
    if (apiTokenEl) apiTokenEl.value = config.apiToken || '';
    document.getElementById('cfg-llm-mode').value = config.llmMode || 'standard';
    document.getElementById('cfg-disable-stateful').checked = config.disableStateful || false;
    updateSettingsVisibility(); // Update UI visibility based on mode
    document.getElementById('cfg-enable-tts').checked = config.enableTTS;
    // Load MCP setting
    const mcpEl = document.getElementById('cfg-enable-mcp');
    if (mcpEl) mcpEl.checked = config.enableMCP || false;

    // Load Memory Setting
    const memEl = document.getElementById('setting-enable-memory');
    if (memEl) memEl.checked = config.enableMemory || false;
    const memControls = document.getElementById('memory-controls');
    if (memControls) memControls.style.display = config.enableMemory ? 'block' : 'none';
    const statefulTurnLimitEl = document.getElementById('cfg-stateful-turn-limit');
    if (statefulTurnLimitEl) statefulTurnLimitEl.value = parseInt(config.statefulTurnLimit, 10) || 8;
    const statefulCharBudgetEl = document.getElementById('cfg-stateful-char-budget');
    if (statefulCharBudgetEl) statefulCharBudgetEl.value = parseInt(config.statefulCharBudget, 10) || 12000;
    const statefulTokenBudgetEl = document.getElementById('cfg-stateful-token-budget');
    if (statefulTokenBudgetEl) statefulTokenBudgetEl.value = parseInt(config.statefulTokenBudget, 10) || 10000;

    document.getElementById('cfg-auto-tts').checked = config.autoTTS || false;
    document.getElementById('cfg-tts-lang').value = config.ttsLang;
    document.getElementById('cfg-chunk-size').value = config.chunkSize || 300;
    document.getElementById('cfg-system-prompt').value = config.systemPrompt || 'You are a helpful AI assistant.';
    if (config.ttsVoice) document.getElementById('cfg-tts-voice').value = config.ttsVoice;
    document.getElementById('cfg-tts-speed').value = config.ttsSpeed || 1.0;
    document.getElementById('speed-val').textContent = config.ttsSpeed || 1.0;
    document.getElementById('cfg-tts-steps').value = config.ttsSteps || 5;
    document.getElementById('steps-val').textContent = config.ttsSteps || 5;
    document.getElementById('cfg-tts-threads').value = config.ttsThreads || 4;
    document.getElementById('threads-val').textContent = config.ttsThreads || 4;
    let format = config.ttsFormat || 'wav';
    if (format === 'mp3') format = 'mp3-high'; // Legacy mapping
    document.getElementById('cfg-tts-format').value = format;

    // Mic Layout
    document.getElementById('cfg-mic-layout').value = config.micLayout || 'none';
    updateMicLayout();

    // Language selector
    document.getElementById('cfg-lang').value = config.language || 'ko';

    // Update header with model name
    const headerModelName = document.getElementById('header-model-name');
    if (headerModelName) {
        headerModelName.textContent = config.model || 'No Model Set';
    }

    applyChatFontSize();

    // Apply i18n translations
    // Apply i18n translations
    applyTranslations();

    // Initialize System Prompt Presets (loads from external file)
    loadSystemPrompts();

    // Load TTS Dictionary
    loadTTSDictionary();

    // Setup settings listeners
    setupSettingsListeners();
}

function updateSettingsVisibility() {
    const mode = document.getElementById('cfg-llm-mode').value;
    const tokenContainer = document.getElementById('container-api-token');
    const historyContainer = document.getElementById('container-history');
    const disableStatefulContainer = document.getElementById('container-disable-stateful');
    const mcpContainer = document.getElementById('container-enable-mcp');
    const memContainer = document.getElementById('container-enable-memory');
    const statefulBudgetContainer = document.getElementById('container-stateful-budget');

    // Default (Standard/OpenAI Compatible)
    let showToken = true; // User requested API Token visible in BOTH modes
    let showHistory = true;
    let showDisableStateful = false;
    let showMCP = false; // Hidden in OpenAI mode per request
    let fullText = '';
    let readingBuffer = '';
    let lastResponseId = null;
    let ttsReceived = false;
    let lastToolCallHtml = ''; // Track last tool call HTML to update in-place

    if (mode === 'stateful') {
        // LM Studio Mode
        showToken = true;
        showHistory = false; // LM Studio handles history via response_id
        showDisableStateful = true;
        showMCP = true; // Show MCP only in LM Studio/Stateful mode? 
        // User said: "OpenAI 호환 모드일 때 Enable MCP Features 는 안보여야 합니다."
        // So we default showMCP = false for standard, true for stateful.
    } else {
        // Standard mode -> MCP Hidden, but Token Visible
    }

    if (tokenContainer) tokenContainer.style.display = showToken ? 'block' : 'none';
    if (historyContainer) historyContainer.style.display = showHistory ? 'block' : 'none';
    if (disableStatefulContainer) disableStatefulContainer.style.display = showDisableStateful ? 'block' : 'none';
    if (mcpContainer) mcpContainer.style.display = showMCP ? 'block' : 'none';
    if (statefulBudgetContainer) statefulBudgetContainer.style.display = mode === 'stateful' ? 'block' : 'none';

    // Memory setting is visible only when MCP is both supported (LM Studio mode) and enabled
    const mcpEnabled = document.getElementById('cfg-enable-mcp').checked;
    if (memContainer) memContainer.style.display = (showMCP && mcpEnabled) ? 'block' : 'none';
}

function setupSettingsListeners() {
    // Save button explicit handler
    const saveBtn = document.getElementById('save-cfg-btn');
    if (saveBtn) {
        saveBtn.onclick = () => saveConfig(true);
    }

    // Sliders: update label on input, save on change
    const sliders = [
        { id: 'cfg-tts-speed', val: 'speed-val' },
        { id: 'cfg-tts-steps', val: 'steps-val' },
        { id: 'cfg-tts-threads', val: 'threads-val' }
    ];

    sliders.forEach(item => {
        const el = document.getElementById(item.id);
        const valEl = document.getElementById(item.val);
        if (el) {
            el.oninput = () => { if (valEl) valEl.textContent = el.value; };
            el.onchange = () => saveConfig(false);
        }
    });

    // Selects & Inputs: save on change
    const autoSaveIds = ['cfg-api', 'cfg-tts-lang', 'cfg-tts-voice', 'cfg-tts-format', 'cfg-chunk-size', 'cfg-system-prompt', 'cfg-llm-mode', 'cfg-disable-stateful', 'cfg-stateful-turn-limit', 'cfg-stateful-char-budget', 'cfg-stateful-token-budget'];
    autoSaveIds.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.onchange = () => saveConfig(false);
    });

    // Enable Memory Checkbox
    const memCheck = document.getElementById('setting-enable-memory');
    if (memCheck) {
        memCheck.onchange = () => {
            config.enableMemory = memCheck.checked;
            const controls = document.getElementById('memory-controls');
            if (controls) controls.style.display = config.enableMemory ? 'block' : 'none';
            saveConfig(false);
        };
    }

    // Memory Buttons
    const openMemBtn = document.getElementById('btn-open-memory');
    if (openMemBtn) {
        openMemBtn.onclick = async () => {
            const uid = (typeof currentUser !== 'undefined' && currentUser) ? currentUser.id : "default";
            try {
                const err = await window.go.main.App.OpenMemoryFolder(uid);
                if (err) alert(err);
            } catch (e) {
                alert("Error opening folder: " + e);
            }
        };
    }

    const resetMemBtn = document.getElementById('btn-reset-memory');
    if (resetMemBtn) {
        resetMemBtn.onclick = async () => {
            const confirmation = t('setting.memory.reset.confirm') || "Are you sure you want to reset your personal memory? This cannot be undone.";
            if (!confirm(confirmation)) return;
            const uid = (typeof currentUser !== 'undefined' && currentUser) ? currentUser.id : "default";
            try {
                const res = await window.go.main.App.ResetMemory(uid);
                alert(t('setting.memory.reset.success') || res);
            } catch (e) {
                alert("Error resetting memory: " + e);
            }
        };
    }
}

// Global Dictionary State
let ttsDictionary = {};
let ttsDictionaryRegex = null;

async function loadTTSDictionary(lang) {
    // Default to config language or 'ko' if undefined
    const targetLang = lang || config.ttsLang || 'ko';
    let rawDict = {};
    try {
        if (window.go && window.go.main && window.go.main.App) {
            rawDict = await window.go.main.App.GetTTSDictionary(targetLang);
        } else {
            const res = await fetch(`/api/dictionary?lang=${targetLang}`);
            if (res.ok) rawDict = await res.json();
        }

        // Normalize keys to lowercase for case-insensitive lookup
        ttsDictionary = {};
        if (rawDict) {
            for (const [k, v] of Object.entries(rawDict)) {
                ttsDictionary[k.toLowerCase()] = v;
            }
        }

        // Build optimized regex for performance (O(N) replacement)
        const keys = Object.keys(ttsDictionary);
        if (keys.length > 0) {
            // Escape special chars in keys
            const escapedKeys = keys.map(k => k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
            // Create regex matching any of the keys with word boundaries
            // Note: If keys contain spaces (e.g. "Mac OS"), \b might behave differently depending on chars
            // But user example "macOS" -> "Mac OS" is single word key. 
            // If user has "Mobile Phone", \bMobile Phone\b works.

            // 대소문자 구분을 하지 않기 위해 'g' 대신 'gi' 플래그 사용
            // ttsDictionaryRegex = new RegExp(`\\b(${escapedKeys.join('|')})\\b`, 'g');
            ttsDictionaryRegex = new RegExp(`\\b(${escapedKeys.join('|')})\\b`, 'gi');

            console.log(`[TTS] Dictionary loaded with ${keys.length} entries.`);
        }
    } catch (e) {
        console.error("Failed to load dictionary:", e);
    }
}

// 시스템 프롬프트 프리셋 (외부 파일에서 로드)
let systemPromptPresets = [];

async function loadSystemPrompts() {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            systemPromptPresets = await window.go.main.App.GetSystemPrompts();
        } else {
            const res = await fetch('/api/prompts');
            if (res.ok) systemPromptPresets = await res.json();
        }
        console.log(`[Prompts] Loaded ${systemPromptPresets.length} system prompts.`);
        initSystemPromptPresets(); // Re-initialize dropdown with loaded data
    } catch (e) {
        console.error("Failed to load system prompts:", e);
        systemPromptPresets = [{ title: "Default", prompt: "You are a helpful AI assistant." }];
    }
}

function applySystemPromptPreset(key) {
    const preset = systemPromptPresets.find(p => p.title === key);
    if (preset) {
        document.getElementById('cfg-system-prompt').value = preset.prompt;
    }
}

function initSystemPromptPresets() {
    const selector = document.getElementById('cfg-system-prompt-preset');
    if (!selector) return;

    // Clear existing options (except first)
    while (selector.options.length > 1) {
        selector.remove(1);
    }

    for (const preset of systemPromptPresets) {
        const option = document.createElement('option');
        option.value = preset.title;
        option.textContent = preset.title;
        selector.appendChild(option);
    }
}

// 외부 파일(system_prompts.json, dictionary_*.txt) 새로고침
async function reloadExternalFiles() {
    try {
        await loadSystemPrompts();
        await loadTTSDictionary(config.ttsLang);
        await fetchModels(); // Reload models
        showToast(t('action.reload') + ' ✓');
    } catch (e) {
        console.error("Failed to reload external files:", e);
        showToast('Reload failed');
    }
}

function saveConfig(closeModal = true) {
    const cfgApiEl = document.getElementById('cfg-api');
    // Sanitize Endpoint: Trim whitespace and trailing slash
    let endpoint = cfgApiEl ? cfgApiEl.value.trim() : config.apiEndpoint;
    if (endpoint.endsWith('/')) {
        endpoint = endpoint.slice(0, -1);
    }
    config.apiEndpoint = endpoint;

    config.model = document.getElementById('cfg-model').value.trim();
    config.hideThink = document.getElementById('cfg-hide-think').checked;
    config.temperature = parseFloat(document.getElementById('cfg-temp').value);
    config.maxTokens = parseInt(document.getElementById('cfg-max-tokens').value);
    config.historyCount = parseInt(document.getElementById('cfg-history').value);
    config.enableTTS = document.getElementById('cfg-enable-tts').checked;

    // Save MCP setting
    const mcpEl = document.getElementById('cfg-enable-mcp');
    config.enableMCP = mcpEl ? mcpEl.checked : false;

    // Save Memory setting
    const memEl = document.getElementById('setting-enable-memory');
    config.enableMemory = memEl ? memEl.checked : false;

    config.autoTTS = document.getElementById('cfg-auto-tts').checked;
    config.ttsLang = document.getElementById('cfg-tts-lang').value;

    // API Token handling - skip if element not present (web.html removed it)
    const apiTokenEl = document.getElementById('cfg-api-token');
    if (apiTokenEl) {
        const rawToken = apiTokenEl.value.trim();
        if (rawToken && !rawToken.startsWith('***') && !rawToken.includes('...')) {
            config.apiToken = rawToken;
        } else if (rawToken === '') {
            config.apiToken = '';
        }
    }

    config.llmMode = document.getElementById('cfg-llm-mode').value;
    config.disableStateful = document.getElementById('cfg-disable-stateful').checked;
    config.statefulTurnLimit = Math.max(1, parseInt(document.getElementById('cfg-stateful-turn-limit')?.value, 10) || 8);
    config.statefulCharBudget = Math.max(1000, parseInt(document.getElementById('cfg-stateful-char-budget')?.value, 10) || 12000);
    config.statefulTokenBudget = Math.max(1000, parseInt(document.getElementById('cfg-stateful-token-budget')?.value, 10) || 10000);
    config.micLayout = document.getElementById('cfg-mic-layout').value;
    config.chatFontSize = Math.max(12, Math.min(24, parseInt(config.chatFontSize, 10) || 16));

    // Update visibility immediately
    updateSettingsVisibility();
    updateMicLayout();
    applyChatFontSize();

    // Reload dictionary since language changes
    loadTTSDictionary(config.ttsLang);

    config.chunkSize = parseInt(document.getElementById('cfg-chunk-size').value) || 300;
    config.systemPrompt = document.getElementById('cfg-system-prompt').value.trim() || 'You are a helpful AI assistant.';
    config.ttsVoice = document.getElementById('cfg-tts-voice').value;
    config.ttsSpeed = parseFloat(document.getElementById('cfg-tts-speed').value);
    config.ttsSteps = parseInt(document.getElementById('cfg-tts-steps').value);
    config.ttsThreads = parseInt(document.getElementById('cfg-tts-threads').value);
    config.ttsFormat = document.getElementById('cfg-tts-format').value;

    localStorage.setItem('appConfig', JSON.stringify(config));

    // Sync configs to server
    if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetLLMEndpoint(config.apiEndpoint).catch(console.error);
        window.go.main.App.SetLLMApiToken(config.apiToken).catch(console.error);
        window.go.main.App.SetLLMMode(config.llmMode).catch(console.error);
        window.go.main.App.SetEnableTTS(config.enableTTS);
        window.go.main.App.SetEnableMCP(config.enableMCP);

        // This is separate from saveConfig in app.go, but SetTTSThreads triggers reload
        if (config.ttsThreads) {
            window.go.main.App.SetTTSThreads(config.ttsThreads);
        }
    }

    // Also try fetch for web mode or as backup
    // Build config payload - only include api_token if it was explicitly changed by user
    const configPayload = {
        api_endpoint: config.apiEndpoint,
        llm_mode: config.llmMode,
        enable_tts: config.enableTTS,
        enable_mcp: config.enableMCP,
        enable_memory: config.enableMemory,
        tts_threads: config.ttsThreads
    };

    // Only include api_token if element exists and has a valid non-masked value
    const apiTokenElForPost = document.getElementById('cfg-api-token');
    if (apiTokenElForPost) {
        const tokenVal = apiTokenElForPost.value.trim();
        if (tokenVal && !tokenVal.startsWith('***') && !tokenVal.includes('...')) {
            configPayload.api_token = tokenVal;
        } else if (tokenVal === '') {
            // Explicitly clearing token
            configPayload.api_token = '';
        }
        // If masked (***...), don't include api_token at all - preserve server value
    }

    fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(configPayload)
    }).then(r => {
        if (!r.ok) console.warn('Failed to sync settings');
    }).catch(e => console.warn('Sync error:', e));


    // Update header model name
    const headerModelName = document.getElementById('header-model-name');
    if (headerModelName) {
        headerModelName.textContent = config.model || 'No Model Set';
    }

    // Trigger explicit model load via backend
    if (config.model && config.apiEndpoint && config.apiEndpoint.includes('localhost')) {
        // Only try to auto-load if it looks like a local server (speed optimization)
        // Or we can just try always. Let's try always but non-blocking.
        fetch('/api/models', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model: config.model })
        }).then(async r => {
            if (r.ok) {
                console.log('[Model] Explicitly loaded:', config.model);
            } else {
                console.warn('[Model] Load skipped/failed:', await r.text());
            }
        }).catch(e => console.error('[Model] Load req error:', e));
    }

    // Close modal only if requested
    if (closeModal) {
        closeSettingsModal();
    }
    showToast(t('action.save') + ' ✓');
}

function applyChatFontSize() {
    const root = document.documentElement;
    const fontSize = Math.max(12, Math.min(24, parseInt(config.chatFontSize, 10) || 16));
    const lineHeight = fontSize >= 20 ? 1.7 : 1.6;

    config.chatFontSize = fontSize;
    root.style.setProperty('--chat-font-size', `${fontSize}px`);
    root.style.setProperty('--chat-line-height', String(lineHeight));

    if (typeof autoResizeInput === 'function') {
        autoResizeInput();
    }
}

async function fetchLastSession() {
    try {
        const response = await fetch('/api/last-session', { credentials: 'include' });
        if (!response.ok) {
            return null;
        }
        const data = await response.json();
        if (!data.has_session) {
            return null;
        }
        return data;
    } catch (e) {
        console.warn('Failed to fetch last session:', e);
        return null;
    }
}

function buildRestoredStatefulSummary(session) {
    const userText = cleanContentForStatefulSummary(session?.user_message || '');
    const assistantText = cleanContentForStatefulSummary(session?.assistant_message || '');
    if (!userText && !assistantText) return '';

    return [
        'Restored last conversation:',
        userText ? `User: ${userText}` : '',
        assistantText ? `Assistant: ${assistantText}` : ''
    ].filter(Boolean).join('\n');
}

async function saveLastSessionTurn(userMsg, assistantText) {
    const userText = (userMsg?.content || '').trim();
    const assistantMessage = (assistantText || '').trim();
    if (!currentUser || !userText || !assistantMessage) {
        return;
    }

    const payload = {
        user_message: userText,
        assistant_message: assistantMessage,
        mode: config.llmMode || 'standard'
    };

    try {
        const response = await fetch('/api/last-session', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify(payload)
        });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        lastSessionCache = {
            has_session: true,
            ...payload,
            updated_at: new Date().toISOString()
        };
    } catch (e) {
        console.warn('Failed to save last session:', e);
    }
}

async function restoreLastSession() {
    if (!lastSessionCache) {
        showToast(t('chat.startup.restoreMissing'), true);
        return;
    }

    clearChat();

    const restoredUser = {
        role: 'user',
        content: lastSessionCache.user_message || ''
    };
    const restoredAssistant = {
        role: 'assistant',
        content: lastSessionCache.assistant_message || ''
    };

    appendMessage(restoredUser);
    appendMessage(restoredAssistant);
    messages.push(restoredUser, restoredAssistant);

    if (config.llmMode === 'stateful') {
        statefulSummary = buildRestoredStatefulSummary(lastSessionCache);
        statefulEstimatedChars = statefulSummary.length;
        statefulTurnCount = 1;
        statefulLastInputTokens = estimateTokensFromText(statefulSummary);
        statefulLastOutputTokens = estimateTokensFromText(restoredAssistant.content);
        statefulPeakInputTokens = Math.max(statefulPeakInputTokens, statefulLastInputTokens);
    }

    showToast(t('chat.startup.restoreLoaded'));
}

function adjustChatFontSize(delta) {
    const nextSize = Math.max(12, Math.min(24, (parseInt(config.chatFontSize, 10) || 16) + delta));
    if (nextSize === config.chatFontSize) {
        return;
    }

    config.chatFontSize = nextSize;
    applyChatFontSize();
    localStorage.setItem('appConfig', JSON.stringify(config));
}

async function syncServerConfig() {
    try {
        const response = await fetch('/api/config', { credentials: 'include' }); // Fetch current server config
        if (response.ok) {
            const serverCfg = await response.json();
            console.log('[Config] Synced from server:', serverCfg);

            if (serverCfg.llm_endpoint) {
                config.apiEndpoint = serverCfg.llm_endpoint;
                const cfgApi = document.getElementById('cfg-api');
                if (cfgApi) cfgApi.value = config.apiEndpoint;
            }
            if (serverCfg.llm_mode) {
                config.llmMode = serverCfg.llm_mode;
                const cfgMode = document.getElementById('cfg-llm-mode');
                if (cfgMode) {
                    cfgMode.value = config.llmMode;
                    updateSettingsVisibility();
                }
            }
            if (serverCfg.enable_tts !== undefined) {
                config.enableTTS = serverCfg.enable_tts;
                document.getElementById('cfg-enable-tts').checked = config.enableTTS;
            }
            if (serverCfg.enable_mcp !== undefined) {
                config.enableMCP = serverCfg.enable_mcp;
                const mcpEl = document.getElementById('cfg-enable-mcp');
                if (mcpEl) mcpEl.checked = config.enableMCP;
            }
            if (serverCfg.enable_memory !== undefined) {
                config.enableMemory = serverCfg.enable_memory;
                const memEl = document.getElementById('setting-enable-memory');
                if (memEl) memEl.checked = config.enableMemory;
                const memControls = document.getElementById('memory-controls');
                if (memControls) memControls.style.display = config.enableMemory ? 'block' : 'none';
            }

            // Save to localStorage so next reload uses these
            localStorage.setItem('appConfig', JSON.stringify(config));
        }
    } catch (e) {
        console.warn('Failed to sync server config:', e);
    }
}

async function loadVoiceStyles() {
    try {
        const response = await fetch('/api/tts/styles');
        if (response.ok) {
            const styles = await response.json();
            const select = document.getElementById('cfg-tts-voice');
            select.innerHTML = styles.map(s => `<option value="${s}">${s}</option>`).join('');
            const saved = config.ttsVoice ? config.ttsVoice.replace('.json', '') : null;
            if (saved && styles.includes(saved)) {
                select.value = saved;
            } else if (styles.length > 0) {
                select.value = styles[0];
                config.ttsVoice = styles[0];
            }
        }
    } catch (e) {
        console.warn('Failed to load voice styles:', e);
    }
}


function toggleSwitch(id) {
    const el = document.getElementById(id);
    if (el) {
        el.checked = !el.checked;
        saveConfig(false);
    }
}

function setupEventListeners() {
    document.getElementById('save-cfg-btn').addEventListener('click', saveConfig);

    if (inputContainer) {
        inputContainer.addEventListener('pointerdown', (e) => {
            if (e.target.closest('.input-actions')) return;
            if (document.activeElement === messageInput) return;
            requestAnimationFrame(() => messageInput.focus());
        });

        inputContainer.addEventListener('click', (e) => {
            if (e.target.closest('.input-actions')) return;
            if (document.activeElement === messageInput) return;

            messageInput.focus();
        });
    }

    messageInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            // Fix Korean IME duplicate submission / residual character issue
            if (e.isComposing) return;
            e.preventDefault();

            unlockAudioContext(); // Unlock audio on user interaction
            sendMessage();
        }
        autoResizeInput();
    });

    messageInput.addEventListener('input', () => {
        autoResizeInput();
        updateInlineComposerActionVisibility();
    });

    // Paste Handle
    messageInput.addEventListener('paste', (e) => {
        const items = (e.clipboardData || e.originalEvent.clipboardData).items;
        let hasImage = false;
        for (let index in items) {
            const item = items[index];
            if (item.kind === 'file' && item.type.startsWith('image/')) {
                hasImage = true;
                const blob = item.getAsFile();
                const reader = new FileReader();
                reader.onload = function (event) {
                    pendingImage = event.target.result; // Base64 string
                    imagePreviewVal.src = pendingImage;
                    previewContainer.style.display = 'block';
                    updateInlineComposerActionVisibility();
                };
                reader.readAsDataURL(blob);
            }
        }
        // If an image was found, prevent default to avoid pasting source URLs or other metadata
        if (hasImage) {
            e.preventDefault();
        }
    });

    // TTS Settings listeners
    document.getElementById('cfg-tts-speed').addEventListener('input', (e) => {
        document.getElementById('speed-val').textContent = e.target.value;
    });
    document.getElementById('cfg-tts-steps').addEventListener('input', (e) => {
        document.getElementById('steps-val').textContent = e.target.value;
    });
    document.getElementById('cfg-tts-threads').addEventListener('input', (e) => {
        document.getElementById('threads-val').textContent = e.target.value;
    });

    // Stop handling
    sendBtn.addEventListener('click', () => {
        if (isGenerating) {
            stopGeneration();
        } else {
            unlockAudioContext(); // Unlock audio on user interaction
            sendMessage();
        }
    });

    // Enter key handling (prevent duplicate listener if one exists in HTML? No, setupEventListeners covers it)
    // Note: The previous listener on sendBtn was not shown in viewed lines but likely exists or was default form submission? 
    // Ah, line 301 calls sendMessage(). I need to make sure the Button Click also calls sendMessage OR stopGeneration.
    // I will Assume there isn't one and add it.
}

function autoResizeInput() {
    messageInput.style.height = 'auto';
    messageInput.style.height = Math.min(messageInput.scrollHeight, 150) + 'px';
    messageInput.style.overflowY = messageInput.scrollHeight > 150 ? 'auto' : 'hidden';
}

function handleImageUpload(input) {
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function (e) {
            pendingImage = e.target.result; // Base64 string
            imagePreviewVal.src = pendingImage;
            previewContainer.style.display = 'block';
            updateInlineComposerActionVisibility();
        };
        reader.readAsDataURL(input.files[0]);
    }
}

function removeImage() {
    pendingImage = null;
    document.getElementById('image-upload').value = '';
    previewContainer.style.display = 'none';
    updateInlineComposerActionVisibility();
}

function clearChat() {
    // Stop any TTS playback and generation
    stopAllAudio();

    if (isGenerating) {
        stopGeneration();
    }

    lastResponseId = null; // Clear session ID for Stateful Chat
    statefulTurnCount = 0;
    statefulEstimatedChars = 0;
    statefulSummary = '';
    statefulResetCount = 0;
    pendingStatefulResetReason = 'manual_clear_chat';
    statefulLastInputTokens = 0;
    statefulLastOutputTokens = 0;
    statefulPeakInputTokens = 0;
    messages = [];
    pendingScrollToBottom = false;
    chatMessages.innerHTML = '';
}

function clearContext() {
    lastResponseId = null;
    statefulTurnCount = 0;
    statefulEstimatedChars = statefulSummary.length;
    pendingStatefulResetReason = 'manual_context_reset';
    statefulLastInputTokens = 0;
    statefulLastOutputTokens = 0;
    showAlert(t('setting.memory.reset.success') + ' (Context/Session ID Cleared)');
    console.log('[Context] Manual context reset trigger.');
}

function buildStatefulSystemPrompt() {
    let prompt = config.systemPrompt || 'You are a helpful AI assistant.';
    if (statefulSummary) {
        prompt += `\n\n### Conversation Summary ###\n${statefulSummary}\n\nUse this summary as compressed context from earlier turns.`;
    }
    return prompt;
}

function cleanContentForStatefulSummary(text) {
    if (!text) return '';
    return String(text)
        .replace(/<think>[\s\S]*?<\/think>/g, '')
        .replace(/\s+/g, ' ')
        .trim();
}

function summarizeMessagesForStatefulReset() {
    const recent = messages.slice(-12);
    const lines = recent.map((m, idx) => {
        const label = m.role === 'assistant' ? 'Assistant' : 'User';
        let content = cleanContentForStatefulSummary(m.content || '');
        if (content.length > 320) {
            content = content.slice(0, 320) + '...';
        }
        if (m.image) {
            content = `[Image Attached] ${content}`.trim();
        }
        return `${idx + 1}. ${label}: ${content}`;
    }).filter(Boolean);

    let summary = lines.join('\n');
    if (statefulSummary) {
        summary = `Previous summary:\n${statefulSummary}\n\nRecent turns:\n${summary}`;
    }

    const maxLen = 1800;
    if (summary.length > maxLen) {
        summary = summary.slice(summary.length - maxLen);
    }
    return summary.trim();
}

function estimateTokensFromText(text) {
    if (!text) return 0;
    return Math.ceil(String(text).trim().length / 3.5);
}

function getStatefulRiskMetrics(nextUserText = '') {
    const limitTurns = parseInt(config.statefulTurnLimit, 10) || 8;
    const charBudget = parseInt(config.statefulCharBudget, 10) || 12000;
    const tokenBudget = parseInt(config.statefulTokenBudget, 10) || 10000;
    const projectedChars = statefulEstimatedChars + (nextUserText ? nextUserText.length : 0);
    const projectedTokens = statefulLastInputTokens + estimateTokensFromText(nextUserText);
    const turnFactor = Math.min(1, statefulTurnCount / Math.max(limitTurns, 1));
    const charFactor = Math.min(1, projectedChars / Math.max(charBudget, 1));
    const tokenFactor = Math.min(1, projectedTokens / Math.max(tokenBudget, 1));
    const score = Math.round((turnFactor * 20 + charFactor * 15 + tokenFactor * 65) * 100) / 100;

    let level = 'low';
    if (score >= 0.9) level = 'critical';
    else if (score >= 0.7) level = 'high';
    else if (score >= 0.45) level = 'medium';

    return {
        score,
        level,
        projectedChars,
        projectedTokens,
        turnLimit: limitTurns,
        charBudget,
        tokenBudget
    };
}

function shouldResetStatefulContext(nextUserText = '') {
    if (config.llmMode !== 'stateful' || config.disableStateful) {
        return false;
    }
    const risk = getStatefulRiskMetrics(nextUserText);
    return statefulTurnCount >= risk.turnLimit ||
        risk.projectedChars >= risk.charBudget ||
        risk.projectedTokens >= risk.tokenBudget ||
        statefulLastInputTokens >= risk.tokenBudget;
}

async function ensureStatefulContextBudget(nextUserText = '') {
    if (!shouldResetStatefulContext(nextUserText)) {
        return;
    }

    statefulSummary = summarizeMessagesForStatefulReset();
    lastResponseId = null;
    statefulTurnCount = 0;
    statefulEstimatedChars = statefulSummary.length;
    statefulResetCount += 1;
    pendingStatefulResetReason = 'auto_summary_reset';
    statefulLastInputTokens = estimateTokensFromText(statefulSummary);
    statefulLastOutputTokens = 0;
    appendMessage({
        role: 'system',
        content: `Stateful context compacted ${statefulPeakInputTokens || 0} -> ~${statefulLastInputTokens}`
    });
}


/* Chat Logic */

async function sendMessage() {
    // Unlock audio context on user interaction
    unlockAudioContext();

    let text = messageInput.value.trim();
    const currentImage = pendingImage; // Capture early

    if (!text && !currentImage) return;
    if (isGenerating) return;

    if (config.llmMode === 'stateful') {
        await ensureStatefulContextBudget(text);
    }

    dismissStartupCards();

    // Stop and clear any existing audio/TTS
    stopAllAudio();

    // Prepare User Message
    const userMsg = {
        role: 'user',
        content: text,
        image: currentImage
    };

    appendMessage(userMsg);
    messages.push(userMsg);
    if (config.llmMode === 'stateful') {
        statefulEstimatedChars += text.length;
    }

    // Reset Input
    messageInput.value = '';
    removeImage();
    autoResizeInput();

    // Prepare Assistant Placeholder
    isGenerating = true;
    lockScrollToLatest = true;
    updateSendButtonState();

    // Create new AbortController
    abortController = new AbortController();

    const assistantId = 'msg-' + Date.now();
    activeStreamingMessageId = assistantId;
    startStreamingMessageAutoScroll(assistantId);

    // Build API Payload
    // Always start with a system prompt to define behavior and anchor the context
    const systemMsg = { role: 'system', content: config.systemPrompt };

    // Trim old messages if history exceeds limit (user+assistant pairs)
    const maxMessages = (parseInt(config.historyCount) || 10) * 2;
    if (messages.length > maxMessages) {
        // Remove oldest messages, keeping recent ones
        messages = messages.slice(-maxMessages);
    }

    let payload = {};

    if (config.llmMode === 'stateful') {
        // Stateful Chat Mode
        // LM Studio Stateful API (Experimental) uses a specific multimodal format:
        // { type: 'text', content: '...' } and { type: 'image', data_url: '...' }
        let inputData = text;
        if (currentImage) {
            inputData = [];
            if (text) {
                inputData.push({ type: 'text', content: text });
            }
            inputData.push({ type: 'image', data_url: currentImage });
        }

        payload = {
            model: config.model,
            input: inputData,
            system_prompt: buildStatefulSystemPrompt(),
            temperature: config.temperature,
            stream: true
        };

        if (config.disableStateful) {
            payload.store = false;
        }

        if (lastResponseId) {
            payload.previous_response_id = lastResponseId;
        }
    } else {
        // Standard Stateless Mode (Default)
        const payloadHistory = messages.map(m => {
            if (m.image) {
                // Vision format
                const visionContent = [];
                if (m.content) {
                    visionContent.push({ type: 'text', text: m.content });
                }
                visionContent.push({ type: 'image_url', image_url: { url: m.image } });
                return {
                    role: m.role,
                    content: visionContent
                };
            } else {
                // Clean content for history
                let content = m.content || '';
                if (m.role === 'assistant') {
                    // Remove think tags from history to prevent recursion/confusion
                    content = content.replace(/<think>[\s\S]*?<\/think>/g, '').trim();
                }
                return { role: m.role, content: content };
            }
        });

        payload = {
            model: config.model,
            messages: [systemMsg, ...payloadHistory],
            temperature: config.temperature,
            max_tokens: config.maxTokens,
            stream: true
        };
    }

    // Debug: Log payload to verify what's being sent
    console.log('=== LLM Request Payload ===');
    console.log('System Prompt:', systemMsg.content);
    console.log('History Count:', history.length);
    console.log('Messages:', JSON.stringify(payload.messages, null, 2));

    let assistantContent = '';
    try {
        assistantContent = await streamResponse(payload, assistantId);
    } catch (e) {
        if (e.name === 'AbortError') {
            finalizeAssistantStatusCards(assistantId, 'stopped', t('status.stopped'));
            updateMessageContent(assistantId, `**[Stopped by User]**`);
        } else {
            finalizeAssistantStatusCards(assistantId, 'failed', e.message || t('status.failed'));
            updateMessageContent(assistantId, `**Error:** ${e.message}`);
        }
    } finally {
        isGenerating = false;
        lockScrollToLatest = false;
        stopStreamingMessageAutoScroll();
        activeStreamingMessageId = null;
        abortController = null;
        updateSendButtonState();
    }

    if (assistantContent && assistantContent.trim()) {
        await saveLastSessionTurn(userMsg, assistantContent);
    }
}

function stopGeneration() {
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
    hideProgressDock();
    // Stop any currently playing audio/TTS
    stopAllAudio();
}

function updateSendButtonState() {
    if (isGenerating) {
        sendBtn.disabled = false; // Enabled so we can Click to Stop
        sendBtn.innerHTML = '<span class="material-icons-round">stop</span>';
        sendBtn.title = "Stop Generation";
        sendBtn.classList.add('stop-btn');
    } else {
        sendBtn.disabled = false;
        sendBtn.innerHTML = '<span class="material-icons-round">send</span>';
        sendBtn.title = "Send Message";
        sendBtn.classList.remove('stop-btn');
    }

    // Also update giant mic icon if layout is active
    updateMicUIForGeneration(isGenerating);
    updateInlineComposerActionVisibility();
}

function hasComposableUserInput() {
    const text = messageInput?.value?.trim() || '';
    return !!text || !!pendingImage;
}

function setComposerPrimaryButtons({ showSend, showInlineMic }) {
    if (sendBtn) {
        sendBtn.hidden = !showSend;
        sendBtn.style.display = showSend ? 'inline-flex' : 'none';
    }
    if (inlineMicBtn) {
        inlineMicBtn.hidden = !showInlineMic;
        inlineMicBtn.style.display = showInlineMic ? 'inline-flex' : 'none';
    }
}

function updateInlineComposerActionVisibility() {
    if (!sendBtn || !inlineMicBtn) return;

    const inlineMicAvailable = inlineMicBtn.classList.contains('is-visible');
    if (!inlineMicAvailable) {
        setComposerPrimaryButtons({ showSend: true, showInlineMic: false });
        return;
    }

    if (isGenerating) {
        setComposerPrimaryButtons({ showSend: true, showInlineMic: false });
        return;
    }

    const shouldShowSend = hasComposableUserInput();
    setComposerPrimaryButtons({ showSend: shouldShowSend, showInlineMic: !shouldShowSend });
}


async function streamResponse(payload, elementId) {
    const headers = { 'Content-Type': 'application/json' };
    if (currentUserLocation) {
        headers['X-User-Location'] = currentUserLocation;
    }
    const statefulRisk = getStatefulRiskMetrics(typeof payload.input === 'string' ? payload.input : '');
    headers['X-Stateful-Turn-Count'] = String(statefulTurnCount);
    headers['X-Stateful-Est-Chars'] = String(statefulEstimatedChars);
    headers['X-Stateful-Summary-Chars'] = String(statefulSummary.length);
    headers['X-Stateful-Reset-Count'] = String(statefulResetCount);
    headers['X-Stateful-Input-Tokens'] = String(statefulLastInputTokens);
    headers['X-Stateful-Peak-Input-Tokens'] = String(statefulPeakInputTokens);
    headers['X-Stateful-Token-Budget'] = String(parseInt(config.statefulTokenBudget, 10) || 10000);
    headers['X-Stateful-Risk-Score'] = String(statefulRisk.score);
    headers['X-Stateful-Risk-Level'] = statefulRisk.level;
    if (pendingStatefulResetReason) {
        headers['X-Stateful-Reset-Reason'] = pendingStatefulResetReason;
    }

    // Use the Go server's API endpoint
    const response = await fetch('/api/chat', {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload),
        signal: abortController.signal
    });

    if (!response.ok) {
        let errorDetails = `Server Error ${response.status}: ${response.statusText}`;
        const errorBody = await response.text();
        if (errorBody) {
            // Check for localized auth error
            if (errorBody.startsWith("LM_STUDIO_AUTH_ERROR: ")) {
                const originalMsg = errorBody.replace("LM_STUDIO_AUTH_ERROR: ", "");
                // Translate using i18n key 'error.authFailed'
                // t() function is available globally
                const localizedMsg = t('error.authFailed');
                errorDetails = localizedMsg + originalMsg;

                // Throwing here will be caught by sendMessage catch block
                throw new Error(errorDetails);
            }

            // Check for localized MCP permission error
            if (errorBody.startsWith("LM_STUDIO_MCP_ERROR: ")) {
                const originalMsg = errorBody.replace("LM_STUDIO_MCP_ERROR: ", "");
                const localizedMsg = t('error.mcpFailed');
                errorDetails = localizedMsg + originalMsg;
                throw new Error(errorDetails);
            }

            // Check for Context Overflow error (from non-200 check)
            if (errorBody.startsWith("LM_STUDIO_CONTEXT_ERROR: ")) {
                // "LM_STUDIO_CONTEXT_ERROR: Context size exceeded."
                // Use localized message
                errorDetails = t('error.contextExceeded');
                errorDetails = t('error.contextExceeded');
                throw new Error(errorDetails);
            }

            // Check for Vision Support Error
            if (errorBody.startsWith("LM_STUDIO_VISION_ERROR: ")) {
                errorDetails = t('error.visionNotSupported');
                throw new Error(errorDetails);
            }

            if (errorBody.includes("Could not find stored response for previous_response_id")) {
                console.warn("[Stateful] previous_response_id became invalid. Resetting and retrying without it...");
                lastResponseId = null;
                statefulTurnCount = 0;
                statefulEstimatedChars = statefulSummary.length;
                statefulLastInputTokens = estimateTokensFromText(statefulSummary);
                statefulLastOutputTokens = 0;
                statefulResetCount += 1;
                pendingStatefulResetReason = 'invalid_previous_response_id';
                // Re-attempt without current lastResponseId
                delete payload.previous_response_id;
                return await streamResponse(payload, elementId);
            }

            errorDetails += ` - ${errorBody}`;

        }
        throw new Error(errorDetails);
    }
    pendingStatefulResetReason = null;

    return await processStream(response, elementId);
}

// Helper to process the stream reader (shared by direct and proxy)
async function processStream(response, elementId) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let fullText = '';           // Content to display (no reasoning)
    let loopDetected = false;    // Loop detection state
    let reasoningBuffer = '';     // Separate buffer for reasoning content (for history only)
    let speechBuffer = '';        // Dedicated buffer for speech content (no HTML/Tools)
    let currentlyReasoning = false; // State track for reasoning blocks
    let reasoningSource = null;    // 'sse' or 'field' to prevent duplication
    let historyContent = '';
    let lastToolCallHtml = '';
    let streamAborted = false;




    // Initialize streaming TTS if enabled
    const useStreamingTTS = config.enableTTS && config.autoTTS;
    if (useStreamingTTS) {
        initStreamingTTS(elementId);
        requestWakeLock(); // Request wake lock when TTS streaming starts
    }

    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });

            const lines = buffer.split('\n\n');
            buffer = lines.pop();

            for (const line of lines) {
                const trimmed = line.trim();
                if (!trimmed) continue;

                let json = null;
                try {
                    if (trimmed.startsWith('data: ')) {
                        const dataStr = trimmed.substring(6);
                        if (dataStr === '[DONE]') break;
                        json = JSON.parse(dataStr);
                    } else if (trimmed.startsWith('{')) {
                        // Handle raw JSON (non-streaming or Stateful response)
                        json = JSON.parse(trimmed);
                    } else {
                        continue;
                    }

                    // DEBUG: Log all event types
                    // console.log('[SSE Event]', json.type); 

                    // Capture response_id if present (Stateful Chat)
                    if (json.response_id) {
                        lastResponseId = json.response_id;
                        console.log(`[Stateful] Captured response_id: ${lastResponseId}`);
                    }

                    // Check for explicit error in stream (Context Overflow etc)
                    if (json.error) {
                        let errorMsg = json.error;
                        if (errorMsg.startsWith("LM_STUDIO_CONTEXT_ERROR: ")) {
                            errorMsg = t('error.contextExceeded');
                        }
                        // Throw to stop generation and show error in bubble
                        throw new Error(errorMsg);
                    }

                    let contentToAdd = '';
                    let speechToAdd = ''; // Content that should be spoken

                    // Handle Standard/SSE format
                    if (json.choices && json.choices.length > 0) {
                        const delta = json.choices[0].delta || {};
                        const message = json.choices[0].message || {}; // Non-streaming fallback

                        // Support for OpenAI-style reasoning_content - store in reasoningBuffer, not contentToAdd
                        if (delta.reasoning_content) {
                            if (!currentlyReasoning) {
                                reasoningBuffer += '<think>';
                                currentlyReasoning = true;
                                reasoningSource = 'field';
                                showReasoningStatus(elementId, '...'); // Start status
                            }
                            // Prioritize SSE if both present (LM Studio)
                            if (reasoningSource !== 'sse') {
                                reasoningBuffer += delta.reasoning_content;
                                showReasoningStatus(elementId, reasoningBuffer); // Update status with full buffer
                            }
                        }

                        const part = delta.content || message.content || '';

                        // Auto-close reasoning block if we transition to normal content
                        if (part && currentlyReasoning && !delta.reasoning_content) {
                            // If we see actual content and we were in reasoning, close the block
                            reasoningBuffer += '</think>\n';
                            currentlyReasoning = false;
                            reasoningSource = null;
                            showReasoningStatus(elementId, null, true); // Remove status
                        }

                        if (!currentlyReasoning) {
                            contentToAdd += part;
                            speechToAdd += part;
                        }

                    }


                    // Handle Stateful Chat JSON format (output array mechanism - legacy/alternative?)
                    else if (json.output && Array.isArray(json.output)) {
                        for (const item of json.output) {
                            // Skip reasoning type items - they should not be displayed in bubble
                            if (item.type === 'reasoning') continue;

                            if (item.content && item.type === 'message') {
                                contentToAdd += item.content;
                                if (!currentlyReasoning) {
                                    speechToAdd += item.content;
                                }
                            }
                        }
                    }

                    // Handle LM Studio Stateful Chat Streaming Format (based on logs)
                    else if (json.type === 'message.delta' && json.content) {
                        contentToAdd = json.content;
                        if (!currentlyReasoning) {
                            speechToAdd = json.content;
                        }
                    }

                    // Handle Reasoning (Thinking) - Status indicator only, NO display in bubble
                    else if (json.type === 'reasoning.start') {
                        if (!currentlyReasoning) {
                            reasoningBuffer += '<think>';
                            currentlyReasoning = true;
                        }
                        reasoningSource = 'sse';
                        showReasoningStatus(elementId, '...'); // Start status
                    }
                    else if (json.type === 'reasoning.delta' && json.content) {
                        // Add to reasoning buffer, NOT to contentToAdd/fullText
                        reasoningBuffer += json.content;
                        currentlyReasoning = true;
                        reasoningSource = 'sse';
                        showReasoningStatus(elementId, reasoningBuffer); // Update with full buffer
                    }
                    else if (json.type === 'reasoning.end') {
                        reasoningBuffer += '</think>\n';
                        currentlyReasoning = false;
                        reasoningSource = null;
                        showReasoningStatus(elementId, null, true); // Remove status
                    }




                    // Handle MCP Tool Calls - Display only, NO SPEECH
                    else if (json.type === 'tool_call.start') {
                        const toolName = json.tool || 'Tool';
                        lastToolCallHtml = toolName;
                        setToolCardState(elementId, 'running', '', null, toolName);
                    }
                    else if (json.type === 'tool_call.arguments' && json.arguments) {
                        const toolName = json.tool || 'Tool';
                        setToolCardState(elementId, 'running', '', json.arguments, toolName);
                    }
                    else if (json.type === 'tool_call.success') {
                        setToolCardState(elementId, 'success', t('tool.executionFinished'));
                    }
                    else if (json.type === 'tool_call.failure') {
                        setToolCardState(elementId, 'failure', json.reason || t('tool.unknownError'));
                    }
                    else if (json.type === 'chat.end' && json.result) {
                        hideProgressDock();
                        if (json.result.response_id) {
                            lastResponseId = json.result.response_id;
                            console.log(`[Stateful] Captured response_id from chat.end: ${lastResponseId}`);
                        }
                        const stats = json.result.stats || {};
                        if (typeof stats.input_tokens === 'number' && Number.isFinite(stats.input_tokens)) {
                            statefulLastInputTokens = stats.input_tokens;
                            statefulPeakInputTokens = Math.max(statefulPeakInputTokens, stats.input_tokens);
                            console.log(`[Stateful] Captured input_tokens: ${statefulLastInputTokens}`);
                        }
                        if (typeof stats.total_output_tokens === 'number' && Number.isFinite(stats.total_output_tokens)) {
                            statefulLastOutputTokens = stats.total_output_tokens;
                        }
                    }
                    // Handle Prompt Processing Progress
                    else if (json.type === 'prompt_processing.progress') {
                        renderProgressDock(t('progress.processingPrompt'), json.progress * 100, 'prompt-processing', false);
                    }
                    // Handle Model Loading Progress (LM Studio Mode)
                    else if (json.type === 'model_load.start') {
                        console.log('[Model Load] Start:', json.model_instance_id);
                        renderProgressDock(t('progress.loadingModel'), null, 'model-loading', true);
                    }
                    else if (json.type === 'model_load.progress') {
                        renderProgressDock(t('progress.loadingModel'), json.progress * 100, 'model-loading', false);
                    }
                    else if (json.type === 'model_load.end') {
                        console.log('[Model Load] End:', json.model_instance_id, 'Time:', json.load_time_seconds);
                        renderProgressDock(`${t('progress.modelLoaded')} (${json.load_time_seconds?.toFixed(1) || '?'}s)`, 100, 'model-loading', false);
                        setTimeout(() => hideProgressDock(), 1200);
                    }
                    // Handle Generative Errors (Tool Parsing, etc.)
                    else if (json.type === 'error') {
                        console.error('[SSE Error]', json.error);
                        hideProgressDock();
                        let errMsg = t('tool.unknownError');
                        if (json.error && json.error.message) {
                            errMsg = json.error.message;
                        } else if (typeof json.error === 'string') {
                            errMsg = json.error;
                        }

                        if (lastToolCallHtml) {
                            setToolCardState(elementId, 'failure', errMsg);
                        } else {
                            contentToAdd = `\n\n**Error:** ${errMsg}\n`;
                        }
                    }

                    if (contentToAdd) {
                        hideProgressDock();

                        fullText += contentToAdd;

                        // --- LOOP DETECTION (Regex-based) ---
                        // --- LOOP DETECTION (Regex-based) ---
                        // Relaxed logic per user request (Step Id: 4593)
                        // 1. Increased thresholds (Short: 5->10, Long: 3->6)
                        // 2. Explicitly ignore Tool Call patterns (Agents loops are handled by backend)
                        if (!loopDetected && fullText.length >= 100) {
                            // Pattern: 5+ chars repeated 10+ times consecutively (was 5)
                            const shortLoopMatch = fullText.match(/([\s\S]{5,}?)\1{9,}/);
                            // Pattern: 50+ chars repeated 6+ times consecutively (was 3)
                            const longLoopMatch = fullText.match(/([\s\S]{50,}?)\1{5,}/);
                            const loopMatch = shortLoopMatch || longLoopMatch;

                            if (loopMatch && loopMatch[1].length >= 4) {
                                // Exclude Tool Call logs from loop detection (False positives common in agents)
                                const isToolLog = loopMatch[1].includes("Tool Call") ||
                                    loopMatch[1].includes("Tool Finished") ||
                                    loopMatch[1].includes("🛠️") ||
                                    loopMatch[1].includes("✅");

                                if (!isToolLog) {
                                    console.warn(`[Loop Detection] Pattern detected: "${loopMatch[1].substring(0, 30)}..." repeated ${Math.floor(loopMatch[0].length / loopMatch[1].length)}+ times`);
                                    loopDetected = true;
                                    stopGeneration();

                                    fullText += "\n\n" + (typeof t === 'function' ? t('warning.loopDetected') : "⚠️ Loop detected. Generation stopped.");
                                } else {
                                    console.log(`[Loop Detection] Ignored tool pattern: "${loopMatch[1].substring(0, 20)}..."`);
                                }
                            }
                        }
                        // --- END LOOP DETECTION ---
                        // --- END LOOP DETECTION ---

                        let displayText = fullText;

                        // LM Studio Reasoning Status Detection (Text-based fallback for tags)
                        // Only run if not already handled by SSE/reasoning_content events
                        if (config.llmMode === 'lm-studio' && !currentlyReasoning && !config.hideThink) {
                            const hasAnalysis = displayText.includes('<|channel|>analysis');

                            const hasFinal = displayText.includes('<|channel|>final');
                            const hasThink = displayText.includes('<think>');
                            const hasThinkEnd = displayText.includes('</think>');

                            if ((hasAnalysis && !hasFinal) || (hasThink && !hasThinkEnd)) {
                                // Extract the "new" content part for status update
                                // This is tricky during streaming, but we can show the last part of fullText
                                let statusText = "Thinking...";
                                if (hasAnalysis) {
                                    const parts = fullText.split('<|channel|>analysis');
                                    statusText = parts[parts.length - 1].split('<|channel|>')[0].trim();
                                } else if (hasThink) {
                                    const parts = fullText.split('<think>');
                                    statusText = parts[parts.length - 1].split('</think>')[0].trim();
                                }

                                // Limit status text length for display
                                if (statusText.length > 150) statusText = "..." + statusText.slice(-147);
                                showReasoningStatus(elementId, statusText, false);
                            } else if (hasFinal || hasThinkEnd) {
                                showReasoningStatus(elementId, null, true);
                            }
                        }

                        // UI Display Logic - ALWAYS hide <think> content from bubble
                        // (Status indicator shows thinking progress instead)
                        // 1. Remove complete <think>...</think> blocks
                        displayText = displayText.replace(/<think>[\s\S]*?<\/think>/g, '');
                        // 2. Remove complete <|channel|>analysis...<|channel|> blocks
                        displayText = displayText.replace(/<\|channel\|>analysis[\s\S]*?(?=<\|channel\|>final|$)/g, '');
                        // 3. Remove standalone tags
                        displayText = displayText.replace(/<\|channel\|>(analysis|final|message)/g, '');
                        displayText = displayText.replace(/<\|end\|>/g, '');

                        // 4. Remove text-based tool call JSON patterns (for models without native function calling)
                        // Pattern 1: {"name": "tool_name", "arguments": {...}}
                        // Pattern 2: {"name": "tool_name", "query": "..."}  (alternative format)
                        displayText = displayText.replace(/\{"name"\s*:\s*"[^"]+"\s*,\s*"arguments"\s*:\s*\{[^}]*\}\}/g, '');
                        displayText = displayText.replace(/\{"name"\s*:\s*"[^"]+"[^}]*\}/g, '');


                        // Handle case where </think> exists without opening tag (remove everything before it)
                        if (displayText.includes('</think>')) {
                            displayText = displayText.split('</think>').pop().trim();
                        }
                        // Handle incomplete <think> tag (still being streamed)
                        if (displayText.includes('<think>')) {
                            displayText = displayText.split('<think>')[0];
                        }

                        updateMessageContent(elementId, displayText);

                    }

                    // Separate TTS Logic using speechBuffer
                    if (useStreamingTTS && speechToAdd) {
                        // Ultimate safety: Ensure no <think> or channel tags ever reach TTS
                        let cleanedSpeech = speechToAdd;
                        if (cleanedSpeech.includes('<think>') || cleanedSpeech.includes('<|channel|>')) {
                            cleanedSpeech = cleanedSpeech.replace(/<think>[\s\S]*?<\/think>/g, '');
                            cleanedSpeech = cleanedSpeech.replace(/<\|channel\|>analysis[\s\S]*?(?=<\|channel\|>final|$)/g, '');
                            cleanedSpeech = cleanedSpeech.replace(/<\|channel\|>(analysis|final|message)/g, '');
                            cleanedSpeech = cleanedSpeech.replace(/<\|end\|>/g, '');
                            // Remove standalone partial tags
                            cleanedSpeech = cleanedSpeech.replace(/<think>/g, '').replace(/<\/think>/g, '');
                        }

                        if (cleanedSpeech) {
                            speechBuffer += cleanedSpeech;
                            feedStreamingTTS(speechBuffer);
                        }
                    }

                } catch (e) {
                    console.error('JSON Parse Error', e);
                }
            }
        }
    } catch (err) {
        if (err.name === 'AbortError') {
            streamAborted = true;
            console.log('Stream aborted by user');
        } else {
            console.error('Stream Error:', err);
            throw err; // Re-throw other errors
        }
    } finally {
        hideProgressDock();
        if (!streamAborted) {
            if (currentlyReasoning) {
                finalizeReasoningStatus(elementId, 'failed', t('status.unexpectedStop'));
            }
            if (getRunningToolCards(elementId).length > 0) {
                finalizeAssistantStatusCards(elementId, 'failed', t('status.failed'));
            }
        }
        // Finalize (Save to history even if aborted)
        // Keep only the user-visible answer in history to avoid ballooning context.
        historyContent = fullText.trim();
        if (historyContent) {
            messages.push({ role: 'assistant', content: historyContent });
            if (config.llmMode === 'stateful') {
                statefulTurnCount += 1;
                statefulEstimatedChars += historyContent.length;
            }
        }


        // Finalize streaming TTS (commit any remaining text)
        if (useStreamingTTS) {
            finalizeStreamingTTS(speechBuffer); // Pass final speech buffer
        }
        if (historyContent) {
            setAssistantActionBarReady(elementId);
        }
        releaseWakeLock(); // Release screen lock after generation and TTS streaming is done
    }

    return historyContent;
}

function createMessageElement(msg) {
    const div = document.createElement('div');
    div.className = `message ${msg.role}`;
    if (msg.id) div.id = msg.id;

    const textContent = msg.content || '';
    if (msg.role === 'user') {
        div.innerHTML = `
            <div class="message-inner">
                <div class="message-label">You</div>
                ${msg.image ? `<img src="${msg.image}" class="message-image">` : ''}
                ${textContent ? `<div class="message-bubble">${escapeHtml(textContent)}</div>` : ''}
            </div>`;
    } else if (msg.role === 'system') {
        div.innerHTML = `
            <div class="message-inner">
                <div class="message-label">System</div>
                <div class="assistant-sections">
                    <section class="system-strip-card">
                        <div class="reasoning-header system-strip-header">
                            <span class="reasoning-chevron material-icons-round">info</span>
                            <span class="reasoning-title">${escapeHtml(textContent)}</span>
                        </div>
                    </section>
                </div>
            </div>`;
    } else if (msg.startup) {
        div.classList.add('has-startup-card');
        const startup = msg.startup;
        const issues = Array.isArray(startup.issues) ? startup.issues : [];
        const issuesHtml = issues.length > 0
            ? `<ul class="startup-issues">${issues.map(issue => `<li>${escapeHtml(issue)}</li>`).join('')}</ul>`
            : '';
        const actionHtml = startup.showRestoreButton
            ? `<div class="startup-actions"><button class="startup-action-btn" onclick="restoreLastSession()">${escapeHtml(startup.restoreLabel || t('chat.startup.restore'))}</button></div>`
            : '';

        div.innerHTML = `
            <div class="message-inner">
                <div class="message-label">Assistant</div>
                <div class="assistant-sections">
                    <section class="assistant-response-card startup-response-card">
                        <div class="message-bubble plain-assistant-bubble">
                            <div class="startup-card">
                                <div class="startup-title">${escapeHtml(startup.title || '')}</div>
                                <div class="startup-body">${escapeHtml(startup.body || '')}</div>
                                ${issuesHtml}
                                ${actionHtml}
                            </div>
                        </div>
                    </section>
                </div>
            </div>`;
    } else {
        const assistantMarkdown = renderInitialAssistantMarkdown(textContent);
        div.innerHTML = `
            <div class="message-inner">
                <div class="message-label">Assistant</div>
                <div class="assistant-sections">
                    <div class="assistant-reasoning"></div>
                    <div class="assistant-tools"></div>
                    <section class="assistant-response-card" ${textContent.trim() ? '' : 'hidden'}>
                        <div class="message-bubble plain-assistant-bubble">
                            ${msg.image ? `<img src="${msg.image}" class="message-image">` : ''}
                            <div class="markdown-body">${assistantMarkdown}</div>
                        </div>
                    </section>
                </div>
                <div class="message-actions${textContent.trim() ? ' is-ready' : ' is-pending'}" ${textContent.trim() ? '' : 'hidden'}>
                    <button class="icon-btn copy-btn" onclick="copyMessage(this)" title="Copy">
                        <span class="material-icons-round">content_copy</span>
                    </button>
                    <button class="icon-btn speak-btn" onclick="speakMessageFromBtn(this)" title="Speak">
                        <span class="material-icons-round">volume_up</span>
                    </button>
                </div>
            </div>`;
    }

    return div;
}

function appendMessage(msg) {
    const wasNearBottom = isChatNearBottom();
    const div = createMessageElement(msg);
    chatMessages.appendChild(div);
    scrollToBottom(wasNearBottom || msg.role === 'user');
}

function dismissStartupCards() {
    const startupMessages = Array.from(document.querySelectorAll('.message.has-startup-card'));
    startupMessages.forEach((msgEl) => {
        if (msgEl.classList.contains('is-dismissing')) return;
        msgEl.classList.add('is-dismissing');
        window.setTimeout(() => {
            if (msgEl.parentNode) {
                msgEl.remove();
            }
        }, 320);
    });
}

function formatThoughtDuration(durationMs = 0) {
    const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
    if (totalSeconds < 60) {
        return t('status.thoughtForSeconds')
            .replace('{seconds}', String(totalSeconds));
    }
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    if (seconds === 0) {
        return t('status.thoughtForMinutes')
            .replace('{minutes}', String(minutes));
    }
    return t('status.thoughtForMinutesSeconds')
        .replace('{minutes}', String(minutes))
        .replace('{seconds}', String(seconds));
}

function ensureAssistantMessageElement(id) {
    let el = document.getElementById(id);
    if (el) return el;
    appendMessage({ role: 'assistant', content: '', id });
    return document.getElementById(id);
}

function getAssistantMessageParts(elementId) {
    const msgEl = ensureAssistantMessageElement(elementId);
    if (!msgEl) return {};
    return {
        msgEl,
        reasoningHost: msgEl.querySelector('.assistant-reasoning'),
        toolsHost: msgEl.querySelector('.assistant-tools'),
        bubble: msgEl.querySelector('.assistant-response-card .message-bubble'),
        markdownBody: msgEl.querySelector('.assistant-response-card .markdown-body'),
        committedHost: msgEl.querySelector('.assistant-response-card .markdown-committed'),
        pendingHost: msgEl.querySelector('.assistant-response-card .markdown-pending')
    };
}

function setAssistantActionBarReady(elementId) {
    const { msgEl } = getAssistantMessageParts(elementId);
    const actionBar = msgEl?.querySelector('.message-actions');
    if (!actionBar) return;
    actionBar.hidden = false;
    actionBar.classList.remove('is-pending');
    requestAnimationFrame(() => {
        actionBar.classList.add('is-ready');
    });
}

function renderProgressDock(label, percent = null, mode = 'prompt-processing', indeterminate = false) {
    if (!chatProgressDock) return;
    if (progressDockHideTimer) {
        clearTimeout(progressDockHideTimer);
        progressDockHideTimer = null;
    }
    const clamped = typeof percent === 'number' ? Math.max(0, Math.min(100, percent)) : null;
    const cardClass = `llm-progress-card ${mode}${indeterminate ? ' indeterminate' : ''}`;
    const percentLabel = clamped === null ? '' : `${clamped.toFixed(2)}%`;
    const width = indeterminate ? '32%' : `${clamped || 0}%`;
    composerProgressLabel = label || '';
    composerProgressActive = true;
    composerProgressPercent = percentLabel;
    updateMessageInputPlaceholder();
    inputContainer?.classList.add('has-progress');

    const wasHidden = chatProgressDock.hidden;
    chatProgressDock.hidden = false;
    chatProgressDock.innerHTML = `
        <div class="${cardClass}">
            <div class="llm-progress-track">
                <div class="llm-progress-fill" style="width: ${width};"></div>
            </div>
        </div>`;
    if (wasHidden) {
        requestAnimationFrame(() => {
            if (!chatProgressDock.hidden) {
                chatProgressDock.classList.add('is-visible');
            }
        });
    } else {
        chatProgressDock.classList.add('is-visible');
    }
}

function hideProgressDock() {
    if (!chatProgressDock) return;
    chatProgressDock.classList.remove('is-visible');
    if (progressDockHideTimer) {
        clearTimeout(progressDockHideTimer);
    }
    progressDockHideTimer = setTimeout(() => {
        chatProgressDock.hidden = true;
        chatProgressDock.innerHTML = '';
        composerProgressLabel = '';
        composerProgressActive = false;
        composerProgressPercent = null;
        inputContainer?.classList.remove('has-progress');
        updateMessageInputPlaceholder();
        progressDockHideTimer = null;
    }, 180);
}

function isToolExecutionFinishedSummary(text = '') {
    const normalized = String(text || '').trim().toLowerCase();
    if (!normalized) return false;
    return normalized === translations.ko['tool.executionFinished'].toLowerCase()
        || normalized === translations.en['tool.executionFinished'].toLowerCase()
        || normalized === 'tool execution finished'
        || normalized === 'tool execution finished.';
}

function getRunningToolCards(elementId) {
    const { toolsHost } = getAssistantMessageParts(elementId);
    if (!toolsHost) return [];
    return Array.from(toolsHost.querySelectorAll('.tool-status-card.is-running'));
}

function finalizeToolCard(card, outcome = 'done', detail = '') {
    if (!card) return;

    const headerGroupEl = card.querySelector('.tool-header-group');
    const summaryEl = card.querySelector('.tool-header-status');
    const queryEl = card.querySelector('.tool-card-query');
    const historyEl = card.querySelector('.tool-card-history');

    card.classList.remove('is-running', 'is-success', 'is-failure');
    if (outcome === 'failed') {
        card.classList.add('is-failure');
    } else {
        card.classList.add('is-success');
    }
    const keepExpanded = card.dataset.userExpanded === 'true';
    if (keepExpanded) {
        card.dataset.collapsed = 'false';
        card.classList.remove('collapsed');
    } else {
        card.dataset.collapsed = 'true';
        card.classList.add('collapsed');
    }

    if (headerGroupEl) {
        headerGroupEl.classList.remove('is-live');
    }
    if (summaryEl) {
        if (outcome === 'failed') summaryEl.textContent = detail || t('status.failed');
        else if (outcome === 'stopped') summaryEl.textContent = detail || t('status.stopped');
        else summaryEl.textContent = detail || t('status.done');
        summaryEl.classList.remove('is-query-preview');
        summaryEl.title = summaryEl.textContent;
    }
    if (queryEl) {
        queryEl.hidden = true;
    }
    renderToolHistory(card, historyEl, outcome === 'failed' ? 'failure' : 'success');
}

function ensureReasoningCard(elementId) {
    const { reasoningHost } = getAssistantMessageParts(elementId);
    if (!reasoningHost) return null;

    let card = reasoningHost.querySelector('.reasoning-status');
    if (!card) {
        card = document.createElement('section');
        card.className = 'reasoning-status';
        card.dataset.collapsed = 'true';
        card.dataset.startedAt = String(Date.now());
        card.innerHTML = `
            <button type="button" class="reasoning-header" onclick="toggleReasoningCard(this)">
                <span class="reasoning-chevron material-icons-round">play_arrow</span>
                <span class="reasoning-title is-live">${escapeHtml(t('status.thinking'))}</span>
                <span class="section-meta">${escapeHtml(t('status.live'))}</span>
            </button>
            <div class="reasoning-body"></div>`;
        reasoningHost.appendChild(card);
        card.classList.add('collapsed');
    }

    return card;
}

function toggleReasoningCard(btn) {
    const card = btn.closest('.reasoning-status, .tool-status-card');
    if (!card) return;
    const nextCollapsed = card.dataset.collapsed !== 'true';
    card.dataset.collapsed = nextCollapsed ? 'true' : 'false';
    card.classList.toggle('collapsed', nextCollapsed);
    card.dataset.userExpanded = nextCollapsed ? 'false' : 'true';
}

function formatStripDuration(startedAt, fallbackMs = 0) {
    const durationMs = startedAt ? Math.max(0, Date.now() - Number(startedAt)) : fallbackMs;
    return formatThoughtDuration(durationMs);
}

function ensureToolCard(elementId, toolName = 'Tool') {
    const { msgEl, toolsHost } = getAssistantMessageParts(elementId);
    if (!msgEl || !toolsHost) return null;

    const card = document.createElement('section');
    card.className = 'tool-status-card is-running collapsed';
    card.id = `${elementId}-tool-${Date.now()}-${toolsHost.children.length}`;
    card.dataset.collapsed = 'true';
    card.dataset.toolName = toolName;
    card.dataset.startedAt = String(Date.now());
    card._history = [];
    card.innerHTML = `
        <button type="button" class="reasoning-header tool-strip-header" onclick="toggleReasoningCard(this)">
            <span class="reasoning-chevron material-icons-round">play_arrow</span>
            <span class="tool-header-group is-live">
                <span class="reasoning-title">MCP</span>
                <span class="tool-header-separator" aria-hidden="true">•</span>
                <span class="tool-header-name">${escapeHtml(formatToolDisplayName(toolName))}</span>
                <span class="tool-header-separator" aria-hidden="true">•</span>
                <span class="tool-header-status">${escapeHtml(t('status.running'))}</span>
            </span>
        </button>
        <div class="tool-card-body">
            <div class="tool-card-query" hidden></div>
            <div class="tool-card-history" hidden></div>
        </div>`;
    toolsHost.appendChild(card);
    msgEl.dataset.activeToolCard = card.id;
    return card;
}

function getActiveToolCard(elementId) {
    const { msgEl } = getAssistantMessageParts(elementId);
    if (!msgEl || !msgEl.dataset.activeToolCard) return null;
    return document.getElementById(msgEl.dataset.activeToolCard);
}

function setToolCardState(elementId, state, summary = '', args = null, toolName = '') {
    let card = getActiveToolCard(elementId);
    if (!card && state === 'running') {
        card = ensureToolCard(elementId, toolName || 'Tool');
    }
    if (!card) return;

    const titleEl = card.querySelector('.reasoning-title');
    const headerGroupEl = card.querySelector('.tool-header-group');
    const nameEl = card.querySelector('.tool-header-name');
    const summaryEl = card.querySelector('.tool-header-status');
    const queryEl = card.querySelector('.tool-card-query');
    const historyEl = card.querySelector('.tool-card-history');
    const activeToolName = toolName || card.dataset.toolName || 'Tool';
    const previewText = extractToolPreview(args, summary, activeToolName);
    const lastPreviewText = card.dataset.lastPreviewText || '';
    const shouldKeepExpanded = card.dataset.userExpanded === 'true';

    card.classList.remove('is-running', 'is-success', 'is-failure');
    if (state === 'failure') {
        card.classList.add('is-failure');
        if (shouldKeepExpanded) {
            card.dataset.collapsed = 'false';
            card.classList.remove('collapsed');
        } else {
            card.dataset.collapsed = 'true';
            card.classList.add('collapsed');
        }
    } else if (state === 'success') {
        card.classList.add('is-success');
        if (shouldKeepExpanded) {
            card.dataset.collapsed = 'false';
            card.classList.remove('collapsed');
        } else {
            card.dataset.collapsed = 'true';
            card.classList.add('collapsed');
        }
    } else {
        card.classList.add('is-running');
        if (shouldKeepExpanded) {
            card.dataset.collapsed = 'false';
            card.classList.remove('collapsed');
        } else {
            card.dataset.collapsed = 'true';
            card.classList.add('collapsed');
        }
    }

    if (titleEl) {
        titleEl.textContent = 'MCP';
    }
    if (nameEl) {
        nameEl.textContent = formatToolDisplayName(activeToolName);
    }
    card.dataset.toolName = activeToolName;
    if (previewText) {
        card.dataset.lastPreviewText = previewText;
    }

        if (summaryEl) {
        let statusLabel = t('status.done');
        if (state === 'running') statusLabel = previewText || lastPreviewText || t('status.running');
        else if (state === 'failure') statusLabel = summary || t('status.failed');
        else if (summary && !isToolExecutionFinishedSummary(summary)) statusLabel = summary;
        summaryEl.textContent = statusLabel;
        summaryEl.classList.toggle('is-query-preview', state === 'running' && !!(previewText || lastPreviewText));
        summaryEl.title = state === 'running' && (previewText || lastPreviewText) ? (previewText || lastPreviewText) : statusLabel;
    }
    if (headerGroupEl) {
        headerGroupEl.classList.toggle('is-live', state === 'running');
    }

    if (state === 'running' && (previewText || activeToolName)) {
        appendToolHistory(card, activeToolName, previewText, args);
    }

    if (queryEl) {
        const detailText = previewText || (state === 'failure' ? summary : '');
        queryEl.hidden = !detailText || state !== 'running';
        queryEl.textContent = detailText || '';
    }
    renderToolHistory(card, historyEl, state);
}

function extractToolPreview(args, summary = '', toolName = '') {
    const normalizedTool = String(toolName || '').trim().toLowerCase();

    if (normalizedTool === 'get_current_time') {
        return t('tool.currentTimeChecked');
    }

    if (args && typeof args === 'object') {
        if (normalizedTool === 'namu_wiki') {
            const keyword = extractToolObjectValue(args, ['keyword', 'query', 'title', 'input']);
            if (keyword) return keyword;
        }

        const candidateKeys = ['query', 'url', 'text', 'prompt', 'input', 'title'];
        for (const key of candidateKeys) {
            const value = args[key];
            if (typeof value === 'string' && value.trim()) {
                return value.trim();
            }
        }

        const compactJson = stringifyToolArgs(args);
        if (compactJson) {
            return compactJson;
        }
    }

    if (typeof args === 'string' && args.trim()) {
        if (normalizedTool === 'get_current_time' && args.trim() === '{}') {
            return t('tool.currentTimeChecked');
        }
        return args.trim();
    }

    if (summary && !/^running$/i.test(summary.trim()) && !isToolExecutionFinishedSummary(summary.trim())) {
        return summary.trim();
    }

    return '';
}

function extractToolObjectValue(args, keys) {
    for (const key of keys) {
        const value = args?.[key];
        if (typeof value === 'string' && value.trim()) {
            return value.trim();
        }
    }
    return '';
}

function stringifyToolArgs(args) {
    try {
        const json = JSON.stringify(args);
        if (!json) return '';
        if (json.length <= 220) return json;
        return `${json.slice(0, 220)}...`;
    } catch (_) {
        return '';
    }
}

function formatToolDisplayName(toolName = '') {
    const cleaned = String(toolName || '').trim();
    if (!cleaned) return 'Tool';

    return cleaned
        .replace(/[_-]+/g, ' ')
        .replace(/\s+/g, ' ')
        .replace(/\b\w/g, char => char.toUpperCase());
}

function appendToolHistory(card, toolName, previewText, args) {
    if (!card) return;
    if (!Array.isArray(card._history)) card._history = [];

    const displayTool = formatToolDisplayName(toolName);
    const detail = (previewText || extractToolPreview(args, '', toolName) || '').trim();
    if (!detail) return;
    const signature = `${displayTool}::${detail}`;
    const last = card._history[card._history.length - 1];
    if (last && last.signature === signature) return;

    card._history.push({
        signature,
        tool: displayTool,
        detail
    });
}

function renderToolHistory(card, historyEl, state) {
    if (!historyEl || !card) return;
    const history = Array.isArray(card._history) ? card._history : [];

    if (history.length === 0) {
        historyEl.hidden = true;
        historyEl.innerHTML = '';
        return;
    }

    historyEl.hidden = false;
    const renderedCount = Number(card.dataset.renderedHistoryCount || 0);

    if (renderedCount > history.length || renderedCount === 0) {
        historyEl.innerHTML = history.map((entry, index) => `
            <div class="tool-history-item">
                <div class="tool-history-title">${escapeHtml(`${index + 1}. ${entry.tool}`)}</div>
                <div class="tool-history-detail">${escapeHtml(entry.detail || t('tool.noQueryDetails'))}</div>
            </div>
        `).join('');
    } else if (history.length > renderedCount) {
        const fragment = document.createDocumentFragment();
        history.slice(renderedCount).forEach((entry, index) => {
            const item = document.createElement('div');
            item.className = 'tool-history-item is-new';
            item.innerHTML = `
                <div class="tool-history-title">${escapeHtml(`${renderedCount + index + 1}. ${entry.tool}`)}</div>
                <div class="tool-history-detail">${escapeHtml(entry.detail || t('tool.noQueryDetails'))}</div>
            `;
            fragment.appendChild(item);
            requestAnimationFrame(() => item.classList.remove('is-new'));
        });
        historyEl.appendChild(fragment);
    }

    card.dataset.renderedHistoryCount = String(history.length);
}

// New helper functions
// New helper functions
async function copyMessage(btn) {
    const bubble = btn.closest('.message-inner').querySelector('.markdown-body');
    if (!bubble) return;

    // Get text content (try to get clean text without HTML if possible, or just innerText)
    const text = bubble.innerText;
    try {
        await navigator.clipboard.writeText(text);
        showToast('Copied to clipboard');
    } catch (err) {
        console.warn('Clipboard API failed, trying fallback', err);
        fallbackCopyTextToClipboard(text);
    }
}

function fallbackCopyTextToClipboard(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;

    // Avoid scrolling to bottom
    textArea.style.top = "0";
    textArea.style.left = "0";
    textArea.style.position = "fixed";

    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
        var successful = document.execCommand('copy');
        var msg = successful ? 'successful' : 'unsuccessful';
        if (successful) {
            showToast('Copied to clipboard');
        } else {
            showToast('Failed to copy', true);
        }
    } catch (err) {
        console.error('Fallback: Oops, unable to copy', err);
        showToast('Failed to copy', true);
    }
    document.body.removeChild(textArea);
}

function showToast(message, isError = false) {
    let toast = document.getElementById('toast-notification');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast-notification';
        toast.className = 'toast';
        document.body.appendChild(toast);
    }

    const icon = isError ? 'error_outline' : 'check_circle';
    const color = isError ? 'var(--danger-color)' : 'var(--success-color)';

    toast.innerHTML = `
        <span class="material-icons-round" style="color: ${color}">${icon}</span>
        <span>${message}</span>
    `;
    toast.style.bottom = `${getToastBottomOffset()}px`;

    // Trigger reflow
    void toast.offsetWidth;

    toast.classList.add('show');

    // Hide after 3s
    if (toast.timeoutId) clearTimeout(toast.timeoutId);
    toast.timeoutId = setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

function speakMessageFromBtn(btn) {
    const bubble = btn.closest('.message-inner').querySelector('.markdown-body');
    if (bubble) {
        speakMessage(bubble.innerText, btn);
    }
}

function getToastBottomOffset() {
    if (!inputArea) return 20;
    const rect = inputArea.getBoundingClientRect();
    return Math.max(20, window.innerHeight - rect.top + 16);
}

/**
 * Stop all audio playback and clear queues
 */
function stopAllAudio() {
    // Clear queues
    ttsQueue = [];
    audioWarmup = null;

    // Stop current audio source
    if (currentAudio) {
        try {
            currentAudio.pause();
            currentAudio.src = '';
            currentAudio.load(); // Forces resource release
        } catch (e) {
            // Ignore
        }
    }

    // Clear audio cache to free memory
    clearTTSAudioCache();
    clearMediaSessionMetadata();
    releaseWakeLock(); // Release lock on stop

    // Reset loop state
    isPlayingQueue = false;

    // Cancel streaming
    streamingTTSActive = false;
    streamingTTSBuffer = "";
    streamingTTSCommittedIndex = 0;

    // Increment session ID to invalidate pending ops
    ttsSessionId++;
    activeTTSSessionLabel = "";

    // Reset UI
    const btn = currentAudioBtn;
    if (btn) {
        const iconEl = btn.querySelector('.material-icons-round');
        if (iconEl) iconEl.textContent = 'volume_up';
        btn.title = 'Speak';
        btn.disabled = false;
    }
    currentAudioBtn = null;
}

// Show/Hide Reasoning Status Helper with Streaming Support
function showReasoningStatus(elementId, text, isFinal = false) {
    if (config.hideThink) return;

    const card = ensureReasoningCard(elementId);
    if (!card) return;

    const metaEl = card.querySelector('.section-meta');
    const titleEl = card.querySelector('.reasoning-title');
    const bodyEl = card.querySelector('.reasoning-body');
    if (!bodyEl) return;

    const startedAt = Number(card.dataset.startedAt || Date.now());
    const durationMs = Math.max(0, Date.now() - startedAt);

    if (isFinal) {
        finalizeReasoningStatus(elementId, 'done');
        return;
    }

    let cleanText = (text || '')
        .replace(/<think>/g, '')
        .replace(/<\/think>/g, '')
        .trim();

    if (!cleanText) {
        cleanText = 'Thinking...';
    }

    const MAX_DISPLAY_LENGTH = 2400;
    if (cleanText.length > MAX_DISPLAY_LENGTH) {
        cleanText = '...\n' + cleanText.slice(-MAX_DISPLAY_LENGTH);
    }

    const shouldKeepExpanded = card.dataset.userExpanded === 'true';
    if (shouldKeepExpanded) {
        card.dataset.collapsed = 'false';
        card.classList.remove('collapsed');
    } else {
        card.dataset.collapsed = 'true';
        card.classList.add('collapsed');
    }
    card.classList.remove('completed', 'failed');
    if (metaEl) metaEl.textContent = t('status.live');
    if (titleEl) {
        titleEl.classList.add('is-live');
        titleEl.textContent = t('status.thinking');
    }
    bodyEl.textContent = cleanText;
}

function finalizeReasoningStatus(elementId, outcome = 'done', detail = '') {
    if (config.hideThink) return;

    const card = ensureReasoningCard(elementId);
    if (!card) return;

    const metaEl = card.querySelector('.section-meta');
    const titleEl = card.querySelector('.reasoning-title');
    const bodyEl = card.querySelector('.reasoning-body');
    const startedAt = Number(card.dataset.startedAt || Date.now());
    const durationMs = Math.max(0, Date.now() - startedAt);
    const shouldKeepExpanded = card.dataset.userExpanded === 'true';

    card.classList.remove('completed', 'failed');
    card.classList.add(outcome === 'failed' ? 'failed' : 'completed');
    if (shouldKeepExpanded) {
        card.dataset.collapsed = 'false';
        card.classList.remove('collapsed');
    } else {
        card.dataset.collapsed = 'true';
        card.classList.add('collapsed');
    }
    card.dataset.durationMs = String(durationMs);

    if (metaEl) {
        if (outcome === 'failed') metaEl.textContent = t('status.failed');
        else if (outcome === 'stopped') metaEl.textContent = t('status.stopped');
        else metaEl.textContent = t('status.done');
    }

    if (titleEl) {
        titleEl.classList.remove('is-live');
        if (outcome === 'failed') titleEl.textContent = t('status.failed');
        else if (outcome === 'stopped') titleEl.textContent = t('status.stopped');
        else titleEl.textContent = formatThoughtDuration(durationMs);
    }

    if (bodyEl && detail) {
        bodyEl.textContent = detail;
    }
}

function finalizeAssistantStatusCards(elementId, outcome = 'done', detail = '') {
    if (!elementId) return;

    const runningToolCards = getRunningToolCards(elementId);
    if (runningToolCards.length > 0) {
        runningToolCards.forEach((card) => {
            finalizeToolCard(card, outcome, detail);
        });
    }

    const reasoningCard = getAssistantMessageParts(elementId).reasoningHost?.querySelector('.reasoning-status');
    if (reasoningCard && reasoningCard.querySelector('.reasoning-title')?.classList.contains('is-live')) {
        finalizeReasoningStatus(elementId, outcome, detail);
    }
}

function renderInitialAssistantMarkdown(text) {
    const normalized = normalizeMarkdownForRender(text || '');
    if (!normalized.trim()) {
        return '<div class="markdown-committed"></div><div class="markdown-pending"></div>';
    }
    return `
        <div class="markdown-committed">${marked.parse(normalized)}</div>
        <div class="markdown-pending"></div>
    `;
}

function ensureStreamingMarkdownHosts(bubble) {
    if (!bubble) return {};

    let markdownBody = bubble.querySelector('.markdown-body');
    if (!markdownBody) {
        markdownBody = document.createElement('div');
        markdownBody.className = 'markdown-body';
        bubble.prepend(markdownBody);
    }

    let committedHost = markdownBody.querySelector('.markdown-committed');
    let pendingHost = markdownBody.querySelector('.markdown-pending');
    if (!committedHost || !pendingHost) {
        const existingHtml = markdownBody.innerHTML;
        markdownBody.innerHTML = `
            <div class="markdown-committed">${existingHtml}</div>
            <div class="markdown-pending"></div>
        `;
        committedHost = markdownBody.querySelector('.markdown-committed');
        pendingHost = markdownBody.querySelector('.markdown-pending');
    }

    return { markdownBody, committedHost, pendingHost };
}

function splitStreamingMarkdown(text) {
    const normalized = String(text || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    if (!normalized) {
        return { committedText: '', pendingText: '' };
    }

    const lines = normalized.split('\n');
    let inCodeBlock = false;
    let cursor = 0;
    let committedParts = [];
    let pendingParts = [];
    let currentBlock = [];
    let currentBlockStart = 0;

    const flushCommittedBlock = (endOffsetInclusive) => {
        if (endOffsetInclusive <= currentBlockStart) {
            currentBlock = [];
            return;
        }
        committedParts.push(normalized.slice(currentBlockStart, endOffsetInclusive));
        currentBlock = [];
        currentBlockStart = endOffsetInclusive;
    };

    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const lineWithBreak = i < lines.length - 1 ? `${line}\n` : line;
        const trimmed = line.trim();
        const lineStart = cursor;
        const lineEnd = cursor + lineWithBreak.length;

        if (/^```/.test(trimmed)) {
            if (!inCodeBlock && currentBlock.length === 0) {
                currentBlockStart = lineStart;
            }
            inCodeBlock = !inCodeBlock;
            currentBlock.push(lineWithBreak);
            if (!inCodeBlock) flushCommittedBlock(lineEnd);
            cursor = lineEnd;
            continue;
        }

        if (inCodeBlock) {
            currentBlock.push(lineWithBreak);
            cursor = lineEnd;
            continue;
        }

        if (trimmed === '') {
            if (currentBlock.length > 0) {
                flushCommittedBlock(lineStart);
            }
            committedParts.push(lineWithBreak);
            currentBlockStart = lineEnd;
            cursor = lineEnd;
            continue;
        }

        if (currentBlock.length === 0) {
            currentBlockStart = lineStart;
        }
        currentBlock.push(lineWithBreak);
        cursor = lineEnd;
    }

    if (currentBlock.length > 0) {
        pendingParts.push(normalized.slice(currentBlockStart));
    }

    return {
        committedText: committedParts.join(''),
        pendingText: pendingParts.join('')
    };
}

function highlightMarkdownBlocks(container) {
    if (!container) return;
    container.querySelectorAll('pre code').forEach((block) => {
        highlight.highlightElement(block);
        block.dataset.highlighted = 'true';
    });
}

function renderMarkdownIntoHost(host, markdownText) {
    if (!host) return;
    const normalized = normalizeMarkdownForRender(markdownText || '');
    host.innerHTML = normalized.trim() ? marked.parse(normalized) : '';
    host.querySelectorAll('a').forEach((link) => {
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
    });
    highlightMarkdownBlocks(host);
}

function schedulePendingMarkdownRender(el, pendingHost, pendingText) {
    if (!el || !pendingHost) return;
    if (!el._pendingRenderState) {
        el._pendingRenderState = { scheduled: false, text: '' };
    }

    el._pendingRenderState.text = pendingText;
    if (el._pendingRenderState.scheduled) return;

    el._pendingRenderState.scheduled = true;
    requestAnimationFrame(() => {
        if (!el._pendingRenderState) return;
        el._pendingRenderState.scheduled = false;
        renderMarkdownIntoHost(pendingHost, el._pendingRenderState.text || '');
    });
}

function updateMessageContent(id, text) {
    const el = ensureAssistantMessageElement(id);
    if (!el) return;
    const wasNearBottom = isChatNearBottom();

    const bubble = el.querySelector('.message-bubble');
    const { markdownBody: mdBody, committedHost, pendingHost } = ensureStreamingMarkdownHosts(bubble);

    // Filter out common special tokens that might leak during streaming
    let cleanText = text;
    // Aggressive cleaning for Command-R / GPT-OSS leakage (Step Id: 4815)
    // 1. Remove all special tokens like <|...|> (multi-line)
    cleanText = cleanText.replace(/<\|[\s\S]*?\|>/g, '');

    // 2. Remove <commentary ...> style pseudo-tags
    cleanText = cleanText.replace(/<commentary[\s\S]*?>/gi, '');

    // 3. Remove "commentary to=..." text artifacts
    cleanText = cleanText.replace(/commentary to=[a-z_]+(\s+(json|code|text))?/gi, '');

    // 4. Remove standalone leakage of tool call JSON objects
    cleanText = cleanText.replace(/\{"name"\s*:\s*"[^"]+"\s*,\s*"arguments"\s*:\s*([\s\S]*?)\}/g, '');

    // 5. Clean up remaining standalone markers and trim
    cleanText = cleanText.trim().replace(/^(json|code|text)\s*/gi, '');

    // Final pass for any partial tag leftovers or repeated markers
    cleanText = cleanText.replace(/<\|.*?\|>/g, '');
    const streamState = el._streamRenderState || { committedText: '', pendingText: '' };
    const { committedText, pendingText } = splitStreamingMarkdown(cleanText);

    if (committedText !== streamState.committedText) {
        renderMarkdownIntoHost(committedHost, committedText);
        streamState.committedText = committedText;
    }

    if (pendingText !== streamState.pendingText) {
        schedulePendingMarkdownRender(el, pendingHost, pendingText);
        streamState.pendingText = pendingText;
    }
    el._streamRenderState = streamState;

    const responseCard = el.querySelector('.assistant-response-card');
    const actionBar = el.querySelector('.message-actions');
    const hasVisibleContent = !!cleanText.trim();
    if (responseCard) responseCard.hidden = !hasVisibleContent;
    if (actionBar) {
        if (el.id && !actionBar.classList.contains('is-ready')) {
            actionBar.hidden = true;
        } else {
            actionBar.hidden = !hasVisibleContent;
        }
    }

    scrollToBottom(wasNearBottom);
    const codeBlocks = mdBody.querySelectorAll('pre code');
    if (wasNearBottom && codeBlocks.length > 0) {
        holdAutoScrollAtBottom(900);
        observeAutoScrollResizes([el, bubble, mdBody, ...mdBody.querySelectorAll('pre')]);
    }
}


function isChatNearBottom() {
    if (!chatMessages) return true;
    const distanceFromBottom = chatMessages.scrollHeight - chatMessages.clientHeight - chatMessages.scrollTop;
    return distanceFromBottom <= AUTO_SCROLL_THRESHOLD_PX;
}

function scrollToBottom(force = false) {
    if (!chatMessages) return;
    if (!force && !shouldAutoScroll && !lockScrollToLatest) return;
    scheduleChatScrollToBottom();
    shouldAutoScroll = true;
}

function holdAutoScrollAtBottom(durationMs = 700) {
    if (!chatMessages) return;
    if (autoScrollHoldTimeout) {
        clearTimeout(autoScrollHoldTimeout);
        autoScrollHoldTimeout = null;
    }
    scheduleChatScrollToBottom();
    autoScrollHoldTimeout = setTimeout(() => {
        if (chatMessages && (shouldAutoScroll || lockScrollToLatest)) {
            scheduleChatScrollToBottom();
        }
        autoScrollHoldTimeout = null;
    }, durationMs);
}

function scheduleChatScrollToBottom() {
    if (!chatMessages || pendingScrollToBottom) return;
    pendingScrollToBottom = true;
    requestAnimationFrame(() => {
        pendingScrollToBottom = false;
        if (!chatMessages) return;
        suppressNextScrollEvent = true;
        chatMessages.scrollTop = chatMessages.scrollHeight;
        scrollActiveMessageIntoView();
    });
}

function observeAutoScrollResizes(elements) {
    if (!chatMessages || typeof ResizeObserver === 'undefined') return;
    if (autoScrollResizeObserver) {
        autoScrollResizeObserver.disconnect();
        autoScrollResizeObserver = null;
    }

    const targets = (elements || []).filter(Boolean);
    if (targets.length === 0) return;

    autoScrollResizeObserver = new ResizeObserver(() => {
        if (!shouldAutoScroll && !lockScrollToLatest) return;
        scheduleChatScrollToBottom();
    });

    targets.forEach((target) => autoScrollResizeObserver.observe(target));
}

function scrollActiveMessageIntoView() {
    if (!chatMessages || !activeStreamingMessageId) return;
    const activeMessage = document.getElementById(activeStreamingMessageId);
    if (!activeMessage) return;
    const responseCard = activeMessage.querySelector('.assistant-response-card');
    const target = responseCard || activeMessage;
    const containerRect = chatMessages.getBoundingClientRect();
    const targetRect = target.getBoundingClientRect();
    const inputRect = inputArea ? inputArea.getBoundingClientRect() : null;
    const occlusion = inputRect ? Math.max(0, containerRect.bottom - inputRect.top) : 0;
    const desiredBottom = containerRect.bottom - occlusion - 16;
    const delta = targetRect.bottom - desiredBottom;

    if (delta > 0) {
        suppressNextScrollEvent = true;
        chatMessages.scrollTop += delta;
    }
}

function startStreamingMessageAutoScroll(messageId) {
    activeStreamingMessageId = messageId;
    const activeMessage = document.getElementById(messageId);
    if (!activeMessage) return;
    const responseCard = activeMessage.querySelector('.assistant-response-card');
    const markdownBody = activeMessage.querySelector('.markdown-body');
    const codeBlocks = activeMessage.querySelectorAll('pre');
    observeAutoScrollResizes([activeMessage, responseCard, markdownBody, ...codeBlocks]);
}

function stopStreamingMessageAutoScroll() {
    if (autoScrollResizeObserver) {
        autoScrollResizeObserver.disconnect();
        autoScrollResizeObserver = null;
    }
}

// TTS: Speak a message using the Go server's /api/tts endpoint
async function speakMessage(text, btn = null) {
    // If clicking the same button that is currently playing or streaming, stop it
    if ((isPlayingQueue || streamingTTSActive) && btn && btn === currentAudioBtn) {
        stopAllAudio();
        return;
    }

    // Stop previous audio before starting new one
    stopAllAudio();

    if (!config.enableTTS) {
        if (!btn) return; // Don't alert on auto-play failure if disabled
        alert('TTS is disabled. Enable it in settings.');
        return;
    }

    // Clean text for TTS (remove emojis, markdown, etc.)
    const cleanText = cleanTextForTTS(text);
    if (!cleanText) return;
    activeTTSSessionLabel = cleanText.substring(0, 120) + (cleanText.length > 120 ? '...' : '');

    // Initialize/Clear queue
    ttsQueue = []; // Clear existing queue if any (though stopAllAudio called above)
    if (btn) currentAudioBtn = btn;

    // Minimum chunk length - chunks shorter than this will be merged
    const MIN_CHUNK_LENGTH = 50;

    // 1. Split by paragraphs (double newlines) first
    const paragraphs = cleanText.split(/\n\s*\n+/);

    let currentChunk = "";

    for (const paragraph of paragraphs) {
        if (!paragraph.trim()) continue;

        // 2. Smart sentence splitting:
        // - Split on sentence endings (.!?) followed by space and uppercase/Korean
        // - But NOT on numbered lists like "1.", "2." or abbreviations
        // - Pattern: sentence ending + space + (uppercase letter OR Korean character)
        const sentencePattern = /(?<=[.!?])(?=\s+(?:[A-Z가-힣]|$))/g;
        const rawChunks = paragraph.split(sentencePattern).filter(s => s.trim());

        // If no splits found, use the whole paragraph
        const sentences = rawChunks.length > 0 ? rawChunks : [paragraph];

        // 3. Combine sentences up to chunkSize, merging short chunks
        for (const part of sentences) {
            const trimmedPart = part.trim();
            if (!trimmedPart) continue;

            // If adding this part exceeds chunkSize and we have content, queue current chunk
            if ((currentChunk + " " + trimmedPart).length > config.chunkSize && currentChunk.length >= MIN_CHUNK_LENGTH) {
                ttsQueue.push(currentChunk.trim());
                currentChunk = "";
                processTTSQueue();
            }

            // Add to current chunk
            currentChunk = currentChunk ? currentChunk + " " + trimmedPart : trimmedPart;
        }

        // At paragraph end, queue if we have enough content
        if (currentChunk.length >= MIN_CHUNK_LENGTH) {
            ttsQueue.push(currentChunk.trim());
            currentChunk = "";
            processTTSQueue();
        }
    }

    // Final chunk - queue even if short (it's the last one)
    if (currentChunk.trim()) {
        ttsQueue.push(currentChunk.trim());
        processTTSQueue();
    }
}

// ============================================================================
// Streaming TTS Functions
// These enable TTS generation to start while LLM is still streaming
// ============================================================================

/**
 * Clean text for TTS - removes emojis, markdown, and non-speakable characters
 */
/**
 * Clean text for TTS - removes emojis, markdown, and non-speakable characters
 * Optimized to prevent duplicates and improve performance
 */
function cleanTextForTTS(text) {
    if (!text) return '';

    let cleaned = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');

    // 1. Remove Structural Artifacts (Tools, Thinking, HTML)
    // Remove tool status messages (content validation)
    cleaned = cleaned.replace(/<span class="tool-status"[\s\S]*?<\/span>/g, '');
    // Remove raw Tool Call logs
    cleaned = cleaned.replace(/Tool Call:.*?(?:[.!?\n]|$)+/gi, '');
    // Remove <think> blocks if any remain
    cleaned = cleaned.replace(/<think>[\s\S]*?<\/think>/g, '');
    // Remove all HTML tags
    cleaned = cleaned.replace(/<[^>]*>/g, '');

    // 2. Apply Custom Dictionary Corrections
    if (typeof ttsDictionaryRegex !== 'undefined' && ttsDictionaryRegex) {
        cleaned = cleaned.replace(ttsDictionaryRegex, (match) => {
            return ttsDictionary[match.toLowerCase()] || match;
        });
    }

    // 3. Remove URLs and Links
    // Remove raw URLs (http://...) but keep the text
    cleaned = cleaned.replace(/https?:\/\/[^\s]+/g, '');
    // Convert Markdown links [Text](URL) -> Text
    cleaned = cleaned.replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1');
    // Remove Images ![Alt](URL) -> Alt
    cleaned = cleaned.replace(/!\[([^\]]*)\]\([^\)]+\)/g, '$1');

    // 4. Clean Markdown Syntax (Preserving Content)
    // Remove Code Blocks entirely (fence + content)
    // User requested to skip source code reading
    cleaned = cleaned.replace(/```[\s\S]*?```/g, '');
    // Remove inline code if it looks like variable names (optional, but requested "exclude source code")
    // Let's keep inline code for now as it might be relevant words, but strict block removal is key.
    cleaned = cleaned.replace(/`[^`]+`/g, ''); // Remove inline code too for cleaner reading?
    // User said "source code", usually implies blocks.
    // But inline `text` often contains variable names that disrupt flow.
    // Let's stick to removing blocks first. Inline: remove backticks but keep text?
    // Previous logic was "remove backticks keep text".
    // User complaint was about "source code".
    // Let's remove blocks entirely. Inline: keep text.
    cleaned = cleaned.replace(/`/g, ''); // Remove inline backticks

    // Convert Markdown headings into explicit paragraph breaks for stronger pauses.
    cleaned = cleaned.replace(/^(#{1,6})\s+(.+?)([.!?]?)$/gm, (_, hashes, title, punct) => {
        const level = hashes.length;
        const suffix = punct || '.';
        const pauseBreak = level <= 2 ? '\n\n' : '\n';
        return `${title}${suffix}${pauseBreak}`;
    });

    // Remove Bold/Italic wrappers (**text**, __text__, *text*, _text_)
    cleaned = cleaned.replace(/(\*\*|__)(.*?)\1/g, '$2');
    cleaned = cleaned.replace(/(\*|_)(.*?)\1/g, '$2');

    // Remove Blockquotes (>)
    cleaned = cleaned.replace(/^>\s+/gm, '');
    // Remove Horizontal Rules (---)
    cleaned = cleaned.replace(/^([-*_]){3,}\s*$/gm, '\n\n');
    // Treat list items as separate spoken units with an explicit boundary.
    cleaned = cleaned.replace(/^\s*[-*+]\s+(.+)$/gm, '\n$1.\n');
    cleaned = cleaned.replace(/^\s*(\d+)[\.\)]\s+(.+)$/gm, '\n$1. $2.\n');

    // 5. Remove Emojis and Symbols (Consolidated Regex)
    // Ranges: Emoticons, Symbols, Transport, Flags, Dingbats, Shapes, Arrows, etc.
    const symbolRegex = /[\u{1F600}-\u{1F64F}\u{1F300}-\u{1F5FF}\u{1F680}-\u{1F6FF}\u{1F1E0}-\u{1F1FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{FE00}-\u{FE0F}\u{1F900}-\u{1F9FF}\u{1FA00}-\u{1FA6F}\u{1FA70}-\u{1FAFF}\u{2300}-\u{23FF}\u{25A0}-\u{25FF}\u{2B00}-\u{2BFF}\u{2190}-\u{21FF}\u{2900}-\u{297F}\u{3290}-\u{329F}\u{3030}\u{303D}]/gu;
    cleaned = cleaned.replace(symbolRegex, '');

    // 6. Normalization and Punctuation
    // Fancy quotes to space or simple quotes
    cleaned = cleaned.replace(/[«»""„‚]/g, ' ');
    // Separators to comma/pause
    cleaned = cleaned.replace(/[=→—–]/g, ', ');
    cleaned = cleaned.replace(/\s*[-•◦▪▸►]\s*/g, ', ');
    // Ellipsis
    cleaned = cleaned.replace(/\.{3,}/g, '.');

    // Remove specific unwanted characters if any remain
    cleaned = cleaned.replace(/[*~|]/g, '');

    // Ensure sentence spacing (e.g. "end.Next" -> "end. Next")
    cleaned = cleaned.replace(/([.!?])(?=[^ \n])/g, '$1 ');
    cleaned = cleaned.replace(/([^\s.!?])\n/g, '$1.\n');
    cleaned = cleaned.replace(/\n([^\s])/g, '\n$1');

    // Normalize Whitespace
    cleaned = cleaned.replace(/\n{4,}/g, '\n\n\n'); // Preserve strong paragraph/list pauses
    cleaned = cleaned.replace(/[ \t]+/g, ' ');    // Collapse spaces
    cleaned = cleaned.replace(/^\s+|\s+$/gm, ''); // Trim lines

    return cleaned.trim();
}

/**
 * Initialize streaming TTS for a new message
 */
function initStreamingTTS(elementId) {
    // Stop any existing audio/TTS
    stopAllAudio();

    streamingTTSActive = true;
    streamingTTSCommittedIndex = 0;
    streamingTTSBuffer = "";
    activeTTSSessionLabel = "Streaming TTS";

    // Get speak button for UI updates
    const msgEl = document.getElementById(elementId);
    if (msgEl) {
        currentAudioBtn = msgEl.querySelector('.speak-btn');
    }

    console.log("[Streaming TTS] Initialized");
}

function getStreamingChunkTargets() {
    const baseTarget = Math.max(parseInt(config.chunkSize) || 200, 80);
    const firstChunkTarget = Math.min(baseTarget, 48);
    return {
        firstChunkTarget,
        weakBoundaryTarget: Math.max(Math.floor(baseTarget * 0.45), 36),
        strongBoundaryTarget: Math.max(Math.floor(baseTarget * 0.72), 64),
        hardCeiling: Math.max(Math.floor(baseTarget * 1.2), 120)
    };
}

function detectStreamingBoundary(newText) {
    const patterns = [
        { kind: 'strong', regex: /^([\s\S]*?\n{2,})/ },
        { kind: 'strong', regex: /^([\s\S]*?\n)/ },
        { kind: 'strong', regex: /^([\s\S]*?[.!?])(?:\s+|$)/ },
        { kind: 'weak', regex: /^([\s\S]*?[,;:])(?:\s+|$)/ }
    ];

    for (const pattern of patterns) {
        const match = newText.match(pattern.regex);
        if (match && match[1] && match[1].trim()) {
            return { text: match[1], kind: pattern.kind };
        }
    }
    return null;
}

function shouldCommitStreamingBoundary(length, boundaryKind, hasQueuedAudio) {
    const targets = getStreamingChunkTargets();
    if (!hasQueuedAudio) {
        return length >= targets.firstChunkTarget || boundaryKind === 'strong';
    }
    if (boundaryKind === 'strong') {
        return length >= targets.strongBoundaryTarget;
    }
    return length >= targets.weakBoundaryTarget;
}

function splitTTSParagraphByPriority(text, maxChunkSize, minChunkLength, force = false) {
    const chunks = [];
    let remaining = (text || '').trim();
    if (!remaining) return chunks;

    const boundaryRegex = /([\s\S]*?(?:\n{2,}|\n|[.!?](?=\s|$)|[,;:](?=\s|$)))/g;

    while (remaining) {
        if (remaining.length <= maxChunkSize) {
            if ((remaining.length >= minChunkLength || force) && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(remaining)) {
                chunks.push(remaining.trim());
            }
            break;
        }

        const windowText = remaining.slice(0, maxChunkSize + Math.floor(maxChunkSize * 0.25));
        let bestStrong = null;
        let bestWeak = null;
        let match;

        boundaryRegex.lastIndex = 0;
        while ((match = boundaryRegex.exec(windowText)) !== null) {
            const segment = match[1];
            const boundaryEnd = match.index + segment.length;
            if (boundaryEnd < minChunkLength) continue;
            if (boundaryEnd > maxChunkSize) break;

            const trimmed = segment.trimEnd();
            if (!trimmed) continue;

            const isStrong = /(?:\n{2,}|\n|[.!?])\s*$/.test(trimmed);
            if (isStrong) {
                bestStrong = boundaryEnd;
            } else {
                bestWeak = boundaryEnd;
            }
        }

        let splitAt = bestStrong || bestWeak;
        if (!splitAt) {
            splitAt = remaining.lastIndexOf(' ', maxChunkSize);
            if (splitAt < minChunkLength) {
                splitAt = maxChunkSize;
            }
        }

        const chunk = remaining.slice(0, splitAt).trim();
        if (chunk && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(chunk)) {
            chunks.push(chunk);
        }
        remaining = remaining.slice(splitAt).trimStart();
    }

    return chunks;
}

/**
 * Feed new display text to the streaming TTS processor
 * This is called every time the LLM emits new tokens
 */
function feedStreamingTTS(displayText) {
    if (!streamingTTSActive) return;

    // 최적화된 청킹 로직:
    // - 첫 청크: 줄바꿈 발견 즉시 커밋 (빠른 시작)
    // - 중간 청크: chunkSize 이상 + 줄바꿈 발견 시 커밋
    // - 마지막 청크: finalizeStreamingTTS()에서 길이 무관 커밋

    // Process all available committed segments in a loop
    let iterations = 0;
    const maxIterations = 20; // Safety limit

    while (iterations < maxIterations) {
        iterations++;

        // Get the new portion of text since last commit
        const newText = displayText.substring(streamingTTSCommittedIndex);
        if (!newText || newText.length < 3) break; // Need at least some content

        let committed = null;
        let advanceBy = 0;
        const hasQueuedAudio = firstChunkPlayedInCurrentSession();
        const targets = getStreamingChunkTargets();

        // PRIORITY 1: Check for Code Blocks (Skip them entirely)
        const codeBlockMatch = newText.match(/(.*?)```[\s\S]*?```/);

        if (codeBlockMatch) {
            const textBefore = codeBlockMatch[1];
            const fullMatch = codeBlockMatch[0];

            if (textBefore.trim()) {
                const cleanedBefore = cleanTextForTTS(textBefore);
                // If cleaned text is substantial, commit it.
                if (cleanedBefore.trim()) {
                    committed = streamingTTSBuffer + cleanedBefore;
                    streamingTTSBuffer = ""; // Reset buffer
                }
            }

            if (committed) {
                advanceBy = fullMatch.length;
            } else {
                streamingTTSCommittedIndex += fullMatch.length;
                // Flush buffer if any
                if (streamingTTSBuffer.trim()) {
                    const toSpeak = streamingTTSBuffer;
                    streamingTTSBuffer = "";
                    pushToStreamingTTSQueue(toSpeak, true);
                }
                continue;
            }
        }

        // PRIORITY 2: Weighted punctuation/newline boundaries
        if (!committed) {
            const boundary = detectStreamingBoundary(newText);
            if (boundary) {
                const potentialCommit = streamingTTSBuffer + cleanTextForTTS(boundary.text);
                if (shouldCommitStreamingBoundary(potentialCommit.length, boundary.kind, hasQueuedAudio)) {
                    committed = potentialCommit;
                    streamingTTSBuffer = "";
                    advanceBy = boundary.text.length;
                } else {
                    streamingTTSBuffer = potentialCommit + " ";
                    streamingTTSCommittedIndex += boundary.text.length;
                    continue;
                }
            }
        }

        if (!committed && (streamingTTSBuffer.length + cleanTextForTTS(newText).length) >= targets.hardCeiling) {
            const forcedCommit = (streamingTTSBuffer + " " + cleanTextForTTS(newText.slice(0, targets.hardCeiling))).trim();
            if (forcedCommit) {
                committed = forcedCommit;
                streamingTTSBuffer = "";
                advanceBy = Math.min(newText.length, targets.hardCeiling);
            }
        }

        // If nothing matched, stop the loop
        if (!committed) break;

        // Commit the segment
        console.log(`[Streaming TTS] Committing (${committed.length} chars): "${committed.substring(0, 50)}..."`);
        pushToStreamingTTSQueue(committed, true);
        streamingTTSCommittedIndex += advanceBy;
    }
}


/**
 * Finalize streaming TTS when LLM stream ends
 * Commits any remaining uncommitted text
 */
function finalizeStreamingTTS(finalDisplayText) {
    if (!streamingTTSActive) return;

    // Commit any remaining text including buffer
    const remainingText = finalDisplayText.substring(streamingTTSCommittedIndex);
    const cleanText = cleanTextForTTS(remainingText);

    // Combine buffer with remaining text
    const finalText = (streamingTTSBuffer + " " + (cleanText || "")).trim();

    if (finalText) {
        console.log(`[Streaming TTS] Finalizing: "${finalText.substring(0, 50)}..."`);
        pushToStreamingTTSQueue(finalText, true); // Force output even if short
    }

    streamingTTSBuffer = "";
    streamingTTSActive = false;
    console.log("[Streaming TTS] Finalized");
}

/**
 * Push a text segment to the TTS queue and ensure processing is running
 * @param {string} text - Text to speak
 * @param {boolean} force - If true, ignores MIN_CHUNK_LENGTH check (use for final chunk)
 */
function pushToStreamingTTSQueue(text, force = false) {
    if (!text || !text.trim()) return;

    const hasQueuedAudio = ttsQueue.length > 0 || isPlayingQueue;
    const minChunkLength = hasQueuedAudio ? 40 : 18;

    const maxChunkSize = Math.max(parseInt(config.chunkSize) || 200, 80);

    // Split into smaller chunks if needed (using weighted boundaries)
    const paragraphs = text.split(/\n+/);
    const newChunks = [];

    for (const para of paragraphs) {
        if (!para.trim()) continue;
        const chunks = splitTTSParagraphByPriority(para, maxChunkSize, minChunkLength, force);
        for (const chunk of chunks) {
            ttsQueue.push(chunk);
            newChunks.push(chunk);
        }
    }

    // IMMEDIATELY start prefetching new chunks (don't wait for the playback loop)
    for (const chunk of newChunks) {
        prefetchTTSAudio(chunk);
    }

    // Start processing if not already running
    if (!isPlayingQueue && ttsQueue.length > 0) {
        processTTSQueue();
    }
}

// ============================================================================
// Global TTS Audio Cache and Prefetch System
// ============================================================================
const ttsAudioCache = new Map(); // text -> Promise<url>

function firstChunkPlayedInCurrentSession() {
    return ttsQueue.length > 0 || isPlayingQueue;
}

/**
 * Prefetch audio for a given text chunk
 * Can be called anytime - will use cached promise if already fetching/fetched
 */
function prefetchTTSAudio(text) {
    if (!text) return null;
    if (ttsAudioCache.has(text)) return ttsAudioCache.get(text);

    const promise = (async () => {
        // Check if session is still valid
        const sessionAtStart = ttsSessionId;

        try {
            const payload = {
                text: text,
                lang: config.ttsLang,
                chunkSize: parseInt(config.chunkSize) || 300,
                voiceStyle: config.ttsVoice,
                speed: parseFloat(config.ttsSpeed) || 1.0,
                steps: parseInt(config.ttsSteps) || 5,
                format: config.ttsFormat || 'wav'
            };

            console.log(`[TTS] Prefetching: "${text.substring(0, 25)}..."`);
            const response = await fetch('/api/tts', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });

            // Check if session changed during fetch
            if (sessionAtStart !== ttsSessionId) {
                console.log(`[TTS] Session changed, discarding prefetch`);
                return null;
            }

            if (!response.ok) {
                console.error(`[TTS] Chunk failed:`, await response.text());
                return null;
            }

            const blob = await response.blob();
            const url = URL.createObjectURL(blob);
            console.log(`[TTS] Prefetch complete: "${text.substring(0, 25)}..."`);
            return url;
        } catch (e) {
            console.error(`[TTS] Chunk error:`, e);
            return null;
        }
    })();

    ttsAudioCache.set(text, promise);
    return promise;
}

/**
 * Clear the audio cache (called on stopAllAudio)
 */
function clearTTSAudioCache() {
    // Revoke all URLs
    ttsAudioCache.forEach(async (promise) => {
        const url = await promise;
        if (url) URL.revokeObjectURL(url);
    });
    ttsAudioCache.clear();
}

async function processTTSQueue(isFirstChunk = false) {
    if (ttsQueue.length === 0) return;
    if (isPlayingQueue) return; // Already running

    isPlayingQueue = true;
    requestWakeLock(); // Request screen keep-alive
    const btn = currentAudioBtn;
    const sessionId = ttsSessionId;
    const mediaSessionLabel = activeTTSSessionLabel;

    if (btn) {
        const iconEl = btn.querySelector('.material-icons-round');
        if (iconEl) iconEl.textContent = 'hourglass_empty';
        btn.disabled = true;
    }

    let firstChunkPlayed = false;

    // Start prefetching first few items immediately
    for (let i = 0; i < Math.min(3, ttsQueue.length); i++) {
        prefetchTTSAudio(ttsQueue[i]);
    }

    // Main processing loop
    while (true) {
        // Check cancellation
        if (sessionId !== ttsSessionId) break;

        // Get next item from queue
        const text = ttsQueue.shift();

        if (!text) {
            // Queue empty - check if streaming is still active
            if (streamingTTSActive) {
                // Wait a bit for more items to arrive
                await new Promise(r => setTimeout(r, 100));
                continue;
            } else {
                // Streaming finished and queue empty - we're done
                break;
            }
        }

        // Start prefetching next items while we process current
        for (let i = 0; i < Math.min(2, ttsQueue.length); i++) {
            prefetchTTSAudio(ttsQueue[i]);
        }

        // Get current audio
        let audioUrl = null;
        let playbackBundle = null;
        try {
            const audioUrlPromise = prefetchTTSAudio(text);
            audioUrl = await audioUrlPromise;
        } catch (e) {
            console.error("Prefetch failed", e);
        }

        // Remove from cache after getting
        ttsAudioCache.delete(text);

        if (!audioUrl) {
            continue; // Skip failed chunks
        }

        // Check cancellation again
        if (sessionId !== ttsSessionId) {
            URL.revokeObjectURL(audioUrl);
            break;
        }

        // Play audio using HTML5 Audio (Better for iOS Background)
        try {
            if (!firstChunkPlayed && mediaSessionLabel) {
                updateMediaSessionMetadata(mediaSessionLabel);
            }

            if (!currentAudio) {
                currentAudio = new Audio();
                currentAudio.playsInline = true;
                currentAudio.preload = 'auto';
            }

            playbackBundle = await combinePlayableChunks(audioUrl, [...ttsQueue]);
            const playbackUrl = playbackBundle?.url || audioUrl;

            // Update UI on first chunk playing
            if (!firstChunkPlayed && btn) {
                btn.disabled = false;
                const iconEl = btn.querySelector('.material-icons-round');
                if (iconEl) iconEl.textContent = 'stop';
                btn.title = "Stop";
                firstChunkPlayed = true;
            }

            // Play via Audio element
            await new Promise((resolve, reject) => {
                const onEnded = () => {
                    currentAudio.removeEventListener('ended', onEnded);
                    currentAudio.removeEventListener('error', onError);
                    resolve();
                };
                const onError = (e) => {
                    console.error("Audio element error:", e);
                    currentAudio.removeEventListener('ended', onEnded);
                    currentAudio.removeEventListener('error', onError);
                    reject(e);
                };

                currentAudio.addEventListener('ended', onEnded);
                currentAudio.addEventListener('error', onError);

                // Check cancellation before starting
                if (sessionId !== ttsSessionId) {
                    resolve();
                    return;
                }

                currentAudio.src = playbackUrl;
                currentAudio.play().catch(reject);
            });

        } catch (e) {
            console.error("Playback failed for chunk:", e);
        } finally {
            if (playbackBundle?.revokeInputs) {
                for (const url of playbackBundle.revokeInputs) {
                    URL.revokeObjectURL(url);
                }
            } else if (audioUrl) {
                URL.revokeObjectURL(audioUrl);
            }

            if (playbackBundle?.url && playbackBundle.url !== audioUrl) {
                URL.revokeObjectURL(playbackBundle.url);
            }
        }
    }

    // Finished or Cancelled
    if (sessionId === ttsSessionId) {
        endTTS(btn, sessionId);
    }
}

/**
 * Reset TTS UI state after playback completes
 */
function endTTS(btn, sessionId) {
    // Only update UI if we are still in the same session
    if (sessionId === ttsSessionId) {
        if (btn) {
            const iconEl = btn.querySelector('.material-icons-round');
            if (iconEl) iconEl.textContent = 'volume_up';
            btn.title = 'Speak';
            btn.disabled = false;
        }
        currentAudioBtn = null;
        isPlayingQueue = false;
        activeTTSSessionLabel = "";
        clearMediaSessionMetadata();
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

async function checkSystemHealth() {
    let health;

    // 1. Try Wails (Desktop)
    if (typeof window.go !== 'undefined' && window.go.main && window.go.main.App) {
        try {
            health = await window.go.main.App.CheckHealth();
        } catch (e) {
            console.error("Wails health check failed:", e);
        }
    }

    // 2. Fallback to API (Web Mode)
    if (!health) {
        let retries = 3;
        while (retries > 0 && !health) {
            try {
                const res = await fetch('/api/health', {
                    headers: { 'Cache-Control': 'no-cache' }
                });
                if (res.ok) {
                    health = await res.json();
                    break;
                }
            } catch (e) {
                console.error(`API health check failed (${retries} retries left):`, e);
            }
            retries--;
            if (retries > 0) {
                await new Promise(r => setTimeout(r, 1000));
            }
        }
    }

    if (!health) {
        const errorMsg = {
            role: 'assistant',
            content: `### ❌ ${t('health.checkFailed')}\n\n${t('health.backendError')}`
        };
        appendMessage(errorMsg);
        return;
    }

    try {
        lastSessionCache = await fetchLastSession();
        const issues = [];

        if (health.llmStatus !== 'ok') {
            let errorDetail = health.llmMessage;
            if (errorDetail.includes('401')) {
                errorDetail += t('health.checkToken');
            } else if (errorDetail.includes('connect') || errorDetail.includes('refused')) {
                errorDetail += t('health.checkServer');
            }
            issues.push(`${t('health.llm')}: ${errorDetail}`);
        }

        if (health.ttsStatus !== 'ok') {
            if (health.ttsStatus !== 'disabled') {
                issues.push(`${t('health.tts')}: ${health.ttsMessage}`);
            }
        }

        const healthMsg = {
            role: 'assistant',
            startup: {
                title: issues.length === 0 ? t('chat.startup.welcomeTitle') : t('health.checkRequired'),
                body: issues.length === 0 ? t('chat.startup.welcomeBody') : t('chat.startup.issueBody'),
                issues,
                showRestoreButton: !!lastSessionCache,
                restoreLabel: t('chat.startup.restore')
            }
        };

        appendMessage(healthMsg);

    } catch (e) {
        console.error("Health check rendering failed:", e);
    }
}

/**
 * Updates UI layout based on mic layout setting
 */
function updateMicLayout() {
    const container = document.getElementById('mic-layout-container');
    if (!container) return;

    // Reset classes
    container.className = '';
    document.body.classList.remove('layout-mic-bottom');
    if (inlineMicBtn) {
        inlineMicBtn.classList.remove('is-visible');
    }

    if (!config.micLayout || config.micLayout === 'none') {
        container.style.display = 'none';
    } else if (config.micLayout === 'inline') {
        container.style.display = 'none';
        if (inlineMicBtn) {
            inlineMicBtn.classList.add('is-visible');
        }
    } else {
        container.style.display = 'flex';
        container.classList.add(`mic-layout-${config.micLayout}`);
        if (config.micLayout === 'bottom') {
            document.body.classList.add('layout-mic-bottom');
        }
    }

    updateMicUIForGeneration(isGenerating);
    syncMicRecordingUI();
    updateInlineComposerActionVisibility();
}

// Global STT state
let recognition = null;
let isSTTActive = false;
let sttPlaceholderTimer = null;
let sttPlaceholderIndex = 0;

/**
 * Toggles Speech-to-Text (STT) recognition
 */
function toggleSTT() {
    // 1. If generating response, stop it (Mic acts as stop button)
    if (isGenerating) {
        stopGeneration();
        return;
    }

    // 2. If TTS is currently playing, stop it (interruption support)
    if (isPlayingQueue) {
        stopAllAudio();
    }

    // 3. STT Logic
    if (isSTTActive) {
        stopSTT();
    } else {
        startSTT();
    }
}

function startSTT() {
    if (!('webkitSpeechRecognition' in window)) {
        alert("Speech Recognition is not supported by this browser.");
        return;
    }

    if (!recognition) {
        recognition = new webkitSpeechRecognition();
        recognition.continuous = false;
        recognition.interimResults = true;

        recognition.lang = config.language === 'ko' ? 'ko-KR' : 'en-US';

        recognition.onstart = () => {
            isSTTActive = true;
            startSTTPlaceholderAnimation();
            syncMicRecordingUI();
            console.log("[STT] Recording started");
        };

        recognition.onresult = (event) => {
            let interimTranscript = '';
            let finalTranscript = '';

            for (let i = event.resultIndex; i < event.results.length; ++i) {
                if (event.results[i].isFinal) {
                    finalTranscript += event.results[i][0].transcript;
                } else {
                    interimTranscript += event.results[i][0].transcript;
                }
            }

            if (finalTranscript || interimTranscript) {
                const input = document.getElementById('message-input');
                // Real-time update message input
                input.value = finalTranscript || interimTranscript;
                input.dispatchEvent(new Event('input')); // Auto-resize textarea
                updateInlineComposerActionVisibility();
            }
        };

        recognition.onerror = (event) => {
            console.error("[STT] Error:", event.error);
            stopSTTPlaceholderAnimation();
            stopSTT();
        };

        recognition.onend = () => {
            isSTTActive = false;
            stopSTTPlaceholderAnimation();
            syncMicRecordingUI();
            console.log("[STT] Recording ended");

            // Auto-send if there is content
            const input = document.getElementById('message-input');
            if (input.value.trim()) {
                sendMessage();
            }
        };
    }

    recognition.start();
}

function stopSTT() {
    if (recognition) {
        recognition.stop();
    }
}

function updateMessageInputPlaceholder() {
    if (!messageInput) return;

    const listeningPhrases = [
        t('input.placeholder.sttA'),
        t('input.placeholder.sttB')
    ];
    const nextPlaceholder = isSTTActive
        ? listeningPhrases[sttPlaceholderIndex % listeningPhrases.length]
        : (composerProgressActive
            ? [composerProgressLabel, composerProgressPercent].filter(Boolean).join(' - ')
            : t('input.placeholder'));

    messageInput.placeholder = nextPlaceholder;
    messageInput.classList.toggle('stt-listening', isSTTActive);
}

function startSTTPlaceholderAnimation() {
    stopSTTPlaceholderAnimation();
    sttPlaceholderIndex = 0;
    updateMessageInputPlaceholder();
    sttPlaceholderTimer = window.setInterval(() => {
        sttPlaceholderIndex = (sttPlaceholderIndex + 1) % 2;
        updateMessageInputPlaceholder();
    }, 1400);
}

function stopSTTPlaceholderAnimation() {
    if (sttPlaceholderTimer) {
        window.clearInterval(sttPlaceholderTimer);
        sttPlaceholderTimer = null;
    }
    sttPlaceholderIndex = 0;
    updateMessageInputPlaceholder();
}

function syncMicRecordingUI() {
    const giantMicBtn = document.getElementById('giant-mic-btn');
    if (giantMicBtn) {
        giantMicBtn.classList.toggle('stt-active', isSTTActive);
    }
    if (inlineMicBtn) {
        inlineMicBtn.classList.toggle('stt-active', isSTTActive);
    }
}


/**
 * Hook into global state to update giant mic icon if generating
 */
function updateMicUIForGeneration(generating) {
    const giantMicBtn = document.getElementById('giant-mic-btn');
    const isInlineLayout = config.micLayout === 'inline';

    if (giantMicBtn) {
        giantMicBtn.classList.toggle('gen-active', generating && !isInlineLayout);
        const giantIcon = giantMicBtn.querySelector('.material-icons-round');
        if (giantIcon) {
            giantIcon.textContent = generating && !isInlineLayout ? 'stop' : 'mic';
        }
    }

    if (inlineMicBtn) {
        inlineMicBtn.classList.toggle('gen-active', false);
        const inlineIcon = inlineMicBtn.querySelector('.material-icons-round');
        if (inlineIcon) {
            inlineIcon.textContent = 'mic';
        }
    }

    const micContainer = document.getElementById('mic-layout-container');
    if (micContainer) {
        if (generating && !isInlineLayout) {
            micContainer.style.pointerEvents = '';
            micContainer.style.opacity = '';
        } else {
            micContainer.style.pointerEvents = '';
            micContainer.style.opacity = '';
        }
    }

    if (!generating) {
        syncMicRecordingUI();
    }
}



