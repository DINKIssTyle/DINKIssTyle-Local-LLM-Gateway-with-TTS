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
    hideThink: true,
    temperature: 0.7,
    maxTokens: 4096,
    historyCount: 10,
    enableTTS: true,
    ttsLang: 'ko',
    chunkSize: 150,
    systemPrompt: 'You are a helpful AI assistant.',
    ttsVoice: '',
    ttsSpeed: 1.1,
    autoTTS: true,
    ttsFormat: 'wav',
    ttsSteps: 5,
    ttsThreads: 2,
    ttsFormat: 'wav',
    ttsSteps: 5,
    ttsThreads: 2,
    language: 'ko', // UI language
    apiToken: '',
    llmMode: 'standard', // 'standard' or 'stateful'
    disableStateful: false // LM Studio specific
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
        'setting.maxTokens.desc': '(기본값: 4096) LLM이 생성할 최대 토큰 수',
        'setting.history.label': 'History Count',
        'setting.history.desc': '(기본값: 10) 대화 기억 횟수',
        'setting.apiToken.label': 'API Token',
        'setting.apiToken.desc': 'LM Studio API Token (인증 활성화 시 필요, 빈칸이면 무시)',
        'setting.llmMode.label': 'Connection Mode',
        'setting.llmMode.desc': 'Select OpenAI Compatible or LM Studio (Stateful)',
        'setting.disableStateful.label': 'Disable Stateful Storage',
        'setting.disableStateful.desc': 'Do not store conversation history on server (LM Studio).',
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
        'setting.threads.desc': '(기본값: 4) TTS 생성에 할당하는 CPU 스레드',
        'setting.format.label': '재생 형식',
        'setting.format.desc': 'MP3는 WAV를 변환하여 재생합니다.',
        // Chat
        'chat.welcome': '안녕하세요! 채팅할 준비가 되었습니다. 우측 상단 기어 아이콘에서 설정하세요.',
        'chat.instruction': '우측 상단 설정(⚙️)에서 연결 모드 및 API Token을 확인해주세요.',
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
        'health.status.unreachable': '연결 불가'
    },
    en: {
        // Modal
        'modal.settings.title': 'Settings',
        // Sections
        'section.llm': 'LLM Settings',
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
        'setting.maxTokens.desc': '(Default: 4096) Maximum tokens to generate',
        'setting.history.label': 'History Count',
        'setting.history.desc': '(Default: 10) Number of messages to remember',
        'setting.apiToken.label': 'API Token',
        'setting.apiToken.desc': 'LM Studio API Token (Required if Auth is enabled)',
        'setting.llmMode.label': 'Connection Mode',
        'setting.llmMode.desc': 'Select between OpenAI Compatible or LM Studio',
        'setting.disableStateful.label': 'Disable Stateful Storage',
        'setting.disableStateful.desc': 'Do not store conversation on server (LM Studio).',
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
        'setting.threads.desc': '(Default: 4) CPU threads for TTS generation',
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
        'health.status.unreachable': 'Unreachable'
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
        const response = await fetch('/api/models');
        if (!response.ok) {
            throw new Error('Failed to fetch models');
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
        select.innerHTML = '<option value="">Connection error</option>';
        // Add a manual input option as fallback
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
        const response = await fetch('/api/auth/check');
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
    document.getElementById('cfg-api-token').value = config.apiToken || '';
    document.getElementById('cfg-llm-mode').value = config.llmMode || 'standard';
    document.getElementById('cfg-disable-stateful').checked = config.disableStateful || false;
    updateSettingsVisibility(); // Update UI visibility based on mode
    document.getElementById('cfg-enable-tts').checked = config.enableTTS;
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

    // Default (Standard/OpenAI Compatible)
    let showToken = false;
    let showHistory = true;
    let showDisableStateful = false;

    if (mode === 'stateful') {
        // LM Studio Mode
        showToken = true;
        showHistory = false; // LM Studio handles history via response_id
        showDisableStateful = true;
    }

    if (tokenContainer) tokenContainer.style.display = showToken ? 'block' : 'none';
    if (historyContainer) historyContainer.style.display = showHistory ? 'block' : 'none';
    if (disableStatefulContainer) disableStatefulContainer.style.display = showDisableStateful ? 'block' : 'none';
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
    const autoSaveIds = ['cfg-api', 'cfg-tts-lang', 'cfg-tts-voice', 'cfg-tts-format', 'cfg-chunk-size', 'cfg-system-prompt', 'cfg-llm-mode', 'cfg-api-token', 'cfg-disable-stateful'];
    autoSaveIds.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.onchange = () => saveConfig(false);
    });
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
        showToast(t('action.reload') + ' ✓');
    } catch (e) {
        console.error("Failed to reload external files:", e);
        showToast('Reload failed');
    }
}

