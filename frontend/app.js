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
    micLayout: 'none' // 'none', 'left', 'right', 'bottom'
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
        'setting.memory.warning': '주의: 개인 정보가 PC에 평문으로 저장됩니다.',
        'setting.memory.open': '파일 열기',
        'setting.memory.reset': '메모리 초기화',
        'setting.memory.reset.confirm': '개인 메모리를 초기화하시겠습니까? 이 작업은 되돌릴 수 없습니다.',
        'setting.memory.reset.success': '메모리가 초기화되었습니다.',
        'setting.micLayout.label': '마이크 레이아웃',
        'setting.micLayout.desc': '화면에 거대 마이크 버튼을 배치합니다.',
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
        // Chat
        'chat.welcome': '안녕하세요! 채팅할 준비가 되었습니다. 우측 상단 기어 아이콘에서 설정하세요.',
        'chat.instruction': '우측 상단 설정(⚙️)에서 설정을 변경하실 수 있습니다.',
        'input.placeholder': '메시지를 입력하세요...',
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
        'setting.memory.warning': 'Warning: Personal data is stored unencrypted on local disk.',
        'setting.memory.open': 'Open File',
        'setting.memory.reset': 'Reset Memory',
        'setting.memory.reset.confirm': 'Are you sure you want to reset your personal memory? This cannot be undone.',
        'setting.memory.reset.success': 'Memory reset successfully.',
        'setting.micLayout.label': 'Mic Layout',
        'setting.micLayout.desc': 'Place a giant microphone button on the screen.',
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
        // Chat
        'chat.welcome': 'Hello! I am ready to chat. Configure settings using the gear icon.',
        'chat.instruction': 'You can configure settings in the top right menu.',
        'input.placeholder': 'Type a message...',
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

// Chat State
let messages = [];
let pendingImage = null;
let isGenerating = false;
let abortController = null;
let lastResponseId = null; // For Stateful Chat

// Audio State
let currentAudio = null;
let currentAudioBtn = null;
let audioWarmup = null; // Used to bypass auto-play blocks
let ttsQueue = [];

let isPlayingQueue = false;

// Streaming TTS State
let streamingTTSActive = false;
let streamingTTSCommittedIndex = 0; // How much of the display text has been sent to TTS
let streamingTTSBuffer = ""; // Uncommitted text buffer
let streamingTTSProcessor = null; // Reference to the active processor loop
let ttsSessionId = 0;


// DOM Elements
const chatMessages = document.getElementById('chat-messages');
const messageInput = document.getElementById('message-input');
const sendBtn = document.getElementById('send-btn');
const imagePreviewVal = document.getElementById('image-preview');
const previewContainer = document.getElementById('preview-container');

// Audio Context for Auto-play
let audioContextUnlocked = false;
let audioCtx = null;
let currentSource = null;

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
}


// Initialization
document.addEventListener('DOMContentLoaded', async () => {
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


    // Setup Markdown
    marked.setOptions({
        highlight: function (code, lang) {
            const language = highlight.getLanguage(lang) ? lang : 'plaintext';
            return highlight.highlight(code, { language }).value;
        },
        langPrefix: 'hljs language-'
    });
});

// Current user state
let currentUser = null;

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
    const autoSaveIds = ['cfg-api', 'cfg-tts-lang', 'cfg-tts-voice', 'cfg-tts-format', 'cfg-chunk-size', 'cfg-system-prompt', 'cfg-llm-mode', 'cfg-disable-stateful'];
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
    config.micLayout = document.getElementById('cfg-mic-layout').value;

    // Update visibility immediately
    updateSettingsVisibility();
    updateMicLayout();

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

    messageInput.addEventListener('input', autoResizeInput);

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
}

function handleImageUpload(input) {
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function (e) {
            pendingImage = e.target.result; // Base64 string
            imagePreviewVal.src = pendingImage;
            previewContainer.style.display = 'block';
        };
        reader.readAsDataURL(input.files[0]);
    }
}

function removeImage() {
    pendingImage = null;
    document.getElementById('image-upload').value = '';
    previewContainer.style.display = 'none';
}

function clearChat() {
    // Stop any TTS playback and generation
    stopAllAudio();

    if (isGenerating) {
        stopGeneration();
    }

    lastResponseId = null; // Clear session ID for Stateful Chat
    messages = [];
    chatMessages.innerHTML = '';
}


