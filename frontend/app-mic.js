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

            if (isActive) {
                if (messageInput) messageInput.classList.add('stt-active');
                startPlaceholderAnimation();
            } else {
                if (messageInput) messageInput.classList.remove('stt-active');
                stopPlaceholderAnimation();
                updateMessageInputPlaceholder?.();
            }
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

        function start() {
            if (!webkitSpeechRecognition) {
                alert(t('error.sttNotSupported'));
                return;
            }

            if (AppState.input.isSTTActive) return;

            triggerHaptic?.('light');
            
            const recognition = new webkitSpeechRecognition();
            recognition.lang = config.ttsLang === 'ko' ? 'ko-KR' : 'en-US';
            recognition.continuous = false;
            recognition.interimResults = true;

            recognition.onstart = () => {
                AppState.input.isSTTActive = true;
                updateUIState();
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

                if (messageInput) {
                    if (finalTranscript) {
                        const currentText = messageInput.value.trim();
                        messageInput.value = currentText ? `${currentText} ${finalTranscript}` : finalTranscript;
                        
                        // Auto-send if configured (logic simplified here, app.js will handle final send)
                        if (!AppState.input.sttSuppressAutoSend) {
                            // In original code, it auto-sends on final if short? 
                            // Let's just update the input and let user send for now or trigger callback
                        }
                    }
                }
            };

            recognition.onerror = (event) => {
                console.warn('[STT] Error:', event.error);
                stop();
                if (event.error === 'not-allowed') {
                    alert(t('error.micPermission'));
                }
            };

            recognition.onend = () => {
                stop();
            };

            try {
                recognition.start();
                AppState.input.recognition = recognition;
            } catch (e) {
                console.error('[STT] Start failed:', e);
                stop();
            }
        }

        function stop(options = {}) {
            const suppressAutoSend = options.suppressAutoSend === true;
            if (suppressAutoSend) {
                AppState.input.sttSuppressAutoSend = true;
            }

            if (AppState.input.recognition) {
                try {
                    // Logic for forceAbort could be added here if needed
                    AppState.input.recognition.stop();
                } catch (e) {}
                AppState.input.recognition = null;
            }
            AppState.input.isSTTActive = false;
            updateUIState();
        }

        function updateLayout() {
            const container = micLayoutContainer;
            if (!container) return;
            
            global.document.body.classList.remove('layout-mic-bottom');
            container.className = '';
            
            if (!config.micLayout || config.micLayout === 'none') {
                container.hidden = true;
            } else if (config.micLayout === 'inline') {
                container.hidden = true;
                if (inlineMicBtn) inlineMicBtn.hidden = false;
            } else {
                container.hidden = false;
                container.classList.add(`mic-layout-${config.micLayout}`);
                if (config.micLayout === 'bottom') {
                    global.document.body.classList.add('layout-mic-bottom');
                }
                if (inlineMicBtn) inlineMicBtn.hidden = true;
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