function saveConfig(closeModal = true) {
    const cfgApiEl = document.getElementById('cfg-api');
    config.apiEndpoint = cfgApiEl ? cfgApiEl.value.trim() : config.apiEndpoint;
    config.model = document.getElementById('cfg-model').value.trim();
    config.hideThink = document.getElementById('cfg-hide-think').checked;
    config.temperature = parseFloat(document.getElementById('cfg-temp').value);
    config.maxTokens = parseInt(document.getElementById('cfg-max-tokens').value);
    config.historyCount = parseInt(document.getElementById('cfg-history').value);
    config.enableTTS = document.getElementById('cfg-enable-tts').checked;
    config.autoTTS = document.getElementById('cfg-auto-tts').checked;
    config.ttsLang = document.getElementById('cfg-tts-lang').value;

    // New fields
    config.apiToken = document.getElementById('cfg-api-token').value.trim();
    config.llmMode = document.getElementById('cfg-llm-mode').value;
    config.disableStateful = document.getElementById('cfg-disable-stateful').checked;

    // Update visibility immediately
    updateSettingsVisibility();

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

        // This is separate from saveConfig in app.go, but SetTTSThreads triggers reload
        if (config.ttsThreads) {
            window.go.main.App.SetTTSThreads(config.ttsThreads);
        }
    }

    // Also try fetch for web mode or as backup
    fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            api_endpoint: config.apiEndpoint,
            api_token: config.apiToken,
            llm_mode: config.llmMode,
            enable_tts: config.enableTTS,
            tts_threads: config.ttsThreads
        })
    }).then(r => {
        if (!r.ok) console.warn('Failed to sync settings');
    }).catch(e => console.warn('Sync error:', e));

    // Update header model name
    const headerModelName = document.getElementById('header-model-name');
    if (headerModelName) {
        headerModelName.textContent = config.model || 'No Model Set';
    }

    // Close modal only if requested
    if (closeModal) {
        closeSettingsModal();
    }
    showToast(t('action.save') + ' ✓');
}

