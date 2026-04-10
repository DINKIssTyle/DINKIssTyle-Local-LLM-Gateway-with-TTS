/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTProgressUI(global) {
    /**
     * ProgressDock Component: Handles the display of progressive background tasks
     */
    function createProgressController(options = {}) {
        const { refs = {}, deps = {} } = options;
        const { 
            chatProgressDock,
            inputContainer
        } = refs;

        const { AppState, updateMessageInputPlaceholder, updateSendButtonStateCore } = deps;

        function render(label, percent = null, mode = 'prompt-processing', indeterminate = false) {
            if (!chatProgressDock) return;
            
            if (AppState.ui.progress.hideTimer) {
                global.clearTimeout(AppState.ui.progress.hideTimer);
                AppState.ui.progress.hideTimer = null;
            }

            const clamped = typeof percent === 'number' ? Math.max(0, Math.min(100, percent)) : null;
            const cardClass = `llm-progress-card ${mode}${indeterminate ? ' indeterminate' : ''}`;
            const percentLabel = clamped === null ? '' : `${clamped.toFixed(2)}%`;
            const width = indeterminate ? '32%' : `${clamped || 0}%`;

            AppState.ui.progress.active = true;
            AppState.ui.progress.label = label || '';
            AppState.ui.progress.percent = percentLabel;

            updateMessageInputPlaceholder?.();
            updateSendButtonStateCore?.();
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
                global.requestAnimationFrame(() => {
                    if (!chatProgressDock.hidden) {
                        chatProgressDock.classList.add('is-visible');
                    }
                });
            } else {
                chatProgressDock.classList.add('is-visible');
            }
        }

        function hide(delay = 500) {
            if (!chatProgressDock) return;
            
            if (AppState.ui.progress.hideTimer) {
                global.clearTimeout(AppState.ui.progress.hideTimer);
            }

            AppState.ui.progress.hideTimer = global.setTimeout(() => {
                chatProgressDock.classList.remove('is-visible');
                inputContainer?.classList.remove('has-progress');
                
                global.setTimeout(() => {
                    if (!AppState.ui.progress.active) {
                        chatProgressDock.hidden = true;
                        chatProgressDock.innerHTML = '';
                    }
                }, 300);

                AppState.ui.progress.active = false;
                AppState.ui.progress.hideTimer = null;
                updateMessageInputPlaceholder?.();
                updateSendButtonStateCore?.();
            }, delay);
        }

        return {
            render,
            hide
        };
    }

    global.DKSTProgressUI = {
        createProgressController
    };
})(window);
