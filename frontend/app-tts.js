/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 * High-performance TTS Pipeline with Supertonic 3 (Server / On-Device WebGPU & WASM / OS)
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
            getSpeakableTextFromMarkdownHost,
            getPlaybackState,
            onDetachCurrentAudioPlaybackListeners,
            onSetAssistantActionBarReady,
            onSyncCurrentAudioButtonUI,
            onSyncWakeLock,
            setPlaybackState,
            t
        } = deps;

        let osTTSVoices = [];
        let osTTSVoicesReady = false;
        let activeTTSSessionId = 0;
        let lastSupertonicToastAt = 0;
        let streamingCommittedIndex = 0;
        let streamingChunks = [];
        let isStreamingPlaying = false;
        let streamingElementId = null;

        function normalizeTTSSynthesisSpeed(speed) {
            return Math.max(0.7, Math.min(2.0, Number(speed) || 0.9));
        }

        function getConfiguredTTSSynthesisSpeed() {
            return normalizeTTSSynthesisSpeed(config.ttsSpeed);
        }

        function supportsOSTTS() {
            return typeof global.speechSynthesis !== 'undefined'
                && typeof global.SpeechSynthesisUtterance !== 'undefined';
        }

        function getCurrentTTSEngine() {
            if (config.ttsEngine === 'os') return 'os';
            if (config.ttsEngine === 'supertonic-ondevice') return 'supertonic-ondevice';
            return 'supertonic'; // default: server
        }

        function unlockAudioContext() {
            if (global.DKSTSupertonic3?.unlockSupertonic3Audio) {
                global.DKSTSupertonic3.unlockSupertonic3Audio().catch(() => {});
            }
        }

        function cleanTextForTTS(text) {
            if (!text) return '';
            let cleaned = String(text);

            // Remove markdown code blocks
            cleaned = cleaned.replace(/```[\s\S]*?```/g, ' ');
            // Remove inline code
            cleaned = cleaned.replace(/`([^`]+)`/g, '$1');
            // Remove markdown images and links
            cleaned = cleaned.replace(/!\[([^\]]*)\]\([^)]*\)/g, ' ');
            cleaned = cleaned.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1');
            // Remove headers, bold, italics, strikethrough, blockquotes
            cleaned = cleaned.replace(/^[#>\s*+-]+/gm, ' ');
            cleaned = cleaned.replace(/[*_~]{1,3}([^*_~]+)[*_~]{1,3}/g, '$1');
            // Remove HTML tags
            cleaned = cleaned.replace(/<[^>]*>/g, ' ');
            // Remove thinking blocks
            cleaned = cleaned.replace(/<think>[\s\S]*?<\/think>/gi, ' ');
            // Use Supertonic speech cleaner
            if (global.DKSTSupertonic3?.cleanSpeechText) {
                cleaned = global.DKSTSupertonic3.cleanSpeechText(cleaned);
            }
            return cleaned.trim();
        }

        function updateMediaSessionMetadata(title) {
            if (!('mediaSession' in global.navigator)) return;
            try {
                const MediaMetadataClass = global.MediaMetadata || global.window?.MediaMetadata;
                if (!MediaMetadataClass) return;
                global.navigator.mediaSession.metadata = new MediaMetadataClass({
                    title: String(title || 'TTS Audio').trim().substring(0, 100),
                    artist: 'DKST Chat Supertonic 3',
                    album: 'DKST Voice'
                });
                global.navigator.mediaSession.playbackState = 'playing';
            } catch (e) {
                console.warn('[TTS] Failed to update media session metadata:', e);
            }
        }

        function clearMediaSessionMetadata() {
            if (!('mediaSession' in global.navigator)) return;
            try {
                global.navigator.mediaSession.metadata = null;
                global.navigator.mediaSession.playbackState = 'none';
            } catch (e) {
                console.warn('[TTS] Failed to clear media session metadata:', e);
            }
        }

        function stopAllAudio() {
            activeTTSSessionId += 1;
            streamingChunks = [];
            isStreamingPlaying = false;
            streamingCommittedIndex = 0;
            streamingElementId = null;

            if (global.DKSTSupertonic3?.cancelSupertonic3Speech) {
                global.DKSTSupertonic3.cancelSupertonic3Speech();
            }

            if (supportsOSTTS()) {
                try {
                    global.speechSynthesis.cancel();
                } catch {
                    // Ignore cancel error
                }
            }

            const state = getPlaybackState?.() || {};
            const btn = state.currentAudioBtn;
            if (btn) {
                const icon = btn.querySelector('.material-icons-round');
                if (icon) icon.textContent = 'volume_up';
                btn.classList.remove('playing', 'loading');
                btn.disabled = false;
            }

            setPlaybackState?.({
                currentAudioBtn: null,
                isPlayingQueue: false,
                streamingTTSActive: false,
                streamingTTSBuffer: '',
                streamingTTSCommittedIndex: 0,
                activeTTSSessionLabel: '',
                ttsQueue: []
            });

            clearMediaSessionMetadata();
            onSyncCurrentAudioButtonUI?.();
            onSyncWakeLock?.();
        }

        function endTTS(btn, sessionId) {
            const state = getPlaybackState?.() || {};
            if (sessionId && sessionId !== state.ttsSessionId && sessionId !== activeTTSSessionId) {
                return;
            }
            const targetBtn = btn || state.currentAudioBtn;
            if (targetBtn) {
                const icon = targetBtn.querySelector('.material-icons-round');
                if (icon) icon.textContent = 'volume_up';
                targetBtn.classList.remove('playing', 'loading');
                targetBtn.disabled = false;
            }
            streamingElementId = null;
            setPlaybackState?.({
                currentAudioBtn: null,
                isPlayingQueue: false,
                streamingTTSActive: false,
                activeTTSSessionLabel: ''
            });
            clearMediaSessionMetadata();
            onSyncCurrentAudioButtonUI?.();
            onSyncWakeLock?.();
        }

        function showPreparationToast(progress) {
            if (!progress) return;
            const now = Date.now();
            if (now - lastSupertonicToastAt < 1000) return;
            lastSupertonicToastAt = now;

            let text = '';
            if (progress.phase === 'downloading') {
                const pct = progress.fraction != null ? ` (${Math.round(progress.fraction * 100)}%)` : '';
                text = `Supertonic 3 모델 다운로드 중...${pct}`;
            } else if (progress.phase === 'restoring') {
                const pct = progress.fraction != null ? ` (${Math.round(progress.fraction * 100)}%)` : '';
                text = `Supertonic 3 캐시 로드 중...${pct}`;
            } else if (progress.phase === 'compiling') {
                const pct = progress.fraction != null ? ` (${Math.round(progress.fraction * 100)}%)` : '';
                text = `Supertonic 3 엔진 준비 중...${pct}`;
            } else if (progress.phase === 'runtime') {
                text = 'Supertonic 3 런타임 초기화 중...';
            }
            if (text && global.showToast) {
                global.showToast(text);
            }
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

            const cleanText = cleanTextForTTS(text);
            if (!cleanText) return;

            const sessionId = ++activeTTSSessionId;
            setPlaybackState?.({
                currentAudioBtn: btn,
                isPlayingQueue: true,
                ttsSessionId: sessionId,
                activeTTSSessionLabel: cleanText.substring(0, 120) + (cleanText.length > 120 ? '...' : '')
            });

            if (btn) {
                btn.classList.add('loading');
                const icon = btn.querySelector('.material-icons-round');
                if (icon) icon.textContent = 'hourglass_empty';
                onSyncCurrentAudioButtonUI?.();
            }

            onSyncWakeLock?.();
            updateMediaSessionMetadata(cleanText);

            const engine = getCurrentTTSEngine();

            if (engine === 'os') {
                await speakWithOSTTS(cleanText, btn, sessionId);
                return;
            }

            if (engine === 'supertonic-ondevice') {
                await speakWithOnDeviceSupertonic(cleanText, btn, sessionId);
                return;
            }

            // Server-side Supertonic 3
            await speakWithServerSupertonic(cleanText, btn, sessionId);
        }

        async function speakWithOnDeviceSupertonic(cleanText, btn, sessionId, options = {}) {
            const isStreamingChunk = !!options.isStreaming;
            const targetBtn = btn || (getPlaybackState?.() || {}).currentAudioBtn;
            try {
                const chunks = global.DKSTSupertonic3?.chunkSpeechText
                    ? global.DKSTSupertonic3.chunkSpeechText(cleanText)
                    : [cleanText];

                if (targetBtn) {
                    targetBtn.classList.remove('loading');
                    targetBtn.classList.add('playing');
                    const icon = targetBtn.querySelector('.material-icons-round');
                    if (icon) icon.textContent = 'stop';
                }

                await global.DKSTSupertonic3.speakSupertonic3(chunks, {
                    voice: config.ttsVoice || 'F1',
                    speed: getConfiguredTTSSynthesisSpeed(),
                    language: config.ttsLang || 'ko',
                    steps: parseInt(config.ttsSteps) || 5,
                    onPreparationProgress: showPreparationToast
                });

                if (!isStreamingChunk && sessionId === activeTTSSessionId) {
                    endTTS(targetBtn, sessionId);
                }
            } catch (err) {
                if (sessionId === activeTTSSessionId) {
                    console.error('[On-device TTS] Supertonic synthesis error:', err);
                    if (!isStreamingChunk) {
                        endTTS(targetBtn, sessionId);
                    }
                }
            }
        }

        async function speakWithServerSupertonic(cleanText, btn, sessionId, options = {}) {
            const isStreamingChunk = !!options.isStreaming;
            const targetBtn = btn || (getPlaybackState?.() || {}).currentAudioBtn;
            try {
                const chunks = global.DKSTSupertonic3?.chunkSpeechText
                    ? global.DKSTSupertonic3.chunkSpeechText(cleanText, 300)
                    : [cleanText];

                if (targetBtn) {
                    targetBtn.classList.remove('loading');
                    targetBtn.classList.add('playing');
                    const icon = targetBtn.querySelector('.material-icons-round');
                    if (icon) icon.textContent = 'stop';
                }

                unlockAudioContext();
                const audioCtx = global.DKSTSupertonic3?.getSupertonic3AudioContext?.()
                    || new (global.AudioContext || global.webkitAudioContext)({ sampleRate: 44100 });

                let finalPlayback = Promise.resolve();

                for (let index = 0; index < chunks.length; index += 1) {
                    if (sessionId !== activeTTSSessionId) break;
                    const chunk = chunks[index];
                    if (!chunk.trim()) continue;

                    const res = await fetch('/api/tts', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            text: chunk,
                            lang: config.ttsLang || 'ko',
                            voiceStyle: config.ttsVoice || 'F1',
                            speed: getConfiguredTTSSynthesisSpeed(),
                            format: config.ttsAudioFormat || 'mp3',
                            steps: parseInt(config.ttsSteps) || 5,
                            prechunked: true
                        })
                    });

                    if (sessionId !== activeTTSSessionId) break;
                    if (!res.ok) {
                        throw new Error(`Server TTS failed (${res.status})`);
                    }

                    const arrayBuffer = await res.arrayBuffer();
                    if (sessionId !== activeTTSSessionId) break;

                    const decodedBuffer = await audioCtx.decodeAudioData(arrayBuffer);
                    if (sessionId !== activeTTSSessionId) break;

                    const isFinal = index === chunks.length - 1;
                    const pauseSeconds = isFinal ? 0 : (global.DKSTSupertonic3?.boundaryPause?.(chunk) ?? 0.08);

                    if (global.DKSTSupertonic3?.scheduleSupertonic3AudioBuffer) {
                        finalPlayback = global.DKSTSupertonic3.scheduleSupertonic3AudioBuffer(decodedBuffer, pauseSeconds);
                    }
                }

                await finalPlayback;

                if (!isStreamingChunk && sessionId === activeTTSSessionId) {
                    endTTS(targetBtn, sessionId);
                }
            } catch (err) {
                if (sessionId === activeTTSSessionId) {
                    console.error('[Server TTS] synthesis error:', err);
                    if (!isStreamingChunk) {
                        endTTS(targetBtn, sessionId);
                    }
                }
            }
        }

        async function speakWithOSTTS(text, btn, sessionId, options = {}) {
            const isStreamingChunk = !!options.isStreaming;
            const targetBtn = btn || (getPlaybackState?.() || {}).currentAudioBtn;
            if (!supportsOSTTS()) {
                if (targetBtn) global.alert?.(t('setting.osVoice.unavailable') || 'OS TTS unavailable');
                if (!isStreamingChunk) endTTS(targetBtn, sessionId);
                return;
            }

            const chunks = global.DKSTSupertonic3?.chunkSpeechText
                ? global.DKSTSupertonic3.chunkSpeechText(text, 200)
                : [text];

            if (targetBtn) {
                targetBtn.classList.remove('loading');
                targetBtn.classList.add('playing');
                const icon = targetBtn.querySelector('.material-icons-round');
                if (icon) icon.textContent = 'stop';
            }

            const osVoice = getSelectedOSTTSVoice();
            const osRate = Math.max(0.1, Math.min(10.0, Number(config.osTtsRate) || 1.0));
            const osPitch = Math.max(0, Math.min(2, Number(config.osTtsPitch) || 1.0));

            for (const chunk of chunks) {
                if (sessionId !== activeTTSSessionId) break;
                await new Promise((resolve) => {
                    const utterance = new global.SpeechSynthesisUtterance(chunk);
                    if (osVoice) utterance.voice = osVoice;
                    utterance.rate = osRate;
                    utterance.pitch = osPitch;
                    utterance.onend = () => resolve();
                    utterance.onerror = (e) => {
                        console.error('[OS TTS] Playback error:', e);
                        resolve();
                    };
                    global.speechSynthesis.speak(utterance);
                });
            }

            if (!isStreamingChunk && sessionId === activeTTSSessionId) {
                endTTS(targetBtn, sessionId);
            }
        }

        function speakMessageFromBtn(btn) {
            const bubble = btn?.closest('.message-inner')?.querySelector('.markdown-body');
            if (bubble) {
                return speakMessage(getSpeakableTextFromMarkdownHost?.(bubble) || '', btn);
            }
            return null;
        }

        // Streaming TTS support
        function initStreamingTTS(elementId = null) {
            stopAllAudio();
            streamingChunks = [];
            isStreamingPlaying = false;
            streamingCommittedIndex = 0;
            const sessionId = ++activeTTSSessionId;
            streamingElementId = elementId || getActiveStreamingMessageId?.() || null;

            let btn = null;
            if (streamingElementId) {
                const el = global.document.getElementById(streamingElementId);
                btn = el?.querySelector('.speak-btn') || null;
            }

            setPlaybackState?.({
                currentAudioBtn: btn,
                streamingTTSActive: true,
                streamingTTSBuffer: '',
                streamingTTSCommittedIndex: 0,
                ttsSessionId: sessionId,
                isPlayingQueue: true
            });
            syncCurrentAudioButtonUI();
            onSyncWakeLock?.();
        }

        function attachStreamingAudioButtonToMessage(msgEl) {
            if (!msgEl) return;
            const state = getPlaybackState?.() || {};
            if (!state.streamingTTSActive && !state.isPlayingQueue) return;

            const targetId = streamingElementId || getActiveStreamingMessageId?.();
            if (targetId && msgEl.id && msgEl.id !== targetId) return;

            const btn = msgEl.querySelector('.speak-btn');
            if (btn && (!state.currentAudioBtn || state.currentAudioBtn !== btn)) {
                setPlaybackState?.({
                    currentAudioBtn: btn
                });
                syncCurrentAudioButtonUI();
            }
        }

        function feedStreamingTTS(fullDisplayText) {
            const state = getPlaybackState?.() || {};
            if (!state.streamingTTSActive) return;

            const rawText = String(fullDisplayText || '');
            if (rawText.length <= streamingCommittedIndex) return;

            const uncommitted = rawText.slice(streamingCommittedIndex);

            // Find all complete sentences in uncommitted text
            // Regex matches sentence ending with ., !, ?, 。, ！, ？, or \n followed by whitespace or string end
            const sentenceBoundaryRegex = /([\s\S]+?[.!?。！？\n])(?:\s+|$)/g;
            let match;
            let lastMatchedEnd = 0;

            while ((match = sentenceBoundaryRegex.exec(uncommitted)) !== null) {
                const rawSentence = match[1];
                const clean = cleanTextForTTS(rawSentence);
                if (clean) {
                    pushStreamingChunk(clean);
                }
                lastMatchedEnd = sentenceBoundaryRegex.lastIndex;
            }

            if (lastMatchedEnd > 0) {
                streamingCommittedIndex += lastMatchedEnd;
                setPlaybackState?.({
                    streamingTTSCommittedIndex: streamingCommittedIndex
                });
            }
        }

        function pushStreamingChunk(sentence) {
            streamingChunks.push(sentence);
            if (!isStreamingPlaying) {
                playStreamingQueue();
            }
        }

        async function playStreamingQueue() {
            if (isStreamingPlaying) return;
            isStreamingPlaying = true;
            const sessionId = activeTTSSessionId;

            syncCurrentAudioButtonUI();

            while (true) {
                if (sessionId !== activeTTSSessionId) break;
                if (streamingChunks.length === 0) {
                    const state = getPlaybackState?.() || {};
                    if (state.streamingTTSActive) {
                        // Wait briefly for more tokens from the live stream
                        await new Promise((resolve) => global.setTimeout(resolve, 80));
                        continue;
                    }
                    // Streaming ended and all chunks played
                    break;
                }

                const chunk = streamingChunks.shift();
                if (!chunk) continue;

                const engine = getCurrentTTSEngine();
                if (engine === 'os') {
                    await speakWithOSTTS(chunk, null, sessionId, { isStreaming: true });
                } else if (engine === 'supertonic-ondevice') {
                    await speakWithOnDeviceSupertonic(chunk, null, sessionId, { isStreaming: true });
                } else {
                    await speakWithServerSupertonic(chunk, null, sessionId, { isStreaming: true });
                }
            }

            isStreamingPlaying = false;
            const finalState = getPlaybackState?.() || {};
            if (sessionId === activeTTSSessionId && !finalState.streamingTTSActive) {
                endTTS(finalState.currentAudioBtn, sessionId);
            }
        }

        function finalizeStreamingTTS(finalDisplayText) {
            const rawText = String(finalDisplayText || '');
            if (rawText.length > streamingCommittedIndex) {
                const remaining = rawText.slice(streamingCommittedIndex);
                const clean = cleanTextForTTS(remaining);
                if (clean) {
                    pushStreamingChunk(clean);
                }
                streamingCommittedIndex = rawText.length;
            }
            setPlaybackState?.({
                streamingTTSActive: false,
                streamingTTSBuffer: ''
            });
        }

        async function loadVoiceStyles() {
            const select = global.document.getElementById('cfg-tts-voice');
            if (!select) return;

            const voices = global.DKSTSupertonic3?.SUPERTONIC3_VOICES || [
                { id: 'M1', name: 'Alex' },
                { id: 'M2', name: 'James' },
                { id: 'M3', name: 'Robert' },
                { id: 'M4', name: 'Sam' },
                { id: 'M5', name: 'Daniel' },
                { id: 'F1', name: 'Sarah' },
                { id: 'F2', name: 'Lily' },
                { id: 'F3', name: 'Jessica' },
                { id: 'F4', name: 'Olivia' },
                { id: 'F5', name: 'Emily' }
            ];

            select.innerHTML = voices.map((v) => {
                const selected = (v.id === (config.ttsVoice || 'F1')) ? 'selected' : '';
                return `<option value="${escapeAttr(v.id)}" ${selected}>${escapeHtml(v.id)} - ${escapeHtml(v.name)}</option>`;
            }).join('');
        }

        function populateOSTTSVoiceList() {
            if (!osTTSVoiceSelect) return;
            if (!supportsOSTTS()) {
                osTTSVoiceSelect.innerHTML = `<option value="">${escapeHtml(t('setting.osVoice.unavailable') || 'Unavailable')}</option>`;
                return;
            }

            const rawVoices = global.speechSynthesis.getVoices() || [];
            osTTSVoices = rawVoices;
            osTTSVoicesReady = rawVoices.length > 0;

            if (!osTTSVoicesReady) {
                osTTSVoiceSelect.innerHTML = `<option value="">${escapeHtml(t('setting.osVoice.loading') || 'Loading voices...')}</option>`;
                return;
            }

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

        function getSelectedOSTTSVoice() {
            if (!supportsOSTTS() || !osTTSVoices.length) return null;
            if (config.osTtsVoiceUri) {
                const match = osTTSVoices.find((v) => v.voiceURI === config.osTtsVoiceUri);
                if (match) return match;
            }
            if (config.osTtsVoiceName) {
                const match = osTTSVoices.find((v) => v.name === config.osTtsVoiceName);
                if (match) return match;
            }
            return osTTSVoices[0] || null;
        }

        function syncOSTTSVoiceConfigFromSelection() {
            if (!osTTSVoiceSelect) return;
            const selectedURI = osTTSVoiceSelect.value;
            const selected = osTTSVoices.find((v) => v.voiceURI === selectedURI);
            if (selected) {
                config.osTtsVoiceUri = selected.voiceURI;
                config.osTtsVoiceName = selected.name;
                config.osTtsVoiceLang = selected.lang;
            }
        }

        function updateTTSSettingsVisibility() {
            const engine = getCurrentTTSEngine();
            const supertonicIds = ['row-tts-voice', 'row-tts-speed', 'row-tts-format', 'row-tts-steps'];
            const osIds = ['row-os-voice', 'row-os-rate', 'row-os-pitch'];
            const serverOnlyIds = ['row-tts-format'];

            supertonicIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine !== 'os' ? 'block' : 'none';
            });

            osIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine === 'os' ? 'block' : 'none';
            });

            serverOnlyIds.forEach((id) => {
                const el = global.document.getElementById(id);
                if (el) el.style.display = engine === 'supertonic' ? 'block' : 'none';
            });
        }

        function syncCurrentAudioButtonUI() {
            const state = getPlaybackState?.() || {};
            let btn = state.currentAudioBtn;
            if (!btn && (state.isPlayingQueue || state.streamingTTSActive)) {
                const targetId = streamingElementId || getActiveStreamingMessageId?.();
                if (targetId) {
                    const el = global.document.getElementById(targetId);
                    btn = el?.querySelector('.speak-btn') || null;
                    if (btn) {
                        setPlaybackState?.({ currentAudioBtn: btn });
                    }
                }
            }
            if (!btn) return;
            const icon = btn.querySelector('.material-icons-round');
            if (state.isPlayingQueue || state.streamingTTSActive) {
                if (icon) icon.textContent = 'stop';
                btn.classList.add('playing');
                btn.classList.remove('loading');
            } else {
                if (icon) icon.textContent = 'volume_up';
                btn.classList.remove('playing', 'loading');
            }
        }

        return {
            attachStreamingAudioButtonToMessage,
            clearMediaSessionMetadata,
            cleanTextForTTS,
            combinePlayableChunks: (chunks) => chunks,
            concatenateWavArrayBuffers: () => new ArrayBuffer(0),
            detectStreamingBoundary: () => false,
            detachCurrentAudioPlaybackListeners: () => {},
            endTTS,
            feedStreamingTTS,
            finalizeStreamingTTS,
            firstChunkPlayedInCurrentSession: () => false,
            getCurrentTTSEngine,
            getSelectedOSTTSVoice,
            getStreamingChunkTargets: () => ({}),
            getVoices: () => osTTSVoices,
            initStreamingTTS,
            initOSTTSVoiceLoading,
            isVoicesReady: () => osTTSVoicesReady,
            loadVoiceStyles,
            populateOSTTSVoiceList,
            prefetchTTSAudio: async () => null,
            processOSTTSQueue: async () => {},
            processTTSQueue: async () => {},
            pushToStreamingTTSQueue: () => {},
            readWavHeader: () => ({ sampleRate: 44100, numChannels: 1 }),
            clearTTSAudioCache: () => {},
            speakMessage,
            speakMessageFromBtn,
            stopAllAudio,
            shouldCommitStreamingBoundary: () => false,
            splitTTSParagraphByPriority: (text) => global.DKSTSupertonic3?.chunkSpeechText ? global.DKSTSupertonic3.chunkSpeechText(text) : [text],
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
