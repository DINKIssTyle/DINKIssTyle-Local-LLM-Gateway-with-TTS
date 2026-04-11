/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

// Configuration State
// [NOTICE] 웹페이지(web.html)의 초기 설정값은 아래 객체에서 정의됩니다. HTML 파일의 value 속성은 무시됩니다.
// 브라우저 캐시(LocalStorage)에 저장된 값이 있다면 그것이 가장 우선됩니다.
const DEFAULT_STATEFUL_TURN_LIMIT = 8;
const DEFAULT_STATEFUL_CHAR_BUDGET = 32000;
const DEFAULT_STATEFUL_TOKEN_BUDGET = 30000;
const LM_STUDIO_REPEAT_RECOVERY_PENALTY = 1.15;
const LM_STUDIO_REPEAT_RECOVERY_TEMPERATURE_DELTA = 0.15;
const DEFAULT_REASONING_OPTIONS = ['off', 'low', 'medium', 'high', 'on'];
const STREAM_LOOP_DETECTION_TAIL_CHARS = 1200;

let config = {
    apiEndpoint: 'http://127.0.0.1:1234',
    model: 'qwen/qwen3.5-35b-a3b',
    secondaryModel: '',
    hideThink: false,       // Default: False
    temperature: null,     // null => Auto (omit from payload)
    historyCount: 10,
    enableTTS: true,       // Default: True
    enableTTS: true,       // Default: True
    enableMCP: true,       // Default: True
    enableMemory: false,   // Default: False
    ttsLang: 'ko',
    chunkSize: 100,        // Default: 100 (Smart Chunking)
    systemPrompt: 'You are a helpful AI assistant.',
    ttsEngine: 'supertonic', // 'supertonic' or 'os'
    ttsVoice: 'F1',        // Default: F1
    ttsSpeed: 1.1,         // Default: 1.1
    autoTTS: true,         // Default: True (Auto-play)
    voiceInputAutoTTS: true, // Default: True (Override auto-play for STT messages)
    ttsFormat: 'wav',      // Default: wav
    ttsSteps: 5,           // Default: 5
    ttsThreads: 2,         // Default: 2
    enableEmbeddings: false,
    embeddingProvider: 'local',
    embeddingModelId: 'multilingual-e5-small',
    osTtsVoiceURI: '',
    osTtsVoiceName: '',
    osTtsVoiceLang: '',
    osTtsRate: 1.0,
    osTtsPitch: 1.0,
    language: 'ko', // UI language
    apiToken: '',
    llmMode: 'standard', // 'standard' or 'stateful'
    contextStrategy: 'history', // LM Studio: retrieval/stateful/none, OpenAI: retrieval/history/none
    reasoning: '',
    showReasoningControl: true,
    forceShowReasoningControl: false,
    statefulTurnLimit: DEFAULT_STATEFUL_TURN_LIMIT,
    statefulCharBudget: DEFAULT_STATEFUL_CHAR_BUDGET,
    statefulTokenBudget: DEFAULT_STATEFUL_TOKEN_BUDGET,
    micLayout: 'none', // 'none', 'left', 'right', 'bottom', 'inline'
    chatFontSize: 16,
    userBubbleTheme: 'ocean', // Options: 'ocean', 'lime', 'sunset', 'amber', 'magenta'
    streamingScrollMode: 'auto', // Options: 'auto', 'label-top'
    markdownRenderMode: 'fast', // Options: 'fast', 'balanced', 'final'
    hapticsEnabled: true
};

let AppState = {
    chat: {
        messages: [],
        isGenerating: false,
        activeLocalTurnId: '',
        abortController: null,
        streamLoopWorker: null,
        streamLoopWorkerFailed: false,
        streamLoopWorkerMessageId: 0,
        pendingStreamLoopChecks: new Map(),
        activeLocalAssistantId: '',
        assistantTurnIdMap: new Map(),
        locallyRenderedTurnIds: new Set(),
        stateful: {
            lastResponseId: null,
            turnCount: 0,
            estimatedChars: 0,
            summary: '',
            resetCount: 0,
            pendingResetReason: null,
            lastInputTokens: 0,
            lastOutputTokens: 0,
            peakInputTokens: 0
        },
        passive: {
            placeholder: '',
            llmBusy: false,
            externalLLMBusy: false,
            externalLLMPhase: '',
            sessionLLMRunning: false
        }
    },
    audio: {
        currentAudio: null,
        currentAudioBtn: null,
        audioWarmup: null,
        ttsQueue: [],
        activeTTSSessionLabel: '',
        currentAudioPlaybackController: null,
        isPlayingQueue: false,
        streamingTTSActive: false,
        streamingTTSCommittedIndex: 0,
        streamingTTSBuffer: '',
        streamingTTSProcessor: null,
        ttsSessionId: 0,
        dictionary: {},
        dictionaryRegex: null,
        dictionaryLang: '',
        dictionaryLoadPromise: null
    },
    input: {
        pendingVoiceInputAutoTTS: false,
        isSTTActive: false,
        recognition: null,
        sttPlaceholderTimer: null,
        sttPlaceholderIndex: 0,
        sttSuppressAutoSend: false,
        sttStopFallbackTimer: null,
        pendingImage: null
    },
    ui: {
        activeStreamingMessageId: null,
        progress: {
            active: false,
            label: '',
            percent: null,
            hideTimer: null
        },
        scroll: {
            shouldAutoScroll: true,
            holdAutoScrollTimeout: null,
            resizeObserver: null,
            lockToLatest: false,
            suppressNextEvent: false,
            lastObservedHeight: 0,
            pendingToBottom: false,
            metrics: {
                scrollTop: 0,
                scrollHeight: 0,
                clientHeight: 0,
                isNearBottom: true
            },
            pendingFrame: null,
            pendingInputFocusTop: null,
            pendingFirstInputRepairTop: null
        },
        composerBackgroundTasks: new Map(),
        pin: {
            activePinnedToTop: false,
            pinPending: false
        },
        wakeLock: null,
        lastKeyboardViewportSignalAt: 0,
        location: {
            currentUserLocation: null,
            lastErrorMessage: ''
        },
        micLayout: 'none',
        systemPromptPresets: []
    },
    session: {
        currentUser: null,
        currentCache: null,
        eventSeq: 0,
        clearedAt: '',
        lastCache: null,
        didInitialBootstrap: false,
        isRestoring: false,
        serverRunning: false,
        lastSyncedConfigSignature: '',
        syncPromise: null,
        pendingSync: false,
        refreshPromise: null,
        pendingRefresh: false,
        retryTimer: null,
        pollTimer: null,
        reconnectWatchdogTimer: null,
        eventStreamSource: null,
        eventStreamConnected: false,
        eventStreamRetryTimer: null,
        eventStreamLastErrorAt: 0,
        reconnectCardVisible: false,
        localStreamOwnershipReleased: false,
        retryRecoveryInProgress: false,
        lastFetchPromise: null,
        foregroundSyncTimers: [],
        replay: {
            currentTurnId: '',
            currentAssistantId: '',
            messageBuffers: new Map(),
            reasoningBuffers: new Map()
        }
    }
};

/**
 * AppState Subscription System
 * Simple Pub/Sub for state changes.
 */
const AppStateEvents = {
    _listeners: new Map(),
    on(path, callback) {
        if (!this._listeners.has(path)) this._listeners.set(path, []);
        this._listeners.get(path).push(callback);
    },
    emit(path, value) {
        if (this._listeners.has(path)) {
            this._listeners.get(path).forEach(cb => cb(value));
        }
    }
};

function isReactivePlainObject(val) {
    if (val === null || typeof val !== 'object') return false;
    if (Array.isArray(val)) return false;
    if (val instanceof Map || val instanceof Set) return false;
    if (window.Node && val instanceof window.Node) return false;
    if (val instanceof Date || val instanceof RegExp) return false;
    if (typeof AbortController !== 'undefined' && val instanceof AbortController) return false;
    if (typeof AbortSignal !== 'undefined' && val instanceof AbortSignal) return false;
    const proto = Object.getPrototypeOf(val);
    return proto === Object.prototype || proto === null;
}

/**
 * createReactiveObject: Recursively wrap object in Proxy to detect changes
 */
function createReactiveObject(obj, path = '') {
    return new Proxy(obj, {
        set(target, key, value) {
            const currentPath = path ? `${path}.${key}` : key;
            const oldValue = target[key];
            target[key] = value;
            if (oldValue !== value) {
                AppStateEvents.emit(currentPath, value);
            }
            return true;
        },
        get(target, key) {
            const val = target[key];
            if (isReactivePlainObject(val)) {
                return createReactiveObject(val, path ? `${path}.${key}` : key);
            }
            return typeof val === 'function' ? val.bind(target) : val;
        }
    });
}

// Transform AppState into a reactive proxy
AppState = createReactiveObject(AppState);

let streamLoopWorker = null;
let streamLoopWorkerFailed = false;
let streamLoopWorkerMessageId = 0;
const pendingStreamLoopChecks = new Map();
const appUtils = window.DKSTAppUtils;
const appChatStreaming = window.DKSTChatStreaming;
const appChatUI = window.DKSTChatUI;
const appI18n = window.DKSTI18n;
const appModels = window.DKSTModels;
const appSavedLibrary = window.DKSTSavedLibrary;
const appSession = window.DKSTSession;
const appTTS = window.DKSTTTS;

if (!appUtils) {
    throw new Error('DKSTAppUtils failed to load before app.js');
}

if (!appChatStreaming) {
    throw new Error('DKSTChatStreaming failed to load before app.js');
}

if (!appChatUI) {
    throw new Error('DKSTChatUI failed to load before app.js');
}

if (!appI18n) {
    throw new Error('DKSTI18n failed to load before app.js');
}

if (!appModels) {
    throw new Error('DKSTModels failed to load before app.js');
}

if (!appSavedLibrary) {
    throw new Error('DKSTSavedLibrary failed to load before app.js');
}

if (!appSession) {
    throw new Error('DKSTSession failed to load before app.js');
}

if (!appTTS) {
    throw new Error('DKSTTTS failed to load before app.js');
}

const {
    buildSessionFetchOptions,
    escapeAttr,
    escapeHtml,
    getMarkdownRenderer,
    normalizeMarkdownForRender,
    renderLooseMarkdownToHtml,
    sanitizeRenderedMarkdownHtml,
    shouldFallbackToLooseMarkdown
} = appUtils;

const { applyTranslations: applyI18nTranslations, t: translateKey, translations } = appI18n;

function getDefaultContextStrategyForMode(mode) {
    return mode === 'stateful' ? 'stateful' : 'history';
}

function normalizeContextStrategyForMode(mode, strategy) {
    const normalizedMode = String(mode || '').trim().toLowerCase() === 'stateful' ? 'stateful' : 'standard';
    const normalizedStrategy = String(strategy || '').trim().toLowerCase();
    const allowed = normalizedMode === 'stateful'
        ? ['retrieval', 'stateful', 'none']
        : ['retrieval', 'history', 'none'];
    return allowed.includes(normalizedStrategy)
        ? normalizedStrategy
        : getDefaultContextStrategyForMode(normalizedMode);
}

function getNormalizedContextStrategy() {
    config.contextStrategy = normalizeContextStrategyForMode(config.llmMode, config.contextStrategy);
    return config.contextStrategy;
}

function usesStatefulConversationContext() {
    return config.llmMode === 'stateful' && getNormalizedContextStrategy() === 'stateful';
}

function usesHistoryConversationContext() {
    return config.llmMode !== 'stateful' && getNormalizedContextStrategy() === 'history';
}

function usesRetrievalConversationContext() {
    return getNormalizedContextStrategy() === 'retrieval';
}

function enforceMCPPolicyForMode(mode) {
    return mode === 'stateful' ? !!config.enableMCP : false;
}

const USER_BUBBLE_THEMES = {
    ocean: {
        id: 'ocean',
        gradient: 'linear-gradient(180deg, rgba(34, 88, 161, 0.9), rgba(24, 70, 132, 0.94))',
        border: 'rgba(110, 173, 255, 0.26)',
        shadow: '0 10px 28px rgba(18, 57, 109, 0.28)',
        text: '#ffffff',
        icon: '#78a1ff'
    },
    lime: {
        id: 'lime',
        gradient: 'linear-gradient(180deg, #7cff35, #28c800)',
        border: 'rgba(188, 255, 136, 0.32)',
        shadow: '0 10px 28px rgba(48, 136, 0, 0.26)',
        text: '#091305',
        icon: '#7cff35'
    },
    sunset: {
        id: 'sunset',
        gradient: 'linear-gradient(180deg, #ff624b, #c81800)',
        border: 'rgba(255, 156, 138, 0.3)',
        shadow: '0 10px 28px rgba(145, 28, 6, 0.28)',
        text: '#ffffff',
        icon: '#ff624b'
    },
    amber: {
        id: 'amber',
        gradient: 'linear-gradient(180deg, #ffe96a, #ffb300)',
        border: 'rgba(255, 229, 121, 0.34)',
        shadow: '0 10px 28px rgba(171, 114, 0, 0.24)',
        text: '#171204',
        icon: '#ffc533'
    },
    magenta: {
        id: 'magenta',
        gradient: 'linear-gradient(180deg, #ff43b2, #d5167f)',
        border: 'rgba(255, 140, 203, 0.32)',
        shadow: '0 10px 28px rgba(146, 15, 90, 0.28)',
        text: '#ffffff',
        icon: '#ff43b2'
    }
};

function rerenderAllMarkdownHosts() {
    document.querySelectorAll('.markdown-committed, #saved-turn-modal-response').forEach((host) => {
        const source = host?.dataset?.markdownSource;
        if (typeof source !== 'string') return;
        renderMarkdownIntoHost(host, source);
    });
}

function syncHapticsPreference() {
    appUtils.syncHapticsPreference(config);
}

function triggerHaptic(type) {
    appUtils.triggerHaptic(config, type);
}

function getLoopDetectionTail(text = '', minLength = 0) {
    const source = String(text || '');
    if (source.length < minLength) return '';
    return source.slice(-STREAM_LOOP_DETECTION_TAIL_CHARS);
}

function detectMessageRunawayRepetitionLocally(text = '') {
    const tail = getLoopDetectionTail(text, 100);
    if (!tail) return null;

    const shortLoopMatch = tail.match(/([\s\S]{5,}?)\1{9,}/);
    const longLoopMatch = tail.match(/([\s\S]{50,}?)\1{5,}/);
    const loopMatch = shortLoopMatch || longLoopMatch;
    if (!loopMatch || !loopMatch[1] || loopMatch[1].length < 4) return null;

    const snippet = loopMatch[1];
    const isToolLog = snippet.includes('Tool Call')
        || snippet.includes('Tool Finished')
        || snippet.includes('🛠️')
        || snippet.includes('✅');
    if (isToolLog) return null;

    return {
        snippet: snippet.slice(0, 120),
        repetitions: Math.max(2, Math.floor(loopMatch[0].length / loopMatch[1].length)),
        source: 'message-loop'
    };
}

function createStreamLoopWorker() {
    if (AppState.chat.streamLoopWorker || AppState.chat.streamLoopWorkerFailed || typeof Worker !== 'function') return AppState.chat.streamLoopWorker;

    try {
        AppState.chat.streamLoopWorker = new Worker('public/stream-loop-worker.js?v=1');
        AppState.chat.streamLoopWorker.onmessage = (event) => {
            const data = event?.data || {};
            const id = Number(data.id || 0);
            const pending = AppState.chat.pendingStreamLoopChecks.get(id);
            if (!pending) return;
            AppState.chat.pendingStreamLoopChecks.delete(id);
            pending.resolve(data.result || null);
        };
        AppState.chat.streamLoopWorker.onerror = (error) => {
            console.warn('[Stream] Loop worker failed, falling back to main thread', error);
            AppState.chat.streamLoopWorkerFailed = true;
            if (AppState.chat.streamLoopWorker) {
                AppState.chat.streamLoopWorker.terminate();
                AppState.chat.streamLoopWorker = null;
            }
            AppState.chat.pendingStreamLoopChecks.forEach((pending) => pending.resolve(null));
            AppState.chat.pendingStreamLoopChecks.clear();
        };
    } catch (error) {
        console.warn('[Stream] Failed to start loop worker, falling back to main thread', error);
        AppState.chat.streamLoopWorkerFailed = true;
        AppState.chat.streamLoopWorker = null;
    }

    return AppState.chat.streamLoopWorker;
}

function detectRunawayRepetitionAsync(text = '', kind = 'message') {
    const source = String(text || '');
    if (!source) return Promise.resolve(null);

    const worker = createStreamLoopWorker();
    if (!worker) {
        return Promise.resolve(
            kind === 'reasoning'
                ? detectRunawayRepetition(source)
                : detectMessageRunawayRepetitionLocally(source)
        );
    }

    return new Promise((resolve) => {
        const id = ++AppState.chat.streamLoopWorkerMessageId;
        AppState.chat.pendingStreamLoopChecks.set(id, { resolve });
        worker.postMessage({ id, kind, text: source });
    });
}
/**
 * SSEParser: raw bytes -> JSON events
 */
class SSEParser {
    constructor() {
        this.decoder = new TextDecoder();
        this.buffer = '';
    }

    parse(value) {
        this.buffer += this.decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n');
        const events = [];
        let boundary = this.buffer.indexOf('\n\n');
        while (boundary !== -1) {
            const rawBlock = this.buffer.slice(0, boundary);
            this.buffer = this.buffer.slice(boundary + 2);
            boundary = this.buffer.indexOf('\n\n');

            const block = String(rawBlock || '').trim();
            if (!block) continue;

            let eventName = 'message';
            const dataLines = [];
            for (const rawLine of block.split('\n')) {
                const line = String(rawLine || '');
                if (!line) continue;
                if (line.startsWith(':')) continue;
                if (line.startsWith('event:')) {
                    eventName = line.slice(6).trim() || 'message';
                    continue;
                }
                if (line.startsWith('data:')) {
                    dataLines.push(line.startsWith('data: ') ? line.slice(6) : line.slice(5));
                    continue;
                }
                if (line.startsWith('{')) {
                    dataLines.push(line);
                }
            }

            const dataStr = dataLines.join('\n').trim();
            if (!dataStr || dataStr === '[DONE]') continue;

            try {
                const parsed = JSON.parse(dataStr);
                if (parsed && typeof parsed === 'object' && !Array.isArray(parsed) && !parsed.type && eventName && eventName !== 'message') {
                    parsed.type = eventName;
                }
                events.push(parsed);
            } catch (e) {
                if (eventName === 'error') {
                    events.push({
                        type: 'error',
                        error: {
                            message: dataStr
                        }
                    });
                } else {
                    console.warn('[SSE] JSON Parse Error', e, dataStr);
                }
            }
        }
        return events;
    }
}

/**
 * StreamState: Maintains state for a single streaming response
 */
class StreamState {
    constructor(elementId, turnId, options = {}) {
        this.elementId = elementId;
        this.turnId = turnId;
        this.options = options;

        this.fullText = '';           
        this.reasoningBuffer = '';    
        this.speechBuffer = '';       
        this.currentlyReasoning = false;
        this.reasoningSource = null;   
        this.lastToolCallHtml = '';
        this.loopDetected = false;
        this.streamRestartRequested = false;
        this.streamAborted = false;
        this.streamDetached = false;

        this.useStreamingTTS = shouldAutoPlayTTSForRequest({
            fromVoiceInput: options.fromVoiceInput === true || AppState.input.pendingVoiceInputAutoTTS
        });

        if (this.useStreamingTTS) {
            initStreamingTTS(this.elementId);
            requestWakeLock();
        }
    }
}

/**
 * handleChatEndEvent: Process metadata and stats at the end of a stream
 */
function handleChatEndEvent(json, ctx) {
    if (AppState.ui.progress.active) hideProgressDock();
    
    if (json.result.response_id) {
        AppState.chat.stateful.lastResponseId = json.result.response_id;
    }

    const stats = json.result.stats || {};
    if (typeof stats.input_tokens === 'number' && Number.isFinite(stats.input_tokens)) {
        AppState.chat.stateful.lastInputTokens = stats.input_tokens;
        AppState.chat.stateful.peakInputTokens = Math.max(AppState.chat.stateful.peakInputTokens, stats.input_tokens);
    }
    if (typeof stats.total_output_tokens === 'number' && Number.isFinite(stats.total_output_tokens)) {
        AppState.chat.stateful.lastOutputTokens = stats.total_output_tokens;
    }

    const payload = {
        result: json.result,
        elapsed_ms: json.elapsed_ms,
        total_elapsed_ms: json.total_elapsed_ms
    };

    // Tool state extraction
    const toolState = extractToolStateFromPayload(payload);
    if (isMeaningfulToolState(toolState)) {
        const card = ensureToolCard(ctx.elementId, toolState.toolName || 'Tool');
        if (card) card._history = Array.isArray(toolState.history) ? [...toolState.history] : [];
        setToolCardState(ctx.elementId, toolState.state || 'success', toolState.summary || '', toolState.args || null, toolState.toolName || '');
    }

    // Reasoning extraction
    const reasoningText = extractReasoningContentFromPayload(payload);
    if (reasoningText) {
        AppState.session.replay.reasoningBuffers.set(ctx.elementId, reasoningText);
        const duration = Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms)) ? Number(payload.total_elapsed_ms || payload.elapsed_ms) : null;
        showReasoningStatus(ctx.elementId, reasoningText, false, duration);
        finalizeReasoningStatus(ctx.elementId, 'done', '', duration);
    }
}

/**
 * handleModelLoadEvent: Process model loading progress
 */
function handleModelLoadEvent(json, ctx) {
    switch (json.type) {
        case 'model_load.start':
            renderProgressDock(t('progress.loadingModel'), null, 'model-loading', true);
            break;
        case 'model_load.progress':
            renderProgressDock(t('progress.loadingModel'), json.progress * 100, 'model-loading', false);
            break;
        case 'model_load.end':
            renderProgressDock(`${t('progress.modelLoaded')} (${json.load_time_seconds?.toFixed(1) || '?'}s)`, 100, 'model-loading', false);
            setTimeout(() => hideProgressDock(), 1200);
            break;
    }
}

/**
 * handleErrorEvent: Show generative errors in bubble or tool card
 */
function handleErrorEvent(json, ctx) {
    if (AppState.ui.progress.active) hideProgressDock();
    const errMsg = getLocalizedRuntimeErrorMessage(json.error) || extractRuntimeErrorMessage(json.error) || t('tool.unknownError');

    if (ctx.lastToolCallHtml) {
        setToolCardState(ctx.elementId, 'failure', errMsg);
    } else {
        ctx.fullText += `\n\n**Error:** ${errMsg}\n`;
    }
}

/**
 * handleContentAddition: Append text, check for loops, detect text-based reasoning, and feed TTS
 */
async function handleContentAddition(contentToAdd, speechToAdd, ctx) {
    hideProgressDock();
    ctx.fullText += contentToAdd;

    // Loop detection
    if (!ctx.loopDetected && ctx.fullText.length >= 100) {
        const loopMatch = await detectRunawayRepetitionAsync(ctx.fullText, 'message');
        if (loopMatch) {
            ctx.loopDetected = true;
            const repetitionError = new Error(config.llmMode === 'stateful' && !ctx.options.repeatRecoveryApplied ? t('warning.repeatRetrying') : t('warning.repeatStopped'));
            repetitionError.code = 'LMSTUDIO_RUNAWAY_REPETITION';
            repetitionError.phase = 'message';
            repetitionError.snippet = loopMatch.snippet;
            ctx.streamRestartRequested = true;
            throw repetitionError;
        }
    }

    // Text-based Reasoning Detection fallback (e.g. <think>)
    if (!ctx.currentlyReasoning && !config.hideThink) {
        processTextBasedReasoning(ctx.fullText, ctx.elementId);
    }

    // UI rendering
    const displayText = stripHiddenAssistantProtocolText(ctx.fullText);
    scheduleStreamMessageRender(ctx.elementId, displayText);

    // TTS handling
    if (ctx.useStreamingTTS && speechToAdd) {
        handleSpeechAddition(speechToAdd, ctx);
    }
}

/**
 * processTextBasedReasoning: Detect and handle <think> tags in the main content stream
 */
function processTextBasedReasoning(rawText, elementId) {
    const hasAnalysis = rawText.includes('<|channel|>analysis');
    const hasFinal = rawText.includes('<|channel|>final');
    const hasThink = rawText.includes('<think>');
    const hasThinkEnd = rawText.includes('</think>');

    if ((hasAnalysis && !hasFinal) || (hasThink && !hasThinkEnd)) {
        let fullReasoningText = "";
        if (hasAnalysis) {
            const parts = rawText.split('<|channel|>analysis');
            fullReasoningText = parts[parts.length - 1].split('<|channel|>')[0].trim();
        } else if (hasThink) {
            const parts = rawText.split('<think>');
            fullReasoningText = parts[parts.length - 1].split('</think>')[0].trim();
        }
        AppState.session.replay.reasoningBuffers.set(elementId, fullReasoningText);
        let statusText = fullReasoningText;
        if (statusText.length > 150) statusText = "..." + statusText.slice(-147);
        showReasoningStatus(elementId, statusText, false);
    } else if (hasFinal || hasThinkEnd) {
        let fullReasoningText = "";
        if (hasFinal) {
            const parts = rawText.split('<|channel|>analysis');
            if (parts.length > 1) fullReasoningText = parts[parts.length - 1].split('<|channel|>final')[0].trim();
        } else if (hasThinkEnd) {
            const parts = rawText.split('<think>');
            if (parts.length > 1) fullReasoningText = parts[parts.length - 1].split('</think>')[0].trim();
        }
        if (fullReasoningText) {
            AppState.session.replay.reasoningBuffers.set(elementId, fullReasoningText);
            showReasoningStatus(elementId, fullReasoningText, true);
        } else {
            showReasoningStatus(elementId, null, true);
        }
    }
}

/**
 * handleSpeechAddition: Clean and feed text to TTS engine
 */
function handleSpeechAddition(speechToAdd, ctx) {
    let cleaned = speechToAdd;
    if (cleaned.includes('<think>') || cleaned.includes('<|channel|>')) {
        cleaned = cleaned.replace(/<think>[\s\S]*?<\/think>/g, '');
        cleaned = cleaned.replace(/<\|channel\|>analysis[\s\S]*?(?=<\|channel\|>final|$)/g, '');
        cleaned = cleaned.replace(/<\|channel\|>(analysis|final|message)/g, '');
        cleaned = cleaned.replace(/<\|end\|>/g, '');
        cleaned = cleaned.replace(/<think>/g, '').replace(/<\/think>/g, '');
    }
    if (cleaned) {
        ctx.speechBuffer += cleaned;
        feedStreamingTTS(ctx.speechBuffer);
    }
}

async function handleStreamEvent(json, ctx) {
    const elementId = ctx.elementId;
    if (json.type === 'reasoning.start'
        || json.type === 'tool_call.start'
        || json.type === 'message.delta') {
        logScrollTrace(`stream:${json.type}`, {
            elementId,
            contentLength: typeof json.content === 'string' ? json.content.length : 0
        });
    }

    if (json.response_id) { AppState.chat.stateful.lastResponseId = json.response_id; }

    if (json.error) {
        const errorMsg = getLocalizedRuntimeErrorMessage(json.error) || extractRuntimeErrorMessage(json.error) || t('tool.unknownError');
        throw new Error(errorMsg);
    }

    let contentToAdd = '';
    let speechToAdd = '';

    if (json.choices && json.choices.length > 0) {
        const delta = json.choices[0].delta || {};
        const message = json.choices[0].message || {};

        if (delta.reasoning_content) {
            if (!ctx.currentlyReasoning) {
                ctx.reasoningBuffer += '<think>';
                ctx.currentlyReasoning = true;
                ctx.reasoningSource = 'field';
                showReasoningStatus(elementId, '...');
            }
            if (ctx.reasoningSource !== 'sse') {
                ctx.reasoningBuffer += delta.reasoning_content;
                showReasoningStatus(elementId, ctx.reasoningBuffer);
            }
        }

        const part = delta.content || message.content || '';
        if (part && ctx.currentlyReasoning && !delta.reasoning_content) {
            ctx.reasoningBuffer += '</think>\n';
            ctx.currentlyReasoning = false;
            ctx.reasoningSource = null;
            finalizeReasoningStatus(elementId, 'done', '');
        }

        if (!ctx.currentlyReasoning) {
            contentToAdd += part;
            speechToAdd += part;
        }
    }
    else if (json.output && Array.isArray(json.output)) {
        for (const item of json.output) {
            if (item.type === 'reasoning') continue;
            if (item.content && item.type === 'message') {
                contentToAdd += item.content;
                if (!ctx.currentlyReasoning) speechToAdd += item.content;
            }
        }
    }
    else if (json.type === 'message.delta' && json.content) {
        contentToAdd = json.content;
        if (!ctx.currentlyReasoning) speechToAdd = json.content;
    }
    else if (json.type === 'reasoning.start') {
        if (!ctx.currentlyReasoning) { ctx.reasoningBuffer += '<think>'; ctx.currentlyReasoning = true; }
        ctx.reasoningSource = 'sse';
        const startElapsed = Number.isFinite(Number(json.total_elapsed_ms || json.elapsed_ms)) ? Number(json.total_elapsed_ms || json.elapsed_ms) : null;
        showReasoningStatus(elementId, '...', false, startElapsed);
    }
    else if (json.type === 'reasoning.delta' && json.content) {
        ctx.reasoningBuffer += json.content;
        const reasoningLoop = await detectRunawayRepetitionAsync(ctx.reasoningBuffer, 'reasoning');
        if (reasoningLoop) {
            const repetitionError = new Error(config.llmMode === 'stateful' && !ctx.options.repeatRecoveryApplied ? t('warning.repeatRetrying') : t('warning.repeatStopped'));
            repetitionError.code = 'LMSTUDIO_RUNAWAY_REPETITION';
            ctx.streamRestartRequested = true;
            throw repetitionError;
        }
        ctx.currentlyReasoning = true;
        ctx.reasoningSource = 'sse';
        const elapsedMs = Number.isFinite(Number(json.total_elapsed_ms || json.elapsed_ms)) ? Number(json.total_elapsed_ms || json.elapsed_ms) : null;
        AppState.session.replay.reasoningBuffers.set(elementId, ctx.reasoningBuffer);
        showReasoningStatus(elementId, ctx.reasoningBuffer, false, elapsedMs);
    }
    else if (json.type === 'reasoning.end') {
        ctx.reasoningBuffer += '</think>\n';
        ctx.currentlyReasoning = false;
        const elapsedMs = Number.isFinite(Number(json.total_elapsed_ms || json.elapsed_ms)) ? Number(json.total_elapsed_ms || json.elapsed_ms) : null;
        ctx.reasoningSource = null;
        AppState.session.replay.reasoningBuffers.set(elementId, ctx.reasoningBuffer);
        finalizeReasoningStatus(elementId, 'done', ctx.reasoningBuffer, elapsedMs);
    }
    else if (json.type.startsWith('tool_call.')) {
        if (json.type === 'tool_call.start') {
            ctx.lastToolCallHtml = json.tool || 'Tool';
            setToolCardState(elementId, 'running', '', null, json.tool || 'Tool');
        } else if (json.type === 'tool_call.arguments') {
            setToolCardState(elementId, 'running', '', json.arguments, json.tool || 'Tool');
        } else if (json.type === 'tool_call.success') {
            setToolCardState(elementId, 'success', t('tool.executionFinished'));
        } else if (json.type === 'tool_call.failure') {
            setToolCardState(elementId, 'failure', json.reason || t('tool.unknownError'));
        }
    }
    else if (json.type === 'chat.end') { handleChatEndEvent(json, ctx); }
    else if (json.type === 'prompt_processing.progress') { renderProgressDock(t('progress.processingPrompt'), json.progress * 100, 'prompt-processing', false); }
    else if (json.type.startsWith('model_load.')) { handleModelLoadEvent(json, ctx); }
    else if (json.type === 'error') { handleErrorEvent(json, ctx); }

    if (contentToAdd) {
        await handleContentAddition(contentToAdd, speechToAdd, ctx);
    }
}