async function syncServerConfig() {
    try {
        const response = await fetch('/api/config'); // Fetch current server config
        if (response.ok) {
            const serverCfg = await response.json();
            if (serverCfg.enable_tts !== undefined) {
                config.enableTTS = serverCfg.enable_tts;
                document.getElementById('cfg-enable-tts').checked = config.enableTTS;
                localStorage.setItem('appConfig', JSON.stringify(config));
            }
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

    messages = [];
    chatMessages.innerHTML = '';
}

/* Chat Logic */

async function sendMessage() {
    // Unlock audio context on user interaction
    unlockAudioContext();

    const text = messageInput.value.trim();
    if (!text && !pendingImage) return;
    if (isGenerating) return;

    // Stop and clear any existing audio/TTS
    stopAllAudio();

    // Prepare User Message
    const userMsg = {
        role: 'user',
        content: text,
        image: pendingImage
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
        payload = {
            model: config.model,
            input: text, // Only current input
            system_prompt: config.systemPrompt, // Explicitly pass system prompt
            temperature: config.temperature,
            stream: true
        };

        if (config.disableStateful) {
            payload.store = false;
        }
        // Add image if present
        if (pendingImage) {
            // LM Studio Stateful chat might support array input for images? 
            // The docs example is just 'input': "string". 
            // If vision is needed, we might need a complex input object or standard stateless for vision.
            // For now, assume simple text input for stateful.
            // If user attached image, we might force stateless or warn?
            // Let's attach it as text for now to avoid breaking.
            // Actually, Stateful Chat docs don't explicitly show vision examples.
            // We'll stick to text input. If vision is critical, we might need Standard mode.
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
    let fullText = '';

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

                    // Capture response_id if present (Stateful Chat)
                    if (json.response_id) {
                        lastResponseId = json.response_id;
                        console.log(`[Stateful] Captured response_id: ${lastResponseId}`);
                    }

                    let contentToAdd = '';

                    // Handle Standard/SSE format
                    if (json.choices && json.choices.length > 0) {
                        const delta = json.choices[0].delta || {};
                        const message = json.choices[0].message || {}; // Non-streaming fallback
                        contentToAdd = delta.content || message.content || '';
                    }
                    // Handle Stateful Chat JSON format (output array mechanism - legacy/alternative?)
                    else if (json.output && Array.isArray(json.output)) {
                        for (const item of json.output) {
                            if (item.content) {
                                contentToAdd += item.content;
                            }
                        }
                    }
                    // Handle LM Studio Stateful Chat Streaming Format (based on logs)
                    else if (json.type === 'message.delta' && json.content) {
                        contentToAdd = json.content;
                    }
                    // Handle Reasoning (Thinking)
                    else if (json.type === 'reasoning.start') {
                        contentToAdd = '<think>';
                    }
                    else if (json.type === 'reasoning.delta' && json.content) {
                        contentToAdd = json.content;
                    }
                    else if (json.type === 'reasoning.end') {
                        contentToAdd = '</think>\n';
                    }
                    // Handle MCP Tool Calls
                    else if (json.type === 'tool_call.start') {
                        contentToAdd = `\n> **🛠️ Tool Call:** \`${json.tool}\`\n`;
                    }
                    else if (json.type === 'tool_call.success') {
                        contentToAdd = `> ✅ **Tool Finished**\n\n`;
                    }
                    else if (json.type === 'tool_call.failure') {
                        contentToAdd = `\n> ❌ **Tool Failed:** ${json.reason || 'Unknown error'}\n\n`;
                    }
                    else if (json.type === 'chat.end' && json.result && json.result.response_id) {
                        lastResponseId = json.result.response_id;
                        console.log(`[Stateful] Captured response_id from chat.end: ${lastResponseId}`);
                    }

                    if (contentToAdd) {
                        fullText += contentToAdd;
                        let displayText = fullText;

                        // UI Display Logic (Depends on config.hideThink)
                        if (config.hideThink) {
                            // Remove complete <think>...</think> blocks
                            displayText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');
                            // Handle case where </think> exists without opening tag (remove everything before it)
                            if (displayText.includes('</think>')) {
                                displayText = displayText.split('</think>').pop().trim();
                            }
                            // Handle incomplete <think> tag (still being streamed)
                            if (displayText.includes('<think>')) {
                                displayText = displayText.split('<think>')[0];
                            }
                        }
                        updateMessageContent(elementId, displayText);

                        // TTS Logic (ALWAYS filter <think> regardless of settings)
                        if (useStreamingTTS) {
                            // Remove complete <think>...</think> blocks
                            let ttsText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');

                            // Handle incomplete <think> tag (ongoing thought)
                            if (ttsText.includes('<think>')) {
                                ttsText = ttsText.split('<think>')[0];
                            }

                            // Handle case where </think> exists without opening tag
                            if (ttsText.includes('</think>')) {
                                ttsText = ttsText.split('</think>').pop();
                            }

                            // Feed the filtered text to TTS engine
                            if (ttsText) {
                                feedStreamingTTS(ttsText);
                            }
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
        if (fullText) {
            messages.push({ role: 'assistant', content: fullText });
        }

        // Finalize streaming TTS (commit any remaining text)
        if (useStreamingTTS) {
            let finalTTSText = fullText;
            // ALWAYS filter think tags for TTS finalization too
            finalTTSText = fullText.replace(/<think>[\s\S]*?<\/think>/g, '');
            if (finalTTSText.includes('<think>')) {
                finalTTSText = finalTTSText.split('<think>')[0];
            }
            if (finalTTSText.includes('</think>')) {
                finalTTSText = finalTTSText.split('</think>').pop();
            }
            finalizeStreamingTTS(finalTTSText);
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

function updateMessageContent(id, text) {
    const el = document.getElementById(id);
    if (!el) return;

    const bubble = el.querySelector('.message-bubble');
    bubble.innerHTML = `<div class="markdown-body">${marked.parse(text)}</div>`;

    // Re-highlight code blocks
    bubble.querySelectorAll('pre code').forEach((block) => {
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
function cleanTextForTTS(text) {
    if (!text) return '';

    let cleaned = text;

    // Apply Dictionary Corrections (Optimized with Regex)
    if (ttsDictionaryRegex) {
        cleaned = cleaned.replace(ttsDictionaryRegex, (match) => {
            return ttsDictionary[match.toLowerCase()] || match;
        });
    }

    // Remove HTML tags
    cleaned = cleaned.replace(/<[^>]*>/g, '');

    // IMPROVED: Add pauses for Markdown structure (Headers, Horizontal Rules)
    // Replace headers (# Title) with "Title." for pause
    cleaned = cleaned.replace(/^(#{1,6})\s+(.+)$/gm, '$2.');
    // Replace horizontal rules (---) with pause
    cleaned = cleaned.replace(/^([-*_]){3,}\s*$/gm, '.');
    // Replace list items (* Item) - optional, but might help
    // cleaned = cleaned.replace(/^[\*\-]\s+(.+)$/gm, '$1,'); 


    // Remove emoji (comprehensive Unicode ranges)
    cleaned = cleaned.replace(/[\u{1F600}-\u{1F64F}]/gu, ''); // Emoticons
    cleaned = cleaned.replace(/[\u{1F300}-\u{1F5FF}]/gu, ''); // Misc Symbols and Pictographs
    cleaned = cleaned.replace(/[\u{1F680}-\u{1F6FF}]/gu, ''); // Transport and Map
    cleaned = cleaned.replace(/[\u{1F1E0}-\u{1F1FF}]/gu, ''); // Flags
    cleaned = cleaned.replace(/[\u{2600}-\u{26FF}]/gu, '');   // Misc symbols
    cleaned = cleaned.replace(/[\u{2700}-\u{27BF}]/gu, '');   // Dingbats
    cleaned = cleaned.replace(/[\u{FE00}-\u{FE0F}]/gu, '');   // Variation Selectors
    cleaned = cleaned.replace(/[\u{1F900}-\u{1F9FF}]/gu, ''); // Supplemental Symbols and Pictographs
    cleaned = cleaned.replace(/[\u{1FA00}-\u{1FA6F}]/gu, ''); // Chess Symbols
    cleaned = cleaned.replace(/[\u{1FA70}-\u{1FAFF}]/gu, ''); // Symbols and Pictographs Extended-A
    cleaned = cleaned.replace(/[\u{231A}-\u{231B}]/gu, '');   // Watch, Hourglass
    cleaned = cleaned.replace(/[\u{23E9}-\u{23F3}]/gu, '');   // Various symbols
    cleaned = cleaned.replace(/[\u{23F8}-\u{23FA}]/gu, '');   // Various symbols
    cleaned = cleaned.replace(/[\u{25AA}-\u{25AB}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{25B6}]/gu, '');            // Play button
    cleaned = cleaned.replace(/[\u{25C0}]/gu, '');            // Reverse button
    cleaned = cleaned.replace(/[\u{25FB}-\u{25FE}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{2614}-\u{2615}]/gu, '');   // Umbrella, Hot beverage
    cleaned = cleaned.replace(/[\u{2648}-\u{2653}]/gu, '');   // Zodiac
    cleaned = cleaned.replace(/[\u{267F}]/gu, '');            // Wheelchair
    cleaned = cleaned.replace(/[\u{2693}]/gu, '');            // Anchor
    cleaned = cleaned.replace(/[\u{26A1}]/gu, '');            // High voltage
    cleaned = cleaned.replace(/[\u{26AA}-\u{26AB}]/gu, '');   // Circles
    cleaned = cleaned.replace(/[\u{26BD}-\u{26BE}]/gu, '');   // Sports
    cleaned = cleaned.replace(/[\u{26C4}-\u{26C5}]/gu, '');   // Weather
    cleaned = cleaned.replace(/[\u{26CE}]/gu, '');            // Ophiuchus
    cleaned = cleaned.replace(/[\u{26D4}]/gu, '');            // No entry
    cleaned = cleaned.replace(/[\u{26EA}]/gu, '');            // Church
    cleaned = cleaned.replace(/[\u{26F2}-\u{26F3}]/gu, '');   // Fountain, Golf
    cleaned = cleaned.replace(/[\u{26F5}]/gu, '');            // Sailboat
    cleaned = cleaned.replace(/[\u{26FA}]/gu, '');            // Tent
    cleaned = cleaned.replace(/[\u{26FD}]/gu, '');            // Fuel pump
    cleaned = cleaned.replace(/[\u{2702}]/gu, '');            // Scissors
    cleaned = cleaned.replace(/[\u{2705}]/gu, '');            // Check mark
    cleaned = cleaned.replace(/[\u{2708}-\u{270D}]/gu, '');   // Various
    cleaned = cleaned.replace(/[\u{270F}]/gu, '');            // Pencil
    cleaned = cleaned.replace(/[\u{2712}]/gu, '');            // Black nib
    cleaned = cleaned.replace(/[\u{2714}]/gu, '');            // Check mark
    cleaned = cleaned.replace(/[\u{2716}]/gu, '');            // X mark
    cleaned = cleaned.replace(/[\u{271D}]/gu, '');            // Latin cross
    cleaned = cleaned.replace(/[\u{2721}]/gu, '');            // Star of David
    cleaned = cleaned.replace(/[\u{2728}]/gu, '');            // Sparkles
    cleaned = cleaned.replace(/[\u{2733}-\u{2734}]/gu, '');   // Eight spoked asterisk
    cleaned = cleaned.replace(/[\u{2744}]/gu, '');            // Snowflake
    cleaned = cleaned.replace(/[\u{2747}]/gu, '');            // Sparkle
    cleaned = cleaned.replace(/[\u{274C}]/gu, '');            // Cross mark
    cleaned = cleaned.replace(/[\u{274E}]/gu, '');            // Cross mark
    cleaned = cleaned.replace(/[\u{2753}-\u{2755}]/gu, '');   // Question marks
    cleaned = cleaned.replace(/[\u{2757}]/gu, '');            // Exclamation mark
    cleaned = cleaned.replace(/[\u{2763}-\u{2764}]/gu, '');   // Hearts
    cleaned = cleaned.replace(/[\u{2795}-\u{2797}]/gu, '');   // Math symbols
    cleaned = cleaned.replace(/[\u{27A1}]/gu, '');            // Right arrow
    cleaned = cleaned.replace(/[\u{27B0}]/gu, '');            // Curly loop
    cleaned = cleaned.replace(/[\u{27BF}]/gu, '');            // Double curly loop
    cleaned = cleaned.replace(/[\u{2934}-\u{2935}]/gu, '');   // Arrows
    cleaned = cleaned.replace(/[\u{2B05}-\u{2B07}]/gu, '');   // Arrows
    cleaned = cleaned.replace(/[\u{2B1B}-\u{2B1C}]/gu, '');   // Squares
    cleaned = cleaned.replace(/[\u{2B50}]/gu, '');            // Star
    cleaned = cleaned.replace(/[\u{2B55}]/gu, '');            // Circle
    cleaned = cleaned.replace(/[\u{3030}]/gu, '');            // Wavy dash
    cleaned = cleaned.replace(/[\u{303D}]/gu, '');            // Part alternation mark
    cleaned = cleaned.replace(/[\u{3297}]/gu, '');            // Circled Ideograph Congratulation
    cleaned = cleaned.replace(/[\u{3299}]/gu, '');            // Circled Ideograph Secret

    // FIRST: Add punctuation to markdown elements that typically don't have them
    // Headlines (# text) - add period at end if no punctuation
    cleaned = cleaned.replace(/^(#{1,6})\s*([^\n]+?)([.!?]?)$/gm, (match, hashes, text, punct) => {
        // If headline doesn't end with punctuation, add a period
        if (!punct) {
            return `${hashes} ${text}.`;
        }
        return match;
    });

    // Bold/Italic text that ends a line without punctuation
    cleaned = cleaned.replace(/(\*\*[^*]+\*\*|\*[^*]+\*)(\s*)$/gm, '$1.$2');

    // Remove markdown formatting (AFTER adding punctuation)
    cleaned = cleaned.replace(/#{1,6}\s*/gm, ''); // Headlines - remove # but keep text
    cleaned = cleaned.replace(/[*`_~|]/g, ''); // Other markdown syntax characters
    cleaned = cleaned.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1'); // [link text](url) -> link text
    cleaned = cleaned.replace(/!\[[^\]]*\]\([^)]*\)/g, ''); // Remove image markdown
    cleaned = cleaned.replace(/```[\s\S]*?```/g, ''); // Code blocks
    cleaned = cleaned.replace(/`[^`]*`/g, ''); // Inline code
    cleaned = cleaned.replace(/^>\s*/gm, ''); // Blockquotes
    cleaned = cleaned.replace(/^-{3,}$/gm, ''); // Horizontal rules
    cleaned = cleaned.replace(/^\s*[-*+]\s+/gm, ''); // List markers
    cleaned = cleaned.replace(/^\s*\d+\.\s+/gm, ''); // Numbered lists

    // Remove special characters that shouldn't be spoken
    cleaned = cleaned.replace(/[«»""„‚]/g, ' '); // Fancy quotes -> space (prevent stuck words)
    cleaned = cleaned.replace(/[=→]/g, ', '); // Equals and arrows to pauses
    cleaned = cleaned.replace(/[—–]/g, ', '); // Dashes to pauses
    cleaned = cleaned.replace(/\.{3,}/g, '.'); // Ellipsis
    cleaned = cleaned.replace(/\s*[-•◦▪▸►]\s*/g, ', '); // Bullet points to pauses

    // Ensure space after punctuation to prevent stuck sentences
    // e.g., "word.Next" -> "word. Next"
    cleaned = cleaned.replace(/([.!?])(?=[^ \n])/g, '$1 ');

    // Normalize whitespace
    cleaned = cleaned.replace(/\r\n/g, '\n'); // Normalize line endings
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n'); // Max 2 newlines
    cleaned = cleaned.replace(/[ \t]+/g, ' '); // Multiple spaces to single
    cleaned = cleaned.replace(/^\s+|\s+$/gm, ''); // Trim each line

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

        // 우선순위 1: 문단 경계 (줄바꿈 두 개) - 첫 청크는 길이 무관, 이후는 chunkSize 적용
        const paragraphMatch = newText.match(/^([\s\S]*?\n\s*\n)/);
        if (paragraphMatch && paragraphMatch[1].trim()) {
            const potentialCommit = streamingTTSBuffer + cleanTextForTTS(paragraphMatch[1]);
            const isFirstChunk = (ttsQueue.length === 0 && !isPlayingQueue);

            // 첫 청크는 즉시 커밋, 이후는 chunkSize 이상일 때만
            if (isFirstChunk || potentialCommit.length >= config.chunkSize) {
                committed = potentialCommit;
                streamingTTSBuffer = "";
                advanceBy = paragraphMatch[0].length;
            } else {
                // 버퍼에 누적
                streamingTTSBuffer = potentialCommit + " ";
                streamingTTSCommittedIndex += paragraphMatch[0].length;
                continue;
            }
        }

        // 우선순위 2: 단일 줄바꿈 - 첫 청크는 즉시, 이후는 chunkSize 이상
        if (!committed) {
            const lineMatch = newText.match(/^([^\n]+)\n/);
            if (lineMatch) {
                const cleanedLine = cleanTextForTTS(lineMatch[1]);
                const potentialCommit = streamingTTSBuffer + cleanedLine;
                const isFirstChunk = (ttsQueue.length === 0 && !isPlayingQueue);

                // 첫 청크: 5자 이상이면 즉시 커밋 (빠른 시작)
                // 중간 청크: chunkSize 이상일 때만 커밋
                if ((isFirstChunk && potentialCommit.length >= 5) || potentialCommit.length >= config.chunkSize) {
                    committed = potentialCommit;
                    streamingTTSBuffer = "";
                    advanceBy = lineMatch[0].length;
                } else {
                    // 버퍼에 누적하고 계속 진행
                    streamingTTSBuffer = potentialCommit + " ";
                    streamingTTSCommittedIndex += lineMatch[0].length;
                    continue;
                }
            }
        }

        // 우선순위 3: 문장 종료 (.!?) + 다음 문장 시작 - chunkSize 누적 시 커밋
        if (!committed) {
            const sentenceMatch = newText.match(/^([\s\S]*?[.!?])(\s+[A-Z가-힣])/);
            if (sentenceMatch && sentenceMatch[1].trim()) {
                const potentialCommit = streamingTTSBuffer + cleanTextForTTS(sentenceMatch[1]);

                if (potentialCommit.length >= config.chunkSize) {
                    committed = potentialCommit;
                    streamingTTSBuffer = "";
                    advanceBy = sentenceMatch[1].length;
                } else {
                    // 버퍼에 누적
                    streamingTTSBuffer = potentialCommit + " ";
                    streamingTTSCommittedIndex += sentenceMatch[1].length;
                    continue;
                }
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
        statusDetails += `\n- **모드**: ${modeLabel}`;

        // Analyze health
        if (health.llmStatus !== 'ok') {
            statusIcon = "⚠️";
            statusTitle = t('health.checkRequired');

            let errorDetail = health.llmMessage;
            if (errorDetail.includes('401')) {
                errorDetail += " -> **API Token**을 확인해주세요.";
            } else if (errorDetail.includes('connect') || errorDetail.includes('refused')) {
                errorDetail += " -> **LM Studio 서버**가 실행 중인지 확인해주세요.";
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