/* Chat Logic */

async function sendMessage() {
    // Unlock audio context on user interaction
    unlockAudioContext();

    let text = messageInput.value.trim();
    const currentImage = pendingImage; // Capture early

    if (!text && !currentImage) return;
    if (isGenerating) return;

    // Default text for vision if empty
    if (!text && currentImage) {
        text = (config.language === 'ko') ? '이미지를 설명해주세요.' : 'Please describe this image.';
    }

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

    // Reset Input
    messageInput.value = '';
    removeImage();
    autoResizeInput();

    // Prepare Assistant Placeholder
    isGenerating = true;
    updateSendButtonState();

    // Create new AbortController
    abortController = new AbortController();

    const assistantId = 'msg-' + Date.now();
    appendMessage({ role: 'assistant', content: '', id: assistantId });

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
        const inputData = currentImage ? [
            { type: 'text', content: text },
            { type: 'image', data_url: currentImage }
        ] : text;

        payload = {
            model: config.model,
            input: inputData,
            system_prompt: config.systemPrompt,
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
                return {
                    role: m.role,
                    content: [
                        { type: 'text', text: m.content || 'Please describe this image.' },
                        { type: 'image_url', image_url: { url: m.image } }
                    ]
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

    try {
        await streamResponse(payload, assistantId);
    } catch (e) {
        if (e.name === 'AbortError') {
            updateMessageContent(assistantId, `**[Stopped by User]**`);
        } else {
            updateMessageContent(assistantId, `**Error:** ${e.message}`);
        }
    } finally {
        isGenerating = false;
        abortController = null;
        updateSendButtonState();
    }
}

function stopGeneration() {
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
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
}


async function streamResponse(payload, elementId) {
    // Use the Go server's API endpoint
    const response = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
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

            if (errorBody.includes("Could not find stored response for previous_response_id")) {
                console.warn("[Stateful] previous_response_id became invalid. Resetting and retrying without it...");
                lastResponseId = null;
                // Re-attempt without current lastResponseId
                delete payload.previous_response_id;
                return await streamResponse(payload, elementId);
            }

            errorDetails += ` - ${errorBody}`;

        }
        throw new Error(errorDetails);
    }

    await processStream(response, elementId);
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
                        const toolName = json.tool || 'Running...';
                        // Store the exact HTML to be able to replace it later
                        lastToolCallHtml = `<span class="tool-status" style="font-size: small; color: gray; display: block; margin: 4px 0;">🛠️ Tool Call: ${toolName}</span>`;
                        contentToAdd = lastToolCallHtml;
                    }
                    else if (json.type === 'tool_call.arguments' && json.arguments) {
                        // Update the last tool call line with arguments
                        const toolName = json.tool || 'Tool';
                        let argsDisplay = '';

                        // Extract specific interesting fields (like 'query' for search)
                        if (json.arguments.query) {
                            argsDisplay = ` <span style="font-size: 0.9em;">(Query: "${json.arguments.query}")</span>`;
                        } else {
                            // Generic fallback for other args? 
                            // Only show if small, otherwise might be huge JSON.
                            // For now, prioritize 'query'.
                            const keys = Object.keys(json.arguments);
                            if (keys.length === 1 && typeof json.arguments[keys[0]] === 'string') {
                                argsDisplay = ` <span style="font-size: 0.9em;">(${keys[0]}: "${json.arguments[keys[0]]}")</span>`;
                            }
                        }

                        if (lastToolCallHtml && argsDisplay) {
                            const newToolCallHtml = `<span class="tool-status" style="font-size: small; color: gray; display: block; margin: 4px 0;">🛠️ Tool Call: ${toolName}${argsDisplay}</span>`;

                            // Replace in fullText
                            if (fullText.endsWith(lastToolCallHtml)) {
                                // Calculate start index of the tool call line
                                const startIndex = fullText.length - lastToolCallHtml.length;

                                fullText = fullText.slice(0, -lastToolCallHtml.length) + newToolCallHtml;
                                lastToolCallHtml = newToolCallHtml; // Update for next chunk
                                contentToAdd = null; // Already updated fullText manually

                                // Rewind TTS index if it has already passed the start of this line
                                // This ensures we re-process the full tag so the regex can strip it cleanly
                                if (typeof streamingTTSCommittedIndex !== 'undefined' && streamingTTSCommittedIndex > startIndex) {
                                    // console.log(`[TTS Debug] Rewinding index from ${streamingTTSCommittedIndex} to ${startIndex}`);
                                    streamingTTSCommittedIndex = startIndex;
                                    if (typeof streamingTTSBuffer !== 'undefined') {
                                        streamingTTSBuffer = ""; // Clear buffer to avoid duplication
                                    }
                                }

                                // Force UI update since we modified fullText directly
                                let displayText = fullText;
                                if (config.hideThink) {
                                    // ... logic duplicated from below ...
                                    displayText = displayText.replace(/<think>[\s\S]*?<\/think>/g, '');
                                    if (displayText.includes('</think>')) displayText = displayText.split('</think>').pop().trim();
                                    if (displayText.includes('<think>')) displayText = displayText.split('<think>')[0];
                                }
                                updateMessageContent(elementId, displayText);
                            }
                        }
                    }
                    else if (json.type === 'tool_call.success') {
                        contentToAdd = `<span class="tool-status" style="font-size: small; color: gray; display: block; margin: 4px 0;">✅ Tool Finished</span>\n`;
                    }
                    else if (json.type === 'tool_call.failure') {
                        contentToAdd = `<span class="tool-status" style="font-size: small; color: #ff6b6b; display: block; margin: 4px 0;">❌ Tool Failed: ${json.reason || 'Unknown error'}</span>\n`;
                    }
                    else if (json.type === 'chat.end' && json.result && json.result.response_id) {
                        lastResponseId = json.result.response_id;
                        console.log(`[Stateful] Captured response_id from chat.end: ${lastResponseId}`);
                    }
                    // Handle Prompt Processing Progress
                    else if (json.type === 'prompt_processing.progress') {
                        // console.log('[Progress Debug] Event Received:', json.progress);
                        const progress = (json.progress * 100).toFixed(2);
                        const progressId = `progress-${elementId}`;
                        let progressEl = document.getElementById(progressId);

                        // FIX: elementId already includes 'msg-' prefix
                        let msgBubble = document.getElementById(elementId);
                        // console.log('[Progress Debug] MsgBubble:', msgBubble ? 'Found' : 'Not Found');

                        if (msgBubble) {
                            if (!progressEl) {
                                // console.log('[Progress Debug] Creating new progress element');
                                progressEl = document.createElement('div');
                                progressEl.id = progressId;
                                progressEl.className = 'progress-status';
                                progressEl.style.fontSize = '0.8em';
                                progressEl.style.color = '#888';
                                progressEl.style.marginTop = '4px';
                                // progressEl.style.fontStyle = 'italic';
                                const bubbleContent = msgBubble.querySelector('.message-bubble');
                                if (bubbleContent) {
                                    bubbleContent.append(progressEl); // Append to bottom
                                }
                            }
                            progressEl.textContent = `⏳ Processing Prompt: ${progress}%`;
                        }
                    }
                    // Handle Model Loading Progress (LM Studio Mode)
                    else if (json.type === 'model_load.start') {
                        console.log('[Model Load] Start:', json.model_instance_id);
                        const loadId = `model-load-${elementId}`;
                        let loadEl = document.getElementById(loadId);
                        let msgBubble = document.getElementById(elementId);

                        if (msgBubble && !loadEl) {
                            loadEl = document.createElement('div');
                            loadEl.id = loadId;
                            loadEl.className = 'model-load-status';
                            loadEl.innerHTML = `
                                <div style="font-size: 0.8em; color: #888; margin-bottom: 8px;">
                                    <span>📦 Loading Model...</span>
                                    <div style="background: var(--border-color); border-radius: 4px; height: 6px; margin-top: 4px; overflow: hidden;">
                                        <div class="model-load-bar" style="background: var(--accent-color); height: 100%; width: 0%; transition: width 0.2s;"></div>
                                    </div>
                                    <span class="model-load-percent" style="font-size: 0.9em;">0%</span>
                                </div>`;
                            const bubbleContent = msgBubble.querySelector('.message-bubble');
                            if (bubbleContent) {
                                bubbleContent.append(loadEl); // Append to bottom
                            }
                        }
                    }
                    else if (json.type === 'model_load.progress') {
                        const loadId = `model-load-${elementId}`;
                        const loadEl = document.getElementById(loadId);
                        if (loadEl) {
                            const percent = (json.progress * 100).toFixed(1);
                            const bar = loadEl.querySelector('.model-load-bar');
                            const percentText = loadEl.querySelector('.model-load-percent');
                            if (bar) bar.style.width = `${percent}%`;
                            if (percentText) percentText.textContent = `${percent}%`;
                        }
                    }
                    else if (json.type === 'model_load.end') {
                        console.log('[Model Load] End:', json.model_instance_id, 'Time:', json.load_time_seconds);
                        const loadId = `model-load-${elementId}`;
                        const loadEl = document.getElementById(loadId);
                        if (loadEl) {
                            // Show completion briefly then remove
                            const bar = loadEl.querySelector('.model-load-bar');
                            const percentText = loadEl.querySelector('.model-load-percent');
                            if (bar) bar.style.width = '100%';
                            if (percentText) percentText.textContent = `✓ Loaded (${json.load_time_seconds?.toFixed(1) || '?'}s)`;
                            setTimeout(() => loadEl.remove(), 1500);
                        }
                    }
                    // Handle Generative Errors (Tool Parsing, etc.)
                    else if (json.type === 'error') {
                        console.error('[SSE Error]', json.error);
                        let errMsg = "Unknown Error";
                        if (json.error && json.error.message) {
                            errMsg = json.error.message;
                        } else if (typeof json.error === 'string') {
                            errMsg = json.error;
                        }

                        // If we are waiting for a tool call (lastToolCallHtml exists), update it
                        if (lastToolCallHtml) {
                            contentToAdd = `<span class="tool-status" style="font-size: small; color: #ff6b6b; display: block; margin: 4px 0;">❌ Error: ${errMsg}</span>\n`;
                        } else {
                            // Otherwise just append as text with error styling
                            contentToAdd = `\n\n**Error:** ${errMsg}\n`;
                        }
                    }

                    // If we have content, remove the progress indicator
                    if (contentToAdd) {
                        // Remove progress indicator if exists
                        const progressId = `progress-${elementId}`;
                        const progressEl = document.getElementById(progressId);
                        if (progressEl) {
                            progressEl.remove();
                        }

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
            console.log('Stream aborted by user');
        } else {
            console.error('Stream Error:', err);
            throw err; // Re-throw other errors
        }
    } finally {
        // Finalize (Save to history even if aborted)
        // Combine reasoning and content for history (reasoning first, then response)
        const historyContent = reasoningBuffer + fullText;
        if (historyContent) {
            messages.push({ role: 'assistant', content: historyContent });
        }


        // Finalize streaming TTS (commit any remaining text)
        if (useStreamingTTS) {
            finalizeStreamingTTS(speechBuffer); // Pass final speech buffer
        }
        releaseWakeLock(); // Release screen lock after generation and TTS streaming is done
    }
}