function finalizeStream(ctx) {
    if (AppState.ui.progress.active) hideProgressDock();

    if (!ctx.streamAborted && !ctx.streamDetached && !ctx.streamRestartRequested) {
        if (ctx.currentlyReasoning) {
            finalizeReasoningStatus(ctx.elementId, 'failed', t('status.unexpectedStop'));
        }
        if (getRunningToolCards(ctx.elementId).length > 0) {
            finalizeAssistantStatusCards(ctx.elementId, 'failed', t('status.failed'));
        }
    }

    const historyContent = sanitizeAssistantRenderText(ctx.fullText).trim();
    const ownershipReserved = AppState.session.localStreamOwnershipReleased;

    if (historyContent && !ctx.streamDetached && !ctx.streamRestartRequested && !ownershipReserved) {
        AppState.chat.messages.push({ role: 'assistant', content: historyContent, turnId: ctx.turnId });
        if (config.llmMode === 'stateful') {
            AppState.chat.stateful.turnCount += 1;
            AppState.chat.stateful.estimatedChars += historyContent.length;
        }
    }

    if (ctx.useStreamingTTS) {
        finalizeStreamingTTS(ctx.speechBuffer);
    }

    if (historyContent && !ctx.streamRestartRequested && !ownershipReserved) {
        finalizeMessageContent(ctx.elementId, historyContent);
        setAssistantActionBarReady(ctx.elementId);
    }

    if (AppState.chat.activeLocalAssistantId === ctx.elementId) {
        AppState.chat.activeLocalTurnId = '';
        AppState.chat.activeLocalAssistantId = '';
    }

    AppState.session.localStreamOwnershipReleased = false;
    syncWakeLock();

    return historyContent;
}

function extractFinalAssistantContentFromPayload(payload = {}) {
    const payloadMap = payload && typeof payload === 'object' ? payload : {};
    const result = payloadMap.result && typeof payloadMap.result === 'object' ? payloadMap.result : null;
    const output = Array.isArray(result?.output) ? result.output : [];
    const messageParts = output
        .filter((item) => item && item.type === 'message' && typeof item.content === 'string')
        .map((item) => item.content)
        .filter((content) => content.trim());
    if (messageParts.length > 0) {
        return messageParts.join('');
    }

    if (typeof payloadMap.final_assistant_content === 'string' && payloadMap.final_assistant_content.trim()) {
        return payloadMap.final_assistant_content;
    }

    return '';
}

function extractReasoningContentFromPayload(payload = {}) {
    const payloadMap = payload && typeof payload === 'object' ? payload : {};
    if (typeof payloadMap.reasoning_content === 'string' && payloadMap.reasoning_content.trim()) {
        return payloadMap.reasoning_content;
    }
    const result = payloadMap.result && typeof payloadMap.result === 'object' ? payloadMap.result : null;
    const output = Array.isArray(result?.output) ? result.output : [];
    const reasoningParts = output
        .filter((item) => item && item.type === 'reasoning' && typeof item.content === 'string')
        .map((item) => item.content)
        .filter((content) => content.trim());
    if (reasoningParts.length > 0) {
        return reasoningParts.join('\n\n');
    }

    return '';
}

function extractToolStateFromPayload(payload = {}) {
    const payloadMap = payload && typeof payload === 'object' ? payload : {};
    if (payloadMap.tool && typeof payloadMap.tool === 'object') {
        const tool = payloadMap.tool;
        const directState = {
            state: String(tool.state || 'success').trim() || 'success',
            summary: String(tool.summary || '').trim(),
            args: tool.args ?? null,
            toolName: String(tool.tool_name || tool.toolName || 'Tool').trim() || 'Tool',
            history: Array.isArray(tool.history) ? tool.history : []
        };
        return isMeaningfulToolState(directState) ? directState : null;
    }
    const result = payloadMap.result && typeof payloadMap.result === 'object' ? payloadMap.result : null;
    const output = Array.isArray(result?.output) ? result.output : [];
    const toolCalls = output.filter((item) => item && item.type === 'tool_call');
    if (toolCalls.length === 0) return null;

    const history = [];
    let toolName = '';
    let args = null;
    let state = 'success';

    toolCalls.forEach((item) => {
        const itemTool = String(item.tool || '').trim();
        if (itemTool) {
            toolName = itemTool;
        }
        if (item.arguments != null) {
            args = item.arguments;
            const detail = extractToolPreview(item.arguments, '', itemTool || toolName || 'Tool');
            if (detail) {
                const displayTool = formatToolDisplayName(itemTool || toolName || 'Tool');
                const signature = `${displayTool}::${detail}`;
                const last = history[history.length - 1];
                if (!last || last.signature !== signature) {
                    history.push({
                        signature,
                        tool: displayTool,
                        detail
                    });
                }
            }
        }
        if (typeof item.output === 'string' && !item.output.trim()) {
            state = 'failure';
        }
    });

    const extractedState = {
        state,
        summary: state === 'failure' ? t('tool.unknownError') : t('tool.executionFinished'),
        args,
        toolName: toolName || 'Tool',
        history
    };
    return isMeaningfulToolState(extractedState) ? extractedState : null;
}

function isMeaningfulToolState(toolState = null) {
    if (!toolState || typeof toolState !== 'object') return false;
    if (String(toolState.summary || '').trim()) return true;
    if (String(toolState.toolName || '').trim() && String(toolState.toolName || '').trim().toLowerCase() !== 'tool') return true;
    if (Array.isArray(toolState.history) && toolState.history.some((entry) => String(entry?.tool || '').trim() || String(entry?.detail || '').trim())) return true;
    if (toolState.args != null) {
        if (typeof toolState.args === 'string') return toolState.args.trim().length > 0;
        if (typeof toolState.args === 'object') {
            try {
                const serialized = JSON.stringify(toolState.args);
                return !!serialized && serialized !== '{}' && serialized !== '[]' && serialized !== 'null';
            } catch (_) {
                return true;
            }
        }
        return true;
    }
    return false;
}

function normalizeSessionTurnSnapshot(turn = {}) {
    const raw = turn && typeof turn === 'object' ? turn : {};
    const reasoningRaw = raw.reasoning && typeof raw.reasoning === 'object' ? raw.reasoning : {};
    const toolRaw = raw.tool && typeof raw.tool === 'object' ? raw.tool : null;
    return {
        turn_id: String(raw.turn_id || raw.turnId || '').trim(),
        status: String(raw.status || '').trim(),
        user_content: String(raw.user_content || '').trim(),
        assistant_content: String(raw.assistant_content || ''),
        reasoning: {
            state: String(reasoningRaw.state || '').trim(),
            content: String(reasoningRaw.content || raw.reasoning_content || ''),
            duration_ms: Number(reasoningRaw.duration_ms || raw.reasoning_duration_ms || 0),
            accumulated_ms: Number(reasoningRaw.accumulated_ms || raw.reasoning_accumulated_ms || 0),
            current_phase_ms: Number(reasoningRaw.current_phase_ms || raw.reasoning_current_phase_ms || 0)
        },
        tool: toolRaw ? {
            state: String(toolRaw.state || '').trim(),
            summary: String(toolRaw.summary || '').trim(),
            args: toolRaw.args ?? null,
            tool_name: String(toolRaw.tool_name || toolRaw.toolName || '').trim(),
            history: Array.isArray(toolRaw.history) ? toolRaw.history : []
        } : null
    };
}

function buildLegacySessionViewsFromTurns(turns = []) {
    const messages = [];
    const tool_cards = {};
    turns.forEach((turn) => {
        const normalized = normalizeSessionTurnSnapshot(turn);
        if (!normalized.turn_id) return;
        messages.push({
            turn_id: normalized.turn_id,
            user_content: normalized.user_content,
            assistant_content: normalized.assistant_content,
            reasoning_content: normalized.reasoning.content,
            reasoning_duration_ms: normalized.reasoning.duration_ms,
            reasoning_accumulated_ms: normalized.reasoning.accumulated_ms,
            reasoning_current_phase_ms: normalized.reasoning.current_phase_ms
        });
        if (normalized.tool) {
            tool_cards[normalized.turn_id] = {
                state: normalized.tool.state,
                summary: normalized.tool.summary,
                args: normalized.tool.args,
                tool_name: normalized.tool.tool_name,
                history: normalized.tool.history
            };
        }
    });
    return { messages, tool_cards };
}

function getSessionSnapshotTurns(sessionUISnapshot = {}) {
    const turns = Array.isArray(sessionUISnapshot?.turns) ? sessionUISnapshot.turns : [];
    if (turns.length > 0) {
        return turns.map((turn) => normalizeSessionTurnSnapshot(turn)).filter((turn) => turn.turn_id);
    }
    const messages = Array.isArray(sessionUISnapshot?.messages) ? sessionUISnapshot.messages : [];
    return messages.map((item) => {
        const turnId = String(item?.turn_id || '').trim();
        const toolState = sessionUISnapshot?.tool_cards?.[turnId] || null;
        return normalizeSessionTurnSnapshot({
            turn_id: turnId,
            user_content: item?.user_content || '',
            assistant_content: item?.assistant_content || '',
            reasoning_content: item?.reasoning_content || '',
            reasoning_duration_ms: item?.reasoning_duration_ms || 0,
            reasoning_accumulated_ms: item?.reasoning_accumulated_ms || 0,
            reasoning_current_phase_ms: item?.reasoning_current_phase_ms || 0,
            tool: toolState
        });
    }).filter((turn) => turn.turn_id);
}

// ============================================================================
// i18n Translation System
// ============================================================================

function extractRuntimeErrorMessage(errorLike) {
    if (typeof errorLike === 'string') {
        return errorLike.trim();
    }
    if (errorLike && typeof errorLike === 'object' && typeof errorLike.message === 'string') {
        return errorLike.message.trim();
    }
    return '';
}

function getSuggestedMcpServerUrl() {
    const defaultUrl = 'http://127.0.0.1:8081/mcp/sse';

    try {
        const protocol = String(window?.location?.protocol || '').toLowerCase();
        const host = String(window?.location?.host || '').trim();
        if ((protocol === 'http:' || protocol === 'https:') && host) {
            return `http://${host}/mcp/sse`;
        }
    } catch (_) {
        // Ignore window/location access failures and fall back below.
    }

    const configuredPort = String(
        document.getElementById('server-port')?.value
        || ''
    ).trim();
    if (configuredPort) {
        return `http://127.0.0.1:${configuredPort}/mcp/sse`;
    }

    return defaultUrl;
}

function isLMStudioPluginToolConnectionError(errorLike, rawMessage = '') {
    const errorType = errorLike && typeof errorLike === 'object'
        ? String(errorLike.type || '').trim()
        : '';
    const errorParam = errorLike && typeof errorLike === 'object'
        ? String(errorLike.param || '').trim()
        : '';
    const normalizedMessage = String(rawMessage || '').toLowerCase();

    return errorType === 'plugin_connection_error'
        || (
            errorParam === 'integrations'
            && normalizedMessage.includes('unable to get plugin tools')
        )
        || normalizedMessage.includes("unable to get plugin tools for 'mcp/dinkisstyle-gateway'")
        || normalizedMessage.includes("plugin identifier 'mcp/dinkisstyle-gateway'")
        || normalizedMessage.includes('plugin process exited with code 1');
}

function getLocalizedRuntimeErrorMessage(errorLike) {
    const rawMessage = extractRuntimeErrorMessage(errorLike);
    if (!rawMessage) return '';

    if (rawMessage.startsWith('LM_STUDIO_AUTH_ERROR: ')) {
        return t('error.authFailed') + rawMessage.replace('LM_STUDIO_AUTH_ERROR: ', '');
    }
    if (rawMessage.startsWith('LM_STUDIO_MCP_ERROR: ')) {
        return t('error.mcpFailed') + rawMessage.replace('LM_STUDIO_MCP_ERROR: ', '');
    }
    if (rawMessage.startsWith('LM_STUDIO_CONTEXT_ERROR: ')) {
        return t('error.contextExceeded');
    }
    if (rawMessage.startsWith('LM_STUDIO_VISION_ERROR: ')) {
        return t('error.visionNotSupported');
    }
    if (isLMStudioPluginToolConnectionError(errorLike, rawMessage)) {
        return t('error.mcpPluginToolsUnavailable')
            .replace('{url}', getSuggestedMcpServerUrl());
    }

    return '';
}

function t(key) {
    return translateKey(config.language || 'ko', key);
}

function applyTranslations() {
    const lang = config.language || 'ko';
    applyI18nTranslations({ language: lang, root: document });
    const langSelect = document.getElementById('cfg-lang');
    if (langSelect) langSelect.value = lang;
    if (savedLibrarySearchInput) {
        savedLibrarySearchInput.placeholder = t('library.searchPlaceholder');
    }
    renderContextStrategyOptions();
    updateMessageInputPlaceholder();
    renderSavedLibraryList();
    syncTemperatureUI();
}

function setLanguage(lang) {
    config.language = lang;
    persistClientConfig();
    applyTranslations();
    renderReasoningControl();
    syncTemperatureUI();
}

// ============================================================================
// Screen Wake Lock API
// ============================================================================

// Audio Context & Wake Lock Recovery for iOS/PWA
document.addEventListener('visibilitychange', async () => {
    if (document.visibilityState === 'visible') {
        console.log('[Audio] App foregrounded, checking recovery...');

        // Re-acquire Wake Lock if it should be active
        syncWakeLock();
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
document.addEventListener('pointerup', (event) => {
    if (event.pointerType !== 'touch') return;
    const active = document.activeElement;
    if (active instanceof HTMLElement && active.matches('button, .icon-btn')) {
        active.blur();
    }
});

function scheduleHeaderIconButtonBlur(target) {
    if (!(target instanceof HTMLElement)) return;
    if (!target.matches('#chat-header .icon-btn, #chat-header .header-library-btn')) return;

    window.setTimeout(() => {
        if (document.activeElement === target) {
            target.blur();
        }
    }, 180);

    window.setTimeout(() => {
        if (document.activeElement === target) {
            target.blur();
        }
    }, 1200);
}

document.addEventListener('click', (event) => {
    const button = event.target instanceof Element
        ? event.target.closest('#chat-header .icon-btn, #chat-header .header-library-btn')
        : null;
    if (button instanceof HTMLElement) {
        scheduleHeaderIconButtonBlur(button);
    }
}, true);

document.addEventListener('touchend', (event) => {
    const touch = event.target instanceof Element
        ? event.target.closest('#chat-header .icon-btn, #chat-header .header-library-btn')
        : null;
    if (touch instanceof HTMLElement) {
        scheduleHeaderIconButtonBlur(touch);
    }
}, { passive: true, capture: true });

document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
        closeModelPickerModal();
    }
});

/**
 * Holistic sync for Screen Wake Lock based on app state
 */
async function syncWakeLock() {
    const shouldBeActive = !!(
        AppState.chat.isGenerating
        || AppState.audio.isPlayingQueue
        || AppState.input.isSTTActive
        || AppState.audio.streamingTTSActive
    );
    if (shouldBeActive) {
        await requestWakeLock();
    } else {
        await releaseWakeLock();
    }
}

async function requestWakeLock() {
    if (!('AppState.ui.wakeLock' in navigator)) return;
    if (AppState.ui.wakeLock) return; // Already acquired

    try {
        AppState.ui.wakeLock = await navigator.wakeLock.request('screen');
        console.info('[WakeLock] Screen Wake Lock acquired');

        AppState.ui.wakeLock.addEventListener('release', () => {
            console.info('[WakeLock] Screen Wake Lock released');
            const stillBusy = !!(
                AppState.chat.isGenerating
                || AppState.audio.isPlayingQueue
                || AppState.input.isSTTActive
                || AppState.audio.streamingTTSActive
            );
            if (document.visibilityState === 'visible' && stillBusy) {
                AppState.ui.wakeLock = null;
                setTimeout(() => syncWakeLock(), 1000);
            } else {
                AppState.ui.wakeLock = null;
            }
        });
    } catch (err) {
        if (err.name !== 'NotAllowedError') {
            console.warn('[WakeLock] Request failed:', err);
        }
        AppState.ui.wakeLock = null;
    }
}

async function releaseWakeLock() {
    if (!AppState.ui.wakeLock) return;
    try {
        const lock = AppState.ui.wakeLock;
        AppState.ui.wakeLock = null;
        await lock.release();
    } catch (err) {
        console.warn('[WakeLock] Release failed:', err);
    }
}

// ============================================================================
// Settings Modal Control
// ============================================================================

/**
 * Fetch available models from LLM server and populate dropdown
 */
function fetchModels() {
    return modelController.fetchModels();
}

function openSettingsModal() {
    return modelController.openSettingsModal();
}

function closeSettingsModal() {
    return modelController.closeSettingsModal();
}

function normalizeTemperatureValue(value, fallback = null) {
    if (value === null || value === undefined || value === '' || value === 'auto') {
        return fallback;
    }
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) {
        return fallback;
    }
    const clamped = clampNumber(numeric, 0, 1);
    if (clamped <= 0) {
        return null;
    }
    return Math.round(clamped * 10) / 10;
}

function isTemperatureAuto(value = config.temperature) {
    return normalizeTemperatureValue(value, null) === null;
}

function formatTemperatureSettingLabel(value = config.temperature) {
    const normalized = normalizeTemperatureValue(value, null);
    return normalized === null ? t('setting.temperature.auto') : normalized.toFixed(1);
}

function syncTemperatureUI() {
    const triggerLabel = document.getElementById('cfg-temp-display');
    const slider = document.getElementById('temperature-modal-slider');
    const valueEl = document.getElementById('temperature-modal-value');
    const autoBtn = document.getElementById('temperature-modal-auto');
    const normalized = normalizeTemperatureValue(config.temperature, null);
    const sliderValue = normalized === null ? 0 : normalized;
    if (triggerLabel) triggerLabel.textContent = formatTemperatureSettingLabel(normalized);
    if (slider) slider.value = String(sliderValue);
    if (valueEl) valueEl.textContent = formatTemperatureSettingLabel(normalized);
    if (autoBtn) autoBtn.classList.toggle('btn-primary', normalized === null);
}

function openTemperatureModal() {
    syncTemperatureUI();
    document.getElementById('temperature-modal')?.classList.add('active');
}

function closeTemperatureModal() {
    document.getElementById('temperature-modal')?.classList.remove('active');
}

function setTemperatureAuto() {
    config.temperature = null;
    syncTemperatureUI();
    saveConfig(false);
}

function formatBytes(value) {
    const bytes = Number(value || 0);
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let size = bytes;
    let unitIndex = 0;
    while (size >= 1024 && unitIndex < units.length - 1) {
        size /= 1024;
        unitIndex += 1;
    }
    const digits = size >= 100 ? 0 : size >= 10 ? 1 : 2;
    return `${size.toFixed(digits)} ${units[unitIndex]}`;
}

