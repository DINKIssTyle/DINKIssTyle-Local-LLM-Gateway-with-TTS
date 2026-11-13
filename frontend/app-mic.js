/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTMic(global) {
    /**
     * MicController: Handles STT (Speech to Text) and giant mic UI
     */
    function createMicController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const {
            messageInput = null,
            inlineMicBtn = null,
            giantMicBtn = null,
            micLayoutContainer = null
        } = refs;

        const {
            AppState,
            config,
            t,
            triggerHaptic,
            sendMessage,
            updateSendButtonState,
            updateMessageInputPlaceholder
        } = deps;

        const webkitSpeechRecognition = global.webkitSpeechRecognition || global.SpeechRecognition;

        function updateUIState() {
            const isActive = AppState.input.isSTTActive;
            
            if (giantMicBtn) {
                giantMicBtn.classList.toggle('active', isActive);
            }
            if (inlineMicBtn) {
                inlineMicBtn.classList.toggle('active', isActive);
            }
            if (micLayoutContainer) {
                micLayoutContainer.classList.toggle('active', isActive);
            }
            if (inlineMicBtn) {
                inlineMicBtn.title = isActive
                    ? (t('action.stopGeneration') || 'Stop')
                    : (t('action.voiceInput') || 'Voice Input');
                inlineMicBtn.innerHTML = `<span class="material-icons-round">${isActive ? 'stop' : 'mic'}</span>`;
            }

            if (isActive) {
                if (messageInput) messageInput.classList.add('stt-active');
                startPlaceholderAnimation();
            } else {
                if (messageInput) messageInput.classList.remove('stt-active');
                stopPlaceholderAnimation();
                updateMessageInputPlaceholder?.();
            }
            updateSendButtonState?.();
        }

        function startPlaceholderAnimation() {
            stopPlaceholderAnimation();
            AppState.input.sttPlaceholderIndex = 0;
            const updatePlaceholder = () => {
                const dots = '.'.repeat((AppState.input.sttPlaceholderIndex % 3) + 1);
                if (messageInput) {
                    messageInput.placeholder = `${t('input.listening')}${dots}`;
                }
                AppState.input.sttPlaceholderIndex++;
            };
            updatePlaceholder();
            AppState.input.sttPlaceholderTimer = global.setInterval(updatePlaceholder, 500);
        }

        function stopPlaceholderAnimation() {
            if (AppState.input.sttPlaceholderTimer) {
                global.clearInterval(AppState.input.sttPlaceholderTimer);
                AppState.input.sttPlaceholderTimer = null;
            }
        }

        function toggle() {
            if (AppState.input.isSTTActive) {
                stop();
            } else {
                start();
            }
        }

        let session = 0;
        let resumeTimer = null;
        let localCapture = null;
        let requestController = null;
        let processing = false;
        let browserHasText = false;

        function isBusy() {
            return !!(AppState.chat?.isGenerating || AppState.audio?.isPlayingQueue
                || AppState.audio?.currentAudio || AppState.audio?.streamingTTSActive
                || global.speechSynthesis?.speaking);
        }

        function isCurrent(id) {
            return session === id && AppState.input.isSTTActive;
        }

        function reportError(error) {
            console.warn('[STT]', error);
            stop({ suppressAutoSend: true, forceAbort: true });
            global.alert(error?.name === 'NotAllowedError'
                ? t('error.micPermission')
                : `${t('error.sttFailed')}: ${error.message || error}`);
        }

        function scheduleListen(id) {
            if (!isCurrent(id)) return;
            global.clearTimeout(resumeTimer);
            resumeTimer = global.setTimeout(() => {
                if (!isCurrent(id)) return;
                if (isBusy() || processing) return scheduleListen(id);
                if (config.sttMethod === 'local') {
                    if (!localCapture) startLocal(id).catch(error => {
                        if (isCurrent(id)) reportError(error);
                    });
                } else startBrowser(id);
            }, 400);
        }

        async function acceptTranscript(text, id) {
            if (!isCurrent(id)) return;
            text = String(text || '').trim();
            if (text && messageInput) {
                const current = messageInput.value.trim();
                messageInput.value = current ? `${current} ${text}` : text;
                messageInput.dispatchEvent(new Event('input', { bubbles: true }));
                updateSendButtonState?.();
            }
            if (!config.sttContinuous) stop();
            if (text && !AppState.input.sttSuppressAutoSend) {
                await sendMessage?.({ fromVoiceInput: true });
            }
            if (config.sttContinuous && isCurrent(id)) scheduleListen(id);
        }

        function start() {
            if (AppState.input.isSTTActive) return;
            if (config.sttMethod !== 'local' && !webkitSpeechRecognition) {
                global.alert(t('error.sttNotSupported'));
                return;
            }
            triggerHaptic?.('light');
            AppState.input.sttSuppressAutoSend = false;
            AppState.input.isSTTActive = true;
            const id = ++session;
            updateUIState();
            if (config.sttMethod === 'local') {
                startLocal(id).catch(error => { if (isCurrent(id)) reportError(error); });
            } else startBrowser(id);
        }

        function startBrowser(id) {
            if (!isCurrent(id)) return;
            const recognition = new webkitSpeechRecognition();
            const recognitionLangs = {
                ar: 'ar-SA',
                bg: 'bg-BG',
                cs: 'cs-CZ',
                da: 'da-DK',
                de: 'de-DE',
                el: 'el-GR',
                en: 'en-US',
                es: 'es-ES',
                et: 'et-EE',
                fi: 'fi-FI',
                fr: 'fr-FR',
                hi: 'hi-IN',
                hr: 'hr-HR',
                hu: 'hu-HU',
                id: 'id-ID',
                it: 'it-IT',
                ja: 'ja-JP',
                ko: 'ko-KR',
                lt: 'lt-LT',
                lv: 'lv-LV',
                nl: 'nl-NL',
                pl: 'pl-PL',
                pt: 'pt-PT',
                ro: 'ro-RO',
                ru: 'ru-RU',
                sk: 'sk-SK',
                sl: 'sl-SI',
                sv: 'sv-SE',
                tr: 'tr-TR',
                uk: 'uk-UA',
                vi: 'vi-VN',
            };
            recognition.lang = recognitionLangs[config.ttsLang] || 'en-US';
            recognition.continuous = false;
            recognition.interimResults = true;
            browserHasText = false;
            let failed = false;
            let paused = false;
            const pauseTimer = global.setInterval(() => {
                if (isCurrent(id) && isBusy()) {
                    paused = true;
                    recognition.abort();
                }
            }, 100);
            recognition.onresult = (event) => {
                if (!isCurrent(id) || paused || isBusy()) return;
                let text = '';
                for (let i = event.resultIndex; i < event.results.length; i++) {
                    if (event.results[i].isFinal) text += event.results[i][0].transcript;
                }
                if (text && messageInput) {
                    const current = messageInput.value.trim();
                    messageInput.value = current ? `${current} ${text}` : text;
                    browserHasText = true;
                    messageInput.dispatchEvent(new Event('input', { bubbles: true }));
                    updateSendButtonState?.();
                }
            };
            recognition.onerror = (event) => {
                if (!isCurrent(id) || paused || event.error === 'aborted') return;
                if (event.error === 'no-speech' && config.sttContinuous) return;
                failed = true;
                const error = new Error(event.error);
                if (event.error === 'not-allowed' || event.error === 'service-not-allowed') error.name = 'NotAllowedError';
                reportError(error);
            };
            recognition.onend = async () => {
                global.clearInterval(pauseTimer);
                if (AppState.input.recognition === recognition) AppState.input.recognition = null;
                if (!isCurrent(id) || failed) return;
                const shouldSend = browserHasText && !paused && !AppState.input.sttSuppressAutoSend;
                browserHasText = false;
                if (!config.sttContinuous) stop();
                try {
                    if (shouldSend && messageInput?.value.trim()) await sendMessage?.({ fromVoiceInput: true });
                    if (config.sttContinuous && isCurrent(id)) scheduleListen(id);
                } catch (error) { if (isCurrent(id)) reportError(error); }
            };
            AppState.input.recognition = recognition;
            try { recognition.start(); }
            catch (error) {
                global.clearInterval(pauseTimer);
                reportError(error);
            }
        }

        function releaseCapture() {
            if (!localCapture) return;
            const capture = localCapture;
            localCapture = null;
            capture.processor.onaudioprocess = null;
            capture.processor.disconnect();
            capture.source.disconnect();
            capture.stream.getTracks().forEach(track => track.stop());
            capture.context.close().catch(() => {});
        }

        async function startLocal(id) {
            if (!global.navigator.mediaDevices?.getUserMedia || !(global.AudioContext || global.webkitAudioContext)) {
                throw new Error(t('error.sttSecureContext'));
            }
            // Resume audio in the microphone button's user gesture, before awaiting permission/status.
            const AudioContext = global.AudioContext || global.webkitAudioContext;
            const context = new AudioContext({ sampleRate: 16000 });
            let stream;
            try {
                await context.resume();
                const statusResponse = await global.fetch('/api/stt/status', { credentials: 'same-origin' });
                if (!statusResponse.ok) throw new Error(await statusResponse.text());
                const status = await statusResponse.json();
                if (status.status !== 'ready') throw new Error(t('error.sttModelMissing'));
                if (!isCurrent(id)) { await context.close(); return; }
                stream = await global.navigator.mediaDevices.getUserMedia({
                    audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true }
                });
                if (!isCurrent(id)) {
                    stream.getTracks().forEach(track => track.stop());
                    await context.close();
                    return;
                }
                const source = context.createMediaStreamSource(stream);
                const processor = context.createScriptProcessor(4096, 1, 1);
                localCapture = { stream, context, source, processor };
                let chunks = [], samples = 0, silent = 0, voiced = 0;
                let preRoll = [];
                const reset = () => { chunks = []; samples = 0; silent = 0; voiced = 0; preRoll = []; };
                processor.onaudioprocess = event => {
                    if (!isCurrent(id) || processing || isBusy()) { reset(); return; }
                    const input = event.inputBuffer.getChannelData(0);
                    const ratio = context.sampleRate / 16000;
                    const pcm = new Int16Array(Math.floor(input.length / ratio));
                    let energy = 0;
                    for (let i = 0; i < pcm.length; i++) {
                        const sample = Math.max(-1, Math.min(1, input[Math.min(input.length - 1, Math.floor(i * ratio))]));
                        energy += sample * sample;
                        pcm[i] = sample < 0 ? sample * 32768 : sample * 32767;
                    }
                    const speech = Math.sqrt(energy / pcm.length) > 0.012;
                    if (!samples && !speech) {
                        preRoll.push(pcm);
                        if (preRoll.length > 2) preRoll.shift();
                        return;
                    }
                    if (!samples) { chunks.push(...preRoll); samples += preRoll.reduce((n, chunk) => n + chunk.length, 0); preRoll = []; }
                    chunks.push(pcm);
                    samples += pcm.length;
                    if (speech) { voiced += pcm.length; silent = 0; }
                    else silent += pcm.length;
                    if (samples >= 25 * 16000 || silent >= 0.9 * 16000) {
                        if (voiced < 0.25 * 16000) { reset(); return; }
                        const audio = new Int16Array(samples);
                        let offset = 0;
                        for (const chunk of chunks) { audio.set(chunk, offset); offset += chunk.length; }
                        reset();
                        transcribe(audio, id).catch(error => { if (isCurrent(id)) reportError(error); });
                    }
                };
                source.connect(processor);
                processor.connect(context.destination);
            } catch (error) {
                stream?.getTracks().forEach(track => track.stop());
                if (context.state !== 'closed') await context.close();
                throw error;
            }
        }

        async function transcribe(audio, id) {
            processing = true;
            if (!config.sttContinuous) releaseCapture();
            const controller = new AbortController();
            requestController = controller;
            try {
                // Encode explicitly as little-endian PCM16, as expected by Whisper.
                const bytes = new Uint8Array(audio.length * 2);
                const view = new DataView(bytes.buffer);
                audio.forEach((sample, i) => view.setInt16(i * 2, sample, true));
                let binary = '';
                for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
                const response = await global.fetch('/api/stt/transcribe', {
                    method: 'POST', credentials: 'same-origin', signal: controller.signal,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ audio: global.btoa(binary), format: 'pcm', sampleRate: 16000,
                        language: config.ttsLang || 'ko', speechFilter: 'strict' })
                });
                if (!response.ok) throw new Error(await response.text());
                const result = await response.json();
                await acceptTranscript(result.rejected ? '' : result.text, id);
            } finally {
                if (requestController === controller) {
                    requestController = null;
                    processing = false;
                }
            }
        }

        function stop(options = {}) {
            const shouldSend = !options.suppressAutoSend && !options.forceAbort
                && browserHasText && !!messageInput?.value.trim();
            AppState.input.sttSuppressAutoSend = options.suppressAutoSend === true;
            ++session;
            global.clearTimeout(resumeTimer);
            resumeTimer = null;
            requestController?.abort();
            requestController = null;
            processing = false;
            releaseCapture();
            const recognition = AppState.input.recognition;
            AppState.input.recognition = null;
            browserHasText = false;
            AppState.input.isSTTActive = false;
            if (recognition) {
                try { recognition.abort(); } catch (_) {}
            }
            updateUIState();
            if (shouldSend) Promise.resolve(sendMessage?.({ fromVoiceInput: true })).catch(console.error);
        }

        function updateLayout() {
            const container = micLayoutContainer;
            if (!container) return;
            
            const layout = config.micLayout || 'none';
            
            // Clear all previous layout classes and specific styles
            global.document.body.classList.remove('layout-mic-bottom');
            container.classList.remove('mic-layout-left', 'mic-layout-right', 'mic-layout-bottom', 'mic-layout-inline');
            if (inlineMicBtn) {
                inlineMicBtn.hidden = true;
                inlineMicBtn.classList.remove('is-visible');
            }
            
            if (layout === 'none') {
                container.style.display = 'none';
            } else if (layout === 'inline') {
                container.style.display = 'none';
                if (inlineMicBtn) {
                    inlineMicBtn.hidden = false;
                    inlineMicBtn.classList.add('is-visible');
                }
            } else {
                container.style.display = 'flex';
                container.classList.add(`mic-layout-${layout}`);
                if (layout === 'bottom') {
                    global.document.body.classList.add('layout-mic-bottom');
                }
            }
        }

        return {
            toggle,
            start,
            stop,
            updateLayout
        };
    }

    global.DKSTMic = {
        createMicController
    };
})(window);