function appendMessage(msg) {
    const div = document.createElement('div');
    div.className = `message ${msg.role}`;
    if (msg.id) div.id = msg.id;

    let innerHtml = '';

    // Wrapper start
    innerHtml += `<div class="message-inner">`;

    // Bubble content
    let bubbleContent = '';
    if (msg.image) {
        bubbleContent += `<img src="${msg.image}" class="message-image">`;
    }

    const textContent = msg.content || '';
    if (msg.role === 'user') {
        bubbleContent += `<div class="message-bubble">${escapeHtml(textContent)}</div>`;
    } else {
        bubbleContent += `<div class="message-bubble"><div class="markdown-body">${marked.parse(textContent)}</div></div>`;
    }

    innerHtml += bubbleContent;

    // Action Bar (for assistant only)
    if (msg.role === 'assistant') {
        innerHtml += `
            <div class="message-actions">
                <button class="icon-btn copy-btn" onclick="copyMessage(this)" title="Copy">
                    <span class="material-icons-round">content_copy</span>
                </button>
                <button class="icon-btn speak-btn" onclick="speakMessageFromBtn(this)" title="Speak">
                    <span class="material-icons-round">volume_up</span>
                </button>
            </div>`;
    }

    // Wrapper end
    innerHtml += `</div>`;

    div.innerHTML = innerHtml;
    chatMessages.appendChild(div);
    scrollToBottom();
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

/**
 * Stop all audio playback and clear queues
 */
function stopAllAudio() {
    // Clear queues
    ttsQueue = [];
    audioWarmup = null;

    // Stop current audio source
    if (currentSource) {
        try {
            currentSource.stop();
        } catch (e) {
            // Ignore errors if already stopped
        }
        currentSource = null;
    }

    // Clear audio cache to free memory
    clearTTSAudioCache();
    releaseWakeLock(); // Release lock on stop

    // Reset loop state
    isPlayingQueue = false;

    // Cancel streaming
    streamingTTSActive = false;
    streamingTTSBuffer = "";
    streamingTTSCommittedIndex = 0;

    // Increment session ID to invalidate pending ops
    ttsSessionId++;

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
    const statusId = `reasoning-${elementId}`;
    let statusEl = document.getElementById(statusId);
    const msgBubble = document.getElementById(elementId);

    if (!msgBubble || config.hideThink) return;

    const bubbleContent = msgBubble.querySelector('.message-bubble');
    if (!bubbleContent) return;

    if (isFinal) {
        if (statusEl) statusEl.remove();
        return;
    }

    if (!statusEl) {
        statusEl = document.createElement('div');
        statusEl.id = statusId;
        statusEl.className = 'reasoning-status';
        statusEl.style.fontSize = '0.8em';
        statusEl.style.color = '#888';
        statusEl.style.marginTop = '4px';
        statusEl.style.overflow = 'hidden';
        statusEl.style.whiteSpace = 'pre-wrap';      // Allow wrapping
        statusEl.style.wordBreak = 'break-word';     // Break long words
        statusEl.style.maxWidth = '100%';            // Force width constraint
        statusEl.style.display = 'block';
        // statusEl.style.textOverflow = 'ellipsis'; // Not needed with wrapping
        bubbleContent.append(statusEl);
    }

    // Fixed-length style matching "⏳ Processing Prompt: XX.XX%"
    // Increased length to show more context as requested
    const MAX_DISPLAY_LENGTH = 300;
    const prefix = '💭 Thinking: ';

    if (text) {
        // Get last part of accumulated reasoning (most recent)
        // cleanText might contain <think> tags, remove them for cleaner display
        let cleanText = text.replace(/<think>/g, '').replace(/[\r\n]+/g, ' ').trim();

        // Use tail truncation (show the end) instead of head truncation
        const truncated = cleanText.length > MAX_DISPLAY_LENGTH
            ? '...' + cleanText.slice(-MAX_DISPLAY_LENGTH)
            : cleanText;
        statusEl.textContent = prefix + truncated;
    } else {
        statusEl.textContent = prefix + '...';
    }
}


function updateMessageContent(id, text) {
    const el = document.getElementById(id);
    if (!el) return;

    const bubble = el.querySelector('.message-bubble');
    let mdBody = bubble.querySelector('.markdown-body');
    if (!mdBody) {
        // Create if doesn't exist (ensure text stays at top if other status elements exist)
        mdBody = document.createElement('div');
        mdBody.className = 'markdown-body';
        bubble.prepend(mdBody);
    }

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


    mdBody.innerHTML = marked.parse(cleanText);

    // Make all links open in new window/tab
    mdBody.querySelectorAll('a').forEach((link) => {
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
    });

    // Re-highlight code blocks
    mdBody.querySelectorAll('pre code').forEach((block) => {
        highlight.highlightElement(block);
    });

    scrollToBottom();
}


function scrollToBottom() {
    chatMessages.scrollTop = chatMessages.scrollHeight;
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

    let cleaned = text;

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

    // Remove Header Hashes (# Title -> Title.)
    // Add period if missing to ensure pause
    cleaned = cleaned.replace(/^(#{1,6})\s+(.+?)([.!?]?)$/gm, '$2$3.');

    // Remove Bold/Italic wrappers (**text**, __text__, *text*, _text_)
    cleaned = cleaned.replace(/(\*\*|__)(.*?)\1/g, '$2');
    cleaned = cleaned.replace(/(\*|_)(.*?)\1/g, '$2');

    // Remove Blockquotes (>)
    cleaned = cleaned.replace(/^>\s+/gm, '');
    // Remove Horizontal Rules (---)
    cleaned = cleaned.replace(/^([-*_]){3,}\s*$/gm, '.');
    // Remove List Markers (-, *, 1.)
    cleaned = cleaned.replace(/^\s*[-*+]\s+/gm, '');
    cleaned = cleaned.replace(/^\s*\d+\.\s+/gm, '');

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

    // Normalize Whitespace
    cleaned = cleaned.replace(/\r\n/g, '\n');
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n'); // Max 2 newlines
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

    // Get speak button for UI updates
    const msgEl = document.getElementById(elementId);
    if (msgEl) {
        currentAudioBtn = msgEl.querySelector('.speak-btn');
    }

    console.log("[Streaming TTS] Initialized");
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
        if (!newText || newText.length < 5) break; // Need at least some content

        let committed = null;
        let advanceBy = 0;

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

        // PRIORITY 2: Newlines (Fast response)
        if (!committed) {
            const newlineMatch = newText.match(/^([\s\S]*?\n)/);
            if (newlineMatch && newlineMatch[1].trim()) {
                const potentialCommit = streamingTTSBuffer + cleanTextForTTS(newlineMatch[1]);
                if (potentialCommit.length >= 10 || streamingTTSBuffer.length > 0) {
                    committed = potentialCommit;
                    streamingTTSBuffer = "";
                    advanceBy = newlineMatch[1].length;
                } else {
                    streamingTTSBuffer = potentialCommit + " "; // Keep buffering
                    streamingTTSCommittedIndex += newlineMatch[1].length;
                    continue;
                }
            }
        }

        // PRIORITY 3: Sentence Endings (.!?)
        if (!committed) {
            const sentenceMatch = newText.match(/^([\s\S]*?[.!?])(\s+[A-Z가-힣])/);
            if (sentenceMatch && sentenceMatch[1].trim()) {
                const potentialCommit = streamingTTSBuffer + cleanTextForTTS(sentenceMatch[1]);
                if (potentialCommit.length >= config.chunkSize) {
                    committed = potentialCommit;
                    streamingTTSBuffer = "";
                    advanceBy = sentenceMatch[1].length;
                } else {
                    streamingTTSBuffer = potentialCommit + " ";
                    streamingTTSCommittedIndex += sentenceMatch[1].length;
                    continue;
                }
            }
        }      // If nothing matched, stop the loop
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

    const MIN_CHUNK_LENGTH = 50;

    // Split into smaller chunks if needed (by paragraph/sentence within the segment)
    const paragraphs = text.split(/\n\s*\n+/);
    const newChunks = [];

    for (const para of paragraphs) {
        if (!para.trim()) continue;

        // Smart sentence splitting - avoid breaking on numbered lists like "1.", "2."
        const sentencePattern = /(?<=[.!?])(?=\s+(?:[A-Z가-힣]|$))/g;
        const rawChunks = para.split(sentencePattern).filter(s => s.trim());
        const sentences = rawChunks.length > 0 ? rawChunks : [para];

        let currentChunk = "";

        for (const part of sentences) {
            const trimmedPart = part.trim();
            if (!trimmedPart) continue;

            // If adding this part exceeds chunkSize and we have content, queue current chunk
            if ((currentChunk + " " + trimmedPart).length > config.chunkSize && (currentChunk.length >= MIN_CHUNK_LENGTH || force)) {
                // Only add if it has actual speakable content
                if (/[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(currentChunk)) {
                    ttsQueue.push(currentChunk.trim());
                    newChunks.push(currentChunk.trim());
                }
                currentChunk = "";
            }
            currentChunk = currentChunk ? currentChunk + " " + trimmedPart : trimmedPart;
        }

        // Queue paragraph remainder if long enough OR forced
        if ((currentChunk.length >= MIN_CHUNK_LENGTH || force) && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(currentChunk)) {
            ttsQueue.push(currentChunk.trim());
            newChunks.push(currentChunk.trim());
            currentChunk = "";
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

        // Play audio using Web Audio API
        try {
            // Unlock audio context if needed (last ditch effort)
            await unlockAudioContext();

            if (!audioCtx) {
                throw new Error("AudioContext not available");
            }

            // Fetch blob data
            const response = await fetch(audioUrl);
            const arrayBuffer = await response.arrayBuffer();

            // Decode audio data
            const audioBuffer = await audioCtx.decodeAudioData(arrayBuffer);

            // Update UI on first chunk playing
            if (!firstChunkPlayed && btn) {
                btn.disabled = false;
                const iconEl = btn.querySelector('.material-icons-round');
                if (iconEl) iconEl.textContent = 'stop';
                btn.title = "Stop";
                firstChunkPlayed = true;
            }

            // Create source and play
            await new Promise((resolve, reject) => {
                // Check cancellation before starting
                if (sessionId !== ttsSessionId) {
                    resolve();
                    return;
                }

                const source = audioCtx.createBufferSource();
                source.buffer = audioBuffer;
                source.connect(audioCtx.destination);
                currentSource = source;

                source.onended = () => {
                    currentSource = null;
                    resolve();
                };

                source.start(0);
            });

        } catch (e) {
            console.error("Playback failed for chunk:", e);
        } finally {
            URL.revokeObjectURL(audioUrl);
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
        try {
            const res = await fetch('/api/health');
            if (res.ok) {
                health = await res.json();
            }
        } catch (e) {
            console.error("API health check failed:", e);
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
        let statusIcon = "✅";
        let statusTitle = t('health.systemReady');
        let statusDetails = "";

        // Display Current Mode
        const modeLabel = config.llmMode === 'stateful' ? 'LM Studio' : 'OpenAI Compatible';
        statusDetails += `\n- **${t('health.mode')}**: ${modeLabel}`;

        // Analyze health
        if (health.llmStatus !== 'ok') {
            statusIcon = "⚠️";
            statusTitle = t('health.checkRequired');

            let errorDetail = health.llmMessage;
            if (errorDetail.includes('401')) {
                errorDetail += t('health.checkToken');
            } else if (errorDetail.includes('connect') || errorDetail.includes('refused')) {
                errorDetail += t('health.checkServer');
            }

            statusDetails += `\n- **${t('health.llm')}**: ${errorDetail}`;
        } else {
            // Translate "Connected" if exact match, otherwise keep
            let llmDisplay = health.llmMessage === 'Connected' ? t('health.status.connected') : health.llmMessage;
            statusDetails += `\n- **${t('health.llm')}**: ${llmDisplay}`;
            if (health.serverModel) {
                statusDetails += ` (${health.serverModel})`;
            }
        }

        if (health.ttsStatus !== 'ok') {
            if (health.ttsStatus === 'disabled') {
                statusDetails += `\n- **${t('health.tts')}**: ${t('health.status.disabled')}`;
            } else {
                statusIcon = "⚠️";
                if (statusTitle === t('health.systemReady')) statusTitle = t('health.checkRequired');
                statusDetails += `\n- **${t('health.tts')}**: ${health.ttsMessage}`;
            }
        } else {
            statusDetails += `\n- **${t('health.tts')}**: ${t('health.status.ready')}`;
        }

        const healthMsg = {
            role: 'assistant',
            content: `### ${statusIcon} ${statusTitle}\n${statusDetails}\n\n${t('chat.instruction') || 'You can configure settings in the top right menu.'}`
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

    if (!config.micLayout || config.micLayout === 'none') {
        container.style.display = 'none';
    } else {
        container.style.display = 'flex';
        container.classList.add(`mic-layout-${config.micLayout}`);
        if (config.micLayout === 'bottom') {
            document.body.classList.add('layout-mic-bottom');
        }
    }
}

// Global STT state
let recognition = null;
let isSTTActive = false;

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
            document.getElementById('giant-mic-btn').classList.add('stt-active');
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
            }
        };

        recognition.onerror = (event) => {
            console.error("[STT] Error:", event.error);
            stopSTT();
        };

        recognition.onend = () => {
            isSTTActive = false;
            document.getElementById('giant-mic-btn').classList.remove('stt-active');
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


/**
 * Hook into global state to update giant mic icon if generating
 */
function updateMicUIForGeneration(generating) {
    const micBtn = document.getElementById('giant-mic-btn');
    if (!micBtn) return;

    if (generating) {
        micBtn.classList.add('gen-active');
        micBtn.querySelector('.material-icons-round').textContent = 'stop';
    } else {
        micBtn.classList.remove('gen-active');
        micBtn.querySelector('.material-icons-round').textContent = 'mic';
    }
}