function generateTurnId() {
    return `turn-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function getUserBubbleTheme(themeId) {
    return USER_BUBBLE_THEMES[themeId] || USER_BUBBLE_THEMES.ocean;
}

function applyUserBubbleTheme() {
    const theme = getUserBubbleTheme(config.userBubbleTheme);
    const root = document.documentElement;
    root.style.setProperty('--user-bubble-gradient', theme.gradient);
    root.style.setProperty('--user-bubble-border', theme.border);
    root.style.setProperty('--user-bubble-shadow', theme.shadow);
    root.style.setProperty('--user-text-color', theme.text);
    root.style.setProperty('--send-btn-gradient', theme.gradient);
    root.style.setProperty('--send-btn-border', theme.border);
    root.style.setProperty('--send-btn-shadow', theme.shadow);
    root.style.setProperty('--send-btn-text-color', theme.icon || theme.text);
}

function renderUserBubbleThemeOptions() {
    const host = document.getElementById('user-bubble-theme-options');
    if (!host) return;

    host.innerHTML = Object.values(USER_BUBBLE_THEMES).map((theme, index) => {
        const selected = theme.id === (config.userBubbleTheme || 'ocean');
        const textColor = escapeAttr(theme.text);
        const gradient = escapeAttr(theme.gradient);
        return `
            <button
                type="button"
                class="theme-swatch-btn ${selected ? 'is-selected' : ''}"
                role="radio"
                aria-checked="${selected ? 'true' : 'false'}"
                aria-label="Bubble theme ${index + 1}"
                data-theme-id="${theme.id}"
                onclick="selectUserBubbleTheme('${theme.id}')"
            >
                <div class="theme-swatch-preview" style="background: ${gradient}; color: ${textColor};">Aa</div>
            </button>
        `;
    }).join('');
}

function selectUserBubbleTheme(themeId) {
    if (!USER_BUBBLE_THEMES[themeId]) return;
    config.userBubbleTheme = themeId;
    applyUserBubbleTheme();
    renderUserBubbleThemeOptions();
    saveConfig(false);
}

function loadSavedTurns() {
    return savedLibraryController.loadSavedTurns();
}

function openSavedLibrary() {
    return savedLibraryController.openSavedLibrary();
}

function toggleSavedLibrary() {
    return savedLibraryController.toggleSavedLibrary();
}

function closeSavedLibrary() {
    return savedLibraryController.closeSavedLibrary();
}

function setupSavedLibrarySwipeGestures() {
    return savedLibraryController.setupSavedLibrarySwipeGestures();
}

function handleSavedLibrarySearch(value) {
    return savedLibraryController.handleSavedLibrarySearch(value);
}

function updateSavedLibrarySearchClearButton() {
    return savedLibraryController.updateSavedLibrarySearchClearButton();
}

function clearSavedLibrarySearch() {
    return savedLibraryController.clearSavedLibrarySearch();
}

function renderSavedLibraryList() {
    return savedLibraryController.renderSavedLibraryList();
}

function openSavedTurnModal(id) {
    return savedLibraryController.openSavedTurnModal(id);
}

function closeSavedTurnModal() {
    return savedLibraryController.closeSavedTurnModal();
}

function copySavedTurnResponse() {
    return savedLibraryController.copySavedTurnResponse();
}

function speakSavedTurnResponse(btn) {
    return savedLibraryController.speakSavedTurnResponse(btn);
}

function getActiveComposerBackgroundTask() {
    for (const task of AppState.ui.composerBackgroundTasks.values()) {
        if (task?.active) return task;
    }
    return null;
}

function updateComposerBackgroundTaskUI() {
    const hasBackgroundTask = !!getActiveComposerBackgroundTask();
    inputContainer?.classList.toggle('has-background-task', hasBackgroundTask && !AppState.chat.isGenerating && !AppState.ui.progress.active);
    updateMessageInputPlaceholder();
}

function setComposerBackgroundTask(id, task = {}) {
    if (!id) return;
    AppState.ui.composerBackgroundTasks.set(id, {
        id,
        active: true,
        label: task.label || '',
        abortController: task.abortController || null
    });
    updateComposerBackgroundTaskUI();
}

function clearComposerBackgroundTask(id, { abort = false } = {}) {
    if (!id) return;
    const existing = AppState.ui.composerBackgroundTasks.get(id);
    if (abort) {
        existing?.abortController?.abort?.();
    }
    AppState.ui.composerBackgroundTasks.delete(id);
    updateComposerBackgroundTaskUI();
}

function cancelComposerBackgroundTasks(reason = 'user-interrupt') {
    savedLibraryController.cancelBackgroundTasks(reason);
    for (const [id, task] of AppState.ui.composerBackgroundTasks.entries()) {
        task?.abortController?.abort?.(reason);
        AppState.ui.composerBackgroundTasks.delete(id);
    }
    updateComposerBackgroundTaskUI();
}

function isLikelyStreamDetachError(err) {
    const message = String(err?.message || err || '').toLowerCase();
    return err?.name === 'TypeError'
        || message.includes('load failed')
        || message.includes('fetch failed')
        || message.includes('failed to fetch')
        || message.includes('networkerror')
        || message.includes('network error')
        || message.includes('the network connection was lost');
}

function startEditSavedTurnTitle() {
    return savedLibraryController.startEditSavedTurnTitle();
}

function cancelEditSavedTurnTitle() {
    return savedLibraryController.cancelEditSavedTurnTitle();
}

function saveEditedSavedTurnTitle() {
    return savedLibraryController.saveEditedSavedTurnTitle();
}

function buildSavedTurnTitleRequestPayload(extra = {}) {
    const payload = {
        model_id: config.model || '',
        secondary_model: config.secondaryModel || '',
        api_token: config.apiToken || '',
        llm_mode: config.llmMode || 'standard',
        ...extra
    };
    const temperature = getConfiguredTemperature();
    if (temperature !== null) {
        payload.temperature = temperature;
    }
    return payload;
}

function saveTurn(promptText, responseText) {
    return savedLibraryController.saveTurn(promptText, responseText);
}

function deleteSavedTurn(id) {
    return savedLibraryController.deleteSavedTurn(id);
}

function refreshSavedTurnTitleById(id) {
    return savedLibraryController.refreshSavedTurnTitleById(id);
}

function getTurnDataFromAssistantButton(btn) {
    const messageEl = btn?.closest('.message.assistant');
    const turnId = messageEl?.dataset.turnId;
    if (!turnId) {
        console.warn('[SavedTurn] Missing turnId on assistant button', { button: btn, messageEl });
        return null;
    }

    const userEl = document.querySelector(`.message.user[data-turn-id="${turnId}"] .message-bubble`);
    const responseEl = messageEl.querySelector('.markdown-body');
    const committedResponseEl = responseEl?.querySelector('.markdown-committed');
    const pendingResponseEl = responseEl?.querySelector('.markdown-pending');
    const userMessage = AppState.chat.messages.find((entry) => entry?.role === 'user' && entry?.turnId === turnId);
    const assistantMessage = [...AppState.chat.messages].reverse().find((entry) => entry?.role === 'assistant' && entry?.turnId === turnId);

    const promptText = (userEl?.innerText || userMessage?.content || '').trim();
    const responseMarkdownSource = (
        committedResponseEl?.dataset?.markdownSource
        || responseEl?.dataset?.markdownSource
        || messageEl?._streamRenderState?.committedText
        || assistantMessage?.content
        || pendingResponseEl?.dataset?.markdownSource
        || responseEl?.innerText
        || ''
    ).trim();
    console.log('[SavedTurn] Extracted turn data candidates', {
        turnId,
        hasUserElement: !!userEl,
        hasResponseElement: !!responseEl,
        hasCommittedMarkdownSource: !!committedResponseEl?.dataset?.markdownSource,
        hasStreamingCommittedText: !!messageEl?._streamRenderState?.committedText,
        userMessageLen: (userMessage?.content || '').trim().length,
        assistantMessageLen: (assistantMessage?.content || '').trim().length,
        promptLen: promptText.length,
        responseLen: responseMarkdownSource.length,
        responsePreview: responseMarkdownSource.slice(0, 120)
    });
    if (!promptText || !responseMarkdownSource) {
        console.warn('[SavedTurn] Turn data incomplete', {
            turnId,
            promptText,
            responseText: responseMarkdownSource,
            userHtml: userEl?.outerHTML || null,
            responseHtml: responseEl?.outerHTML || null
        });
        return null;
    }
    return {
        promptText,
        responseText: responseMarkdownSource
    };
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
// Audio State
// DOM Elements
const chatMessages = document.getElementById('chat-messages');
const chatRestoreOverlay = document.getElementById('chat-restore-overlay');
const AUTO_SCROLL_THRESHOLD_PX = 80;
const scrollToBottomBtn = document.getElementById('scroll-to-bottom-btn');



function readChatScrollMetrics() {
    if (!chatMessages) {
        return { ...chatScrollMetrics };
    }
    const scrollTop = chatMessages.scrollTop;
    const scrollHeight = chatMessages.scrollHeight;
    const clientHeight = chatMessages.clientHeight;
    const distanceFromBottom = scrollHeight - clientHeight - scrollTop;
    return {
        scrollTop,
        scrollHeight,
        clientHeight,
        distanceFromBottom,
        nearBottom: distanceFromBottom <= AUTO_SCROLL_THRESHOLD_PX,
        longScrollable: (scrollHeight - clientHeight) > Math.max(320, Math.round(window.innerHeight * 0.45))
    };
}

function getStreamingScrollMode() {
    const mode = String(config.streamingScrollMode || 'auto').trim().toLowerCase();
    return mode === 'label-top' ? 'label-top' : 'auto';
}

function commitChatScrollMetrics(metrics) {
    AppState.ui.scroll.metrics = metrics;
    AppState.ui.scroll.lastObservedHeight = metrics.scrollHeight;
    return AppState.ui.scroll.metrics;
}

function refreshChatScrollMetrics() {
    return commitChatScrollMetrics(readChatScrollMetrics());
}

function scheduleChatScrollMetricsRefresh() {
    if (!chatMessages || AppState.ui.scroll.pendingFrame != null) return;
    AppState.ui.scroll.pendingFrame = requestAnimationFrame(() => {
        AppState.ui.scroll.pendingFrame = null;
        refreshChatScrollMetrics();
        updateScrollToBottomButton();
    });
}

if (chatMessages) {
    refreshChatScrollMetrics();
    chatMessages.addEventListener('scroll', () => {
        if (AppState.ui.scroll.suppressNextEvent) {
            AppState.ui.scroll.suppressNextEvent = false;
            updateScrollToBottomButton();
            return;
        }

        const metrics = refreshChatScrollMetrics();
        AppState.ui.scroll.shouldAutoScroll = metrics.nearBottom;
        if (!AppState.ui.scroll.shouldAutoScroll) {
            if (AppState.ui.scroll.holdAutoScrollTimeout) {
                clearTimeout(AppState.ui.scroll.holdAutoScrollTimeout);
                AppState.ui.scroll.holdAutoScrollTimeout = null;
            }
            AppState.ui.scroll.lockToLatest = false;
        }

        if (AppState.chat.isGenerating) {
            if (AppState.ui.scroll.shouldAutoScroll) {
                AppState.ui.scroll.lockToLatest = true;
            } else {
                if (metrics.distanceFromBottom > AUTO_SCROLL_THRESHOLD_PX * 2) {
                    AppState.ui.scroll.lockToLatest = false;
                }
            }
        }
        updateScrollToBottomButton();
    }, { passive: true });
}
const messageInput = document.getElementById('message-input');
const sendBtn = document.getElementById('send-btn');
const imagePreviewVal = document.getElementById('image-preview');
const previewContainer = document.getElementById('preview-container');
const chatProgressDock = document.getElementById('chat-progress-dock');
const inputArea = document.getElementById('input-area');
const inputContainer = document.querySelector('#input-area .input-container');
const reasoningControlBar = document.getElementById('reasoning-control-bar');
const composerReasoningSelect = document.getElementById('composer-reasoning-select');
const inlineMicBtn = document.getElementById('inline-mic-btn');
const statefulBudgetIndicator = document.getElementById('stateful-budget-indicator');
const savedLibraryView = document.getElementById('saved-library-view');
const savedLibraryList = document.getElementById('saved-library-list');
const savedLibrarySearchInput = document.getElementById('saved-library-search');
const savedTurnModal = document.getElementById('saved-turn-modal');
const savedTurnModalTitleView = document.getElementById('saved-turn-inline-title-view');
const savedTurnModalTitleEdit = document.getElementById('saved-turn-inline-title-edit');
const savedTurnModalTitleInput = document.getElementById('saved-turn-inline-title-input');
const savedTurnModalTitleSaveBtn = document.getElementById('saved-turn-inline-title-save');
const savedTurnModalTitleCancelBtn = document.getElementById('saved-turn-inline-title-cancel');
const osTTSVoiceSelect = document.getElementById('cfg-os-tts-voice');
let chatUIController = null;
let chatStreamingController = null;
let progressController = null;
let micController = null;

function getTTSPlaybackState() {
    return {
        currentAudio: AppState.audio.currentAudio,
        currentAudioBtn: AppState.audio.currentAudioBtn,
        audioWarmup: AppState.audio.audioWarmup,
        ttsQueue: AppState.audio.ttsQueue,
        activeTTSSessionLabel: AppState.audio.activeTTSSessionLabel,
        currentAudioPlaybackController: AppState.audio.currentAudioPlaybackController,
        isPlayingQueue: AppState.audio.isPlayingQueue,
        streamingTTSActive: AppState.audio.streamingTTSActive,
        streamingTTSCommittedIndex: AppState.audio.streamingTTSCommittedIndex,
        streamingTTSBuffer: AppState.audio.streamingTTSBuffer,
        ttsSessionId: AppState.audio.ttsSessionId
    };
}

function setTTSPlaybackState(patch = {}) {
    if (Object.prototype.hasOwnProperty.call(patch, 'currentAudio')) AppState.audio.currentAudio = patch.currentAudio;
    if (Object.prototype.hasOwnProperty.call(patch, 'currentAudioBtn')) AppState.audio.currentAudioBtn = patch.currentAudioBtn;
    if (Object.prototype.hasOwnProperty.call(patch, 'audioWarmup')) AppState.audio.audioWarmup = patch.audioWarmup;
    if (Object.prototype.hasOwnProperty.call(patch, 'ttsQueue')) AppState.audio.ttsQueue = patch.ttsQueue;
    if (Object.prototype.hasOwnProperty.call(patch, 'activeTTSSessionLabel')) AppState.audio.activeTTSSessionLabel = patch.activeTTSSessionLabel;
    if (Object.prototype.hasOwnProperty.call(patch, 'currentAudioPlaybackController')) AppState.audio.currentAudioPlaybackController = patch.currentAudioPlaybackController;
    if (Object.prototype.hasOwnProperty.call(patch, 'isPlayingQueue')) AppState.audio.isPlayingQueue = patch.isPlayingQueue;
    if (Object.prototype.hasOwnProperty.call(patch, 'streamingTTSActive')) AppState.audio.streamingTTSActive = patch.streamingTTSActive;
    if (Object.prototype.hasOwnProperty.call(patch, 'streamingTTSCommittedIndex')) AppState.audio.streamingTTSCommittedIndex = patch.streamingTTSCommittedIndex;
    if (Object.prototype.hasOwnProperty.call(patch, 'streamingTTSBuffer')) AppState.audio.streamingTTSBuffer = patch.streamingTTSBuffer;
    if (Object.prototype.hasOwnProperty.call(patch, 'ttsSessionId')) AppState.audio.ttsSessionId = patch.ttsSessionId;
}

const ttsController = appTTS.createTTSController({
    refs: {
        osTTSVoiceSelect
    },
    deps: {
        getActiveStreamingMessageId: () => AppState.ui.activeStreamingMessageId,
        config,
        escapeAttr,
        escapeHtml,
        getAudioCache: () => ttsAudioCache,
        getCachedAudioPromise: (text) => ttsAudioCache.get(text),
        getPlaybackState: () => getTTSPlaybackState(),
        getSpeakableTextFromMarkdownHost: (host) => getSpeakableTextFromMarkdownHost(host),
        getToastBottomOffset,
        onCombinedQueueConsumed: (texts) => {
            texts.forEach((text) => {
                if (AppState.audio.ttsQueue[0] === text) {
                    AppState.audio.ttsQueue.shift();
                } else {
                    const idx = AppState.audio.ttsQueue.indexOf(text);
                    if (idx >= 0) AppState.audio.ttsQueue.splice(idx, 1);
                }
                ttsAudioCache.delete(text);
            });
        },
        onDetachCurrentAudioPlaybackListeners: () => detachCurrentAudioPlaybackListeners(),
        onProcessQueue: () => processTTSQueue(),
        onSetAssistantActionBarReady: (elementId) => setAssistantActionBarReady(elementId),
        onSyncCurrentAudioButtonUI: () => syncCurrentAudioButtonUI(),
        onSyncWakeLock: () => syncWakeLock(),
        setPlaybackState: (patch) => setTTSPlaybackState(patch),
        t
    }
});
const modelController = appModels.createModelController({
    refs: {
        composerReasoningSelect,
        reasoningControlBar,
        scrollToBottomBtn
    },
    deps: {
        DEFAULT_REASONING_OPTIONS,
        config,
        enforceMCPPolicyForMode,
        escapeAttr,
        escapeHtml,
        isGenerating: () => AppState.chat.isGenerating,
        normalizeContextStrategyForMode,
        normalizeReasoningValue,
        persistClientConfig,
        saveConfig,
        showToast,
        t,
        triggerHaptic,
        updateComposerLayoutMetrics,
        updateReasoningControlVisibility,
        updateScrollToBottomButton
    }
});
const savedLibraryController = appSavedLibrary.createSavedLibraryController({
    refs: {
        savedLibraryView,
        savedLibraryList,
        savedLibrarySearchInput,
        savedTurnModal,
        savedTurnModalTitleView,
        savedTurnModalTitleEdit,
        savedTurnModalTitleInput,
        savedTurnModalTitleSaveBtn,
        savedTurnModalTitleCancelBtn
    },
    deps: {
        buildSavedTurnTitleRequestPayload,
        broadcastSavedTurnsChange,
        clearComposerBackgroundTask,
        escapeAttr,
        escapeHtml,
        fallbackCopyTextToClipboard,
        getCurrentUser: () => AppState.session.currentUser,
        onOpenStateChange: () => updateScrollToBottomButton(),
        renderMarkdownIntoHost,
        setComposerBackgroundTask,
        showToast,
        speakMessage,
        t,
        triggerHaptic
    }
});
const sessionController = appSession.createSessionController({
    deps: {
        buildSessionFetchOptions,
        getCurrentUser: () => AppState.session.currentUser,
        onExternalConfigSync: () => syncServerConfig({ log: true }),
        onLLMActivitySyncState: (state) => applyExternalLLMActivityState(state),
        onSavedTurnsExternalSync: () => loadSavedTurns()
    }
});

savedTurnModalTitleInput?.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
        event.preventDefault();
        saveEditedSavedTurnTitle();
    } else if (event.key === 'Escape') {
        event.preventDefault();
        cancelEditSavedTurnTitle();
    }
});

savedLibraryList?.addEventListener('touchmove', () => {
    if (document.activeElement === savedLibrarySearchInput) {
        savedLibrarySearchInput.blur();
    }
}, { passive: true });

savedLibraryList?.addEventListener('scroll', () => {
    if (document.activeElement === savedLibrarySearchInput) {
        savedLibrarySearchInput.blur();
    }
}, { passive: true });

function updateViewportMetrics() {
    const root = document.documentElement;
    const vv = window.visualViewport;
    const visibleHeight = vv ? vv.height : window.innerHeight;
    const offsetTop = vv ? vv.offsetTop : 0;
    const occupiedBottom = vv ? Math.max(0, window.innerHeight - (vv.height + vv.offsetTop)) : 0;
    const active = document.activeElement;
    const isEditable = active instanceof HTMLElement
        && (active.matches('textarea, input, [contenteditable="true"]') || active.isContentEditable);
    const now = Date.now();
    const keyboardSignalDetected = occupiedBottom > 120;
    const wasKeyboardOpen = document.body.classList.contains('keyboard-open');

    if (keyboardSignalDetected) {
        AppState.ui.lastKeyboardViewportSignalAt = now;
    }

    const keyboardLikelyOpen = keyboardSignalDetected
        || (wasKeyboardOpen && occupiedBottom > 56)
        || (isEditable && now - AppState.ui.lastKeyboardViewportSignalAt < 420);

    root.style.setProperty('--app-height', `${Math.round(visibleHeight + offsetTop)}px`);
    root.style.setProperty('--viewport-bottom-offset', `${Math.round(occupiedBottom)}px`);
    document.body.classList.toggle('keyboard-open', keyboardLikelyOpen);
    updateComposerLayoutMetrics();
    updateScrollToBottomButton();
}

function logScrollTrace(event, details = {}) {
    const ENABLE_SCROLL_TRACE = false;
    if (!ENABLE_SCROLL_TRACE) return;
    if (!console?.debug) return;
    const metrics = chatMessages ? {
        top: Math.round(chatMessages.scrollTop || 0),
        height: Math.round(chatMessages.scrollHeight || 0),
        client: Math.round(chatMessages.clientHeight || 0)
    } : null;
    console.debug('[ScrollTrace:App]', event, {
        activeUIMessageId: AppState.ui.activeStreamingMessageId,
        activeLocalAssistantId: AppState.chat.activeLocalAssistantId,
        activeLocalTurnId: AppState.chat.activeLocalTurnId,
        isGenerating: AppState.chat.isGenerating,
        metrics,
        ...details
    });
}

function restoreChatScrollPosition(scrollTop) {
    if (!chatMessages || !Number.isFinite(scrollTop)) return;
    const nextTop = Math.max(0, scrollTop);

    const apply = () => {
        AppState.ui.scroll.suppressNextEvent = true;
        chatMessages.scrollTop = nextTop;
        updateScrollToBottomButton();
    };

    requestAnimationFrame(() => {
        apply();
        requestAnimationFrame(apply);
    });
}

function runAfterViewportStabilizes(callback, { maxWaitMs = 420, requiredStableFrames = 2 } = {}) {
    if (typeof callback !== 'function') return () => { };

    const controller = new AbortController();
    let finished = false;
    let rafId = 0;
    let stableFrames = 0;
    let lastSignature = '';
    let timeoutId = 0;

    const getSignature = () => {
        const viewportHeight = Math.round(window.visualViewport?.height || window.innerHeight || 0);
        const viewportOffsetTop = Math.round(window.visualViewport?.offsetTop || 0);
        const scrollHeight = Math.round(chatMessages?.scrollHeight || 0);
        return `${viewportHeight}:${viewportOffsetTop}:${scrollHeight}`;
    };

    const cleanup = () => {
        controller.abort();
        if (rafId) cancelAnimationFrame(rafId);
        if (timeoutId) clearTimeout(timeoutId);
    };

    const finish = () => {
        if (finished) return;
        finished = true;
        cleanup();
    };

    const tick = () => {
        if (finished) return;
        callback();
        const signature = getSignature();
        if (signature === lastSignature) {
            stableFrames += 1;
        } else {
            stableFrames = 0;
            lastSignature = signature;
        }
        if (stableFrames >= requiredStableFrames) {
            finish();
            return;
        }
        rafId = requestAnimationFrame(tick);
    };

    if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', callback, { signal: controller.signal });
        window.visualViewport.addEventListener('scroll', callback, { signal: controller.signal });
    }
    window.addEventListener('resize', callback, { passive: true, signal: controller.signal });

    timeoutId = window.setTimeout(() => {
        callback();
        finish();
    }, maxWaitMs);

    rafId = requestAnimationFrame(tick);
    return finish;
}

function scheduleInputScrollRepair(scrollTop) {
    if (!Number.isFinite(scrollTop)) return;
    runAfterViewportStabilizes(() => {
        restoreChatScrollPosition(scrollTop);
    });
}

function focusMessageInput({ preserveChatScroll = true } = {}) {
    if (!messageInput) return;
    const previousScrollTop = preserveChatScroll ? (chatMessages?.scrollTop ?? null) : null;

    requestAnimationFrame(() => {
        if (document.activeElement !== messageInput) {
            try {
                messageInput.focus({ preventScroll: true });
            } catch (_) {
                messageInput.focus();
            }
        }
        if (previousScrollTop != null) {
            restoreChatScrollPosition(previousScrollTop);
        }
    });
}

function maintainInputFocusAfterTouch() {
    if (!messageInput) return;
    if (document.activeElement === messageInput) return;
    focusMessageInput({ preserveChatScroll: true });
}

function ensureChatRestoredToLatest() {
    runAfterViewportStabilizes(() => {
        scrollToBottom(true);
    }, { maxWaitMs: 360, requiredStableFrames: 1 });
}

if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', updateViewportMetrics);
    window.visualViewport.addEventListener('scroll', updateViewportMetrics);
}
window.addEventListener('resize', updateViewportMetrics, { passive: true });
window.addEventListener('orientationchange', updateViewportMetrics);
updateViewportMetrics();

async function unlockAudioContext() {
    return ttsController.unlockAudioContext();
}

/**
 * Update Media Session Metadata
 */
function updateMediaSessionMetadata(text) {
    return ttsController.updateMediaSessionMetadata(text);
}

function clearMediaSessionMetadata() {
    return ttsController.clearMediaSessionMetadata();
}

function readWavHeader(view) {
    return ttsController.readWavHeader(view);
}

function concatenateWavArrayBuffers(buffers) {
    return ttsController.concatenateWavArrayBuffers(buffers);
}

async function combinePlayableChunks(primaryUrl, queuedTexts) {
    return ttsController.combinePlayableChunks(primaryUrl, queuedTexts);
}


// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    updateViewportMetrics();
    closeSavedLibrary();
    setupSavedLibrarySwipeGestures();
    updateMessageInputPlaceholder();
    // Check authentication first
    await checkAuth();

    // Initial Config Load
    try {
        loadConfig();
    } catch (e) {
        console.error("Config load failed, using defaults:", e);
    }

    fetchModels().catch(console.warn); // Fetch models in background

    await loadVoiceStyles(); // Fetch voice styles
    initOSTTSVoiceLoading();
    await syncServerConfig({ log: true, forceDictionaryReload: true }); // Sync with server
    sessionController.setupSyncListeners();
    setupEventListeners();
    initServerControl();
    if (window.runtime?.EventsOn) {
        window.runtime.EventsOn('llm-activity', (state) => {
            AppState.chat.passive.llmBusy = !!state?.busy;
            syncGlobalLLMComposerUI();
        });
    }
    sessionController.setupLLMActivitySyncListener();

    // Setup Markdown
    marked.setOptions({
        gfm: true,
        breaks: true,
        highlight: function (code, lang) {
            const hljs = window.hljs;
            if (!hljs) return code;
            const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext';
            return hljs.highlight(code, { language }).value;
        },
        langPrefix: 'hljs language-'
    });

    if (window.markdownEngineReady?.then) {
        window.markdownEngineReady.then((renderer) => {
            if (!renderer?.render) return;
            rerenderAllMarkdownHosts();
        }).catch((error) => {
            console.warn('[Markdown] remark renderer warm-up failed', error);
        });
    }

    // Initial chat restore first, then startup/health UI only if needed.
    try {
        await bootstrapInitialChatView();
    } catch (e) {
        console.warn('Initial chat bootstrap failed:', e);
    }

    // Start Location Tracking
    updateUserLocation();
    setInterval(updateUserLocation, 300000); // Update every 5 mins

    // Register Service Worker for PWA
    if ('serviceWorker' in navigator) {
        window.addEventListener('load', () => {
            navigator.serviceWorker.register('/sw.js?v=6')
                .then(reg => console.log('[PWA] Service Worker registered:', reg.scope))
                .catch(err => console.warn('[PWA] Service Worker failed:', err));
        });
    }

    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            stopSTT({ suppressAutoSend: true, forceAbort: true });
            relinquishLocalStreamOwnership('document-hidden');
            cancelComposerBackgroundTasks('document-hidden');
            clearReconnectWatchdog();
            clearForegroundSyncTimers();
            stopChatSessionEventStream();
        } else {
            savedLibraryController.scheduleSavedTitleRefresh(800);
            restartChatSessionEventStream();
            scheduleForegroundSessionRefresh('visibility-visible');
        }
    });

    window.addEventListener('pagehide', () => {
        stopSTT({ suppressAutoSend: true, forceAbort: true });
        relinquishLocalStreamOwnership('pagehide');
        stopChatSessionEventStream();
    });

    window.addEventListener('pageshow', () => {
        restartChatSessionEventStream();
        scheduleForegroundSessionRefresh('pageshow');
    });

    window.addEventListener('focus', () => {
        if (!AppState.session.eventStreamConnected) {
            restartChatSessionEventStream();
        }
        scheduleForegroundSessionRefresh('focus');
    });

    window.addEventListener('online', () => {
        if (!AppState.session.eventStreamConnected) {
            restartChatSessionEventStream();
        }
        scheduleForegroundSessionRefresh('online');
    });
});

// Current user state

function isPassiveServerSession(session = AppState.session.currentCache) {
    return !!session
        && session.Status === 'running'
        && !AppState.chat.abortController;
}

function getPassiveSyncWaitingText() {
    return t('chat.passiveSyncWaiting');
}

function getPassiveGenerationLabel(phase = '') {
    const normalized = String(phase || '').trim().toLowerCase();
    if (normalized === 'tool_call') {
        return t('chat.passiveSyncTool');
    }
    if (normalized === 'thinking') {
        return t('chat.passiveSyncThinking');
    }
    return getPassiveSyncWaitingText();
}

function syncPassiveGenerationUI(session = AppState.session.currentCache, phase = '') {
    const isPassiveRunning = isPassiveServerSession(session);
    inputContainer?.classList.toggle('is-passive-generating', isPassiveRunning);
    if (!isPassiveRunning) {
        AppState.chat.passive.placeholder = '';
        clearComposerBackgroundTask('passive-server-chat');
        updateMessageInputPlaceholder();
        return;
    }
    AppState.chat.passive.placeholder = getPassiveGenerationLabel(phase);
    setComposerBackgroundTask('passive-server-chat', {
        label: AppState.chat.passive.placeholder
    });
    updateMessageInputPlaceholder();
}

function syncGlobalLLMComposerUI(session = AppState.session.currentCache) {
    const localGenerating = !!AppState.chat.abortController;
    const sessionRunning = AppState.chat.passive.sessionLLMRunning || String(session?.Status || '').trim().toLowerCase() === 'running';
    const active = localGenerating || sessionRunning || AppState.chat.passive.llmBusy || AppState.chat.passive.externalLLMBusy;
    const passiveBusy = sessionRunning && !localGenerating;

    inputContainer?.classList.toggle('is-llm-active', active);

    if (passiveBusy) {
        AppState.chat.passive.placeholder = getPassiveGenerationLabel(AppState.chat.passive.externalLLMPhase);
        inputContainer?.classList.add('is-passive-generating');
        setComposerBackgroundTask('passive-server-chat', {
            label: AppState.chat.passive.placeholder
        });
    } else if (!isPassiveServerSession()) {
        inputContainer?.classList.remove('is-passive-generating');
        AppState.chat.passive.placeholder = '';
        clearComposerBackgroundTask('passive-server-chat');
    }

    updateMessageInputPlaceholder();

    // Ensure the send button reflects the global active state
    // We call a variant that doesn't trigger syncGlobalLLMComposerUI again to avoid loop
    updateSendButtonStateCore();
}

function applyExternalLLMActivityState(state = {}) {
    AppState.chat.passive.externalLLMBusy = !!state?.busy;
    AppState.chat.passive.externalLLMPhase = String(state?.phase || '').trim().toLowerCase();
    if (!AppState.chat.passive.externalLLMBusy && !isPassiveServerSession()) {
        AppState.chat.passive.placeholder = '';
    }
    syncGlobalLLMComposerUI();
}

function broadcastLLMActivityState(busy, phase = 'answering') {
    return sessionController.broadcastLLMActivityState(busy, phase);
}

function ensurePassiveSyncPlaceholder(turnId = '', sessionId = 'default', eventSeq = 0, phase = '') {
    const resolvedTurnId = String(turnId || `server-turn-${sessionId || 'default'}-${eventSeq || '0'}`).trim();
    if (!resolvedTurnId) return '';

    AppState.session.replay.currentTurnId = resolvedTurnId;
    const assistantId = ensureServerReplayAssistant(resolvedTurnId, sessionId, eventSeq);
    if (!assistantId) return '';

    AppState.session.replay.currentAssistantId = assistantId;

    const assistantEl = document.getElementById(assistantId);
    const existingContent = AppState.session.replay.messageBuffers.get(assistantId) || assistantEl?._streamRenderState?.committedText || '';
    const isPlaceholder = existingContent === getPassiveSyncWaitingText();
    const hasRealContent = !!existingContent && !isPlaceholder;

    if (!hasRealContent) {
        AppState.session.replay.messageBuffers.delete(assistantId);
        AppState.session.replay.reasoningBuffers.delete(assistantId);
        updateSyncedMessageContent(assistantId, getPassiveSyncWaitingText(), { animate: false });
    }

    syncPassiveGenerationUI(AppState.session.currentCache, phase);

    const actionBar = assistantEl?.querySelector('.message-actions');
    if (actionBar) {
        actionBar.hidden = true;
        actionBar.classList.remove('is-ready', 'is-pending');
    }
    return assistantId;
}

function ensurePassiveSyncPlaceholderForRunningSession(session = AppState.session.currentCache) {
    return '';
}

function syncSnapshotUserMessages(session = AppState.session.currentCache) {
    const sessionUISnapshot = getCurrentChatSessionUISnapshot(session);
    const snapshotTurns = getSessionSnapshotTurns(sessionUISnapshot);
    snapshotTurns.forEach((turn, index) => {
        const turnId = String(turn?.turn_id || '').trim();
        const userContent = String(turn?.user_content || '').trim();
        if (!turnId || !userContent) return;
        let userEl = document.querySelector(`.message.user[data-turn-id="${turnId}"]`);
        if (!userEl) {
            userEl = createMessageElement({ role: 'user', content: userContent, turnId });
        }

        const assistantEl = document.querySelector(`.message.assistant[data-turn-id="${turnId}"]`);
        let anchor = assistantEl?.parentNode === chatMessages ? assistantEl : null;
        if (!anchor) {
            for (let nextIndex = index + 1; nextIndex < snapshotTurns.length; nextIndex += 1) {
                const nextTurnId = String(snapshotTurns[nextIndex]?.turn_id || '').trim();
                if (!nextTurnId) continue;
                anchor = document.querySelector(`.message.user[data-turn-id="${nextTurnId}"]`)
                    || document.querySelector(`.message.assistant[data-turn-id="${nextTurnId}"]`);
                if (anchor?.parentNode === chatMessages) break;
                anchor = null;
            }
        }

        if (userEl.parentNode !== chatMessages) {
            if (anchor) {
                chatMessages.insertBefore(userEl, anchor);
            } else {
                chatMessages.appendChild(userEl);
            }
            updateScrollToBottomButton();
        } else if (assistantEl?.parentNode === chatMessages) {
            const children = Array.from(chatMessages.children);
            const userIndex = children.indexOf(userEl);
            const assistantIndex = children.indexOf(assistantEl);
            if (userIndex > assistantIndex) {
                chatMessages.insertBefore(userEl, assistantEl);
                updateScrollToBottomButton();
            }
        } else if (anchor && !!(userEl.compareDocumentPosition(anchor) & Node.DOCUMENT_POSITION_PRECEDING)) {
            chatMessages.insertBefore(userEl, anchor);
            updateScrollToBottomButton();
        }
        if (!AppState.chat.messages.some((entry) => entry?.role === 'user' && entry?.turnId === turnId && entry?.content === userContent)) {
            AppState.chat.messages.push({ role: 'user', content: userContent, turnId });
        }
    });
}

function broadcastConfigSync() {
    return sessionController.broadcastConfigSync();
}

function dismissReconnectNoticeCard() {
    const reconnectMessages = Array.from(document.querySelectorAll('.message.has-reconnect-card'));
    reconnectMessages.forEach((msgEl) => {
        if (msgEl.classList.contains('is-dismissing')) return;
        msgEl.classList.add('is-dismissing');
        window.setTimeout(() => {
            if (msgEl.parentNode) {
                msgEl.remove();
            }
        }, 320);
    });
    AppState.session.reconnectCardVisible = false;
}

function clearReconnectWatchdog() {
    if (!AppState.session.reconnectWatchdogTimer) return;
    clearTimeout(AppState.session.reconnectWatchdogTimer);
    AppState.session.reconnectWatchdogTimer = null;
}

function markChatSessionRealtimeHealthy() {
    clearReconnectWatchdog();
    dismissReconnectNoticeCard();
}

function clearForegroundSyncTimers() {
    if (!AppState.session.foregroundSyncTimers.length) return;
    AppState.session.foregroundSyncTimers.forEach((timerId) => clearTimeout(timerId));
    AppState.session.foregroundSyncTimers = [];
}

function scheduleForegroundSessionRefresh(reason = 'foreground') {
    if (document.hidden) return;

    clearForegroundSyncTimers();
    if (!AppState.session.eventStreamConnected) {
        restartChatSessionEventStream();
    }
    armReconnectWatchdog();

    const delays = [0, 900, 2600];
    AppState.session.foregroundSyncTimers = delays.map((delay, index) => window.setTimeout(async () => {
        if (document.hidden) return;
        try {
            await refreshSessionStateFromServer();
        } catch (error) {
            console.warn(`[ChatSession] Foreground sync failed (${reason}, retry ${index + 1}/${delays.length}):`, error);
        } finally {
            if (index === delays.length - 1) {
                AppState.session.foregroundSyncTimers = [];
            }
        }
    }, delay));
}

function showReconnectNoticeCard() {
    if (AppState.session.reconnectCardVisible || hasSubstantiveChatMessages() === false) return;
    const reconnectMsg = {
        role: 'assistant',
        startup: {
            kind: 'reconnect',
            title: t('chat.reconnect.title'),
            body: t('chat.reconnect.body'),
            issues: [],
            actionLabel: t('chat.reconnect.action'),
            actionHandler: 'retryChatReconnect()'
        }
    };
    appendMessage(reconnectMsg, { skipScroll: true });
    AppState.session.reconnectCardVisible = true;
}

function armReconnectWatchdog(delay = 4200) {
    clearReconnectWatchdog();
    if (document.hidden) return;
    if (AppState.session.eventStreamConnected) return;
    AppState.session.reconnectWatchdogTimer = window.setTimeout(() => {
        AppState.session.reconnectWatchdogTimer = null;
        if (AppState.session.eventStreamConnected) return;
        showReconnectNoticeCard();
    }, Math.max(1500, delay));
}

async function retryChatReconnect() {
    dismissReconnectNoticeCard();
    restartChatSessionEventStream();
    armReconnectWatchdog(5200);
    try {
        await checkAuth();
        const results = await Promise.allSettled([
            syncCurrentChatSessionFromServer(),
            checkSystemHealth()
        ]);
        const failed = results.some((result) => result.status === 'rejected');
        if (failed) {
            showReconnectNoticeCard();
            return;
        }
        dismissReconnectNoticeCard();
    } catch (error) {
        console.warn('Manual reconnect failed:', error);
        showReconnectNoticeCard();
    } finally {
        clearReconnectWatchdog();
    }
}

function broadcastSavedTurnsChange(reason = 'updated') {
    return sessionController.broadcastSavedTurnsChange(reason);
}

function restoreLastSessionIntoChatView() {
    if (!AppState.session.lastCache || hasSubstantiveChatMessages()) return false;
    const userText = String(AppState.session.lastCache.user_message || '').trim();
    const assistantText = String(AppState.session.lastCache.assistant_message || '').trim();
    if (!userText || !assistantText) return false;

    const turnId = generateTurnId();
    const restoredUser = { role: 'user', content: userText, turnId };
    const restoredAssistant = { role: 'assistant', content: assistantText, turnId };
    appendMessage(restoredUser, { skipScroll: true });
    appendMessage(restoredAssistant, { skipScroll: true });
    AppState.chat.messages.push(restoredUser, restoredAssistant);
    holdAutoScrollAtBottom(1200);
    ensureChatRestoredToLatest();
    return true;
}

function clearLastSessionRetryTimer() {
    if (!AppState.session.retryTimer) return;
    clearTimeout(AppState.session.retryTimer);
    AppState.session.retryTimer = null;
}

function relinquishLocalStreamOwnership(reason = 'server-sync') {
    if (!AppState.chat.activeLocalTurnId && !AppState.chat.activeLocalAssistantId && !AppState.chat.isGenerating) return false;
    console.info('[ChatSession] Relinquishing local stream ownership:', reason, {
        turnId: AppState.chat.activeLocalTurnId,
        assistantId: AppState.chat.activeLocalAssistantId
    });
    AppState.chat.activeLocalTurnId = '';
    AppState.chat.activeLocalAssistantId = '';
    AppState.chat.locallyRenderedTurnIds = new Set();
    AppState.chat.isGenerating = false;
    broadcastLLMActivityState(false, 'finished');
    AppState.session.localStreamOwnershipReleased = true;
    hideProgressDock();
    updateSendButtonState();
    return true;
}

async function ensureLastSessionCacheLoaded(force = false) {
    if (force) {
        AppState.session.lastFetchPromise = null;
    } else if (AppState.session.lastCache) {
        return AppState.session.lastCache;
    }

    if (!AppState.session.lastFetchPromise) {
        AppState.session.lastFetchPromise = fetchLastSession()
            .then((data) => {
                AppState.session.lastCache = data;
                return data;
            })
            .finally(() => {
                AppState.session.lastFetchPromise = null;
            });
    }

    return AppState.session.lastFetchPromise;
}

async function refreshSessionStateFromServer() {
    if (AppState.session.refreshPromise) {
        AppState.session.pendingRefresh = true;
        return AppState.session.refreshPromise;
    }

    AppState.session.refreshPromise = (async () => {
        try {
            do {
                AppState.session.pendingRefresh = false;

                if (!AppState.session.currentUser) {
                    await checkAuth();
                }
                if (!AppState.session.currentUser) return;

                await syncCurrentChatSessionFromServer();
            } while (AppState.session.pendingRefresh);

            markChatSessionRealtimeHealthy();
        } catch (error) {
            console.warn('Session refresh failed:', error);
            clearReconnectWatchdog();
            showReconnectNoticeCard();
            throw error;
        } finally {
            AppState.session.refreshPromise = null;
        }
    })();

    return AppState.session.refreshPromise;
}

async function bootstrapInitialChatView() {
    if (AppState.session.didInitialBootstrap) return;
    AppState.session.didInitialBootstrap = true;

    let restoredFromSession = false;
    try {
        await syncCurrentChatSessionFromServer();
        restoredFromSession = hasSubstantiveChatMessages();
    } catch (e) {
        console.warn('Initial chat session sync failed:', e);
    }

    startChatSessionEventStream();

    await checkSystemHealth();
}

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
            AppState.ui.location.currentUserLocation = JSON.stringify({
                lat: latitude,
                lon: longitude,
                acc: accuracy
            });
            console.log("[Location] Updated:", AppState.ui.location.currentUserLocation);
            AppState.ui.location.lastErrorMessage = '';
        },
        (err) => {
            const nextMessage = String(err?.message || 'Unknown geolocation error');
            if (nextMessage !== AppState.ui.location.lastErrorMessage) {
                console.warn("[Location] Error:", nextMessage);
                AppState.ui.location.lastErrorMessage = nextMessage;
            }
            AppState.ui.location.currentUserLocation = null;
        },
        { enableHighAccuracy: false, timeout: 10000, maximumAge: 600000 } // 10 min cache
    );
}

// Check authentication status
async function checkAuth() {
    try {
        const response = await fetch('/api/auth/check', buildSessionFetchOptions());
        const data = await response.json();

        if (!data.authenticated) {
            stopChatSessionEventStream();
            localStorage.removeItem('sessionToken');
            window.location.href = '/login.html';
            return;
        }

        AppState.session.currentUser = {
            id: data.user_id,
            role: data.role
        };

        loadSavedTurns();

        // Show admin features if admin
        if (AppState.session.currentUser.role === 'admin') {
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
                ${u.id !== AppState.session.currentUser.id ? `<button class="icon-btn" onclick="deleteUser('${u.id}')" title="Delete"><span class="material-icons-round">delete</span></button>` : ''}
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
        stopChatSessionEventStream();
        localStorage.removeItem('sessionToken');
        await fetch('/api/logout', buildSessionFetchOptions({ method: 'POST' }));
        window.location.href = '/login.html';
    } catch (e) {
        console.error('Logout failed:', e);
    }
}

async function logoutAllSessions() {
    const confirmation = t('action.logoutAllSessions') === '모든 위치에서 로그아웃'
        ? '이 계정의 모든 로그인 유지 세션을 해제하고, 모든 위치에서 로그아웃할까요?'
        : 'Log out this account from every device and browser now?';
    if (!confirm(confirmation)) return;

    try {
        stopChatSessionEventStream();
        const response = await fetch('/api/logout-all-sessions', buildSessionFetchOptions({ method: 'POST' }));
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        localStorage.removeItem('sessionToken');
        window.location.href = '/login.html';
    } catch (e) {
        console.error('Logout all sessions failed:', e);
        alert(`Logout failed: ${e.message}`);
    }
}

// Server state

// Initialize server control and check status
async function initServerControl() {
    // Check if Wails runtime is available
    if (typeof window.go === 'undefined') {
        console.log('Wails runtime not detected. Running in web mode.');
        const serverControl = document.querySelector('.server-control');
        if (serverControl) {
            serverControl.style.display = 'none';
        }
        // Web mode: do not add is-desktop class
        return;
    }

    // Desktop mode: add class to show desktop-only elements
    document.body.classList.add('is-desktop');

    // Get initial server status
    try {
        const status = await window.go.core.App.GetServerStatus();
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
        if (AppState.session.serverRunning) {
            await window.go.core.App.StopServer();
            updateServerUI(false, port);
        } else {
            // Also update LLM endpoint
            // const llmEndpoint = document.getElementById('cfg-api').value; // UI Element removed
            await window.go.core.App.SetLLMEndpoint(config.apiEndpoint);
            await window.go.core.App.StartServer(port);
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
    AppState.session.serverRunning = running;
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
            // Keep the original config object reference so controllers created
            // before loadConfig() continue observing the latest settings.
            Object.assign(config, JSON.parse(saved));
        } catch (e) {
            console.error('Failed to parse saved config:', e);
            // Optional: localStorage.removeItem('appConfig');
        }
    }

    config.ttsEngine = config.ttsEngine === 'os' ? 'os' : 'supertonic';
    config.temperature = normalizeTemperatureValue(config.temperature, null);
    delete config.maxTokens;
    config.reasoning = normalizeReasoningValue(config.reasoning);
    config.showReasoningControl = config.showReasoningControl !== false;
    config.forceShowReasoningControl = config.forceShowReasoningControl === true;
    config.hapticsEnabled = config.hapticsEnabled !== false;
    config.osTtsRate = Number(config.osTtsRate) > 0 ? Number(config.osTtsRate) : 1.0;
    config.osTtsPitch = Number(config.osTtsPitch) >= 0 ? Number(config.osTtsPitch) : 1.0;
    config.voiceInputAutoTTS = config.voiceInputAutoTTS !== false;
    config.contextStrategy = normalizeContextStrategyForMode(config.llmMode, config.contextStrategy);
    config.enableMCP = enforceMCPPolicyForMode(config.llmMode);

    // Update UI
    const cfgApi = document.getElementById('cfg-api');
    if (cfgApi) cfgApi.value = config.apiEndpoint;
    document.getElementById('cfg-model').value = config.model;
    const secondaryModelEl = document.getElementById('cfg-secondary-model');
    if (secondaryModelEl) secondaryModelEl.value = config.secondaryModel || '';
    document.getElementById('cfg-hide-think').checked = config.hideThink;
    document.getElementById('cfg-show-reasoning-control').checked = config.showReasoningControl;
    document.getElementById('cfg-force-show-reasoning-control').checked = config.forceShowReasoningControl;
    document.getElementById('cfg-history').value = config.historyCount;
    const apiTokenEl = document.getElementById('cfg-api-token');
    if (apiTokenEl) apiTokenEl.value = config.apiToken || '';
    document.getElementById('cfg-llm-mode').value = config.llmMode || 'standard';
    renderContextStrategyOptions();
    document.getElementById('cfg-context-strategy').value = config.contextStrategy;
    const mcpEl = document.getElementById('cfg-enable-mcp');
    if (mcpEl) mcpEl.checked = config.enableMCP || false;
    updateSettingsVisibility(); // Update UI visibility based on mode
    renderReasoningControl();
    document.getElementById('cfg-enable-tts').checked = config.enableTTS;

    // Load Memory Setting
    const memEl = document.getElementById('setting-enable-memory');
    if (memEl) memEl.checked = config.enableMemory || false;
    const memControls = document.getElementById('memory-controls');
    if (memControls) memControls.style.display = config.enableMemory ? 'block' : 'none';
    const statefulTurnLimitEl = document.getElementById('cfg-stateful-turn-limit');
    if (statefulTurnLimitEl) statefulTurnLimitEl.value = parseInt(config.statefulTurnLimit, 10) || DEFAULT_STATEFUL_TURN_LIMIT;
    const statefulCharBudgetEl = document.getElementById('cfg-stateful-char-budget');
    if (statefulCharBudgetEl) statefulCharBudgetEl.value = parseInt(config.statefulCharBudget, 10) || DEFAULT_STATEFUL_CHAR_BUDGET;
    const statefulTokenBudgetEl = document.getElementById('cfg-stateful-token-budget');
    if (statefulTokenBudgetEl) statefulTokenBudgetEl.value = parseInt(config.statefulTokenBudget, 10) || DEFAULT_STATEFUL_TOKEN_BUDGET;

    document.getElementById('cfg-auto-tts').checked = config.autoTTS || false;
    const voiceInputAutoTTSEl = document.getElementById('cfg-voice-input-auto-tts');
    if (voiceInputAutoTTSEl) voiceInputAutoTTSEl.checked = config.voiceInputAutoTTS !== false;
    document.getElementById('cfg-tts-engine').value = config.ttsEngine || 'supertonic';
    document.getElementById('cfg-tts-lang').value = config.ttsLang;
    document.getElementById('cfg-enable-embeddings').checked = config.enableEmbeddings || false;
    document.getElementById('cfg-embedding-model').value = config.embeddingModelId || 'multilingual-e5-small';
    document.getElementById('cfg-chunk-size').value = config.chunkSize || 300;
    document.getElementById('cfg-system-prompt').value = config.systemPrompt || 'You are a helpful AI assistant.';
    if (config.ttsVoice) document.getElementById('cfg-tts-voice').value = String(config.ttsVoice).replace(/\.json$/i, '');
    document.getElementById('cfg-tts-speed').value = config.ttsSpeed || 1.0;
    document.getElementById('speed-val').textContent = config.ttsSpeed || 1.0;
    document.getElementById('cfg-tts-steps').value = config.ttsSteps || 5;
    document.getElementById('steps-val').textContent = config.ttsSteps || 5;
    document.getElementById('cfg-tts-threads').value = config.ttsThreads || 4;
    document.getElementById('threads-val').textContent = config.ttsThreads || 4;
    document.getElementById('cfg-os-tts-rate').value = config.osTtsRate || 1.0;
    document.getElementById('os-rate-val').textContent = config.osTtsRate || 1.0;
    document.getElementById('cfg-os-tts-pitch').value = config.osTtsPitch || 1.0;
    document.getElementById('os-pitch-val').textContent = config.osTtsPitch || 1.0;
    let format = config.ttsFormat || 'wav';
    if (format === 'mp3') format = 'mp3-high'; // Legacy mapping
    document.getElementById('cfg-tts-format').value = format;
    populateOSTTSVoiceList();
    if (config.osTtsVoiceURI && osTTSVoiceSelect && ttsController.isVoicesReady()) {
        osTTSVoiceSelect.value = config.osTtsVoiceURI;
    }
    syncOSTTSVoiceConfigFromSelection();
    updateTTSSettingsVisibility();

    // Mic Layout
    document.getElementById('cfg-mic-layout').value = config.micLayout || 'none';
    AppState.ui.micLayout = config.micLayout;

    config.userBubbleTheme = USER_BUBBLE_THEMES[config.userBubbleTheme] ? config.userBubbleTheme : 'ocean';
    config.streamingScrollMode = ['auto', 'label-top'].includes(config.streamingScrollMode) ? config.streamingScrollMode : 'auto';
    config.markdownRenderMode = ['fast', 'balanced', 'final'].includes(config.markdownRenderMode) ? config.markdownRenderMode : 'balanced';
    applyUserBubbleTheme();
    renderUserBubbleThemeOptions();
    const streamingScrollModeEl = document.getElementById('cfg-streaming-scroll-mode');
    if (streamingScrollModeEl) streamingScrollModeEl.value = config.streamingScrollMode;
    const markdownRenderModeEl = document.getElementById('cfg-markdown-render-mode');
    if (markdownRenderModeEl) markdownRenderModeEl.value = config.markdownRenderMode;
    const hapticsEl = document.getElementById('cfg-enable-haptics');
    if (hapticsEl) hapticsEl.checked = config.hapticsEnabled;
    syncHapticsPreference();

    // Language selector
    document.getElementById('cfg-lang').value = config.language || 'ko';

    // Update header with model name
    updateHeaderModelDisplay();
    renderModelPickerModal();

    applyChatFontSize();
    updateComposerLayoutMetrics();

    // Apply i18n translations
    // Apply i18n translations
    applyTranslations();
    syncTemperatureUI();

    // Initialize System Prompt Presets (loads from external file)
    loadSystemPrompts();

    // Load TTS Dictionary
    loadTTSDictionary(getEffectiveTTSDictionaryLang());

    // Setup settings listeners
    setupSettingsListeners();
}

function updateSettingsVisibility() {
    const mode = document.getElementById('cfg-llm-mode').value;
    const tokenContainer = document.getElementById('container-api-token');
    const historyContainer = document.getElementById('container-history');
    const contextStrategyContainer = document.getElementById('container-context-strategy');
    const mcpContainer = document.getElementById('container-enable-mcp');
    const memContainer = document.getElementById('container-enable-memory');
    const statefulBudgetContainer = document.getElementById('container-stateful-budget');
    const micLayoutValue = document.getElementById('cfg-mic-layout')?.value || config.micLayout || 'none';
    config.llmMode = mode;
    renderContextStrategyOptions();
    config.contextStrategy = normalizeContextStrategyForMode(mode, document.getElementById('cfg-context-strategy')?.value || config.contextStrategy);
    config.enableMCP = enforceMCPPolicyForMode(mode);
    const contextStrategyEl = document.getElementById('cfg-context-strategy');
    if (contextStrategyEl) {
        contextStrategyEl.value = config.contextStrategy;
    }

    const showToken = true;
    const showHistory = usesHistoryConversationContext();
    const showMCP = mode === 'stateful';
    const showStatefulBudget = usesStatefulConversationContext();
    const voiceInputAutoTTSContainer = document.getElementById('container-voice-input-auto-tts');
    const mcpEl = document.getElementById('cfg-enable-mcp');
    if (mcpEl) {
        mcpEl.checked = config.enableMCP;
    }

    if (tokenContainer) tokenContainer.style.display = showToken ? 'block' : 'none';
    if (historyContainer) historyContainer.style.display = showHistory ? 'block' : 'none';
    if (contextStrategyContainer) contextStrategyContainer.style.display = 'block';
    if (mcpContainer) mcpContainer.style.display = showMCP ? 'block' : 'none';
    if (statefulBudgetContainer) statefulBudgetContainer.style.display = showStatefulBudget ? 'block' : 'none';
    if (voiceInputAutoTTSContainer) voiceInputAutoTTSContainer.style.display = micLayoutValue !== 'none' ? 'block' : 'none';

    if (memContainer) memContainer.style.display = usesRetrievalConversationContext() ? 'block' : 'none';
}

function renderContextStrategyOptions() {
    return modelController.renderContextStrategyOptions();
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
        { id: 'cfg-tts-threads', val: 'threads-val' },
        { id: 'cfg-os-tts-rate', val: 'os-rate-val' },
        { id: 'cfg-os-tts-pitch', val: 'os-pitch-val' }
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
    const autoSaveIds = ['cfg-api', 'cfg-tts-lang', 'cfg-tts-voice', 'cfg-os-tts-voice', 'cfg-tts-format', 'cfg-chunk-size', 'cfg-system-prompt', 'cfg-llm-mode', 'cfg-context-strategy', 'cfg-show-reasoning-control', 'cfg-force-show-reasoning-control', 'cfg-stateful-turn-limit', 'cfg-stateful-char-budget', 'cfg-stateful-token-budget', 'cfg-secondary-model', 'cfg-tts-engine', 'cfg-streaming-scroll-mode', 'cfg-markdown-render-mode', 'cfg-enable-haptics', 'cfg-embedding-model', 'cfg-mic-layout'];
    autoSaveIds.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.onchange = () => saveConfig(false);
    });
    const temperatureSlider = document.getElementById('temperature-modal-slider');
    if (temperatureSlider) {
        temperatureSlider.oninput = () => {
            config.temperature = normalizeTemperatureValue(temperatureSlider.value, null);
            syncTemperatureUI();
        };
        temperatureSlider.onchange = () => saveConfig(false);
    }

    if (composerReasoningSelect) {
        composerReasoningSelect.onchange = () => {
            config.reasoning = normalizeReasoningValue(composerReasoningSelect.value);
            persistClientConfig();
            renderReasoningControl();
        };
    }

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
            const uid = (typeof AppState.session.currentUser !== 'undefined' && AppState.session.currentUser) ? AppState.session.currentUser.id : "default";
            try {
                const err = await window.go.core.App.OpenMemoryFolder(uid);
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
            const uid = (typeof AppState.session.currentUser !== 'undefined' && AppState.session.currentUser) ? AppState.session.currentUser.id : "default";
            try {
                const res = await window.go.core.App.ResetMemory(uid);
                alert(t('setting.memory.reset.success') || res);
            } catch (e) {
                alert("Error resetting memory: " + e);
            }
        };
    }
}

// Global Dictionary State

function buildServerConfigSignature(serverCfg) {
    try {
        return JSON.stringify({
            llm_endpoint: serverCfg?.llm_endpoint || '',
            llm_mode: serverCfg?.llm_mode || '',
            context_strategy: serverCfg?.context_strategy || '',
            secondary_model: serverCfg?.secondary_model || '',
            enable_tts: serverCfg?.enable_tts === true,
            enable_mcp: serverCfg?.enable_mcp === true,
            enable_memory: serverCfg?.enable_memory === true,
            stateful_turn_limit: Number(serverCfg?.stateful_turn_limit || 0),
            stateful_char_budget: Number(serverCfg?.stateful_char_budget || 0),
            stateful_token_budget: Number(serverCfg?.stateful_token_budget || 0),
            embedding_config: serverCfg?.embedding_config || null,
            tts_config: serverCfg?.tts_config || null
        });
    } catch (_) {
        return '';
    }
}

async function loadTTSDictionary(lang, options = {}) {
    // Default to config language or 'ko' if undefined
    const targetLang = lang || config.ttsLang || 'ko';
    const forceReload = options.forceReload === true;
    const log = options.log !== false;

    if (!forceReload && AppState.audio.dictionaryLang === targetLang && (AppState.audio.dictionaryRegex || Object.keys(AppState.audio.dictionary).length > 0)) {
        return AppState.audio.dictionary;
    }
    if (!forceReload && AppState.audio.dictionaryLoadPromise && AppState.audio.dictionaryLang === targetLang) {
        return AppState.audio.dictionaryLoadPromise;
    }

    let rawDict = {};

    AppState.audio.dictionaryLoadPromise = (async () => {
        try {
            if (window.go && window.go.main && window.go.core.App) {
                rawDict = await window.go.core.App.GetTTSDictionary(targetLang);
            } else {
                const res = await fetch(`/api/dictionary?lang=${targetLang}`);
                if (res.ok) rawDict = await res.json();
            }

            // Normalize keys to lowercase for case-insensitive lookup
            AppState.audio.dictionary = {};
            if (rawDict) {
                for (const [k, v] of Object.entries(rawDict)) {
                    AppState.audio.dictionary[k.toLowerCase()] = v;
                }
            }

            // Build optimized regex for performance (O(N) replacement)
            const keys = Object.keys(AppState.audio.dictionary);
            if (keys.length > 0) {
                const escapedKeys = keys.map(k => k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
                AppState.audio.dictionaryRegex = new RegExp(`\\b(${escapedKeys.join('|')})\\b`, 'gi');
            } else {
                AppState.audio.dictionaryRegex = null;
            }
            AppState.audio.dictionaryLang = targetLang;

            if (log) {
                console.log(`[TTS] Dictionary loaded with ${keys.length} entries. (lang=${targetLang})`);
            }
            return AppState.audio.dictionary;
        } catch (e) {
            console.error("Failed to load dictionary:", e);
            throw e;
        } finally {
            AppState.audio.dictionaryLoadPromise = null;
        }
    })();

    return AppState.audio.dictionaryLoadPromise;
}

// 시스템 프롬프트 프리셋 (외부 파일에서 로드)

async function loadSystemPrompts() {
    try {
        if (window.go && window.go.main && window.go.core.App) {
            AppState.ui.systemPromptPresets = await window.go.core.App.GetSystemPrompts();
        } else {
            const res = await fetch('/api/prompts');
            if (res.ok) AppState.ui.systemPromptPresets = await res.json();
        }
        console.log(`[Prompts] Loaded ${AppState.ui.systemPromptPresets.length} system prompts.`);
        initSystemPromptPresets(); // Re-initialize dropdown with loaded data
    } catch (e) {
        console.error("Failed to load system prompts:", e);
        AppState.ui.systemPromptPresets = [{ title: "Default", prompt: "You are a helpful AI assistant." }];
    }
}

function applySystemPromptPreset(key) {
    const preset = AppState.ui.systemPromptPresets.find(p => p.title === key);
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

    for (const preset of AppState.ui.systemPromptPresets) {
        const option = document.createElement('option');
        option.value = preset.title;
        option.textContent = preset.title;
        selector.appendChild(option);
    }
}

// 외부 파일(system_prompts.json, dictionary/dictionary_*.txt) 새로고침
async function reloadExternalFiles() {
    try {
        await loadSystemPrompts();
        await loadTTSDictionary(getEffectiveTTSDictionaryLang());
        await fetchModels(); // Reload models
        showToast(t('action.reload') + ' ✓');
    } catch (e) {
        console.error("Failed to reload external files:", e);
        showToast('Reload failed');
    }
}

function saveConfig(closeModal = true) {
    const previousVoice = config.ttsVoice;
    const previousEngine = config.ttsEngine;
    const previousDictionaryLang = getEffectiveTTSDictionaryLang();
    const cfgApiEl = document.getElementById('cfg-api');
    // Sanitize Endpoint: Trim whitespace and trailing slash
    let endpoint = cfgApiEl ? cfgApiEl.value.trim() : config.apiEndpoint;
    if (endpoint.endsWith('/')) {
        endpoint = endpoint.slice(0, -1);
    }
    config.apiEndpoint = endpoint;

    config.model = document.getElementById('cfg-model').value.trim();
    config.secondaryModel = document.getElementById('cfg-secondary-model')?.value?.trim() || '';
    config.hideThink = document.getElementById('cfg-hide-think').checked;
    config.showReasoningControl = document.getElementById('cfg-show-reasoning-control').checked;
    config.forceShowReasoningControl = document.getElementById('cfg-force-show-reasoning-control').checked;
    config.temperature = normalizeTemperatureValue(config.temperature, null);
    delete config.maxTokens;
    config.historyCount = parseInt(document.getElementById('cfg-history').value);
    config.enableTTS = document.getElementById('cfg-enable-tts').checked;

    // Save MCP setting
    const mcpEl = document.getElementById('cfg-enable-mcp');
    config.enableMCP = mcpEl ? mcpEl.checked : false;

    // Save Memory setting
    const memEl = document.getElementById('setting-enable-memory');
    config.enableMemory = memEl ? memEl.checked : false;

    config.autoTTS = document.getElementById('cfg-auto-tts').checked;
    const voiceInputAutoTTSEl = document.getElementById('cfg-voice-input-auto-tts');
    config.voiceInputAutoTTS = voiceInputAutoTTSEl ? voiceInputAutoTTSEl.checked : true;
    config.ttsEngine = document.getElementById('cfg-tts-engine').value || 'supertonic';
    config.ttsLang = document.getElementById('cfg-tts-lang').value;
    config.enableEmbeddings = document.getElementById('cfg-enable-embeddings').checked;
    config.embeddingProvider = 'local';
    config.embeddingModelId = document.getElementById('cfg-embedding-model').value || 'multilingual-e5-small';

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
    config.contextStrategy = normalizeContextStrategyForMode(config.llmMode, document.getElementById('cfg-context-strategy')?.value);
    config.enableMCP = enforceMCPPolicyForMode(config.llmMode);
    config.reasoning = normalizeReasoningValue(config.reasoning);
    config.statefulTurnLimit = Math.max(1, parseInt(document.getElementById('cfg-stateful-turn-limit')?.value, 10) || DEFAULT_STATEFUL_TURN_LIMIT);
    config.statefulCharBudget = Math.max(1000, parseInt(document.getElementById('cfg-stateful-char-budget')?.value, 10) || DEFAULT_STATEFUL_CHAR_BUDGET);
    config.statefulTokenBudget = Math.max(1000, parseInt(document.getElementById('cfg-stateful-token-budget')?.value, 10) || DEFAULT_STATEFUL_TOKEN_BUDGET);
    config.micLayout = document.getElementById('cfg-mic-layout').value;
    config.userBubbleTheme = USER_BUBBLE_THEMES[config.userBubbleTheme] ? config.userBubbleTheme : 'ocean';
    config.streamingScrollMode = document.getElementById('cfg-streaming-scroll-mode')?.value === 'label-top' ? 'label-top' : 'auto';
    config.markdownRenderMode = document.getElementById('cfg-markdown-render-mode')?.value || 'balanced';
    config.hapticsEnabled = document.getElementById('cfg-enable-haptics')?.checked !== false;
    config.chatFontSize = Math.max(12, Math.min(24, parseInt(config.chatFontSize, 10) || 16));

    config.chunkSize = parseInt(document.getElementById('cfg-chunk-size').value) || 300;
    config.systemPrompt = document.getElementById('cfg-system-prompt').value.trim() || 'You are a helpful AI assistant.';
    config.ttsVoice = document.getElementById('cfg-tts-voice').value;
    config.ttsSpeed = parseFloat(document.getElementById('cfg-tts-speed').value);
    config.ttsSteps = parseInt(document.getElementById('cfg-tts-steps').value);
    config.ttsThreads = parseInt(document.getElementById('cfg-tts-threads').value);
    config.ttsFormat = document.getElementById('cfg-tts-format').value;
    config.osTtsRate = parseFloat(document.getElementById('cfg-os-tts-rate').value) || 1.0;
    config.osTtsPitch = parseFloat(document.getElementById('cfg-os-tts-pitch').value) || 1.0;

    // Update visibility immediately
    updateSettingsVisibility();
    updateTTSSettingsVisibility();
    
    if (config.ttsVoice !== previousVoice || config.ttsEngine !== previousEngine) {
        clearTTSAudioCache();
    }
    
    AppState.ui.micLayout = config.micLayout;
    applyUserBubbleTheme();
    applyChatFontSize();
    syncHapticsPreference();
    renderReasoningControl();

    if (osTTSVoiceSelect) {
        config.osTtsVoiceURI = osTTSVoiceSelect.value || '';
        const selectedVoice = ttsController.getVoices().find((voice) => voice.voiceURI === config.osTtsVoiceURI) || null;
        config.osTtsVoiceName = selectedVoice?.name || '';
        config.osTtsVoiceLang = selectedVoice?.lang || '';
    }

    persistClientConfig();

    const nextDictionaryLang = getEffectiveTTSDictionaryLang();
    if (nextDictionaryLang !== previousDictionaryLang) {
        loadTTSDictionary(nextDictionaryLang, { log: true });
    }

    // Sync configs to server
    if (window.go && window.go.main && window.go.core.App) {
        window.go.core.App.SetLLMEndpoint(config.apiEndpoint).catch(console.error);
        window.go.core.App.SetLLMApiToken(config.apiToken).catch(console.error);
        window.go.core.App.SetLLMMode(config.llmMode).catch(console.error);
        window.go.core.App.SetEnableTTS(config.enableTTS);
        window.go.core.App.SetEnableMCP(config.enableMCP);
        window.go.core.App.SetServerTTSConfig({
            engine: config.ttsEngine,
            voiceStyle: config.ttsVoice,
            speed: config.ttsSpeed,
            threads: config.ttsThreads,
            osVoiceURI: config.osTtsVoiceURI,
            osVoiceName: config.osTtsVoiceName,
            osVoiceLang: config.osTtsVoiceLang,
            osRate: config.osTtsRate,
            osPitch: config.osTtsPitch
        }).catch(console.error);
        window.go.core.App.SetEmbeddingModelConfig({
            provider: config.embeddingProvider,
            modelId: config.embeddingModelId,
            enabled: config.enableEmbeddings
        }).catch(console.error);

        // This is separate from saveConfig in app.go, but SetTTSThreads triggers reload
        if (config.ttsThreads && config.ttsEngine === 'supertonic') {
            window.go.core.App.SetTTSThreads(config.ttsThreads);
        }
    }

    // Also try fetch for web mode or as backup
    // Build config payload - only include api_token if it was explicitly changed by user
    const configPayload = {
        api_endpoint: config.apiEndpoint,
        secondary_model: config.secondaryModel,
        llm_mode: config.llmMode,
        context_strategy: config.contextStrategy,
        enable_tts: config.enableTTS,
        enable_mcp: config.enableMCP,
        enable_memory: config.enableMemory,
        stateful_turn_limit: config.statefulTurnLimit,
        stateful_char_budget: config.statefulCharBudget,
        stateful_token_budget: config.statefulTokenBudget,
        tts_threads: config.ttsThreads,
        embedding_config: {
            provider: config.embeddingProvider,
            modelId: config.embeddingModelId,
            enabled: config.enableEmbeddings
        },
        tts_config: {
            engine: config.ttsEngine,
            voiceStyle: config.ttsVoice,
            speed: config.ttsSpeed,
            threads: config.ttsThreads,
            osVoiceURI: config.osTtsVoiceURI,
            osVoiceName: config.osTtsVoiceName,
            osVoiceLang: config.osTtsVoiceLang,
            osRate: config.osTtsRate,
            osPitch: config.osTtsPitch
        }
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
        broadcastConfigSync();
    }).catch(e => console.warn('Sync error:', e));


    // Update header model name
    updateHeaderModelDisplay();
    renderModelPickerModal();

    // Trigger explicit model load via backend
    if (config.model && config.apiEndpoint && config.apiEndpoint.includes('localhost')) {
        // Only try to auto-load if it looks like a local server (speed optimization)
        // Or we can just try always. Let's try always but non-blocking.
        fetch('/api/models', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: config.model,
                mode: config.llmMode || 'standard'
            })
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
    return sessionController.fetchLastSession();
}

async function fetchCurrentChatSession() {
    return sessionController.fetchCurrentChatSession();
}

async function fetchCurrentChatSessionEvents(afterSeq = 0, limit = 400) {
    return sessionController.fetchCurrentChatSessionEvents(afterSeq, limit);
}

async function fastForwardChatSessionEvents(limit = 400) {
    if (!AppState.session.currentUser) return;

    const result = await fetchCurrentChatSessionEvents(AppState.session.eventSeq, limit);
    if (result.session) {
        applyCurrentChatSessionSnapshot(result.session);
    }
    if (Array.isArray(result.items)) {
        for (const entry of result.items) {
            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
        }
    }

    const totalCount = Number(result.totalCount || 0);
    let loadedCount = Array.isArray(result.items) ? result.items.length : 0;
    let afterSeq = loadedCount > 0
        ? Number(result.items[result.items.length - 1].EventSeq || AppState.session.eventSeq)
        : AppState.session.eventSeq;

    while (loadedCount < totalCount) {
        const page = await fetchCurrentChatSessionEvents(afterSeq, limit);
        if (page.session) {
            applyCurrentChatSessionSnapshot(page.session);
        }
        const pageItems = Array.isArray(page.items) ? page.items : [];
        if (pageItems.length === 0) break;
        for (const entry of pageItems) {
            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
        }
        loadedCount += pageItems.length;
        afterSeq = Number(pageItems[pageItems.length - 1].EventSeq || afterSeq);
    }
}

function stopChatSessionPolling() {
    if (AppState.session.pollTimer) {
        clearTimeout(AppState.session.pollTimer);
        AppState.session.pollTimer = null;
    }
}

function clearChatSessionEventStreamRetry() {
    if (AppState.session.eventStreamRetryTimer) {
        clearTimeout(AppState.session.eventStreamRetryTimer);
        AppState.session.eventStreamRetryTimer = null;
    }
}

function stopChatSessionEventStream() {
    clearChatSessionEventStreamRetry();
    AppState.session.eventStreamConnected = false;
    if (AppState.session.eventStreamSource) {
        sessionController.closeCurrentChatSessionEventStream(AppState.session.eventStreamSource);
        AppState.session.eventStreamSource = null;
    }
}

function restartChatSessionEventStream() {
    stopChatSessionEventStream();
    startChatSessionEventStream();
}

function scheduleChatSessionEventStreamReconnect(delay = 1200) {
    if (!AppState.session.currentUser) return;
    clearChatSessionEventStreamRetry();
    AppState.session.eventStreamRetryTimer = window.setTimeout(() => {
        AppState.session.eventStreamRetryTimer = null;
        startChatSessionEventStream();
    }, Math.max(300, Number(delay) || 1200));
}

function shouldDeferLiveChatSessionEvent(entry = null) {
    if (!entry || typeof entry !== 'object') return false;
    if (AppState.session.isRestoring) return true;
    if (AppState.chat.activeLocalTurnId || AppState.chat.activeLocalAssistantId) return false;
    if (AppState.session.eventSeq > 0) return false;
    if (hasSubstantiveChatMessages()) return false;
    const eventType = String(entry.EventType || '').trim();
    return eventType !== '' && eventType !== 'session.cleared';
}

function startChatSessionEventStream() {
    if (!AppState.session.currentUser) return;
    if (AppState.session.eventStreamSource) return;

    const source = sessionController.openCurrentChatSessionEventStream({
        afterSeq: AppState.session.eventSeq,
        onOpen: () => {
            AppState.session.eventStreamConnected = true;
            AppState.session.eventStreamLastErrorAt = 0;
            clearChatSessionEventStreamRetry();
            markChatSessionRealtimeHealthy();
        },
        onSession: (payload) => {
            AppState.session.eventStreamConnected = true;
            markChatSessionRealtimeHealthy();
            if (payload?.has_session) {
                applyCurrentChatSessionSnapshot(payload.session || null);
            } else {
                applyCurrentChatSessionSnapshot(null);
            }
        },
        onEvent: (entry) => {
            if (!entry || typeof entry !== 'object') return;
            if (shouldDeferLiveChatSessionEvent(entry)) {
                syncCurrentChatSessionFromServer().catch((error) => {
                    console.warn('Deferred live chat event sync failed:', error);
                });
                return;
            }
            AppState.session.eventStreamConnected = true;
            markChatSessionRealtimeHealthy();
            dismissStartupCards();
            applyCurrentChatSessionEvent(entry);
            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
        },
        onError: (_event, payload) => {
            const now = Date.now();
            const wasConnected = AppState.session.eventStreamConnected;
            AppState.session.eventStreamConnected = false;
            if (payload?.message) {
                console.warn('Chat session SSE payload error:', payload.message);
            }
            if (source.readyState === EventSource.CLOSED) {
                stopChatSessionEventStream();
                scheduleChatSessionEventStreamReconnect(wasConnected ? 900 : 1800);
                scheduleChatSessionPolling(500);
                armReconnectWatchdog(wasConnected ? 3000 : 5200);
                return;
            }
            if (!AppState.session.eventStreamLastErrorAt || now - AppState.session.eventStreamLastErrorAt > 1000) {
                AppState.session.eventStreamLastErrorAt = now;
                scheduleChatSessionPolling(500);
                armReconnectWatchdog(wasConnected ? 3200 : 5200);
            }
        }
    });

    if (!source) {
        scheduleChatSessionPolling(900);
        return;
    }

    AppState.session.eventStreamSource = source;
}

function scheduleChatSessionPolling(delay = 1000) {
    stopChatSessionPolling();
    if (AppState.session.eventStreamConnected) {
        return;
    }
    AppState.session.pollTimer = window.setTimeout(() => {
        AppState.session.pollTimer = null;
        syncCurrentChatSessionFromServer().catch((error) => {
            console.warn('Failed to sync chat session from server:', error);
        });
    }, Math.max(200, delay));
}

function resetServerChatReplayState() {
    AppState.session.eventSeq = 0;
    AppState.session.clearedAt = '';
    AppState.session.replay.currentTurnId = '';
    AppState.session.replay.currentAssistantId = '';
    AppState.session.replay.messageBuffers = new Map();
    AppState.session.replay.reasoningBuffers = new Map();
}

function extractSessionClearedAt(session) {
    if (!session || !session.ClearedAt) return '';
    const raw = session.ClearedAt;
    if (typeof raw === 'string') return raw;
    if (typeof raw === 'object') {
        if (raw.Valid === true && raw.Time) return String(raw.Time);
        if (raw.time) return String(raw.time);
    }
    return '';
}

function getCurrentChatSessionUISnapshot(session = null) {
    const raw = String((session?.UIStateJSON ?? AppState.session.currentCache?.UIStateJSON ?? '')).trim();
    if (!raw) return { tool_cards: {}, messages: [], turns: [], last_event_seq: 0 };
    try {
        const parsed = JSON.parse(raw);
        const turns = Array.isArray(parsed?.turns) ? parsed.turns.map((turn) => normalizeSessionTurnSnapshot(turn)).filter((turn) => turn.turn_id) : [];
        const legacyFromTurns = turns.length > 0 ? buildLegacySessionViewsFromTurns(turns) : null;
        return {
            tool_cards: legacyFromTurns?.tool_cards || (parsed?.tool_cards && typeof parsed.tool_cards === 'object' ? parsed.tool_cards : {}),
            messages: legacyFromTurns?.messages || (Array.isArray(parsed?.messages) ? parsed.messages : []),
            turns,
            last_event_seq: Number(parsed?.last_event_seq || 0)
        };
    } catch (_) {
        return { tool_cards: {}, messages: [], turns: [], last_event_seq: 0 };
    }
}

function hydrateChatSessionUISnapshot(sessionSnapshot = null) {
    const sessionUISnapshot = getCurrentChatSessionUISnapshot(sessionSnapshot);
    const snapshotTurns = getSessionSnapshotTurns(sessionUISnapshot);
    if (snapshotTurns.length === 0) return false;
    const passiveRunningSession = isPassiveServerSession(sessionSnapshot);
    const lastItem = snapshotTurns[snapshotTurns.length - 1];
    const pendingTurnId = (passiveRunningSession && lastItem && !hasAssistantSnapshotContent(lastItem.assistant_content, lastItem.reasoning.content, lastItem.tool))
        ? String(lastItem.turn_id).trim()
        : '';

    AppState.chat.messages = [];

    const fragment = document.createDocumentFragment();
    let lastTurnId = '';
    let lastAssistantId = '';

    snapshotTurns.forEach((item, index) => {
        const turnId = String(item?.turn_id || `snapshot-turn-${index + 1}`);
        const assistantId = buildServerAssistantMessageId(turnId, `snapshot-turn-${index + 1}`);
        const snapshotToolState = item.tool || sessionUISnapshot.tool_cards?.[turnId] || null;
        const waitingForRemoteReply = passiveRunningSession && turnId === pendingTurnId;
        const hasAssistantContent = waitingForRemoteReply || hasAssistantSnapshotContent(item?.assistant_content, item?.reasoning?.content, snapshotToolState);
        lastTurnId = turnId;
        if (hasAssistantContent) {
            lastAssistantId = assistantId;
        }

        if (item?.user_content) {
            appendMessage({ role: 'user', content: item.user_content, turnId }, { parent: fragment, skipScroll: true });
            AppState.chat.messages.push({ role: 'user', content: item.user_content, turnId });
        }
        if (hasAssistantContent) {
            appendMessage({ role: 'assistant', content: '', id: assistantId, turnId }, { parent: fragment, skipScroll: true });
        }
    });

    if (fragment.childNodes.length > 0) {
        chatMessages.appendChild(fragment);
    }
    updateChatSessionRestoreProgress(snapshotTurns.length, snapshotTurns.length);

    snapshotTurns.forEach((item, index) => {
        const turnId = String(item?.turn_id || `snapshot-turn-${index + 1}`);
        const assistantId = buildServerAssistantMessageId(turnId, `snapshot-turn-${index + 1}`);
        const waitingForRemoteReply = passiveRunningSession && turnId === pendingTurnId;
        let assistantText = String(item?.assistant_content || '');
        let reasoningText = String(item?.reasoning?.content || '');
        let reasoningDuration = getSnapshotReasoningDuration(item);
        const snapshotToolState = item.tool || sessionUISnapshot.tool_cards?.[turnId] || null;

        // Fallback: If no reasoning_content is explicitly defined but <think> exists in the text
        if (!reasoningText && assistantText.includes('<think>')) {
            const parts = assistantText.split(/<think>([\s\S]*?)<\/think>/);
            if (parts.length >= 3) {
                reasoningText = parts[1].trim();
            } else {
                const openParts = assistantText.split('<think>');
                if (openParts.length > 1) reasoningText = openParts[openParts.length - 1].trim();
            }
        }

        if (!waitingForRemoteReply && !hasAssistantSnapshotContent(assistantText, reasoningText, snapshotToolState)) {
            return;
        }

        ensureAssistantMessageElement(assistantId, turnId);
        if (waitingForRemoteReply) {
            ensurePassiveSyncPlaceholder(turnId, sessionSnapshot?.ID || 'default', sessionUISnapshot.last_event_seq || 0);
        }
        if (reasoningText && !config.hideThink) {
            AppState.session.replay.reasoningBuffers.set(assistantId, reasoningText);
            const card = ensureReasoningCard(assistantId);
            const titleEl = card?.querySelector('.reasoning-title');
            const metaEl = card?.querySelector('.section-meta');
            const bodyEl = card?.querySelector('.reasoning-body');
            if (card) {
                card.classList.remove('failed');
                card.classList.add('completed');
                card.dataset.collapsed = 'true';
                card.dataset.userExpanded = 'false';
                card.classList.add('collapsed');
                card.dataset.durationMs = String(Math.max(0, reasoningDuration));
                card.dataset.accumulatedDurationMs = String(Math.max(0, reasoningDuration));
                if (titleEl) {
                    titleEl.classList.remove('is-live');
                    titleEl.textContent = formatThoughtDuration(Math.max(0, reasoningDuration));
                }
                if (metaEl) {
                    metaEl.textContent = t('status.done');
                    metaEl.classList.remove('is-live');
                }
                if (bodyEl) bodyEl.textContent = reasoningText;
            }
        }
        if (isMeaningfulToolState(snapshotToolState)) {
            ensureToolCard(assistantId, snapshotToolState.toolName || 'Tool');
            setToolCardState(assistantId, snapshotToolState.state, snapshotToolState.summary, snapshotToolState.args, snapshotToolState.toolName);
            const card = getActiveToolCard(assistantId);
            if (card) {
                card._history = Array.isArray(snapshotToolState.history) ? [...snapshotToolState.history] : [];
                const historyEl = card.querySelector('.tool-card-history');
                renderToolHistory(card, historyEl, snapshotToolState.state);
            }
        }
        AppState.session.replay.messageBuffers.set(assistantId, assistantText);
        updateSyncedMessageContent(assistantId, assistantText, { animate: false });
        finalizeMessageContent(assistantId, assistantText);
        finalizeAssistantStatusCards(assistantId, 'done');
        setAssistantActionBarReady(assistantId);
        AppState.chat.messages.push({ role: 'assistant', content: assistantText, turnId });
    });

    AppState.session.replay.currentTurnId = lastTurnId;
    AppState.session.replay.currentAssistantId = lastAssistantId;
    return true;
}

function isActiveLocalTurn(turnId = '') {
    return !!turnId && !!AppState.chat.activeLocalTurnId && turnId === AppState.chat.activeLocalTurnId;
}

function hasSubstantiveChatMessages() {
    if (!chatMessages) return false;
    return !!chatMessages.querySelector(
        '.message.user, .message.assistant:not(.has-startup-card), .message.system'
    );
}

function hasSubstantiveAssistantMessages() {
    if (!chatMessages) return false;
    return !!chatMessages.querySelector('.message.assistant:not(.has-startup-card)');
}

function hasRestorableChatEvents(items = []) {
    return Array.isArray(items) && items.some((entry) => {
        const eventType = String(entry?.EventType || '').trim();
        return eventType && eventType !== 'session.cleared';
    });
}

function buildServerAssistantMessageId(turnId = '', fallbackKey = '') {
    const key = String(turnId || fallbackKey || 'default').trim();
    return `server-assistant-${key}`;
}

function hasAssistantSnapshotContent(assistantText = '', reasoningText = '', toolState = null) {
    if (String(assistantText || '').trim()) return true;
    if (String(reasoningText || '').trim() && !config.hideThink) return true;
    return isMeaningfulToolState(toolState);
}

function findAssistantMessageByTurnId(turnId = '') {
    const resolvedTurnId = String(turnId || '').trim();
    if (!resolvedTurnId) return null;
    return document.querySelector(`.message.assistant[data-turn-id="${resolvedTurnId}"]`);
}

function cleanupAssistantMessagesForTurn(turnId = '', preferredId = '') {
    const resolvedTurnId = String(turnId || '').trim();
    if (!resolvedTurnId || !chatMessages) return;
    const nodes = Array.from(chatMessages.querySelectorAll(`.message.assistant[data-turn-id="${resolvedTurnId}"]`));
    if (nodes.length <= 1) return;

    let keepNode = preferredId ? document.getElementById(preferredId) : null;
    if (!keepNode || !nodes.includes(keepNode)) {
        keepNode = [...nodes].reverse().find((node) => !isAssistantMessageVisiblyEmpty(node)) || nodes[nodes.length - 1];
    }

    nodes.forEach((node) => {
        if (node === keepNode) return;
        if (isAssistantMessageVisiblyEmpty(node) || keepNode) {
            node.remove();
        }
    });
}

function ensureServerReplayAssistant(turnId, sessionId, seq) {
    const resolvedTurnId = String(turnId || '').trim();
    const fallbackKey = `server-turn-${sessionId || 'default'}-${seq || '0'}`;
    const messageId = buildServerAssistantMessageId(resolvedTurnId, fallbackKey);
    const existingByTurn = findAssistantMessageByTurnId(resolvedTurnId || fallbackKey);
    if (existingByTurn) {
        if (!existingByTurn.id) {
            existingByTurn.id = messageId;
        }
        AppState.session.replay.currentAssistantId = existingByTurn.id || messageId;
        cleanupAssistantMessagesForTurn(resolvedTurnId || fallbackKey, AppState.session.replay.currentAssistantId);
        return AppState.session.replay.currentAssistantId;
    }
    const stableMessageId = messageId;
    AppState.session.replay.currentAssistantId = stableMessageId;
    if (!document.getElementById(stableMessageId)) {
        appendMessage({
            role: 'assistant',
            content: '',
            id: stableMessageId,
            turnId: resolvedTurnId || fallbackKey
        });
    }
    cleanupAssistantMessagesForTurn(resolvedTurnId || fallbackKey, stableMessageId);
    return stableMessageId;
}

function applyCurrentChatSessionSnapshot(session) {
    if (session && AppState.session.currentCache && AppState.session.currentCache.ID !== session.ID) {
        resetServerChatReplayState();
    }
    if (!session) {
        AppState.session.currentCache = null;
        AppState.chat.passive.sessionLLMRunning = false;
        syncGlobalLLMComposerUI(null);
        if (!AppState.chat.abortController && AppState.chat.isGenerating) {
            AppState.chat.isGenerating = false;
            updateSendButtonState();
            hideProgressDock();
        }
        return;
    }

    const nextClearedAt = extractSessionClearedAt(session);
    if (nextClearedAt && nextClearedAt !== AppState.session.clearedAt) {
        resetChatViewState();
        AppState.session.eventSeq = 0;
        AppState.session.clearedAt = nextClearedAt;
    } else if (!nextClearedAt && AppState.session.clearedAt) {
        AppState.session.clearedAt = '';
    }

    AppState.session.currentCache = session;
    AppState.chat.passive.sessionLLMRunning = String(session?.Status || '').trim().toLowerCase() === 'running';
    syncSnapshotUserMessages(session);
    syncGlobalLLMComposerUI();

    const serverGenerating = session.Status === 'running';
    if (!AppState.chat.abortController && AppState.chat.isGenerating !== serverGenerating) {
        AppState.chat.isGenerating = serverGenerating;
        updateSendButtonState();
    }
    if (!serverGenerating) {
        clearComposerBackgroundTask('server-chat-detached');
        hideProgressDock();
    }

    if (config.llmMode === 'stateful') {
        AppState.chat.stateful.lastResponseId = session.LastResponseID || null;
        AppState.chat.stateful.summary = session.SummaryText || '';
        AppState.chat.stateful.turnCount = Number(session.TurnCount || 0);
        AppState.chat.stateful.estimatedChars = Number(session.EstimatedChars || 0);
        AppState.chat.stateful.lastInputTokens = Number(session.LastInputTokens || 0);
        AppState.chat.stateful.lastOutputTokens = Number(session.LastOutputTokens || 0);
        AppState.chat.stateful.peakInputTokens = Number(session.PeakInputTokens || 0);
        updateStatefulBudgetIndicator();
    }

    syncPassiveGenerationUI(session);
}

function applyCurrentChatSessionEvent(entry) {
    if (!entry?.EventType) return;

    let payload = {};
    try {
        payload = JSON.parse(entry.PayloadJSON || '{}');
    } catch (_) {
        payload = {};
    }

    const sessionId = entry.SessionID || 'default';
    const entryTurnId = entry.TurnID || payload.turn_id || '';
    const isLocalActiveTurn = isActiveLocalTurn(entryTurnId);
    const isPassiveRunning = !isLocalActiveTurn && isPassiveServerSession(AppState.session.currentCache);

    if (isLocalActiveTurn) {
        switch (entry.EventType) {
            case 'message.created':
            case 'prompt_processing.progress':
            case 'model_load.start':
            case 'model_load.progress':
            case 'model_load.end':
                return;
            default:
                break;
        }
    }

    if (!isLocalActiveTurn && !isPassiveRunning) {
        switch (entry.EventType) {
            case 'message.delta':
            case 'reasoning.start':
            case 'reasoning.delta':
            case 'reasoning.end':
            case 'tool_call.start':
            case 'tool_call.arguments':
            case 'tool_call.success':
            case 'tool_call.failure':
            case 'prompt_processing.progress':
            case 'model_load.start':
            case 'model_load.progress':
            case 'model_load.end':
                return;
            default:
                break;
        }
    }

    switch (entry.EventType) {
        case 'generation.started':
            if (isPassiveRunning) {
                syncPassiveGenerationUI(AppState.session.currentCache, payload.phase || 'queued');
            }
            break;
        case 'generation.first_token':
            if (isPassiveRunning) {
                syncPassiveGenerationUI(AppState.session.currentCache, payload.phase || 'answering');
            }
            break;
        case 'generation.phase':
            if (isPassiveRunning) {
                syncPassiveGenerationUI(AppState.session.currentCache, payload.phase || '');
            }
            break;
        case 'generation.finished':
            AppState.chat.passive.sessionLLMRunning = false;
            AppState.chat.passive.placeholder = '';
            inputContainer?.classList.remove('is-passive-generating');
            clearComposerBackgroundTask('passive-server-chat');
            syncGlobalLLMComposerUI();
            updateMessageInputPlaceholder();
            break;
        case 'message.created': {
            if (entry.Role === 'user') {
                const userContent = payload.content || '';
                if (!userContent) break;
                const turnId = entryTurnId || `server-turn-${sessionId}-${entry.EventSeq}`;
                if (isLocalActiveTurn) {
                    AppState.session.replay.currentTurnId = turnId;
                    AppState.session.replay.currentAssistantId = AppState.chat.activeLocalAssistantId || '';
                    break;
                }
                AppState.session.replay.currentTurnId = turnId;
                AppState.session.replay.currentAssistantId = '';
                if (!document.querySelector(`.message.user[data-turn-id="${turnId}"]`)) {
                    appendMessage({ role: 'user', content: userContent, turnId });
                }
            }
            break;
        }
        case 'message.delta': {
            let assistantId = '';
            if (isLocalActiveTurn) {
                AppState.session.replay.currentTurnId = entryTurnId || AppState.session.replay.currentTurnId;
                AppState.session.replay.currentAssistantId = AppState.chat.activeLocalAssistantId || '';
                assistantId = AppState.chat.activeLocalAssistantId || '';
            } else {
                if (!AppState.session.replay.currentTurnId) {
                    AppState.session.replay.currentTurnId = entryTurnId || `server-turn-${sessionId}-${entry.EventSeq}`;
                }
                assistantId = ensureServerReplayAssistant(AppState.session.replay.currentTurnId, sessionId, entry.EventSeq);
            }
            if (!assistantId) break;
            hideProgressDock();
            const next = typeof payload.full_content === 'string'
                ? payload.full_content
                : appendStreamChunkDedup(AppState.session.replay.messageBuffers.get(assistantId) || '', String(payload.content || ''));

            // Deep Sync Rationale: Do not buffer the "Generation in progress..." placeholder text
            // so that it doesn't get treated as final content if the server doesn't send a full_content payload later.
            if (next !== getPassiveSyncWaitingText()) {
                AppState.session.replay.messageBuffers.set(assistantId, next);
            }
            updateSyncedMessageContent(assistantId, next);

            // Handle text-based reasoning tags for passive sync (DeepSeek / LM Studio)
            if (config.llmMode === 'lm-studio' && !config.hideThink) {
                const hasAnalysis = next.includes('<|channel|>analysis');
                const hasFinal = next.includes('<|channel|>final');
                const hasThink = next.includes('<think>');
                const hasThinkEnd = next.includes('</think>');

                if ((hasAnalysis && !hasFinal) || (hasThink && !hasThinkEnd)) {
                    let statusText = "Thinking...";
                    if (hasAnalysis) {
                        const parts = next.split('<|channel|>analysis');
                        statusText = parts[parts.length - 1].split('<|channel|>')[0].trim();
                    } else if (hasThink) {
                        const parts = next.split('<think>');
                        statusText = parts[parts.length - 1].split('</think>')[0].trim();
                    }
                    if (statusText.length > 150) statusText = "..." + statusText.slice(-147);
                    showReasoningStatus(assistantId, statusText, false);
                } else if (hasFinal || hasThinkEnd) {
                    showReasoningStatus(assistantId, null, true);
                }
            }

            cleanupAssistantMessagesForTurn(AppState.session.replay.currentTurnId || entryTurnId, assistantId);
            break;
        }
        case 'reasoning.start':
            if (!isLocalActiveTurn && !AppState.session.replay.currentAssistantId && (entryTurnId || AppState.session.replay.currentTurnId)) {
                const nextTurnId = entryTurnId || AppState.session.replay.currentTurnId;
                AppState.session.replay.currentTurnId = nextTurnId;
                AppState.session.replay.currentAssistantId = ensureServerReplayAssistant(nextTurnId, sessionId, entry.EventSeq);
            }
            {
                const reasoningAssistantId = isLocalActiveTurn ? AppState.chat.activeLocalAssistantId : AppState.session.replay.currentAssistantId;
                if (reasoningAssistantId) {
                    if (!AppState.session.replay.reasoningBuffers.has(reasoningAssistantId)) {
                        AppState.session.replay.reasoningBuffers.set(reasoningAssistantId, '');
                    }
                }
            }
            if (payload.started_at) {
                setReasoningCardStartedAt(isLocalActiveTurn ? AppState.chat.activeLocalAssistantId : AppState.session.replay.currentAssistantId, payload.started_at);
            }
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) showReasoningStatus(AppState.chat.activeLocalAssistantId, '...');
                break;
            }
            if (AppState.session.replay.currentAssistantId) showReasoningStatus(AppState.session.replay.currentAssistantId, '...');
            break;
        case 'reasoning.delta':
            if (!isLocalActiveTurn && !AppState.session.replay.currentAssistantId && (entryTurnId || AppState.session.replay.currentTurnId)) {
                const nextTurnId = entryTurnId || AppState.session.replay.currentTurnId;
                AppState.session.replay.currentTurnId = nextTurnId;
                AppState.session.replay.currentAssistantId = ensureServerReplayAssistant(nextTurnId, sessionId, entry.EventSeq);
            }
            {
                const reasoningAssistantId = isLocalActiveTurn ? AppState.chat.activeLocalAssistantId : AppState.session.replay.currentAssistantId;
                const reasoningText = payload.content || payload.reasoning_content || payload.text || payload.delta?.content || '';
                const elapsedMs = Number.isFinite(Number(payload.total_elapsed_ms))
                    ? Number(payload.total_elapsed_ms)
                    : (Number.isFinite(Number(payload.elapsed_ms)) ? Number(payload.elapsed_ms) : null);
                if (reasoningAssistantId) {
                    const prevReasoning = AppState.session.replay.reasoningBuffers.get(reasoningAssistantId) || '';
                    const nextReasoning = appendStreamChunkDedup(prevReasoning, reasoningText);
                    AppState.session.replay.reasoningBuffers.set(reasoningAssistantId, nextReasoning);
                    if (isLocalActiveTurn) {
                        showReasoningStatus(reasoningAssistantId, nextReasoning || '...', false, elapsedMs);
                        break;
                    }
                    showReasoningStatus(reasoningAssistantId, nextReasoning || '...', false, elapsedMs);
                    break;
                }
                if (isLocalActiveTurn) {
                    if (AppState.chat.activeLocalAssistantId) showReasoningStatus(AppState.chat.activeLocalAssistantId, reasoningText || '...', false, elapsedMs);
                    break;
                }
                if (AppState.session.replay.currentAssistantId) showReasoningStatus(AppState.session.replay.currentAssistantId, reasoningText || '...', false, elapsedMs);
                break;
            }
        case 'reasoning.end':
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) {
                    const reasoningEndDuration = Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms))
                        ? Number(payload.total_elapsed_ms || payload.elapsed_ms) : null;
                    finalizeReasoningStatus(
                        AppState.chat.activeLocalAssistantId,
                        'done',
                        '',
                        reasoningEndDuration
                    );
                }
                break;
            }
            if (AppState.session.replay.currentAssistantId) {
                const duration = Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms))
                    ? Number(payload.total_elapsed_ms || payload.elapsed_ms)
                    : null;
                finalizeReasoningStatus(
                    AppState.session.replay.currentAssistantId,
                    'done',
                    '',
                    duration
                );
            }
            break;
        case 'tool_call.start':
            if (!isLocalActiveTurn && !AppState.session.replay.currentAssistantId && (entryTurnId || AppState.session.replay.currentTurnId)) {
                const nextTurnId = entryTurnId || AppState.session.replay.currentTurnId;
                AppState.session.replay.currentTurnId = nextTurnId;
                AppState.session.replay.currentAssistantId = ensureServerReplayAssistant(nextTurnId, sessionId, entry.EventSeq);
            }
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) setToolCardState(AppState.chat.activeLocalAssistantId, 'running', '', null, payload.tool || '');
                break;
            }
            if (AppState.session.replay.currentAssistantId) setToolCardState(AppState.session.replay.currentAssistantId, 'running', '', null, payload.tool || '');
            break;
        case 'tool_call.arguments':
            if (!isLocalActiveTurn && !AppState.session.replay.currentAssistantId && (entryTurnId || AppState.session.replay.currentTurnId)) {
                const nextTurnId = entryTurnId || AppState.session.replay.currentTurnId;
                AppState.session.replay.currentTurnId = nextTurnId;
                AppState.session.replay.currentAssistantId = ensureServerReplayAssistant(nextTurnId, sessionId, entry.EventSeq);
            }
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) setToolCardState(AppState.chat.activeLocalAssistantId, 'running', '', payload.arguments || null, payload.tool || '');
                break;
            }
            if (AppState.session.replay.currentAssistantId) setToolCardState(AppState.session.replay.currentAssistantId, 'running', '', payload.arguments || null, payload.tool || '');
            break;
        case 'tool_call.success':
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) setToolCardState(AppState.chat.activeLocalAssistantId, 'success', t('tool.executionFinished'), null, payload.tool || '');
                break;
            }
            if (AppState.session.replay.currentAssistantId) setToolCardState(AppState.session.replay.currentAssistantId, 'success', t('tool.executionFinished'), null, payload.tool || '');
            break;
        case 'tool_call.failure':
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) setToolCardState(AppState.chat.activeLocalAssistantId, 'failure', payload.reason || t('tool.unknownError'), null, payload.tool || '');
                break;
            }
            if (AppState.session.replay.currentAssistantId) setToolCardState(AppState.session.replay.currentAssistantId, 'failure', payload.reason || t('tool.unknownError'), null, payload.tool || '');
            break;
        case 'prompt_processing.progress':
            if (isLocalActiveTurn) {
                renderProgressDock(t('progress.processingPrompt'), (payload.progress || 0) * 100, 'prompt-processing', false);
                break;
            }
            renderProgressDock(t('progress.processingPrompt'), (payload.progress || 0) * 100, 'prompt-processing', false);
            break;
        case 'model_load.start':
            if (isLocalActiveTurn) {
                renderProgressDock(t('progress.loadingModel'), null, 'model-loading', true);
                break;
            }
            renderProgressDock(t('progress.loadingModel'), null, 'model-loading', true);
            break;
        case 'model_load.progress':
            if (isLocalActiveTurn) {
                renderProgressDock(t('progress.loadingModel'), (payload.progress || 0) * 100, 'model-loading', false);
                break;
            }
            renderProgressDock(t('progress.loadingModel'), (payload.progress || 0) * 100, 'model-loading', false);
            break;
        case 'model_load.end':
            if (isLocalActiveTurn) {
                renderProgressDock(`${t('progress.modelLoaded')} (${payload.load_time_seconds?.toFixed?.(1) || '?'}s)`, 100, 'model-loading', false);
                break;
            }
            renderProgressDock(`${t('progress.modelLoaded')} (${payload.load_time_seconds?.toFixed?.(1) || '?'}s)`, 100, 'model-loading', false);
            break;
        case 'chat.end':
        case 'request.complete':
            AppState.chat.passive.sessionLLMRunning = false;
            AppState.chat.passive.placeholder = '';
            inputContainer?.classList.remove('is-passive-generating');
            clearComposerBackgroundTask('passive-server-chat');
            syncGlobalLLMComposerUI();
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) {
                    const payloadToolState = extractToolStateFromPayload(payload);
                    if (isMeaningfulToolState(payloadToolState)) {
                        const card = ensureToolCard(AppState.chat.activeLocalAssistantId, payloadToolState.toolName || 'Tool');
                        if (card) {
                            card._history = Array.isArray(payloadToolState.history) ? [...payloadToolState.history] : [];
                        }
                        setToolCardState(AppState.chat.activeLocalAssistantId, payloadToolState.state || 'success', payloadToolState.summary || '', payloadToolState.args || null, payloadToolState.toolName || '');
                    }
                    const payloadReasoningText = extractReasoningContentFromPayload(payload);
                    if (payloadReasoningText && !AppState.session.replay.reasoningBuffers.get(AppState.chat.activeLocalAssistantId)) {
                        AppState.session.replay.reasoningBuffers.set(AppState.chat.activeLocalAssistantId, payloadReasoningText);
                        showReasoningStatus(AppState.chat.activeLocalAssistantId, payloadReasoningText, false, Number(payload.total_elapsed_ms || payload.elapsed_ms || 0) || null);
                    }
                }
                const duration = Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms))
                    ? Number(payload.total_elapsed_ms || payload.elapsed_ms)
                    : null;
                if (AppState.chat.activeLocalAssistantId && !AppState.session.replay.reasoningBuffers.has(AppState.chat.activeLocalAssistantId) && duration !== null) {
                    finalizeReasoningStatus(AppState.chat.activeLocalAssistantId, 'done', '', duration);
                } else if (AppState.chat.activeLocalAssistantId && AppState.session.replay.reasoningBuffers.has(AppState.chat.activeLocalAssistantId)) {
                    finalizeReasoningStatus(AppState.chat.activeLocalAssistantId, 'done', '', duration);
                }
                if (AppState.chat.activeLocalAssistantId) {
                    const payloadFinalText = extractFinalAssistantContentFromPayload(payload);
                    const finalText = payloadFinalText || (AppState.session.replay.messageBuffers.get(AppState.chat.activeLocalAssistantId) || '');
                    if (payloadFinalText) {
                        AppState.session.replay.messageBuffers.set(AppState.chat.activeLocalAssistantId, payloadFinalText);
                        updateSyncedMessageContent(AppState.chat.activeLocalAssistantId, payloadFinalText, { animate: false });
                    }
                    finalizeMessageContent(AppState.chat.activeLocalAssistantId, finalText);
                    finalizeAssistantStatusCards(AppState.chat.activeLocalAssistantId, 'done');
                    setAssistantActionBarReady(AppState.chat.activeLocalAssistantId);
                }
                AppState.chat.activeLocalTurnId = '';
                AppState.chat.activeLocalAssistantId = '';
                hideProgressDock();
                cleanupTrailingEmptyAssistantMessages();
                break;
            }
            const resolvedTurnId = entryTurnId || AppState.session.replay.currentTurnId || `server-turn-${sessionId}-${entry.EventSeq}`;
            const currentAssistantTurnId = String(
                document.getElementById(AppState.session.replay.currentAssistantId || '')?.dataset?.turnId || ''
            ).trim();
            AppState.session.replay.currentTurnId = resolvedTurnId;
            if (!AppState.session.replay.currentAssistantId || currentAssistantTurnId !== resolvedTurnId) {
                AppState.session.replay.currentAssistantId = ensureServerReplayAssistant(resolvedTurnId, sessionId, entry.EventSeq);
            }
            if (AppState.session.replay.currentAssistantId) {
                const payloadToolState = extractToolStateFromPayload(payload);
                if (isMeaningfulToolState(payloadToolState)) {
                    const card = ensureToolCard(AppState.session.replay.currentAssistantId, payloadToolState.toolName || 'Tool');
                    if (card) {
                        card._history = Array.isArray(payloadToolState.history) ? [...payloadToolState.history] : [];
                    }
                    setToolCardState(AppState.session.replay.currentAssistantId, payloadToolState.state || 'success', payloadToolState.summary || '', payloadToolState.args || null, payloadToolState.toolName || '');
                }
                const payloadReasoningText = extractReasoningContentFromPayload(payload);
                if (payloadReasoningText && !AppState.session.replay.reasoningBuffers.get(AppState.session.replay.currentAssistantId)) {
                    AppState.session.replay.reasoningBuffers.set(AppState.session.replay.currentAssistantId, payloadReasoningText);
                    showReasoningStatus(AppState.session.replay.currentAssistantId, payloadReasoningText, false, Number(payload.total_elapsed_ms || payload.elapsed_ms || 0) || null);
                }
                if (!AppState.session.replay.reasoningBuffers.has(AppState.session.replay.currentAssistantId) && Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms))) {
                    finalizeReasoningStatus(AppState.session.replay.currentAssistantId, 'done', '', Number(payload.total_elapsed_ms || payload.elapsed_ms));
                } else if (AppState.session.replay.reasoningBuffers.has(AppState.session.replay.currentAssistantId)) {
                    finalizeReasoningStatus(AppState.session.replay.currentAssistantId, 'done', '', Number(payload.total_elapsed_ms || payload.elapsed_ms || 0) || null);
                }
                const payloadFinalText = extractFinalAssistantContentFromPayload(payload);
                const finalText = payloadFinalText || (AppState.session.replay.messageBuffers.get(AppState.session.replay.currentAssistantId) || '');
                if (payloadFinalText) {
                    AppState.session.replay.messageBuffers.set(AppState.session.replay.currentAssistantId, payloadFinalText);
                    updateSyncedMessageContent(AppState.session.replay.currentAssistantId, payloadFinalText, { animate: false });
                }
                finalizeMessageContent(AppState.session.replay.currentAssistantId, finalText);
                finalizeAssistantStatusCards(AppState.session.replay.currentAssistantId, 'done');
                setAssistantActionBarReady(AppState.session.replay.currentAssistantId);
                cleanupAssistantMessagesForTurn(AppState.session.replay.currentTurnId || entryTurnId, AppState.session.replay.currentAssistantId);
                if (getStreamingScrollMode() !== 'label-top') {
                    holdAutoScrollAtBottom(900);
                    scrollToBottom(true);
                } else {
                    updateScrollToBottomButton();
                }
            }
            hideProgressDock();
            cleanupTrailingEmptyAssistantMessages();
            break;
        case 'request.cancelled':
            AppState.chat.passive.sessionLLMRunning = false;
            if (isLocalActiveTurn) {
                if (AppState.chat.activeLocalAssistantId) {
                    finalizeAssistantStatusCards(AppState.chat.activeLocalAssistantId, 'stopped', t('status.stopped'));
                }
                AppState.chat.activeLocalTurnId = '';
                AppState.chat.activeLocalAssistantId = '';
                cleanupTrailingEmptyAssistantMessages();
                break;
            }
            if (AppState.session.replay.currentAssistantId) {
                finalizeAssistantStatusCards(AppState.session.replay.currentAssistantId, 'stopped', t('status.stopped'));
            }
            hideProgressDock();
            syncGlobalLLMComposerUI();
            cleanupTrailingEmptyAssistantMessages();
            break;
        case 'session.cleared':
            resetChatViewState();
            AppState.chat.stateful.pendingResetReason = 'manual_clear_chat';
            AppState.session.clearedAt = payload.cleared_at || AppState.session.clearedAt;
            AppState.session.eventSeq = 0;
            scheduleChatSessionPolling(1200);
            break;
    }
}

async function syncCurrentChatSessionFromServer() {
    if (AppState.session.syncPromise) {
        AppState.session.pendingSync = true;
        return AppState.session.syncPromise;
    }

    AppState.session.syncPromise = (async () => {
        do {
            AppState.session.pendingSync = false;
            await syncCurrentChatSessionFromServerInternal();
        } while (AppState.session.pendingSync);
    })().finally(() => {
        AppState.session.syncPromise = null;
    });

    return AppState.session.syncPromise;
}

async function syncCurrentChatSessionFromServerInternal() {
    if (!AppState.session.currentUser) return;

    const session = await fetchCurrentChatSession();
    applyCurrentChatSessionSnapshot(session);
    const hasLocalInFlightResponse = !!AppState.chat.activeLocalTurnId || !!AppState.chat.activeLocalAssistantId || !!AppState.chat.isGenerating;
    const hasServerOwnedSession = !!session && String(session?.Status || '').trim().length > 0;
    if (hasLocalInFlightResponse && hasServerOwnedSession && !AppState.session.retryRecoveryInProgress) {
        relinquishLocalStreamOwnership(`session-sync:${session?.Status || 'none'}`);
    }

    if (!session) {
        scheduleChatSessionPolling(1800);
        return;
    }

    const hasRenderedMessages = hasSubstantiveAssistantMessages();
    if (AppState.session.eventSeq === 0 && !hasRenderedMessages) {
        const sessionUISnapshot = getCurrentChatSessionUISnapshot(session);
        const snapshotMessages = sessionUISnapshot.messages || [];
        if (snapshotMessages.length > 0) {
            const snapshotLastEventSeq = Number(sessionUISnapshot.last_event_seq || 0);
            if (snapshotLastEventSeq <= 0) {
                beginChatSessionRestore(snapshotMessages.length);
                const seedResult = await fetchCurrentChatSessionEvents(0, 200);
                const seedItems = Array.isArray(seedResult.items) ? [...seedResult.items] : [];
                if (seedItems.length > 0) {
                    try {
                        updateChatSessionRestoreProgress(seedItems.length, seedResult.totalCount || seedItems.length);
                        let allItems = seedItems;
                        let afterSeq = Number(allItems[allItems.length - 1]?.EventSeq || 0);
                        while (allItems.length < Number(seedResult.totalCount || 0)) {
                            const page = await fetchCurrentChatSessionEvents(afterSeq, 200);
                            const pageItems = Array.isArray(page.items) ? page.items : [];
                            if (pageItems.length === 0) break;
                            allItems = allItems.concat(pageItems);
                            afterSeq = Number(pageItems[pageItems.length - 1]?.EventSeq || afterSeq);
                            updateChatSessionRestoreProgress(allItems.length, seedResult.totalCount || allItems.length);
                        }
                        hydrateChatSessionEventsSnapshot(allItems, seedResult.session || session);
                        for (const entry of allItems) {
                            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
                        }
                    } finally {
                        finishChatSessionRestore();
                    }
                    scheduleChatSessionPolling(session.Status === 'running' ? 900 : 1600);
                    return;
                }
            }
            if (!AppState.session.isRestoring) {
                beginChatSessionRestore(snapshotMessages.length);
            }
            try {
                updateChatSessionRestoreProgress(0, snapshotMessages.length);
                let trailingItems = [];
                let afterSeq = snapshotLastEventSeq;
                while (true) {
                    const page = await fetchCurrentChatSessionEvents(afterSeq, 200);
                    if (page.session) {
                        applyCurrentChatSessionSnapshot(page.session);
                    }
                    const pageItems = Array.isArray(page.items) ? page.items : [];
                    if (pageItems.length === 0) break;
                    trailingItems = trailingItems.concat(pageItems);
                    afterSeq = Number(pageItems[pageItems.length - 1].EventSeq || afterSeq);
                    if (pageItems.length < 200) break;
                }
                hydrateChatSessionUISnapshot(session);
                if (trailingItems.length > 0) {
                    dismissStartupCards();
                    for (const entry of trailingItems) {
                        applyCurrentChatSessionEvent(entry);
                        AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
                    }
                }
                AppState.session.eventSeq = Math.max(AppState.session.eventSeq, snapshotLastEventSeq);
                updateChatSessionRestoreProgress(snapshotMessages.length + trailingItems.length, snapshotMessages.length + trailingItems.length);
            } finally {
                finishChatSessionRestore();
            }
            scheduleChatSessionPolling(session.Status === 'running' ? 900 : 1600);
            return;
        }
    }

    const result = await fetchCurrentChatSessionEvents(AppState.session.eventSeq, 200);
    if (result.session) {
        applyCurrentChatSessionSnapshot(result.session);
    }

    const renderedMessagesNow = hasSubstantiveAssistantMessages();
    const hasRestorableEvents = hasRestorableChatEvents(result.items);
    const shouldRestoreSnapshot = AppState.session.eventSeq === 0 && result.totalCount > 0 && hasRestorableEvents && !renderedMessagesNow;
    const shouldFastForwardSeqOnly = AppState.session.eventSeq === 0 && result.totalCount > 0 && (!hasRestorableEvents || renderedMessagesNow);
    if (shouldRestoreSnapshot) {
        beginChatSessionRestore(result.totalCount);
        try {
            let allItems = Array.isArray(result.items) ? [...result.items] : [];
            updateChatSessionRestoreProgress(allItems.length, result.totalCount);
            let afterSeq = allItems.length > 0 ? Number(allItems[allItems.length - 1].EventSeq || 0) : 0;

            while (allItems.length < result.totalCount) {
                const page = await fetchCurrentChatSessionEvents(afterSeq, 200);
                const pageItems = Array.isArray(page.items) ? page.items : [];
                if (pageItems.length === 0) break;
                allItems = allItems.concat(pageItems);
                afterSeq = Number(pageItems[pageItems.length - 1].EventSeq || afterSeq);
                updateChatSessionRestoreProgress(allItems.length, result.totalCount);
            }

            hydrateChatSessionEventsSnapshot(allItems, result.session || session);
            for (const entry of allItems) {
                AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
            }
        } finally {
            finishChatSessionRestore();
        }
    } else if (shouldFastForwardSeqOnly) {
        for (const entry of result.items) {
            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
        }
    } else if (Array.isArray(result.items) && result.items.length > 0) {
        dismissStartupCards();
        for (const entry of result.items) {
            applyCurrentChatSessionEvent(entry);
            AppState.session.eventSeq = Math.max(AppState.session.eventSeq, Number(entry.EventSeq || 0));
        }
    }

    if (session.Status === 'running') {
        scheduleChatSessionPolling(900);
    } else {
        scheduleChatSessionPolling(1600);
    }
}

function hydrateChatSessionEventsSnapshot(items, sessionSnapshot = null) {
    if (!Array.isArray(items) || items.length === 0) return;

    const sessionUISnapshot = getCurrentChatSessionUISnapshot(sessionSnapshot);
    const passiveRunningSession = isPassiveServerSession(sessionSnapshot);
    chatMessages?.classList.add('is-session-hydrating');
    resetChatViewState();
    dismissStartupCards();
    AppState.chat.messages = [];

    const users = [];
    const assistantByTurn = new Map();
    const assistantTextById = new Map();
    const reasoningTextById = new Map();
    const reasoningStartedAtById = new Map();
    const reasoningDurationById = new Map();
    const toolStateById = new Map();
    const assistantOrder = [];

    let currentTurnId = '';
    let currentAssistantId = '';
    let currentSessionId = 'default';

    const ensureAssistantId = (turnId, eventSeq) => {
        const key = turnId || `server-turn-default-${eventSeq}`;
        if (!assistantByTurn.has(key)) {
            const assistantId = buildServerAssistantMessageId(key, `server-turn-${currentSessionId}-${eventSeq}`);
            assistantByTurn.set(key, assistantId);
            assistantOrder.push({ turnId: key, assistantId });
        }
        return assistantByTurn.get(key);
    };

    for (const entry of items) {
        let payload = {};
        try {
            payload = JSON.parse(entry?.PayloadJSON || '{}');
        } catch (_) {
            payload = {};
        }

        currentSessionId = entry?.SessionID || currentSessionId;
        const entryTurnId = entry?.TurnID || payload.turn_id || '';

        switch (entry?.EventType) {
            case 'message.created': {
                if (entry.Role !== 'user') break;
                const userContent = String(payload.content || '');
                if (!userContent) break;
                currentTurnId = entryTurnId || `server-turn-${currentSessionId}-${entry.EventSeq}`;
                currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                users.push({ turnId: currentTurnId, content: userContent });
                break;
            }
            case 'message.delta': {
                if (!currentTurnId) {
                    currentTurnId = entryTurnId || `server-turn-${currentSessionId}-${entry.EventSeq}`;
                }
                currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                const next = typeof payload.full_content === 'string'
                    ? payload.full_content
                    : appendStreamChunkDedup(assistantTextById.get(currentAssistantId) || '', String(payload.content || ''));
                assistantTextById.set(currentAssistantId, next);
                break;
            }
            case 'reasoning.start': {
                if (!currentAssistantId && currentTurnId) {
                    currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                }
                if (currentAssistantId) {
                    if (!reasoningTextById.has(currentAssistantId)) {
                        reasoningTextById.set(currentAssistantId, '');
                    }
                    if (payload.started_at) {
                        reasoningStartedAtById.set(currentAssistantId, payload.started_at);
                    }
                }
                break;
            }
            case 'reasoning.delta': {
                if (!currentAssistantId && currentTurnId) {
                    currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                }
                if (currentAssistantId) {
                    const prev = reasoningTextById.get(currentAssistantId) || '';
                    const delta = String(payload.content || payload.reasoning_content || payload.text || payload.delta?.content || '');
                    const next = appendStreamChunkDedup(prev, delta);
                    reasoningTextById.set(currentAssistantId, next);
                }
                break;
            }
            case 'reasoning.end': {
                if (!currentAssistantId && currentTurnId) {
                    currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                }
                if (currentAssistantId && Number.isFinite(Number(payload.total_elapsed_ms || payload.elapsed_ms))) {
                    reasoningDurationById.set(currentAssistantId, Number(payload.total_elapsed_ms || payload.elapsed_ms));
                }
                break;
            }
            case 'tool_call.start':
            case 'tool_call.arguments':
            case 'tool_call.success':
            case 'tool_call.failure': {
                if (!currentAssistantId && currentTurnId) {
                    currentAssistantId = ensureAssistantId(currentTurnId, entry.EventSeq);
                }
                if (!currentAssistantId) break;
                const nextState = {
                    state: entry.EventType === 'tool_call.failure'
                        ? 'failure'
                        : entry.EventType === 'tool_call.success'
                            ? 'success'
                            : 'running',
                    summary: entry.EventType === 'tool_call.failure'
                        ? (payload.reason || t('tool.unknownError'))
                        : entry.EventType === 'tool_call.success'
                            ? t('tool.executionFinished')
                            : '',
                    args: entry.EventType === 'tool_call.arguments' ? (payload.arguments || null) : null,
                    toolName: payload.tool || ''
                };
                const prev = toolStateById.get(currentAssistantId) || { history: [] };
                const previewText = extractToolPreview(nextState.args, nextState.summary, nextState.toolName);
                const displayTool = formatToolDisplayName(nextState.toolName || prev.toolName || 'Tool');
                const nextHistory = Array.isArray(prev.history) ? [...prev.history] : [];
                if (previewText) {
                    const signature = `${displayTool}::${previewText}`;
                    const last = nextHistory[nextHistory.length - 1];
                    if (!last || last.signature !== signature) {
                        nextHistory.push({
                            signature,
                            tool: displayTool,
                            detail: previewText
                        });
                    }
                }
                toolStateById.set(currentAssistantId, {
                    state: nextState.state || prev.state || 'running',
                    summary: nextState.summary || prev.summary || '',
                    args: nextState.args != null ? nextState.args : (prev.args || null),
                    toolName: nextState.toolName || prev.toolName || '',
                    history: nextHistory
                });
                break;
            }
            default:
                break;
        }
    }

    const fragment = document.createDocumentFragment();
    const pendingTurnId = passiveRunningSession && users.length > 0
        ? String(users[users.length - 1]?.turnId || '').trim()
        : '';
    for (const user of users) {
        if (!document.querySelector(`.message.user[data-turn-id="${user.turnId}"]`)) {
            appendMessage({ role: 'user', content: user.content, turnId: user.turnId }, { parent: fragment, skipScroll: true });
        }
        AppState.chat.messages.push({ role: 'user', content: user.content, turnId: user.turnId });
        const assistantId = assistantByTurn.get(user.turnId);
        if (!assistantId) continue;
        const waitingForRemoteReply = passiveRunningSession && user.turnId === pendingTurnId;
        const assistantText = assistantTextById.get(assistantId) || '';
        const reasoningText = reasoningTextById.get(assistantId) || '';
        const toolState = toolStateById.get(assistantId) || sessionUISnapshot.tool_cards?.[user.turnId] || null;
        if (!waitingForRemoteReply && !hasAssistantSnapshotContent(assistantText, reasoningText, toolState)) continue;
        if (!document.getElementById(assistantId)) {
            appendMessage({
                role: 'assistant',
                content: '',
                id: assistantId,
                turnId: user.turnId
            }, { parent: fragment, skipScroll: true });
        }
    }

    if (fragment.childNodes.length > 0) {
        chatMessages.appendChild(fragment);
    }

    for (const user of users) {
        const assistantId = assistantByTurn.get(user.turnId);
        if (!assistantId) continue;
        const waitingForRemoteReply = passiveRunningSession && user.turnId === pendingTurnId;
        const assistantText = assistantTextById.get(assistantId) || '';
        const reasoningText = reasoningTextById.get(assistantId) || '';
        const toolState = toolStateById.get(assistantId);
        const snapshotToolState = sessionUISnapshot.tool_cards?.[user.turnId] || null;
        const mergedToolState = toolState || snapshotToolState;
        if (waitingForRemoteReply) {
            ensurePassiveSyncPlaceholder(user.turnId, sessionSnapshot?.ID || 'default', AppState.session.eventSeq || 0);
            continue;
        }
        if (!hasAssistantSnapshotContent(assistantText, reasoningText, mergedToolState)) continue;
        const assistantEl = ensureAssistantMessageElement(assistantId);
        if (!assistantEl) continue;

        if (reasoningStartedAtById.has(assistantId)) {
            setReasoningCardStartedAt(assistantId, reasoningStartedAtById.get(assistantId));
        }
        if (reasoningText && !config.hideThink) {
            AppState.session.replay.reasoningBuffers.set(assistantId, reasoningText);
            showReasoningStatus(assistantId, reasoningText);
            const duration = reasoningDurationById.get(assistantId);
            finalizeReasoningStatus(assistantId, 'done', '', Number.isFinite(duration) ? duration : null);
        }

        if (isMeaningfulToolState(mergedToolState)) {
            ensureToolCard(assistantId, mergedToolState.toolName || 'Tool');
            setToolCardState(assistantId, mergedToolState.state, mergedToolState.summary, mergedToolState.args, mergedToolState.toolName);
            const card = getActiveToolCard(assistantId);
            if (card) {
                card._history = Array.isArray(mergedToolState.history) ? [...mergedToolState.history] : [];
                const historyEl = card.querySelector('.tool-card-history');
                renderToolHistory(card, historyEl, mergedToolState.state);
            }
        }

        AppState.session.replay.messageBuffers.set(assistantId, assistantText);
        updateSyncedMessageContent(assistantId, assistantText, { animate: false });
        finalizeMessageContent(assistantId, assistantText);
        finalizeAssistantStatusCards(assistantId, 'done');
        setAssistantActionBarReady(assistantId);
        AppState.chat.messages.push({ role: 'assistant', content: assistantText, turnId: user.turnId });
    }

    if (users.length > 0) {
        AppState.session.replay.currentTurnId = users[users.length - 1].turnId;
        AppState.session.replay.currentAssistantId = assistantByTurn.get(AppState.session.replay.currentTurnId) || '';
    }
    scrollToBottom(true);
    ensureChatRestoredToLatest();
    requestAnimationFrame(() => {
        reconcileVisibleAssistantActionBars();
        chatMessages?.classList.remove('is-session-hydrating');
    });
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

async function restoreLastSession() {
    await syncCurrentChatSessionFromServer();
    if (AppState.session.eventSeq > 0) {
        showToast(t('chat.startup.restoreLoaded'));
        return;
    }

    await ensureLastSessionCacheLoaded();
    if (!AppState.session.lastCache) {
        showToast(t('chat.startup.restoreMissing'), true);
        return;
    }

    await clearChat();
    const turnId = generateTurnId();

    const restoredUser = {
        role: 'user',
        content: AppState.session.lastCache.user_message || '',
        turnId
    };
    const restoredAssistant = {
        role: 'assistant',
        content: AppState.session.lastCache.assistant_message || '',
        turnId
    };

    appendMessage(restoredUser);
    appendMessage(restoredAssistant);
    AppState.chat.messages.push(restoredUser, restoredAssistant);
    holdAutoScrollAtBottom(1200);
    ensureChatRestoredToLatest();

    if (config.llmMode === 'stateful') {
        AppState.chat.stateful.summary = buildRestoredStatefulSummary(AppState.session.lastCache);
        AppState.chat.stateful.estimatedChars = AppState.chat.stateful.summary.length;
        AppState.chat.stateful.turnCount = 1;
        AppState.chat.stateful.lastInputTokens = estimateTokensFromText(AppState.chat.stateful.summary);
        AppState.chat.stateful.lastOutputTokens = estimateTokensFromText(restoredAssistant.content);
        AppState.chat.stateful.peakInputTokens = Math.max(AppState.chat.stateful.peakInputTokens, AppState.chat.stateful.lastInputTokens);
        updateStatefulBudgetIndicator();
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
    persistAppConfigSnapshot();
}

async function syncServerConfig(options = {}) {
    const forceApply = options.forceApply === true;
    const forceDictionaryReload = options.forceDictionaryReload === true;
    const log = options.log === true;
    try {
        const response = await fetch('/api/config', { credentials: 'include' }); // Fetch current server config
        if (response.ok) {
            const serverCfg = await response.json();
            const nextSignature = buildServerConfigSignature(serverCfg);
            const configChanged = forceApply || nextSignature !== AppState.session.lastSyncedConfigSignature;
            if (!configChanged) {
                return false;
            }
            AppState.session.lastSyncedConfigSignature = nextSignature;
            if (log) {
                console.log('[Config] Synced from server:', serverCfg);
            }

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
                }
            }
            config.contextStrategy = normalizeContextStrategyForMode(config.llmMode, serverCfg.context_strategy || config.contextStrategy);
            renderContextStrategyOptions();
            const contextStrategyEl = document.getElementById('cfg-context-strategy');
            if (contextStrategyEl) {
                contextStrategyEl.value = config.contextStrategy;
            }
            updateSettingsVisibility();
            if (serverCfg.secondary_model !== undefined) {
                config.secondaryModel = String(serverCfg.secondary_model || '').trim();
                const el = document.getElementById('cfg-secondary-model');
                if (el) el.value = config.secondaryModel;
            }
            if (serverCfg.enable_tts !== undefined) {
                config.enableTTS = serverCfg.enable_tts;
                document.getElementById('cfg-enable-tts').checked = config.enableTTS;
            }
            if (serverCfg.tts_config) {
                const ttsCfg = serverCfg.tts_config;
                config.ttsEngine = ttsCfg.engine || config.ttsEngine || 'supertonic';
                config.ttsVoice = ttsCfg.voiceStyle || config.ttsVoice;
                config.ttsSpeed = Number(ttsCfg.speed) > 0 ? Number(ttsCfg.speed) : config.ttsSpeed;
                config.ttsThreads = Number(ttsCfg.threads) > 0 ? Number(ttsCfg.threads) : config.ttsThreads;
                config.osTtsVoiceURI = ttsCfg.osVoiceURI || config.osTtsVoiceURI || '';
                config.osTtsVoiceName = ttsCfg.osVoiceName || config.osTtsVoiceName || '';
                config.osTtsVoiceLang = ttsCfg.osVoiceLang || config.osTtsVoiceLang || '';
                config.osTtsRate = Number(ttsCfg.osRate) > 0 ? Number(ttsCfg.osRate) : (config.osTtsRate || 1.0);
                config.osTtsPitch = Number(ttsCfg.osPitch) > 0 ? Number(ttsCfg.osPitch) : (config.osTtsPitch || 1.0);

                const engineEl = document.getElementById('cfg-tts-engine');
                if (engineEl) engineEl.value = config.ttsEngine;
                const voiceEl = document.getElementById('cfg-tts-voice');
                if (voiceEl && config.ttsVoice) voiceEl.value = config.ttsVoice.replace('.json', '');
                const speedEl = document.getElementById('cfg-tts-speed');
                if (speedEl) speedEl.value = config.ttsSpeed || 1.0;
                const speedValEl = document.getElementById('speed-val');
                if (speedValEl) speedValEl.textContent = String(config.ttsSpeed || 1.0);
                const threadsEl = document.getElementById('cfg-tts-threads');
                if (threadsEl) threadsEl.value = config.ttsThreads || 4;
                const threadsValEl = document.getElementById('threads-val');
                if (threadsValEl) threadsValEl.textContent = String(config.ttsThreads || 4);
                const osRateEl = document.getElementById('cfg-os-tts-rate');
                if (osRateEl) osRateEl.value = config.osTtsRate || 1.0;
                const osRateValEl = document.getElementById('os-rate-val');
                if (osRateValEl) osRateValEl.textContent = String(config.osTtsRate || 1.0);
                const osPitchEl = document.getElementById('cfg-os-tts-pitch');
                if (osPitchEl) osPitchEl.value = config.osTtsPitch || 1.0;
                const osPitchValEl = document.getElementById('os-pitch-val');
                if (osPitchValEl) osPitchValEl.textContent = String(config.osTtsPitch || 1.0);
                populateOSTTSVoiceList();
                if (osTTSVoiceSelect && config.osTtsVoiceURI && ttsController.isVoicesReady()) {
                    osTTSVoiceSelect.value = config.osTtsVoiceURI;
                }
                updateTTSSettingsVisibility();
            }
            if (serverCfg.embedding_config) {
                const embeddingCfg = serverCfg.embedding_config;
                config.embeddingProvider = embeddingCfg.provider || 'local';
                config.embeddingModelId = embeddingCfg.modelId || 'multilingual-e5-small';
                config.enableEmbeddings = embeddingCfg.enabled === true;
                const enabledEl = document.getElementById('cfg-enable-embeddings');
                if (enabledEl) enabledEl.checked = config.enableEmbeddings;
                const modelEl = document.getElementById('cfg-embedding-model');
                if (modelEl) modelEl.value = config.embeddingModelId;
            }
            if (serverCfg.enable_mcp !== undefined) {
                config.enableMCP = serverCfg.llm_mode === 'stateful' ? serverCfg.enable_mcp : false;
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
            updateHeaderModelDisplay();
            renderModelPickerModal();
            if (serverCfg.stateful_turn_limit !== undefined) {
                config.statefulTurnLimit = Math.max(1, Number(serverCfg.stateful_turn_limit) || DEFAULT_STATEFUL_TURN_LIMIT);
                const el = document.getElementById('cfg-stateful-turn-limit');
                if (el) el.value = String(config.statefulTurnLimit);
            }
            if (serverCfg.stateful_char_budget !== undefined) {
                config.statefulCharBudget = Math.max(1000, Number(serverCfg.stateful_char_budget) || DEFAULT_STATEFUL_CHAR_BUDGET);
                const el = document.getElementById('cfg-stateful-char-budget');
                if (el) el.value = String(config.statefulCharBudget);
            }
            if (serverCfg.stateful_token_budget !== undefined) {
                config.statefulTokenBudget = Math.max(1000, Number(serverCfg.stateful_token_budget) || DEFAULT_STATEFUL_TOKEN_BUDGET);
                const el = document.getElementById('cfg-stateful-token-budget');
                if (el) el.value = String(config.statefulTokenBudget);
            }

            // Save to localStorage so next reload uses these
            persistAppConfigSnapshot();
            await loadTTSDictionary(getEffectiveTTSDictionaryLang(), {
                forceReload: forceDictionaryReload,
                log
            });
            return true;
        }
    } catch (e) {
        console.warn('Failed to sync server config:', e);
    }
    return false;
}

async function loadVoiceStyles() {
    return ttsController.loadVoiceStyles();
}

function supportsOSTTS() {
    return ttsController.supportsOSTTS();
}

function getCurrentTTSEngine() {
    return ttsController.getCurrentTTSEngine();
}

function mapVoiceLangToDictionaryLang(lang = '') {
    const normalized = String(lang || '').toLowerCase();
    if (normalized.startsWith('ko')) return 'ko';
    if (normalized.startsWith('en')) return 'en';
    if (normalized.startsWith('es')) return 'es';
    if (normalized.startsWith('fr')) return 'fr';
    if (normalized.startsWith('pt')) return 'pt';
    return config.ttsLang || 'ko';
}

function getEffectiveTTSDictionaryLang() {
    if (getCurrentTTSEngine() === 'os' && config.osTtsVoiceLang) {
        return mapVoiceLangToDictionaryLang(config.osTtsVoiceLang);
    }
    return config.ttsLang || 'ko';
}

function getSelectedOSTTSVoice() {
    return ttsController.getSelectedOSTTSVoice();
}

function syncOSTTSVoiceConfigFromSelection() {
    return ttsController.syncOSTTSVoiceConfigFromSelection();
}

function populateOSTTSVoiceList() {
    return ttsController.populateOSTTSVoiceList();
}

function initOSTTSVoiceLoading() {
    return ttsController.initOSTTSVoiceLoading();
}

function updateTTSSettingsVisibility() {
    return ttsController.updateTTSSettingsVisibility();
}


function toggleSwitch(id) {
    const el = document.getElementById(id);
    if (el) {
        el.checked = !el.checked;
        saveConfig(false);
    }
}

function insertPlainTextAtCursor(text) {
    if (!messageInput) return;
    const normalized = String(text || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const start = messageInput.selectionStart ?? messageInput.value.length;
    const end = messageInput.selectionEnd ?? messageInput.value.length;
    messageInput.setRangeText(normalized, start, end, 'end');
    messageInput.dispatchEvent(new Event('input', { bubbles: true }));
}

function setupEventListeners() {
    document.getElementById('save-cfg-btn').addEventListener('click', saveConfig);

    if (inputContainer) {
        inputContainer.addEventListener('pointerdown', (e) => {
            if (e.pointerType === 'touch') return;
            if (e.target.closest('.input-actions')) return;
            if (document.activeElement === messageInput) return;
            focusMessageInput({ preserveChatScroll: true });
        });

        inputContainer.addEventListener('click', (e) => {
            if (e.target === messageInput) return;
            if (e.target.closest('.input-actions')) return;
            if (document.activeElement === messageInput) return;

            focusMessageInput({ preserveChatScroll: true });
        });

        inputContainer.addEventListener('touchend', (e) => {
            if (e.target instanceof Element && e.target.closest('.input-actions')) return;
            if (e.target === messageInput) return;
            maintainInputFocusAfterTouch();
        }, { passive: true });
    }

    messageInput.addEventListener('touchstart', () => {
        savedLibraryController.resetSwipeState();
        AppState.ui.scroll.pendingInputFocusTop = chatMessages?.scrollTop ?? null;
        AppState.ui.scroll.pendingFirstInputRepairTop = AppState.ui.scroll.pendingInputFocusTop;
    }, { passive: true });

    messageInput.addEventListener('pointerdown', () => {
        AppState.ui.scroll.pendingInputFocusTop = chatMessages?.scrollTop ?? null;
        AppState.ui.scroll.pendingFirstInputRepairTop = AppState.ui.scroll.pendingInputFocusTop;
    }, { passive: true });

    messageInput.addEventListener('focus', () => {
        document.body.classList.add('keyboard-open');
        if (AppState.ui.scroll.pendingInputFocusTop != null) {
            scheduleInputScrollRepair(AppState.ui.scroll.pendingInputFocusTop);
            AppState.ui.scroll.pendingInputFocusTop = null;
        }
        updateViewportMetrics();
    });

    messageInput.addEventListener('blur', () => {
        AppState.ui.scroll.pendingInputFocusTop = null;
        AppState.ui.scroll.pendingFirstInputRepairTop = null;
        window.setTimeout(() => {
            updateViewportMetrics();
        }, 120);
    });

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
        if (AppState.ui.scroll.pendingFirstInputRepairTop != null) {
            scheduleInputScrollRepair(AppState.ui.scroll.pendingFirstInputRepairTop, [0, 90, 220]);
            AppState.ui.scroll.pendingFirstInputRepairTop = null;
        }
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
                    AppState.input.pendingImage = event.target.result; // Base64 string
                    imagePreviewVal.src = AppState.input.pendingImage;
                    previewContainer.style.display = 'block';
                    updateInlineComposerActionVisibility();
                };
                reader.readAsDataURL(blob);
            }
        }
        // If an image was found, prevent default to avoid pasting source URLs or other metadata
        if (hasImage) {
            e.preventDefault();
            return;
        }

        const plainText = (e.clipboardData || e.originalEvent.clipboardData).getData('text/plain');
        if (typeof plainText === 'string') {
            e.preventDefault();
            insertPlainTextAtCursor(plainText);
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
        if (isComposerStopActionActive()) {
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
    updateComposerLayoutMetrics();
}

function handleImageUpload(input) {
    if (input.files && input.files[0]) {
        const reader = new FileReader();
        reader.onload = function (e) {
            AppState.input.pendingImage = e.target.result; // Base64 string
            imagePreviewVal.src = AppState.input.pendingImage;
            previewContainer.style.display = 'block';
            updateInlineComposerActionVisibility();
        };
        reader.readAsDataURL(input.files[0]);
    }
}

function triggerImagePicker() {
    triggerHaptic('success');
    document.getElementById('image-upload')?.click();
}

function removeImage() {
    AppState.input.pendingImage = null;
    document.getElementById('image-upload').value = '';
    previewContainer.style.display = 'none';
    updateInlineComposerActionVisibility();
}

function resetChatViewState() {
    stopAllAudio();
    AppState.chat.stateful.summary = '';
    AppState.chat.stateful.lastResponseId = null;
    AppState.chat.stateful.turnCount = 0;
    AppState.chat.stateful.estimatedChars = 0;
    AppState.chat.stateful.resetCount = 0;
    AppState.chat.stateful.lastInputTokens = 0;
    AppState.chat.stateful.lastOutputTokens = 0;
    AppState.chat.stateful.peakInputTokens = 0;
    AppState.chat.messages = [];
    AppState.ui.scroll.pendingToBottom = false;
    chatMessages.innerHTML = '';
    updateScrollToBottomButton();
    resetServerChatReplayState();
    AppState.session.currentCache = null;
    stopChatSessionPolling();
    AppState.chat.activeLocalTurnId = '';
    AppState.chat.activeLocalAssistantId = '';
    AppState.chat.locallyRenderedTurnIds = new Set();
    AppState.chat.isGenerating = false;
    AppState.chat.abortController = null;
    broadcastLLMActivityState(false, 'finished');
    hideProgressDock();
    updateSendButtonState();
    updateStatefulBudgetIndicator();
}

function renderSessionRestoreSkeleton(cardCount = 5) {
    if (!chatRestoreOverlay) return;
    const total = Math.max(3, Math.min(8, Number(cardCount) || 5));
    const cards = Array.from({ length: total }, (_, index) => {
        const widthClass = index % 3 === 0 ? 'is-wide' : index % 3 === 1 ? 'is-medium' : 'is-short';
        return `
            <div class="session-restore-card">
                <div class="session-restore-line is-title ${widthClass}"></div>
                <div class="session-restore-line is-body is-wide"></div>
                <div class="session-restore-line is-body is-medium"></div>
            </div>`;
    }).join('');

    chatRestoreOverlay.hidden = false;
    chatRestoreOverlay.innerHTML = `
        <div class="session-restore-skeleton">
            <div class="session-restore-heading">${escapeHtml(t('restore.skeletonTitle'))}</div>
            <div class="session-restore-subheading">${escapeHtml(t('restore.skeletonBody'))}</div>
            <div class="session-restore-list">${cards}</div>
        </div>`;
}

function beginChatSessionRestore(totalCount = 0) {
    AppState.session.isRestoring = true;
    resetChatViewState();
    chatMessages?.classList.add('is-session-hydrating');
    renderSessionRestoreSkeleton(totalCount);
    renderProgressDock(t('progress.restoringHistory'), 0, 'prompt-processing', false);
    updateScrollToBottomButton();
    updateMessageInputPlaceholder();
}

function updateChatSessionRestoreProgress(loadedCount, totalCount) {
    if (!AppState.session.isRestoring) return;
    const total = Math.max(1, Number(totalCount) || 1);
    const loaded = Math.max(0, Math.min(total, Number(loadedCount) || 0));
    renderProgressDock(t('progress.restoringHistory'), (loaded / total) * 100, 'prompt-processing', false);
    updateMessageInputPlaceholder();
}

function finishChatSessionRestore() {
    AppState.session.isRestoring = false;
    hideProgressDock();
    requestAnimationFrame(() => {
        if (chatRestoreOverlay) {
            chatRestoreOverlay.innerHTML = '';
            chatRestoreOverlay.hidden = true;
        }
        chatMessages?.classList.remove('is-session-hydrating');
        scrollToBottom(true);
        requestAnimationFrame(() => {
            scrollToBottom(true);
        });
        ensureChatRestoredToLatest();
    });
    updateMessageInputPlaceholder();
}

async function clearChat() {
    triggerHaptic('buzz');
    // Stop any TTS playback and generation
    stopAllAudio();

    if (AppState.chat.isGenerating) {
        await stopGeneration();
    }

    AppState.chat.stateful.pendingResetReason = 'manual_clear_chat';

    try {
        await fetch('/api/chat-session/clear', {
            method: 'POST',
            credentials: 'include'
        });
    } catch (e) {
        console.warn('Failed to clear current chat session on server:', e);
    }

    AppState.session.lastCache = null;
    AppState.session.lastFetchPromise = null;
    resetChatViewState();
}

function clearContext() {
    AppState.chat.stateful.lastResponseId = null;
    AppState.chat.stateful.turnCount = 0;
    AppState.chat.stateful.estimatedChars = AppState.chat.stateful.summary.length;
    AppState.chat.stateful.pendingResetReason = 'manual_context_reset';
    AppState.chat.stateful.lastInputTokens = 0;
    AppState.chat.stateful.lastOutputTokens = 0;
    showAlert(t('setting.memory.reset.success') + ' (Context/Session ID Cleared)');
    console.log('[Context] Manual context reset trigger.');
    updateStatefulBudgetIndicator();
}

function getBaseSystemPrompt() {
    return (config.systemPrompt || 'You are a helpful AI assistant.').trim() || 'You are a helpful AI assistant.';
}

function buildClientStatefulPrompt() {
    let prompt = getBaseSystemPrompt();
    if (usesStatefulConversationContext() && AppState.chat.stateful.summary) {
        prompt += `\n\n### Conversation Summary ###\n${AppState.chat.stateful.summary}\n\nUse this summary as compressed context from earlier turns.`;
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
    const recent = AppState.chat.messages.slice(-12);
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
    if (AppState.chat.stateful.summary) {
        summary = `Previous summary:\n${AppState.chat.stateful.summary}\n\nRecent turns:\n${summary}`;
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
    const limitTurns = parseInt(config.statefulTurnLimit, 10) || DEFAULT_STATEFUL_TURN_LIMIT;
    const charBudget = parseInt(config.statefulCharBudget, 10) || DEFAULT_STATEFUL_CHAR_BUDGET;
    const tokenBudget = parseInt(config.statefulTokenBudget, 10) || DEFAULT_STATEFUL_TOKEN_BUDGET;
    const projectedChars = AppState.chat.stateful.estimatedChars + (nextUserText ? nextUserText.length : 0);
    const projectedTokens = AppState.chat.stateful.lastInputTokens + estimateTokensFromText(nextUserText);
    const turnFactor = Math.min(1, AppState.chat.stateful.turnCount / Math.max(limitTurns, 1));
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

function updateStatefulBudgetIndicator(nextUserText = '') {
    if (!statefulBudgetIndicator) {
        return;
    }

    const shouldShow = usesStatefulConversationContext();
    if (!shouldShow) {
        statefulBudgetIndicator.hidden = true;
        return;
    }

    const risk = getStatefulRiskMetrics(nextUserText);
    const charBudget = Math.max(risk.charBudget || 0, 1);
    const fillRatio = Math.max(0, Math.min(1, risk.projectedChars / charBudget));
    const coreOpacity = Math.max(0, Math.min(1, (fillRatio - 0.55) / 0.45));

    let ringColor = 'rgba(113, 153, 133, 0.92)';
    if (fillRatio >= 0.9) {
        ringColor = 'rgba(248, 81, 73, 0.96)';
    } else if (fillRatio >= 0.72) {
        ringColor = 'rgba(210, 153, 34, 0.96)';
    } else if (fillRatio >= 0.45) {
        ringColor = 'rgba(88, 166, 255, 0.94)';
    }

    statefulBudgetIndicator.hidden = false;
    statefulBudgetIndicator.style.setProperty('--stateful-budget-progress', `${Math.round(fillRatio * 360)}deg`);
    statefulBudgetIndicator.style.setProperty('--stateful-budget-color', ringColor);
    statefulBudgetIndicator.style.setProperty('--stateful-budget-core-opacity', coreOpacity.toFixed(3));
}

function shouldResetStatefulContext(nextUserText = '') {
    if (!usesStatefulConversationContext()) {
        return { shouldReset: false, reasons: [], risk: getStatefulRiskMetrics(nextUserText) };
    }
    const risk = getStatefulRiskMetrics(nextUserText);
    const reasons = [];
    if (AppState.chat.stateful.turnCount >= risk.turnLimit) {
        reasons.push(`turns ${AppState.chat.stateful.turnCount}/${risk.turnLimit}`);
    }
    if (risk.projectedChars >= risk.charBudget) {
        reasons.push(`chars ${risk.projectedChars}/${risk.charBudget}`);
    }
    if (risk.projectedTokens >= risk.tokenBudget) {
        reasons.push(`projected tokens ${risk.projectedTokens}/${risk.tokenBudget}`);
    }
    if (AppState.chat.stateful.lastInputTokens >= risk.tokenBudget) {
        reasons.push(`input tokens ${AppState.chat.stateful.lastInputTokens}/${risk.tokenBudget}`);
    }
    return {
        shouldReset: reasons.length > 0,
        reasons,
        risk
    };
}

function clampNumber(value, min, max) {
    return Math.min(max, Math.max(min, value));
}

function normalizeReasoningValue(value) {
    const normalized = String(value || '').trim().toLowerCase();
    return DEFAULT_REASONING_OPTIONS.includes(normalized) ? normalized : '';
}

function updateHeaderModelDisplay() {
    return modelController.updateHeaderModelDisplay();
}

function closeModelPickerModal() {
    return modelController.closeModelPickerModal();
}

function renderModelPickerModal() {
    return modelController.renderModelPickerModal();
}

function openModelPickerModal(event) {
    return modelController.openModelPickerModal(event);
}

function selectHeaderModel(modelId) {
    return modelController.selectHeaderModel(modelId);
}

function unloadHeaderModel(event, instanceId) {
    return modelController.unloadHeaderModel(event, instanceId);
}

function persistAppConfigSnapshot() {
    localStorage.setItem('appConfig', JSON.stringify(config));
}

function persistClientConfig() {
    persistAppConfigSnapshot();
}

function updateComposerLayoutMetrics() {
    const root = document.documentElement;
    if (!root) return;

    const inputAreaHeight = inputArea ? Math.ceil(inputArea.getBoundingClientRect().height) : 0;
    root.style.setProperty('--input-area-height', `${Math.max(88, inputAreaHeight)}px`);
    root.style.setProperty('--reasoning-control-height', '0px');
}

function renderReasoningControl() {
    return modelController.renderReasoningControl();
}

function updateReasoningControlVisibility() {
    if (!reasoningControlBar) return;

    const scrollButtonVisible = !!scrollToBottomBtn?.classList.contains('is-visible');
    const shouldReveal = !reasoningControlBar.hidden && !scrollButtonVisible && !AppState.chat.isGenerating;

    reasoningControlBar.classList.toggle('is-visible', shouldReveal);
    reasoningControlBar.classList.toggle('is-suppressed', !shouldReveal);
    reasoningControlBar.setAttribute('aria-hidden', shouldReveal ? 'false' : 'true');
}

function getEffectiveReasoningSelection() {
    return modelController.getEffectiveReasoningSelection();
}

function getConfiguredTemperature() {
    config.temperature = normalizeTemperatureValue(config.temperature, null);
    return config.temperature;
}

function buildRepeatRecoveryOverrides() {
    const baseTemperature = getConfiguredTemperature();
    const overrides = {
        repeatPenalty: LM_STUDIO_REPEAT_RECOVERY_PENALTY
    };
    if (baseTemperature !== null) {
        overrides.temperature = clampNumber(baseTemperature + LM_STUDIO_REPEAT_RECOVERY_TEMPERATURE_DELTA, 0, 1);
    }
    return overrides;
}

function buildChatPayload({ text, currentImage, temperatureOverride = null, repeatPenaltyOverride = null } = {}) {
    const systemMsg = { role: 'system', content: getBaseSystemPrompt() };
    const configuredTemperature = getConfiguredTemperature();
    const reasoningSelection = getEffectiveReasoningSelection();
    const hasTemperatureOverride = temperatureOverride !== null
        && temperatureOverride !== undefined
        && Number.isFinite(Number(temperatureOverride));
    const resolvedTemperature = hasTemperatureOverride
        ? Number(temperatureOverride)
        : configuredTemperature;
    let payload = {};
    const contextStrategy = getNormalizedContextStrategy();

    if (config.llmMode === 'stateful') {
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
            // The client sends only the user's base prompt and local summary.
            // Runtime tool/memory instructions are injected on the server.
            system_prompt: buildClientStatefulPrompt(),
            stream: true
        };
        if (resolvedTemperature !== null) {
            payload.temperature = resolvedTemperature;
        }

        if (repeatPenaltyOverride !== null
            && repeatPenaltyOverride !== undefined
            && Number.isFinite(Number(repeatPenaltyOverride))
            && Number(repeatPenaltyOverride) > 0) {
            payload.repeat_penalty = Number(repeatPenaltyOverride);
        }

        if (contextStrategy !== 'stateful') {
            payload.store = false;
        }

        if (contextStrategy === 'stateful' && AppState.chat.stateful.lastResponseId) {
            payload.previous_response_id = AppState.chat.stateful.lastResponseId;
        }
        if (reasoningSelection) {
            payload.reasoning = reasoningSelection;
        }
        return payload;
    }

    const messageSource = contextStrategy === 'history'
        ? AppState.chat.messages.slice(-((parseInt(config.historyCount, 10) || 10) * 2))
        : AppState.chat.messages.slice(-1);

    const payloadHistory = messageSource.map(m => {
        if (m.image) {
            const visionContent = [];
            if (m.content) {
                visionContent.push({ type: 'text', text: m.content });
            }
            visionContent.push({ type: 'image_url', image_url: { url: m.image } });
            return {
                role: m.role,
                content: visionContent
            };
        }

        let content = m.content || '';
        if (m.role === 'assistant') {
            content = content.replace(/<think>[\s\S]*?<\/think>/g, '').trim();
        }
        return { role: m.role, content };
    });

    payload = {
        model: config.model,
        messages: [systemMsg, ...payloadHistory],
        stream: true
    };
    if (resolvedTemperature !== null) {
        payload.temperature = resolvedTemperature;
    }

    if (reasoningSelection) {
        payload.reasoning_effort = reasoningSelection;
    }

    return payload;
}

async function ensureStatefulContextBudget(nextUserText = '') {
    const resetDecision = shouldResetStatefulContext(nextUserText);
    if (!resetDecision.shouldReset) {
        return;
    }

    const compactedFrom = AppState.chat.stateful.peakInputTokens || AppState.chat.stateful.lastInputTokens || 0;
    const compactReasons = resetDecision.reasons.join(', ');
    console.log('[Stateful] Auto compact triggered', {
        reasons: resetDecision.reasons,
        turnCount: AppState.chat.stateful.turnCount,
        projectedChars: resetDecision.risk.projectedChars,
        projectedTokens: resetDecision.risk.projectedTokens,
        tokenBudget: resetDecision.risk.tokenBudget,
        charBudget: resetDecision.risk.charBudget,
        turnLimit: resetDecision.risk.turnLimit
    });
    AppState.chat.stateful.summary = summarizeMessagesForStatefulReset();
    AppState.chat.stateful.lastResponseId = null;
    AppState.chat.stateful.turnCount = 0;
    AppState.chat.stateful.estimatedChars = AppState.chat.stateful.summary.length;
    AppState.chat.stateful.resetCount += 1;
    AppState.chat.stateful.pendingResetReason = 'auto_summary_reset';
    AppState.chat.stateful.lastInputTokens = estimateTokensFromText(AppState.chat.stateful.summary);
    AppState.chat.stateful.lastOutputTokens = 0;
    updateStatefulBudgetIndicator(nextUserText);
    appendMessage({
        role: 'system',
        content: `Stateful context compacted ${compactedFrom} -> ~${AppState.chat.stateful.lastInputTokens}`
    });
}


/* Chat Logic */

async function sendMessage(options = {}) {
    if (savedLibraryController.isOpen()) {
        closeSavedLibrary();
    }
    cancelComposerBackgroundTasks('user-message');
    // Unlock audio context on user interaction
    unlockAudioContext();

    let text = messageInput.value.trim();
    const currentImage = AppState.input.pendingImage; // Capture early

    if (!text && !currentImage) return;
    if (AppState.chat.isGenerating) {
        await stopGeneration();
        await new Promise((resolve) => setTimeout(resolve, 120));
    }

    dismissStartupCards();

    // Stop and clear any existing audio/TTS
    stopAllAudio();
    AppState.input.pendingVoiceInputAutoTTS = !!options.fromVoiceInput;

    // Prepare User Message
    const turnId = generateTurnId();
    const userMsg = {
        role: 'user',
        content: text,
        image: currentImage,
        turnId
    };

    const isLabelTop = getStreamingScrollMode() === 'label-top';
    logScrollTrace('sendMessage:start', {
        turnId,
        mode: isLabelTop ? 'label-top' : 'auto',
        textLength: text.length
    });
    appendMessage(userMsg);
    if (!isLabelTop) {
        AppState.ui.scroll.lockToLatest = true;
        AppState.ui.scroll.shouldAutoScroll = true;
        holdAutoScrollAtBottom(600);
    }
    AppState.chat.messages.push(userMsg);
    if (usesStatefulConversationContext()) {
        AppState.chat.stateful.estimatedChars += text.length;
        updateStatefulBudgetIndicator();
    }

    // Reset Input
    messageInput.value = '';
    removeImage();
    autoResizeInput();

    // Prepare Assistant Placeholder
    AppState.chat.isGenerating = true;
    syncWakeLock(); // Request wake lock for generation
    broadcastLLMActivityState(true, 'answering');
    AppState.ui.scroll.lockToLatest = true;
    updateSendButtonState();

    // Create new AbortController
    AppState.chat.abortController = new AbortController();

    const assistantId = buildServerAssistantMessageId(turnId, '');
    logScrollTrace('sendMessage:assistant-placeholder', { turnId, assistantId });
    AppState.ui.activeStreamingMessageId = assistantId;
    AppState.chat.activeLocalTurnId = turnId;
    AppState.chat.activeLocalAssistantId = assistantId;
    AppState.chat.assistantTurnIdMap.set(assistantId, turnId);
    ensureAssistantMessageElement(assistantId, turnId);
    const assistantActionBar = document.getElementById(assistantId)?.querySelector('.message-actions');
    if (assistantActionBar) {
        assistantActionBar.hidden = true;
        assistantActionBar.classList.remove('is-ready');
        assistantActionBar.classList.add('is-pending');
    }
    stopChatSessionPolling();

    let assistantContent = '';
    try {
        let retryOverrides = null;
        for (let attempt = 0; attempt < 2; attempt++) {
            const payload = buildChatPayload({
                text,
                currentImage,
                temperatureOverride: retryOverrides?.temperature ?? null,
                repeatPenaltyOverride: retryOverrides?.repeatPenalty ?? null
            });

            console.log('=== LLM Request Payload ===');
            console.log('Attempt:', attempt + 1);
            if (Object.prototype.hasOwnProperty.call(payload, 'temperature')) {
                console.log('Temperature:', payload.temperature);
            } else {
                console.log('Temperature: Auto (omitted)');
            }
            if (payload.repeat_penalty) {
                console.log('Repeat Penalty:', payload.repeat_penalty);
            }

            try {
                assistantContent = await streamResponse(payload, assistantId, turnId, {
                    repeatRecoveryApplied: attempt > 0,
                    fromVoiceInput: !!options.fromVoiceInput
                });
                break;
            } catch (e) {
                if (e?.code === 'LMSTUDIO_RUNAWAY_REPETITION'
                    && config.llmMode === 'stateful'
                    && attempt === 0) {
                    retryOverrides = buildRepeatRecoveryOverrides();
                    AppState.session.retryRecoveryInProgress = true;
                    await stopGeneration({ preserveAssistantUI: true });
                    updateMessageContent(assistantId, '');
                    finalizeReasoningStatus(assistantId, 'failed', t('warning.repeatRetrying'));
                    hideProgressDock();
                    continue;
                }
                throw e;
            }
        }
    } catch (e) {
        if (e.name === 'AbortError') {
            finalizeAssistantStatusCards(assistantId, 'stopped', t('status.stopped'));
            updateMessageContent(assistantId, `**[Stopped by User]**`);
            setAssistantActionBarReady(assistantId);
        } else if (e?.streamDetached || isLikelyStreamDetachError(e)) {
            setComposerBackgroundTask('server-chat-detached', {
                label: t('background.serverChatContinuing')
            });
            scheduleChatSessionPolling(250);
        } else {
            finalizeAssistantStatusCards(assistantId, 'failed', e.message || t('status.failed'));
            updateMessageContent(assistantId, `**Error:** ${e.message}`);
            setAssistantActionBarReady(assistantId);
        }
    } finally {
        AppState.session.retryRecoveryInProgress = false;
        AppState.input.pendingVoiceInputAutoTTS = false;
        AppState.chat.isGenerating = false;
        syncWakeLock(); // Release wake lock if nothing else is active
        broadcastLLMActivityState(false, 'finished');
        AppState.ui.scroll.lockToLatest = false;
        stopStreamingMessageAutoScroll();
        AppState.ui.activeStreamingMessageId = null;
        AppState.chat.abortController = null;
        updateSendButtonState();
        fastForwardChatSessionEvents().catch(console.warn);
        scheduleChatSessionPolling(600);
    }

    if (assistantContent && assistantContent.trim()) {
        ensureLastSessionCacheLoaded(true).catch(console.warn);
    }
}

function shouldAutoPlayTTSForRequest(options = {}) {
    if (!config.enableTTS) return false;
    if (options.fromVoiceInput) {
        return config.voiceInputAutoTTS !== false;
    }
    return !!config.autoTTS;
}

async function stopCurrentChatSessionOnServer() {
    try {
        await fetch('/api/chat-session/stop', buildSessionFetchOptions({ method: 'POST' }));
    } catch (e) {
        console.warn('Failed to stop current chat session on server:', e);
    }
}

async function stopGeneration({ preserveAssistantUI = false } = {}) {
    if (AppState.chat.abortController) {
        AppState.chat.abortController.abort();
        AppState.chat.abortController = null;
    }
    await stopCurrentChatSessionOnServer();
    hideProgressDock();
    if (!preserveAssistantUI) {
        relinquishLocalStreamOwnership('stop-generation');
    } else {
        AppState.session.localStreamOwnershipReleased = false;
    }
    // Stop any currently playing audio/TTS
    stopAllAudio();
}

function detectRunawayRepetition(text = '') {
    const normalized = getLoopDetectionTail(text, 140).replace(/\s+/g, ' ').trim();
    if (normalized.length < 140) return null;

    const shortLoopMatch = normalized.match(/([\s\S]{6,}?)\1{8,}/);
    if (shortLoopMatch && shortLoopMatch[1]) {
        return {
            snippet: shortLoopMatch[1].slice(0, 80),
            source: 'chunk-loop'
        };
    }

    const sentenceLoopMatch = normalized.match(/(.{12,}?[.!?"])(?:\s+\1){5,}/i);
    if (sentenceLoopMatch && sentenceLoopMatch[1]) {
        return {
            snippet: sentenceLoopMatch[1].slice(0, 120),
            source: 'sentence-loop'
        };
    }

    const wordLoopMatch = normalized.match(/\b([^\s]{2,30})\b(?:\s+\1){11,}/i);
    if (wordLoopMatch && wordLoopMatch[1]) {
        return {
            snippet: wordLoopMatch[1],
            source: 'word-loop'
        };
    }

    return null;
}

function updateSendButtonState() {
    updateSendButtonStateCore();
    syncGlobalLLMComposerUI(); // Calls updateMessageInputPlaceholder internally
}

function isComposerStopActionActive(session = AppState.session.currentCache) {
    const sessionRunning = AppState.chat.passive.sessionLLMRunning || String(session?.Status || '').trim().toLowerCase() === 'running';
    const localGenerating = !!AppState.chat.abortController || !!AppState.chat.isGenerating;
    return localGenerating || sessionRunning;
}

/**
 * Core logic for updating button icon and labels without triggering full UI sync
 */
function updateSendButtonStateCore() {
    const isCurrentlyActive = isComposerStopActionActive();

    if (isCurrentlyActive) {
        sendBtn.disabled = false; // Enabled so we can Click to Stop
        sendBtn.innerHTML = '<span class="material-icons-round">stop</span>';
        sendBtn.title = t('action.stopGeneration');
        sendBtn.classList.add('stop-btn');
    } else {
        sendBtn.disabled = false;
        sendBtn.innerHTML = '<span class="material-icons-round">send</span>';
        sendBtn.title = t('action.send') || "Send Message";
        sendBtn.classList.remove('stop-btn');
    }

    inputContainer?.classList.toggle('is-generating', isCurrentlyActive);

    // Also update giant mic icon if layout is active
    updateMicUIForGeneration(isCurrentlyActive);
    updateInlineComposerActionVisibility();
}

function hasComposableUserInput() {
    const text = messageInput?.value?.trim() || '';
    return !!text || !!AppState.input.pendingImage;
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

    const nextUserText = messageInput?.value?.trim() || '';
    updateStatefulBudgetIndicator(nextUserText);

    const inlineMicAvailable = inlineMicBtn.classList.contains('is-visible');
    if (!inlineMicAvailable) {
        setComposerPrimaryButtons({ showSend: true, showInlineMic: false });
        return;
    }

    if (isComposerStopActionActive()) {
        setComposerPrimaryButtons({ showSend: true, showInlineMic: false });
        return;
    }

    const shouldShowSend = hasComposableUserInput();
    setComposerPrimaryButtons({ showSend: shouldShowSend, showInlineMic: !shouldShowSend });
}


async function streamResponse(payload, elementId, turnId = '', streamOptions = {}) {
    const headers = { 'Content-Type': 'application/json' };
    if (turnId) {
        headers['X-Client-Turn-Id'] = turnId;
    }
    if (AppState.ui.location.currentUserLocation) {
        headers['X-User-Location'] = AppState.ui.location.currentUserLocation;
    }
    if (AppState.chat.stateful.pendingResetReason) {
        headers['X-Stateful-Reset-Reason'] = AppState.chat.stateful.pendingResetReason;
    }
    headers['X-Context-Strategy'] = getNormalizedContextStrategy();

    let response;
    try {
        response = await fetch('/api/chat', buildSessionFetchOptions({
            method: 'POST',
            headers,
            body: JSON.stringify(payload),
            signal: AppState.chat.abortController?.signal
        }));
    } catch (error) {
        console.warn('[Chat] Primary /api/chat request failed before response. Retrying with minimal request options.', error);
        response = await fetch('/api/chat', buildSessionFetchOptions({
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        }));
    }

    if (!response.ok) {
        let errorDetails = `Server Error ${response.status}: ${response.statusText}`;
        const errorBody = await response.text();
        if (errorBody) {
            const localizedError = getLocalizedRuntimeErrorMessage(errorBody);
            if (localizedError) {
                errorDetails = localizedError;
                throw new Error(errorDetails);
            }

            if (errorBody.includes("Could not find stored response for previous_response_id")) {
                console.warn("[Stateful] previous_response_id became invalid. Resetting and retrying without it...");
                AppState.chat.stateful.lastResponseId = null;
                AppState.chat.stateful.turnCount = 0;
                AppState.chat.stateful.estimatedChars = AppState.chat.stateful.summary.length;
                AppState.chat.stateful.lastInputTokens = estimateTokensFromText(AppState.chat.stateful.summary);
                AppState.chat.stateful.lastOutputTokens = 0;
                AppState.chat.stateful.resetCount += 1;
                AppState.chat.stateful.pendingResetReason = 'invalid_previous_response_id';
                // Re-attempt without current AppState.chat.stateful.lastResponseId
                delete payload.previous_response_id;
                return await streamResponse(payload, elementId);
            }

            errorDetails += ` - ${errorBody}`;

        }
        throw new Error(errorDetails);
    }
    AppState.chat.stateful.pendingResetReason = null;

    return await processStream(response, elementId, turnId, streamOptions);
}

// Helper to process the stream reader (shared by direct and proxy)
async function processStream(response, elementId, turnId = '', streamOptions = {}) {
    const reader = response.body.getReader();
    const parser = new SSEParser();
    const ctx = new StreamState(elementId, turnId, streamOptions);

    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const events = parser.parse(value);
            for (const event of events) {
                await handleStreamEvent(event, ctx);
            }
        }
    } catch (err) {
        if (err.name === 'AbortError') {
            ctx.streamAborted = true;
            console.log('Stream aborted by user');
        } else if (isLikelyStreamDetachError(err)) {
            ctx.streamDetached = true;
            err.streamDetached = true;
            console.warn('Stream detached while server may still be running:', err);
            throw err;
        } else {
            console.error('Stream Error:', err);
            throw err;
        }
    } finally {
        return finalizeStream(ctx);
    }
}

function createMessageElement(msg) {
    const div = document.createElement('div');
    div.className = `message ${msg.role}`;
    if (msg.id) div.id = msg.id;
    if (msg.turnId) div.dataset.turnId = msg.turnId;

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
        if (msg.startup.kind === 'reconnect') {
            div.classList.add('has-reconnect-card');
        }
        const startup = msg.startup;
        const issues = Array.isArray(startup.issues) ? startup.issues : [];
        const issuesHtml = issues.length > 0
            ? `<ul class="startup-issues">${issues.map(issue => `<li>${escapeHtml(issue)}</li>`).join('')}</ul>`
            : '';
        const actionButtons = [];
        if (startup.showRestoreButton) {
            actionButtons.push(`<button class="startup-action-btn" onclick="restoreLastSession()">${escapeHtml(startup.restoreLabel || t('chat.startup.restore'))}</button>`);
        }
        if (startup.actionLabel && startup.actionHandler) {
            actionButtons.push(`<button class="startup-action-btn" onclick="${escapeAttr(startup.actionHandler)}">${escapeHtml(startup.actionLabel)}</button>`);
        }
        const actionHtml = actionButtons.length > 0
            ? `<div class="startup-actions">${actionButtons.join('')}</div>`
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
                <div class="message-actions">
                    <button class="icon-btn save-btn" onclick="saveMessageTurn(this)" title="${escapeAttr(t('action.saveTurn'))}">
                        <span class="material-icons-round">bookmark_add</span>
                    </button>
                    <button class="icon-btn copy-btn" onclick="copyMessage(this)" title="Copy">
                        <span class="material-icons-round">content_copy</span>
                    </button>
                    <button class="icon-btn speak-btn" onclick="speakMessageFromBtn(this)" title="Speak">
                        <span class="material-icons-round">volume_up</span>
                    </button>
                </div>
            </div>`;
    }

    if (msg.role === 'assistant' && textContent.trim()) {
        const cleanText = sanitizeAssistantRenderText(textContent);
        const committedHost = div.querySelector('.assistant-response-card .markdown-committed');
        const pendingHost = div.querySelector('.assistant-response-card .markdown-pending');
        if (committedHost) {
            renderMarkdownIntoHost(committedHost, cleanText);
        }
        if (pendingHost) {
            pendingHost.innerHTML = '';
            pendingHost.textContent = '';
            pendingHost.dataset.markdownSource = '';
            pendingHost.classList.remove('is-stream-preview');
        }
        div._streamRenderState = {
            committedText: cleanText,
            pendingText: ''
        };
    }

    return div;
}

function appendMessage(msg, options = {}) {
    const wasNearBottom = isChatNearBottom();
    const div = createMessageElement(msg);
    const target = options.parent || chatMessages;
    target.appendChild(div);
    if (!options.skipScroll) {
        scrollToBottom(wasNearBottom || msg.role === 'user');
    } else {
        updateScrollToBottomButton();
    }
    return div;
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

function getSnapshotReasoningDuration(item) {
    const reasoning = item?.reasoning && typeof item.reasoning === 'object' ? item.reasoning : {};
    const total = Number(reasoning?.duration_ms || item?.reasoning_duration_ms || 0);
    const accumulated = Number(reasoning?.accumulated_ms || item?.reasoning_accumulated_ms || 0);
    const currentPhase = Number(reasoning?.current_phase_ms || item?.reasoning_current_phase_ms || 0);
    return Math.max(0, total, accumulated + currentPhase);
}

function ensureAssistantMessageElement(id, turnId = '') {
    const resolvedTurnId = String(
        turnId
        || AppState.chat.assistantTurnIdMap.get(id)
        || (id === AppState.chat.activeLocalAssistantId ? AppState.chat.activeLocalTurnId : '')
        || ''
    ).trim();
    if (id && resolvedTurnId) {
        AppState.chat.assistantTurnIdMap.set(id, resolvedTurnId);
    }
    let el = document.getElementById(id);
    if (el) {
        if (resolvedTurnId && !el.dataset.turnId) {
            el.dataset.turnId = resolvedTurnId;
        }
        attachStreamingAudioButtonToMessage(el);
        syncAssistantMessageShellState(el);
        return el;
    }
    appendMessage({ role: 'assistant', content: '', id, turnId: resolvedTurnId }, { skipScroll: true });
    el = document.getElementById(id);
    if (el && AppState.ui.activeStreamingMessageId === id) {
        startStreamingMessageAutoScroll(id);
    }
    attachStreamingAudioButtonToMessage(el);
    syncAssistantMessageShellState(el);
    return el;
}

function isAssistantMessageVisiblyEmpty(msgEl) {
    if (!msgEl || !msgEl.classList?.contains('assistant')) return false;
    if (msgEl.classList.contains('has-startup-card')) return false;
    const markdownHost = msgEl.querySelector('.assistant-response-card .markdown-body');
    const markdownSource = String(markdownHost?.dataset?.markdownSource || '').trim();
    const markdownText = String(markdownHost?.textContent || '').trim();
    // Also check the committed host's markdownSource and the element's stream render state,
    // because after finalizeMessageContent the content lives in .markdown-committed and _streamRenderState.
    const committedHost = msgEl.querySelector('.assistant-response-card .markdown-committed');
    const committedSource = String(committedHost?.dataset?.markdownSource || '').trim();
    const streamStateText = String(msgEl._streamRenderState?.committedText || '').trim();
    const reasoningText = String(msgEl.querySelector('.assistant-reasoning')?.textContent || '').trim();
    // Corrected selector from .tool-card to .tool-status-card
    const hasToolCards = msgEl.querySelectorAll('.assistant-tools .tool-status-card').length > 0;
    const hasImage = !!msgEl.querySelector('.message-image');
    const responseCard = msgEl.querySelector('.assistant-response-card');
    // If the response card is NOT hidden and has content, it's not empty.
    // Also, if it has an image (which is inside the response card but might be shown even if text is empty), it's not empty.
    const hasVisibleResponse = (!responseCard?.hidden && (!!markdownSource || !!markdownText || !!committedSource || !!streamStateText)) || hasImage;
    const hasVisibleReasoning = !!reasoningText;
    return !hasVisibleResponse && !hasVisibleReasoning && !hasToolCards;
}

function syncAssistantMessageShellState(msgEl) {
    if (!msgEl || !msgEl.classList?.contains('assistant')) return;
    msgEl.classList.toggle('is-empty-shell', isAssistantMessageVisiblyEmpty(msgEl));
}

function cleanupTrailingEmptyAssistantMessages() {
    if (!chatMessages) return;
    const children = Array.from(chatMessages.children);
    for (let i = children.length - 1; i >= 0; i -= 1) {
        const node = children[i];
        if (!(node instanceof HTMLElement) || !node.classList.contains('message')) {
            continue;
        }
        syncAssistantMessageShellState(node);
        if (!node.classList.contains('assistant')) {
            break;
        }
        if (isAssistantMessageVisiblyEmpty(node)) {
            node.remove();
            continue;
        }
        break;
    }
}

function getAssistantMessageParts(elementId, turnId = '') {
    return chatUIController.getAssistantMessageParts(elementId, turnId);
}

function syncCurrentAudioButtonUI() {
    return ttsController.syncCurrentAudioButtonUI();
}

function attachStreamingAudioButtonToMessage(msgEl) {
    return ttsController.attachStreamingAudioButtonToMessage(msgEl);
}

chatUIController = appChatUI.createChatUIController({
    refs: {
        chatMessages,
        inputArea,
        reasoningControlBar,
        scrollToBottomBtn
    },
    deps: {
        attachStreamingAudioButtonToMessage,
        commitChatScrollMetrics,
        getStreamingScrollMode,
        isAssistantMessageVisiblyEmpty,
        pulseMessageRender,
        refreshChatScrollMetrics,
        savedLibraryIsOpen: () => savedLibraryController.isOpen(),
        triggerHaptic,
        updateReasoningControlVisibility
    }
});

chatStreamingController = appChatStreaming.createChatStreamingController({
    deps: {
        attachStreamingAudioButtonToMessage,
        checkAndTriggerLabelPin,
        ensureAssistantMessageElement,
        ensureStreamingMarkdownHosts,
        getMarkdownRenderMode,
        getPassiveSyncWaitingText,
        getStreamingScrollMode,
        holdAutoScrollAtBottom,
        isChatNearBottom,
        observeAutoScrollResizes,
        pulseMessageRender,
        reconcileAssistantActionBarForMessage,
        renderMarkdownIntoHost,
        renderStreamingPreviewIntoHost,
        sanitizeAssistantRenderText,
        scrollToBottom,
        syncAssistantMessageShellState
    }
});

progressController = DKSTProgressUI.createProgressController({
    refs: {
        chatProgressDock,
        inputContainer
    },
    deps: {
        AppState,
        updateMessageInputPlaceholder,
        updateSendButtonStateCore
    }
});

micController = DKSTMic.createMicController({
    refs: {
        messageInput,
        inlineMicBtn,
        giantMicBtn: document.getElementById('giant-mic-btn'),
        micLayoutContainer: document.getElementById('mic-layout-container')
    },
    deps: {
        AppState,
        config,
        t,
        triggerHaptic,
        sendMessage,
        updateSendButtonState,
        updateMessageInputPlaceholder
    }
});

/**
 * Register AppState Reactors (Automatic UI Updates)
 */
function registerReactors() {
    // 1. Chat Generation State
    AppStateEvents.on('chat.isGenerating', () => {
        updateSendButtonState();
        updateMessageInputPlaceholder();
        if (config.micLayout !== 'none') micController.updateLayout();
    });

    // 2. Audio Playback State
    AppStateEvents.on('audio.isPlayingQueue', () => {
        syncCurrentAudioButtonUI();
        updateSendButtonState();
    });
    AppStateEvents.on('audio.streamingTTSActive', () => {
        syncCurrentAudioButtonUI();
        updateSendButtonState();
    });

    // 3. STT State
    AppStateEvents.on('input.isSTTActive', () => {
        // Handled by controller internally or via reactors if needed
    });

    // 4. Mic Layout State
    AppStateEvents.on('ui.micLayout', () => {
        updateMicLayout();
    });
}

registerReactors();
updateMicLayout();

function setAssistantActionBarReady(elementId) {
    return chatUIController.setAssistantActionBarReady(elementId);
}

function reconcileAssistantActionBarForMessage(msgEl) {
    return chatUIController.reconcileAssistantActionBarForMessage(msgEl);
}

function reconcileVisibleAssistantActionBars() {
    return chatUIController.reconcileVisibleAssistantActionBars();
}

function renderProgressDock(label, percent = null, mode = 'prompt-processing', indeterminate = false) {
    return progressController.render(label, percent, mode, indeterminate);
}

function hideProgressDock() {
    return progressController.hide();
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
        card.dataset.accumulatedDurationMs = '0';
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

function setReasoningCardStartedAt(elementId, startedAtValue) {
    const card = ensureReasoningCard(elementId);
    if (!card || !startedAtValue) return;
    const timestamp = new Date(startedAtValue).getTime();
    if (Number.isFinite(timestamp) && timestamp > 0) {
        card.dataset.startedAt = String(timestamp);
    }
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

    const activeCard = getActiveToolCard(elementId);
    if (activeCard) {
        if (toolName) {
            activeCard.dataset.toolName = toolName;
        }
        return activeCard;
    }

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
    checkAndTriggerLabelPin(); // Transition to assistant view when tool starts
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
    syncAssistantMessageShellState(card.closest('.message.assistant'));

    // Explicitly refresh send button to ensure Stop icon is visible during tool use
    updateSendButtonStateCore();
}

function extractToolPreview(args, summary = '', toolName = '') {
    const normalizedTool = String(toolName || '').trim().toLowerCase();

    if (normalizedTool === 'get_current_time') {
        return t('tool.currentTimeChecked');
    }
    if (normalizedTool === 'get_current_location') {
        return t('tool.currentLocationChecked');
    }

    if (args && typeof args === 'object') {
        const detail = formatToolPreviewFromObject(args, normalizedTool);
        if (detail) return detail;
    }

    if (typeof args === 'string' && args.trim()) {
        if (normalizedTool === 'get_current_time' && args.trim() === '{}') {
            return t('tool.currentTimeChecked');
        }
        if (normalizedTool === 'get_current_location' && args.trim() === '{}') {
            return t('tool.currentLocationChecked');
        }
        return args.trim();
    }

    if (summary && !/^running$/i.test(summary.trim()) && !isToolExecutionFinishedSummary(summary.trim())) {
        return summary.trim();
    }

    return '';
}

function formatToolPreviewFromObject(args, normalizedTool) {
    const queryLike = extractToolObjectValue(args, ['query', 'keyword', 'title', 'input', 'prompt', 'text']);
    const url = extractToolObjectValue(args, ['url']);
    const sourceID = extractToolObjectValue(args, ['source_id']);
    const command = extractToolObjectValue(args, ['command']);
    const memoryID = extractToolObjectValue(args, ['memory_id']);

    switch (normalizedTool) {
        case 'search_web':
        case 'namu_wiki':
        case 'naver_search':
            if (queryLike) return t('tool.searchQuery').replace('{value}', queryLike);
            break;
        case 'read_web_page':
            if (url) return t('tool.openUrl').replace('{value}', url);
            break;
        case 'read_buffered_source':
            if (queryLike) return t('tool.readBufferedSource').replace('{value}', queryLike);
            if (sourceID) return t('tool.readBufferedSource').replace('{value}', sourceID);
            break;
        case 'search_memory':
            if (queryLike) return t('tool.searchMemory').replace('{value}', queryLike);
            break;
        case 'read_memory':
            if (memoryID) return t('tool.readMemory').replace('{value}', memoryID);
            break;
        case 'delete_memory':
            if (memoryID) return t('tool.deleteMemory').replace('{value}', memoryID);
            break;
        case 'execute_command':
            if (command) return t('tool.executeCommand').replace('{value}', command);
            break;
        case 'get_current_location':
            return t('tool.currentLocationChecked');
        case 'get_current_time':
            return t('tool.currentTimeChecked');
    }

    const candidateKeys = ['query', 'url', 'text', 'prompt', 'input', 'title'];
    for (const key of candidateKeys) {
        const value = args[key];
        if (typeof value === 'string' && value.trim()) {
            return value.trim();
        }
    }

    const compactJson = stringifyToolArgs(args);
    if (compactJson && compactJson !== '{}') {
        return compactJson;
    }
    return '';
}

function extractToolObjectValue(args, keys) {
    for (const key of keys) {
        const value = args?.[key];
        if (typeof value === 'string' && value.trim()) {
            return value.trim();
        }
        if (typeof value === 'number' && Number.isFinite(value)) {
            return String(value);
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
    if (!cleaned) return t('tool.fallbackName');

    const normalized = cleaned.toLowerCase();
    const knownLabels = {
        get_current_time: 'Get Current Time',
        get_current_location: 'Get Current Location',
        execute_command: 'Execute Command',
        search_web: 'Search Web',
        namu_wiki: 'Namu Wiki',
        naver_search: 'Naver Search',
        read_web_page: 'Read Web Page',
        read_buffered_source: 'Read Buffered Source',
        search_memory: 'Search Memory',
        read_memory: 'Read Memory',
        delete_memory: 'Delete Memory'
    };
    if (knownLabels[normalized]) {
        return knownLabels[normalized];
    }

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
                <div class="tool-history-title">${escapeHtml(`${index + 1}. ${formatToolDisplayName(entry.tool)}`)}</div>
                <div class="tool-history-detail">${escapeHtml(entry.detail || t('tool.noQueryDetails'))}</div>
            </div>
        `).join('');
    } else if (history.length > renderedCount) {
        const fragment = document.createDocumentFragment();
        history.slice(renderedCount).forEach((entry, index) => {
            const item = document.createElement('div');
            item.className = 'tool-history-item is-new';
            item.innerHTML = `
                <div class="tool-history-title">${escapeHtml(`${renderedCount + index + 1}. ${formatToolDisplayName(entry.tool)}`)}</div>
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
async function saveMessageTurn(btn) {
    if (btn?.dataset?.saving === 'true') return;
    triggerHaptic('error');
    const turnData = getTurnDataFromAssistantButton(btn);
    if (!turnData) {
        console.warn('[SavedTurn] saveMessageTurn aborted because turnData is missing');
        showToast(t('library.saveFailed'), true);
        return;
    }
    if (btn) {
        btn.dataset.saving = 'true';
        btn.disabled = true;
    }
    try {
        await saveTurn(turnData.promptText, turnData.responseText);
    } finally {
        if (btn) {
            delete btn.dataset.saving;
            btn.disabled = false;
        }
    }
}

async function copyMessage(btn) {
    const bubble = btn.closest('.message-inner').querySelector('.markdown-body');
    if (!bubble) return;

    // Get text content (try to get clean text without HTML if possible, or just innerText)
    const text = bubble.innerText;
    try {
        await navigator.clipboard.writeText(text);
        triggerHaptic('success');
        showToast(t('clipboard.copied'));
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
        if (successful) {
            triggerHaptic('success');
            showToast(t('clipboard.copied'));
        } else {
            showToast(t('clipboard.copyFailed'), true);
        }
    } catch (err) {
        console.error('Fallback: Oops, unable to copy', err);
        showToast(t('clipboard.copyFailed'), true);
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
    return ttsController.speakMessageFromBtn(btn);
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
    return ttsController.stopAllAudio();
}

// Show/Hide Reasoning Status Helper with Streaming Support
function showReasoningStatus(elementId, text, isFinal = false, elapsedOverrideMs = null) {
    if (config.hideThink) return;

    const card = ensureReasoningCard(elementId);
    if (!card) return;

    const metaEl = card.querySelector('.section-meta');
    const titleEl = card.querySelector('.reasoning-title');
    const bodyEl = card.querySelector('.reasoning-body');
    if (!bodyEl) return;

    const startedAt = Number(card.dataset.startedAt || Date.now());
    const accumulatedMs = Math.max(0, Number(card.dataset.accumulatedDurationMs || 0));
    // When server provides total_elapsed_ms (non-null), use it directly as the absolute total.
    // When null, compute locally: accumulated previous phases + current segment duration.
    const durationMs = (elapsedOverrideMs !== null && elapsedOverrideMs !== undefined && Number.isFinite(Number(elapsedOverrideMs)))
        ? Math.max(0, Number(elapsedOverrideMs))
        : accumulatedMs + Math.max(0, Date.now() - startedAt);

    if (isFinal) {
        finalizeReasoningStatus(elementId, 'done', '', durationMs);
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
        titleEl.textContent = formatThoughtDuration(durationMs);
        if (text && text.trim()) checkAndTriggerLabelPin();
    }
    bodyEl.textContent = cleanText;
    syncAssistantMessageShellState(card.closest('.message.assistant'));

    // Refresh send button during active reasoning
    updateSendButtonStateCore();
}

function finalizeReasoningStatus(elementId, outcome = 'done', detail = '', durationOverrideMs = null) {
    if (config.hideThink) return;

    const card = ensureReasoningCard(elementId);
    if (!card) return;

    const metaEl = card.querySelector('.section-meta');
    const titleEl = card.querySelector('.reasoning-title');
    const bodyEl = card.querySelector('.reasoning-body');
    const startedAt = Number(card.dataset.startedAt || Date.now());
    const accumulatedMs = Math.max(0, Number(card.dataset.accumulatedDurationMs || 0));
    // When server provides total_elapsed_ms (non-null), use it directly as the absolute total.
    // When null, compute locally: accumulated previous phases + current segment duration.
    const durationMs = (durationOverrideMs !== null && durationOverrideMs !== undefined && Number.isFinite(Number(durationOverrideMs)))
        ? Math.max(0, Number(durationOverrideMs))
        : accumulatedMs + Math.max(0, Date.now() - startedAt);
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
    card.dataset.accumulatedDurationMs = String(durationMs);
    card.dataset.startedAt = String(Date.now());

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

    if (bodyEl) {
        let cleanText = (detail || '').replace(/<think>/g, '').replace(/<\/think>/g, '').trim();
        if (!cleanText) {
            cleanText = (AppState.session.replay.reasoningBuffers.get(elementId) || '').replace(/<think>/g, '').replace(/<\/think>/g, '').trim();
        }
        if (cleanText) {
            bodyEl.textContent = cleanText;
        }
    }
    syncAssistantMessageShellState(card.closest('.message.assistant'));

    // Refresh send button when reasoning ends
    updateSendButtonStateCore();
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
        const storedDuration = Number(reasoningCard.dataset.durationMs || reasoningCard.dataset.accumulatedDurationMs || 0);
        finalizeReasoningStatus(
            elementId,
            outcome,
            detail,
            Number.isFinite(storedDuration) && storedDuration > 0 ? storedDuration : null
        );
    }

    // Comprehensive refresh on finalization
    updateSendButtonStateCore();
}

function renderInitialAssistantMarkdown(text) {
    const rendered = buildRenderedMarkdownState(text || '');
    if (!rendered.normalized.trim()) {
        return '<div class="markdown-committed"></div><div class="markdown-pending"></div>';
    }
    return `
        <div class="markdown-committed">${rendered.html}</div>
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

function renderStreamingPreviewIntoHost(host, text) {
    if (!host) return;
    const normalized = String(text || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    host.dataset.markdownSource = normalized;
    host.classList.toggle('is-stream-preview', !!normalized.trim());
    host.textContent = normalized;
}

function getMarkdownRenderMode() {
    const mode = String(config.markdownRenderMode || 'balanced').trim();
    if (mode === 'fast' || mode === 'final') return mode;
    return 'balanced';
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
    const hljs = window.hljs;
    if (!hljs?.highlightElement) return;
    container.querySelectorAll('pre code').forEach((block) => {
        hljs.highlightElement(block);
        block.dataset.highlighted = 'true';
    });
}

function renderMathInHost(host) {
    if (!host) return;

    const mathJax = window.MathJax;
    if (mathJax?.typesetPromise) {
        try {
            if (typeof mathJax.typesetClear === 'function') {
                mathJax.typesetClear([host]);
            }
            mathJax.typesetPromise([host]).catch((error) => {
                console.warn('[Math] MathJax typeset failed, falling back to KaTeX', error);
                renderMathWithKatex(host);
            });
            return;
        } catch (error) {
            console.warn('[Math] MathJax render failed, falling back to KaTeX', error);
        }
    }

    renderMathWithKatex(host);
}

function renderMathWithKatex(host) {
    if (!host || typeof window.renderMathInElement !== 'function') return;
    try {
        window.renderMathInElement(host, {
            throwOnError: false,
            strict: 'ignore',
            delimiters: [
                { left: '$$', right: '$$', display: true },
                { left: '\\[', right: '\\]', display: true },
                { left: '$', right: '$', display: false },
                { left: '\\(', right: '\\)', display: false }
            ]
        });
    } catch (error) {
        console.warn('[Math] KaTeX fallback failed', error);
    }
}

function buildRenderedMarkdownState(markdownText) {
    const normalized = normalizeMarkdownForRender(markdownText || '');
    const renderer = getMarkdownRenderer();
    const html = normalized.trim()
        ? sanitizeRenderedMarkdownHtml(renderer.render(normalized))
        : '';

    return { normalized, renderer, html };
}

function renderMarkdownIntoHost(host, markdownText, options = {}) {
    if (!host) return;
    const allowLooseFallback = options.allowLooseFallback !== false;
    const rendered = buildRenderedMarkdownState(markdownText || '');
    host.dataset.markdownSource = rendered.normalized;
    host.innerHTML = rendered.html;
    if (allowLooseFallback && shouldFallbackToLooseMarkdown(host, rendered.normalized)) {
        host.innerHTML = sanitizeRenderedMarkdownHtml(renderLooseMarkdownToHtml(rendered.normalized));
    }
    if (rendered.renderer.name !== 'remark') {
        renderMathInHost(host);
    }
    host.querySelectorAll('a').forEach((link) => {
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
    });
    highlightMarkdownBlocks(host);
}

function pulseMessageRender(el) {
    if (!el) return;
    if (getMarkdownRenderMode() !== 'fast') return;
    const targets = [el, el.querySelector('.markdown-body')].filter(Boolean);
    targets.forEach((target) => {
        target.classList.remove('is-stream-updated');
        void target.offsetWidth;
        target.classList.add('is-stream-updated');
    });
}

function sanitizeAssistantRenderText(text) {
    let cleanText = stripHiddenAssistantProtocolText(text);
    cleanText = cleanText.replace(/<commentary[\s\S]*?>/gi, '');
    cleanText = cleanText.replace(/commentary to=[a-z_]+(\s+(json|code|text))?/gi, '');
    cleanText = cleanText.trim().replace(/^(json|code|text)\s*/gi, '');
    cleanText = deduplicateTrailingParagraph(cleanText);
    return cleanText;
}

function stripHiddenAssistantProtocolText(text) {
    let cleanText = String(text || '');

    cleanText = cleanText.replace(/<think>[\s\S]*?<\/think>/g, '');
    cleanText = cleanText.replace(/<\|channel\|>analysis[\s\S]*?(?=<\|channel\|>final|$)/g, '');
    cleanText = cleanText.replace(/<\|channel\|>(analysis|final|message)/g, '');
    cleanText = cleanText.replace(/<\|end\|>/g, '');
    cleanText = cleanText.replace(/\{"name"\s*:\s*"[^"]+"\s*,\s*"arguments"\s*:\s*\{[^}]*\}\}/g, '');
    cleanText = cleanText.replace(/\{"name"\s*:\s*"[^"]+"[^}]*\}/g, '');
    cleanText = cleanText.replace(/<\|[\s\S]*?\|>/g, '');

    if (cleanText.includes('</think>')) {
        cleanText = cleanText.split('</think>').pop().trim();
    }
    if (cleanText.includes('<think>')) {
        cleanText = cleanText.split('<think>')[0];
    }

    return cleanText;
}

function detachCurrentAudioPlaybackListeners() {
    return ttsController.detachCurrentAudioPlaybackListeners();
}

function appendStreamChunkDedup(existingText, nextChunk) {
    return chatStreamingController.appendStreamChunkDedup(existingText, nextChunk);
}

function deduplicateTrailingParagraph(text) {
    return chatStreamingController.deduplicateTrailingParagraph(text);
}

function normalizeComparableTail(text) {
    return chatStreamingController.normalizeComparableTail(text);
}

function deduplicateCommittedPending(committedText, pendingText) {
    return chatStreamingController.deduplicateCommittedPending(committedText, pendingText);
}

function finalizeMessageContent(id, text) {
    return chatStreamingController.finalizeMessageContent(id, text);
}

function updateSyncedMessageContent(id, text, options = {}) {
    return chatStreamingController.updateSyncedMessageContent(id, text, options);
}

function getSpeakableTextFromMarkdownHost(host) {
    return chatStreamingController.getSpeakableTextFromMarkdownHost(host);
}

function schedulePendingMarkdownRender(el, pendingHost, pendingText) {
    return chatStreamingController.schedulePendingMarkdownRender(el, pendingHost, pendingText);
}

function flushStreamMessageRender(id) {
    return chatStreamingController.flushStreamMessageRender(id);
}

function scheduleStreamMessageRender(id, text) {
    return chatStreamingController.scheduleStreamMessageRender(id, text);
}

function updateMessageContent(id, text) {
    return chatStreamingController.updateMessageContent(id, text);
}


function isChatNearBottom() {
    if (!chatUIController) return true;
    return chatUIController.isChatNearBottom();
}

function hasLongScrollableChat() {
    if (!chatUIController) return false;
    return chatUIController.hasLongScrollableChat();
}

function updateScrollToBottomButton() {
    if (AppState.session.isRestoring || !chatUIController) return;
    return chatUIController.updateScrollToBottomButton();
}

function jumpToLatestMessages() {
    if (!chatUIController) return;
    return chatUIController.jumpToLatestMessages();
}

function scrollToBottom(force = false) {
    if (!chatUIController) return;
    return chatUIController.scrollToBottom(force);
}

function holdAutoScrollAtBottom(durationMs = 700) {
    return chatUIController.holdAutoScrollAtBottom(durationMs);
}

function scheduleChatScrollToBottom() {
    return chatUIController.scheduleChatScrollToBottom();
}

function observeAutoScrollResizes(elements) {
    return chatUIController.observeAutoScrollResizes(elements);
}

function scrollActiveMessageIntoView() {
    return chatUIController.scrollActiveMessageIntoView();
}

function pinActiveMessageLabelToTop() {
    return chatUIController.pinActiveMessageLabelToTop();
}

function checkAndTriggerLabelPin() {
    return chatUIController.checkAndTriggerLabelPin();
}

function pinTurnToTop(turnId) {
    return chatUIController.pinTurnToTop(turnId);
}

function startStreamingMessageAutoScroll(messageId) {
    return chatUIController.startStreamingMessageAutoScroll(messageId);
}

function stopStreamingMessageAutoScroll() {
    return chatUIController.stopStreamingMessageAutoScroll();
}

// TTS: Speak a message using the Go server's /api/tts endpoint
async function speakMessage(text, btn = null) {
    if (btn) {
        triggerHaptic('nudge');
    }
    return ttsController.speakMessage(text, btn);
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
    return ttsController.cleanTextForTTS(text);
}

/**
 * Initialize streaming TTS for a new message
 */
function initStreamingTTS(elementId) {
    return ttsController.initStreamingTTS(elementId);
}

function getStreamingChunkTargets() {
    return ttsController.getStreamingChunkTargets();
}

function detectStreamingBoundary(newText) {
    return ttsController.detectStreamingBoundary(newText);
}

function shouldCommitStreamingBoundary(length, boundaryKind, hasQueuedAudio) {
    return ttsController.shouldCommitStreamingBoundary(length, boundaryKind, hasQueuedAudio);
}

function splitTTSParagraphByPriority(text, maxChunkSize, minChunkLength, force = false) {
    return ttsController.splitTTSParagraphByPriority(text, maxChunkSize, minChunkLength, force);
}

/**
 * Feed new display text to the streaming TTS processor
 * This is called every time the LLM emits new tokens
 */
function feedStreamingTTS(displayText) {
    return ttsController.feedStreamingTTS(displayText);
}


/**
 * Finalize streaming TTS when LLM stream ends
 * Commits any remaining uncommitted text
 */
function finalizeStreamingTTS(finalDisplayText) {
    return ttsController.finalizeStreamingTTS(finalDisplayText);
}

/**
 * Push a text segment to the TTS queue and ensure processing is running
 * @param {string} text - Text to speak
 * @param {boolean} force - If true, ignores MIN_CHUNK_LENGTH check (use for final chunk)
 */
function pushToStreamingTTSQueue(text, force = false) {
    return ttsController.pushToStreamingTTSQueue(text, force);
}

// ============================================================================
// Global TTS Audio Cache and Prefetch System
// ============================================================================
const ttsAudioCache = new Map(); // text -> Promise<url>

function firstChunkPlayedInCurrentSession() {
    return ttsController.firstChunkPlayedInCurrentSession();
}

async function processOSTTSQueue() {
    return ttsController.processOSTTSQueue();
}

/**
 * Prefetch audio for a given text chunk
 * Can be called anytime - will use cached promise if already fetching/fetched
 */
function prefetchTTSAudio(text) {
    return ttsController.prefetchTTSAudio(text);
}

/**
 * Clear the audio cache (called on stopAllAudio)
 */
function clearTTSAudioCache() {
    return ttsController.clearTTSAudioCache();
}

async function processTTSQueue(isFirstChunk = false) {
    return ttsController.processTTSQueue(isFirstChunk);
}

/**
 * Reset TTS UI state after playback completes
 */
function endTTS(btn, sessionId) {
    return ttsController.endTTS(btn, sessionId);
}

async function checkSystemHealth() {
    let health;

    // 1. Try Wails (Desktop)
    if (typeof window.go !== 'undefined' && window.go.main && window.go.core.App) {
        try {
            health = await window.go.core.App.CheckHealth();
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
        await ensureLastSessionCacheLoaded(true);
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

        const shouldShowStartupCard = !hasSubstantiveChatMessages();
        if (shouldShowStartupCard) {
            const healthMsg = {
                role: 'assistant',
                startup: {
                    title: issues.length === 0 ? t('chat.startup.welcomeTitle') : t('health.checkRequired'),
                    body: issues.length === 0 ? t('chat.startup.welcomeBody') : t('chat.startup.issueBody'),
                    issues,
                    showRestoreButton: !!AppState.session.lastCache,
                    restoreLabel: t('chat.startup.restore')
                }
            };

            appendMessage(healthMsg);
        }

    } catch (e) {
        console.error("Health check rendering failed:", e);
    }
}

/**
 * Updates UI layout based on mic layout setting
 */
function updateMicLayout() {
    micController.updateLayout();
    updateInlineComposerActionVisibility();
}

/**
 * Toggles Speech-to-Text (STT) AppState.input.recognition
 */
function toggleSTT() {
    triggerHaptic('nudge');
    // 1. If generating response, stop it (Mic acts as stop button)
    if (AppState.chat.isGenerating) {
        return stopGeneration();
    }

    // 2. If TTS is currently playing, stop it (interruption support)
    if (AppState.audio.isPlayingQueue) {
        stopAllAudio();
    }

    // 3. STT Logic
    return micController.toggle();
}

function startSTT() {
    return micController.start();
}

function stopSTT(options = {}) {
    return micController.stop(options);
}

function updateSTTButtonState() {
    // Legacy wrapper if still called elsewhere
}

function updateMicUIForGeneration(generating) {
    // Legacy wrapper if still called elsewhere
    return micController.updateLayout();
}

function updateMessageInputPlaceholder() {
    if (!messageInput) return;

    const backgroundTask = getActiveComposerBackgroundTask();
    const nextPlaceholder = AppState.input.isSTTActive
        ? `${t('input.listening')}...`
        : AppState.session.isRestoring
            ? t('input.placeholder.restoring')
            : (AppState.ui.progress.active
                ? [AppState.ui.progress.label, AppState.ui.progress.percent].filter(Boolean).join(' - ')
                : (AppState.chat.passive.placeholder || backgroundTask?.label || t('input.placeholder')));

    messageInput.placeholder = nextPlaceholder;
    messageInput.classList.toggle('stt-listening', AppState.input.isSTTActive);
}
