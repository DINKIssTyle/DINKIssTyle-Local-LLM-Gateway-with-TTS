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
                btn.classList.remove('btn-danger', 'playing', 'loading');
                btn.disabled = false;
            }

            setPlaybackState?.({
                currentAudioBtn: null,
                isPlayingQueue: false,
                streamingTTSActive: false,
                streamingTTSBuffer: '',
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
            if (btn) {
                const icon = btn.querySelector('.material-icons-round');
                if (icon) icon.textContent = 'volume_up';
                btn.classList.remove('btn-danger', 'playing', 'loading');
                btn.disabled = false;
            }
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

        async function speakWithOnDeviceSupertonic(cleanText, btn, sessionId) {
            try {
                const chunks = global.DKSTSupertonic3?.chunkSpeechText
                    ? global.DKSTSupertonic3.chunkSpeechText(cleanText)
                    : [cleanText];

                if (btn) {
                    btn.classList.remove('loading');
                    btn.classList.add('playing', 'btn-danger');
                    const icon = btn.querySelector('.material-icons-round');
                    if (icon) icon.textContent = 'stop';
                }

                await global.DKSTSupertonic3.speakSupertonic3(chunks, {
                    voice: config.ttsVoice || 'F1',
                    speed: getConfiguredTTSSynthesisSpeed(),
                    language: config.ttsLang || 'ko',
                    steps: parseInt(config.ttsSteps) || 5,
                    onPreparationProgress: showPreparationToast
                });

                if (sessionId === activeTTSSessionId) {
                    endTTS(btn, sessionId);
                }
            } catch (err) {
                if (sessionId === activeTTSSessionId) {
                    console.error('[On-device TTS] Supertonic synthesis error:', err);
                    endTTS(btn, sessionId);
                }
            }
        }

        async function speakWithServerSupertonic(cleanText, btn, sessionId) {
            try {
                const chunks = global.DKSTSupertonic3?.chunkSpeechText
                    ? global.DKSTSupertonic3.chunkSpeechText(cleanText, 300)
                    : [cleanText];

                if (btn) {
                    btn.classList.remove('loading');
                    btn.classList.add('playing', 'btn-danger');
                    const icon = btn.querySelector('.material-icons-round');
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

                if (sessionId === activeTTSSessionId) {
                    endTTS(btn, sessionId);
                }
            } catch (err) {
                if (sessionId === activeTTSSessionId) {
                    console.error('[Server TTS] synthesis error:', err);
                    endTTS(btn, sessionId);
                }
            }
        }

        async function speakWithOSTTS(text, btn, sessionId) {
            if (!supportsOSTTS()) {
                if (btn) global.alert?.(t('setting.osVoice.unavailable') || 'OS TTS unavailable');
                endTTS(btn, sessionId);
                return;
            }

            const chunks = global.DKSTSupertonic3?.chunkSpeechText
                ? global.DKSTSupertonic3.chunkSpeechText(text, 200)
                : [text];

            if (btn) {
                btn.classList.remove('loading');
                btn.classList.add('playing', 'btn-danger');
                const icon = btn.querySelector('.material-icons-round');
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

            if (sessionId === activeTTSSessionId) {
                endTTS(btn, sessionId);
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
        let streamingChunks = [];
        let isStreamingPlaying = false;

        function initStreamingTTS() {
            stopAllAudio();
            streamingChunks = [];
            isStreamingPlaying = false;
            setPlaybackState?.({
                streamingTTSActive: true,
                streamingTTSBuffer: '',
                streamingTTSCommittedIndex: 0,
                ttsSessionId: ++activeTTSSessionId
            });
            onSyncWakeLock?.();
        }

        function feedStreamingTTS(newTextDelta) {
            const state = getPlaybackState?.() || {};
            if (!state.streamingTTSActive) return;

            const buffer = (state.streamingTTSBuffer || '') + (newTextDelta || '');
            const cleaned = cleanTextForTTS(buffer);

            // Sentence boundary detection for smooth streaming
            const match = cleaned.match(/^([\s\S]+?[.!?。！？\n])\s*([\s\S]*)$/);
            if (match && match[1].trim()) {
                const completeSentence = match[1].trim();
                const remainder = match[2] || '';
                setPlaybackState?.({ streamingTTSBuffer: remainder });
                pushStreamingChunk(completeSentence);
            } else {
                setPlaybackState?.({ streamingTTSBuffer: buffer });
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

            while (streamingChunks.length > 0) {
                if (sessionId !== activeTTSSessionId) break;
                const chunk = streamingChunks.shift();
                if (!chunk) continue;

                const engine = getCurrentTTSEngine();
                if (engine === 'os') {
                    await speakWithOSTTS(chunk, null, sessionId);
                } else if (engine === 'supertonic-ondevice') {
                    await speakWithOnDeviceSupertonic(chunk, null, sessionId);
                } else {
                    await speakWithServerSupertonic(chunk, null, sessionId);
                }
            }
            isStreamingPlaying = false;
        }

        function finalizeStreamingTTS() {
            const state = getPlaybackState?.() || {};
            if (state.streamingTTSBuffer?.trim()) {
                const remaining = cleanTextForTTS(state.streamingTTSBuffer);
                if (remaining) {
                    pushStreamingChunk(remaining);
                }
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
            const supertonicIds = [
                'container-tts-supertonic-voice',
                'container-tts-supertonic-speed',
                'container-tts-lang',
                'container-tts-supertonic-steps'
            ];
            const osIds = [
                'container-tts-os-voice',
                'container-tts-os-rate',
                'container-tts-os-pitch'
            ];
            const serverOnlyIds = [
                'container-tts-supertonic-threads',
                'container-tts-supertonic-format'
            ];

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
            const btn = state.currentAudioBtn;
            if (!btn) return;
            const icon = btn.querySelector('.material-icons-round');
            if (state.isPlayingQueue || state.streamingTTSActive) {
                if (icon) icon.textContent = 'stop';
                btn.classList.add('playing', 'btn-danger');
                btn.classList.remove('loading');
            } else {
                if (icon) icon.textContent = 'volume_up';
                btn.classList.remove('playing', 'btn-danger', 'loading');
            }
        }

        return {
            attachStreamingAudioButtonToMessage: () => {},
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
