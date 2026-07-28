/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTTTS(global) {
    function createTTSController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const {
            osTTSVoiceSelect = null
        } = refs;
        const {
            getActiveStreamingMessageId,
            config,
            escapeAttr,
            escapeHtml,
            getCachedAudioPromise,
            getSpeakableTextFromMarkdownHost,
            getPlaybackState,
            getAudioCache,
            getToastBottomOffset,
            onDetachCurrentAudioPlaybackListeners,
            onProcessQueue,
            onCombinedQueueConsumed,
            onSetAssistantActionBarReady,
            onSyncCurrentAudioButtonUI,
            onSyncWakeLock,
            setPlaybackState,
            t
        } = deps;

        let osTTSVoices = [];
        let osTTSVoicesReady = false;
        let audioContextUnlocked = false;
        let audioCtx = null;
        let synthesisTail = Promise.resolve();
        let onDeviceModulePromise = null;
        let lastOnDeviceProgressAt = 0;
        let activeTTSFetch = null;
        let ttsRequestSequence = 0;
        const TTS_INTER_CHUNK_PAUSE_MS = 300;
        const ttsClientId = global.crypto?.randomUUID?.()
            || `tts-${Date.now()}-${Math.random().toString(36).slice(2)}`;

        function normalizeTTSSynthesisSpeed(speed) {
            return Math.max(0.7, Math.min(2.0, Number(speed) || 0.9));
        }

        function getConfiguredTTSSynthesisSpeed() {
            return normalizeTTSSynthesisSpeed(config.ttsSpeed);
        }

        function abortActiveTTSFetch() {
            activeTTSFetch?.controller?.abort?.();
            activeTTSFetch = null;
        }

        function getTTSFetchController(sessionId) {
            if (!activeTTSFetch
                || activeTTSFetch.sessionId !== sessionId
                || activeTTSFetch.controller.signal.aborted) {
                abortActiveTTSFetch();
                activeTTSFetch = { sessionId, controller: new AbortController() };
            }
            return activeTTSFetch.controller;
        }

        function reportOnDeviceProgress(message) {
            console.log(`[On-device TTS] ${message}`);
            const now = Date.now();
            if (now - lastOnDeviceProgressAt < 1200) return;
            lastOnDeviceProgressAt = now;
            global.showToast?.(message);
        }

        function getOnDeviceModule() {
            if (!onDeviceModulePromise) {
                onDeviceModulePromise = import('./app-tts-ondevice.mjs?v=4').catch((error) => {
                    onDeviceModulePromise = null;
                    throw error;
                });
            }
            return onDeviceModulePromise;
        }

        function getTTSChunkLimits() {
            const safetyLimit = 1000;
            const configured = Math.max(parseInt(config.chunkSize) || 300, 50);
            const chunkLimit = Math.min(configured, safetyLimit);
            return {
                first: chunkLimit,
                subsequent: chunkLimit,
                safety: safetyLimit
            };
        }

        function buildTTSAudioCacheKey(text, synthesisSpeed = getConfiguredTTSSynthesisSpeed()) {
            return [
                config.ttsEngine,
                config.ttsVoice,
                config.ttsLang,
                normalizeTTSSynthesisSpeed(synthesisSpeed),
                config.ttsSteps,
                config.chunkSize,
                config.ttsFormat,
                text
            ].join(':');
        }

        function updateMediaSessionMetadata(text) {
            if (!('mediaSession' in navigator)) return;
            try {
                navigator.mediaSession.metadata = new MediaMetadata({
                    title: 'DINKIssTyle Chat TTS',
                    artist: 'TTS Playback',
                    album: text && text.length > 60 ? text.substring(0, 60) + '...' : text || 'Audio'
                });
                navigator.mediaSession.playbackState = 'playing';
            } catch (error) {
                console.warn('[MediaSession] Metadata update failed:', error);
            }
        }

        function clearMediaSessionMetadata() {
            if (!('mediaSession' in navigator)) return;
            navigator.mediaSession.playbackState = 'none';
            navigator.mediaSession.metadata = null;
        }

        function readWavHeader(view) {
            if (!view || view.byteLength < 44) return null;
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
            let dataLength = 0;

            while (offset + 8 <= view.byteLength) {
                const chunkId = readTag(offset);
                const chunkSize = view.getUint32(offset + 4, true);
                const chunkDataOffset = offset + 8;

                if (chunkId === 'fmt ') {
                    fmt = {
                        audioFormat: view.getUint16(chunkDataOffset, true),
                        numChannels: view.getUint16(chunkDataOffset + 2, true),
                        sampleRate: view.getUint32(chunkDataOffset + 4, true),
                        byteRate: view.getUint32(chunkDataOffset + 8, true),
                        blockAlign: view.getUint16(chunkDataOffset + 12, true),
                        bitsPerSample: view.getUint16(chunkDataOffset + 14, true)
                    };
                } else if (chunkId === 'data') {
                    dataOffset = chunkDataOffset;
                    dataLength = chunkSize;
                    break;
                }

                offset = chunkDataOffset + chunkSize + (chunkSize % 2);
            }

            if (!fmt || dataOffset < 0 || dataOffset + dataLength > view.byteLength) return null;
            return {
                ...fmt,
                dataOffset,
                dataLength
            };
        }

        function concatenateWavArrayBuffers(buffers, silenceDurationMs = 0) {
            if (!Array.isArray(buffers) || buffers.length === 0) return null;
            if (buffers.length === 1) return buffers[0];

            const views = buffers.map((buffer) => new DataView(buffer));
            const headers = views.map((view) => readWavHeader(view));
            const firstHeader = headers[0];
            if (!firstHeader || headers.some((header) => !header
                || header.audioFormat !== firstHeader.audioFormat
                || header.numChannels !== firstHeader.numChannels
                || header.sampleRate !== firstHeader.sampleRate
                || header.bitsPerSample !== firstHeader.bitsPerSample)) {
                return null;
            }

            const silenceFrameCount = Math.max(
                0,
                Math.round(firstHeader.sampleRate * (Number(silenceDurationMs) || 0) / 1000)
            );
            const silenceByteLength = silenceFrameCount * firstHeader.blockAlign;
            const totalDataLength = headers.reduce((sum, header) => sum + header.dataLength, 0)
                + (silenceByteLength * (buffers.length - 1));
            const totalSize = 44 + totalDataLength;
            const merged = new ArrayBuffer(totalSize);
            const view = new DataView(merged);
            const bytes = new Uint8Array(merged);

            const writeTag = (offset, tag) => {
                for (let i = 0; i < tag.length; i += 1) {
                    view.setUint8(offset + i, tag.charCodeAt(i));
                }
            };

            writeTag(0, 'RIFF');
            view.setUint32(4, totalSize - 8, true);
            writeTag(8, 'WAVE');
            writeTag(12, 'fmt ');
            view.setUint32(16, 16, true);
            view.setUint16(20, firstHeader.audioFormat, true);
            view.setUint16(22, firstHeader.numChannels, true);
            view.setUint32(24, firstHeader.sampleRate, true);
            view.setUint32(28, firstHeader.byteRate, true);
            view.setUint16(32, firstHeader.blockAlign, true);
            view.setUint16(34, firstHeader.bitsPerSample, true);
            writeTag(36, 'data');
            view.setUint32(40, totalDataLength, true);

            let writeOffset = 44;
            buffers.forEach((buffer, index) => {
                const header = headers[index];
                const source = new Uint8Array(buffer, header.dataOffset, header.dataLength);
                bytes.set(source, writeOffset);
                writeOffset += header.dataLength;
                if (index < buffers.length - 1) {
                    writeOffset += silenceByteLength;
                }
            });

            return merged;
        }

        async function promiseWithTimeout(promise, timeoutMs) {
            let timeoutId = null;
            try {
                return await Promise.race([
                    promise,
                    new Promise((resolve) => {
                        timeoutId = global.setTimeout(() => resolve(null), timeoutMs);
                    })
                ]);
            } finally {
                if (timeoutId) global.clearTimeout(timeoutId);
            }
        }

        async function combinePlayableChunks(primaryUrl, queuedTexts, synthesisSpeed = getConfiguredTTSSynthesisSpeed()) {
            if (!primaryUrl || !queuedTexts || queuedTexts.length === 0) {
                return { url: primaryUrl, revokeInputs: null };
            }

            if ((config.ttsFormat || 'wav') !== 'wav') {
                return { url: primaryUrl, revokeInputs: null };
            }

            const urls = [primaryUrl];
            const consumedTexts = [];
            try {
                for (const text of queuedTexts.slice(0, 2)) {
                    const cachedPromise = getCachedAudioPromise?.(buildTTSAudioCacheKey(text, synthesisSpeed));
                    if (!cachedPromise) break;

                    const nextUrl = await promiseWithTimeout(cachedPromise, 120);
                    if (!nextUrl) break;

                    urls.push(nextUrl);
                    consumedTexts.push(text);
                }

                if (urls.length === 1) {
                    return { url: primaryUrl, revokeInputs: null };
                }

                const buffers = await Promise.all(urls.map(async (url) => {
                    const response = await fetch(url);
                    return await response.arrayBuffer();
                }));
                const mergedBuffer = concatenateWavArrayBuffers(buffers, TTS_INTER_CHUNK_PAUSE_MS);
                if (!mergedBuffer) {
                    return { url: primaryUrl, revokeInputs: null };
                }

                if (consumedTexts.length > 0) {
                    onCombinedQueueConsumed?.(consumedTexts);
                    const cache = getAudioCache?.();
                    consumedTexts.forEach((text) => cache?.delete(buildTTSAudioCacheKey(text, synthesisSpeed)));
                }

                return {
                    url: URL.createObjectURL(new Blob([mergedBuffer], { type: 'audio/wav' })),
                    revokeInputs: urls
                };
            } catch (error) {
                console.error('[TTS] Failed to combine WAV chunks:', error);
                return { url: primaryUrl, revokeInputs: null };
            }
        }

        async function loadVoiceStyles() {
            const voiceSelect = global.document.getElementById('cfg-tts-voice');
            if (!voiceSelect) return;

            try {
                const response = await fetch('/api/tts/styles', {
                    credentials: 'include'
                });
                if (response.status === 404) {
                    // Some deployments don't expose server-side voice presets.
                    // Keep the existing select options and fail quietly.
                    return;
                }
                if (!response.ok) throw new Error(await response.text());

                const voices = await response.json();
                voiceSelect.innerHTML = '';
                voices.forEach((voice) => {
                    const voiceId = String(typeof voice === 'string' ? voice : (voice?.id || '')).replace(/\.json$/i, '');
                    if (!voiceId) return;
                    const option = global.document.createElement('option');
                    option.value = voiceId;
                    option.textContent = typeof voice === 'string'
                        ? voiceId
                        : (voice?.name || voiceId);
                    voiceSelect.appendChild(option);
                });

                if (config.ttsVoice) {
                    voiceSelect.value = String(config.ttsVoice).replace(/\.json$/i, '');
                }
            } catch (error) {
                console.warn('[TTS] Failed to load voice styles:', error);
            }
        }

        function cleanTextForTTS(text) {
            if (!text) return '';

            let cleaned = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');

            cleaned = cleaned.replace(/<span class="tool-status"[\s\S]*?<\/span>/g, '');
            cleaned = cleaned.replace(/Tool Call:.*?(?:[.!?\n]|$)+/gi, '');
            // Strip think blocks
            if (cleaned.includes('</think>')) {
                cleaned = cleaned.split('</think>').pop();
            }
            if (cleaned.includes('<think>')) {
                cleaned = cleaned.split('<think>')[0];
            }

            // Strip Gemma 4 reasoning channels
            const gemmaStartRegex = /<\|channel(?:\||>)?\s*thought/i;
            const gemmaEndRegex = /<channel\|>|<\/channel\|>|<\|channel(?:\||>)?\s*(?:message|final|instruction|response)/i;

            if (gemmaStartRegex.test(cleaned)) {
                const startMatch = cleaned.match(gemmaStartRegex);
                const startIndex = startMatch.index;
                const remaining = cleaned.slice(startIndex + startMatch[0].length);
                const endMatch = remaining.match(gemmaEndRegex);
                if (endMatch) {
                    const endIndex = startIndex + startMatch[0].length + endMatch.index + endMatch[0].length;
                    cleaned = cleaned.slice(0, startIndex) + cleaned.slice(endIndex);
                } else {
                    cleaned = cleaned.slice(0, startIndex);
                }
            }

            cleaned = cleaned.replace(/<\|channel(?:\||>)?\s*thought/gi, '');
            cleaned = cleaned.replace(/<channel\|>/gi, '');
            cleaned = cleaned.replace(/<\/channel\|>/gi, '');
            cleaned = cleaned.replace(/<think>[\s\S]*?<\/think>/g, '');
            cleaned = cleaned.replace(/<[^>]*>/g, '');

            if (typeof global.ttsDictionaryRegex !== 'undefined' && global.ttsDictionaryRegex) {
                cleaned = cleaned.replace(global.ttsDictionaryRegex, (match) => {
                    return global.ttsDictionary[match.toLowerCase()] || match;
                });
            }

            cleaned = cleaned.replace(/https?:\/\/[^\s]+/g, '');
            cleaned = cleaned.replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1');
            cleaned = cleaned.replace(/!\[([^\]]*)\]\([^\)]+\)/g, '$1');

            cleaned = cleaned.replace(/```[\s\S]*?```/g, '');
            cleaned = cleaned.replace(/`[^`]+`/g, '');
            cleaned = cleaned.replace(/`/g, '');

            cleaned = cleaned.replace(/^(#{1,6})\s+(.+?)([.!?]?)$/gm, (_, hashes, title, punct) => {
                const level = hashes.length;
                const suffix = punct || '.';
                const pauseBreak = level <= 2 ? '\n\n' : '\n';
                return `${title}${suffix}${pauseBreak}`;
            });

            cleaned = cleaned.replace(/(\*\*|__)(.*?)\1/g, '$2');
            cleaned = cleaned.replace(/(\*|_)(.*?)\1/g, '$2');

            cleaned = cleaned.replace(/^>\s+/gm, '');
            cleaned = cleaned.replace(/^([-*_]){3,}\s*$/gm, '\n\n');
            cleaned = cleaned.replace(/^\s*[-*+]\s+(.+)$/gm, '\n$1.\n');
            cleaned = cleaned.replace(/^\s*(\d+)[\.\)]\s+(.+)$/gm, '\n$1. $2.\n');

            const symbolRegex = /[\u{1F600}-\u{1F64F}\u{1F300}-\u{1F5FF}\u{1F680}-\u{1F6FF}\u{1F1E0}-\u{1F1FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{FE00}-\u{FE0F}\u{1F900}-\u{1F9FF}\u{1FA00}-\u{1FA6F}\u{1FA70}-\u{1FAFF}\u{2300}-\u{23FF}\u{25A0}-\u{25FF}\u{2B00}-\u{2BFF}\u{2190}-\u{21FF}\u{2900}-\u{297F}\u{3290}-\u{329F}\u{3030}\u{303D}]/gu;
            cleaned = cleaned.replace(symbolRegex, '');

            cleaned = cleaned.replace(/[«»""„‚]/g, ' ');
            cleaned = cleaned.replace(/[=→—–]/g, ', ');
            cleaned = cleaned.replace(/\s*[-•◦▪▸►]\s*/g, ', ');
            cleaned = cleaned.replace(/\.{3,}/g, '.');
            cleaned = cleaned.replace(/[*~|]/g, '');
            cleaned = cleaned.replace(/([.!?])(?=[^ \n])/g, '$1 ');
            cleaned = cleaned.replace(/([^\s.!?])\n/g, '$1.\n');
            cleaned = cleaned.replace(/\n([^\s])/g, '\n$1');
            cleaned = cleaned.replace(/\n{4,}/g, '\n\n\n');
            cleaned = cleaned.replace(/[ \t]+/g, ' ');
            cleaned = cleaned.replace(/^\s+|\s+$/gm, '');

            return cleaned.trim();
        }

        function getStreamingChunkTargets() {
            const limits = getTTSChunkLimits();
            const baseTarget = limits.subsequent;
            return {
                firstChunkTarget: baseTarget,
                weakBoundaryTarget: baseTarget,
                strongBoundaryTarget: baseTarget,
                hardCeiling: Math.min(baseTarget, limits.safety)
            };
        }

        function detectStreamingBoundary(newText) {
            const match = String(newText || '').match(/^([\s\S]*?\n\s*\n+)/);
            if (match && match[1] && match[1].trim()) {
                return { text: match[1], kind: 'paragraph' };
            }
            return null;
        }

        function shouldCommitStreamingBoundary(length, boundaryKind, hasQueuedAudio) {
            void hasQueuedAudio;
            return boundaryKind === 'paragraph' && length > 0;
        }

        function splitTTSParagraphByPriority(text, maxChunkSize, minChunkLength, force = false) {
            const chunks = [];
            const limit = Math.max(parseInt(maxChunkSize) || 300, 1);
            const paragraphs = String(text || '')
                .split(/\n\s*\n+/)
                .map((paragraph) => paragraph.trim())
                .filter(Boolean);

            for (const paragraph of paragraphs) {
                let remaining = paragraph;
                while (remaining) {
                    if (remaining.length <= limit) {
                        if (/[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(remaining)) {
                            chunks.push(remaining.trim());
                        }
                        break;
                    }

                    let splitAt = limit;
                    for (let index = limit; index > 0; index -= 1) {
                        if (/\s/.test(remaining.charAt(index))) {
                            splitAt = index;
                            break;
                        }
                    }

                    if (!force && splitAt < minChunkLength) {
                        splitAt = limit;
                    }

                    const chunk = remaining.slice(0, splitAt).trim();
                    if (chunk && /[a-zA-Z가-힣ㄱ-ㅎㅏ-ㅣ0-9]/.test(chunk)) {
                        chunks.push(chunk);
                    }
                    remaining = remaining.slice(splitAt).trimStart();
                }
            }

            return chunks;
        }

        function clearTTSAudioCache() {
            const cache = getAudioCache?.();
            if (!cache) return;
            cache.forEach(async (promise) => {
                const url = await promise;
                if (url) {
                    global.URL.revokeObjectURL(url);
                }
            });
            cache.clear();
        }

        function stopAllAudio() {
            const state = getPlaybackState?.() || {};
            const currentAudio = state.currentAudio || null;

            abortActiveTTSFetch();

            setPlaybackState?.({
                ttsQueue: [],
                audioWarmup: null
            });

            if (supportsOSTTS()) {
                try {
                    global.speechSynthesis.cancel();
                } catch (_) {
                    // Ignore OS TTS cancellation errors
                }
            }

            if (currentAudio) {
                try {
                    onDetachCurrentAudioPlaybackListeners?.();
                    currentAudio.pause();
                    currentAudio.src = '';
                    currentAudio.load();
                } catch (_) {
                    // Ignore audio stop errors
                }
            }

            clearTTSAudioCache();
            clearMediaSessionMetadata();

            setPlaybackState?.({
                isPlayingQueue: false,
                streamingTTSActive: false,
                streamingTTSBuffer: '',
                streamingTTSCommittedIndex: 0,
                ttsSynthesisSpeed: 0.9,
                activeTTSSessionLabel: '',
                currentAudioBtn: null,
                currentAudioPlaybackController: null
            });

            onSyncWakeLock?.();

            const nextSessionId = Number(state.ttsSessionId || 0) + 1;
            setPlaybackState?.({
                ttsSessionId: nextSessionId
            });
        }

        function syncCurrentAudioButtonUI() {
            const state = getPlaybackState?.() || {};
            const activeBtn = state.currentAudioBtn;
            
            // Find all toggle-able audio buttons in the chat
            const allBtns = global.document.querySelectorAll('.message-actions .icon-btn .material-icons-round');
            
            allBtns.forEach(iconEl => {
                const btn = iconEl.closest('button');
                if (!btn) return;
                
                if (activeBtn && btn === activeBtn) {
                    const queue = Array.isArray(state.ttsQueue) ? state.ttsQueue : [];
                    if (state.isPlayingQueue) {
                        iconEl.textContent = 'stop';
                        btn.title = 'Stop';
                        btn.disabled = false;
                    } else if (state.streamingTTSActive || queue.length > 0) {
                        iconEl.textContent = 'hourglass_empty';
                        btn.title = 'Preparing audio';
                        btn.disabled = true;
                    } else {
                        iconEl.textContent = 'volume_up';
                        btn.title = 'Speak';
                        btn.disabled = false;
                    }
                } else {
                    // Reset other buttons
                    if (iconEl.textContent === 'stop' || iconEl.textContent === 'hourglass_empty') {
                        iconEl.textContent = 'volume_up';
                        btn.title = 'Speak';
                        btn.disabled = false;
                    }
                }
            });
        }

        function attachStreamingAudioButtonToMessage(msgEl) {
            if (!msgEl || !msgEl.id) return;
            if (msgEl.id !== getActiveStreamingMessageId?.()) return;

            const state = getPlaybackState?.() || {};
            const queue = Array.isArray(state.ttsQueue) ? state.ttsQueue : [];
            if (!(state.streamingTTSActive || state.isPlayingQueue || queue.length > 0)) return;

            const speakBtn = msgEl.querySelector('.speak-btn');
            if (!speakBtn) return;
            setPlaybackState?.({ currentAudioBtn: speakBtn });
            syncCurrentAudioButtonUI();

            const actionBar = msgEl.querySelector('.message-actions');
            if (actionBar && !actionBar.classList.contains('is-ready')) {
                onSetAssistantActionBarReady?.(msgEl.id);
            }
        }

        async function unlockAudioContext() {
            if (!audioCtx) {
                const AudioContextCtor = global.AudioContext || global.webkitAudioContext;
                if (AudioContextCtor) {
                    audioCtx = new AudioContextCtor();
                }
            }

            if (audioCtx && audioCtx.state === 'suspended') {
                try {
                    await audioCtx.resume();
                    const buffer = audioCtx.createBuffer(1, 1, 22050);
                    const source = audioCtx.createBufferSource();
                    source.buffer = buffer;
                    source.connect(audioCtx.destination);
                    source.start(0);
                    audioContextUnlocked = true;
                    console.log('AudioContext unlocked/resumed');
                } catch (e) {
                    console.error('Failed to resume AudioContext', e);
                }
            }

            const state = getPlaybackState?.() || {};
            if (!state.currentAudio) {
                const audio = new global.Audio();
                audio.playsInline = true;
                audio.src = 'data:audio/wav;base64,UklGRigAAABXQVZFRm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQQAAAAAAA==';
                audio.play().catch(() => {});
                setPlaybackState?.({ currentAudio: audio });
            }
        }

        function detachCurrentAudioPlaybackListeners() {
            const state = getPlaybackState?.() || {};
            state.currentAudioPlaybackController?.abort?.();
            setPlaybackState?.({ currentAudioPlaybackController: null });
        }

        async function pauseBeforeNextTTSChunk(sessionId) {
            const state = getPlaybackState?.() || {};
            const queue = Array.isArray(state.ttsQueue) ? state.ttsQueue : [];
            if (sessionId !== state.ttsSessionId
                || (!state.streamingTTSActive && queue.length === 0)) {
                return;
            }
            await new Promise((resolve) => global.setTimeout(resolve, TTS_INTER_CHUNK_PAUSE_MS));
        }

        function prefetchTTSAudio(text, synthesisSpeed = null) {
            if (!text) return null;
            const cache = getAudioCache?.();
            if (!cache) return null;
            const playbackState = getPlaybackState?.() || {};
            const requestedSpeed = normalizeTTSSynthesisSpeed(
                synthesisSpeed ?? playbackState.ttsSynthesisSpeed ?? config.ttsSpeed
            );
            const cacheKey = buildTTSAudioCacheKey(text, requestedSpeed);
            if (cache.has(cacheKey)) return cache.get(cacheKey);
            const sessionAtSchedule = (getPlaybackState?.() || {}).ttsSessionId;

            const runSynthesis = async () => {
                try {
                    if (sessionAtSchedule !== (getPlaybackState?.() || {}).ttsSessionId) return null;
                    const payload = {
                        text,
                        lang: config.ttsLang,
                        chunkSize: parseInt(config.chunkSize) || 300,
                        voiceStyle: config.ttsVoice,
                        speed: requestedSpeed,
                        steps: Math.max(1, Math.min(20, parseInt(config.ttsSteps, 10) || 5)),
                        format: config.ttsFormat || 'wav',
                        prechunked: true
                    };

                    console.log(`[TTS] Prefetching (${config.ttsVoice}): "${text.substring(0, 25)}..."`);
                    if (getCurrentTTSEngine() === 'supertonic-ondevice') {
                        const module = await getOnDeviceModule();
                        const blob = await module.synthesize({
                            text,
                            lang: payload.lang,
                            voice: payload.voiceStyle,
                            speed: payload.speed,
                            steps: payload.steps,
                            onProgress: reportOnDeviceProgress
                        });
                        if (sessionAtSchedule !== (getPlaybackState?.() || {}).ttsSessionId) return null;
                        return global.URL.createObjectURL(blob);
                    }
                    const fetchController = getTTSFetchController(sessionAtSchedule);
                    const requestId = ++ttsRequestSequence;
                    const response = await global.fetch('/api/tts', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                            'X-TTS-Client-ID': ttsClientId,
                            'X-TTS-Session-ID': String(sessionAtSchedule),
                            'X-TTS-Request-ID': String(requestId)
                        },
                        credentials: 'include',
                        signal: fetchController.signal,
                        body: JSON.stringify(payload)
                    });

                    const latestState = getPlaybackState?.() || {};
                    if (sessionAtSchedule !== latestState.ttsSessionId) {
                        console.log('[TTS] Session changed, discarding prefetch');
                        return null;
                    }

                    if (!response.ok) {
                        console.error('[TTS] Chunk failed:', await response.text());
                        return null;
                    }

                    const blob = await response.blob();
                    const url = global.URL.createObjectURL(blob);
                    const timing = response.headers.get('Server-Timing');
                    console.log(`[TTS] Prefetch complete: "${text.substring(0, 25)}..."${timing ? ` (${timing})` : ''}`);
                    return url;
                } catch (e) {
                    if (e?.name === 'AbortError') {
                        console.debug('[TTS] Request aborted because the playback session changed.');
                        return null;
                    }
                    console.error('[TTS] Chunk error:', e);
                    return null;
                }
            };

            // One ONNX inference at a time avoids oversubscribing the CPU with
            // multiple requests that each use their own intra-op thread pool.
            const promise = synthesisTail.catch(() => null).then(runSynthesis);
            synthesisTail = promise.then(() => undefined, () => undefined);

            cache.set(cacheKey, promise);
            return promise;
        }

        function endTTS(btn, sessionId) {
            const state = getPlaybackState?.() || {};
            if (sessionId !== state.ttsSessionId) return;

            if (activeTTSFetch?.sessionId === sessionId) {
                activeTTSFetch = null;
            }

            if (btn) {
                const iconEl = btn.querySelector('.material-icons-round');
                if (iconEl) iconEl.textContent = 'volume_up';
                btn.title = 'Speak';
                btn.disabled = false;
            }

            setPlaybackState?.({
                currentAudioBtn: null,
                isPlayingQueue: false,
                activeTTSSessionLabel: ''
            });
            clearMediaSessionMetadata();
            onSyncWakeLock?.();
        }

        async function speakMessage(text, btn = null) {
            const state = getPlaybackState?.() || {};
            if ((state.isPlayingQueue || state.streamingTTSActive) && btn && btn === state.currentAudioBtn) {
                stopAllAudio();
                return;
            }

            stopAllAudio();

            if (!config.enableTTS) {
                if (!btn) return;
                global.alert?.('TTS is unavailable. Enable "Enable TTS & Vector DB" in Server Manager.');
                return;
            }

            if (getCurrentTTSEngine() === 'os' && !supportsOSTTS()) {
                if (btn) {
                    global.alert?.(t('setting.osVoice.unavailable'));
                }
                return;
            }

            const cleanText = cleanTextForTTS(text);
            if (!cleanText) return;

            if (btn) {
                setPlaybackState?.({ currentAudioBtn: btn });
            }

            setPlaybackState?.({
                activeTTSSessionLabel: cleanText.substring(0, 120) + (cleanText.length > 120 ? '...' : ''),
                ttsSynthesisSpeed: getConfiguredTTSSynthesisSpeed(),
                ttsQueue: []
            });

            const limits = getTTSChunkLimits();
            const nextQueue = splitTTSParagraphByPriority(cleanText, limits.subsequent, 1, true);

            setPlaybackState?.({
                ttsQueue: nextQueue
            });

            onSyncCurrentAudioButtonUI?.();
            if (nextQueue.length > 0) {
                onProcessQueue?.();
            }
        }

        function speakMessageFromBtn(btn) {
            const bubble = btn?.closest('.message-inner')?.querySelector('.markdown-body');
            if (bubble) {
                return speakMessage(getSpeakableTextFromMarkdownHost?.(bubble) || '', btn);
            }
            return null;
        }

        async function processOSTTSQueue() {
            const initialState = getPlaybackState?.() || {};
            if (!supportsOSTTS()) return;
            if (initialState.isPlayingQueue) return;
            if (!Array.isArray(initialState.ttsQueue) || initialState.ttsQueue.length === 0) return;

            setPlaybackState?.({ isPlayingQueue: true });
            onSyncWakeLock?.();

            const btn = initialState.currentAudioBtn;
            const sessionId = initialState.ttsSessionId;
            const mediaSessionLabel = initialState.activeTTSSessionLabel;
            const osRate = Math.max(0.1, Math.min(10.0, Number(config.osTtsRate) || 1.0));
            const osPitch = Math.max(0.0, Math.min(2.0, Number(config.osTtsPitch) || 1.0));
            let firstChunkPlayed = false;

            while (true) {
                const state = getPlaybackState?.() || {};
                if (sessionId !== state.ttsSessionId) break;

                const queue = Array.isArray(state.ttsQueue) ? [...state.ttsQueue] : [];
                const text = queue.shift();
                setPlaybackState?.({ ttsQueue: queue });

                if (!text) {
                    if (state.streamingTTSActive) {
                        await new Promise((resolve) => global.setTimeout(resolve, 100));
                        continue;
                    }
                    break;
                }

                const utterance = new global.SpeechSynthesisUtterance(text);
                const selectedVoice = getSelectedOSTTSVoice();
                if (selectedVoice) {
                    utterance.voice = selectedVoice;
                    utterance.lang = selectedVoice.lang || config.osTtsVoiceLang || config.ttsLang || 'ko';
                } else {
                    utterance.lang = config.osTtsVoiceLang || config.ttsLang || 'ko';
                }
                utterance.rate = osRate;
                utterance.pitch = osPitch;

                try {
                    if (!firstChunkPlayed && mediaSessionLabel) {
                        updateMediaSessionMetadata(mediaSessionLabel);
                    }

                    await new Promise((resolve, reject) => {
                        utterance.onstart = () => {
                            if (!firstChunkPlayed && btn) {
                                firstChunkPlayed = true;
                                onSyncCurrentAudioButtonUI?.();
                            }
                        };
                        utterance.onend = () => resolve();
                        utterance.onerror = (event) => {
                            console.error('[OS TTS] Playback failed:', event);
                            reject(event);
                        };

                        const latestState = getPlaybackState?.() || {};
                        if (sessionId !== latestState.ttsSessionId) {
                            resolve();
                            return;
                        }

                        global.speechSynthesis.speak(utterance);
                    });
                } catch (e) {
                    console.error('[OS TTS] Chunk playback error:', e);
                }

                await pauseBeforeNextTTSChunk(sessionId);
            }

            const finalState = getPlaybackState?.() || {};
            if (sessionId === finalState.ttsSessionId) {
                endTTS(btn, sessionId);
            }
        }

        async function processTTSQueue() {
            const initialState = getPlaybackState?.() || {};
            if (getCurrentTTSEngine() === 'os') {
                return processOSTTSQueue();
            }
            if (!Array.isArray(initialState.ttsQueue) || initialState.ttsQueue.length === 0) return;
            if (initialState.isPlayingQueue) return;

            setPlaybackState?.({ isPlayingQueue: true });
            onSyncWakeLock?.();

            const btn = initialState.currentAudioBtn;
            const sessionId = initialState.ttsSessionId;
            const mediaSessionLabel = initialState.activeTTSSessionLabel;
            const synthesisSpeed = normalizeTTSSynthesisSpeed(initialState.ttsSynthesisSpeed);

            if (btn) {
                onSyncCurrentAudioButtonUI?.();
            }

            let firstChunkPlayed = false;

            const seededState = getPlaybackState?.() || {};
            const seededQueue = Array.isArray(seededState.ttsQueue) ? seededState.ttsQueue : [];
            // Keep only the current chunk and one look-ahead synthesis in flight.
            for (let i = 0; i < Math.min(2, seededQueue.length); i += 1) {
                prefetchTTSAudio(seededQueue[i], synthesisSpeed);
            }

            while (true) {
                const state = getPlaybackState?.() || {};
                if (sessionId !== state.ttsSessionId) break;

                const queue = Array.isArray(state.ttsQueue) ? [...state.ttsQueue] : [];
                const text = queue.shift();
                setPlaybackState?.({ ttsQueue: queue });

                if (!text) {
                    if (state.streamingTTSActive) {
                        await new Promise((resolve) => global.setTimeout(resolve, 100));
                        continue;
                    }
                    break;
                }

                const nextQueue = Array.isArray(getPlaybackState?.().ttsQueue) ? getPlaybackState().ttsQueue : [];
                for (let i = 0; i < Math.min(1, nextQueue.length); i += 1) {
                    prefetchTTSAudio(nextQueue[i], synthesisSpeed);
                }

                let audioUrl = null;
                let playbackBundle = null;
                try {
                    audioUrl = await prefetchTTSAudio(text, synthesisSpeed);
                } catch (e) {
                    console.error('Prefetch failed', e);
                }

                const cacheKey = buildTTSAudioCacheKey(text, synthesisSpeed);
                getAudioCache?.()?.delete(cacheKey);

                if (!audioUrl) {
                    continue;
                }

                const latestState = getPlaybackState?.() || {};
                if (sessionId !== latestState.ttsSessionId) {
                    global.URL.revokeObjectURL(audioUrl);
                    break;
                }

                try {
                    if (!firstChunkPlayed && mediaSessionLabel) {
                        updateMediaSessionMetadata(mediaSessionLabel);
                    }

                    let currentAudio = latestState.currentAudio || null;
                    if (!currentAudio) {
                        currentAudio = new global.Audio();
                        currentAudio.playsInline = true;
                        currentAudio.preload = 'auto';
                        setPlaybackState?.({ currentAudio });
                    }

                    playbackBundle = firstChunkPlayed
                        ? await combinePlayableChunks(audioUrl, [...(latestState.ttsQueue || [])], synthesisSpeed)
                        : { url: audioUrl, revokeInputs: null };
                    const playbackUrl = playbackBundle?.url || audioUrl;

                    if (!firstChunkPlayed) {
                        firstChunkPlayed = true;
                        if (btn) onSyncCurrentAudioButtonUI?.();
                    }

                    await new Promise((resolve, reject) => {
                        onDetachCurrentAudioPlaybackListeners?.();
                        const playbackController = new AbortController();
                        setPlaybackState?.({ currentAudioPlaybackController: playbackController });

                        const activeAudio = (getPlaybackState?.() || {}).currentAudio || currentAudio;
                        const onEnded = () => {
                            const activeState = getPlaybackState?.() || {};
                            if (activeState.currentAudioPlaybackController === playbackController) {
                                setPlaybackState?.({ currentAudioPlaybackController: null });
                            }
                            resolve();
                        };
                        const onError = (e) => {
                            console.error('Audio element error:', e);
                            const activeState = getPlaybackState?.() || {};
                            if (activeState.currentAudioPlaybackController === playbackController) {
                                setPlaybackState?.({ currentAudioPlaybackController: null });
                            }
                            reject(e);
                        };

                        activeAudio.addEventListener('ended', onEnded, { once: true, signal: playbackController.signal });
                        activeAudio.addEventListener('error', onError, { once: true, signal: playbackController.signal });

                        const currentState = getPlaybackState?.() || {};
                        if (sessionId !== currentState.ttsSessionId) {
                            playbackController.abort();
                            if (currentState.currentAudioPlaybackController === playbackController) {
                                setPlaybackState?.({ currentAudioPlaybackController: null });
                            }
                            resolve();
                            return;
                        }

                        activeAudio.src = playbackUrl;
                        activeAudio.defaultPlaybackRate = 1.0;
                        activeAudio.playbackRate = 1.0;
                        activeAudio.play().catch(reject);
                    });
                } catch (e) {
                    console.error('Playback failed for chunk:', e);
                } finally {
                    onDetachCurrentAudioPlaybackListeners?.();
                    if (playbackBundle?.revokeInputs) {
                        for (const url of playbackBundle.revokeInputs) {
                            global.URL.revokeObjectURL(url);
                        }
                    } else if (audioUrl) {
                        global.URL.revokeObjectURL(audioUrl);
                    }

                    if (playbackBundle?.url && playbackBundle.url !== audioUrl) {
                        global.URL.revokeObjectURL(playbackBundle.url);
                    }
                }

                await pauseBeforeNextTTSChunk(sessionId);
            }

            const finalState = getPlaybackState?.() || {};
            if (sessionId === finalState.ttsSessionId) {
                endTTS(btn, sessionId);
            }
        }

        function firstChunkPlayedInCurrentSession() {
            const state = getPlaybackState?.() || {};
            const queue = Array.isArray(state.ttsQueue) ? state.ttsQueue : [];
            return queue.length > 0 || !!state.isPlayingQueue;
        }

        function initStreamingTTS(elementId) {
            stopAllAudio();

            setPlaybackState?.({
                streamingTTSActive: true,
                streamingTTSCommittedIndex: 0,
                streamingTTSBuffer: '',
                ttsSynthesisSpeed: getConfiguredTTSSynthesisSpeed(),
                activeTTSSessionLabel: 'Streaming TTS'
            });

            const msgEl = global.document.getElementById(elementId);
            if (msgEl) {
                setPlaybackState?.({
                    currentAudioBtn: msgEl.querySelector('.speak-btn')
                });
            }
            onSyncCurrentAudioButtonUI?.();
            console.log('[Streaming TTS] Initialized');
        }

        function pushToStreamingTTSQueue(text, force = false) {
            if (!text || !text.trim()) return;

            const state = getPlaybackState?.() || {};
            const queue = Array.isArray(state.ttsQueue) ? [...state.ttsQueue] : [];
            const hasQueuedAudio = queue.length > 0 || !!state.isPlayingQueue;
            const minChunkLength = hasQueuedAudio ? 40 : 18;
            const maxChunkSize = getTTSChunkLimits().subsequent;
            const chunks = splitTTSParagraphByPriority(text, maxChunkSize, minChunkLength, force);
            for (const chunk of chunks) {
                queue.push(chunk);
            }

            setPlaybackState?.({ ttsQueue: queue });

            if (!state.isPlayingQueue && queue.length > 0) {
                onProcessQueue?.();
            }
        }

        function feedStreamingTTS(displayText) {
            const initialState = getPlaybackState?.() || {};
            if (!initialState.streamingTTSActive) return;

            let iterations = 0;
            const maxIterations = 20;

            while (iterations < maxIterations) {
                iterations += 1;

                const state = getPlaybackState?.() || {};
                const newText = String(displayText || '').substring(Number(state.streamingTTSCommittedIndex || 0));
                if (!newText || newText.length < 3) break;

                let committed = null;
                let advanceBy = 0;
                const hasQueuedAudio = firstChunkPlayedInCurrentSession();
                const targets = getStreamingChunkTargets();
                let nextBuffer = String(state.streamingTTSBuffer || '');
                let nextCommittedIndex = Number(state.streamingTTSCommittedIndex || 0);

                const codeBlockMatch = newText.match(/(.*?)```[\s\S]*?```/);
                if (codeBlockMatch) {
                    const textBefore = codeBlockMatch[1];
                    const fullMatch = codeBlockMatch[0];

                    if (textBefore.trim()) {
                        const cleanedBefore = cleanTextForTTS(textBefore);
                        if (cleanedBefore.trim()) {
                            committed = nextBuffer + cleanedBefore;
                            nextBuffer = '';
                        }
                    }

                    if (committed) {
                        advanceBy = fullMatch.length;
                    } else {
                        nextCommittedIndex += fullMatch.length;
                        if (nextBuffer.trim()) {
                            const toSpeak = nextBuffer;
                            nextBuffer = '';
                            setPlaybackState?.({
                                streamingTTSBuffer: nextBuffer,
                                streamingTTSCommittedIndex: nextCommittedIndex
                            });
                            pushToStreamingTTSQueue(toSpeak, true);
                        } else {
                            setPlaybackState?.({
                                streamingTTSCommittedIndex: nextCommittedIndex
                            });
                        }
                        continue;
                    }
                }

                if (!committed) {
                    const boundary = detectStreamingBoundary(newText);
                    if (boundary) {
                        const potentialCommit = nextBuffer + cleanTextForTTS(boundary.text);
                        if (shouldCommitStreamingBoundary(potentialCommit.length, boundary.kind, hasQueuedAudio)) {
                            committed = potentialCommit;
                            nextBuffer = '';
                            advanceBy = boundary.text.length;
                        } else {
                            nextBuffer = `${potentialCommit} `;
                            nextCommittedIndex += boundary.text.length;
                            setPlaybackState?.({
                                streamingTTSBuffer: nextBuffer,
                                streamingTTSCommittedIndex: nextCommittedIndex
                            });
                            continue;
                        }
                    }
                }

                if (!committed && (nextBuffer.length + cleanTextForTTS(newText).length) >= targets.hardCeiling) {
                    let forceEnd = Math.min(newText.length, targets.hardCeiling);
                    for (let index = forceEnd; index > 0; index -= 1) {
                        if (/\s/.test(newText.charAt(index))) {
                            forceEnd = index;
                            break;
                        }
                    }
                    const forcedCommit = `${nextBuffer} ${cleanTextForTTS(newText.slice(0, forceEnd))}`.trim();
                    if (forcedCommit) {
                        committed = forcedCommit;
                        nextBuffer = '';
                        advanceBy = forceEnd;
                    }
                }

                if (!committed) break;

                console.log(`[Streaming TTS] Committing (${committed.length} chars): "${committed.substring(0, 50)}..."`);
                nextCommittedIndex += advanceBy;
                setPlaybackState?.({
                    streamingTTSBuffer: nextBuffer,
                    streamingTTSCommittedIndex: nextCommittedIndex
                });
                pushToStreamingTTSQueue(committed, true);
            }
        }

        function finalizeStreamingTTS(finalDisplayText) {
            const state = getPlaybackState?.() || {};
            if (!state.streamingTTSActive) return;

            const remainingText = String(finalDisplayText || '').substring(Number(state.streamingTTSCommittedIndex || 0));
            const cleanText = cleanTextForTTS(remainingText);
            const finalText = `${String(state.streamingTTSBuffer || '')} ${cleanText || ''}`.trim();

            if (finalText) {
                console.log(`[Streaming TTS] Finalizing: "${finalText.substring(0, 50)}..."`);
                pushToStreamingTTSQueue(finalText, true);
            }

            setPlaybackState?.({
                streamingTTSBuffer: '',
                streamingTTSActive: false
            });
            console.log('[Streaming TTS] Finalized');
        }

        function supportsOSTTS() {
            return typeof global.speechSynthesis !== 'undefined'
                && typeof global.SpeechSynthesisUtterance !== 'undefined';
        }

        function getCurrentTTSEngine() {
            if (config.ttsEngine === 'os') return 'os';
            return config.ttsEngine === 'supertonic-ondevice' ? 'supertonic-ondevice' : 'supertonic';
        }

        function getSelectedOSTTSVoice() {
            if (!supportsOSTTS()) return null;
            if (!Array.isArray(osTTSVoices) || osTTSVoices.length === 0) {
                osTTSVoices = global.speechSynthesis.getVoices() || [];
            }

            const byURI = osTTSVoices.find((voice) => voice.voiceURI === config.osTtsVoiceURI);
            if (byURI) return byURI;

            const byNameLang = osTTSVoices.find((voice) =>
                voice.name === config.osTtsVoiceName && voice.lang === config.osTtsVoiceLang
            );
            if (byNameLang) return byNameLang;

            return osTTSVoices.find((voice) => String(voice.lang || '').toLowerCase().startsWith('ko'))
                || osTTSVoices[0]
                || null;
        }

        function syncOSTTSVoiceConfigFromSelection() {
            const selectedVoiceURI = osTTSVoiceSelect?.value || config.osTtsVoiceURI;
            const selected = osTTSVoices.find((voice) => voice.voiceURI === selectedVoiceURI) || getSelectedOSTTSVoice();
            if (!selected) return;
            config.osTtsVoiceURI = selected.voiceURI || '';
            config.osTtsVoiceName = selected.name || '';
            config.osTtsVoiceLang = selected.lang || '';
        }

        function populateOSTTSVoiceList() {
            if (!osTTSVoiceSelect) return;

            if (!supportsOSTTS()) {
                osTTSVoices = [];
                osTTSVoicesReady = false;
                osTTSVoiceSelect.innerHTML = `<option value="">${escapeHtml(t('setting.osVoice.unavailable'))}</option>`;
                osTTSVoiceSelect.disabled = true;
                return;
            }

            osTTSVoices = global.speechSynthesis.getVoices() || [];
            if (!osTTSVoices.length) {
                osTTSVoicesReady = false;
                osTTSVoiceSelect.innerHTML = `<option value="">${escapeHtml(t('setting.osVoice.loading'))}</option>`;
                osTTSVoiceSelect.disabled = true;
                return;
            }

            osTTSVoicesReady = true;
            osTTSVoiceSelect.disabled = false;
            osTTSVoiceSelect.innerHTML = osTTSVoices.map((voice) => {
                const label = `${voice.name} (${voice.lang})${voice.default ? ' - DEFAULT' : ''}`;
                return `<option value="${escapeAttr(voice.voiceURI || '')}">${escapeHtml(label)}</option>`;
            }).join('');

            const selected = getSelectedOSTTSVoice();
            if (selected?.voiceURI) {
                osTTSVoiceSelect.value = selected.voiceURI;
            }
            syncOSTTSVoiceConfigFromSelection();
        }

        function initOSTTSVoiceLoading() {
            if (!supportsOSTTS()) {
                populateOSTTSVoiceList();
                return;
            }

            populateOSTTSVoiceList();
            if (typeof global.speechSynthesis.onvoiceschanged !== 'undefined') {
                global.speechSynthesis.onvoiceschanged = () => {
                    populateOSTTSVoiceList();
                };
            }
        }

        function updateTTSSettingsVisibility() {
            const engine = getCurrentTTSEngine();
            const supertonicIds = [
                'container-tts-supertonic-voice',
                'container-tts-supertonic-speed',
                'container-tts-lang',
                'container-tts-supertonic-steps',
                'container-tts-supertonic-threads',
                'container-tts-supertonic-format'
            ];
            const osIds = [
                'container-tts-os-voice',
                'container-tts-os-rate',
                'container-tts-os-pitch'
            ];

            supertonicIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine !== 'os' ? 'block' : 'none';
            });

            osIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine === 'os' ? 'block' : 'none';
            });

            const serverOnlyIds = ['container-tts-supertonic-threads', 'container-tts-supertonic-format'];
            serverOnlyIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el && engine === 'supertonic-ondevice') el.style.display = 'none';
            });
        }

        return {
            attachStreamingAudioButtonToMessage,
            clearMediaSessionMetadata,
            cleanTextForTTS,
            combinePlayableChunks,
            concatenateWavArrayBuffers,
            detectStreamingBoundary,
            detachCurrentAudioPlaybackListeners,
            endTTS,
            feedStreamingTTS,
            finalizeStreamingTTS,
            firstChunkPlayedInCurrentSession,
            getCurrentTTSEngine,
            getSelectedOSTTSVoice,
            getStreamingChunkTargets,
            getVoices: () => osTTSVoices,
            initStreamingTTS,
            initOSTTSVoiceLoading,
            isVoicesReady: () => osTTSVoicesReady,
            loadVoiceStyles,
            populateOSTTSVoiceList,
            prefetchTTSAudio,
            processOSTTSQueue,
            processTTSQueue,
            pushToStreamingTTSQueue,
            readWavHeader,
            clearTTSAudioCache,
            speakMessage,
            speakMessageFromBtn,
            stopAllAudio,
            shouldCommitStreamingBoundary,
            splitTTSParagraphByPriority,
            supportsOSTTS,
            syncCurrentAudioButtonUI,
            syncOSTTSVoiceConfigFromSelection,
            unlockAudioContext,
            updateMediaSessionMetadata,
            updateTTSSettingsVisibility
        };
    }

    global.DKSTTTS = {
        createTTSController
    };
})(window);
