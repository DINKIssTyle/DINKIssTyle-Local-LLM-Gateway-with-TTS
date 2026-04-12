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

        function concatenateWavArrayBuffers(buffers) {
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

            const totalDataLength = headers.reduce((sum, header) => sum + header.dataLength, 0);
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

        async function combinePlayableChunks(primaryUrl, queuedTexts) {
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
                    const cachedPromise = getCachedAudioPromise?.(text);
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
                const mergedBuffer = concatenateWavArrayBuffers(buffers);
                if (!mergedBuffer) {
                    return { url: primaryUrl, revokeInputs: null };
                }

                if (consumedTexts.length > 0) {
                    onCombinedQueueConsumed?.(consumedTexts);
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

        function prefetchTTSAudio(text) {
            if (!text) return null;
            const cache = getAudioCache?.();
            if (!cache) return null;
            const cacheKey = `${config.ttsEngine}:${config.ttsVoice}:${config.ttsSpeed}:${text}`;
            if (cache.has(cacheKey)) return cache.get(cacheKey);

            const promise = (async () => {
                const state = getPlaybackState?.() || {};
                const sessionAtStart = state.ttsSessionId;

                try {
                    const payload = {
                        text,
                        lang: config.ttsLang,
                        chunkSize: parseInt(config.chunkSize) || 300,
                        voiceStyle: config.ttsVoice,
                        speed: parseFloat(config.ttsSpeed) || 1.0,
                        steps: parseInt(config.ttsSteps) || 5,
                        format: config.ttsFormat || 'wav'
                    };

                    console.log(`[TTS] Prefetching (${config.ttsVoice}): "${text.substring(0, 25)}..."`);
                    const response = await global.fetch('/api/tts', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload)
                    });

                    const latestState = getPlaybackState?.() || {};
                    if (sessionAtStart !== latestState.ttsSessionId) {
                        console.log('[TTS] Session changed, discarding prefetch');
                        return null;
                    }

                    if (!response.ok) {
                        console.error('[TTS] Chunk failed:', await response.text());
                        return null;
                    }

                    const blob = await response.blob();
                    const url = global.URL.createObjectURL(blob);
                    console.log(`[TTS] Prefetch complete: "${text.substring(0, 25)}..."`);
                    return url;
                } catch (e) {
                    console.error('[TTS] Chunk error:', e);
                    return null;
                }
            })();

            cache.set(cacheKey, promise);
            return promise;
        }

        function endTTS(btn, sessionId) {
            const state = getPlaybackState?.() || {};
            if (sessionId !== state.ttsSessionId) return;

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
                ttsQueue: []
            });

            const MIN_CHUNK_LENGTH = 50;
            const paragraphs = cleanText.split(/\n\s*\n+/);
            const nextQueue = [];
            let currentChunk = '';

            for (const paragraph of paragraphs) {
                if (!paragraph.trim()) continue;

                const sentencePattern = /(?<=[.!?])(?=\s+(?:[A-Z가-힣]|$))/g;
                const rawChunks = paragraph.split(sentencePattern).filter((value) => value.trim());
                const sentences = rawChunks.length > 0 ? rawChunks : [paragraph];

                for (const part of sentences) {
                    const trimmedPart = part.trim();
                    if (!trimmedPart) continue;

                    if ((currentChunk + ' ' + trimmedPart).length > config.chunkSize && currentChunk.length >= MIN_CHUNK_LENGTH) {
                        nextQueue.push(currentChunk.trim());
                        currentChunk = '';
                    }

                    currentChunk = currentChunk ? `${currentChunk} ${trimmedPart}` : trimmedPart;
                }

                if (currentChunk.length >= MIN_CHUNK_LENGTH) {
                    nextQueue.push(currentChunk.trim());
                    currentChunk = '';
                }
            }

            if (currentChunk.trim()) {
                nextQueue.push(currentChunk.trim());
            }

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
                utterance.rate = parseFloat(config.osTtsRate) || 1.0;
                utterance.pitch = parseFloat(config.osTtsPitch) || 1.0;

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

            if (btn) {
                onSyncCurrentAudioButtonUI?.();
            }

            let firstChunkPlayed = false;

            const seededState = getPlaybackState?.() || {};
            const seededQueue = Array.isArray(seededState.ttsQueue) ? seededState.ttsQueue : [];
            for (let i = 0; i < Math.min(3, seededQueue.length); i += 1) {
                prefetchTTSAudio(seededQueue[i]);
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
                for (let i = 0; i < Math.min(2, nextQueue.length); i += 1) {
                    prefetchTTSAudio(nextQueue[i]);
                }

                let audioUrl = null;
                let playbackBundle = null;
                try {
                    audioUrl = await prefetchTTSAudio(text);
                } catch (e) {
                    console.error('Prefetch failed', e);
                }

                const cacheKey = `${config.ttsEngine}:${config.ttsVoice}:${config.ttsSpeed}:${text}`;
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

                    playbackBundle = await combinePlayableChunks(audioUrl, [...(latestState.ttsQueue || [])]);
                    const playbackUrl = playbackBundle?.url || audioUrl;

                    if (!firstChunkPlayed && btn) {
                        firstChunkPlayed = true;
                        onSyncCurrentAudioButtonUI?.();
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
            const maxChunkSize = Math.max(parseInt(config.chunkSize) || 200, 80);
            const paragraphs = text.split(/\n+/);
            const newChunks = [];

            for (const para of paragraphs) {
                if (!para.trim()) continue;
                const chunks = splitTTSParagraphByPriority(para, maxChunkSize, minChunkLength, force);
                for (const chunk of chunks) {
                    queue.push(chunk);
                    newChunks.push(chunk);
                }
            }

            setPlaybackState?.({ ttsQueue: queue });

            if (getCurrentTTSEngine() === 'supertonic') {
                for (const chunk of newChunks) {
                    prefetchTTSAudio(chunk);
                }
            }

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
                    const forcedCommit = `${nextBuffer} ${cleanTextForTTS(newText.slice(0, targets.hardCeiling))}`.trim();
                    if (forcedCommit) {
                        committed = forcedCommit;
                        nextBuffer = '';
                        advanceBy = Math.min(newText.length, targets.hardCeiling);
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
            return config.ttsEngine === 'os' ? 'os' : 'supertonic';
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
                if (el) el.style.display = engine === 'supertonic' ? 'block' : 'none';
            });

            osIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine === 'os' ? 'block' : 'none';
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
